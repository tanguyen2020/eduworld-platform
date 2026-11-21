//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// placeStudent registers a student of tenant A and places them in ref.students at (school, grade) (projection of F020).
func (c *curEnv) placeStudent(ctx context.Context, t *testing.T, sub string, school uuid.UUID, grade int) uuid.UUID {
	t.Helper()
	c.user(sub, authztest.Student(c.tenantA).PrincipalData, false)
	d := c.users[sub]
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version)
		VALUES ($1, $2, $3, $4, 1) ON CONFLICT (student_id) DO UPDATE SET school_id = EXCLUDED.school_id,
		grade_level = EXCLUDED.grade_level`, d.UserID, *d.TenantID, school, grade); err != nil {
		t.Fatal(err)
	}
	return d.UserID
}

// targeted lists the students of the snapshot of an assignment with their source and grade when targeted.
func (c *curEnv) targeted(ctx context.Context, t *testing.T, assignment string) map[uuid.UUID][2]any {
	t.Helper()
	rows, err := c.platform.QueryContext(ctx, `SELECT student_id, source, grade_level FROM lms.assignment_target_students
		WHERE assignment_id = $1 AND deleted_at IS NULL`, assignment)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[uuid.UUID][2]any{}
	for rows.Next() {
		var id uuid.UUID
		var source string
		var grade *int
		if err := rows.Scan(&id, &source, &grade); err != nil {
			t.Fatal(err)
		}
		out[id] = [2]any{source, grade}
	}
	return out
}

// F041-AC-32, AC-33 (ADR-0095 mục 3): an assignment for grade 3 of the school takes its active students plus the
// included student of grade 4 of the same school minus the excluded one (exclude wins over include); the snapshot keeps
// the grade when targeted; assignment.created/published v2 carry the counts; an included or listed student of another
// school or tenant is 422; invalid targets 422 by JSON pointer; listAssignments filters by school, subject and grade.
func TestAssignmentGradeTargets(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	an, binh := c.users[hsAn].UserID, c.users[hsBinh].UserID
	g4 := c.placeStudent(ctx, t, "hs.nam.4a", k.school, 4)
	g4other := c.placeStudent(ctx, t, "hs.lan.4b", k.school, 4)
	elsewhere := c.placeStudent(ctx, t, "hs.khac.3a", uuid.New(), 3) // tenant A, another school
	body := c.asgBody(k, map[string]any{"targets": []any{map[string]any{"target_type": "grade", "grade_level": 3,
		"include_student_ids": []string{g4.String(), binh.String()}, "exclude_student_ids": []string{binh.String()}}}})
	r := must(t, c.createAsg(t, adA, body), http.StatusCreated, "create grade 3")
	id := r.str("assignment_id")
	tg := r.body["targets"].([]any)[0].(map[string]any)
	if tg["target_type"] != "grade" || tg["grade_level"] != float64(3) || len(tg["include_student_ids"].([]any)) != 2 ||
		len(tg["exclude_student_ids"].([]any)) != 1 || tg["student_count"] != nil || r.str("subject_id") != k.subject {
		t.Fatalf("draft targets: %s", r.raw)
	}
	if ps := c.validPayloads(ctx, t, "assignment.created", id); len(ps) != 1 || ps[0]["subject_id"] != k.subject {
		t.Fatalf("assignment.created v2: %v", ps)
	}
	p := must(t, c.publishAsg(t, adA, r), http.StatusOK, "publish")
	snap := c.targeted(ctx, t, id)
	if len(snap) != 2 || snap[an][0] != "publish" || snap[g4][0] != "publish" {
		t.Fatalf("snapshot: %v", snap)
	}
	if _, ok := snap[binh]; ok {
		t.Fatal("the excluded student is targeted (exclude wins over include)")
	}
	if _, ok := snap[g4other]; ok {
		t.Fatal("a grade-4 student not included is targeted")
	}
	if g := snap[an][1].(*int); g == nil || *g != 3 {
		t.Fatalf("grade of An when targeted: %v", g)
	}
	if g := snap[g4][1].(*int); g == nil || *g != 4 {
		t.Fatalf("grade of the included student when targeted: %v", g)
	}
	if pt := p.body["targets"].([]any)[0].(map[string]any); pt["student_count"] != float64(2) {
		t.Fatalf("published targets: %s", p.raw)
	}
	pub := c.validPayloads(ctx, t, "assignment.published", id)
	if len(pub) != 1 {
		t.Fatalf("assignment.published: %v", pub)
	}
	pt := pub[0]["targets"].([]any)[0].(map[string]any)
	if pt["target_type"] != "grade" || pt["grade_level"] != float64(3) || pt["student_count"] != float64(2) ||
		pt["included_student_count"] != float64(2) || pt["excluded_student_count"] != float64(1) {
		t.Fatalf("assignment.published v2 targets: %v", pt)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type IN ('assignment.created','assignment.published')
		AND envelope->'payload'->>'assignment_id' = $1 AND (envelope->>'schema_version')::int = 2`, id); n != 2 {
		t.Fatalf("schema_version 2 envelopes: %d", n)
	}
	// 422: a student of another school included or listed, a student of another tenant listed.
	for _, bad := range []struct {
		targets []any
		field   string
	}{
		{[]any{map[string]any{"target_type": "grade", "grade_level": 3, "include_student_ids": []string{elsewhere.String()}}},
			"/targets/0/include_student_ids:INVALID"},
		{[]any{map[string]any{"target_type": "students", "student_ids": []string{an.String(), elsewhere.String()}}}, "/targets/0/student_ids:INVALID"},
		{[]any{map[string]any{"target_type": "students", "student_ids": []string{c.users[hsDung].UserID.String()}}}, "/targets/0/student_ids:INVALID"},
		{[]any{map[string]any{"target_type": "grade", "grade_level": 3}, map[string]any{"target_type": "grade", "grade_level": 3}},
			"/targets/1/grade_level:INVALID"},
		{[]any{map[string]any{"target_type": "grade"}}, "/targets/0/grade_level:INVALID"},
		{[]any{map[string]any{"target_type": "students", "student_ids": []string{an.String()}, "exclude_student_ids": []string{binh.String()}}},
			"/targets/0:INVALID"},
	} {
		x := c.createAsg(t, adA, c.asgBody(k, map[string]any{"targets": bad.targets}))
		if x.status != http.StatusUnprocessableEntity || !slices.Contains(x.errorCodes(), bad.field) {
			t.Fatalf("%v: %d %s", bad.targets, x.status, x.raw)
		}
	}
	// The removed target types are refused by the contract (an unknown course_id is ignored like any unknown field).
	for _, extra := range []map[string]any{{"targets": []any{map[string]any{"target_type": "class", "target_id": k.class.String()}}},
		{"targets": []any{map[string]any{"target_type": "study_group", "target_id": uuid.New().String()}}}} {
		if x := c.createAsg(t, adA, c.asgBody(k, extra)); x.status != http.StatusUnprocessableEntity && x.status != http.StatusBadRequest {
			t.Fatalf("%v: %d %s", extra, x.status, x.raw)
		}
	}
	// A subject the tenant does not see is 422; school_id is required.
	if x := c.createAsg(t, adA, c.asgBody(k, map[string]any{"subject_id": uuid.New().String()})); x.status != http.StatusForbidden &&
		x.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown subject: %d %s", x.status, x.raw)
	}
	// listAssignments by school, subject and targeted grade.
	list := func(q string) int {
		return len(must(t, c.do(t, http.MethodGet, assignmentsPath+"?"+q, adA, nil, nil), http.StatusOK, "list "+q).items("data"))
	}
	if n := list("grade_level=3&school_id=" + k.school.String() + "&subject_id=" + k.subject); n != 1 {
		t.Fatalf("grade 3 of the school: %d", n)
	}
	if n := list("grade_level=4"); n != 0 {
		t.Fatalf("grade 4: %d", n)
	}
	if n := list("school_id=" + uuid.New().String()); n != 0 {
		t.Fatalf("another school: %d", n)
	}
}

// placementEvent hands an identity event to the consumer learning.assignment_enrollment and returns its error.
func (c *curEnv) placementEvent(ctx context.Context, t *testing.T, typ string, tenant uuid.UUID, payload map[string]any) error {
	t.Helper()
	raw, _ := json.Marshal(payload)
	e := outbox.Envelope{EventID: ids.New(), EventType: typ, TenantID: &tenant, OccurredAt: c.clk.Now(), Payload: raw}
	h := consumer.AssignmentEnrollment{Tracker: c.asg}
	return c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, tenant)), func(ctx context.Context) error { return h.Handle(ctx, e) })
}

// F041-AC-16 (ADR-0095 mục 3, mục 7): a student who joins grade 3 of the school while the assignment is open is added
// (student.placement_changed.v1, also student.synced with a grade; enrollment.changed is gone — WP-I1); not when the
// grade target excludes them, not for another grade or school, not when inactive; a placement of
// another tenant is refused; a locked account is excused before the due date.
func TestAssignmentPlacementChanges(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	excluded := c.placeStudent(ctx, t, "hs.loai.3a", k.school, 2)
	id := c.openAsg(t, k, map[string]any{"targets": []any{map[string]any{"target_type": "grade", "grade_level": 3,
		"exclude_student_ids": []string{excluded.String()}}}})
	placement := func(student uuid.UUID, school uuid.UUID, grade int, status string) map[string]any {
		return map[string]any{"student_id": student, "tenant_id": c.tenantA, "school_id": school, "grade_level": grade, "status": status,
			"source": "sis", "source_version": 2, "changed_fields": []string{"grade_level"}}
	}
	newcomer := c.placeStudent(ctx, t, "hs.moi.3a", k.school, 3)
	if err := c.placementEvent(ctx, t, "student.placement_changed", c.tenantA, placement(newcomer, k.school, 3, "active")); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := c.submission(ctx, t, id, newcomer); st != "not_started" {
		t.Fatalf("newcomer: %s", st)
	}
	if s := c.targeted(ctx, t, id)[newcomer]; s[0] != "grade_added" || s[1] == nil || *(s[1].(*int)) != 3 {
		t.Fatalf("newcomer source: %v", s)
	}
	// Redelivery adds nothing more.
	if err := c.placementEvent(ctx, t, "student.placement_changed", c.tenantA, placement(newcomer, k.school, 3, "active")); err != nil {
		t.Fatal(err)
	}
	if n := c.outboxCount(ctx, t, "assignment.submission_updated", id); n != 1 {
		t.Fatalf("submission_updated after a redelivery: %d", n)
	}
	// Excluded, another grade, another school, inactive: not added.
	other := c.placeStudent(ctx, t, "hs.khac.3a", uuid.New(), 3)
	g4 := c.placeStudent(ctx, t, "hs.bon.4a", k.school, 4)
	inactive := c.placeStudent(ctx, t, "hs.nghi.3a", k.school, 3)
	for _, e := range []map[string]any{placement(excluded, k.school, 3, "active"), placement(g4, k.school, 4, "active"),
		placement(other, other, 3, "active"), placement(inactive, k.school, 3, "inactive")} {
		if err := c.placementEvent(ctx, t, "student.placement_changed", c.tenantA, e); err != nil {
			t.Fatal(err)
		}
	}
	snap := c.targeted(ctx, t, id)
	for _, s := range []uuid.UUID{excluded, g4, other, inactive} {
		if _, ok := snap[s]; ok {
			t.Fatalf("%s was added: %v", s, snap)
		}
	}
	// A placement whose tenant is not the envelope tenant is refused (R8).
	if err := c.placementEvent(ctx, t, "student.placement_changed", c.tenantA, map[string]any{"student_id": uuid.New(), "tenant_id": c.tenantB,
		"school_id": k.school, "grade_level": 3, "status": "active", "source": "sis", "source_version": 1}); err == nil {
		t.Fatal("a placement of another tenant was applied")
	} else if !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("not a permanent error: %v", err)
	}
	// student.synced with the grade adds students too; enrollment.changed (no longer produced, ADR-0095) is ignored.
	synced := c.placeStudent(ctx, t, "hs.dongbo.3a", k.school, 3)
	if err := c.placementEvent(ctx, t, "student.synced", c.tenantA, map[string]any{"student_id": synced, "school_id": k.school,
		"action": "update", "grade_level": 3}); err != nil {
		t.Fatal(err)
	}
	enrolled := c.placeStudent(ctx, t, "hs.ghidanh.3a", k.school, 3)
	if err := c.placementEvent(ctx, t, "enrollment.changed", c.tenantA, map[string]any{"student_id": enrolled, "class_id": uuid.New(),
		"op": "added", "status": "active"}); err != nil {
		t.Fatal(err)
	}
	snap = c.targeted(ctx, t, id)
	if snap[synced][0] != "grade_added" {
		t.Fatalf("%s not added: %v", synced, snap)
	}
	if _, ok := snap[enrolled]; ok {
		t.Fatalf("enrollment.changed must be ignored: %v", snap)
	}
	// A locked account is excused before the due date.
	if err := c.placementEvent(ctx, t, "user.access_revoked", c.tenantA, map[string]any{"user_id": c.users[hsBinh].UserID, "reason": "locked"}); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := c.submission(ctx, t, id, c.users[hsBinh].UserID); st != "excused" {
		t.Fatalf("locked: %s", st)
	}
	if r := asgStart(c, t, "hs.moi.3a", k.s.activityID, id); r.status != http.StatusCreated {
		t.Fatalf("newcomer starts: %d %s", r.status, r.raw)
	}
}
