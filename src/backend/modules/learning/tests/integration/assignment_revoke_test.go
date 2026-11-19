//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

func valkeyClient(ctx context.Context, t *testing.T) valkey.Client {
	t.Helper()
	_, addr := testkit.StartValkey(ctx, t)
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

func poolKey(tenant uuid.UUID) string { return "ew:learning:qb:pool:" + tenant.String() + ":g3" }

func exists(ctx context.Context, t *testing.T, c valkey.Client, key string) bool {
	t.Helper()
	n, err := c.Do(ctx, c.B().Exists().Key(key).Build()).AsInt64()
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// F020-AC-17 (TC-TEN-019): revokeTenantAssignments soft-deletes the assignment; the tenant no longer sees the
// resource at its next read while data pinned to it (attempt history) stays. F020-AC-20: assign/revoke record
// audit.recorded (actor = the original Principal) in the same transaction and drop the tenant's cache.
// F020-AC-14: an overlong reason is 422 naming the field.
func TestRevokeTenantAssignments(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	vk := valkeyClient(ctx, t)
	f := e.facadeOf(t, cache.Invalidator{C: vk})
	sa, saP := systemAdmin(ctx)
	item := platformItem(ctx, t, e, "p-1")
	seed := func() {
		for _, tn := range []uuid.UUID{e.tenantA, e.tenantB} {
			if err := vk.Do(ctx, vk.B().Set().Key(poolKey(tn)).Value("cached").Build()).Error(); err != nil {
				t.Fatal(err)
			}
		}
	}

	seed()
	if _, err := apply(sa, f, newKey(), "question", item, e.tenantA, e.tenantB); err != nil {
		t.Fatal(err)
	}
	if exists(ctx, t, vk, poolKey(e.tenantA)) || exists(ctx, t, vk, poolKey(e.tenantB)) {
		t.Fatal("apply did not invalidate the tenants' cache")
	}
	// Tenant A records an attempt on the resource (student data pinned to it).
	attempt := ids.New()
	if err := e.uow.Do(authztest.Ctx(ctx, authztest.Staff(e.tenantA, "school_admin")), func(ctx context.Context) error {
		return fixtures.Client(ctx).FixtureAttempt.Create().SetID(attempt).SetStudentID(uuid.New()).
			SetFixtureContentVersionID(item).SetScore(7).Exec(ctx)
	}); err != nil {
		t.Fatal(err)
	}

	seed()
	if _, err := revoke(sa, f, newKey(), "x", item, e.tenantA); code(err) != problem.ValidationFailed {
		t.Fatalf("reason too short: %v", err)
	}
	_, err := revoke(sa, f, newKey(), strings.Repeat("r", 1001), item, e.tenantA)
	var pe *problem.Error
	if !asProblem(err, &pe) || pe.Code != problem.ValidationFailed || pe.Fields[0].Field != "reason" {
		t.Fatalf("reason of 1001 characters: %v", err)
	}
	out, err := revoke(sa, f, newKey(), "hết hợp đồng nội dung", item, e.tenantA)
	if err != nil || len(out.Data) != 1 || out.Data[0].Status != "revoked" || out.Data[0].TenantId != e.tenantA {
		t.Fatalf("revoke = %+v %v", out.Data, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_id = $1 AND tenant_id = $2
		AND deleted_at IS NOT NULL AND deleted_by = $3 AND delete_reason LIKE 'unassigned:%'`, item, e.tenantA, saP.UserID); n != 1 {
		t.Fatal("assignment of A not soft-deleted by the System Admin")
	}
	if visibleTo(ctx, t, e, e.tenantA, item) || !visibleTo(ctx, t, e, e.tenantB, item) {
		t.Fatal("after revoke: A must not see the resource, B still must")
	}
	if exists(ctx, t, vk, poolKey(e.tenantA)) || !exists(ctx, t, vk, poolKey(e.tenantB)) {
		t.Fatal("revoke must drop only tenant A's cache")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND tenant_id = $1
		AND envelope->'payload'->>'action' = 'tenant_data.unassigned' AND envelope->'payload'->>'actor_id' = $2`, e.tenantA, saP.UserID.String()); n != 1 {
		t.Fatalf("audit tenant_data.unassigned (actor = System Admin) = %d", n)
	}
	// History pinned to the resource is kept.
	if err := e.read.Read(authztest.Ctx(ctx, authztest.Staff(e.tenantA, "school_admin")), func(ctx context.Context) error {
		a, err := fixtures.Client(ctx).FixtureAttempt.Get(ctx, attempt)
		if err == nil && a.Score != 7 {
			t.Fatalf("attempt changed: %+v", a)
		}
		return err
	}); err != nil {
		t.Fatalf("attempt after revoke: %v", err)
	}
	// Revoking a tenant without an active assignment is a no-op; re-assigning creates a new row.
	if out, err := revoke(sa, f, newKey(), "again", item, e.tenantA); err != nil || len(out.Data) != 0 {
		t.Fatalf("second revoke = %+v %v", out.Data, err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := apply(sa, f, newKey(), "question", item, e.tenantA); err != nil || !visibleTo(ctx, t, e, e.tenantA, item) {
		t.Fatalf("re-assign after revoke: %v", err)
	}
}
