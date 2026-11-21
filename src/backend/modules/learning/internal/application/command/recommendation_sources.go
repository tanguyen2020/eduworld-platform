package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/recommendation"
)

// SnapshotSource adds data of another feature to the snapshot of the rule engine (F041 R1, F042 R5), so their
// rules register without changing the engine nor the loader of F032 (F032-AC-33).
type SnapshotSource interface {
	Fill(ctx context.Context, studentID uuid.UUID, s *recommendation.Snapshot) error
}

// AssignmentsDue is the SnapshotSource of F041: the open school assignments the student has not submitted (rule R1).
type AssignmentsDue struct {
	Store write.SchoolAssignmentStore
}

// Fill implements SnapshotSource.
func (u AssignmentsDue) Fill(ctx context.Context, studentID uuid.UUID, s *recommendation.Snapshot) error {
	xs, err := u.Store.PendingAssignments(ctx, studentID)
	if err != nil {
		return err
	}
	for _, x := range xs {
		s.Assignments = append(s.Assignments, recommendation.PendingAssignment{ID: x.AssignmentID, Title: x.Title, SubjectID: x.SubjectID,
			ObjectiveIDs: x.ObjectiveIDs, DueAt: x.DueAt})
	}
	return nil
}

// ExamFollowupWindow is how far back R5 looks at released exam results (two weeks) [Proposed].
const ExamFollowupWindow = 14 * 24 * time.Hour

// ExamFollowups is the SnapshotSource of F042: the objective results of the student's released exams (rule R5).
type ExamFollowups struct {
	Store write.ExamStore
	Clock func() time.Time
}

// Fill implements SnapshotSource.
func (u ExamFollowups) Fill(ctx context.Context, studentID uuid.UUID, s *recommendation.Snapshot) error {
	xs, err := u.Store.ExamObjectiveResults(ctx, studentID, now(u.Clock).Add(-ExamFollowupWindow))
	if err != nil {
		return err
	}
	for _, x := range xs {
		s.ExamResults = append(s.ExamResults, recommendation.ExamObjectiveResult{ObjectiveID: x.ObjectiveID, Correct: x.Correct, Total: x.Total})
	}
	return nil
}
