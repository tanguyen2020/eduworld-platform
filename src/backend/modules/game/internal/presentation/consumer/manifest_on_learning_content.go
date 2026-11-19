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

// ManifestMerger applies a learning release to the tenant manifests (command.Manifests).
type ManifestMerger interface {
	Merge(ctx context.Context, r command.LearningRelease) error
}

// ManifestOnLearningContent is the consumer game.manifest_on_learning_content (F026 T046, AC-23…AC-26): the release
// events of module learning rebuild the manifest of their target tenants (tenant_ids of the payload: the owner tenant
// or the tenants a platform item is assigned to; never another tenant). Idempotency per event_id is pkg/outbox.Consumer.
type ManifestOnLearningContent struct {
	Manifests ManifestMerger
}

// ManifestOnLearningContentOptions returns the outbox consumer options (platform events accepted).
func ManifestOnLearningContentOptions(h ManifestOnLearningContent, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.ManifestLearningName, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.ManifestLearningEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

type learningRelease struct {
	ItemType         string      `json:"item_type"`
	ItemID           uuid.UUID   `json:"item_id"`
	ToVersionID      *uuid.UUID  `json:"to_version_id"`
	ToVersion        int         `json:"to_version"`
	ReleaseID        uuid.UUID   `json:"release_id"`
	TenantIDs        []uuid.UUID `json:"tenant_ids"`
	SnapshotURL      string      `json:"snapshot_url"`
	SnapshotSHA256   string      `json:"snapshot_sha256"`
	ActivityType     string      `json:"activity_type"`
	MinClientVersion string      `json:"min_client_version"`
	ReleasedAt       time.Time   `json:"released_at"`
}

// Handle implements outbox.Handler.
func (c ManifestOnLearningContent) Handle(ctx context.Context, e outbox.Envelope) error {
	var p learningRelease
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.ItemType == "" || p.ItemID == uuid.Nil {
		observability.ConsumerRejected().WithLabelValues(messaging.ManifestLearningName, "invalid_payload").Inc()
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	return c.Manifests.Merge(ctx, command.LearningRelease{ItemType: p.ItemType, ItemID: p.ItemID, VersionID: p.ToVersionID, VersionNo: p.ToVersion,
		SnapshotURL: p.SnapshotURL, SHA256: p.SnapshotSHA256, ActivityType: p.ActivityType, MinClientVersion: p.MinClientVersion,
		ReleaseID: p.ReleaseID, ReleasedAt: p.ReleasedAt, Tenants: p.TenantIDs})
}
