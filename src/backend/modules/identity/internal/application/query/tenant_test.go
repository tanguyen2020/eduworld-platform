package query_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// fakeModel applies the keyset contract of read.TenantReadModel over an in-memory list.
type fakeModel struct {
	rows  []dto.Tenant
	calls int
	scope tenancy.Scope
}

func (m *fakeModel) ListTenants(ctx context.Context, f read.TenantFilter) ([]dto.Tenant, error) {
	m.calls++
	m.scope, _ = tenancy.ScopeFrom(ctx)
	var out []dto.Tenant
	for _, r := range m.rows {
		if f.Q != "" && !strings.Contains(r.Code, f.Q) {
			continue
		}
		if f.After != nil && (r.Code < f.After.Key || (r.Code == f.After.Key && r.TenantID.String() <= f.After.ID.String())) {
			continue
		}
		out = append(out, r)
		if len(out) == f.Limit+1 {
			break
		}
	}
	return out, nil
}

func codec(t *testing.T) *page.Codec {
	c, err := page.NewCodec([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestListTenantsPaging(t *testing.T) {
	m := &fakeModel{}
	for _, c := range []string{"a-1", "a-2", "a-3", "b-1", "b-2"} {
		m.rows = append(m.rows, dto.Tenant{TenantID: uuid.New(), Code: c})
	}
	sort.Slice(m.rows, func(i, j int) bool { return m.rows[i].Code < m.rows[j].Code })
	h := query.ListTenants{Model: m, Cursors: codec(t)}
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	p1, err := h.Handle(sa, query.ListTenantsInput{Limit: 2})
	if err != nil || len(p1.Data) != 2 || !p1.Page.HasMore || p1.Page.NextCursor == nil || p1.Page.Limit != 2 {
		t.Fatalf("page 1 = %+v %v", p1, err)
	}
	if _, ok := m.scope.(tenancy.PlatformScope); !ok {
		t.Fatalf("read with scope %T, want PlatformScope", m.scope)
	}
	var codes []string
	cur := *p1.Page.NextCursor
	codes = append(codes, p1.Data[0].Code, p1.Data[1].Code)
	for cur != "" {
		p, err := h.Handle(sa, query.ListTenantsInput{Limit: 2, Cursor: cur})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range p.Data {
			codes = append(codes, d.Code)
		}
		cur = ""
		if p.Page.NextCursor != nil {
			cur = *p.Page.NextCursor
		}
	}
	if strings.Join(codes, ",") != "a-1,a-2,a-3,b-1,b-2" {
		t.Fatalf("codes = %v", codes)
	}
	last, _ := h.Handle(sa, query.ListTenantsInput{Q: "b-", Limit: 5})
	if len(last.Data) != 2 || last.Page.HasMore || last.Page.NextCursor != nil {
		t.Fatalf("filtered page = %+v", last)
	}
}

// F010-AC-04 (read): tenant roles never list tenants; invalid cursor and type are rejected before reading.
func TestListTenantsAuthzAndInput(t *testing.T) {
	m := &fakeModel{}
	h := query.ListTenants{Model: m, Cursors: codec(t)}
	authztest.ExpectDenied(t, func() error {
		_, err := h.Handle(authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin")), query.ListTenantsInput{})
		return err
	}())
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	var pe *problem.Error
	if _, err := h.Handle(sa, query.ListTenantsInput{Cursor: "forged"}); !errors.As(err, &pe) || pe.Code != problem.BadRequest {
		t.Fatalf("forged cursor: %v", err)
	}
	if _, err := h.Handle(sa, query.ListTenantsInput{TenantType: "district"}); !errors.As(err, &pe) || pe.Code != problem.ValidationFailed {
		t.Fatalf("bad tenant_type: %v", err)
	}
	if m.calls != 0 {
		t.Fatal("read model reached for a rejected request")
	}
	if out, err := h.Handle(sa, query.ListTenantsInput{}); err != nil || out.Data == nil || out.Page.Limit != 20 {
		t.Fatalf("empty list = %+v %v (data must be [] and limit default 20)", out, err)
	}
}
