package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DataRequest is one row of platform.deletion_requests (F016 US5, createDataRequest kind anonymize).
type DataRequest struct {
	ID            uuid.UUID
	SubjectID     uuid.UUID
	TenantID      uuid.UUID
	Status        string // pending | approved | executed | rejected
	Reason        string
	RequesterKind string
	Reference     *string
	RequestedBy   uuid.UUID
	ApprovedBy    *uuid.UUID
	ErasureID     *uuid.UUID
	CreatedAt     time.Time
	ExecutedAt    *time.Time
}

// AnonymizeSubject is the account an anonymization applies to.
type AnonymizeSubject struct {
	UserID   uuid.UUID
	TenantID *uuid.UUID
	UserType string
	Status   string
}

// ErasureEntry is one append-only row of platform.erasure_log.
type ErasureEntry struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	RequestedBy *uuid.UUID
	LegalBasis  string
	Scope       map[string]any
}

// AnonymizeRepository serves the anonymization (F016 US5). Every method runs in a platform scope
// (identity_platform: deletion_requests and erasure_log have no identity_app privilege).
type AnonymizeRepository interface {
	// Subject returns the account (live or soft-deleted) and its tenant; nil when unknown.
	Subject(ctx context.Context, userID uuid.UUID) (*AnonymizeSubject, error)
	CreateRequest(ctx context.Context, r DataRequest) error
	// Request returns one request (nil when unknown).
	Request(ctx context.Context, id uuid.UUID) (*DataRequest, error)
	// ListRequests lists requests newest first, limited to tenants (nil = every tenant) and status ("" = all).
	ListRequests(ctx context.Context, tenants []uuid.UUID, status string, limit int) ([]DataRequest, error)
	// FinishRequest marks the request executed with its erasure log row.
	FinishRequest(ctx context.Context, id, erasureID uuid.UUID, at time.Time) error
	// AnonymizeUser overwrites the PII of the user and of its student row (username anon.<user_id>, status
	// anonymized); false when it was already anonymized. No row is deleted (ADR-0028, ADR-0070).
	AnonymizeUser(ctx context.Context, userID uuid.UUID, at time.Time) (bool, error)
	AppendErasure(ctx context.Context, e ErasureEntry) error
	// RetentionDue lists students deactivated before `before` and not anonymized (job retention-anonymizer).
	RetentionDue(ctx context.Context, before time.Time, limit int) ([]AnonymizeSubject, error)
}
