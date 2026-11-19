package consumer

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// ReleasePointerCache is the consumer learning.release_pointer_cache (F026-AC-16, ADR-0043 §6): content.published,
// content.rolled_back and content.unpublished set ew:learning:current:<tenant>:<item_type>:<item_id> for every target
// tenant of the payload (the owner tenant, or the tenants a platform item is assigned to; _platform for a platform
// item assigned to nobody). Idempotency per event_id is pkg/outbox.Consumer (processed_events); setting the same
// pointer twice is harmless.
type ReleasePointerCache struct {
	Cache port.SnapshotCache
}

// ReleasePointerOptions returns the outbox consumer options (platform events accepted).
func ReleasePointerOptions(h ReleasePointerCache, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.ReleasePointerName, Stream: messaging.QuestionPoolStream,
		Subjects: messaging.QuestionPoolSubjects(), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c ReleasePointerCache) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		ItemType    string      `json:"item_type"`
		ItemID      uuid.UUID   `json:"item_id"`
		ToVersionID *uuid.UUID  `json:"to_version_id"`
		TenantID    *uuid.UUID  `json:"tenant_id"`
		TenantIDs   []uuid.UUID `json:"tenant_ids"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.ItemType == "" || p.ItemID == uuid.Nil {
		return invalid(e.EventType, "payload is not a release")
	}
	if len(p.TenantIDs) == 0 {
		return c.Cache.SetCurrent(ctx, p.TenantID, p.ItemType, p.ItemID, p.ToVersionID)
	}
	for _, t := range p.TenantIDs {
		if err := c.Cache.SetCurrent(ctx, &t, p.ItemType, p.ItemID, p.ToVersionID); err != nil {
			return err
		}
	}
	return nil
}
