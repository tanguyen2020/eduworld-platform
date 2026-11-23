package challenge

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func start(v Version, level string, grade int) *Attempt {
	r := rules()
	p, _ := v.Profile(level)
	return NewAttempt(Start{ID: uuid.New(), TenantID: uuid.New(), StudentID: uuid.New(), SchoolID: uuid.New(),
		ChallengeID: uuid.MustParse(v.ChallengeID), VersionID: uuid.New(), Type: v.Type, Difficulty: level, PeriodKey: "2026-11-24",
		Lives: r.Lives(v, p), DeadlineAt: r.Deadline(v, grade, t0), At: t0})
}

// open opens the next learning attempt of a, returns its id.
func open(t *testing.T, a *Attempt, v Version) uuid.UUID {
	t.Helper()
	if _, ok := a.NextRef(v); !ok {
		t.Fatal("no next activity")
	}
	id := uuid.New()
	if err := a.Opened(id); err != nil {
		t.Fatal(err)
	}
	return id
}

func graded(id uuid.UUID, correct, items int, at time.Time) Graded {
	return Graded{AttemptID: id, Correct: correct, Items: items, At: at}
}

// F076-AC-10, AC-11: survival — sai trừ sức bền theo từng câu, hết sức bền → failed (lives_depleted) đúng một lần; kết
// quả lặp không áp hai lần; chuỗi đúng hồi sức bền tới mức tối đa.
func TestSurvival(t *testing.T) {
	r := rules()
	v := version(TypeSurvival)
	v.Goal = Goal{Metric: MetricRounds, Target: 10}
	a := start(v, Hard, 7)
	if a.LivesLeft == nil || *a.LivesLeft != 3 {
		t.Fatalf("lives: %v", a.LivesLeft)
	}
	id := open(t, a, v)
	if o := a.Apply(v, 3, r, graded(id, 0, 1, t0.Add(time.Minute))); !o.Changed || o.Ended || *a.LivesLeft != 2 {
		t.Fatalf("wrong: %+v lives %d", o, *a.LivesLeft)
	}
	if o := a.Apply(v, 3, r, graded(id, 0, 1, t0.Add(time.Minute))); o.Changed || *a.LivesLeft != 2 {
		t.Fatal("a replayed result changes nothing")
	}
	if o := a.Apply(v, 3, r, graded(uuid.New(), 0, 1, t0.Add(time.Minute))); o.Changed {
		t.Fatal("an attempt of another challenge changes nothing")
	}
	// Three correct rounds in a row restore one life (up to the level's lives).
	for range 3 {
		a.Apply(v, 3, r, graded(open(t, a, v), 1, 1, t0.Add(2*time.Minute)))
	}
	if *a.LivesLeft != 3 || a.Score != 3 || a.ActivitiesCompleted != 4 || a.GoalProgress != 4 {
		t.Fatalf("regen: lives %d score %d done %d progress %v", *a.LivesLeft, a.Score, a.ActivitiesCompleted, a.GoalProgress)
	}
	for i := range 3 {
		o := a.Apply(v, 3, r, graded(open(t, a, v), 0, 1, t0.Add(3*time.Minute)))
		if (i == 2) != o.Ended {
			t.Fatalf("round %d ended=%v", i, o.Ended)
		}
	}
	if a.Status != StatusFailed || a.EndReason != EndLivesDepleted || *a.LivesLeft != 0 || a.EndedAt == nil {
		t.Fatalf("failed: %s %s", a.Status, a.EndReason)
	}
	if o := a.Apply(v, 3, r, graded(open2(a), 1, 1, t0.Add(4*time.Minute))); o.Changed {
		t.Fatal("an ended challenge never changes")
	}
	if _, err := a.Abandon(t0, EndPlayer); !errors.Is(err, ErrInvalidState) {
		t.Fatal("abandon an ended challenge")
	}
}

func open2(a *Attempt) uuid.UUID {
	id := uuid.New()
	a.LearningAttemptIDs = append(a.LearningAttemptIDs, id)
	return id
}

// F076-AC-11: time — đủ goal trước hạn → succeeded; kết quả chấm tới sau hạn + dung sai không tính.
func TestTimeGoalAndDeadline(t *testing.T) {
	r := rules()
	v := version(TypeTime)
	a := start(v, Normal, 7)
	id := open(t, a, v)
	if o := a.Apply(v, 0, r, graded(id, 3, 4, t0.Add(time.Minute))); o.Ended || a.Score != 3 || a.ItemsAnswered != 4 {
		t.Fatalf("partial: %+v", a)
	}
	late := open(t, a, v)
	if o := a.Apply(v, 0, r, graded(late, 4, 4, t0.Add(303*time.Second))); o.Changed {
		t.Fatal("graded after deadline + grace is not counted")
	}
	inGrace := open(t, a, v)
	if o := a.Apply(v, 0, r, graded(inGrace, 2, 4, t0.Add(301*time.Second))); !o.Ended || a.Status != StatusSucceeded ||
		a.EndReason != EndGoalReached {
		t.Fatalf("goal within the grace: %+v %s", o, a.Status)
	}
	// The refs cycle for a time challenge (new learning attempt each time).
	b := start(v, Normal, 7)
	for i := range 5 {
		ref, ok := b.NextRef(v)
		if !ok || ref.ActivityID != v.Source.ActivityRefs[i%2].ActivityID {
			t.Fatalf("cycle %d: %v", i, ref)
		}
		_ = b.Opened(uuid.New())
	}
}

// F076-AC-11, AC-13: perfect 100% — một câu sai → failed (accuracy_missed); perfect 90% đánh giá khi hết bộ; knowledge
// hết bộ chưa đạt goal → failed; hết hạn → failed deadline_passed, không giới hạn → abandoned sau resume window.
func TestPerfectKnowledgeExpiry(t *testing.T) {
	r := rules()
	p := version(TypePerfect)
	p.Goal = Goal{Metric: MetricAccuracy, Target: 1}
	a := start(p, Easy, 7)
	if o := a.Apply(p, 0, r, graded(open(t, a, p), 3, 4, t0.Add(time.Minute))); !o.Ended || a.EndReason != EndAccuracyMissed {
		t.Fatalf("perfect 100%%: %s", a.EndReason)
	}
	p.Goal.Target = 0.75
	b := start(p, Easy, 7)
	if o := b.Apply(p, 0, r, graded(open(t, b, p), 3, 4, t0.Add(time.Minute))); o.Ended {
		t.Fatal("evaluated at the end of the set")
	}
	if o := b.Apply(p, 0, r, graded(open(t, b, p), 4, 4, t0.Add(time.Minute))); !o.Ended || b.Status != StatusSucceeded {
		t.Fatalf("7/8 ≥ 0.75: %s %s", b.Status, b.EndReason)
	}
	if _, ok := b.NextRef(p); ok {
		t.Fatal("perfect does not cycle")
	}
	k := version(TypeKnowledge)
	k.Goal = Goal{Metric: MetricCorrect, Target: 10}
	c := start(k, Easy, 7)
	c.Apply(k, 0, r, graded(open(t, c, k), 4, 4, t0.Add(time.Minute)))
	if o := c.Apply(k, 0, r, graded(open(t, c, k), 4, 4, t0.Add(time.Minute))); !o.Ended || c.EndReason != EndActivitiesExhausted {
		t.Fatalf("knowledge exhausted: %s", c.EndReason)
	}
	// Server clock expiry.
	d := start(version(TypeTime), Easy, 7)
	if err := d.CanNext(t0.Add(301*time.Second), r); !errors.Is(err, ErrInvalidState) {
		t.Fatal("next after the deadline")
	}
	if d.Expire(t0.Add(301*time.Second), r) {
		t.Fatal("expired within the grace")
	}
	if !d.Expire(t0.Add(302*time.Second), r) || d.Status != StatusFailed || d.EndReason != EndDeadlinePassed {
		t.Fatalf("deadline: %s %s", d.Status, d.EndReason)
	}
	u := version(TypeKnowledge)
	u.TimeLimitSeconds = 0
	e := start(u, Easy, 7)
	if e.DeadlineAt != nil || e.Due(r).Sub(t0) != 24*time.Hour {
		t.Fatalf("resume window due: %v", e.Due(r))
	}
	if err := e.CanNext(t0.Add(23*time.Hour), r); err != nil {
		t.Fatal(err)
	}
	if !e.Expire(t0.Add(24*time.Hour+3*time.Second), r) || e.Status != StatusAbandoned || e.EndReason != EndResumeExpired {
		t.Fatalf("resume window: %s %s", e.Status, e.EndReason)
	}
	f := start(version(TypeTime), Easy, 7)
	if changed, err := f.Abandon(t0, EndPlayer); err != nil || !changed || f.Status != StatusAbandoned {
		t.Fatal("abandon")
	}
}

// Puzzle: một hoạt động "đạt" (passed) là một câu đố giải xong; activity limit 200.
func TestPuzzleAndLimit(t *testing.T) {
	r := rules()
	v := version(TypePuzzle)
	v.Goal = Goal{Metric: MetricPuzzle, Target: 2}
	a := start(v, Easy, 7)
	yes := true
	no := false
	g := graded(open(t, a, v), 1, 2, t0.Add(time.Minute))
	g.Passed = &yes
	a.Apply(v, 0, r, g)
	g = graded(open(t, a, v), 2, 2, t0.Add(time.Minute))
	g.Passed = &no
	a.Apply(v, 0, r, g)
	if a.GoalProgress != 1 || a.Active() == false {
		t.Fatalf("puzzle progress %v", a.GoalProgress)
	}
	b := start(version(TypeTime), Easy, 7)
	for range MaxActivities {
		_ = b.Opened(uuid.New())
	}
	if err := b.Opened(uuid.New()); !errors.Is(err, ErrInvalidState) {
		t.Fatal("activity limit")
	}
}
