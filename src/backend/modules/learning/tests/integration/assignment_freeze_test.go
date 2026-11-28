//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// freezeBlock is the conversion of the class and study-group targets of migration 20261229000001 (between its markers).
func freezeBlock(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "learning", "migrations",
		"20261229000001_adr0095_assignment_grade_targets.sql"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, j := strings.Index(s, "-- BEGIN adr0095-freeze"), strings.Index(s, "-- END adr0095-freeze")
	if i < 0 || j < i {
		t.Fatal("freeze markers not found")
	}
	return s[i:j]
}

// F041-AC-34 (ADR-0095 mục 3): an assignment given to a class or a study group before the change becomes a frozen list
// of students — the snapshot of a published assignment (only the students of that class / group), the current members
// of a draft — never the whole grade; target_ref_id keeps the old id. The migration block is re-run as
// learning_migrator in a rolled back transaction with the old CHECK lifted to write the legacy rows.
func TestAssignmentClassTargetsFrozen(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	tx, err := c.migrator.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%v: %s", err, q)
		}
	}
	preADR0095Schema(ctx, t, tx)
	exec(`ALTER TABLE lms.assignment_targets DROP CONSTRAINT assignment_targets_type_ck`)
	exec(`SELECT set_config('app.tenant_id', $1, true)`, c.tenantA.String())
	an, binh := c.users[hsAn].UserID, c.users[hsBinh].UserID
	// A grade-3 student of the school outside class 3A: a frozen target never takes them.
	other := uuid.New()
	exec(`INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version) VALUES ($1, $2, $3, 3, 1)`,
		other, c.tenantA, k.school)
	group := uuid.New()
	exec(`INSERT INTO lms.study_groups (study_group_id, tenant_id, school_id, grade_level, name, member_count, created_by, updated_by)
		VALUES ($1, $2, $3, 3, 'Nhóm Toán', 1, $4, $4)`, group, c.tenantA, k.school, c.users[adA].UserID)
	exec(`INSERT INTO lms.study_group_members (study_group_member_id, study_group_id, tenant_id, school_id, student_id, added_by, added_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())`, uuid.New(), group, c.tenantA, k.school, binh, c.users[adA].UserID)
	now := time.Now().UTC()
	newAsg := func(status string) uuid.UUID {
		id := uuid.New()
		exec(`INSERT INTO lms.assignments (assignment_id, tenant_id, school_id, subject_id, created_by, created_by_role, title, content_type,
			content_id, pinned_version_id, mode, open_at, due_at, feedback_timing, result_release, status)
			VALUES ($1, $2, $3, $4, $5, 'school_admin', 'Bài cũ', 'activity', $6, $7, 'practice', $8, $9, 'immediate', 'immediate', $10)`,
			id, c.tenantA, k.school, k.subject, c.users[adA].UserID, k.s.activityID, uuid.New(), now, now.Add(72*time.Hour), status)
		return id
	}
	target := func(asg uuid.UUID, typ string, ref uuid.UUID, pos int) {
		exec(`INSERT INTO lms.assignment_targets (assignment_target_id, tenant_id, school_id, assignment_id, target_type, target_ref_id, position)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, uuid.New(), c.tenantA, k.school, asg, typ, ref, pos)
	}
	snapshot := func(asg, student uuid.UUID, class *uuid.UUID) {
		exec(`INSERT INTO lms.assignment_target_students (assignment_target_student_id, tenant_id, school_id, assignment_id, student_id,
			class_id_at_assign, source) VALUES ($1, $2, $3, $4, $5, $6, 'publish')`, uuid.New(), c.tenantA, k.school, asg, student, class)
	}
	// Legacy class 3A of the projection with the active enrollments of An and Bình (tables of the pre-drop schema).
	exec(`INSERT INTO ref.classes (class_id, tenant_id, school_id, grade_level, name, source_version) VALUES ($1, $2, $3, 3, '3A', 1)`,
		k.class, c.tenantA, k.school)
	for _, s := range []uuid.UUID{an, binh} {
		exec(`INSERT INTO ref.enrollments (student_id, class_id, start_date, tenant_id, status, source_version)
			VALUES ($1, $2, current_date, $3, 'active', 1)`, s, k.class, c.tenantA)
	}
	// Published: class 3A (snapshot An, Bình) + study group (snapshot Bình); the grade-3 outsider is not in the snapshot.
	pub := newAsg("open")
	target(pub, "class", k.class, 1)
	target(pub, "study_group", group, 2)
	snapshot(pub, an, &k.class)
	snapshot(pub, binh, &k.class)
	// Draft: class 3A (active enrollments An, Bình) + study group (live member Bình).
	draft := newAsg("draft")
	target(draft, "class", k.class, 1)
	target(draft, "study_group", group, 2)

	exec(freezeBlock(t))
	exec(`SELECT set_config('app.tenant_id', $1, true)`, c.tenantA.String())

	got := map[uuid.UUID][]string{}
	func() {
		rows, err := tx.QueryContext(ctx, `SELECT assignment_id, target_type, target_ref_id, array_to_string(student_ids, ',')
		FROM lms.assignment_targets WHERE assignment_id = ANY($1::uuid[]) ORDER BY assignment_id, position`, "{"+pub.String()+","+draft.String()+"}")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var asg, ref uuid.UUID
			var typ, students string
			if err := rows.Scan(&asg, &typ, &ref, &students); err != nil {
				t.Fatal(err)
			}
			if typ != "students" || (ref != k.class && ref != group) {
				t.Fatalf("converted target: %s %s", typ, ref)
			}
			ids := strings.Split(students, ",")
			sort.Strings(ids)
			got[asg] = append(got[asg], strings.Join(ids, ","))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}()
	both := []string{an.String(), binh.String()}
	sort.Strings(both)
	want := []string{strings.Join(both, ","), binh.String()}
	for _, asg := range []uuid.UUID{pub, draft} {
		if len(got[asg]) != 2 || got[asg][0] != want[0] || got[asg][1] != want[1] {
			t.Fatalf("frozen lists of %s: %v (want %v)", asg, got[asg], want)
		}
		if strings.Contains(strings.Join(got[asg], ","), other.String()) {
			t.Fatalf("a frozen target was widened to the grade: %v", got[asg])
		}
	}
	// The new CHECK accepts the converted rows (no old value left).
	exec(`ALTER TABLE lms.assignment_targets ADD CONSTRAINT assignment_targets_type_ck CHECK (target_type IN ('grade','students'))`)
}
