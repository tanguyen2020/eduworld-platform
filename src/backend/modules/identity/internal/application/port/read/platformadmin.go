package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// SagaStepView is one step of a saga as the queries show it.
type SagaStepView struct {
	No       int
	Service  string
	Action   string
	Status   string
	Response json.RawMessage
	Error    string
}

// CloneJobView is a clone job with its saga (getTenantClone).
type CloneJobView struct {
	JobID          uuid.UUID
	Source, Target uuid.UUID
	ResourceType   string
	Status         string
	IDMap          json.RawMessage
	Errors         []string
	RequestedBy    uuid.UUID
	CreatedAt      time.Time
	FinishedAt     *time.Time
	SagaStatus     string
	SagaError      string
	Steps          []SagaStepView
}

// UserMergeView is a merge with its saga (getUserMerge).
type UserMergeView struct {
	MergeID, SagaID uuid.UUID
	TenantID        uuid.UUID
	From, Into      uuid.UUID
	MergedAt        time.Time
	SagaStatus      string
	SagaError       string
	Steps           []SagaStepView
}

// PlatformAdminReadModel reads the F045 tables and the identity-owned trash in a platform scope or under
// tenancy.Bypass(trash) (explicit tenant and soft-delete conditions, AGENTS R27).
type PlatformAdminReadModel interface {
	// ActiveTenantIDs returns every live, active tenant (tenant_ids empty = every active tenant, F045-AC-01).
	ActiveTenantIDs(ctx context.Context) ([]uuid.UUID, error)
	// CloneJob returns a job (nil when absent).
	CloneJob(ctx context.Context, id uuid.UUID) (*CloneJobView, error)
	// UserMerge returns a merge (nil when absent).
	UserMerge(ctx context.Context, id uuid.UUID) (*UserMergeView, error)
	// DeletedAccounts lists the soft-deleted accounts of the tenants for the trash type rt (user: merged accounts,
	// never restorable here; student: students of the inactive cleanup, restored by the F016 saga), newest first
	// after the (deleted_at, id) cursor, up to limit+1 rows.
	DeletedAccounts(ctx context.Context, rt string, tenants []uuid.UUID, afterAt *time.Time, afterID uuid.UUID, limit int) ([]port.TrashEntry, error)
}
