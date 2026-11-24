package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// RenderScheduler creates the render jobs of a published version (command.GameMedia.ScheduleRenders).
type RenderScheduler interface {
	ScheduleRenders(ctx context.Context, itemType string, versionID uuid.UUID) error
}

// RenderThumbnails is the consumer game.render_thumbnails (F078-AC-19): a game content.published (platform event) of a
// version with 3D assets gets its thumbnail and turntable jobs; a replay creates nothing (unique job key, idempotency
// per event_id of pkg/outbox). It never blocks nor undoes the publication (F078-AC-20).
type RenderThumbnails struct {
	Renders RenderScheduler
}

// RenderThumbnailsOptions returns the outbox consumer options (platform events of the game stream).
func RenderThumbnailsOptions(h RenderThumbnails, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.RenderThumbnailsName, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.RenderThumbnailsEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c RenderThumbnails) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		ItemType    string    `json:"item_type"`
		ToVersionID uuid.UUID `json:"to_version_id"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.ItemType == "" || p.ToVersionID == uuid.Nil {
		observability.ConsumerRejected().WithLabelValues(messaging.RenderThumbnailsName, "invalid_payload").Inc()
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	return c.Renders.ScheduleRenders(ctx, p.ItemType, p.ToVersionID)
}
