// Package write holds the write-context ports of the sample module (ADR-0087).
package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/domain"
)

// UnitOfWork runs fn in one write transaction (entx.UnitOfWork).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// WidgetRepository persists the aggregate; Save fails with 409 CONFLICT on a stale version.
type WidgetRepository interface {
	Create(ctx context.Context, w *domain.Widget) error
	Get(ctx context.Context, id uuid.UUID) (*domain.Widget, error)
	Save(ctx context.Context, w *domain.Widget) error
	Delete(ctx context.Context, id uuid.UUID, reason string) error
}

// EventPublisher writes domain events into the outbox of the transaction.
type EventPublisher interface {
	Publish(ctx context.Context, eventType, key string, payload any) error
}
