package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

type fakeUoW struct{ calls int }

func (u *fakeUoW) Do(ctx context.Context, fn func(context.Context) error) error {
	u.calls++
	return fn(ctx)
}

type fakeEvents struct{ events []write.Event }

func (f *fakeEvents) Publish(_ context.Context, e write.Event) error {
	f.events = append(f.events, e)
	return nil
}

type fakeAudit struct{ events []audit.Event }

func (f *fakeAudit) Record(_ context.Context, e audit.Event) error {
	f.events = append(f.events, e)
	return nil
}

type fakeTenants struct {
	rows  map[uuid.UUID]*tenant.Tenant
	scope tenancy.Scope
}

func newTenants() *fakeTenants { return &fakeTenants{rows: map[uuid.UUID]*tenant.Tenant{}} }

func (f *fakeTenants) Create(ctx context.Context, t *tenant.Tenant) error {
	f.scope, _ = tenancy.ScopeFrom(ctx)
	for _, r := range f.rows {
		if r.Code == t.Code || r.OrgAlias == t.OrgAlias || (r.Type == tenant.TypeCommon && t.Type == tenant.TypeCommon) {
			return problem.New(problem.Conflict, "tenant exists")
		}
	}
	c := *t
	f.rows[t.ID] = &c
	return nil
}

func (f *fakeTenants) Get(_ context.Context, id uuid.UUID) (*tenant.Tenant, error) {
	r, ok := f.rows[id]
	if !ok {
		return nil, problem.New(problem.NotFound, "")
	}
	c := *r
	return &c, nil
}

func (f *fakeTenants) Save(_ context.Context, t *tenant.Tenant) error {
	if f.rows[t.ID].Version != t.Version {
		return problem.New(problem.VersionMismatch, "")
	}
	t.Version++
	c := *t
	f.rows[t.ID] = &c
	return nil
}

func (f *fakeTenants) SetOrgID(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func deps() (command.TenantDeps, *fakeUoW, *fakeTenants, *fakeEvents, *fakeAudit) {
	u, r, e, a := &fakeUoW{}, newTenants(), &fakeEvents{}, &fakeAudit{}
	return command.TenantDeps{UoW: u, Tenants: r, Events: e, Audit: a}, u, r, e, a
}

// F010-AC-01, AC-05: tenant created with event tenant.created (envelope tenant = the tenant) and audit.
func TestCreateTenant(t *testing.T) {
	d, _, repo, ev, au := deps()
	h := command.CreateTenant{TenantDeps: d}
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	out, err := h.Handle(sa, dto.CreateTenant{Code: "th-demo-x", Name: "TH Demo X", TenantType: "school", Alias: "th-demo-x", Reason: "pilot"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "active" || out.TenantType != "school" || out.Alias != "th-demo-x" {
		t.Fatalf("out = %+v", out)
	}
	if s, ok := repo.scope.(tenancy.PlatformScope); !ok || !s.PlatformOnly() {
		t.Fatalf("tenant row written with scope %T, want PlatformScope", repo.scope)
	}
	if len(ev.events) != 1 || ev.events[0].Type != "tenant.created" || *ev.events[0].TenantID != out.TenantID {
		t.Fatalf("events = %+v", ev.events)
	}
	p := ev.events[0].Payload.(command.TenantPayload)
	if p.TenantType != "school" || p.Status != "active" || p.ChangedFields == nil || p.SourceVersion != 1 {
		t.Fatalf("payload = %+v", p)
	}
	if len(au.events) != 1 || au.events[0].Action != "tenant.created" || au.events[0].Reason != "pilot" {
		t.Fatalf("audit = %+v", au.events)
	}
	// F010-AC-02: the common tenant exists once (seed) → 409.
	repo.rows[tenant.CommonTenantID] = &tenant.Tenant{ID: tenant.CommonTenantID, Code: "common", OrgAlias: "common", Type: tenant.TypeCommon}
	if _, err := h.Handle(sa, dto.CreateTenant{Code: "common2", Name: "x", TenantType: "common", Alias: "common2"}); code(err) != problem.Conflict {
		t.Fatalf("second common tenant: %v", err)
	}
}

// F010-AC-01, AC-05: invalid alias / type / lengths → 422 with the field, before any transaction.
func TestCreateTenantValidation(t *testing.T) {
	d, uow, _, _, _ := deps()
	h := command.CreateTenant{TenantDeps: d}
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	for name, in := range map[string]dto.CreateTenant{
		"alias":   {Code: "ok", Name: "n", TenantType: "unit", Alias: "Bad_Alias"},
		"type":    {Code: "ok", Name: "n", TenantType: "district", Alias: "abc"},
		"code":    {Code: "x", Name: "n", TenantType: "unit", Alias: "abc"},
		"no name": {Code: "ok", Name: "", TenantType: "unit", Alias: "abc"},
	} {
		_, err := h.Handle(sa, in)
		var pe *problem.Error
		if !errors.As(err, &pe) || pe.Code != problem.ValidationFailed || len(pe.Fields) != 1 {
			t.Errorf("%s: %v", name, err)
		}
	}
	if uow.calls != 0 {
		t.Fatal("transaction opened for an invalid request")
	}
}

// F010-AC-04: only platform.tenant.manage; tenant roles get 403 and never reach the repository.
func TestCreateTenantAuthz(t *testing.T) {
	d, uow, _, _, _ := deps()
	h := command.CreateTenant{TenantDeps: d}
	in := dto.CreateTenant{Code: "ok", Name: "n", TenantType: "unit", Alias: "abc"}
	for _, ctx := range []context.Context{
		authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin")),
		authztest.Ctx(context.Background(), authztest.Platform("safety_moderator")),
		authztest.Ctx(context.Background(), authztest.Student(authztest.TenantA)),
	} {
		authztest.ExpectDenied(t, func() error { _, err := h.Handle(ctx, in); return err }())
	}
	if _, err := h.Handle(context.Background(), in); code(err) != problem.Unauthenticated {
		t.Fatalf("no principal: %v", err)
	}
	if uow.calls != 0 {
		t.Fatal("repository reached without permission")
	}
}

// F010-AC-03: disabling a tenant emits tenant.updated with changed status; stale If-Match → 412.
func TestUpdateTenant(t *testing.T) {
	d, _, repo, ev, au := deps()
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	created, err := command.CreateTenant{TenantDeps: d}.Handle(sa, dto.CreateTenant{Code: "pgd-a", Name: "PGD A", TenantType: "unit", Alias: "pgd-a"})
	if err != nil {
		t.Fatal(err)
	}
	h := command.UpdateTenant{TenantDeps: d}
	disabled := "disabled"
	out, err := h.Handle(sa, dto.UpdateTenant{ID: created.TenantID, IfMatch: 1, Status: &disabled, Reason: "contract ended"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "disabled" || out.Version != 2 || repo.rows[created.TenantID].Status != tenant.StatusDisabled {
		t.Fatalf("out = %+v", out)
	}
	last := ev.events[len(ev.events)-1]
	if last.Type != "tenant.updated" || last.Payload.(command.TenantPayload).ChangedFields[0] != "status" {
		t.Fatalf("event = %+v", last)
	}
	if au.events[len(au.events)-1].Action != "tenant.updated" {
		t.Fatal("tenant.updated not audited")
	}
	if _, err := h.Handle(sa, dto.UpdateTenant{ID: created.TenantID, IfMatch: 1, Status: &disabled, Reason: "again"}); code(err) != problem.VersionMismatch {
		t.Fatalf("stale If-Match: %v", err)
	}
	n := len(ev.events)
	if _, err := h.Handle(sa, dto.UpdateTenant{ID: created.TenantID, IfMatch: 2, Status: &disabled, Reason: "same"}); err != nil || len(ev.events) != n {
		t.Fatalf("no-op update must succeed without event: %v (%d events)", err, len(ev.events)-n)
	}
	bad := "archived"
	if _, err := h.Handle(sa, dto.UpdateTenant{ID: created.TenantID, IfMatch: 2, Status: &bad, Reason: "x"}); code(err) != problem.ValidationFailed {
		t.Fatalf("bad status: %v", err)
	}
	if _, err := h.Handle(sa, dto.UpdateTenant{ID: uuid.New(), IfMatch: 1, Status: &disabled, Reason: "x"}); code(err) != problem.NotFound {
		t.Fatalf("unknown tenant: %v", err)
	}
}
