//go:build integration

package integration

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/enrollment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// enrolled creates a student enrolled in f.class (platform role, outside the use cases).
func enrolled(ctx context.Context, t *testing.T, platform *sql.DB, f fixture, username string) uuid.UUID {
	t.Helper()
	tx, err := platform.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := insertStudent(ctx, tx, f, username)
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO org.enrollments (enrollment_id, tenant_id, created_at, updated_at, school_id, student_id, class_id, status, start_date, source)
			VALUES ($1, $2, now(), now(), $3, $4, $5, 'active', '2026-09-05', 'sis')`, ids.New().String(), f.tenant, f.school, id, f.class)
	}
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return uuid.MustParse(id)
}

func addClass(ctx context.Context, t *testing.T, platform *sql.DB, tenant, school, name string) uuid.UUID {
	t.Helper()
	var year string
	if err := platform.QueryRowContext(ctx, `SELECT academic_year_id FROM org.academic_years WHERE school_id = $1 AND is_current AND deleted_at IS NULL AND tenant_id = $2`, school, tenant).Scan(&year); err != nil {
		t.Fatal(err)
	}
	id := ids.New()
	if _, err := platform.ExecContext(ctx, `INSERT INTO org.classes (class_id, tenant_id, created_at, updated_at, school_id, academic_year_id, grade_level, name)
		VALUES ($1, $2, now(), now(), $3, $4, 3, $5)`, id, tenant, school, year, name); err != nil {
		t.Fatal(err)
	}
	return id
}

// F010-AC-11, AC-13, AC-14, AC-15 with the real repositories.
func TestTransferStudent(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-ta")
	b := seedTenant(ctx, t, e.platform, "pgd-tb")
	h := command.TransferStudent{TransferDeps: command.TransferDeps{UoW: e.uow, Enrollments: write.EnrollmentRepo{}, Students: write.StudentRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}}
	ta := uuid.MustParse(a.tenant)
	scopeA, _ := authz.EventScope(ta)
	inA := tenancy.WithScope(ctx, scopeA)
	on := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

	// AC-13: class transfer inside the school (3A → 3B).
	an := enrolled(ctx, t, e.platform, a, "hs.an.ta")
	class3B := addClass(ctx, t, e.platform, a.tenant, a.school, "3B")
	tr, err := h.Handle(inA, command.TransferStudentInput{StudentID: an, TargetClassID: class3B, On: on, Source: enrollment.SourceManual})
	if err != nil || tr.Kind != enrollment.KindClass {
		t.Fatalf("class transfer = %+v %v", tr, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE student_id = $1 AND status = 'transferred' AND end_date = '2026-11-19'`, an); n != 1 {
		t.Fatalf("closed enrollments = %d", n)
	}
	// AC-14: school transfer inside the unit tenant keeps tenant_id.
	tr, err = h.Handle(inA, command.TransferStudentInput{StudentID: an, TargetClassID: uuid.MustParse(a.otherClass), On: on.AddDate(0, 0, 1), Source: enrollment.SourceSIS})
	if err != nil || tr.Kind != enrollment.KindSchool {
		t.Fatalf("school transfer = %+v %v", tr, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND school_id = $2 AND tenant_id = $3`, an, a.otherSchool, a.tenant); n != 1 {
		t.Fatal("student school not moved (or tenant changed)")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.transferred' AND envelope->'payload'->>'student_id' = $1`, an.String()); n != 1 {
		t.Fatalf("student.transferred = %d", n)
	}
	// AC-15: other tenant (System Admin cross-tenant scope over both tenants): user_id kept, history keeps old tenant.
	sa := authztest.Platform("system_admin")
	cs, err := authz.CrossTenantScope(sa, []uuid.UUID{ta, uuid.MustParse(b.tenant)}, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		t.Fatal(err)
	}
	cross := tenancy.WithScope(authz.WithPrincipal(ctx, sa), cs)
	tr, err = h.Handle(cross, command.TransferStudentInput{StudentID: an, TargetClassID: uuid.MustParse(b.class), On: on.AddDate(0, 0, 2), Source: enrollment.SourceSIS})
	if err != nil || tr.Kind != enrollment.KindTenant {
		t.Fatalf("tenant transfer = %+v %v", tr, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND primary_tenant_id = $2`, an, b.tenant); n != 1 {
		t.Fatal("primary tenant not moved")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE student_id = $1 AND tenant_id = $2 AND status = 'transferred'`, an, a.tenant); n != 3 {
		t.Fatalf("history in the old tenant = %d, want 3 closed enrollments (3A, 3B, other school)", n)
	}
	var ops []string
	rows, err := e.relay.QueryContext(ctx, `SELECT envelope->'payload'->>'op' FROM infra.outbox_events
		WHERE event_type = 'tenant_membership.changed' AND envelope->'payload'->>'user_id' = $1 ORDER BY envelope->>'recorded_at', event_id`, an.String())
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var op string
		_ = rows.Scan(&op)
		ops = append(ops, op)
	}
	_ = rows.Close()
	if len(ops) != 2 || ops[0] != "removed" || ops[1] != "added" {
		t.Fatalf("tenant_membership.changed ops = %v, want removed then added", ops)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.tenant_memberships_v WHERE user_id = $1`, an); n != 1 {
		t.Fatalf("memberships after transfer = %d", n)
	}

	// AC-11: concurrent transfers of one student end with exactly one active enrollment.
	em := enrolled(ctx, t, e.platform, a, "hs.em.ta")
	targets := []uuid.UUID{addClass(ctx, t, e.platform, a.tenant, a.school, "3C"), addClass(ctx, t, e.platform, a.tenant, a.school, "3D")}
	var wg sync.WaitGroup
	for _, c := range targets {
		wg.Add(1)
		go func(c uuid.UUID) {
			defer wg.Done()
			_, _ = h.Handle(inA, command.TransferStudentInput{StudentID: em, TargetClassID: c, On: on, Source: enrollment.SourceManual})
		}(c)
	}
	wg.Wait()
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE student_id = $1 AND status = 'active' AND deleted_at IS NULL`, em); n != 1 {
		t.Fatalf("active enrollments after concurrent transfers = %d, want 1", n)
	}
	// Tenant B cannot transfer tenant A's students (scope excludes A → 404).
	scopeB, _ := authz.EventScope(uuid.MustParse(b.tenant))
	if _, err := h.Handle(tenancy.WithScope(ctx, scopeB), command.TransferStudentInput{StudentID: em, TargetClassID: uuid.MustParse(b.class), On: on, Source: "sis"}); code(err) != "NOT_FOUND" {
		t.Fatalf("transfer from another tenant's scope: %v", err)
	}
}
