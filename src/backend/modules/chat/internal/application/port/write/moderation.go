package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// NewSanction is a sanction to insert (F116: system mutes only).
type NewSanction struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	SchoolID  *uuid.UUID
	UserID    uuid.UUID
	Type      string
	Scope     string
	ChannelID *uuid.UUID
	Reason    string
	StartsAt  time.Time
	EndsAt    *time.Time
}

// SanctionRepository writes chat.sanctions in the transaction.
type SanctionRepository interface {
	Insert(ctx context.Context, s NewSanction) error
	// CountSince counts the sanctions of user with reason created since (repeat offence of auto_mute_spam).
	CountSince(ctx context.Context, tenant, user uuid.UUID, reason string, since time.Time) (int, error)
	// OpenMute reports an active mute of user with reason and no end (block_and_alert hold already in place).
	OpenMute(ctx context.Context, tenant, user uuid.UUID, reason string) (bool, error)
}
