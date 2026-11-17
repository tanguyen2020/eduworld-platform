//go:build integration

package authz_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F002-AC-10: two-level cache (L1 + Valkey valkey:8); Invalidate clears both levels at once.
func TestCacheTwoLevels(t *testing.T) {
	ctx := context.Background()
	_, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)

	tid := uuid.New()
	p := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &tid,
		Grants: []authz.Grant{{Role: "tenant_admin", Permissions: []authz.Permission{authz.PermTenantUserRead}}}})
	next := &fakeResolver{p: p}
	clk := clock.NewFake(time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC))
	mk := func() *authz.CachedResolver {
		c, err := authz.NewCachedResolver(next, authz.CacheOptions{Module: "learning", TTL: time.Minute, L1TTL: 15 * time.Second,
			L1Size: 100, Valkey: vk, Clock: clk})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	a, b := mk(), mk() // two replicas sharing Valkey
	req := authz.ResolveRequest{Subject: authz.SubjectRef{Issuer: "https://kc/realms/eduworld", Subject: "kc-sub-1"}, TenantID: &tid, ClientID: "eduworld-portal"}

	if _, err := a.Resolve(ctx, req); err != nil {
		t.Fatal(err)
	}
	key := "ew:learning:principal:" + p.UserID.String() + ":" + tid.String()
	if n, err := vk.Do(ctx, vk.B().Exists().Key(key).Build()).AsInt64(); err != nil || n != 1 {
		t.Fatalf("valkey key %s missing: %v", key, err)
	}
	if ttl, _ := vk.Do(ctx, vk.B().Ttl().Key(key).Build()).AsInt64(); ttl <= 0 || ttl > 60 {
		t.Fatalf("ttl = %d, want principal_cache_ttl", ttl)
	}
	// Replica b: L1 miss, Valkey hit → no resolver call.
	got, err := b.Resolve(ctx, req)
	if err != nil || got.UserID != p.UserID || !got.Has(authz.PermTenantUserRead) || got.ClientID != "eduworld-portal" {
		t.Fatalf("valkey hit: %+v %v", got, err)
	}
	if next.calls.Load() != 1 {
		t.Fatalf("resolver calls = %d, want 1", next.calls.Load())
	}

	// Invalidate on replica a clears its L1 and Valkey; b's L1 still holds ≤ L1TTL (bounded staleness),
	// after which b sees the miss as well.
	if err := a.Invalidate(ctx, p.UserID, &tid); err != nil {
		t.Fatal(err)
	}
	if n, _ := vk.Do(ctx, vk.B().Exists().Key(key).Build()).AsInt64(); n != 0 {
		t.Fatal("valkey key not deleted")
	}
	if _, err := a.Resolve(ctx, req); err != nil || next.calls.Load() != 2 {
		t.Fatalf("after invalidate: calls = %d err %v", next.calls.Load(), err)
	}

	// Invalidate(user, nil) removes every tenant of the user.
	if err := a.Invalidate(ctx, p.UserID, nil); err != nil {
		t.Fatal(err)
	}
	if n, _ := vk.Do(ctx, vk.B().Exists().Key(key).Build()).AsInt64(); n != 0 {
		t.Fatal("user-wide invalidate left the key")
	}

	// Valkey down → fall back to the resolver (still correct, not fail-open).
	vk.Close()
	clk.Advance(16 * time.Second)
	if _, err := b.Resolve(ctx, req); err != nil {
		t.Fatalf("valkey down with identity up: %v", err)
	}
}
