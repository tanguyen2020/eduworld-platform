//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func activeAssignments(ctx context.Context, t *testing.T, e *env, item uuid.UUID) int {
	t.Helper()
	return count(ctx, t, e.platform, `SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_id = $1 AND deleted_at IS NULL`, item)
}

// F020-AC-15 (TC-TEN-019): applyTenantAssignments creates one active assignment per (resource, tenant) not yet
// assigned and returns them; the same Idempotency-Key replays the first result; assigning again creates nothing.
func TestApplyTenantAssignments(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	f := e.facadeOf(t, nil)
	sa, saP := systemAdmin(ctx)
	item := platformItem(ctx, t, e, "p-1")

	key := newKey()
	first, err := apply(sa, f, key, "question", item, e.tenantA, e.tenantB)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Data) != 2 || first.Data[0].TenantId != e.tenantA || first.Data[1].TenantId != e.tenantB ||
		first.Data[0].Status != "active" || first.Data[0].AssignedBy != saP.UserID {
		t.Fatalf("apply = %+v", first.Data)
	}
	if n := activeAssignments(ctx, t, e, item); n != 2 {
		t.Fatalf("active assignments = %d", n)
	}
	for _, tn := range []uuid.UUID{e.tenantA, e.tenantB} {
		if n := e.audits(ctx, t, "tenant_data.assigned", &tn); n != 1 {
			t.Fatalf("audit tenant_data.assigned for %s = %d", tn, n)
		}
	}
	if !visibleTo(ctx, t, e, e.tenantA, item) || !visibleTo(ctx, t, e, e.tenantB, item) {
		t.Fatal("assigned tenants do not see the resource")
	}

	// Same key: the stored result is replayed (same assignment ids), nothing is written.
	again, err := apply(sa, f, key, "question", item, e.tenantA, e.tenantB)
	if err != nil || !slices.Equal([]uuid.UUID{again.Data[0].AssignmentId, again.Data[1].AssignmentId},
		[]uuid.UUID{first.Data[0].AssignmentId, first.Data[1].AssignmentId}) {
		t.Fatalf("replay = %+v %v", again.Data, err)
	}
	// Same key with another request → 422 IDEMPOTENCY_KEY_REUSED.
	if _, err := apply(sa, f, key, "question", item, e.tenantA); code(err) != problem.IdempotencyKeyReused {
		t.Fatalf("key reused: %v", err)
	}
	// New key, already assigned tenant: no new row, the existing assignment is returned.
	re, err := apply(sa, f, newKey(), "question", item, e.tenantA)
	if err != nil || len(re.Data) != 1 || re.Data[0].AssignmentId != first.Data[0].AssignmentId {
		t.Fatalf("re-assign = %+v %v", re.Data, err)
	}
	if n := activeAssignments(ctx, t, e, item); n != 2 {
		t.Fatalf("active assignments after re-assign = %d", n)
	}
	if n := e.audits(ctx, t, "tenant_data.assigned", nil); n != 2 {
		t.Fatalf("audits after replay/re-assign = %d", n)
	}

	// listResourceAssignments returns the active assignments across tenants (bypass audited).
	res, err := asIdentity(sa, "listResourceAssignments", "", func(ctx context.Context) (any, error) {
		return f.ListResourceAssignments(ctx, listReq(item))
	})
	if err != nil || len(listData(res)) != 2 {
		t.Fatalf("list = %+v %v", res, err)
	}
}

// F020-AC-16: a resource that is not a live platform resource, or a tenant that is unknown or not active, is
// 422 VALIDATION_FAILED listing every element, and no assignment is written.
func TestApplyTenantAssignmentsInvalid(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	f := e.facadeOf(t, nil)
	sa, _ := systemAdmin(ctx)
	platform := platformItem(ctx, t, e, "p-1")
	own := tenantItem(ctx, t, e, e.tenantA, "a-1")
	gone := platformItem(ctx, t, e, "p-gone")
	if err := e.uow.Do(authztest.Ctx(ctx, authztest.Platform("content_admin")), func(ctx context.Context) error {
		return fixtures.Client(ctx).FixtureContent.DeleteOneID(gone).Exec(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	fieldsOf := func(err error) []string {
		var pe *problem.Error
		if !asProblem(err, &pe) || pe.Code != problem.ValidationFailed {
			t.Fatalf("want 422, got %v", err)
		}
		var out []string
		for _, f := range pe.Fields {
			out = append(out, f.Field)
		}
		return out
	}
	for _, id := range []uuid.UUID{own, gone, uuid.New()} {
		_, err := apply(sa, f, newKey(), "question", id, e.tenantA)
		if got := fieldsOf(err); !slices.Equal(got, []string{"resource_id"}) {
			t.Fatalf("resource %s: %v", id, got)
		}
	}
	_, err := apply(sa, f, newKey(), "question", platform, e.tenantA, e.tenantOff, uuid.New())
	if got := fieldsOf(err); !slices.Equal(got, []string{"tenant_ids[1]", "tenant_ids[2]"}) {
		t.Fatalf("tenants: %v", got)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM infra.tenant_resource_assignments`); n != 0 {
		t.Fatalf("%d assignments written by invalid requests", n)
	}
	// A resource type of another module or without a registered handler.
	if _, err := apply(sa, f, newKey(), "quest", platform, e.tenantA); code(err) != problem.ValidationFailed {
		t.Fatalf("quest: %v", err)
	}
	if _, err := apply(sa, f, newKey(), "lesson", platform, e.tenantA); code(err) != problem.ValidationFailed {
		t.Fatalf("lesson without handler: %v", err)
	}
}
