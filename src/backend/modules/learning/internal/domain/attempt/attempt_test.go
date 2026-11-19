package attempt

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/quiz"
)

var t0 = time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)

func items(n int) []Item {
	out := make([]Item, n)
	for i := range out {
		out[i] = Item{Position: i + 1, QuestionVersionID: uuid.New(), QuestionType: "single_choice", MaxScore: 1,
			ObjectiveIDs: []uuid.UUID{uuid.MustParse("00000000-0000-7000-8000-000000000001")}}
	}
	return out
}

func newAttempt(mode Mode, feedback string, limit *int) Attempt {
	return New(Start{ID: uuid.New(), Mode: mode, SourceVersionID: uuid.New(), Now: t0, TimeLimitSeconds: limit, ActivityFeedback: feedback,
		DisplayLanguage: "fr", Items: items(3)})
}

// F030-AC-01 (T009): start pins the items, server start time, the display language and the feedback timing of the mode.
func TestStart(t *testing.T) {
	a := newAttempt(ModePractice, "", nil)
	if a.Status != StatusInProgress || !a.StartedAt.Equal(t0) || a.DeadlineAt != nil || a.Feedback != FeedbackImmediate ||
		a.DisplayLanguage != "vi" || len(a.ObjectiveIDs) != 1 || len(a.Items) != 3 {
		t.Fatalf("start: %+v", a)
	}
	limit := 60
	if a := newAttempt(ModeQuest, "after_submit", &limit); a.Feedback != FeedbackAfterSubmit || !a.DeadlineAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("timed quest: %+v", a)
	}
	for _, c := range []struct {
		mode Mode
		in   string
		want FeedbackTiming
	}{{ModeDiagnostic, "", FeedbackAfterSubmit}, {ModeDiagnostic, "immediate", FeedbackAfterSubmit}, {ModeExam, "immediate", FeedbackAfterSubmit},
		{ModeMinigame, "", FeedbackImmediate}, {ModePractice, "never", FeedbackNever}, {ModePractice, "bogus", FeedbackImmediate}} {
		if got := ResolveFeedback(c.mode, c.in); got != c.want {
			t.Fatalf("%s/%s: %s", c.mode, c.in, got)
		}
	}
}

// F030-AC-08, AC-10, AC-12 (T020): first answer scored, retries unscored, limit per grade, unanswered = 0, duration
// without pauses, score from the plugin.
func TestRespondAndSubmit(t *testing.T) {
	p := DefaultPolicy()
	a := newAttempt(ModePractice, "", nil)
	ans, err := a.Respond(1, t0.Add(3*time.Second), p, p.AnswerLimit(nil), true, nil)
	if err != nil || !ans.IsScored || ans.AttemptNo != 1 || ans.ResponseMS != 3000 {
		t.Fatalf("first answer: %+v %v", ans, err)
	}
	a.Record(ans, false, 0)
	client := int64(500)
	retry, err := a.Respond(1, t0.Add(5*time.Second), p, 2, true, &client)
	if err != nil || retry.IsScored || retry.AttemptNo != 2 || retry.ResponseMS != 500 {
		t.Fatalf("retry: %+v %v", retry, err)
	}
	a.Record(retry, true, 1)
	if _, err := a.Respond(1, t0.Add(6*time.Second), p, 2, true, nil); !errors.Is(err, ErrAnswerLimit) {
		t.Fatalf("limit: %v", err)
	}
	if _, err := a.Respond(9, t0.Add(6*time.Second), p, 2, true, nil); !errors.Is(err, ErrPositionInvalid) {
		t.Fatalf("position: %v", err)
	}
	g := 2
	if p.AnswerLimit(&g) != 3 || p.AnswerLimit(nil) != 2 {
		t.Fatal("attempts_per_question by grade")
	}
	ans, _ = a.Respond(2, t0.Add(10*time.Second), p, 2, true, nil)
	a.Record(ans, true, 1)
	// A pause of 20 s is excluded from the duration.
	if err := a.Pause(t0.Add(20*time.Second), p); err != nil {
		t.Fatal(err)
	}
	if err := a.Resume(t0.Add(40*time.Second), p); err != nil || a.PausedMS != 20000 {
		t.Fatalf("resume: %v %d", err, a.PausedMS)
	}
	v := activity.Version{Type: activity.Quiz, Config: activity.Config{Evaluation: activity.Evaluation{Strategies: []string{"auto"}}}}
	res := quiz.New().Evaluate(v, a.ItemResults())
	if res.Score != 1 || res.MaxScore != 3 || res.CorrectCount != 1 {
		t.Fatalf("evaluate (item 1 keeps its scored wrong answer, item 3 unanswered): %+v", res)
	}
	if err := a.Submit(t0.Add(60*time.Second), p, ReasonManual, res); err != nil {
		t.Fatal(err)
	}
	if a.Status != StatusGraded || *a.DurationMS != 40000 || *a.Score != 1 || *a.SubmitReason != ReasonManual {
		t.Fatalf("submitted: %+v", a)
	}
	if err := a.Submit(t0.Add(61*time.Second), p, ReasonManual, res); !errors.Is(err, ErrAlreadySubmitted) {
		t.Fatalf("second submit: %v", err)
	}
	if _, err := a.Respond(3, t0.Add(62*time.Second), p, 2, true, nil); !errors.Is(err, ErrNotInProgress) {
		t.Fatalf("respond after submit: %v", err)
	}
}

// Deferred feedback: every answer is scored and the last one counts; immediate without retry: one answer.
func TestFeedbackPolicyOfAnswers(t *testing.T) {
	p := DefaultPolicy()
	a := newAttempt(ModePractice, "after_submit", nil)
	for i, correct := range []bool{true, false} {
		ans, err := a.Respond(1, t0.Add(time.Duration(i+1)*time.Second), p, 2, true, nil)
		if err != nil || !ans.IsScored {
			t.Fatalf("deferred answer %d: %+v %v", i, ans, err)
		}
		a.Record(ans, correct, map[bool]float64{true: 1, false: 0}[correct])
	}
	if *a.Items[0].FinalIsCorrect || *a.Items[0].FinalScore != 0 {
		t.Fatal("the last scored answer counts")
	}
	if a.ShowsItemFeedback() {
		t.Fatal("after_submit shows feedback before grading")
	}
	b := newAttempt(ModePractice, "", nil)
	ans, _ := b.Respond(1, t0.Add(time.Second), p, 3, false, nil)
	b.Record(ans, false, 0)
	if _, err := b.Respond(1, t0.Add(2*time.Second), p, 3, false, nil); !errors.Is(err, ErrAnswerLimit) {
		t.Fatalf("no unscored retry: %v", err)
	}
}

// F030-AC-17, AC-18, AC-19: idle pause, abandon after the resume window, expiry and timeout submit with the deadline as
// the end; manual submit after the grace is refused.
func TestLifecycle(t *testing.T) {
	p := DefaultPolicy()
	a := newAttempt(ModePractice, "", nil)
	if a.IdlePause(t0.Add(4*time.Minute), p) || !a.IdlePause(t0.Add(6*time.Minute), p) || !a.PausedAt.Equal(t0.Add(5*time.Minute)) {
		t.Fatalf("idle pause: %v", a.PausedAt)
	}
	if a.Abandon(t0.Add(23*time.Hour), p) || !a.Abandon(t0.Add(25*time.Hour), p) || a.Status != StatusAbandoned {
		t.Fatal("abandon")
	}
	if err := a.CheckActive(t0.Add(26*time.Hour), p); !errors.Is(err, ErrExpired) {
		t.Fatalf("abandoned attempt: %v", err)
	}
	limit := 60
	b := newAttempt(ModePractice, "", &limit)
	ans, _ := b.Respond(1, t0.Add(10*time.Second), p, 2, true, nil)
	b.Record(ans, true, 1)
	if _, err := b.Respond(2, t0.Add(61*time.Second), p, 2, true, nil); err != nil {
		t.Fatalf("within the grace: %v", err)
	}
	if _, err := b.Respond(2, t0.Add(63*time.Second), p, 2, true, nil); !errors.Is(err, ErrExpired) {
		t.Fatalf("after the grace: %v", err)
	}
	if err := b.Submit(t0.Add(63*time.Second), p, ReasonManual, activity.Result{}); !errors.Is(err, ErrDeadlinePassed) {
		t.Fatalf("manual submit after the deadline: %v", err)
	}
	if !b.Expire(t0.Add(63*time.Second), p) || b.Status != StatusExpired {
		t.Fatal("expire")
	}
	if err := b.Submit(t0.Add(70*time.Second), p, ReasonTimeout, activity.Result{Score: 1, MaxScore: 3}); err != nil {
		t.Fatal(err)
	}
	if b.Status != StatusGraded || !b.SubmittedAt.Equal(t0.Add(time.Minute)) || *b.DurationMS != 60000 {
		t.Fatalf("timeout submit: %+v", b)
	}
}
