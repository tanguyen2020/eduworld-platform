//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// lessonKit runs learning.lesson_completion and learning.progress_daily (with lessons) on a real NATS JetStream.
type lessonKit struct {
	c  *curEnv
	js jetstream.JetStream
}

func (c *curEnv) lessonKit(ctx context.Context, t *testing.T) *lessonKit {
	t.Helper()
	js := jetStream(ctx, t)
	repo := write.MasteryRepo{}
	for _, o := range []outbox.ConsumerOptions{
		consumer.LessonCompletionOptions(consumer.LessonCompletion{Lessons: c.lessons}, c.uow, c.app, js),
		consumer.ProgressDailyOptions(consumer.ProgressDaily{Record: command.Progress{Store: repo, Evidence: repo, Lessons: write.LessonRepo{}}},
			c.uow, c.app, js),
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
	return &lessonKit{c: c, js: js}
}

// deliver publishes the outbox envelope of eventType whose payload key matches value and waits for the consumers.
func (k *lessonKit) deliver(ctx context.Context, t *testing.T, eventType, key, value string, consumers ...string) []byte {
	t.Helper()
	var raw []byte
	if err := k.c.migrator.QueryRowContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type = $1
		AND envelope->'payload'->>`+key+` = $2 ORDER BY created_at DESC LIMIT 1`, eventType, value).Scan(&raw); err != nil {
		t.Fatalf("%s %s: %v", eventType, value, err)
	}
	var env outbox.Envelope
	_ = json.Unmarshal(raw, &env)
	if _, err := k.js.Publish(ctx, natsx.EventSubject("learning", eventType), raw); err != nil {
		t.Fatal(err)
	}
	for _, name := range consumers {
		waitFor(t, 30*time.Second, name+" processed", func() bool {
			return count(ctx, t, k.c.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = $1 AND event_id = $2`,
				name, env.EventID) == 1
		})
	}
	return raw
}

// quizOf creates and publishes a quiz of qlcmA over the given question versions.
func (c *curEnv) quizOf(ctx context.Context, t *testing.T, s attSetup, qvs ...string) string {
	t.Helper()
	id := c.createActivity(t, qlcmA, activityBody(s.qc, "quiz", quizConfig(qvs...)))
	c.publishActivity(ctx, t, id, 1)
	return id
}

// lessonOf creates a lesson of qlcmA with the activity blocks (required unless listed in optional) and publishes it.
func (c *curEnv) lessonOf(ctx context.Context, t *testing.T, s attSetup, rule string, publish bool, optional string, activities ...string) string {
	t.Helper()
	blocks := []any{map[string]any{"kind": "rich_text", "content": rich("Ôn bảng nhân")}}
	for _, a := range activities {
		b := map[string]any{"kind": "activity", "activity_id": a}
		if a == optional {
			b["required"] = false
		}
		blocks = append(blocks, b)
	}
	body := lessonBody(s.qc, blocks...)
	body["completion_rule"], body["grade_level"] = rule, 4
	id := must(t, c.do(t, http.MethodPost, lessonsPath, qlcmA, idem(), body), http.StatusCreated, "lesson").str("content_id")
	if publish {
		c.approve(ctx, t, qlcmA, "lesson", id, 1)
		must(t, c.publishV(ctx, t, qlcmA, "lesson", id, 1, nil, nil), http.StatusOK, "publish lesson")
	}
	return id
}

// lessonRun starts an activity in the context of a lesson, answers its n questions correctly and submits.
func (c *curEnv) lessonRun(t *testing.T, sub, activityID, lessonID string, n int) string {
	t.Helper()
	id := must(t, c.start(t, sub, activityID, nil, map[string]any{"context_type": "lesson", "context_id": lessonID}), http.StatusCreated,
		"start in lesson").str("attempt_id")
	right, _ := correctOption()
	for pos := 1; pos <= n; pos++ {
		must(t, c.answer(t, sub, id, pos, map[string]any{"option_id": right}, nil), http.StatusOK, "answer")
	}
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", sub, idem(), nil), http.StatusOK, "submit")
	return id
}

// F031-AC-25…AC-30 (quickstart 22–27; TC-SVC-003, TC-TEN-001): the lesson session is pinned at the first attempt in the
// lesson context; completion of the last required activity completes the session once per version with lesson.completed
// (learning event + outbox), progress_daily counts it once; completeLesson; authorization and tenant isolation.
func TestLessonCompletion(t *testing.T) {
	authztest.Cover(t, "getLesson", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "completeLesson", authztest.CrossTenant, authztest.RoleDenied)
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	k := c.lessonKit(ctx, t)
	act1 := s.activityID // 3 questions
	act2 := c.quizOf(ctx, t, s, s.questions[0])
	opt := c.quizOf(ctx, t, s, s.questions[1])
	other := c.quizOf(ctx, t, s, s.questions[2])
	lessonID := c.lessonOf(ctx, t, s, "all_required_activities", true, opt, act1, act2, opt)
	draftOnly := c.lessonOf(ctx, t, s, "any_activity", false, "", act1)
	path := "/learning/lessons/" + lessonID

	// Before any session: released version, nothing completed; completeLesson → 409.
	g := must(t, c.do(t, http.MethodGet, path, hsAn, nil, nil), http.StatusOK, "lesson")
	if g.body["version"] != float64(1) || len(g.items("activities")) != 3 || g.body["completed_at"] != nil || strings.Contains(g.raw, "answer_key") {
		t.Fatalf("lesson: %s", g.raw)
	}
	if r := c.do(t, http.MethodPost, path+":complete", hsAn, idem(), nil); r.status != http.StatusConflict {
		t.Fatalf("complete without session: %d %s", r.status, r.raw)
	}
	// F031-AC-29: staff 403, other tenant session header 403 TENANT_MISMATCH, tenant B and unpublished lesson 404.
	if r := c.do(t, http.MethodGet, path, qlcmA, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("staff: %d", r.status)
	}
	if r := c.do(t, http.MethodGet, path, hsAn, hdr{"X-Tenant-Id": c.tenantB.String()}, nil); r.status != http.StatusForbidden ||
		r.str("code") != "TENANT_MISMATCH" {
		t.Fatalf("tenant mismatch: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodGet, path, hsDung, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d", r.status)
	}
	if r := c.do(t, http.MethodGet, "/learning/lessons/"+draftOnly, hsAn, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("unpublished lesson: %d", r.status)
	}

	// F031-AC-25: the first attempt in the lesson context pins version 1; a new version does not change the session; an
	// activity outside the pinned version is 422.
	a1 := c.lessonRun(t, hsAn, act1, lessonID, 3)
	an := c.users[hsAn].UserID
	sessions := `SELECT count(*) FROM learning.lesson_sessions WHERE student_id = $1 AND lesson_id = $2 AND status = $3 AND lesson_version = $4`
	if n := count(ctx, t, c.platform, sessions, an, lessonID, "in_progress", 1); n != 1 {
		t.Fatalf("open sessions v1 = %d", n)
	}
	must(t, c.do(t, http.MethodPost, lessonsPath+"/"+lessonID+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}), http.StatusCreated, "v2")
	c.approve(ctx, t, qlcmA, "lesson", lessonID, 2)
	must(t, c.publishV(ctx, t, qlcmA, "lesson", lessonID, 2, nil, nil), http.StatusOK, "publish v2")
	if r := c.do(t, http.MethodPost, "/learning/activities/"+other+"/attempts", hsAn, idem(),
		map[string]any{"context_type": "lesson", "context_id": lessonID}); r.status != http.StatusUnprocessableEntity || r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("activity outside the lesson: %d %s", r.status, r.raw)
	}
	k.deliver(ctx, t, "activity.graded", "'attempt_id'", a1, "learning.lesson_completion")
	g = must(t, c.do(t, http.MethodGet, path, hsAn, nil, nil), http.StatusOK, "pinned lesson")
	if g.body["version"] != float64(1) {
		t.Fatalf("pinned version: %s", g.raw)
	}
	completed := map[string]bool{}
	for _, it := range g.items("activities") {
		m := it
		completed[m["activity_id"].(string)] = m["completed"] == true
	}
	if !completed[act1] || completed[act2] || completed[opt] {
		t.Fatalf("completed flags: %v", completed)
	}
	// F031-AC-28: not satisfied yet → 409 with the missing required activity; client fields ignored.
	r := c.do(t, http.MethodPost, path+":complete", hsAn, idem(), map[string]any{"duration_ms": 999999, "score": 10})
	if r.status != http.StatusConflict || !slices.Contains(r.errorCodes(), "/activities/"+act2+":NOT_COMPLETED") || len(r.errorCodes()) != 1 {
		t.Fatalf("not satisfied: %d %v %s", r.status, r.errorCodes(), r.raw)
	}

	// F031-AC-26: the last required activity completes the session (one learning event + one outbox lesson.completed).
	a2 := c.lessonRun(t, hsAn, act2, lessonID, 1)
	k.deliver(ctx, t, "activity.graded", "'attempt_id'", a2, "learning.lesson_completion")
	if n := count(ctx, t, c.platform, sessions, an, lessonID, "completed", 1); n != 1 {
		t.Fatal("session not completed")
	}
	events := `SELECT count(*) FROM learning.learning_events WHERE event_type = 'lesson.completed' AND student_id = $1`
	outboxes := `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'lesson.completed' AND envelope->'payload'->>'student_id' = $1`
	if count(ctx, t, c.platform, events, an) != 1 || count(ctx, t, c.migrator, outboxes, an.String()) != 1 {
		t.Fatal("lesson.completed not written once")
	}
	var payload map[string]any
	var raw []byte
	if err := c.migrator.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'lesson.completed'
		AND envelope->'payload'->>'student_id' = $1`, an.String()).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &payload)
	for _, key := range []string{"lesson_session_id", "lesson_id", "lesson_version_id", "lesson_version", "student_id", "completion_rule", "mode",
		"required_activity_count", "completed_activity_count", "attempt_ids", "objective_ids", "grade_levels", "started_at", "completed_at", "duration_ms"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("lesson.completed without %s", key)
		}
	}
	for key := range payload {
		if strings.Contains(key, "name") || strings.Contains(key, "email") || strings.Contains(key, "username") {
			t.Errorf("PII key %s", key)
		}
	}
	if payload["lesson_version"] != float64(1) || payload["required_activity_count"] != float64(2) || payload["completed_activity_count"] != float64(2) {
		t.Fatalf("payload: %v", payload)
	}
	// F031-AC-30: progress_daily lessons_completed +1, once.
	lessonRaw := k.deliver(ctx, t, "lesson.completed", "'student_id'", an.String(), "learning.progress_daily")
	daily := `SELECT COALESCE(sum(lessons_completed), 0) FROM learning.progress_daily WHERE student_id = $1`
	if n := count(ctx, t, c.platform, daily, an); n != 1 {
		t.Fatalf("lessons_completed = %d", n)
	}
	if _, err := k.js.Publish(ctx, natsx.EventSubject("learning", "lesson.completed"), lessonRaw); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if n := count(ctx, t, c.platform, daily, an); n != 1 {
		t.Fatalf("lessons_completed after redelivery = %d", n)
	}

	// F031-AC-27, AC-28: completeLesson twice → same completed_at; redelivered activity.graded → no second event.
	c1 := must(t, c.do(t, http.MethodPost, path+":complete", hsAn, idem(), nil), http.StatusOK, "complete 1")
	c2 := must(t, c.do(t, http.MethodPost, path+":complete", hsAn, idem(), nil), http.StatusOK, "complete 2")
	if c1.str("completed_at") == "" || c1.str("completed_at") != c2.str("completed_at") {
		t.Fatalf("completed_at: %s / %s", c1.raw, c2.raw)
	}
	k.deliver(ctx, t, "activity.graded", "'attempt_id'", a2, "learning.lesson_completion")
	if count(ctx, t, c.platform, events, an) != 1 {
		t.Fatal("lesson.completed emitted twice")
	}

	// F031-AC-27: reopening the lesson pins the new version 2 in a new session; completing it emits a second event.
	c.clk.Advance(2 * time.Minute)
	c.lessonRun(t, hsAn, opt, lessonID, 1)
	if n := count(ctx, t, c.platform, sessions, an, lessonID, "in_progress", 2); n != 1 {
		t.Fatal("no session for version 2")
	}
	if g := must(t, c.do(t, http.MethodGet, path, hsAn, nil, nil), http.StatusOK, "v2"); g.body["version"] != float64(2) || g.body["completed_at"] != nil {
		t.Fatalf("lesson v2: %s", g.raw)
	}
	for _, act := range []struct {
		id string
		n  int
	}{{act1, 3}, {act2, 1}} {
		a := c.lessonRun(t, hsAn, act.id, lessonID, act.n)
		k.deliver(ctx, t, "activity.graded", "'attempt_id'", a, "learning.lesson_completion")
	}
	if count(ctx, t, c.platform, events, an) != 2 {
		t.Fatal("version 2 not completed once")
	}
	if n := count(ctx, t, c.migrator, `SELECT count(DISTINCT envelope->'payload'->>'lesson_version_id') FROM infra.outbox_events
		WHERE event_type = 'lesson.completed' AND envelope->'payload'->>'student_id' = $1`, an.String()); n != 2 {
		t.Fatalf("distinct versions completed = %d", n)
	}
}
