package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// MinigameConfigProjection is the consumer learning.minigame_config_projection (F073-AC-04, T016): a content.published of
// module game (platform event, tenant_id null) with item_type minigame_config stores its minigame_config block in the
// immutable projection ref.minigame_config_versions; other item types are ignored. A replayed event inserts nothing
// (idempotency per event_id is pkg/outbox.Consumer, per version the insert-only projection, TC-SVC-003); a payload the
// projection refuses is dead-lettered at once (DLQ_LEARNING).
type MinigameConfigProjection struct {
	Upsert command.UpsertMiniGameConfigSnapshotHandler
}

// MinigameConfigProjectionOptions returns the outbox consumer options (platform events of the game stream only).
func MinigameConfigProjectionOptions(h MinigameConfigProjection, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.MinigameConfigProjectionName, Stream: messaging.GameStream,
		Subjects: messaging.MinigameConfigProjectionSubjects(), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// minigameItemType is content.published.item_type of a mini-game configuration (F073).
const minigameItemType = "minigame_config"

// Handle implements outbox.Handler.
func (c MinigameConfigProjection) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		ItemType    string    `json:"item_type"`
		ItemID      string    `json:"item_id"`
		ToVersionID uuid.UUID `json:"to_version_id"`
		ToVersion   int       `json:"to_version"`
		ReleasedAt  time.Time `json:"released_at"`
		Minigame    *struct {
			ConfigID    uuid.UUID       `json:"minigame_config_id"`
			VersionID   uuid.UUID       `json:"version_id"`
			Version     int             `json:"version"`
			Type        string          `json:"minigame_type"`
			GradeLevels []int           `json:"grade_levels"`
			Snapshot    json.RawMessage `json:"snapshot"`
		} `json:"minigame_config"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.ItemType == "" {
		observability.ConsumerRejected().WithLabelValues(messaging.MinigameConfigProjectionName, "invalid_payload").Inc()
		return outbox.Permanent(invalid(e.EventType, "item_type is required"))
	}
	if p.ItemType != minigameItemType {
		return nil // other game content (world areas, quests, rewards…): not a mini-game configuration
	}
	if e.TenantID != nil || p.Minigame == nil || p.ReleasedAt.IsZero() || p.Minigame.ConfigID.String() != p.ItemID ||
		p.Minigame.VersionID != p.ToVersionID || p.Minigame.Version != p.ToVersion {
		observability.ConsumerRejected().WithLabelValues(messaging.MinigameConfigProjectionName, "invalid_payload").Inc()
		return outbox.Permanent(invalid(e.EventType, "a platform minigame_config block matching item_id, to_version_id and to_version is required"))
	}
	m := p.Minigame
	_, err := c.Upsert.Upsert(ctx, command.MiniGameConfigSnapshot{ConfigID: m.ConfigID, VersionID: m.VersionID, Version: m.Version, Type: m.Type,
		GradeLevels: m.GradeLevels, Snapshot: m.Snapshot, PublishedAt: p.ReleasedAt.UTC(), SourceVersion: p.ReleasedAt.UnixMicro()})
	if errors.Is(err, command.ErrMinigameSnapshot) {
		observability.ConsumerRejected().WithLabelValues(messaging.MinigameConfigProjectionName, "invalid_snapshot").Inc()
		return outbox.Permanent(err)
	}
	return err
}
