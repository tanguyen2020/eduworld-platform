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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// enrolled creates an effective student of f.school, grade 3 (platform role, outside the use cases). There is no
// enrollment any more: the student row places the student (ADR-0095 mục 2).
func enrolled(ctx context.Context, t *testing.T, platform *sql.DB, f fixture, username string) uuid.UUID {
	t.Helper()
	tx, err := platform.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := insertStudent(ctx, tx, f, username)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return uuid.MustParse(id)
}

// placementEvents counts student.placement_changed of a student with the given changed field.
func placementEvents(ctx context.Context, t *testing.T, relay *sql.DB, student uuid.UUID, field string) int {
	t.Helper()
	return count(ctx, t, relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.placement_changed'
		AND envelope->'payload'->>'student_id' = $1 AND envelope->'payload'->'changed_fields' ? $2`, student.String(), field)
}

// F010-AC-11, AC-13, AC-14, AC-15 with the real repositories (ADR-0095: grade, school, tenant; no class).
func TestTransferStudent(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-ta")
	b := seedTenant(ctx, t, e.platform, "pgd-tb")
	h := command.TransferStudent{TransferDeps: command.TransferDeps{UoW: e.uow, Schools: write.SchoolRepo{}, Students: write.StudentRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}}
	ta := uuid.MustParse(a.tenant)
	scopeA, _ := authz.EventScope(ta)
	inA := tenancy.WithScope(ctx, scopeA)
	on := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

	// AC-13 (ADR-0095): lên khối inside the school (3 → 4): student.placement_changed with grade_level only.
	an := enrolled(ctx, t, e.platform, a, "hs.an.ta")
	tr, err := h.Handle(inA, command.TransferStudentInput{StudentID: an, TargetSchoolID: uuid.MustParse(a.school), TargetGrade: 4, On: on,
		Source: placement.SourceManual})
	if err != nil || tr.Kind != placement.KindGrade {
		t.Fatalf("grade move = %+v %v", tr, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND grade_level = 4 AND school_id = $2`, an, a.school); n != 1 {
		t.Fatal("grade not moved")
	}
	if placementEvents(ctx, t, e.relay, an, "grade_level") != 1 || placementEvents(ctx, t, e.relay, an, "school_id") != 0 {
		t.Fatal("student.placement_changed of the grade move")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.placement_changed'
		AND envelope->'payload'->>'student_id' = $1 AND envelope->'payload'->>'effective_from' = '2026-11-19'
		AND envelope->'payload'->>'previous_grade_level' = '3' AND envelope->'payload'->>'academic_year_id' = $2`, an.String(), a.year); n != 1 {
		t.Fatal("placement payload: effective_from, previous grade, academic year")
	}
	// AC-14: school transfer inside the unit tenant keeps tenant_id.
	tr, err = h.Handle(inA, command.TransferStudentInput{StudentID: an, TargetSchoolID: uuid.MustParse(a.otherSchool), TargetGrade: 4, On: on.AddDate(0, 0, 1),
		Source: placement.SourceSIS})
	if err != nil || tr.Kind != placement.KindSchool {
		t.Fatalf("school transfer = %+v %v", tr, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND school_id = $2 AND tenant_id = $3`, an, a.otherSchool, a.tenant); n != 1 {
		t.Fatal("student school not moved (or tenant changed)")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.transferred' AND envelope->'payload'->>'student_id' = $1`, an.String()); n != 1 {
		t.Fatalf("student.transferred = %d", n)
	}
	if placementEvents(ctx, t, e.relay, an, "school_id") != 1 {
		t.Fatal("student.placement_changed of the school transfer")
	}
	// AC-15: other tenant (System Admin cross-tenant scope over both tenants): user_id kept.
	sa := authztest.Platform("system_admin")
	cs, err := authz.CrossTenantScope(sa, []uuid.UUID{ta, uuid.MustParse(b.tenant)}, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		t.Fatal(err)
	}
	cross := tenancy.WithScope(authz.WithPrincipal(ctx, sa), cs)
	tr, err = h.Handle(cross, command.TransferStudentInput{StudentID: an, TargetSchoolID: uuid.MustParse(b.school), TargetGrade: 4, On: on.AddDate(0, 0, 2),
		Source: placement.SourceSIS})
	if err != nil || tr.Kind != placement.KindTenant {
		t.Fatalf("tenant transfer = %+v %v", tr, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND primary_tenant_id = $2`, an, b.tenant); n != 1 {
		t.Fatal("primary tenant not moved")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.placement_changed'
		AND envelope->'payload'->>'student_id' = $1 AND envelope->>'tenant_id' = $2 AND envelope->'payload'->>'previous_tenant_id' = $3`,
		an.String(), b.tenant, a.tenant); n != 1 {
		t.Fatal("student.placement_changed of the tenant transfer is published in the new tenant")
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
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.tenant_memberships_v WHERE user_id = $1 AND tenant_id = $2 AND school_id = $3`,
		an, b.tenant, b.school); n != 1 {
		t.Fatalf("memberships after transfer = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'enrollment.changed'`); n != 0 {
		t.Fatalf("enrollment.changed is no longer published: %d", n)
	}

	// AC-11: concurrent moves of one student serialize on the student row: both apply in turn, the version grows by 2.
	em := enrolled(ctx, t, e.platform, a, "hs.em.ta")
	var wg sync.WaitGroup
	for _, g := range []int{4, 5} {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			_, _ = h.Handle(inA, command.TransferStudentInput{StudentID: em, TargetSchoolID: uuid.MustParse(a.school), TargetGrade: g, On: on,
				Source: placement.SourceManual})
		}(g)
	}
	wg.Wait()
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND version = 3 AND grade_level IN (4, 5)`, em); n != 1 {
		t.Fatal("concurrent moves did not serialize")
	}
	if placementEvents(ctx, t, e.relay, em, "grade_level") != 2 {
		t.Fatal("one student.placement_changed per move")
	}
	// Tenant B cannot transfer tenant A's students (scope excludes A → 404).
	scopeB, _ := authz.EventScope(uuid.MustParse(b.tenant))
	if _, err := h.Handle(tenancy.WithScope(ctx, scopeB), command.TransferStudentInput{StudentID: em, TargetSchoolID: uuid.MustParse(b.school),
		TargetGrade: 3, On: on, Source: "sis"}); code(err) != "NOT_FOUND" {
		t.Fatalf("transfer from another tenant's scope: %v", err)
	}
}
