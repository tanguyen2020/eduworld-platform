package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// PolicyStore writes the chat policy levels (chat.chat_policies) and the common-tenant overrides in the transaction of a
// policy use case (F120 US2). Reads through read.PolicyReader in the same transaction.
type PolicyStore interface {
	read.PolicyReader
	// Lock serialises the writes of one level (transaction advisory lock).
	Lock(ctx context.Context, tenant *uuid.UUID, scopeType string, scopeID *uuid.UUID) error
	// InsertLevel inserts the first document of a level (version 1).
	InsertLevel(ctx context.Context, r read.PolicyRow) error
	// UpdateLevel stores a new version when the stored one is prev (false: version changed meanwhile).
	UpdateLevel(ctx context.Context, r read.PolicyRow, prev int64) (bool, error)
	// SaveCommonOverride upserts the override of a tenant common (version raised).
	SaveCommonOverride(ctx context.Context, r read.CommonOverrideRow) (read.CommonOverrideRow, error)
}
