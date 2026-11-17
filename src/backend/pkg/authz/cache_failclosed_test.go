package authz_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

type fakeResolver struct {
	calls atomic.Int32
	err   error
	p     authz.Principal
}

func (f *fakeResolver) Resolve(_ context.Context, req authz.ResolveRequest) (authz.Principal, error) {
	f.calls.Add(1)
	if f.err != nil {
		return authz.Principal{}, f.err
	}
	return f.p, nil
}

func newCache(t *testing.T, next authz.PrincipalResolver, clk clock.Clock) *authz.CachedResolver {
	t.Helper()
	c, err := authz.NewCachedResolver(next, authz.CacheOptions{Module: "learning", TTL: time.Minute, L1TTL: 15 * time.Second, L1Size: 100, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// F002-AC-09 (TC-SVC-011): identity unavailable and nothing cached → 503 UNAVAILABLE retryable, never allow.
func TestCacheFailsClosed(t *testing.T) {
	tid := uuid.New()
	next := &fakeResolver{err: errors.New("identity db down")}
	c := newCache(t, next, clock.System())
	req := authz.ResolveRequest{Subject: authz.SubjectRef{Issuer: "https://kc/realms/eduworld", Subject: "kc-1"}, TenantID: &tid, ClientID: "eduworld-portal"}
	p, err := c.Resolve(context.Background(), req)
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.Unavailable || !pe.Retryable {
		t.Fatalf("err = %v", err)
	}
	if p.Valid() {
		t.Fatal("principal returned although resolver failed")
	}
	// ErrNotMember passes through unchanged (→ TENANT_MISMATCH in httpx) and is not cached.
	next.err = authz.ErrNotMember
	if _, err := c.Resolve(context.Background(), req); !errors.Is(err, authz.ErrNotMember) {
		t.Fatalf("not member: %v", err)
	}
}

// A cached principal still within TTL is served while identity is down.
func TestCacheServesWhileIdentityDown(t *testing.T) {
	tid := uuid.New()
	want := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &tid,
		Grants: []authz.Grant{{Role: "school_admin", Permissions: []authz.Permission{authz.PermLmsClassRead}}}})
	next := &fakeResolver{p: want}
	clk := clock.NewFake(time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC))
	c := newCache(t, next, clk)
	req := authz.ResolveRequest{Subject: authz.SubjectRef{Issuer: "iss", Subject: "sub"}, TenantID: &tid, ClientID: "eduworld-portal"}
	if _, err := c.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	next.err = errors.New("down")
	got, err := c.Resolve(context.Background(), req)
	if err != nil || got.UserID != want.UserID || !got.Has(authz.PermLmsClassRead) || got.ClientID != "eduworld-portal" {
		t.Fatalf("cached principal not served: %+v %v", got, err)
	}
	if next.calls.Load() != 1 {
		t.Fatalf("resolver calls = %d", next.calls.Load())
	}
	clk.Advance(16 * time.Second) // L1 expired, no Valkey configured → resolver → fails closed
	if _, err := c.Resolve(context.Background(), req); err == nil {
		t.Fatal("expired entry served")
	}
}

func TestInvalidateL1(t *testing.T) {
	tid := uuid.New()
	p := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &tid})
	next := &fakeResolver{p: p}
	c := newCache(t, next, clock.System())
	req := authz.ResolveRequest{Subject: authz.SubjectRef{Issuer: "iss", Subject: "sub"}, TenantID: &tid}
	_, _ = c.Resolve(context.Background(), req)
	if err := c.Invalidate(context.Background(), p.UserID, &tid); err != nil {
		t.Fatal(err)
	}
	_, _ = c.Resolve(context.Background(), req)
	if next.calls.Load() != 2 {
		t.Fatalf("Invalidate did not clear L1: calls = %d", next.calls.Load())
	}
}

func TestInvalidateTenantAndAll(t *testing.T) {
	ta, tb := uuid.New(), uuid.New()
	pa := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &ta})
	next := &fakeResolver{p: pa}
	c := newCache(t, next, clock.System())
	reqA := authz.ResolveRequest{Subject: authz.SubjectRef{Issuer: "iss", Subject: "a"}, TenantID: &ta}
	_, _ = c.Resolve(context.Background(), reqA)
	_ = c.InvalidateTenant(context.Background(), tb)
	_, _ = c.Resolve(context.Background(), reqA)
	if next.calls.Load() != 1 {
		t.Fatal("InvalidateTenant removed another tenant")
	}
	_ = c.InvalidateTenant(context.Background(), ta)
	_, _ = c.Resolve(context.Background(), reqA)
	if next.calls.Load() != 2 {
		t.Fatal("InvalidateTenant kept the tenant")
	}
	_ = c.InvalidateAll(context.Background())
	_, _ = c.Resolve(context.Background(), reqA)
	if next.calls.Load() != 3 {
		t.Fatal("InvalidateAll kept entries")
	}
}
