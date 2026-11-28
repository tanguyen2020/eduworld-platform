package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// NicknameResetter resets the nickname of a student (command.Profiles.ResetNicknameBySanction).
type NicknameResetter interface {
	ResetNicknameBySanction(ctx context.Context, in command.SanctionReset) error
}

// SanctionNicknameReset is the consumer game.sanction_nickname_reset (F136-AC-20, worker profile events): sanction.applied
// of module chat (schema owner F116) of type nickname_reset → the nickname of the student becomes NULL (setup required);
// every other type is ignored. Idempotent per event_id (processed_events) and per sanction_id (the history keeps it). A
// payload whose tenant differs from the envelope is permanent (MULTITENANCY §4.1).
type SanctionNicknameReset struct{ Reset NicknameResetter }

// SanctionNicknameResetOptions returns the outbox consumer options (stream EVT_CHAT).
func SanctionNicknameResetOptions(h SanctionNicknameReset, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.SanctionNicknameResetName, Stream: messaging.ChatStream,
		Subjects: messaging.ChatSubjects(messaging.SanctionNicknameResetEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

type sanctionApplied struct {
	TenantID     uuid.UUID  `json:"tenant_id"`
	SanctionID   uuid.UUID  `json:"sanction_id"`
	UserID       uuid.UUID  `json:"user_id"`
	SanctionType string     `json:"sanction_type"`
	IssuedBy     *uuid.UUID `json:"issued_by"`
	StartsAt     time.Time  `json:"starts_at"`
}

// SanctionNicknameResetType is the sanction type handled (MODERATION §7, F136).
const SanctionNicknameResetType = "nickname_reset"

// Handle implements outbox.Handler.
func (c SanctionNicknameReset) Handle(ctx context.Context, e outbox.Envelope) error {
	var p sanctionApplied
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.SanctionType == "" {
		return c.reject(e, "invalid_payload")
	}
	if p.SanctionType != SanctionNicknameResetType {
		return nil
	}
	if p.SanctionID == uuid.Nil || p.UserID == uuid.Nil || e.TenantID == nil || p.TenantID != *e.TenantID {
		return c.reject(e, "tenant_mismatch")
	}
	return c.Reset.ResetNicknameBySanction(ctx, command.SanctionReset{TenantID: p.TenantID, StudentID: p.UserID,
		SanctionID: p.SanctionID, IssuedBy: p.IssuedBy, At: p.StartsAt})
}

func (c SanctionNicknameReset) reject(e outbox.Envelope, reason string) error {
	observability.ConsumerRejected().WithLabelValues(messaging.SanctionNicknameResetName, reason).Inc()
	return outbox.Permanent(fmt.Errorf("%s: %s", e.EventType, reason))
}
