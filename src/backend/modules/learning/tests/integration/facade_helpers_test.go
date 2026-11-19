//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	pwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// facadeOf wires the learning facade on the test databases with the fixture resource handler (as module.go
// does in production with the real handlers).
func (e *env) facadeOf(t *testing.T, cache pwrite.CacheInvalidator) facade.Facade {
	t.Helper()
	reg, err := port.NewRegistry(fixtures.Handler{})
	if err != nil {
		t.Fatal(err)
	}
	codec, err := page.NewCodec([]byte("test-cursor-key-of-32-bytes-0123456789"))
	if err != nil {
		t.Fatal(err)
	}
	deps := command.AssignmentDeps{UoW: e.uow, Assignments: write.AssignmentRepo{}, Tenants: read.TenantDirectory{}, Resources: reg,
		Audit: write.Auditor{R: e.rec}, Cache: cache, Metrics: observability.AssignmentCounter{}}
	return facade.Facade{
		Apply: command.ApplyTenantAssignments{AssignmentDeps: deps}, Revoke: command.RevokeTenantAssignments{AssignmentDeps: deps},
		Assignments: query.ListResourceAssignments{Session: e.read, Assignments: read.AssignmentReadModel{}, Cursors: codec},
		Trash:       query.ListTrash{Session: e.read, Resources: reg, Cursors: codec},
		Restore:     command.RestoreResource{TrashDeps: e.trashDeps(reg)},
		Clone: command.ExecuteCloneStep{CloneDeps: command.CloneDeps{UoW: e.uow, Resources: reg, Tenants: read.TenantDirectory{},
			Audit: write.Auditor{R: e.rec}}},
		Idempotency: idempotency.NewStore(e.app, 0, nil),
	}
}

// asIdentity calls a facade operation the way module identity does: through pkg/modcall with the original
// Principal (System Admin of the portal) and an Idempotency-Key in the context.
func asIdentity[T any](ctx context.Context, op string, key string, fn func(ctx context.Context) (T, error)) (T, error) {
	if key != "" {
		ctx = modcall.WithIdempotencyKey(ctx, key)
	}
	return modcall.Call(ctx, modcall.Op{Caller: "identity", Callee: "learning", Operation: op, Kind: modcall.Saga,
		AllowedCallers: learningapi.AllowedCallers[op], ContextRequired: learningapi.ContextRequired[op]}, fn)
}

func newKey() string { return ids.New().String() }

func systemAdmin(ctx context.Context) (context.Context, authz.Principal) {
	p := authztest.Platform("system_admin")
	return authztest.Ctx(ctx, p), p
}

// platformItem creates a platform fixture item (content_admin under PlatformScope).
func platformItem(ctx context.Context, t *testing.T, e *env, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.uow.Do(authztest.Ctx(ctx, authztest.Platform("content_admin")), func(ctx context.Context) error {
		var err error
		id, err = fixtures.CreateItem(ctx, code)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

// tenantItem creates a fixture item of tenant.
func tenantItem(ctx context.Context, t *testing.T, e *env, tenant uuid.UUID, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.uow.Do(authztest.Ctx(ctx, authztest.Staff(tenant, "tenant_content_editor")), func(ctx context.Context) error {
		var err error
		id, err = fixtures.CreateItem(ctx, code)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func apply(ctx context.Context, f facade.Facade, key string, rt string, id uuid.UUID, tenants ...uuid.UUID) (learningapi.CommonInternalCommonV1TenantAssignmentList, error) {
	res, err := asIdentity(ctx, "applyTenantAssignments", key, func(ctx context.Context) (learningapi.ApplyTenantAssignmentsResponseObject, error) {
		body := learningapi.ApplyTenantAssignmentsJSONRequestBody{ResourceType: learningapi.CommonInternalCommonV1ResourceType(rt),
			ResourceId: id, TenantIds: tenants}
		return f.ApplyTenantAssignments(ctx, learningapi.ApplyTenantAssignmentsRequestObject{Body: &body})
	})
	if err != nil {
		return learningapi.CommonInternalCommonV1TenantAssignmentList{}, err
	}
	return learningapi.CommonInternalCommonV1TenantAssignmentList(res.(learningapi.ApplyTenantAssignments200JSONResponse)), nil
}

func revoke(ctx context.Context, f facade.Facade, key, reason string, id uuid.UUID, tenants ...uuid.UUID) (learningapi.CommonInternalCommonV1TenantAssignmentList, error) {
	res, err := asIdentity(ctx, "revokeTenantAssignments", key, func(ctx context.Context) (learningapi.RevokeTenantAssignmentsResponseObject, error) {
		body := learningapi.RevokeTenantAssignmentsJSONRequestBody{ResourceType: "question", ResourceId: id, TenantIds: tenants, Reason: reason}
		return f.RevokeTenantAssignments(ctx, learningapi.RevokeTenantAssignmentsRequestObject{Body: &body})
	})
	if err != nil {
		return learningapi.CommonInternalCommonV1TenantAssignmentList{}, err
	}
	return learningapi.CommonInternalCommonV1TenantAssignmentList(res.(learningapi.RevokeTenantAssignments200JSONResponse)), nil
}

// visibleTo reports whether tenant reads item.
func visibleTo(ctx context.Context, t *testing.T, e *env, tenant, item uuid.UUID) bool {
	t.Helper()
	var ok bool
	if err := e.read.Read(authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin")), func(ctx context.Context) error {
		_, err := fixtures.Get(ctx, item)
		ok = err == nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return ok
}

func listReq(item uuid.UUID) learningapi.ListResourceAssignmentsRequestObject {
	return learningapi.ListResourceAssignmentsRequestObject{Params: learningapi.ListResourceAssignmentsParams{ResourceType: "question", ResourceId: item}}
}

func listData(res any) []learningapi.CommonInternalCommonV1TenantAssignment {
	if r, ok := res.(learningapi.ListResourceAssignments200JSONResponse); ok {
		return r.Data
	}
	return nil
}

func asProblem(err error, target **problem.Error) bool { return errors.As(err, target) }

func (e *env) trashDeps(reg *port.Registry) command.TrashDeps {
	return command.TrashDeps{UoW: e.uow, Resources: reg, Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec}}
}

// softDelete runs the SoftDeleteResource use case with the scope of ctx.
func (e *env) softDelete(ctx context.Context, t *testing.T, id uuid.UUID, reason string) error {
	t.Helper()
	reg, err := port.NewRegistry(fixtures.Handler{})
	if err != nil {
		t.Fatal(err)
	}
	return command.SoftDeleteResource{TrashDeps: e.trashDeps(reg)}.Handle(ctx, command.SoftDeleteInput{ResourceType: "question", ID: id, Reason: reason})
}
