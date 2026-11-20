package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
)

// CloneJob is one row of platform.tenant_clone_jobs (F045).
type CloneJob struct {
	ID             uuid.UUID
	Source, Target uuid.UUID
	ResourceType   string
	ResourceIDs    []uuid.UUID
	Status         string
	SagaID         uuid.UUID
	RequestedBy    uuid.UUID
	IdempotencyKey string
	CreatedAt      time.Time
}

// OpenCloneJob is a queued/running job with the status of its saga (job platform-saga finalizes it).
type OpenCloneJob struct {
	CloneJob
	SagaStatus string
	SagaError  string
	Steps      []SagaStep
}

// DefaultAssignment is one row of platform.tenant_default_assignments.
type DefaultAssignment struct {
	ID           uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	OwnerModule  string
	CreatedBy    uuid.UUID
}

// UserMerge is one row of identity.user_merges (append-only).
type UserMerge struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	From, Into uuid.UUID
	SchoolID   *uuid.UUID
	Reason     string
	MergedBy   uuid.UUID
	MergedAt   time.Time
	SagaID     uuid.UUID
	IssueID    *uuid.UUID
}

// MergeTransfer counts what the identity step of a merge moved.
type MergeTransfer struct {
	Identities, IdentitiesSuperseded, SisLinks, SisLinksSuperseded int
	// MovedEnrollments are the active enrollments of the source moved to the target, which had none (OQ F045-6);
	// EndedEnrollments the active ones of the source ended because the target keeps its own.
	MovedEnrollments, EndedEnrollments []MergedEnrollment
	// StudentRemoved reports that the student record (org.students) of the source was soft-deleted (merged).
	StudentRemoved bool
}

// MergedEnrollment is an enrollment touched by the identity step of a merge.
type MergedEnrollment struct {
	EnrollmentID, SchoolID, ClassID uuid.UUID
	StartDate                       time.Time
	Source                          string
	Version                         int64
}

// PlatformAdminRepository persists the F045 tables with explicit SQL in a platform scope (identity_platform;
// tenant columns are always explicit conditions, AGENTS R8).
type PlatformAdminRepository interface {
	// InactiveTenants returns the ids that are not live, active tenants (missing, disabled or deleted).
	InactiveTenants(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
	// ActiveTenantIDs returns every live, active tenant (apply_to_all_tenants).
	ActiveTenantIDs(ctx context.Context) ([]uuid.UUID, error)

	// CreateCloneJob inserts the job (queued).
	CreateCloneJob(ctx context.Context, j CloneJob) error
	// CloneJobBySaga returns the job of a saga (nil when none).
	CloneJobBySaga(ctx context.Context, sagaID uuid.UUID) (*CloneJob, error)
	// OpenCloneJobs returns queued/running jobs with their saga, oldest first.
	OpenCloneJobs(ctx context.Context, limit int) ([]OpenCloneJob, error)
	// MarkCloneRunning moves a queued job to running.
	MarkCloneRunning(ctx context.Context, jobID uuid.UUID) error
	// CloneMap returns the id_map of a job (source → copy of the steps run so far).
	CloneMap(ctx context.Context, jobID uuid.UUID) ([]platformadmin.ClonePair, error)
	// AppendCloneMap adds the pairs of a step to the id_map once (a replayed step adds nothing) and moves the
	// job to running.
	AppendCloneMap(ctx context.Context, jobID uuid.UUID, pairs []platformadmin.ClonePair) error
	// FinishCloneJob writes the final state once; finished is false when the job was already final.
	FinishCloneJob(ctx context.Context, jobID uuid.UUID, res platformadmin.CloneResult, at time.Time) (finished bool, err error)

	// AddDefaultAssignment stores a default assignment (no-op when the resource already has one).
	AddDefaultAssignment(ctx context.Context, d DefaultAssignment) error
	// TenantSchools returns the live schools of a tenant (identity clone step of school_settings, OQ F045-4).
	TenantSchools(ctx context.Context, tenant uuid.UUID) ([]uuid.UUID, error)
	// RemoveDefaultAssignment soft-deletes the live default assignment of a resource (OQ F045-9); false when none.
	RemoveDefaultAssignment(ctx context.Context, resourceType string, resourceID, by uuid.UUID, at time.Time) (bool, error)
	// DefaultAssignments returns the live default assignments.
	DefaultAssignments(ctx context.Context) ([]DefaultAssignment, error)

	// MergeAccount returns the account and whether it has a membership in the tenant (nil when unknown).
	MergeAccount(ctx context.Context, userID, tenantID uuid.UUID) (acc *platformadmin.MergeAccount, member bool, err error)
	// MergeIssue returns the reconciliation issue of the tenant (nil when absent) and its school.
	MergeIssue(ctx context.Context, issueID, tenantID uuid.UUID) (*platformadmin.MergeIssue, *uuid.UUID, error)
	// MergeByFrom returns the merge whose source is the user (nil when none).
	MergeByFrom(ctx context.Context, from uuid.UUID) (*UserMerge, error)
	// MergeBySaga returns the merge of a saga (nil when none).
	MergeBySaga(ctx context.Context, sagaID uuid.UUID) (*UserMerge, error)
	// CreateMerge inserts the merge row (UNIQUE from_user_id: a concurrent duplicate is 409 CONFLICT).
	CreateMerge(ctx context.Context, m UserMerge) error
	// TransferIdentities is the identity step of a merge: moves the external identities and SIS links of
	// m.From to m.Into (superseding the ones whose issuer/link m.Into already holds), moves the active enrollment
	// of m.From to m.Into when m.Into has none (else ends it), soft-deletes the student record and the account of
	// m.From (delete_reason merged) and closes the issue (resolution merged). Idempotent: a second call does nothing.
	TransferIdentities(ctx context.Context, m UserMerge, at time.Time) (MergeTransfer, error)
}
