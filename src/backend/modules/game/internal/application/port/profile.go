package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// NicknameOptionStore keeps the nickname suggestions of a student in Valkey (ew:game:nick:opt:<student_id>, TTL
// nickname_suggestion_ttl). Temporary data (ADR-0004): a lost option only means "choose again" (F070-AC-09).
type NicknameOptionStore interface {
	// Save adds options (option id → nickname) of student and resets the TTL of the student's set.
	Save(ctx context.Context, student uuid.UUID, options map[uuid.UUID]string, ttl time.Duration) error
	// Get returns the nickname of option of student; false when unknown, expired or of another student.
	Get(ctx context.Context, student, option uuid.UUID) (string, bool, error)
	// Clear removes every option of student (after a nickname is chosen).
	Clear(ctx context.Context, student uuid.UUID) error
}

// Reasons of eduworld_game_avatar_update_rejected_total{reason}.
const (
	RejectInvalid  = "validation_failed"
	RejectNotOwned = "item_not_owned"
	RejectSlot     = "equip_slot_invalid"
)

// ProfileMetrics records the F070 metrics (plan F070 §Quan sát).
type ProfileMetrics interface {
	AvatarCreated()
	AvatarRejected(reason string)
	NicknameConflict()
}
