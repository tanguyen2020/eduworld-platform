package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// AssignmentScope is a (school, subject) pair a grant of the reader covers; nil = any.
type AssignmentScope struct {
	SchoolID  *uuid.UUID
	SubjectID *uuid.UUID
}

// AssignmentFilter filters listAssignments (F041): the scopes of the reader, school, subject, targeted grade (ADR-0095),
// status, keyset.
type AssignmentFilter struct {
	Scopes    []AssignmentScope
	SchoolID  *uuid.UUID
	SubjectID *uuid.UUID
	Grade     *int
	Status    string
	After     *uuid.UUID
	Limit     int
}

// AssignmentRecord is an assignment with its subject (authorization of the reads).
type AssignmentRecord struct {
	View      dto.AssignmentView
	SubjectID uuid.UUID
}

// SubmissionFilter filters listAssignmentSubmissions.
type SubmissionFilter struct {
	AssignmentID uuid.UUID
	Status       string
	After        *uuid.UUID
	Limit        int
}

// MyAssignmentRecord is an assignment of a student with its submission (listMyAssignments).
type MyAssignmentRecord struct {
	Assignment       dto.AssignmentView
	SubjectID        uuid.UUID
	SubjectName      json.RawMessage
	Submission       string
	AttemptsUsed     int
	CurrentAttemptID *uuid.UUID
	DueAtOverride    *time.Time
}

// GameActivityFilter filters listGameActivities.
type GameActivityFilter struct {
	Grade        *int
	ObjectiveIDs []uuid.UUID
	After        *uuid.UUID
	Limit        int
}

// SchoolAssignmentReadModel reads the assignments of F041 in the read transaction of ctx (TenantScope: RLS).
type SchoolAssignmentReadModel interface {
	Assignments(ctx context.Context, f AssignmentFilter) ([]AssignmentRecord, error)
	Assignment(ctx context.Context, id uuid.UUID) (*AssignmentRecord, error)
	Completion(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]dto.AssignmentCompletionView, error)
	Submissions(ctx context.Context, f SubmissionFilter) ([]dto.AssignmentSubmissionView, error)
	MyAssignments(ctx context.Context, studentID uuid.UUID, after *uuid.UUID, limit int) ([]MyAssignmentRecord, error)
	// Release is the result release and status of an assignment (ok = false when invisible).
	Release(ctx context.Context, id uuid.UUID) (release, status string, ok bool, err error)
	GameActivities(ctx context.Context, f GameActivityFilter) ([]dto.GameActivityView, error)
}

// AssignmentRecipientReadModel reads the students who receive an assignment (F123, facade listAssignmentRecipients) in the
// read transaction of ctx (TenantScope: RLS).
type AssignmentRecipientReadModel interface {
	// Recipients returns the title of the assignment (ok = false when it does not exist in the session tenant) and up to
	// limit+1 targeted students after the given student id (ordered by student id), minus the students with a live excuse.
	Recipients(ctx context.Context, assignmentID uuid.UUID, after *uuid.UUID, limit int) (title string, ok bool, ids []uuid.UUID, err error)
}
