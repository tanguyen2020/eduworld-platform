package contract

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// fakeChatAssign is a TEST DOUBLE (R17) of the sticker pack assignments of the chat facade (F082 T040): it records the
// call context and answers like specs/openapi/modules/chat.internal.v1.yaml (TenantAssignmentList). The real facade is
// tested by module chat (tests/integration/chat_assignments_test.go).
type fakeChatAssign struct {
	fakeLearning
	last *chatapi.ApplyTenantAssignmentsJSONRequestBody
}

func chatList(rt chatapi.CommonInternalCommonV1ResourceType, id uuid.UUID, tenants []uuid.UUID, status string) chatapi.CommonInternalCommonV1TenantAssignmentList {
	out := chatapi.CommonInternalCommonV1TenantAssignmentList{}
	for _, tn := range tenants {
		out.Data = append(out.Data, chatapi.CommonInternalCommonV1TenantAssignment{AssignmentId: uuid.New(), ResourceType: rt, ResourceId: id,
			TenantId: tn, AssignedBy: uuid.New(), AssignedAt: time.Now().UTC(), Status: chatapi.CommonInternalCommonV1TenantAssignmentStatus(status)})
	}
	return out
}

func (f *fakeChatAssign) ApplyTenantAssignments(ctx context.Context, req chatapi.ApplyTenantAssignmentsRequestObject) (chatapi.ApplyTenantAssignmentsResponseObject, error) {
	if err := f.record(ctx, "applyTenantAssignments"); err != nil {
		return nil, err
	}
	f.last = req.Body
	return chatapi.ApplyTenantAssignments200JSONResponse(chatList(req.Body.ResourceType, req.Body.ResourceId, req.Body.TenantIds, "active")), nil
}

func (f *fakeChatAssign) RevokeTenantAssignments(ctx context.Context, req chatapi.RevokeTenantAssignmentsRequestObject) (chatapi.RevokeTenantAssignmentsResponseObject, error) {
	if err := f.record(ctx, "revokeTenantAssignments"); err != nil {
		return nil, err
	}
	return chatapi.RevokeTenantAssignments200JSONResponse(chatList(req.Body.ResourceType, req.Body.ResourceId, req.Body.TenantIds, "revoked")), nil
}

func (f *fakeChatAssign) ListResourceAssignments(ctx context.Context, req chatapi.ListResourceAssignmentsRequestObject) (chatapi.ListResourceAssignmentsResponseObject, error) {
	if err := f.record(ctx, "listResourceAssignments"); err != nil {
		return nil, err
	}
	return chatapi.ListResourceAssignments200JSONResponse(chatList(req.Params.ResourceType, req.Params.ResourceId, []uuid.UUID{uuid.New()}, "active")), nil
}

// F082 T040 / F082-AC-04 (caller side): module identity routes the assignments of a sticker pack to the chat facade
// through pkg/modcall (caller identity, Principal, Idempotency-Key); sticker_pack is owned by chat and assignable, never
// clonable; not wired is 503.
func TestOwnerGatewayChatAssignments(t *testing.T) {
	if m, err := platformadmin.Owner("sticker_pack"); err != nil || m != platformadmin.ModuleChat || !platformadmin.Assignable("sticker_pack") ||
		platformadmin.Clonable("sticker_pack") {
		t.Fatalf("routing: %q %v", m, err)
	}
	sa := authztest.Platform("system_admin")
	key := uuid.NewString()
	ctx := modcall.WithIdempotencyKey(authztest.Ctx(context.Background(), sa), key)
	fake := &fakeChatAssign{}
	g := modules.Gateway{ChatAssign: fake}
	id, ta := uuid.New(), uuid.New()
	got, err := g.ApplyAssignments(ctx, "chat", "sticker_pack", id, []uuid.UUID{ta})
	if err != nil || len(got) != 1 || got[0].TenantID != ta || !got[0].Active || fake.last == nil || fake.last.ResourceType != "sticker_pack" {
		t.Fatalf("apply: %+v %v", got, err)
	}
	c := fake.seen[0]
	if c.caller != "identity" || c.principal.UserID != sa.UserID || c.key != key {
		t.Fatalf("call context %+v", c)
	}
	if got, err := g.RevokeAssignments(ctx, "chat", "sticker_pack", id, []uuid.UUID{ta}, "Hết mùa"); err != nil || len(got) != 1 || got[0].Active {
		t.Fatalf("revoke: %+v %v", got, err)
	}
	if p, err := g.ListAssignments(ctx, "chat", "sticker_pack", id, "", 10); err != nil || len(p.Data) != 1 {
		t.Fatalf("list: %+v %v", p, err)
	}
	if _, err := (modules.Gateway{}).ApplyAssignments(ctx, "chat", "sticker_pack", id, []uuid.UUID{ta}); err == nil {
		t.Fatal("not wired must fail")
	}
}
