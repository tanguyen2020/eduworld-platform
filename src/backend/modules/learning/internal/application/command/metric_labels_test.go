package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MONITORING §3: result of eduworld_attempts_submitted_total / eduworld_exam_submissions_total from the submit error.
func TestSubmitResult(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{nil, SubmitOK},
		{problem.New(problem.AttemptAlreadySubmitted, ""), SubmitDuplicate},
		{problem.New(problem.ExamAlreadySubmitted, ""), SubmitDuplicate},
		{problem.New(problem.AttemptExpired, ""), SubmitLate},
		{problem.New(problem.AttemptDeadlinePassed, ""), SubmitLate},
		{problem.New(problem.ExamTimeOver, ""), SubmitLate},
		{problem.New(problem.CueAnswerRequired, ""), SubmitRejected},
		{problem.New(problem.RateLimited, ""), SubmitRejected},
		{problem.New(problem.Unavailable, ""), ""},
		{errors.New("database down"), ""},
	} {
		if got := submitResult(c.err); got != c.want {
			t.Errorf("submitResult(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// Contexts of eduworld_learning_activities_completed_total: exam attempts are not counted here.
func TestContextLabel(t *testing.T) {
	for m, want := range map[attempt.Mode]string{attempt.ModePractice: "practice", attempt.ModeRetryWrong: "practice",
		attempt.ModeDiagnostic: "practice", attempt.ModeMinigame: "practice", attempt.ModeAssignment: "assignment",
		attempt.ModeQuest: "quest", attempt.ModeChallenge: "quest", attempt.ModeCompetition: "competition", attempt.ModeExam: ""} {
		if got := contextLabel(m); got != want {
			t.Errorf("contextLabel(%s) = %q, want %q", m, got, want)
		}
	}
}

type subjectsFake map[uuid.UUID]string // TEST DOUBLE (R17) of the subject catalog

func (f subjectsFake) SubjectCode(_ context.Context, id uuid.UUID) (string, error) {
	if c, ok := f[id]; ok {
		return c, nil
	}
	return "", problem.New(problem.NotFound, "")
}

// Subject and grade band labels are bounded: a catalog code or "unknown", a band or "unknown" — never an id or a grade.
func TestSubjectAndGradeLabels(t *testing.T) {
	math := uuid.New()
	s := subjectsFake{math: "math"}
	ctx := context.Background()
	other := uuid.New()
	for _, c := range []struct {
		s    SubjectCodes
		id   *uuid.UUID
		want string
	}{{s, &math, "math"}, {s, &other, labelUnknown}, {s, nil, labelUnknown}, {nil, &math, labelUnknown}, {s, nonNilSubject(uuid.Nil), labelUnknown}} {
		if got := subjectLabel(ctx, c.s, c.id); got != c.want {
			t.Errorf("subjectLabel = %q, want %q", got, c.want)
		}
	}
	g := func(n int) *int { return &n }
	for _, c := range []struct {
		grade *int
		want  string
	}{{nil, labelUnknown}, {g(2), "g1_3"}, {g(4), "g4_5"}, {g(7), "g6_7"}, {g(9), "g8_9"}, {g(0), labelUnknown}, {g(10), labelUnknown}} {
		if got := gradeBandLabel(c.grade); got != c.want {
			t.Errorf("gradeBandLabel(%v) = %q, want %q", c.grade, got, c.want)
		}
	}
}
