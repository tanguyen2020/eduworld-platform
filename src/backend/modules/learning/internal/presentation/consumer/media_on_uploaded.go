package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// MediaProcessor is the media worker step (command.Media.ProcessMedia).
type MediaProcessor interface {
	ProcessMedia(ctx context.Context, id uuid.UUID) error
}

// MediaWorker is the consumer learning.media_worker (F027-AC-04, AC-05): one media.uploaded event processes one
// asset in the scope of the event tenant (platform media under the platform scope). A scanner outage or a storage
// error is retried with backoff (the asset stays processing); after MaxDeliver the event goes to DLQ_LEARNING.
type MediaWorker struct {
	Process MediaProcessor
}

// MediaWorkerBackOff is the retry delay of the media worker (clamd or object storage unavailable).
var MediaWorkerBackOff = []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute}

// MediaWorkerOptions returns the outbox consumer options (tenant and platform media).
func MediaWorkerOptions(h MediaWorker, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.MediaWorkerName, Stream: messaging.MediaWorkerStream,
		Subjects: messaging.MediaWorkerSubjects(), Platform: true, MaxDeliver: 8, BackOff: MediaWorkerBackOff, AckWait: 15 * time.Minute,
		UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c MediaWorker) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		AssetID uuid.UUID `json:"asset_id"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.AssetID == uuid.Nil {
		return outbox.Permanent(invalid(e.EventType, "payload has no asset_id"))
	}
	return c.Process.ProcessMedia(ctx, p.AssetID)
}
