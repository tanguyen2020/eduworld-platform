//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F016 US4: F016-AC-17, AC-18 (restore on login) and AC-19 (restore by the Tenant Admin, the identity side of
// restoreTenantTrashItem; the HTTP route and its 403 for School Admin are tested by F045). Participants are
// test doubles (fakeParticipant).
func TestStudentRestore(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-rs-a")
	b := seedTenant(ctx, t, g.platform, "pgd-rs-b")
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	valid := g.student(ctx, t, a, "hs.rs.sis")   // active SIS record
	gone := g.student(ctx, t, a, "hs.rs.nghi")   // SIS record inactive (left the school)
	manual := g.student(ctx, t, a, "hs.rs.tudo") // no SIS link (manual account)
	for _, id := range []uuid.UUID{valid, gone, manual} {
		if _, err := g.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'active' WHERE user_id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}
	conn := uuid.New()
	if _, err := g.platform.ExecContext(ctx, `INSERT INTO sis.sis_connections (sis_connection_id, tenant_id, created_at, updated_at, provider_kind,
		config, secret_ref, status, capabilities) VALUES ($1, $2, now(), now(), 'contract_v1', '{}', 'test/sis', 'active', '[]')`, conn, a.tenant); err != nil {
		t.Fatal(err)
	}
	for id, rec := range map[uuid.UUID][2]string{valid: {"HS-RS-1", "active"}, gone: {"HS-RS-2", "inactive"}} {
		if _, err := g.platform.ExecContext(ctx, `INSERT INTO identity.sis_identity_links (link_id, tenant_id, created_at, updated_at, user_id, school_id,
			sis_connection_id, student_external_id, status, linked_at) VALUES ($1, $2, now(), now(), $3, $4, $5, $6, 'active', now())`,
			uuid.New(), a.tenant, id, a.school, conn, rec[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := g.platform.ExecContext(ctx, `INSERT INTO sis.synced_students (synced_student_id, tenant_id, created_at, updated_at, sis_connection_id,
			student_external_id, school_id, student_code, record_status, last_hash, last_synced_at) VALUES ($1, $2, now(), now(), $3, $4, $5, $4, $6, 'h', now())`,
			uuid.New(), a.tenant, conn, rec[0], a.school, rec[1]); err != nil {
			t.Fatal(err)
		}
	}

	game, learning := newFakeParticipant("game"), newFakeParticipant("learning")
	parts := []port.StudentDataParticipant{game, learning}
	eng := saga.Engine{UoW: g.uow, Repo: write.SagaRepo{}}
	cleanup := command.StudentCleanup{UoW: g.uow, Repo: write.CleanupRepo{}, Saga: eng, Participants: parts, Lifecycle: write.LifecycleRepo{},
		Events: g.env.deps.Events, Audit: g.env.deps.Audit}
	restore := command.StudentRestore{UoW: g.uow, Repo: write.CleanupRepo{}, Saga: eng, Participants: parts, Lifecycle: write.LifecycleRepo{},
		Events: g.env.deps.Events, Audit: g.env.deps.Audit}
	g.auth.Restore = restore
	clean := func(id uuid.UUID) {
		t.Helper()
		subject := map[string]any{"student_id": id, "tenant_id": ta, "run_id": uuid.New(), "cutoff_days": 90}
		if o, err := eng.Start(ctx, cleanup.Definition(), &ta, subject, "test:"+uuid.NewString(), nil); err != nil || o.Status != saga.StatusSucceeded {
			t.Fatalf("cleanup %s: %+v %v", id, o.Status, err)
		}
	}
	live := func(id uuid.UUID) bool {
		return count(ctx, t, g.platform, `SELECT count(*) FROM identity.users u JOIN org.students s ON s.student_id = u.user_id
			WHERE u.user_id = $1 AND u.deleted_at IS NULL AND s.deleted_at IS NULL`, id) == 1
	}
	restored := func(id uuid.UUID, via string) int {
		return count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.restored'
			AND envelope::text LIKE '%' || $1 || '%' AND envelope::text LIKE '%"restored_via": "' || $2 || '"%'`, id.String(), via) +
			count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.restored'
			AND envelope::text LIKE '%' || $1 || '%' AND envelope::text LIKE '%"restored_via":"' || $2 || '"%'`, id.String(), via)
	}
	for _, id := range []uuid.UUID{valid, gone, manual} {
		clean(id)
	}

	// AC-17: the Keycloak login succeeds (user kept enabled); valid in the SIS → restore saga, membership back.
	r := g.login(t, "hs.rs.sis", uuid.New())
	if r.code != 200 {
		t.Fatalf("login after cleanup: %d %v", r.code, r.body)
	}
	tokensOf(t, r)
	if !live(valid) || restored(valid, "login") != 1 || game.restored[valid] != 1 || learning.restored[valid] != 1 {
		t.Fatalf("restore on login: live=%v events=%d game=%d", live(valid), restored(valid, "login"), game.restored[valid])
	}
	if count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'tenant_membership.changed'
		AND envelope::text LIKE '%' || $1 || '%' AND envelope::text LIKE '%inactive_cleanup_restore%'`, valid.String()) != 1 {
		t.Fatal("tenant_membership.changed added")
	}
	if count(ctx, t, g.app, `SELECT count(*) FROM identity.memberships_for_user($1)`, valid) != 1 {
		t.Fatal("membership not back")
	}
	if count(ctx, t, g.platform, `SELECT count(*) FROM platform.saga_runs WHERE saga_type = 'student_restore' AND status = 'succeeded'
		AND subject->>'student_id' = $1`, valid.String()) != 1 {
		t.Fatal("saga run")
	}
	// A second login does not restore again.
	if r := g.login(t, "hs.rs.sis", uuid.New()); r.code != 200 || restored(valid, "login") != 1 {
		t.Fatalf("second login: %d", r.code)
	}

	// AC-18: SIS record no longer active → 403 ACCOUNT_DEACTIVATED, nothing restored.
	if r := g.login(t, "hs.rs.nghi", uuid.New()); r.code != 403 || codeOf(r) != string(problem.AccountDeactivated) {
		t.Fatalf("invalid SIS: %d %v", r.code, r.body)
	}
	if live(gone) || game.restored[gone] != 0 {
		t.Fatal("restored despite the SIS record")
	}
	// No SIS link (manual account / common tenant) → restored.
	if r := g.login(t, "hs.rs.tudo", uuid.New()); r.code != 200 || !live(manual) {
		t.Fatalf("manual account: %d %v", r.code, r.body)
	}

	// AC-19: restore by the Tenant Admin of the tenant (identity side of restoreTenantTrashItem); another
	// tenant, or a student not removed by the cleanup, is 404.
	clean(valid)
	admin := func(tenant uuid.UUID) context.Context {
		p := authztest.Staff(tenant, "tenant_admin")
		sc, err := authz.ScopeFor(p)
		if err != nil {
			t.Fatal(err)
		}
		return tenancy.WithScope(authztest.Ctx(ctx, p), sc)
	}
	if err := restore.RestoreStudentByAdmin(admin(tb), tb, valid); problem.From(err, "").Code != problem.NotFound {
		t.Fatalf("other tenant: %v", err)
	}
	if err := restore.RestoreStudentByAdmin(admin(ta), ta, manual); problem.From(err, "").Code != problem.NotFound {
		t.Fatalf("live student: %v", err)
	}
	if err := restore.RestoreStudentByAdmin(admin(ta), ta, valid); err != nil {
		t.Fatal(err)
	}
	if !live(valid) || restored(valid, "admin") != 1 {
		t.Fatal("admin restore")
	}
	if count(ctx, t, g.platform, `SELECT count(*) FROM identity.user_status_history WHERE user_id = $1 AND to_status = 'active'
		AND source = 'restore' AND reason = 'restored_via_admin'`, valid) != 1 {
		t.Fatal("status history")
	}
}
