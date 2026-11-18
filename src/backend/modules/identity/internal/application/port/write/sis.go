package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SyncRun is one SIS import/sync run (sis.sync_runs).
type SyncRun struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	ConnectionID uuid.UUID
	SchoolID     *uuid.UUID
	Mode         string
	DryRun       bool
	Status       string
	FailureCode  *string
	Stats        map[string]int
	StartedAt    time.Time
	FinishedAt   *time.Time
}

// StagingItem is one normalized record of a run with its decided action.
type StagingItem struct {
	Kind, ExternalID, Hash, Action, Status string
	Row                                    int
	Payload                                map[string]any
}

// RecordError is one contract violation of a record.
type RecordError struct {
	Kind, ExternalID, Column, Code, Message string
	Row                                     int
}

// SyncedStudent is the SIS record of a student (sis.synced_students) and its user.
type SyncedStudent struct {
	ConnectionID   uuid.UUID
	ExternalID     string
	SchoolID       uuid.UUID
	StudentCode    string
	RecordStatus   string
	NationalIDHash *string
	Hash           string
	MissingRuns    int
	UserID         *uuid.UUID // from the active identity link
}

// Issue is a reconciliation issue to raise.
type Issue struct {
	SchoolID     *uuid.UUID
	ConnectionID uuid.UUID
	RunID        uuid.UUID
	Type         string
	Refs         map[string]any
	Suggested    string
}

// NewSisStudent is a student created by a SIS apply: user pending + org.students + identity link.
type NewSisStudent struct {
	UserID       uuid.UUID
	Username     string
	DisplayName  string
	TenantID     uuid.UUID
	SchoolID     uuid.UUID
	Grade        int
	StudentCode  string
	ConnectionID uuid.UUID
	ExternalID   string
}

// SisSyncRepository is the persistence of the SIS pipeline (sis.*, org.*, identity.* of module identity).
type SisSyncRepository interface {
	CreateRun(ctx context.Context, r *SyncRun) error
	FinishRun(ctx context.Context, r *SyncRun) error
	AddErrors(ctx context.Context, runID uuid.UUID, errs []RecordError) error
	AddStaging(ctx context.Context, runID uuid.UUID, items []StagingItem) error

	// LinkOf returns the EduWorld id and last hash of a school/class/enrollment source key.
	LinkOf(ctx context.Context, connectionID uuid.UUID, kind, externalID string) (id uuid.UUID, hash string, found bool, err error)
	SaveLink(ctx context.Context, connectionID uuid.UUID, kind, externalID string, entityID uuid.UUID, schoolID *uuid.UUID, hash string) error

	SyncedStudentOf(ctx context.Context, connectionID uuid.UUID, externalID string) (*SyncedStudent, error) // nil when absent
	SaveSyncedStudent(ctx context.Context, s SyncedStudent) error
	// StudentsOfSchool lists the synced students of a school (missing_in_source detection).
	StudentsOfSchool(ctx context.Context, connectionID, schoolID uuid.UUID) ([]SyncedStudent, error)

	// UsernameOwner returns the user owning lower(username), soft-deleted included, and its delete reason.
	UsernameOwner(ctx context.Context, username string) (userID *uuid.UUID, deleteReason *string, err error)
	CreateSisStudent(ctx context.Context, s NewSisStudent) error
	UpdateStudent(ctx context.Context, userID uuid.UUID, grade int, studentCode, fullName string) error
	// SetUserStatus sets the status and returns the previous one (changed = from != status).
	SetUserStatus(ctx context.Context, userID uuid.UUID, status string) (from string, changed bool, err error)
	SetLoginKey(ctx context.Context, connectionID uuid.UUID, externalID, username string) error

	// AcademicYear finds or creates the academic year of a school by name (YYYY-YYYY).
	AcademicYear(ctx context.Context, schoolID uuid.UUID, name string) (uuid.UUID, error)
	CreateClass(ctx context.Context, id, schoolID, yearID uuid.UUID, grade int, name string) error
	UpdateClass(ctx context.Context, id uuid.UUID, grade int, name string) error
	ClassSchool(ctx context.Context, classID uuid.UUID) (schoolID uuid.UUID, grade int, err error)

	RaiseIssue(ctx context.Context, i Issue) (uuid.UUID, error)
}
