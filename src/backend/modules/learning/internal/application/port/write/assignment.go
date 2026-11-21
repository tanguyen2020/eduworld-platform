package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AssignmentRow is one row of lms.assignments (F041).
type AssignmentRow struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	SchoolID        uuid.UUID
	SubjectID       uuid.UUID
	CreatedBy       uuid.UUID
	CreatedByRole   string
	Title           string
	Instructions    *string
	ContentType     string
	ContentID       uuid.UUID
	PinnedVersionID *uuid.UUID
	FollowLatest    bool
	ActivityType    *string
	Mode            string
	OpenAt, DueAt   time.Time
	AllowLate       bool
	LateHours       int
	MaxAttempts     int
	ScorePolicy     string
	FeedbackTiming  string
	ResultRelease   string
	Status          string
	ExamSessionID   *uuid.UUID
	ObjectiveIDs    []uuid.UUID
	PublishedAt     *time.Time
	ClosedAt        *time.Time
	ReleasedAt      *time.Time
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	UpdatedBy       *uuid.UUID
	Targets         []TargetRow
}

// TargetRow is one row of lms.assignment_targets (ADR-0095 mục 3): a grade of the school of the assignment with included
// students of another grade of the school and excluded students, or a list of students of the school.
type TargetRow struct {
	Type       string
	Grade      *int
	Include    []uuid.UUID
	Exclude    []uuid.UUID
	StudentIDs []uuid.UUID
	// StudentCount is the number of students of the target in the snapshot (filled when read after publish).
	StudentCount int
}

// TargetStudent is one student of the snapshot of an assignment.
type TargetStudent struct {
	StudentID uuid.UUID
	Grade     *int   // grade of the student when targeted (ADR-0095: recorded at event time)
	Source    string // publish, grade_added (enrollment_added: history before ADR-0095)
}

// SubmissionRow is one row of lms.assignment_submissions.
type SubmissionRow struct {
	AssignmentID     uuid.UUID
	TenantID         uuid.UUID
	SchoolID         uuid.UUID
	StudentID        uuid.UUID
	Status           string
	AttemptsUsed     int
	CurrentAttemptID *uuid.UUID
	BestAttemptID    *uuid.UUID
	Score, MaxScore  *float64
	SubmittedAt      *time.Time
	RapidGuessCount  int
	DueAtOverride    *time.Time
	CompletedAt      *time.Time
	Version          int64
}

// GradedAttempt is a graded attempt of a student in the context of an assignment (score policy of the submission).
type GradedAttempt struct {
	AttemptID       uuid.UUID
	Score, MaxScore float64
	SubmittedAt     time.Time
	RapidGuesses    int
}

// AttemptOfContext is an attempt of a student in the context of an assignment.
type AttemptOfContext struct {
	AttemptID   uuid.UUID
	Status      string
	StartedAt   time.Time
	SubmittedAt *time.Time
}

// DueAssignment is an assignment the scheduler opens or closes.
type DueAssignment struct {
	ID       uuid.UUID
	TenantID uuid.UUID
}

// SchoolAssignmentStore persists the assignments of F041 in the TenantScope of ctx (RLS T, soft-delete only) and reads what
// its commands check (the students of the projection ref.students, subjects and content versions visible to the tenant).
type SchoolAssignmentStore interface {
	InsertAssignment(ctx context.Context, a AssignmentRow) error
	// Assignment reads an assignment with its targets; ForUpdate locks the row in the transaction.
	Assignment(ctx context.Context, id uuid.UUID, forUpdate bool) (*AssignmentRow, error)
	// UpdateAssignment saves every column (and replaces the targets when replaceTargets) if the row version is still
	// ifVersion; it returns false otherwise.
	UpdateAssignment(ctx context.Context, a AssignmentRow, ifVersion int64, replaceTargets bool) (bool, error)

	// Snapshot of the students targeted (F041-AC-08, AC-16).
	InsertTargetStudents(ctx context.Context, a AssignmentRow, students []TargetStudent) (int, error)
	TargetStudents(ctx context.Context, assignmentID uuid.UUID) ([]TargetStudent, error)
	Targeted(ctx context.Context, assignmentID, studentID uuid.UUID) (bool, error)
	// OpenGradeAssignments lists the open assignments with a grade target of the grade of the school (a student joins the
	// grade, ADR-0095 mục 3).
	OpenGradeAssignments(ctx context.Context, schoolID uuid.UUID, grade int) ([]AssignmentRow, error)
	// OpenAssignmentsOfStudent lists the open assignments a student is targeted by.
	OpenAssignmentsOfStudent(ctx context.Context, studentID uuid.UUID) ([]AssignmentRow, error)

	// Submissions.
	Submission(ctx context.Context, assignmentID, studentID uuid.UUID, forUpdate bool) (*SubmissionRow, error)
	UpsertSubmission(ctx context.Context, s SubmissionRow) error
	HasSubmission(ctx context.Context, assignmentID uuid.UUID) (bool, error)
	// MarkMissed sets missed on every submission of the assignment not submitted, late nor excused whose personal due
	// date (plus the late window) is past; it returns the number of rows.
	MarkMissed(ctx context.Context, assignmentID uuid.UUID, now time.Time, late time.Duration) (int, error)
	// GradedAttempts lists the graded attempts of a student in the context of the assignment.
	GradedAttempts(ctx context.Context, assignmentID, studentID uuid.UUID) ([]GradedAttempt, error)
	// AttemptsOfContext lists every attempt of a student in the context of the assignment, newest first.
	AttemptsOfContext(ctx context.Context, assignmentID, studentID uuid.UUID) ([]AttemptOfContext, error)
	// AttemptContext returns the context (type, id) of an attempt.
	AttemptContext(ctx context.Context, attemptID uuid.UUID) (string, uuid.UUID, uuid.UUID, error)

	// Exceptions (F041-AC-22, AC-23).
	SetException(ctx context.Context, tenant, school, assignmentID, studentID uuid.UUID, kind string, due *time.Time, reason string, by uuid.UUID) error

	// Reminders (F041-AC-24): the reminders of the day (ICT date) and a new one.
	RemindersOn(ctx context.Context, assignmentID uuid.UUID, day string) (int, error)
	InsertReminder(ctx context.Context, tenant, school, assignmentID uuid.UUID, day string, recipients int, by uuid.UUID) error
	// PendingStudents lists the targeted students who did not submit and are not excused.
	PendingStudents(ctx context.Context, assignmentID uuid.UUID) ([]uuid.UUID, error)

	// Scheduler (every tenant under tenancy.Bypass(platform_job)).
	DueToOpen(ctx context.Context, now time.Time, limit int) ([]DueAssignment, error)
	DueToClose(ctx context.Context, now time.Time, limit int) ([]DueAssignment, error)

	// References.
	// SchoolOfTenant reports whether a school is a school of the session tenant (ref.schools).
	SchoolOfTenant(ctx context.Context, schoolID uuid.UUID) (bool, error)
	// SubjectVisible reports whether a subject is visible to the tenant (its own or a platform subject).
	SubjectVisible(ctx context.Context, subjectID uuid.UUID) (bool, error)
	// GradeStudents lists the active students of a grade of a school (ref.students, ordered by id).
	GradeStudents(ctx context.Context, schoolID uuid.UUID, grade int) ([]uuid.UUID, error)
	// ActiveStudents keeps the listed students who are active students of the school.
	ActiveStudents(ctx context.Context, schoolID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error)
	// Placement is the school and grade of an active student (nil when the student is not active in the tenant).
	Placement(ctx context.Context, studentID uuid.UUID) (*StudentPlacement, error)
	// CurrentVersion is the current published version of an item visible to the tenant (nil when absent).
	CurrentVersion(ctx context.Context, contentType string, itemID uuid.UUID) (*VersionAccess, uuid.UUID, error)
	ContentVersion(ctx context.Context, contentType string, versionID uuid.UUID) (*VersionAccess, error)
	// ActivityType is the activity type of an activity version.
	ActivityType(ctx context.Context, versionID uuid.UUID) (string, error)
	// PendingAssignments lists the open assignments a student has not submitted (not excused) with the personal due
	// date (rule R1).
	PendingAssignments(ctx context.Context, studentID uuid.UUID) ([]PendingAssignment, error)
	// GameActivity is the catalog entry of a quest or mini-game for the tenant (nil when absent).
	GameActivity(ctx context.Context, itemType string, itemID uuid.UUID) (*GameActivityRow, error)
}

// StudentPlacement is the school and grade of an active student of ref.students.
type StudentPlacement struct {
	SchoolID uuid.UUID
	Grade    int
}

// GameActivityRow is a row of ref.game_activities.
type GameActivityRow struct {
	ItemType         string
	ItemID           uuid.UUID
	CurrentVersionID uuid.UUID
	Status           string
	ObjectiveIDs     []uuid.UUID
}

// PendingAssignment is an open assignment a student has not submitted yet.
type PendingAssignment struct {
	AssignmentID uuid.UUID
	Title        string
	SubjectID    uuid.UUID
	ObjectiveIDs []uuid.UUID
	DueAt        time.Time
}
