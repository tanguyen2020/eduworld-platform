package resourceassignment

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

func TestAssignmentRules(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	got, err := Tenants([]uuid.UUID{a, b, a})
	if err != nil || !slices.Equal(got, []uuid.UUID{a, b}) {
		t.Fatalf("Tenants dedup = %v %v", got, err)
	}
	for _, bad := range [][]uuid.UUID{nil, {uuid.Nil}, make([]uuid.UUID, MaxTenants+1)} {
		if _, err := Tenants(bad); err == nil {
			t.Fatalf("Tenants(%d ids) accepted", len(bad))
		}
	}
	tenant := uuid.New()
	if _, err := New(uuid.New(), contentmodel.ResourceQuestion, uuid.New(), contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: &tenant},
		a, uuid.New(), time.Now()); !errors.Is(err, ErrNotPlatform) {
		t.Fatalf("tenant resource assigned: %v", err)
	}
	x, err := New(uuid.New(), contentmodel.ResourceQuestion, uuid.New(), contentmodel.Owner{Scope: contentmodel.OwnerPlatform}, a, uuid.New(), time.Now())
	if err != nil || !x.Active() {
		t.Fatalf("New = %+v %v", x, err)
	}
	revoked := x
	now := time.Now()
	revoked.TenantID, revoked.RevokedAt = b, &now
	if m := Missing([]uuid.UUID{a, b}, []Assignment{x, revoked}); !slices.Equal(m, []uuid.UUID{b}) {
		t.Fatalf("Missing = %v (a revoked assignment does not count)", m)
	}
}
