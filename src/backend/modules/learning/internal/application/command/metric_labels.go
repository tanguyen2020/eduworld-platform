package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SubjectCodes resolves the code of a subject of the platform catalog (curriculum.subjects, global, immutable code): the
// subject label of the learning metrics of MONITORING §3 — a bounded catalogue code, never a subject id.
type SubjectCodes interface {
	SubjectCode(ctx context.Context, id uuid.UUID) (string, error)
}

// labelUnknown is the value of a metric label that cannot be resolved (no subject, grade not projected).
const labelUnknown = "unknown"

// Results of eduworld_attempts_submitted_total (MONITORING §3).
const (
	SubmitOK        = "ok"
	SubmitDuplicate = "duplicate"
	SubmitLate      = "late"
	SubmitRejected  = "rejected"
	// ExamAutoSubmitted is the result of an exam attempt submitted by the server (exam.autosubmit, window close).
	ExamAutoSubmitted = "auto_submitted"
)

// subjectLabel is the subject code of id ("unknown" when absent, unknown to the catalog or on error: a metric label never
// fails the use case).
func subjectLabel(ctx context.Context, s SubjectCodes, id *uuid.UUID) string {
	if s == nil || id == nil || *id == uuid.Nil {
		return labelUnknown
	}
	c, err := s.SubjectCode(ctx, *id)
	if err != nil || c == "" {
		return labelUnknown
	}
	return c
}

// gradeBandLabel is the grade band of a grade (g1_3…g8_9), "unknown" when the grade is not projected.
func gradeBandLabel(grade *int) string {
	if grade == nil || *grade < 1 || *grade > 9 {
		return labelUnknown
	}
	return mastery.GradeBand(grade)
}

// contextLabel is the context of eduworld_learning_activities_completed_total: practice (self-study modes), assignment,
// quest (quest steps and challenges) or competition; "" for exams (counted by the exam metrics) and unknown modes.
func contextLabel(m attempt.Mode) string {
	switch m {
	case attempt.ModePractice, attempt.ModeRetryWrong, attempt.ModeDiagnostic, attempt.ModeMinigame:
		return "practice"
	case attempt.ModeAssignment:
		return "assignment"
	case attempt.ModeQuest, attempt.ModeChallenge:
		return "quest"
	case attempt.ModeCompetition:
		return "competition"
	}
	return ""
}

// submitResult classifies the outcome of a student submit (attempt or exam): ok, duplicate (already submitted), late
// (deadline passed, expired) or rejected (any other refusal); "" for a server error (counted by the HTTP metrics only).
func submitResult(err error) string {
	if err == nil {
		return SubmitOK
	}
	var pe *problem.Error
	if !errors.As(err, &pe) || problem.Status(pe.Code) >= 500 {
		return ""
	}
	switch pe.Code {
	case problem.AttemptAlreadySubmitted, problem.ExamAlreadySubmitted:
		return SubmitDuplicate
	case problem.AttemptExpired, problem.AttemptDeadlinePassed, problem.ExamTimeOver:
		return SubmitLate
	}
	return SubmitRejected
}

// completionLabels returns the labels of eduworld_learning_activities_completed_total for an attempt graded by a student
// submit (nil without metrics or for a mode outside the completion contexts): subject of the pinned activity version, grade
// band of the student (ref.students, in the transaction of ctx).
func (h Attempts) completionLabels(ctx context.Context, a attempt.Attempt, src *ActivitySource) (*[4]string, error) {
	c := contextLabel(a.Mode)
	if h.Metrics == nil || c == "" {
		return nil, nil
	}
	var subject *uuid.UUID
	if src != nil {
		subject = src.Version.SubjectID // subject of the pinned version (R31)
	}
	var grade *int
	if h.Students != nil {
		_, g, err := h.Students.StudentSchool(ctx, a.StudentID)
		if err != nil {
			return nil, err
		}
		grade = g
	}
	return &[4]string{a.ActivityType, c, subjectLabel(ctx, h.Subjects, subject), gradeBandLabel(grade)}, nil
}

// nonNilSubject is the subject of a question version (uuid.Nil = none).
func nonNilSubject(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
