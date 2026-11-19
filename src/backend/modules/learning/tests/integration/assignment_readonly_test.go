//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F020-AC-18 (TC-CMS-ASG-01, TC-CMS-ASG-02): assigned platform content is read-only for the tenant (write →
// 403 PERMISSION_DENIED, and RLS refuses the update); referencing an unassigned platform resource is
// 403 RESOURCE_NOT_ASSIGNED; another tenant's resource is 404.
func TestAssignedContentReadOnly(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	f := e.facadeOf(t, nil)
	sa, _ := systemAdmin(ctx)
	assigned := platformItem(ctx, t, e, "p-assigned")
	unassigned := platformItem(ctx, t, e, "p-unassigned")
	own := tenantItem(ctx, t, e, e.tenantA, "a-1")
	other := tenantItem(ctx, t, e, e.tenantB, "b-1")
	if _, err := apply(sa, f, newKey(), "question", assigned, e.tenantA); err != nil {
		t.Fatal(err)
	}
	staff := authztest.Staff(e.tenantA, "tenant_content_editor")
	staffCtx := authztest.Ctx(ctx, staff)

	// Write rule on the owner loaded under the tenant scope.
	var owners map[uuid.UUID]contentmodel.Owner
	if err := e.read.Read(staffCtx, func(ctx context.Context) error {
		var err error
		owners, err = fixtures.Handler{}.Owners(ctx, []uuid.UUID{assigned, own, other})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, seen := owners[other]; seen || len(owners) != 2 {
		t.Fatalf("owners visible to A = %v", owners)
	}
	if err := command.RequireWritable(staff, owners[assigned]); code(err) != problem.PermissionDenied {
		t.Fatalf("write on assigned platform content: %v", err)
	}
	if err := command.RequireWritable(staff, owners[own]); err != nil {
		t.Fatalf("write on own content: %v", err)
	}
	// Defense in depth: RLS lets tenant A update no platform row even when assigned.
	err := e.uow.Do(staffCtx, func(ctx context.Context) error {
		n, err := fixtures.Client(ctx).FixtureContent.Update().SetTitle("hijack").Save(ctx)
		if err == nil && n != 1 { // only its own row
			t.Errorf("tenant A updated %d rows", n)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE fixture_content_id = $1 AND title = 'hijack'`, assigned); n != 0 {
		t.Fatal("assigned platform content was changed by the tenant")
	}

	// References.
	reg, err := port.NewRegistry(fixtures.Handler{})
	if err != nil {
		t.Fatal(err)
	}
	check := query.CheckReference{Session: e.read, Resources: reg}
	cases := map[uuid.UUID]problem.Code{assigned: "", own: "", unassigned: problem.ResourceNotAssigned, other: problem.NotFound, uuid.New(): problem.NotFound}
	for id, want := range cases {
		if got := code(check.Handle(staffCtx, contentmodel.ResourceQuestion, id)); got != want {
			t.Errorf("reference %s: %q, want %q", id, got, want)
		}
	}
}
