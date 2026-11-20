package attempt

import (
	"errors"
	"testing"
	"time"
)

// F032-AC-10…AC-13 (T020, T022): hint levels open in order up to the hints of the pinned version; not in exam,
// diagnostic, competition or without immediate feedback; not after the attempt ended.
func TestRevealHint(t *testing.T) {
	p := DefaultPolicy()
	a := newAttempt(ModePractice, "", nil)
	for want := 1; want <= 2; want++ {
		lvl, err := a.RevealHint(1, 2, t0.Add(time.Second), p)
		if err != nil || lvl != want {
			t.Fatalf("level %d: %d %v", want, lvl, err)
		}
	}
	if _, err := a.RevealHint(1, 2, t0, p); !errors.Is(err, ErrHintsExhausted) {
		t.Fatalf("exhausted: %v", err)
	}
	if _, err := a.RevealHint(2, 0, t0, p); !errors.Is(err, ErrHintsExhausted) {
		t.Fatalf("no hints: %v", err)
	}
	if _, err := a.RevealHint(9, 2, t0, p); !errors.Is(err, ErrPositionInvalid) {
		t.Fatalf("position: %v", err)
	}
	for _, m := range []Mode{ModeExam, ModeDiagnostic, ModeCompetition} {
		b := newAttempt(m, "immediate", nil)
		b.Feedback = FeedbackImmediate
		if _, err := b.RevealHint(1, 2, t0, p); !errors.Is(err, ErrHintsNotAllowed) {
			t.Fatalf("%s: %v", m, err)
		}
	}
	if b := newAttempt(ModePractice, "after_submit", nil); !errors.Is(second(b.RevealHint(1, 2, t0, p)), ErrHintsNotAllowed) {
		t.Fatal("after_submit allows hints")
	}
	a.Status = StatusGraded
	if _, err := a.RevealHint(2, 2, t0, p); !errors.Is(err, ErrNotInProgress) {
		t.Fatalf("graded: %v", err)
	}
}

func second(_ int, err error) error { return err }

// F032-AC-14, AC-16 (T021, T026): hints_used is the number of levels open before the answer; a hint does not restart
// the response interval; the open tail of a pause is excluded; the client time only shortens the server interval.
func TestAnswerTimingAndHints(t *testing.T) {
	p := DefaultPolicy()
	a := newAttempt(ModePractice, "", nil)
	if _, err := a.RevealHint(1, 3, t0.Add(4*time.Second), p); err != nil {
		t.Fatal(err)
	}
	ans, err := a.Respond(1, t0.Add(5*time.Second), p, 3, true, nil)
	if err != nil || ans.HintsUsed != 1 || ans.ResponseMS != 5000 || !ans.IsScored {
		t.Fatalf("answer: %+v %v", ans, err)
	}
	a.Record(ans, false, 0)
	// A hint after the scored answer does not change it; the retry carries the new count.
	if _, err := a.RevealHint(1, 3, t0.Add(6*time.Second), p); err != nil {
		t.Fatal(err)
	}
	retry, _ := a.Respond(1, t0.Add(7*time.Second), p, 3, true, nil)
	if retry.IsScored || retry.HintsUsed != 2 {
		t.Fatalf("retry: %+v", retry)
	}
	// Pause still open: the interval stops at paused_at; the client time is taken when it is shorter.
	b := newAttempt(ModePractice, "", nil)
	pausedAt := t0.Add(3 * time.Second)
	b.PausedAt = &pausedAt
	got, _ := b.Respond(1, t0.Add(10*time.Minute), p, 3, true, nil)
	if got.ResponseMS != 3000 {
		t.Fatalf("paused interval: %d", got.ResponseMS)
	}
	c := newAttempt(ModePractice, "", nil)
	client := int64(800)
	if got, _ := c.Respond(1, t0.Add(9*time.Second), p, 3, true, &client); got.ResponseMS != 800 {
		t.Fatalf("client shorter: %d", got.ResponseMS)
	}
	long := int64(60_000)
	if got, _ := c.Respond(2, t0.Add(11*time.Second), p, 3, true, &long); got.ResponseMS != 2000 {
		t.Fatalf("client longer than the server: %d", got.ResponseMS)
	}
	// A hint during a pause ends it without restarting the interval of the item.
	d := newAttempt(ModePractice, "", nil)
	pd := t0.Add(2 * time.Second)
	d.PausedAt = &pd
	if _, err := d.RevealHint(1, 1, t0.Add(62*time.Second), p); err != nil || d.PausedAt != nil || d.PausedMS != 60_000 {
		t.Fatalf("hint ends the pause: %+v %v", d, err)
	}
	if got, _ := d.Respond(1, t0.Add(63*time.Second), p, 3, true, nil); got.ResponseMS != 3000 {
		t.Fatalf("interval after the pause: %d", got.ResponseMS)
	}
}

// F032-AC-16, AC-19 (T026): thresholds by type, x1.5 for grades 1–3, min_response_ms of the question first; mini-games
// and competition use their own threshold only; unscored retries are never flagged.
func TestRapidGuess(t *testing.T) {
	r := DefaultRapidGuess()
	g3, g7 := 3, 7
	ms := func(v int) *int { return &v }
	for _, c := range []struct {
		in   GuessInput
		want bool
	}{
		{GuessInput{Mode: ModePractice, QuestionType: "single_choice", Grade: &g3, ResponseMS: 800, Scored: true}, true},
		{GuessInput{Mode: ModePractice, QuestionType: "single_choice", Grade: &g3, ResponseMS: 2500, Scored: true}, true}, // 3000 for g1_3
		{GuessInput{Mode: ModePractice, QuestionType: "single_choice", Grade: &g7, ResponseMS: 2500, Scored: true}, false},
		{GuessInput{Mode: ModePractice, QuestionType: "true_false", Grade: &g7, ResponseMS: 1499, Scored: true}, true},
		{GuessInput{Mode: ModePractice, QuestionType: "matching", Grade: &g7, ResponseMS: 3999, Scored: true}, true},
		{GuessInput{Mode: ModePractice, QuestionType: "interactive", Grade: &g7, ResponseMS: 10, Scored: true}, false},
		{GuessInput{Mode: ModePractice, QuestionType: "single_choice", ItemMinMS: ms(500), Grade: &g3, ResponseMS: 800, Scored: true}, false},
		{GuessInput{Mode: ModePractice, QuestionType: "single_choice", Grade: &g3, ResponseMS: 800, Scored: false}, false},
		{GuessInput{Mode: ModeMinigame, QuestionType: "single_choice", Grade: &g3, ResponseMS: 800, Scored: true}, false},
		{GuessInput{Mode: ModeCompetition, QuestionType: "single_choice", ModeMinMS: ms(1000), ResponseMS: 800, Scored: true}, true},
		{GuessInput{Mode: ModeRetryWrong, QuestionType: "numeric", ResponseMS: 2999, Scored: true}, true},
	} {
		if got := r.Flag(c.in); got != c.want {
			t.Fatalf("%+v: %v", c.in, got)
		}
	}
}
