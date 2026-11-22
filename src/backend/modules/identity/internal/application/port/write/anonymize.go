package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

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
	// FinishRequest marks the request executed with its erasure log row.
	FinishRequest(ctx context.Context, id, erasureID uuid.UUID, at time.Time) error
	// AnonymizeUser overwrites the PII of the user and of its student row (username anon.<user_id>, status
	// anonymized); false when it was already anonymized. No row is deleted (ADR-0028, ADR-0070).
	AnonymizeUser(ctx context.Context, userID uuid.UUID, at time.Time) (bool, error)
	AppendErasure(ctx context.Context, e ErasureEntry) error
	// RetentionDue lists students deactivated before `before` and not anonymized (job retention-anonymizer).
	RetentionDue(ctx context.Context, before time.Time, limit int) ([]AnonymizeSubject, error)
}
