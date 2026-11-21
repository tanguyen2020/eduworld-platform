//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// dashProxy serves the HTTP routes with the current query.Dashboards of the test (limiter, cache, session can be swapped).
type dashProxy struct{ q *query.Dashboards }

func (p dashProxy) GetSchoolDashboard(ctx context.Context, in query.SchoolDashboardInput) (dto.SchoolDashboardView, error) {
	return p.q.GetSchoolDashboard(ctx, in)
}
func (p dashProxy) GetGradeDashboard(ctx context.Context, in query.GradeDashboardInput) (dto.GradeDashboardView, error) {
	return p.q.GetGradeDashboard(ctx, in)
}
func (p dashProxy) GetSchoolSeries(ctx context.Context, in query.SeriesInput) (dto.AnalyticsSeriesView, error) {
	return p.q.GetSchoolSeries(ctx, in)
}
func (p dashProxy) GetStudentProfile(ctx context.Context, in query.ProfileInput) (dto.StudentLearningProfileView, error) {
	return p.q.GetStudentProfile(ctx, in)
}
func (p dashProxy) GetTenantSummary(ctx context.Context, in query.TenantSummaryInput) (dto.LearningSummaryView, error) {
	return p.q.GetTenantSummary(ctx, in)
}
func (p dashProxy) GetPlatformSummary(ctx context.Context, in query.PlatformSummaryInput) (dto.LearningSummaryView, error) {
	return p.q.GetPlatformSummary(ctx, in)
}

// fakeEvidence is a TEST DOUBLE of write.EvidenceReader for the golden dataset (F043 T003): the facts and first answers of
// synthetic attempts, so that the counters are computed by hand without running the attempt engine. Never in a binary (R17).
type fakeEvidence struct {
	mu     sync.Mutex
	facts  map[uuid.UUID]write.AttemptFacts
	counts map[uuid.UUID]write.ItemCounts
}

func (f *fakeEvidence) Attempt(_ context.Context, id uuid.UUID) (write.AttemptFacts, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.facts[id], nil
}
func (f *fakeEvidence) AttemptObjectives(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeEvidence) ObjectiveEvidence(context.Context, uuid.UUID, uuid.UUID) ([]write.EvidenceRow, error) {
	return nil, nil
}
func (f *fakeEvidence) FirstAnswers(_ context.Context, id uuid.UUID) (write.ItemCounts, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[id], nil
}

// versionsSpy is a TEST DOUBLE of command.DashboardVersions counting the bumps per tenant.
type versionsSpy struct {
	mu sync.Mutex
	n  map[uuid.UUID]int
}

func (v *versionsSpy) Bump(_ context.Context, t uuid.UUID) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.n[t]++
	return nil
}

func (v *versionsSpy) count(t uuid.UUID) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.n[t]
}

// dashNow is the fixed clock of the dashboard tests: 2026-12-10 12:00 ICT (partitions of progress_daily exist from 2026-11).
var dashNow = time.Date(2026, 12, 10, 12, 0, 0, 0, analytics.Zone)

func ict(y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, analytics.Zone)
}

func date(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

// dashKit is the golden dataset of the dashboard tests (ADR-0095: students belong to a school and a grade, no class): tenant A
// with school S (grade 3: a1, a2, b1 active plus a3, b2 without activity — 5 students, visible) and school S2 (grade 4: c1
// alone — hidden by analytics_min_cell_size), tenant B with school SB (grade 3: x1); subjects M (math) and V (Vietnamese);
// the progress consumer (with rollups) on a real JetStream; the jobs with the fixed clock.
type dashKit struct {
	c                          *curEnv
	js                         jetstream.JetStream
	ev                         *fakeEvidence
	prog                       command.Progress
	jobs                       command.Dashboards
	versions                   *versionsSpy
	school, school2, schoolB   uuid.UUID
	a1, a2, b1, c1, x1, a3, b2 uuid.UUID
	subjM, subjV               uuid.UUID
	objectives                 []uuid.UUID // O1..O3 of math, grade 3
}

const (
	adS    = "dash.ad.s"  // school_admin of school S (tenant A)
	adS2   = "dash.ad.s2" // school_admin of school S2 (tenant A)
	amM    = "dash.am.m"  // academic_manager of school S limited to subject M
	taDash = "dash.ta.a"  // tenant_admin of tenant A
	adSB   = "dash.ad.sb" // school_admin of school SB (tenant B)
	smMod  = "dash.sm"    // safety_moderator (platform)
	saDsh  = "dash.sa"    // system_admin
	hsA1   = "dash.hs.a1" // student a1 (game token)
)

func (c *curEnv) dashKit(ctx context.Context, t *testing.T) *dashKit {
	t.Helper()
	c.clk.mu.Lock()
	c.clk.t = dashNow.UTC()
	c.clk.mu.Unlock()
	k := &dashKit{c: c, js: jetStream(ctx, t), ev: &fakeEvidence{facts: map[uuid.UUID]write.AttemptFacts{}, counts: map[uuid.UUID]write.ItemCounts{}},
		versions: &versionsSpy{n: map[uuid.UUID]int{}},
		school:   uuid.New(), school2: uuid.New(), schoolB: uuid.New(),
		a1: uuid.New(), a2: uuid.New(), b1: uuid.New(), c1: uuid.New(), x1: uuid.New(), a3: uuid.New(), b2: uuid.New()}
	repo := writerepo.MasteryRepo{}
	k.prog = command.Progress{Store: repo, Evidence: k.ev, Lessons: writerepo.LessonRepo{}, Rollups: writerepo.RollupRepo{},
		LateWindow: analytics.DefaultConfig().LateWindow, Late: observability.DashboardCounter{}, Clock: c.clk.Now}
	k.jobs = command.Dashboards{UoW: c.uow, Jobs: writerepo.RollupRepo{}, Versions: k.versions, Config: analytics.DefaultConfig(),
		Metrics: observability.DashboardCounter{}, Clock: c.clk.Now}
	o := consumer.ProgressDailyOptions(consumer.ProgressDaily{Record: k.prog}, c.uow, c.app, k.js)
	o.BackOff = []time.Duration{300 * time.Millisecond}
	cons, err := outbox.NewConsumer(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = cons.Run(runCtx) }()
	t.Cleanup(func() { cancel(); <-done })

	// Reference projections (identity → learning, F020) as learning_platform.
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := c.platform.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for _, s := range []struct{ id, tenant uuid.UUID }{{k.school, c.tenantA}, {k.school2, c.tenantA}, {k.schoolB, c.tenantB}} {
		exec(`INSERT INTO ref.schools (school_id, tenant_id, status, source_version) VALUES ($1, $2, 'active', 1)`, s.id, s.tenant)
	}
	for _, st := range []struct {
		id, tenant, school uuid.UUID
		grade              int
	}{{k.a1, c.tenantA, k.school, 3}, {k.a2, c.tenantA, k.school, 3}, {k.b1, c.tenantA, k.school, 3}, {k.a3, c.tenantA, k.school, 3},
		{k.b2, c.tenantA, k.school, 3}, {k.c1, c.tenantA, k.school2, 4}, {k.x1, c.tenantB, k.schoolB, 3}} {
		exec(`INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version) VALUES ($1, $2, $3, $4, 1)`,
			st.id, st.tenant, st.school, st.grade)
	}
	// Curriculum: platform math with three grade-3 objectives, assigned to tenants A and B.
	k.subjM, k.subjV = uuid.MustParse(c.subject(t, "dash_math")), uuid.MustParse(c.subject(t, "dash_viet"))
	cur, ver := c.curriculum(t, cmNoidung, "DASH_MATH")
	topic := c.topic(t, cmNoidung, ver, k.subjM.String(), "DMATH.G3.T01", 3, "")
	for _, code := range []string{"DMATH.G3.NUM.ADD", "DMATH.G3.NUM.SUB", "DMATH.G3.NUM.MUL"} {
		r := must(t, c.objective(t, cmNoidung, topic, code, nil), 201, "objective "+code)
		k.objectives = append(k.objectives, uuid.MustParse(r.str("objective_id")))
	}
	c.assign(ctx, t, cur, c.tenantA)
	c.assign(ctx, t, cur, c.tenantB)

	c.user(adS, authztest.Staff(c.tenantA, "school_admin", authztest.AtSchool(k.school)).PrincipalData, true)
	c.user(adS2, authztest.Staff(c.tenantA, "school_admin", authztest.AtSchool(k.school2)).PrincipalData, true)
	c.user(amM, authztest.Staff(c.tenantA, "academic_manager", authztest.AtSchool(k.school), authztest.ForSubject(k.subjM)).PrincipalData, true)
	c.user(taDash, authztest.Staff(c.tenantA, "tenant_admin").PrincipalData, true)
	c.user(adSB, authztest.Staff(c.tenantB, "school_admin", authztest.AtSchool(k.schoolB)).PrincipalData, true)
	c.user(smMod, authztest.Platform("safety_moderator").PrincipalData, true)
	c.user(saDsh, authztest.Platform("system_admin").PrincipalData, true)
	st := authztest.Student(c.tenantA).PrincipalData
	c.user(hsA1, st, false)
	return k
}

// attempt registers a synthetic graded attempt of a student (golden dataset) and returns its id.
func (k *dashKit) attempt(student, school, subject uuid.UUID, at time.Time, mode string, durSec int64, n write.ItemCounts) uuid.UUID {
	id := uuid.New()
	k.ev.mu.Lock()
	defer k.ev.mu.Unlock()
	k.ev.facts[id] = write.AttemptFacts{AttemptID: id, StudentID: student, SchoolID: school, Mode: mode, SubjectID: subject, GradedAt: at.UTC(),
		DurationMS: durSec * 1000, StartedAt: at.UTC().Add(-time.Duration(durSec) * time.Second)}
	k.ev.counts[id] = n
	return id
}

// graded builds the activity.graded envelope of an attempt of tenant (event id fixed by the caller to replay it).
func (k *dashKit) graded(t *testing.T, tenant, attempt, eventID uuid.UUID, at time.Time) []byte {
	t.Helper()
	te := tenant
	b, err := json.Marshal(outbox.Envelope{EventID: eventID, EventType: "activity.graded", SchemaVersion: 1, OccurredAt: at.UTC(),
		RecordedAt: at.UTC(), Producer: "learning", TenantID: &te, IdempotencyKey: "activity.graded:" + attempt.String(),
		Payload: json.RawMessage(`{"attempt_id":"` + attempt.String() + `"}`)})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// deliver publishes an envelope and waits until learning.progress_daily recorded it.
func (k *dashKit) deliver(ctx context.Context, t *testing.T, raw []byte) {
	t.Helper()
	var env outbox.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if _, err := k.js.Publish(ctx, natsx.EventSubject("learning", env.EventType), raw); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 30*time.Second, "progress_daily processed", func() bool {
		return count(ctx, t, k.c.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = 'learning.progress_daily'
			AND event_id = $1`, env.EventID) == 1
	})
}

// run delivers one graded attempt (new event id).
func (k *dashKit) run(ctx context.Context, t *testing.T, tenant, student, school, subject uuid.UUID, at time.Time, mode string, durSec int64,
	n write.ItemCounts) []byte {
	t.Helper()
	raw := k.graded(t, tenant, k.attempt(student, school, subject, at, mode, durSec, n), uuid.New(), at)
	k.deliver(ctx, t, raw)
	return raw
}

// golden delivers the golden dataset (days 2026-12-07 and 2026-12-08 of tenant A, 2026-12-07 of tenant B).
func (k *dashKit) golden(ctx context.Context, t *testing.T) {
	t.Helper()
	c := k.c
	d1, d2 := ict(2026, 12, 7, 9), ict(2026, 12, 8, 15)
	k.run(ctx, t, c.tenantA, k.a1, k.school, k.subjM, d1, "practice", 600, write.ItemCounts{Answered: 10, Correct: 8, RapidGuess: 2, CorrectRapid: 1})
	k.run(ctx, t, c.tenantA, k.a1, k.school, k.subjM, d1.Add(time.Hour), "practice", 300, write.ItemCounts{Answered: 5, Correct: 5})
	k.run(ctx, t, c.tenantA, k.a2, k.school, k.subjM, d1, "assignment", 60, write.ItemCounts{Answered: 4, Correct: 1, RapidGuess: 3, CorrectRapid: 1})
	k.run(ctx, t, c.tenantA, k.b1, k.school, k.subjV, d1, "practice", 120, write.ItemCounts{Answered: 6, Correct: 3})
	k.run(ctx, t, c.tenantA, k.a2, k.school, k.subjV, d2, "practice", 240, write.ItemCounts{Answered: 8, Correct: 6, RapidGuess: 1})
	k.run(ctx, t, c.tenantA, k.b1, k.school, k.subjM, d2, "exam", 900, write.ItemCounts{Answered: 10, Correct: 7})
	k.run(ctx, t, c.tenantA, k.c1, k.school2, k.subjM, d1, "practice", 180, write.ItemCounts{Answered: 3, Correct: 3})
	k.run(ctx, t, c.tenantB, k.x1, k.schoolB, k.subjM, d1, "practice", 100, write.ItemCounts{Answered: 2, Correct: 1})
}

// mastery seeds an objective mastery of a student (state read model source, F031).
func (k *dashKit) mastery(ctx context.Context, t *testing.T, tenant, school, student, objective uuid.UUID, level string, value float64) {
	t.Helper()
	if _, err := k.c.platform.ExecContext(ctx, `INSERT INTO learning.objective_masteries (objective_mastery_id, tenant_id, school_id, student_id,
			objective_id, value, level, evidence_count, model_version) VALUES ($1, $2, $3, $4, $5, $6, $7, 3, 'we-1.0')`,
		uuid.New(), tenant, school, student, objective, value, level); err != nil {
		t.Fatal(err)
	}
}

// assignment seeds a school assignment of subject with, per student, the grade when targeted and the submission status (F041
// state read model, ADR-0095: assignment_target_students.grade_level).
func (k *dashKit) assignment(ctx context.Context, t *testing.T, tenant, school, subject uuid.UUID, status string, due time.Time,
	subs map[uuid.UUID][2]any) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := k.c.platform.ExecContext(ctx, `INSERT INTO lms.assignments (assignment_id, tenant_id, school_id, subject_id, created_by,
			created_by_role, title, content_type, content_id, follow_latest, mode, open_at, due_at, feedback_timing, result_release, status)
		VALUES ($1, $2, $3, $4, $5, 'school_admin', $10, 'activity', $6, true, 'practice', $7, $8,
			'immediate', 'immediate', $9)`, id, tenant, school, subject, uuid.New(), uuid.New(), due.Add(-72*time.Hour), due,
		status, "Bài giao "+id.String()[:8]); err != nil {
		t.Fatal(err)
	}
	for student, v := range subs {
		if _, err := k.c.platform.ExecContext(ctx, `INSERT INTO lms.assignment_target_students (assignment_target_student_id, tenant_id,
				school_id, assignment_id, student_id, grade_level, source) VALUES ($1, $2, $3, $4, $5, $6, 'publish')`, uuid.New(), tenant, school, id,
			student, v[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := k.c.platform.ExecContext(ctx, `INSERT INTO lms.assignment_submissions (assignment_submission_id, tenant_id, school_id,
				assignment_id, student_id, status) VALUES ($1, $2, $3, $4, $5, $6)`, uuid.New(), tenant, school, id, student, v[1]); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// principalCtx is the request context of a demo user (principal and scope of the httpx middleware).
func (k *dashKit) principalCtx(ctx context.Context, sub string) context.Context {
	return authztest.Ctx(ctx, authz.NewPrincipal(k.c.users[sub]))
}
