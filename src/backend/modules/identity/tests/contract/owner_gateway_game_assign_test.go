package contract

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// fakeGameAssign is a TEST DOUBLE (R17) of the tenant assignment operations of the game facade (F078): it records the
// call context and answers like specs/openapi/modules/game.internal.v1.yaml (TenantAssignmentList). The real facade is
// tested by module game (tests/integration/gameadmin_test.go).
type fakeGameAssign struct {
	fakeLearning
	last     *gameapi.ApplyTenantAssignmentsJSONRequestBody
	assigned map[uuid.UUID]bool // tenants with an active assignment: apply answers created = false for them
}

func list(rt gameapi.CommonInternalCommonV1ResourceType, id uuid.UUID, tenants []uuid.UUID, status string) gameapi.CommonInternalCommonV1TenantAssignmentList {
	out := gameapi.CommonInternalCommonV1TenantAssignmentList{}
	for _, tn := range tenants {
		out.Data = append(out.Data, gameapi.CommonInternalCommonV1TenantAssignment{AssignmentId: uuid.New(), ResourceType: rt, ResourceId: id,
			TenantId: tn, AssignedBy: uuid.New(), AssignedAt: time.Now().UTC(), Status: gameapi.CommonInternalCommonV1TenantAssignmentStatus(status)})
	}
	return out
}

func (f *fakeGameAssign) ApplyTenantAssignments(ctx context.Context, req gameapi.ApplyTenantAssignmentsRequestObject) (gameapi.ApplyTenantAssignmentsResponseObject, error) {
	if err := f.record(ctx, "applyTenantAssignments"); err != nil {
		return nil, err
	}
	f.last = req.Body
	out := list(req.Body.ResourceType, req.Body.ResourceId, req.Body.TenantIds, "active")
	if f.assigned == nil {
		f.assigned = map[uuid.UUID]bool{}
	}
	for i := range out.Data {
		created := !f.assigned[out.Data[i].TenantId]
		f.assigned[out.Data[i].TenantId] = true
		out.Data[i].Created = &created
	}
	return gameapi.ApplyTenantAssignments200JSONResponse(out), nil
}

func (f *fakeGameAssign) RevokeTenantAssignments(ctx context.Context, req gameapi.RevokeTenantAssignmentsRequestObject) (gameapi.RevokeTenantAssignmentsResponseObject, error) {
	if err := f.record(ctx, "revokeTenantAssignments"); err != nil {
		return nil, err
	}
	return gameapi.RevokeTenantAssignments200JSONResponse(list(req.Body.ResourceType, req.Body.ResourceId, req.Body.TenantIds, "revoked")), nil
}

func (f *fakeGameAssign) ListResourceAssignments(ctx context.Context, req gameapi.ListResourceAssignmentsRequestObject) (gameapi.ListResourceAssignmentsResponseObject, error) {
	if err := f.record(ctx, "listResourceAssignments"); err != nil {
		return nil, err
	}
	return gameapi.ListResourceAssignments200JSONResponse(list(req.Params.ResourceType, req.Params.ResourceId, []uuid.UUID{uuid.New()}, "active")), nil
}

// F078-AC-21 (caller side): module identity routes the assignments of a game resource (quest, minigame_config, item,
// shop_item, event_template) to the game facade through pkg/modcall (caller identity, Principal, Idempotency-Key);
// not wired is still 503.
func TestOwnerGatewayGameAssignments(t *testing.T) {
	sa := authztest.Platform("system_admin")
	key := uuid.NewString()
	ctx := modcall.WithIdempotencyKey(authztest.Ctx(context.Background(), sa), key)
	fake := &fakeGameAssign{}
	g := modules.Gateway{GameAssign: fake}
	id, ta := uuid.New(), uuid.New()
	got, err := g.ApplyAssignments(ctx, "game", "minigame_config", id, []uuid.UUID{ta})
	if err != nil || len(got) != 1 || got[0].TenantID != ta || !got[0].Active || fake.last == nil || fake.last.ResourceType != "minigame_config" {
		t.Fatalf("apply: %+v %v", got, err)
	}
	// F045 Clarifications 2026-11-28: the created flag of the contract reaches the port; a second apply of the same tenant is no
	// change (identity publishes no tenant_data.assigned for it), a flag the owner did not send counts as a change.
	if got[0].Created == nil || !got[0].Changed() {
		t.Fatalf("first apply must be a change: %+v", got[0])
	}
	tb := uuid.New()
	again, err := g.ApplyAssignments(ctx, "game", "minigame_config", id, []uuid.UUID{ta, tb})
	if err != nil || len(again) != 2 || again[0].Changed() || !again[1].Changed() {
		t.Fatalf("re-apply: %+v %v", again, err)
	}
	if (port.ResourceAssignment{}).Changed() != true {
		t.Fatal("an unknown created flag must count as a change")
	}
	c := fake.seen[0]
	if c.caller != "identity" || c.principal.UserID != sa.UserID || c.key != key {
		t.Fatalf("call context %+v", c)
	}
	if got, err := g.RevokeAssignments(ctx, "game", "shop_item", id, []uuid.UUID{ta}, "Hết hạn"); err != nil || len(got) != 1 || got[0].Active {
		t.Fatalf("revoke: %+v %v", got, err)
	}
	if p, err := g.ListAssignments(ctx, "game", "event_template", id, "", 10); err != nil || len(p.Data) != 1 {
		t.Fatalf("list: %+v %v", p, err)
	}
	if _, err := (modules.Gateway{}).ApplyAssignments(ctx, "game", "quest", id, []uuid.UUID{ta}); err == nil {
		t.Fatal("not wired must fail")
	}
}
