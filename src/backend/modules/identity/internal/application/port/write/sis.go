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
	Trigger      string     // F017: schedule, manual (default), webhook, fallback_full
	RequestedBy  *uuid.UUID // F017: the person of a manual run
	DeliveryID   *uuid.UUID // F017: the webhook delivery of a webhook run
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
	RunID        uuid.UUID // uuid.Nil outside a run (provisioning)
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

// PendingStudent is a SIS student waiting for its Keycloak account, with its active SIS link.
type PendingStudent struct {
	UserID       uuid.UUID
	Username     string
	DisplayName  string
	Status       string
	TenantID     uuid.UUID
	SchoolID     uuid.UUID
	Grade        int
	ConnectionID uuid.UUID
	ExternalID   string
	Subject      *string // subject of the local issuer when already provisioned
}

// ProvisioningRepository is the persistence of the keycloak-provisioner (ACCOUNT_PROVISIONING §2).
type ProvisioningRepository interface {
	// PendingStudent returns nil when the user no longer exists.
	PendingStudent(ctx context.Context, userID uuid.UUID, issuer string) (*PendingStudent, error)
	// LinkIdentity inserts external_identities(issuer, subject) and sets users.provisioned_at.
	LinkIdentity(ctx context.Context, userID uuid.UUID, issuer, subject string) error
}

// IssueRow is a reconciliation issue loaded for resolution.
type IssueRow struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	SchoolID     *uuid.UUID
	ConnectionID *uuid.UUID
	Type         string
	Refs         map[string]any
	Status       string
	Resolution   *string
	Note         *string
	ResolvedBy   *uuid.UUID
	ResolvedAt   *time.Time
	Suggested    *string
	CreatedAt    time.Time
}

// SisIssueRepository resolves reconciliation issues (F011-AC-11, AC-17).
type SisIssueRepository interface {
	// LockIssue loads the issue (problem NotFound outside the scope).
	LockIssue(ctx context.Context, id uuid.UUID) (*IssueRow, error)
	// SaveResolution applies only while the issue is open (problem Conflict otherwise).
	SaveResolution(ctx context.Context, i *IssueRow) error
	// StudentOf reports whether userID is a live student of the scope tenant and whether it has an active SIS link.
	StudentOf(ctx context.Context, userID uuid.UUID) (found, linked bool, err error)
	// LinkExisting links an existing student to a SIS source key and records the synced student (empty hash so
	// the next import updates it).
	LinkExisting(ctx context.Context, userID, tenantID, schoolID, connectionID uuid.UUID, externalID string) error
}

// MissingTracker records students missing from full imports (F011-AC-24).
type MissingTracker interface {
	// MarkMissing increments missing_runs of a synced student and returns the new value.
	MarkMissing(ctx context.Context, connectionID uuid.UUID, externalID string) (int, error)
	// ResetMissing sets missing_runs to 0 (the student is back in the source).
	ResetMissing(ctx context.Context, connectionID uuid.UUID, externalID string) error
	// PreviousRunStats returns the stats of the last finished apply run of the school before runID.
	PreviousRunStats(ctx context.Context, connectionID, schoolID, runID uuid.UUID) (map[string]int, bool, error)
}

// TransferCandidate is a synced student of another SIS connection with the same national id hash.
type TransferCandidate struct {
	TenantID     uuid.UUID
	SchoolID     uuid.UUID
	ConnectionID uuid.UUID
	ExternalID   string
	UserID       *uuid.UUID
}

// TransferMatcher finds and moves SIS links between connections (F011-AC-16, ADR-0046 §6).
type TransferMatcher interface {
	// TransferCandidates searches every tenant (platform_job bypass): national id hash → candidate.
	TransferCandidates(ctx context.Context, hashes []string, excludeConnection uuid.UUID) (map[string]TransferCandidate, error)
	// SupersedeLink marks the active link of a source key superseded.
	SupersedeLink(ctx context.Context, connectionID uuid.UUID, externalID string) error
	LinkExisting(ctx context.Context, userID, tenantID, schoolID, connectionID uuid.UUID, externalID string) error
}

// LoginState is what login sync needs to choose its branch (SIS_INTEGRATION §4.5).
type LoginState struct {
	UserID           uuid.UUID
	TenantID         uuid.UUID
	SchoolID         uuid.UUID
	ConnectionID     uuid.UUID
	ExternalID       string
	SchoolExternalID string // "" when the school has no source key yet
	Username         string
	LastSyncedAt     *time.Time
	ActiveEnrollment bool // active enrollment with school and class
	StudentLookup    bool // the connection is active and declares the student_lookup capability
}

// LoginStateReader loads the login sync state of a SIS student (nil when the user has no active SIS link).
type LoginStateReader interface {
	LoginState(ctx context.Context, userID uuid.UUID) (*LoginState, error)
	// LoginStateBySource loads the state of a source key (login_refresh jobs).
	LoginStateBySource(ctx context.Context, connectionID uuid.UUID, externalID string) (*LoginState, error)
}

// SisImportRow is a CSV import of a school (sis.sis_imports).
type SisImportRow struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	SchoolID     uuid.UUID
	ConnectionID uuid.UUID // the csv_import connection serving the school (not stored; resolved on load)
	RequestedBy  uuid.UUID
	StorageKey   string
	SHA256       *string
	Size         int64
	Kinds        []string // one entity kind of the API (students, classes, enrollments, schools)
	Status       string
	DryRun       bool
	RunID        *uuid.UUID
	Stats        map[string]int
	CreatedAt    time.Time
	Version      int64
}

// SisImportRepository stores CSV imports (F011 T014).
type SisImportRepository interface {
	CreateImport(ctx context.Context, r *SisImportRow) error
	// GetImport loads an import of the scope (problem NotFound otherwise).
	GetImport(ctx context.Context, id uuid.UUID) (*SisImportRow, error)
	SaveImport(ctx context.Context, r *SisImportRow) error
	// AwaitingUploads lists imports waiting for their file, oldest first (platform_job scope).
	AwaitingUploads(ctx context.Context, limit int) ([]SisImportRow, error)
	// CsvConnectionOf returns the active csv_import connection serving the school (problem NotFound otherwise).
	CsvConnectionOf(ctx context.Context, schoolID uuid.UUID) (uuid.UUID, error)
}

// StagingAnonymizer clears staged payloads past their retention (no DELETE, R21).
type StagingAnonymizer interface {
	// AnonymizeStagingBefore sets payload = NULL on up to limit staging records created before cutoff.
	AnonymizeStagingBefore(ctx context.Context, cutoff time.Time, limit int) (int, error)
}

// ConflictRules are the lookups of the conflict matrix (SIS_INTEGRATION §6.3).
type ConflictRules interface {
	// StudentByCode returns a live student of the school with this student_code (possible_duplicate).
	StudentByCode(ctx context.Context, schoolID uuid.UUID, code string) (*uuid.UUID, error)
	// ProvinceKnown reports whether a province code exists in the catalog (any status: old codes map through
	// successions).
	ProvinceKnown(ctx context.Context, code string) (bool, error)
	// DeletedUser reports whether a user is soft-deleted and why (the ent filter hides it otherwise).
	DeletedUser(ctx context.Context, userID uuid.UUID) (deleted bool, reason *string, err error)
}
