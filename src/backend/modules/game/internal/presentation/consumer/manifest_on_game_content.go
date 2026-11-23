package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// GameManifestRebuilder rebuilds the game part of every tenant manifest (command.GameManifest).
type GameManifestRebuilder interface {
	Rebuild(ctx context.Context, cause uuid.UUID, itemType string, releasedAt time.Time) error
}

// ManifestOnGameContent is the consumer game.manifest_on_game_content (F071-AC-05): a game content.published (platform
// event) rebuilds the manifest of every active tenant. Idempotency per event_id is pkg/outbox.Consumer.
type ManifestOnGameContent struct {
	Manifests GameManifestRebuilder
}

// ManifestOnGameContentOptions returns the outbox consumer options (platform events only).
func ManifestOnGameContentOptions(h ManifestOnGameContent, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.ManifestGameName, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.ManifestGameEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c ManifestOnGameContent) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		ItemType   string    `json:"item_type"`
		ReleaseID  uuid.UUID `json:"release_id"`
		ReleasedAt time.Time `json:"released_at"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.ItemType == "" || p.ReleaseID == uuid.Nil {
		observability.ConsumerRejected().WithLabelValues(messaging.ManifestGameName, "invalid_payload").Inc()
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	if p.ItemType == "progression_curve" || p.ItemType == "reward_rule" {
		return nil // server-side progression content (F074): not part of the client manifest
	}
	return c.Manifests.Rebuild(ctx, p.ReleaseID, p.ItemType, p.ReleasedAt)
}
