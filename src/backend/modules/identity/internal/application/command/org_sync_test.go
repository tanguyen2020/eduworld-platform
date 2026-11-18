package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
)

// fakeKC records Organization calls of port.KeycloakAdmin (other methods unused here).
type fakeKC struct {
	port.KeycloakAdmin
	orgs    map[string]port.KCOrganization // alias → org
	updates []port.KCOrganization
	fail    error
}

func (f *fakeKC) EnsureOrganization(_ context.Context, o port.KCOrganization) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	if cur, ok := f.orgs[o.Alias]; ok {
		return cur.ID, nil
	}
	o.ID = uuid.NewString()
	f.orgs[o.Alias] = o
	return o.ID, nil
}

func (f *fakeKC) UpdateOrganization(_ context.Context, o port.KCOrganization) error {
	if f.fail != nil {
		return f.fail
	}
	f.updates = append(f.updates, o)
	return nil
}

type orgTenants struct {
	*fakeTenants
	orgSet int
}

func (o *orgTenants) SetOrgID(_ context.Context, id, org uuid.UUID) error {
	o.orgSet++
	o.rows[id].OrgID = &org
	return nil
}

// F010-AC-01: one Organization per tenant with tenant_id/tenant_type attributes, id recorded once.
// F010-AC-03: disabled tenant → Organization enabled=false. F010-AC-23: Keycloak error → error, no state.
func TestSyncTenantOrganization(t *testing.T) {
	repo := &orgTenants{fakeTenants: newTenants()}
	kc := &fakeKC{orgs: map[string]port.KCOrganization{}}
	tn, _ := tenant.New(uuid.New(), "th-demo-x", "TH Demo X", tenant.TypeSchool, "th-demo-x", "")
	repo.rows[tn.ID] = tn
	h := command.SyncTenantOrganization{Tenants: repo, KC: kc}
	ctx := context.Background()

	if err := h.Handle(ctx, tn.ID); err != nil {
		t.Fatal(err)
	}
	org := kc.orgs["th-demo-x"]
	if !org.Enabled || org.Attributes["tenant_id"][0] != tn.ID.String() || org.Attributes["tenant_type"][0] != "school" {
		t.Fatalf("organization = %+v", org)
	}
	if repo.orgSet != 1 || repo.rows[tn.ID].OrgID.String() != org.ID || len(kc.updates) != 0 {
		t.Fatalf("org id set %d times, updates %d", repo.orgSet, len(kc.updates))
	}
	// Redelivery: no second Organization, an update converges the state.
	if err := h.Handle(ctx, tn.ID); err != nil || len(kc.orgs) != 1 || repo.orgSet != 1 {
		t.Fatalf("redelivery: %v orgs=%d set=%d", err, len(kc.orgs), repo.orgSet)
	}
	repo.rows[tn.ID].Status = tenant.StatusDisabled
	if err := h.Handle(ctx, tn.ID); err != nil {
		t.Fatal(err)
	}
	if last := kc.updates[len(kc.updates)-1]; last.Enabled || last.ID != org.ID {
		t.Fatalf("disable: %+v", last)
	}
	kc.fail = errors.New("keycloak 503")
	other, _ := tenant.New(uuid.New(), "pgd-y", "PGD Y", tenant.TypeUnit, "pgd-y", "")
	repo.rows[other.ID] = other
	if err := h.Handle(ctx, other.ID); err == nil || repo.rows[other.ID].OrgID != nil {
		t.Fatalf("keycloak failure must fail without recording an org id: %v", err)
	}
	// A tenant created disabled gets the Organization then the enabled=false update.
	kc.fail = nil
	dis, _ := tenant.New(uuid.New(), "pgd-z", "PGD Z", tenant.TypeUnit, "pgd-z", "")
	dis.Status = tenant.StatusDisabled
	repo.rows[dis.ID] = dis
	n := len(kc.updates)
	if err := h.Handle(ctx, dis.ID); err != nil || len(kc.updates) != n+1 || kc.updates[n].Enabled {
		t.Fatalf("created disabled: %v", err)
	}
}
