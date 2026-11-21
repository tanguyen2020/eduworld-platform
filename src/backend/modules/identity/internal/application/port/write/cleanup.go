package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CleanupCandidate is a student the inactive cleanup may remove (ADR-0032).
type CleanupCandidate struct {
	StudentID     uuid.UUID
	SchoolID      uuid.UUID
	DisplayName   string
	LastLoginAt   *time.Time
	ProvisionedAt time.Time
}

// DueAt is when the student becomes due for the cleanup with days.
func (c CleanupCandidate) DueAt(days int) time.Time {
	base := c.ProvisionedAt
	if c.LastLoginAt != nil {
		base = *c.LastLoginAt
	}
	return base.AddDate(0, 0, days)
}

// CleanupRun is one row of platform.student_cleanup_runs.
type CleanupRun struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Mode          string
	Status        string
	CutoffDays    int
	Candidates    int
	Cleaned       int
	Excluded      int
	SkippedActive int
	ApprovedBy    *uuid.UUID
	StartedAt     time.Time
	FinishedAt    *time.Time
	Version       int64
}

// CleanupExclusion is one live row of platform.cleanup_exclusions.
type CleanupExclusion struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	StudentID uuid.UUID
	SchoolID  uuid.UUID
	Until     *time.Time
	Reason    string
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

// CleanupRepository serves the inactive-student cleanup (F016 US3/US4). Tenant methods run under the
// TenantScope of the tenant (RLS); run methods under the platform scope of the job.
type CleanupRepository interface {
	// Candidates returns the students of the tenant scope whose COALESCE(last_login_at, provisioned_at) is
	// before dueBefore, active or pending, without a live exclusion at now (school limits to one school).
	Candidates(ctx context.Context, dueBefore, now time.Time, school *uuid.UUID) ([]CleanupCandidate, error)
	// StudentCount counts the live students of the tenant scope.
	StudentCount(ctx context.Context) (int, error)
	// SoftDeleteStudent soft-deletes the user and the student (delete_reason inactive_cleanup; the student loses its
	// effective school and grade — ADR-0095); false when the student was already removed.
	SoftDeleteStudent(ctx context.Context, studentID uuid.UUID) (bool, error)
	// RestoreStudent clears the inactive_cleanup soft-delete of the user and the student; returns the
	// time of the cleanup (nil when nothing to restore).
	RestoreStudent(ctx context.Context, studentID uuid.UUID) (*time.Time, error)
	// CleanedStudent returns the tenant and cleanup time of a student removed by the cleanup (platform scope).
	CleanedStudent(ctx context.Context, studentID uuid.UUID) (tenant *uuid.UUID, cleanedAt *time.Time, err error)
	// SisStanding reports whether the student has an active SIS identity link and, if so, whether its SIS record
	// (sis.synced_students) is live and active (F016-AC-17, AC-18; platform scope).
	SisStanding(ctx context.Context, studentID uuid.UUID) (linked, valid bool, err error)

	CreateRun(ctx context.Context, r *CleanupRun) error
	SaveRun(ctx context.Context, r *CleanupRun) error
	Run(ctx context.Context, id uuid.UUID) (*CleanupRun, error)
	// ApprovedRun returns the approved run of a tenant waiting to be applied (nil when none).
	ApprovedRun(ctx context.Context, tenant uuid.UUID) (*CleanupRun, error)
	ListRuns(ctx context.Context, tenants []uuid.UUID, status string, limit int) ([]CleanupRun, error)
	// CleanupTenants lists the active tenants (the job iterates them).
	CleanupTenants(ctx context.Context) ([]uuid.UUID, error)

	AddExclusion(ctx context.Context, e CleanupExclusion) (bool, error)
	RemoveExclusion(ctx context.Context, studentID, by uuid.UUID) (bool, error)
	StudentSchool(ctx context.Context, studentID uuid.UUID) (*uuid.UUID, error)
}
