package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/assignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func auditEvent(action string, a write.AssignmentRow, student uuid.UUID, reason string) audit.Event {
	return audit.Event{Action: action, TargetType: "assignment", TargetID: a.ID.String(), Result: audit.ResultSuccess, Reason: reason,
		Metadata: map[string]any{"student_id": student.String()}, SchoolID: a.SchoolID.String()}
}

// AssignmentAdmission is what an attempt started for an assignment pins (F041-AC-13, AC-14): the content, the version
// (nil = the current release with follow_latest), the feedback timing and release of the assignment; OpenAttempt is the
// attempt of the student still in progress (resumed instead of a new one).
type AssignmentAdmission struct {
	AssignmentID   uuid.UUID
	ContentType    string
	ContentID      uuid.UUID
	VersionID      *uuid.UUID
	FeedbackTiming string
	ResultRelease  string
	OpenAttempt    *uuid.UUID
}

// AssignmentGate admits the attempts of an assignment and tells whether their results are visible (Attempts).
type AssignmentGate interface {
	Admit(ctx context.Context, studentID, assignmentID uuid.UUID) (AssignmentAdmission, error)
	Release(ctx context.Context, assignmentID uuid.UUID) (release string, visible bool, err error)
}

var _ AssignmentGate = Assignments{}

// Admit checks a start of an attempt for an assignment in the transaction of ctx (F041-AC-13): a student outside the
// targets 404 NOT_FOUND; excused, outside open_at to the personal due date plus the late window, or an assignment not
// open (a closed one stays open for a student with a running extension) 403 ASSIGNMENT_NOT_OPEN; max_attempts used
// 409 ASSIGNMENT_MAX_ATTEMPTS. An attempt in progress is returned to resume.
func (h Assignments) Admit(ctx context.Context, studentID, assignmentID uuid.UUID) (out AssignmentAdmission, err error) {
	a, err := h.Store.Assignment(ctx, assignmentID, false)
	if err != nil {
		return out, err
	}
	if a == nil {
		return out, problem.New(problem.NotFound, "")
	}
	s, err := h.Store.Submission(ctx, a.ID, studentID, false)
	if err != nil {
		return out, err
	}
	if s == nil {
		return out, problem.New(problem.NotFound, "")
	}
	out = AssignmentAdmission{AssignmentID: a.ID, ContentType: a.ContentType, ContentID: a.ContentID, VersionID: a.PinnedVersionID,
		FeedbackTiming: a.FeedbackTiming, ResultRelease: a.ResultRelease}
	attempts, err := h.Store.AttemptsOfContext(ctx, a.ID, studentID)
	if err != nil {
		return out, err
	}
	for _, x := range attempts {
		if x.Status == "in_progress" {
			id := x.AttemptID
			out.OpenAttempt = &id
			return out, nil
		}
	}
	w, now := window(*a, s), h.now()
	extended := s.DueAtOverride != nil && a.Status != string(assignment.StatusCancelled) && a.Status != string(assignment.StatusArchived) &&
		a.Status != string(assignment.StatusDraft) && a.Status != string(assignment.StatusScheduled)
	switch {
	case s.Status == "excused":
		return out, problem.New(problem.AssignmentNotOpen, "the student is excused")
	case a.Status != string(assignment.StatusOpen) && !extended:
		return out, problem.New(problem.AssignmentNotOpen, "the assignment is not open")
	case !w.Open(now):
		return out, problem.New(problem.AssignmentNotOpen, "outside the window of the assignment")
	case len(attempts) >= a.MaxAttempts:
		return out, problem.New(problem.AssignmentMaxAttempts, "")
	}
	return out, nil
}

// Release reports the result release of an assignment and whether its results are visible to its students now
// (F041-AC-19).
func (h Assignments) Release(ctx context.Context, assignmentID uuid.UUID) (string, bool, error) {
	a, err := h.Store.Assignment(ctx, assignmentID, false)
	if err != nil || a == nil {
		return "", false, err
	}
	return a.ResultRelease, assignment.ResultsVisible(a.ResultRelease, assignment.Status(a.Status)), nil
}
