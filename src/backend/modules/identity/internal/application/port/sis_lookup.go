package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors of SisLookupPort.
var (
	// ErrSisLookupNotFound: the SIS has no student with this login (404).
	ErrSisLookupNotFound = errors.New("sis lookup: student not found")
	// ErrSisLookupTimeout: no answer within sis_login_sync_timeout_ms.
	ErrSisLookupTimeout = errors.New("sis lookup: timeout")
)

// SisLookupPort is the blocking student lookup of a SIS connection (lookupStudent of the SIS contract,
// SIS_INTEGRATION §4.5). The adapter of contract_v1 is delivered by F017; it returns one Student record
// already validated against the contract DTO (unknown fields dropped). Any other error is a SIS failure.
type SisLookupPort interface {
	Lookup(ctx context.Context, connectionID uuid.UUID, schoolExternalID, loginUsername string) (map[string]any, error)
}

// SisJobQueue enqueues SIS jobs on the JetStream work-queue IDENTITY_SIS_JOBS.
type SisJobQueue interface {
	// EnqueueLoginRefresh queues a login_refresh job deduplicated by Nats-Msg-Id
	// student:<connection>:<external_id> within the stream duplicate window (10 minutes).
	EnqueueLoginRefresh(ctx context.Context, job LoginRefreshJob) error
}

// LoginRefreshJob is the payload of a login_refresh job (subject ew.job.identity.sis.login_refresh).
type LoginRefreshJob struct {
	TenantID          uuid.UUID `json:"tenant_id"`
	ConnectionID      uuid.UUID `json:"sis_connection_id"`
	StudentExternalID string    `json:"student_external_id"`
}

// SyncRunObserver is told when a sync run ends (eduworld_sis_last_success_timestamp, SIS_INTEGRATION §8).
type SyncRunObserver interface {
	RunFinished(connectionID uuid.UUID, mode, status string, at time.Time)
}

// LoginSyncMetrics observes eduworld_sis_login_sync_total{branch,result} and its duration.
type LoginSyncMetrics interface {
	Observe(branch, result string, d time.Duration)
}

// CsvImportJob is the payload of a csv_import job (subject ew.job.identity.sis.csv_import).
type CsvImportJob struct {
	TenantID uuid.UUID `json:"tenant_id"`
	ImportID uuid.UUID `json:"import_id"`
}

// SisImportJobQueue enqueues csv_import jobs (Nats-Msg-Id import:<import_id>).
type SisImportJobQueue interface {
	EnqueueCsvImport(ctx context.Context, job CsvImportJob) error
}
