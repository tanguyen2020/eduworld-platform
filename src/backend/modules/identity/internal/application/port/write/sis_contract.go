package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
)

// ContractConnectionRow is a SIS connection with its F017 columns. Secret references are loaded for the
// worker only; they never leave the application layer (F017-AC-04).
type ContractConnectionRow struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ProviderKind     string
	ContractVersion  string
	Config           sis.ContractConfig
	SecretRef        string
	WebhookSecretRef string
	Schedule         string
	Capabilities     []string
	State            sis.ConnectionState
	DeltaWatermark   string
	LastHealthAt     *time.Time
	LastSuccessAt    *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

// SisContractRepository persists contract_v1 connections and the run bookkeeping of F017 (sis.*, T).
type SisContractRepository interface {
	// CreateContract inserts a contract_v1 connection.
	CreateContract(ctx context.Context, c *ContractConnectionRow) error
	// GetContract loads a connection of the scope (problem NotFound otherwise).
	GetContract(ctx context.Context, id uuid.UUID) (*ContractConnectionRow, error)
	// SaveContract writes config, references, schedule, capabilities, state, watermark and times
	// where version = c.Version (problem VersionMismatch otherwise) and bumps c.Version.
	SaveContract(ctx context.Context, c *ContractConnectionRow) error
	// PendingPullRun reports a queued/running pull run (full, delta, webhook) of the connection started
	// after since (older ones are stale: the worker died).
	PendingPullRun(ctx context.Context, connectionID uuid.UUID, since time.Time) (bool, error)
	// GetRun loads a run of the scope (nil when absent).
	GetRun(ctx context.Context, id uuid.UUID) (*SyncRun, error)
	// StartRun moves a queued run to running.
	StartRun(ctx context.Context, id uuid.UUID) error
	// ActiveContractConnections lists active contract_v1 connections of every tenant (scheduler,
	// platform_job bypass).
	ActiveContractConnections(ctx context.Context) ([]ContractConnectionRow, error)
	// ActiveSchoolIDs returns the schools currently served by the connection.
	ActiveSchoolIDs(ctx context.Context, connectionID uuid.UUID) (map[uuid.UUID]bool, error)
	// TenantOfAny returns the tenant of a connection under a bypass (webhook route, scheduler).
	TenantOfAny(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}
