package consumer

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// PoolInvalidator drops cached question pools (cache.Invalidator).
type PoolInvalidator interface {
	InvalidateTenant(ctx context.Context, tenant uuid.UUID) error
	InvalidateAll(ctx context.Context) error
}

// QuestionPoolInvalidation is the consumer learning.qb_pool_invalidation (F022-AC-28): a question released, rolled
// back or unpublished drops the pools of its tenant, or of every tenant for a platform question (any tenant may have
// it assigned). Assignment changes drop the pools in the assignment use case of F020 (CacheInvalidator).
type QuestionPoolInvalidation struct {
	Cache PoolInvalidator
}

// QuestionPoolOptions returns the outbox consumer options (platform events accepted).
func QuestionPoolOptions(h QuestionPoolInvalidation, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.QuestionPoolName, Stream: messaging.QuestionPoolStream,
		Subjects: messaging.QuestionPoolSubjects(), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c QuestionPoolInvalidation) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		ItemType string `json:"item_type"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return invalid(e.EventType, "payload is not an object")
	}
	if p.ItemType != "question" {
		return nil
	}
	if e.TenantID != nil {
		return c.Cache.InvalidateTenant(ctx, *e.TenantID)
	}
	return c.Cache.InvalidateAll(ctx)
}
