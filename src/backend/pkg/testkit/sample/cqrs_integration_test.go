//go:build integration

package sample_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/query"
	pread "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/infrastructure/persistence/read"
	pwrite "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/infrastructure/persistence/write"
)

type failingAfterOutbox struct{ pwrite.Publisher }

var errAfterOutbox = errors.New("failure after the outbox write")

func (f failingAfterOutbox) Publish(ctx context.Context, t, k string, p any) error {
	if err := f.Publisher.Publish(ctx, t, k, p); err != nil {
		return err
	}
	return errAfterOutbox
}

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F003-AC-09 (TC-SVC-001), F003-AC-12: write/read contexts of the sample module end to end.
func TestSampleCQRS(t *testing.T) {
	e := sample.Setup(context.Background(), t)
	tenancy.Configure(tenancy.Hooks{Recorder: e.Audit})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })

	base := command.Handlers{UoW: e.UoW, Repo: pwrite.WidgetRepo{}, Events: pwrite.Publisher{W: e.Writer}}
	create := command.Create{Handlers: base}
	rename := command.Rename{Handlers: base}
	del := command.Delete{Handlers: base}
	list := query.List{Model: pread.WidgetModel{Session: e.Read}}
	ctx := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin"))

	id, err := create.Handle(ctx, dto.CreateWidget{Code: "w-1", Name: "One"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := create.Handle(ctx, dto.CreateWidget{Code: "w-1", Name: "Dup"}); codeOf(err) != problem.Conflict {
		t.Fatalf("duplicate code: %v", err)
	}

	countOutbox := func() (n int) {
		_ = e.Migrator.QueryRow(`SELECT count(*) FROM infra.outbox_events`).Scan(&n)
		return n
	}
	before := countOutbox()
	failing := command.Create{Handlers: command.Handlers{UoW: e.UoW, Repo: pwrite.WidgetRepo{}, Events: failingAfterOutbox{pwrite.Publisher{W: e.Writer}}}}
	if _, err := failing.Handle(ctx, dto.CreateWidget{Code: "w-2", Name: "Two"}); !errors.Is(err, errAfterOutbox) {
		t.Fatalf("failing command: %v", err)
	}
	if n := countOutbox(); n != before {
		t.Fatalf("outbox has %d new events after rollback", n-before)
	}

	views, err := list.Handle(ctx)
	if err != nil || len(views) != 1 || views[0].ID != id {
		t.Fatalf("list: %v %v", views, err)
	}

	v, err := rename.Handle(ctx, dto.RenameWidget{ID: id, Name: "One!", Version: 1})
	if err != nil || v != 2 {
		t.Fatalf("rename: %d %v", v, err)
	}
	if _, err := rename.Handle(ctx, dto.RenameWidget{ID: id, Name: "stale", Version: 1}); codeOf(err) != problem.Conflict {
		t.Fatalf("stale version: %v", err)
	}

	other := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantB, "tenant_admin"))
	authztest.Cover(t, "sampleRenameWidget", authztest.CrossTenant, authztest.RoleDenied)
	if _, err := rename.Handle(other, dto.RenameWidget{ID: id, Name: "x", Version: 2}); codeOf(err) != problem.NotFound {
		t.Fatalf("other tenant rename: %v", err)
	}

	if err := del.Handle(ctx, id, "test"); err != nil {
		t.Fatal(err)
	}
	if views, _ := list.Handle(ctx); len(views) != 0 {
		t.Fatalf("deleted widget listed: %v", views)
	}
	if err := del.Handle(ctx, id, "again"); codeOf(err) != problem.NotFound {
		t.Fatalf("delete twice: %v", err)
	}
}
