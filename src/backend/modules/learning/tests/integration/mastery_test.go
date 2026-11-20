//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/qbschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// masteryKit runs the consumers learning.mastery_on_graded and learning.progress_daily on a real NATS JetStream and
// delivers the activity.graded envelopes the attempt engine wrote into the outbox (no relay process in the tests).
type masteryKit struct {
	c       *curEnv
	js      jetstream.JetStream
	mastery command.Mastery
}

func (c *curEnv) masteryKit(ctx context.Context, t *testing.T) *masteryKit {
	t.Helper()
	js := jetStream(ctx, t)
	repo := write.MasteryRepo{}
	k := &masteryKit{c: c, js: js, mastery: command.Mastery{Store: repo, Evidence: repo, Learning: write.LearningEventWriter{},
		Events: write.Publisher{W: c.writer}, Clock: c.clk.Now, Metrics: observability.MasteryCounter{}}}
	for _, o := range []outbox.ConsumerOptions{
		consumer.MasteryOptions(consumer.MasteryOnGraded{Apply: k.mastery}, c.uow, c.app, js),
		consumer.ProgressDailyOptions(consumer.ProgressDaily{Record: command.Progress{Store: repo, Evidence: repo}}, c.uow, c.app, js),
	} {
		o.BackOff = []time.Duration{300 * time.Millisecond}
		cons, err := outbox.NewConsumer(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() { defer close(done); _ = cons.Run(runCtx) }()
		t.Cleanup(func() { cancel(); <-done })
	}
	return k
}

// graded returns the outbox envelope of activity.graded of an attempt.
func (k *masteryKit) graded(ctx context.Context, t *testing.T, attemptID string) []byte {
	t.Helper()
	var raw []byte
	if err := k.c.migrator.QueryRowContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type = 'activity.graded'
		AND envelope->'payload'->>'attempt_id' = $1`, attemptID).Scan(&raw); err != nil {
		t.Fatalf("activity.graded of %s: %v", attemptID, err)
	}
	return raw
}

// deliver publishes an envelope and waits until both consumers recorded it.
func (k *masteryKit) deliver(ctx context.Context, t *testing.T, raw []byte) uuid.UUID {
	t.Helper()
	var env outbox.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if _, err := k.js.Publish(ctx, natsx.EventSubject("learning", env.EventType), raw); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"learning.mastery_on_graded", "learning.progress_daily"} {
		waitFor(t, 30*time.Second, name+" processed", func() bool {
			return count(ctx, t, k.c.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = $1 AND event_id = $2`,
				name, env.EventID) == 1
		})
	}
	return env.EventID
}

// quizRun starts the quiz as sub, answers its three questions correctly and submits; returns the attempt id.
func (c *curEnv) quizRun(t *testing.T, sub, activityID string) string {
	t.Helper()
	id := must(t, c.start(t, sub, activityID, nil, nil), http.StatusCreated, "start").str("attempt_id")
	right, _ := correctOption()
	for pos := 1; pos <= 3; pos++ {
		must(t, c.answer(t, sub, id, pos, map[string]any{"option_id": right}, nil), http.StatusOK, "answer")
	}
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", sub, idem(), nil), http.StatusOK, "submit")
	return id
}

type masteryRow struct {
	value          float64
	level, model   string
	evidence, days int
	firstMastered  sql.NullTime
	lastEvidence   sql.NullTime
}

func (c *curEnv) masteryOf(ctx context.Context, t *testing.T, student uuid.UUID, objective string) masteryRow {
	t.Helper()
	var m masteryRow
	if err := c.platform.QueryRowContext(ctx, `SELECT value::float8, level, model_version, evidence_count, distinct_days, first_mastered_at,
		last_evidence_at FROM learning.objective_masteries WHERE student_id = $1 AND objective_id = $2 AND deleted_at IS NULL`, student, objective).
		Scan(&m.value, &m.level, &m.model, &m.evidence, &m.days, &m.firstMastered, &m.lastEvidence); err != nil {
		t.Fatalf("mastery row: %v", err)
	}
	return m
}

// validateStoredEvents rebuilds the envelope of every learning event of an attempt and checks it against the schema
// (F031-AC-01).
func (c *curEnv) validateStoredEvents(ctx context.Context, t *testing.T, attemptID string) map[string]int {
	t.Helper()
	rows, err := c.platform.QueryContext(ctx, `SELECT event_id, occurred_at, event_type, student_id, tenant_id, school_id, class_id,
		game_session_id, activity_id, content_id, content_version_id, array_to_string(objective_ids, ','), context, actor_type, payload,
		idempotency_key FROM learning.learning_events WHERE payload->>'attempt_id' = $1`, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	types := map[string]int{}
	for rows.Next() {
		var e portwrite.LearningEvent
		var class, session, act, content, cv uuid.NullUUID
		var objectives string
		var payload []byte
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.Type, &e.StudentID, &e.TenantID, &e.SchoolID, &class, &session, &act, &content, &cv,
			&objectives, &e.Context, &e.ActorType, &payload, &e.IdempotencyKey); err != nil {
			t.Fatal(err)
		}
		for _, p := range []struct {
			n   uuid.NullUUID
			dst **uuid.UUID
		}{{class, &e.ClassID}, {session, &e.GameSessionID}, {act, &e.ActivityID}, {content, &e.ContentID}, {cv, &e.ContentVersionID}} {
			if p.n.Valid {
				id := p.n.UUID
				*p.dst = &id
			}
		}
		for _, o := range strings.Split(objectives, ",") {
			if id, err := uuid.Parse(o); err == nil {
				e.ObjectiveIDs = append(e.ObjectiveIDs, id)
			}
		}
		if e.ID.Version() != 7 {
			t.Errorf("%s: event_id is not a UUIDv7", e.Type)
		}
		doc, _ := json.Marshal(write.Envelope(e, payload, time.Now()))
		if err := qbschema.ValidateLearningEvent(doc); err != nil {
			t.Errorf("stored %s: %v", e.Type, err)
		}
		types[e.Type]++
	}
	return types
}

func near3(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0005 {
		t.Fatalf("%s = %.4f, want %.4f", what, got, want)
	}
}

// F031-AC-01, AC-02, AC-08, AC-12, AC-13, AC-15, AC-17 (quickstart 1, 2, 8, 10–12, 15; TC-LRN-004, TC-LRN-010, TC-SVC-003):
// a graded quiz becomes evidence of its objective through the consumers; redelivery is processed once; a second day
// makes the objective mastered exactly once; getMyProgress shows the level after decay and the totals.
func TestMasteryOnGraded(t *testing.T) {
	authztest.Cover(t, "getMyProgress", authztest.CrossTenant, authztest.RoleDenied)
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	k := c.masteryKit(ctx, t)
	an := c.users[hsAn].UserID

	// Day 1: three correct answers (practice, difficulty 2, single choice with 4 options: w = 0.7·0.9·0.75).
	a1 := c.quizRun(t, hsAn, s.activityID)
	if types := c.validateStoredEvents(ctx, t, a1); types["activity.started"] != 1 || types["question.answered"] != 3 ||
		types["activity.submitted"] != 1 || types["activity.graded"] != 1 {
		t.Fatalf("learning events of the attempt: %v", types)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE payload->>'attempt_id' = $1
		AND (school_id IS NULL OR content_version_id IS NULL OR idempotency_key = '')`, a1); n != 0 {
		t.Fatalf("%d events without envelope fields", n)
	}
	g1 := k.graded(ctx, t, a1)
	ev1 := k.deliver(ctx, t, g1)
	m := c.masteryOf(ctx, t, an, s.qc.lo)
	near3(t, "M after day 1", m.value, 0.720)
	if m.evidence != 3 || m.days != 1 || m.model != "we-1.0" || m.level != "proficient" || m.firstMastered.Valid {
		t.Fatalf("day 1 mastery: %+v", m)
	}
	hist := `SELECT count(*) FROM learning.mastery_history WHERE student_id = $1`
	if n := count(ctx, t, c.platform, hist, an); n != 1 {
		t.Fatalf("history rows = %d", n)
	}
	progressedEvents := `SELECT count(*) FROM learning.learning_events WHERE event_type = 'learning_objective.progressed' AND student_id = $1`
	if n := count(ctx, t, c.platform, progressedEvents, an); n != 1 {
		t.Fatalf("learning_objective.progressed learning events = %d", n)
	}
	daily := `SELECT activities_completed, items_answered, items_correct FROM learning.progress_daily WHERE student_id = $1 AND subject_id = $2`
	var acts, answered, correct int
	if err := c.platform.QueryRowContext(ctx, daily, an, s.qc.subject).Scan(&acts, &answered, &correct); err != nil || acts != 1 ||
		answered != 3 || correct != 3 {
		t.Fatalf("progress_daily after day 1: %d %d %d (%v)", acts, answered, correct, err)
	}

	// F031-AC-02, AC-17: the same event delivered again is processed once: no new history, no double counters.
	if _, err := k.js.Publish(ctx, natsx.EventSubject("learning", "activity.graded"), g1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 30*time.Second, "duplicate ack", func() bool {
		return testutil.ToFloat64(outbox.ConsumedCounter().WithLabelValues("learning", "learning.progress_daily", "activity.graded",
			"duplicate")) >= 1
	})
	if n := count(ctx, t, c.migrator, `SELECT count(*) FROM infra.processed_events WHERE event_id = $1`, ev1); n != 2 {
		t.Fatalf("processed_events rows = %d, want one per consumer", n)
	}
	if err := c.platform.QueryRowContext(ctx, daily, an, s.qc.subject).Scan(&acts, &answered, &correct); err != nil || acts != 1 || answered != 3 {
		t.Fatalf("progress_daily doubled: %d %d", acts, answered)
	}
	if n := count(ctx, t, c.platform, hist, an); n != 1 {
		t.Fatalf("history rows after redelivery = %d", n)
	}

	// Day 2 (+25 h): three more → n = 6 on 2 days, M = 0.837 → mastered (F031-AC-10 needs the second day, AC-13 once).
	c.clk.Advance(25 * time.Hour)
	a2 := c.quizRun(t, hsAn, s.activityID)
	k.deliver(ctx, t, k.graded(ctx, t, a2))
	m = c.masteryOf(ctx, t, an, s.qc.lo)
	near3(t, "M after day 2", m.value, 0.837)
	if m.evidence != 6 || m.days != 2 || m.level != "mastered" || !m.firstMastered.Valid {
		t.Fatalf("day 2 mastery: %+v", m)
	}
	mastered := `SELECT count(*) FROM learning.learning_events WHERE event_type = 'learning_objective.mastered' AND student_id = $1`
	masteredOutbox := `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'learning_objective.mastered'
		AND envelope->'payload'->>'student_id' = $1`
	if n := count(ctx, t, c.platform, mastered, an); n != 1 {
		t.Fatalf("mastered learning events = %d", n)
	}
	if n := count(ctx, t, c.migrator, masteredOutbox, an.String()); n != 1 {
		t.Fatalf("mastered outbox events = %d", n)
	}
	// A recompute of the same pair (e.g. re-evaluation) never emits mastered again.
	scope, err := authz.EventScope(c.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.uow.InTenantTx(ctx, scope, func(ctx context.Context) error {
		return k.mastery.ApplyGraded(ctx, command.GradedAttempt{EventID: uuid.New(), TenantID: c.tenantA, AttemptID: uuid.MustParse(a2),
			OccurredAt: c.clk.Now()})
	}); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, c.platform, mastered, an); n != 1 {
		t.Fatalf("mastered re-emitted: %d", n)
	}
	if n := count(ctx, t, c.platform, progressedEvents, an); n != 2 {
		t.Fatalf("progressed learning events = %d, want 2", n)
	}

	// F031-AC-15: getMyProgress (own data only) — level, evidence, totals; F031-AC-11: after 104 days the level decays
	// (0.837 × 0.75 = 0.628 → proficient) while the stored value is unchanged.
	p := must(t, c.do(t, http.MethodGet, "/learning/progress/me", hsAn, nil, nil), http.StatusOK, "my progress")
	objs, _ := p.body["objectives"].([]any)
	if len(objs) != 1 || objs[0].(map[string]any)["level"] != "mastered" || objs[0].(map[string]any)["evidence_count"] != float64(6) {
		t.Fatalf("objectives: %s", p.raw)
	}
	totals := p.body["totals"].(map[string]any)
	if totals["items_answered"] != float64(6) || totals["accuracy"] != float64(1) {
		t.Fatalf("totals: %s", p.raw)
	}
	if strings.Contains(p.raw, `"value"`) {
		t.Fatal("stored value returned")
	}
	c.clk.Advance(104 * 24 * time.Hour)
	p = must(t, c.do(t, http.MethodGet, "/learning/progress/me", hsAn, nil, nil), http.StatusOK, "decayed progress")
	if lvl := p.body["objectives"].([]any)[0].(map[string]any)["level"]; lvl != "proficient" {
		t.Fatalf("decayed level %v", lvl)
	}
	near3(t, "stored M after decay read", c.masteryOf(ctx, t, an, s.qc.lo).value, 0.837)
	// Another student sees nothing of hs.an (own data only); a subject filter of another subject is empty.
	other := must(t, c.do(t, http.MethodGet, "/learning/progress/me", hsBinh, nil, nil), http.StatusOK, "other progress")
	if objs, _ := other.body["objectives"].([]any); len(objs) != 0 || other.body["student_id"] != c.users[hsBinh].UserID.String() {
		t.Fatalf("other student: %s", other.raw)
	}
	none := must(t, c.do(t, http.MethodGet, "/learning/progress/me?subject_id="+uuid.NewString(), hsAn, nil, nil), http.StatusOK, "subject")
	if objs, _ := none.body["objectives"].([]any); len(objs) != 0 {
		t.Fatalf("subject filter: %s", none.raw)
	}
	// Portal staff on the game route → 403.
	if r := c.do(t, http.MethodGet, "/learning/progress/me", qlcmA, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("staff on getMyProgress: %d", r.status)
	}
}

// F031-AC-16 (TC-TEN-002): getStudentProgress for the school admin of the student's school and the academic manager
// (tenant-wide) with audit learning.profile_viewed; another school → 404; tenant B → 404; subject scope → 403.
func TestStudentProgressAuthz(t *testing.T) {
	authztest.Cover(t, "getStudentProgress", authztest.CrossTenant, authztest.RoleDenied)
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	an := c.users[hsAn].UserID
	var school uuid.UUID
	if err := c.platform.QueryRowContext(ctx, `SELECT school_id FROM ref.students WHERE student_id = $1`, an).Scan(&school); err != nil {
		t.Fatal(err)
	}
	subject := uuid.MustParse(s.qc.subject)
	c.user("ad.cung.truong", authztest.Staff(c.tenantA, "school_admin", authztest.AtSchool(school)).PrincipalData, true)
	c.user("qlcm.mon.khac", authztest.Staff(c.tenantA, "academic_manager", authztest.ForSubject(uuid.New())).PrincipalData, true)
	c.user("qlcm.mon.toan", authztest.Staff(c.tenantA, "academic_manager", authztest.ForSubject(subject)).PrincipalData, true)
	path := "/students/" + an.String() + "/progress"
	audits := `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'learning.profile_viewed'`
	before := count(ctx, t, c.migrator, audits)
	r := must(t, c.do(t, http.MethodGet, path, "ad.cung.truong", nil, nil), http.StatusOK, "school admin")
	if r.body["student_id"] != an.String() || strings.Contains(r.raw, "school_id") {
		t.Fatalf("progress: %s", r.raw)
	}
	must(t, c.do(t, http.MethodGet, path, qlcmA, nil, nil), http.StatusOK, "academic manager")
	must(t, c.do(t, http.MethodGet, path+"?subject_id="+subject.String(), "qlcm.mon.toan", nil, nil), http.StatusOK, "subject manager")
	if n := count(ctx, t, c.migrator, audits); n != before+3 {
		t.Fatalf("learning.profile_viewed audits = %d, want %d", n, before+3)
	}
	for _, tc := range []struct {
		sub, query string
		want       int
	}{
		{adA, "", http.StatusNotFound},                                             // school admin of another school
		{adB, "", http.StatusNotFound},                                             // tenant B
		{qlcmB, "", http.StatusNotFound},                                           // tenant B
		{"qlcm.mon.khac", "?subject_id=" + subject.String(), http.StatusForbidden}, // subject outside the grant
		{"qlcm.mon.toan", "", http.StatusForbidden},                                // subject-limited grant without subject_id
		{noRoleA, "", http.StatusForbidden},                                        // no analytics.class.read
		{hsAn, "", http.StatusForbidden},                                           // student token on a portal route
	} {
		if got := c.do(t, http.MethodGet, path+tc.query, tc.sub, nil, nil); got.status != tc.want {
			t.Errorf("%s%s: %d, want %d (%s)", tc.sub, tc.query, got.status, tc.want, got.raw)
		}
	}
	if r := c.do(t, http.MethodGet, "/students/"+uuid.NewString()+"/progress", qlcmA, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown student: %d", r.status)
	}
	if n := count(ctx, t, c.migrator, audits); n != before+3 {
		t.Fatalf("denied reads were audited as profile views: %d", n)
	}
}

// F031-AC-03, AC-07 (TC-TEN-016, TC-TEN-005): learning events and mastery history are append-only for the application
// roles (also on their partitions) and every learning table returns no row without a tenant scope.
func TestLearningRecordAppendOnlyAndIsolated(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	k := c.masteryKit(ctx, t)
	a := c.quizRun(t, hsAn, s.activityID)
	k.deliver(ctx, t, k.graded(ctx, t, a))
	for _, db := range []struct {
		name string
		db   *sql.DB
	}{{"app", c.app}, {"platform", c.platform}} {
		for _, stmt := range []string{
			`UPDATE learning.learning_events SET payload = '{}'::jsonb`,
			`DELETE FROM learning.learning_events`,
			`UPDATE learning.learning_events_2026_11 SET payload = '{}'::jsonb`,
			`UPDATE learning.mastery_history SET value_after = 0`,
			`DELETE FROM learning.mastery_history`,
			`UPDATE assessment.attempt_answers SET score = 0`,
		} {
			if _, err := db.db.ExecContext(ctx, stmt); err == nil || !strings.Contains(err.Error(), "permission denied") {
				t.Errorf("%s: %q allowed: %v", db.name, stmt, err)
			}
		}
	}
	for _, table := range []string{"learning.learning_events", "learning.objective_masteries", "learning.mastery_history", "learning.progress_daily"} {
		if n := count(ctx, t, c.app, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("%s without scope: %d rows", table, n)
		}
		if n := count(ctx, t, c.platform, `SELECT count(*) FROM `+table); n == 0 {
			t.Errorf("%s has no row at all", table)
		}
	}
	// The read model of tenant B sees nothing of tenant A.
	scopeB, err := authz.EventScope(c.tenantB)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.uow.InTenantTx(ctx, scopeB, func(ctx context.Context) error {
		row, err := write.MasteryRepo{}.Mastery(ctx, c.users[hsAn].UserID, uuid.MustParse(s.qc.lo))
		if err != nil || row != nil {
			t.Errorf("tenant B reads mastery of tenant A: %v %v", row, err)
		}
		rows, err := write.MasteryRepo{}.ObjectiveEvidence(ctx, c.users[hsAn].UserID, uuid.MustParse(s.qc.lo))
		if err != nil || len(rows) != 0 {
			t.Errorf("tenant B reads evidence of tenant A: %d %v", len(rows), err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// F031-AC-14 (TC-SVC-003): two graded attempts of the same student applied concurrently give the result of sequential
// processing (optimistic row version, retried replay).
func TestMasteryConcurrentEvidence(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	k := c.masteryKit(ctx, t)
	a1 := c.quizRun(t, hsBinh, s.activityID)
	c.clk.Advance(25 * time.Hour)
	a2 := c.quizRun(t, hsBinh, s.activityID)
	scope, err := authz.EventScope(c.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, id := range []string{a1, a2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = c.uow.InTenantTx(ctx, scope, func(ctx context.Context) error {
				return k.mastery.ApplyGraded(ctx, command.GradedAttempt{EventID: uuid.New(), TenantID: c.tenantA, AttemptID: uuid.MustParse(id),
					OccurredAt: c.clk.Now()})
			})
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	m := c.masteryOf(ctx, t, c.users[hsBinh].UserID, s.qc.lo)
	near3(t, "concurrent M", m.value, 0.837)
	if m.evidence != 6 || m.days != 2 || m.level != "mastered" {
		t.Fatalf("concurrent mastery: %+v", m)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = 'learning_objective.mastered'
		AND student_id = $1`, c.users[hsBinh].UserID); n != 1 {
		t.Fatalf("mastered events = %d", n)
	}
}

// F031-AC-04: the partition job (role learning_partition_admin) keeps the current month and three more of every
// partitioned learning table and exposes the months ahead.
func TestLearningPartitionsJob(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	admin := open(t, c.cl.ModuleDSN("learning", "partition_admin"))
	at := time.Date(2027, 11, 15, 3, 0, 0, 0, time.UTC) // the migration created partitions through 2027-12
	job := command.LearningPartitions{Maint: write.PartitionMaint{DB: admin}, Metrics: observability.MasteryCounter{},
		Clock: func() time.Time { return at }}
	n, err := job.RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 { // 2028-01, 2028-02 of four tables
		t.Fatalf("created %d partitions", n)
	}
	for _, table := range portwrite.PartitionedTables {
		if got := testutil.ToFloat64(observability.PartitionMonthsAhead().WithLabelValues(table)); got != 3 {
			t.Errorf("%s months ahead = %v", table, got)
		}
	}
	if again, err := job.RunOnce(ctx); err != nil || again != 0 {
		t.Fatalf("second run: %d %v", again, err)
	}
	// New partitions carry no direct grant: the application roles go through the parent only.
	if _, err := c.app.ExecContext(ctx, `UPDATE learning.learning_events_2028_02 SET payload = '{}'::jsonb`); err == nil ||
		!strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("direct partition access: %v", err)
	}
	// The admin role has nothing but EXECUTE.
	if _, err := admin.ExecContext(ctx, `SELECT count(*) FROM learning.learning_events`); err == nil {
		t.Fatal("partition admin reads learning events")
	}
}
