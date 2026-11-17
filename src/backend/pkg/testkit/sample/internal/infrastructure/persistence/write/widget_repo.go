// Package write implements the write ports of the sample module with ent inside entx.UnitOfWork.
package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent/widget"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/domain"
)

func client(ctx context.Context) *ent.Client { return ent.NewClient(ent.Driver(entx.Driver(ctx))) }

// WidgetRepo implements write.WidgetRepository.
type WidgetRepo struct{}

func toDomain(w *ent.Widget) *domain.Widget {
	return &domain.Widget{ID: w.ID, TenantID: w.TenantID, Code: w.Code, Name: w.Name, Version: w.Version}
}

// Create inserts the widget (tenant from the scope); a duplicate code is 409 CONFLICT.
func (WidgetRepo) Create(ctx context.Context, w *domain.Widget) error {
	row, err := client(ctx).Widget.Create().SetCode(w.Code).SetName(w.Name).Save(ctx)
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, "widget code already exists").Wrap(err)
	}
	if err != nil {
		return err
	}
	*w = *toDomain(row)
	return nil
}

// Get loads one widget of the scope (404 outside it).
func (WidgetRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Widget, error) {
	row, err := client(ctx).Widget.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	return toDomain(row), nil
}

// Save updates with optimistic concurrency: WHERE version = w.Version, version + 1; a stale version
// (no row updated while the row exists) is 409 CONFLICT (F003-AC-12, API_DESIGN §7).
func (WidgetRepo) Save(ctx context.Context, w *domain.Widget) error {
	n, err := client(ctx).Widget.Update().Where(widget.ID(w.ID), widget.Version(w.Version)).
		SetName(w.Name).AddVersion(1).Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.New(problem.Conflict, "widget was modified")
	}
	w.Version++
	return nil
}

// Delete soft-deletes with audit and event widget.deleted in the same transaction.
func (WidgetRepo) Delete(ctx context.Context, id uuid.UUID, reason string) error {
	err := entx.SoftDelete(ctx, "widget", id, reason, func(ctx context.Context) error {
		return client(ctx).Widget.DeleteOneID(id).Exec(ctx)
	})
	if ent.IsNotFound(err) {
		return problem.New(problem.NotFound, "")
	}
	return err
}

// Publisher implements write.EventPublisher with the outbox writer.
type Publisher struct{ W *outbox.Writer }

// Publish writes eventType for key into the outbox of the transaction.
func (p Publisher) Publish(ctx context.Context, eventType, key string, payload any) error {
	return p.W.Write(ctx, eventType, key, payload)
}
