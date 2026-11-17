// Package command holds the write-context use cases of the sample module: each handler is a typed
// interface (no mediator, ADR-0087) that runs UseCase → Validate → authz.Require → UnitOfWork.Do.
package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/domain"
)

// Permission used by the sample commands.
const Permission = authz.PermLearningContentWrite

// CreateWidgetHandler creates a widget.
type CreateWidgetHandler interface {
	Handle(ctx context.Context, in dto.CreateWidget) (uuid.UUID, error)
}

// RenameWidgetHandler renames a widget (optimistic version).
type RenameWidgetHandler interface {
	Handle(ctx context.Context, in dto.RenameWidget) (int64, error)
}

// DeleteWidgetHandler soft-deletes a widget.
type DeleteWidgetHandler interface {
	Handle(ctx context.Context, id uuid.UUID, reason string) error
}

// Handlers implements the three commands.
type Handlers struct {
	UoW    write.UnitOfWork
	Repo   write.WidgetRepository
	Events write.EventPublisher
}

func principal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	}
	return p, nil
}

func tenantResource(p authz.Principal) authz.Resource { return authz.Resource{TenantID: p.TenantID} }

// Create implements CreateWidgetHandler.
type Create struct{ Handlers }

// Handle runs the command.
func (h Create) Handle(ctx context.Context, in dto.CreateWidget) (id uuid.UUID, err error) {
	ctx, done := observability.UseCase(ctx, "sample.command.CreateWidget")
	defer func() { done(err) }()
	if err = in.Validate(); err != nil {
		return uuid.Nil, err
	}
	p, err := principal(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if err = authz.Require(ctx, p, Permission, tenantResource(p)); err != nil {
		return uuid.Nil, err
	}
	w, err := domain.NewWidget(in.Code, in.Name)
	if err != nil {
		return uuid.Nil, problem.New(problem.ValidationFailed, "invalid widget").Wrap(err)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Repo.Create(ctx, w); err != nil {
			return err
		}
		return h.Events.Publish(ctx, "widget.created", w.ID.String(), map[string]any{"id": w.ID.String(), "code": w.Code})
	})
	return w.ID, err
}

// Rename implements RenameWidgetHandler.
type Rename struct{ Handlers }

// Handle runs the command.
func (h Rename) Handle(ctx context.Context, in dto.RenameWidget) (version int64, err error) {
	ctx, done := observability.UseCase(ctx, "sample.command.RenameWidget")
	defer func() { done(err) }()
	if err = in.Validate(); err != nil {
		return 0, err
	}
	p, err := principal(ctx)
	if err != nil {
		return 0, err
	}
	if err = authz.Require(ctx, p, Permission, tenantResource(p)); err != nil {
		return 0, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		w, err := h.Repo.Get(ctx, in.ID)
		if err != nil {
			return err
		}
		w.Version = in.Version // the client's version: a stale one fails with 409
		if err := w.Rename(in.Name); err != nil {
			return problem.New(problem.ValidationFailed, "invalid name").Wrap(err)
		}
		if err := h.Repo.Save(ctx, w); err != nil {
			return err
		}
		version = w.Version
		return nil
	})
	return version, err
}

// Delete implements DeleteWidgetHandler.
type Delete struct{ Handlers }

// Handle runs the command.
func (h Delete) Handle(ctx context.Context, id uuid.UUID, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "sample.command.DeleteWidget")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	if err = authz.Require(ctx, p, Permission, tenantResource(p)); err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error { return h.Repo.Delete(ctx, id, reason) })
}
