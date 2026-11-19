//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/runtime" // mixin hooks, interceptors
)

type env struct {
	app, platform, relay *sql.DB
	uow                  *entx.UnitOfWork
	read                 *entx.ReadSession
	deps                 command.TenantDeps
	cl                   *testkit.Cluster // the isolated databases of the test (other role DSNs)
}

func setup(ctx context.Context, t *testing.T) *env {
	t.Helper()
	cl := testkit.Isolated(ctx, t)
	if _, err := identity.SyncAuthzCatalog(ctx, open(t, cl.ModuleDSN("identity", "migrator"))); err != nil { // F012 catalog + system roles
		t.Fatal(err)
	}
	e := &env{app: open(t, cl.ModuleDSN("identity", "app")), platform: open(t, cl.ModuleDSN("identity", "platform")),
		relay: open(t, cl.ModuleDSN("identity", "relay")), cl: cl}
	w, err := outbox.NewWriter("identity", "core", nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := outbox.NewAuditRecorder(w, e.app)
	// Composition-root hooks: cross-tenant scopes and authz denials are audited through the outbox.
	tenancy.Configure(tenancy.Hooks{Recorder: rec})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	dbs := entx.DBs{App: e.app, Platform: e.platform}
	if e.uow, err = entx.NewUnitOfWork(entx.Options{Module: "identity", DBs: dbs, Events: w, Audit: rec}); err != nil {
		t.Fatal(err)
	}
	if e.read, err = entx.NewReadSession("identity", dbs); err != nil {
		t.Fatal(err)
	}
	e.deps = command.TenantDeps{UoW: e.uow, Tenants: write.TenantRepo{}, Events: write.Publisher{W: w}, Audit: write.Auditor{R: rec}}
	return e
}

type kcStub struct {
	port.KeycloakAdmin
	created, updated int
}

func (k *kcStub) EnsureOrganization(context.Context, port.KCOrganization) (string, error) {
	k.created++
	return uuid.NewString(), nil
}
func (k *kcStub) UpdateOrganization(context.Context, port.KCOrganization) error {
	k.updated++
	return nil
}

// F010-AC-01..AC-05 with the real repository, outbox and read model.
func TestTenantLifecycle(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))

	created, err := command.CreateTenant{TenantDeps: e.deps}.Handle(sa, dto.CreateTenant{Code: "th-demo-x", Name: "TH Demo X",
		TenantType: "school", Alias: "th-demo-x", Reason: "pilot"})
	if err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'tenant.created' AND tenant_id = $1`, created.TenantID); n != 1 {
		t.Fatalf("tenant.created events = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND envelope->'payload'->>'action' = 'tenant.created'`); n != 1 {
		t.Fatalf("audit tenant.created = %d", n)
	}
	// F010-AC-02: the seeded common tenant blocks a second one; duplicate alias → 409.
	for _, in := range []dto.CreateTenant{
		{Code: "common-2", Name: "x", TenantType: "common", Alias: "common-2"},
		{Code: "other", Name: "x", TenantType: "unit", Alias: "th-demo-x"},
	} {
		if _, err := (command.CreateTenant{TenantDeps: e.deps}).Handle(sa, in); code(err) != problem.Conflict {
			t.Fatalf("%+v: %v (want 409)", in, err)
		}
	}
	// F010-AC-03: disable, events and organization sync through the real repository.
	disabled := "disabled"
	upd, err := command.UpdateTenant{TenantDeps: e.deps}.Handle(sa, dto.UpdateTenant{ID: created.TenantID, IfMatch: created.Version, Status: &disabled, Reason: "test"})
	if err != nil || upd.Status != "disabled" || upd.Version != created.Version+1 {
		t.Fatalf("update = %+v %v", upd, err)
	}
	if _, err := (command.UpdateTenant{TenantDeps: e.deps}).Handle(sa, dto.UpdateTenant{ID: created.TenantID, IfMatch: created.Version, Status: &disabled, Reason: "x"}); code(err) != problem.VersionMismatch {
		t.Fatalf("stale If-Match: %v", err)
	}
	kc := &kcStub{}
	sync := command.SyncTenantOrganization{Tenants: write.TenantRepo{}, KC: kc}
	scope, _ := authz.EventScope(created.TenantID)
	if err := e.uow.InTenantTx(ctx, scope, func(ctx context.Context) error { return sync.Handle(ctx, created.TenantID) }); err != nil {
		t.Fatal(err)
	}
	if kc.created != 1 || kc.updated != 1 {
		t.Fatalf("keycloak calls created=%d updated=%d (disabled tenant → ensure then disable)", kc.created, kc.updated)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM platform.tenants WHERE tenant_id = $1 AND keycloak_org_id IS NOT NULL`, created.TenantID); n != 1 {
		t.Fatal("keycloak_org_id not recorded")
	}

	// listTenants: the seeded common tenant plus th-demo-x, keyset pagination.
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	list := query.ListTenants{Model: read.TenantModel{Session: e.read}, Cursors: codec}
	p1, err := list.Handle(sa, query.ListTenantsInput{Limit: 1})
	if err != nil || len(p1.Data) != 1 || !p1.Page.HasMore {
		t.Fatalf("page 1 = %+v %v", p1, err)
	}
	p2, err := list.Handle(sa, query.ListTenantsInput{Limit: 1, Cursor: *p1.Page.NextCursor})
	if err != nil || len(p2.Data) != 1 || p2.Data[0].Code == p1.Data[0].Code {
		t.Fatalf("page 2 = %+v %v", p2, err)
	}
	f, err := list.Handle(sa, query.ListTenantsInput{Q: "DEMO", TenantType: "school"})
	if err != nil || len(f.Data) != 1 || f.Data[0].Alias != "th-demo-x" {
		t.Fatalf("filter = %+v %v", f, err)
	}
}

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}
