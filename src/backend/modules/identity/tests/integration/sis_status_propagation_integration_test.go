//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F016 US2: F016-AC-07, AC-08. A SIS import (F011 apply) suspends one student and transfers another out:
// the same chain as a staff lock (status history, user.status_changed + user.access_revoked with the US1
// payload, Keycloak consumer disables the account) in one sync cycle; the next game login is refused.
// Keycloak is the fake realm of the game harness; kcAccess is a TEST DOUBLE of the admin client.
func TestSisStatusPropagation(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-sis-st")
	ta, school := uuid.MustParse(a.tenant), uuid.MustParse(a.school)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: g.uow, Connections: write.SisConnectionRepo{}, Audit: g.env.deps.Audit}
	conn, err := conns.Create(sa, ta, "csv_import", "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, conn.ID, []uuid.UUID{school}, "pilot"); err != nil {
		t.Fatal(err)
	}
	run := command.SisRun{UoW: g.uow, Savepoints: g.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{},
		Students: write.StudentRepo{}, Events: g.env.deps.Events, Audit: g.env.deps.Audit, History: write.LifecycleRepo{},
		Locks: write.AccountLockRepo{}}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))
	imp := func(s1, s2 string) {
		t.Helper()
		files := map[string]string{
			"classes.csv": "class_external_id,school_external_id,academic_year,grade_level,name\nC-4A,TH-A,2026-2027,4,4A\n",
			"students.csv": "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n" +
				"HS-1,TH-A,001,An,4," + s1 + ",hs.st.an\nHS-2,TH-A,002,Binh,4," + s2 + ",hs.st.binh\n",
			"enrollments.csv": "student_external_id,class_external_id,start_date,status\nHS-1,C-4A,2026-09-05,active\nHS-2,C-4A,2026-09-05,active\n",
		}
		if _, err := run.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: sisFiles(t, files)}); err != nil {
			t.Fatal(err)
		}
	}
	imp("active", "active")
	userOf := func(username string) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := g.platform.QueryRowContext(ctx, `SELECT user_id FROM identity.users WHERE username = $1`, username).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	an, binh := userOf("hs.st.an"), userOf("hs.st.binh")
	for id, u := range map[uuid.UUID]string{an: "hs.st.an", binh: "hs.st.binh"} {
		g.identity(ctx, t, id, u)
		if _, err := g.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'active' WHERE user_id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if r := g.login(t, u, uuid.New()); r.code != 200 {
			t.Fatalf("login before: %s %d %v", u, r.code, r.body)
		}
	}

	// One sync cycle: An suspended, Binh transferred out.
	imp("suspended", "transferred_out")
	if count(ctx, t, g.platform, `SELECT count(*) FROM identity.user_status_history WHERE user_id = $1 AND from_status = 'active'
		AND to_status = 'locked' AND source = 'sis' AND reason = 'sis:suspended'`, an) != 1 {
		t.Fatal("status history of the SIS change")
	}
	var payload json.RawMessage
	if err := g.relay.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'user.access_revoked'
		AND envelope->'payload'->>'user_id' = $1`, an.String()).Scan(&payload); err != nil {
		t.Fatalf("user.access_revoked: %v", err)
	}
	if count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user.status_changed'
		AND envelope->'payload'->>'user_id' = $1 AND envelope->'payload'->>'to' = 'locked'`, an.String()) != 1 {
		t.Fatal("user.status_changed")
	}
	// The US1 Keycloak consumer handles the SIS event as it is (same payload): the account is disabled.
	kc := &kcAccess{disabled: map[string]int{}, enabled: map[string]int{}, logouts: map[string]int{}}
	c := consumer.KeycloakAccess{Access: command.KeycloakAccess{UoW: g.uow, Repo: write.LifecycleRepo{}, KC: kc, Issuers: []string{kcIssuer}}}
	if err := c.Handle(ctx, outbox.Envelope{EventID: uuid.New(), EventType: "user.access_revoked", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if kc.disabled["kc-hs.st.an"] != 1 || kc.logouts["kc-hs.st.an"] != 1 {
		t.Fatalf("keycloak: %+v", kc)
	}
	// AC-07: the next login of the suspended student is refused.
	if r := g.login(t, "hs.st.an", uuid.New()); r.code != 403 || codeOf(r) != string(problem.AccountLocked) {
		t.Fatalf("suspended login: %d %v", r.code, r.body)
	}
	// AC-08: transferred out → the student is no longer effective (org.students inactive, student.placement_changed —
	// ADR-0095), account untouched; login refused by membership.
	if count(ctx, t, g.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND status = 'inactive'`, binh) != 1 {
		t.Fatal("transferred_out student still effective")
	}
	if count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.placement_changed'
		AND envelope->'payload'->>'student_id' = $1 AND envelope->'payload'->>'status' = 'inactive'`, binh.String()) != 1 {
		t.Fatal("student.placement_changed of the transfer out")
	}
	r := g.login(t, "hs.st.binh", uuid.New())
	if c := codeOf(r); r.code != 403 || (c != string(problem.NoActiveEnrollment) && c != string(problem.AccountNotProvisioned)) {
		t.Fatalf("transferred login: %d %v", r.code, r.body)
	}
	// F124-AC-02 (SC-002): the SIS lock has its own source; a SIS record back to active releases that source only — a lock of
	// the Safety Team (sanction account_lock) keeps the account locked.
	if count(ctx, t, g.platform, `SELECT count(*) FROM identity.account_locks WHERE user_id = $1 AND source = 'sis' AND released_at IS NULL`, an) != 1 {
		t.Fatal("sis lock source")
	}
	if _, err := g.platform.ExecContext(ctx, `INSERT INTO identity.account_locks (lock_id, tenant_id, created_at, updated_at, user_id, source,
		source_ref_id) VALUES ($1, $2, now(), now(), $3, 'moderation_sanction', $4)`, uuid.New(), ta, an, uuid.New()); err != nil {
		t.Fatal(err)
	}
	imp("active", "active")
	var st string
	if err := g.platform.QueryRowContext(ctx, `SELECT status FROM identity.users WHERE user_id = $1`, an).Scan(&st); err != nil || st != "locked" {
		t.Fatalf("SIS active must not unlock a moderation lock: %s %v", st, err)
	}
	if count(ctx, t, g.platform, `SELECT count(*) FROM identity.account_locks WHERE user_id = $1 AND source = 'sis' AND released_at IS NULL`, an) != 0 {
		t.Fatal("sis lock released")
	}
}
