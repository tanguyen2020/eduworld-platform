package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors of the contract_v1 provider (F017, SIS_INTEGRATION §7). Adapters wrap them.
var (
	// ErrSisAuth: 401/403 of the facade, OAuth2 token refused, or the secret cannot be read (no retry).
	ErrSisAuth = errors.New("sis: credential rejected")
	// ErrSisSecret: the secret reference cannot be read from the secret manager (no retry).
	ErrSisSecret = errors.New("sis: secret unavailable")
	// ErrSisUnavailable: timeout, network error or 5xx after the retries of sis_retry_backoff.
	ErrSisUnavailable = errors.New("sis: facade unavailable")
	// ErrSisNotFound: 404 of the facade (lookupStudent, unknown school).
	ErrSisNotFound = errors.New("sis: not found")
	// ErrSisWatermarkExpired: 410 of listChanges, a full sync is needed.
	ErrSisWatermarkExpired = errors.New("sis: delta watermark expired")
	// ErrSisPagination: has_more without a new next_cursor (never loop).
	ErrSisPagination = errors.New("sis: invalid pagination")
	// ErrSisBadResponse: the page itself does not match the contract (not one record).
	ErrSisBadResponse = errors.New("sis: response outside the contract")
	// ErrSisLookupUnsupported: the connection is not active or does not declare student_lookup (no call).
	ErrSisLookupUnsupported = errors.New("sis lookup: unsupported")
	// ErrRunLockBusy: another run of the connection holds the lock.
	ErrRunLockBusy = errors.New("sis: run lock busy")
)

// ContractConnection is what the contract_v1 client needs: references only, never a secret value.
type ContractConnection struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	BaseURL    string
	AuthType   string
	TokenURL   string
	OAuthScope string
	SecretRef  string
}

// SisViolation is one record of a response that breaks the contract (SIS_CONTRACT_VIOLATION).
type SisViolation struct {
	Kind       string // school, class, student, enrollment, academic_year, change
	ExternalID string
	Field      string
	Message    string
}

// SisPage is one validated page: records already stripped of unknown fields and checked against the DTO.
type SisPage struct {
	Records       []map[string]any
	Violations    []SisViolation
	UnknownFields int
	NextCursor    string
	HasMore       bool
}

// SisHealth is getHealth.
type SisHealth struct {
	Status          string
	ContractVersion string
	Capabilities    []string
}

// SisChange is one validated Change of listChanges or a webhook ChangeBatch.
type SisChange struct {
	Sequence   int64          `json:"sequence"`
	Kind       string         `json:"entity_kind"`
	Operation  string         `json:"operation"`
	ExternalID string         `json:"external_id"`
	Record     map[string]any `json:"record,omitempty"`
}

// SisChangePage is one validated page of listChanges.
type SisChangePage struct {
	Changes       []SisChange
	Violations    []SisViolation
	UnknownFields int
	NextCursor    string
	HasMore       bool
}

// SisClientStats are the counters of one client (stats of the run).
type SisClientStats struct {
	Requests int
	Retries  int
}

// SisProvider is the contract_v1 client of one connection (SISProvider of SIS_INTEGRATION §4.1). Every
// response is validated against the contract; records breaking it are returned as violations.
type SisProvider interface {
	Health(ctx context.Context) (SisHealth, error)
	ListSchools(ctx context.Context, cursor string) (SisPage, error)
	ListAcademicYears(ctx context.Context, schoolExtID string) (SisPage, error)
	ListClasses(ctx context.Context, schoolExtID, year, cursor string) (SisPage, error)
	ListStudents(ctx context.Context, schoolExtID, cursor string) (SisPage, error)
	ListEnrollments(ctx context.Context, schoolExtID, year, cursor string) (SisPage, error)
	ListChanges(ctx context.Context, since string) (SisChangePage, error)
	// LookupStudent calls once, without retry (login path, ADR-0061).
	LookupStudent(ctx context.Context, schoolExtID, loginUsername string) (map[string]any, error)
	Stats() SisClientStats
}

// SisProviderFactory builds the client of a connection (reads the secret through the secret store).
type SisProviderFactory interface {
	For(ctx context.Context, c ContractConnection) (SisProvider, error)
}

// SecretStore reads a secret by reference (Swarm secret file, research R4). The value is never logged.
type SecretStore interface {
	Get(ctx context.Context, ref string) ([]byte, error)
}

// RunLock is the one-run-per-connection lock (Valkey ew:identity:sis:lock:<connection_id>, SIS_INTEGRATION §5).
type RunLock interface {
	// Acquire returns ErrRunLockBusy when another run holds it; release stops the heartbeat and frees it.
	Acquire(ctx context.Context, connectionID uuid.UUID) (release func(), err error)
}

// WebhookNonces is the replay guard of webhooks (ew:identity:sis:wh:nonce:<delivery_id>, sis_webhook_nonce_ttl).
type WebhookNonces interface {
	// Claim returns false when the delivery id was already received within the TTL.
	Claim(ctx context.Context, deliveryID uuid.UUID) (bool, error)
	// Release frees a claim whose job could not be queued (the SIS retries).
	Release(ctx context.Context, deliveryID uuid.UUID) error
}

// Webhook errors.
var (
	ErrWebhookSignature = errors.New("sis webhook: invalid signature or timestamp")
	ErrWebhookPayload   = errors.New("sis webhook: payload outside the contract")
)

// WebhookBatch is a verified ChangeBatch.
type WebhookBatch struct {
	DeliveryID      uuid.UUID
	ContractVersion string
	Changes         []SisChange
	Violations      []SisViolation
	UnknownFields   int
}

// WebhookVerifier checks X-EduWorld-Signature/Timestamp (HMAC-SHA256, constant time, sis_webhook_skew) and
// validates the ChangeBatch (VerifyAndParseWebhook of SIS_INTEGRATION §4.1).
type WebhookVerifier interface {
	Verify(secret []byte, signature, timestamp string, body []byte, now time.Time) (WebhookBatch, error)
}

// SyncJob is a scheduled_sync job (subject ew.job.identity.sis.scheduled_sync): a queued manual run (RunID),
// or a schedule/fallback run created by the worker (RunID nil).
type SyncJob struct {
	TenantID     uuid.UUID  `json:"tenant_id"`
	ConnectionID uuid.UUID  `json:"sis_connection_id"`
	RunID        *uuid.UUID `json:"sync_run_id,omitempty"`
	Mode         string     `json:"mode"`
	Trigger      string     `json:"trigger"`
	DryRun       bool       `json:"dry_run,omitempty"`
}

// WebhookJob is a webhook_batch job (Nats-Msg-Id = delivery_id).
type WebhookJob struct {
	TenantID     uuid.UUID   `json:"tenant_id"`
	ConnectionID uuid.UUID   `json:"sis_connection_id"`
	DeliveryID   uuid.UUID   `json:"delivery_id"`
	Changes      []SisChange `json:"changes"`
	Violations   int         `json:"violations,omitempty"`
}

// SisSyncJobQueue publishes F017 jobs on IDENTITY_SIS_JOBS; it returns after the JetStream ack.
type SisSyncJobQueue interface {
	EnqueueSync(ctx context.Context, job SyncJob, msgID string) error
	EnqueueWebhookBatch(ctx context.Context, job WebhookJob) error
}

// SisSyncMetrics observes the F017 run outcomes (SIS_INTEGRATION §8).
type SisSyncMetrics interface {
	RunSkipped(connectionID uuid.UUID, reason string)
	Webhook(result string)
	RunDuration(connectionID uuid.UUID, mode string, d time.Duration)
}
