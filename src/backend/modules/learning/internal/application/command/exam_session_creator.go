package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam"
)

// ExamSessionCreator is the F042 adapter of the F041 port port.ExamSessionCreator (F042-AC-31): the publish of an
// assignment on a blueprint creates its exam session in the same transaction, the cancel cancels it. Exams is set by the
// composition root once both use case structs exist (they reference each other).
type ExamSessionCreator struct {
	Exams *Exams
}

var _ port.ExamSessionCreator = ExamSessionCreator{}

// Available implements port.ExamSessionCreator.
func (c ExamSessionCreator) Available() bool { return c.Exams != nil }

// Create implements port.ExamSessionCreator: the assignment's students, window and release policy; the type is custom
// and official for a graded assignment (EXAM_ENGINE §2: a school test), a practice test otherwise.
func (c ExamSessionCreator) Create(ctx context.Context, r port.ExamSessionRequest) (uuid.UUID, error) {
	if c.Exams == nil {
		return uuid.Nil, port.ErrExamSessionUnavailable
	}
	course := r.CourseID
	assignmentID := r.AssignmentID
	official := true
	typ := string(exam.TypeCustom)
	in := ExamSessionInput{Title: r.Title, BlueprintVersionID: r.BlueprintVerID, CourseID: &course, AssignmentID: &assignmentID, Type: &typ,
		IsOfficial: &official, WindowStart: r.OpenAt, WindowEnd: r.CloseAt}
	cfg := c.Exams.config()
	feedback, release := cfg.GradedFeedback, cfg.GradedRelease
	if r.FeedbackTiming != "" && r.FeedbackTiming != exam.FeedbackImmediate {
		feedback = r.FeedbackTiming
	}
	if r.ResultRelease != "" {
		release = r.ResultRelease
	}
	in.FeedbackTiming, in.ResultRelease = &feedback, &release
	if r.MaxAttempts > 0 {
		n := min(r.MaxAttempts, exam.MaxAttemptsLimit)
		in.MaxAttempts = &n
	}
	if r.ScorePolicy != "" {
		sp := r.ScorePolicy
		in.ScorePolicy = &sp
	}
	return c.Exams.CreateForAssignment(ctx, in, r.StudentIDs)
}

// Cancel implements port.ExamSessionCreator.
func (c ExamSessionCreator) Cancel(ctx context.Context, examSessionID uuid.UUID, reason string) error {
	if c.Exams == nil {
		return port.ErrExamSessionUnavailable
	}
	return c.Exams.CancelExamSession(ctx, examSessionID, reason)
}
