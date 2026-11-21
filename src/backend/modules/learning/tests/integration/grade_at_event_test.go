//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// gradesOfAttempt lists the distinct grade_level of the learning events of an attempt ("" for NULL).
func (c *curEnv) gradesOfAttempt(ctx context.Context, t *testing.T, attempt string) string {
	t.Helper()
	var s string
	if err := c.platform.QueryRowContext(ctx, `SELECT COALESCE(string_agg(DISTINCT COALESCE(grade_level::text, ''), ','), '')
		FROM learning.learning_events WHERE payload->>'attempt_id' = $1`, attempt).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// F031-AC-31 (ADR-0095 mục 2): the grade of the student is recorded on each learning event and daily progress row when it
// is written (from ref.students), never derived from the current grade: after the student moves up a grade, the old
// events keep the old grade and the new ones carry the new grade.
func TestGradeAtEventTime(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.lmsSetup(ctx, t)
	first := c.quizRun(t, hsAn, k.s.activityID)
	if g := c.gradesOfAttempt(ctx, t, first); g != "3" {
		t.Fatalf("grade of the events of grade 3: %q", g)
	}
	an := c.users[hsAn].UserID
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	addDaily := func() {
		t.Helper()
		if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, c.tenantA)), func(ctx context.Context) error {
			return write.MasteryRepo{}.AddDaily(ctx, portwrite.ProgressDelta{TenantID: c.tenantA, StudentID: an, SchoolID: k.school,
				SubjectID: uuid.MustParse(k.s.qc.subject), Day: day, ActivitiesCompleted: 1})
		}); err != nil {
			t.Fatal(err)
		}
	}
	addDaily()
	if _, err := c.platform.ExecContext(ctx, `UPDATE ref.students SET grade_level = 4 WHERE student_id = $1`, an); err != nil {
		t.Fatal(err)
	}
	second := c.quizRun(t, hsAn, k.s.activityID)
	if g := c.gradesOfAttempt(ctx, t, second); g != "4" {
		t.Fatalf("grade of the events after the move: %q", g)
	}
	if g := c.gradesOfAttempt(ctx, t, first); g != "3" {
		t.Fatalf("old events follow the current grade: %q", g)
	}
	addDaily() // the bucket of the day keeps the grade it was first written with
	var grade int
	if err := c.platform.QueryRowContext(ctx, `SELECT grade_level FROM learning.progress_daily WHERE student_id = $1 AND bucket_date = $2`,
		an, day).Scan(&grade); err != nil || grade != 3 {
		t.Fatalf("progress_daily grade: %d %v", grade, err)
	}
}

// F031-AC-32 (ADR-0095 mục 8): the backfill of migration 20261229000002 sets the grade of the existing rows from their
// class (ref.classes), otherwise the current grade of the student; re-run as learning_migrator in a rolled back
// transaction.
func TestGradeBackfill(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.lmsSetup(ctx, t)
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "learning", "migrations",
		"20261229000002_adr0095_grade_at_event.sql"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, j := strings.Index(s, "-- BEGIN adr0095-grade-backfill"), strings.Index(s, "-- END adr0095-grade-backfill")
	if i < 0 || j < i {
		t.Fatal("backfill markers not found")
	}
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
	class5 := uuid.New()
	exec(`INSERT INTO ref.classes (class_id, tenant_id, school_id, grade_level, name, source_version) VALUES ($1, $2, $3, 5, '5A', 1)`,
		class5, c.tenantA, k.school)
	exec(`SELECT set_config('app.tenant_id', $1, true)`, c.tenantA.String())
	an, binh := c.users[hsAn].UserID, c.users[hsBinh].UserID
	ev := func(student uuid.UUID, class *uuid.UUID, key string) {
		exec(`INSERT INTO learning.learning_events (event_id, occurred_at, event_type, student_id, tenant_id, school_id, class_id, context,
			payload, idempotency_key) VALUES ($1, now(), 'activity.started', $2, $3, $4, $5, 'practice', '{}'::jsonb, $6)`,
			uuid.New(), student, c.tenantA, k.school, class, key)
	}
	ev(an, &class5, "bf-class")
	ev(binh, nil, "bf-student")
	exec(s[i:j])
	exec(`SELECT set_config('app.tenant_id', $1, true)`, c.tenantA.String())
	for key, want := range map[string]int{"bf-class": 5, "bf-student": 3} {
		var g int
		if err := tx.QueryRowContext(ctx, `SELECT grade_level FROM learning.learning_events WHERE idempotency_key = $1`, key).Scan(&g); err != nil || g != want {
			t.Fatalf("%s: grade %d (want %d) %v", key, g, want, err)
		}
	}
	// The temporary policies of the backfill are gone.
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM pg_policies WHERE policyname LIKE 'adr0095_backfill%'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("backfill policies left: %d %v", n, err)
	}
}
