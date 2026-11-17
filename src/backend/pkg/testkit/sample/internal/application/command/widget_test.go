package command_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/domain"
)

type spyUoW struct{ calls int }

func (s *spyUoW) Do(ctx context.Context, fn func(context.Context) error) error {
	s.calls++
	return fn(ctx)
}

type spyRepo struct{ calls int }

func (r *spyRepo) Create(context.Context, *domain.Widget) error { r.calls++; return nil }
func (r *spyRepo) Get(context.Context, uuid.UUID) (*domain.Widget, error) {
	r.calls++
	return nil, nil
}
func (r *spyRepo) Save(context.Context, *domain.Widget) error      { r.calls++; return nil }
func (r *spyRepo) Delete(context.Context, uuid.UUID, string) error { r.calls++; return nil }

type nopEvents struct{}

func (nopEvents) Publish(context.Context, string, string, any) error { return nil }

// F003-AC-11: validation before any transaction; missing permission never reaches the repository.
func TestCreateWidgetValidationAndAuthz(t *testing.T) {
	uow, repo := &spyUoW{}, &spyRepo{}
	h := command.Create{Handlers: command.Handlers{UoW: uow, Repo: repo, Events: nopEvents{}}}
	admin := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_content_editor"))

	_, err := h.Handle(admin, dto.CreateWidget{Code: "c1", Name: strings.Repeat("x", dto.MaxName+1)})
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.ValidationFailed || len(pe.Fields) != 1 || pe.Fields[0].Code != problem.SubMaxLength || pe.Fields[0].Field != "name" {
		t.Fatalf("too long name: %v", err)
	}
	if uow.calls != 0 {
		t.Fatal("unit of work opened before validation passed")
	}

	reviewer := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_content_reviewer"))
	authztest.ExpectDenied(t, func() error { _, err := h.Handle(reviewer, dto.CreateWidget{Code: "c1", Name: "ok"}); return err }())
	if repo.calls != 0 || uow.calls != 0 {
		t.Fatal("repository reached without permission")
	}

	if _, err := h.Handle(admin, dto.CreateWidget{Code: "c1", Name: "ok"}); err != nil || repo.calls != 1 {
		t.Fatalf("valid create: %v, repo calls %d", err, repo.calls)
	}
}
