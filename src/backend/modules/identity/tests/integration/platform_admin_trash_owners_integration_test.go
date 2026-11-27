//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	competitionapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ownerTrash is a TEST DOUBLE (R17) of listTrash/restoreTrashItem of the game, competition and chat facades behind the real
// gateway: one soft-deleted platform item per listing (none when tenant_ids is set or for a tenant principal, like the
// real owners), restore refused with 404 for a tenant principal. The real facades are tested by each module.
type ownerTrash struct {
	mu       sync.Mutex
	tenants  map[string]int // "<module>:<resource_type>" → len(tenant_ids) of the last listing
	restored []string       // "<module>:<resource_type>:<id>:<reason>"
}

func (o *ownerTrash) list(ctx context.Context, module, rt string, tenants *[]uuid.UUID) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := 0
	if tenants != nil {
		n = len(*tenants)
	}
	o.tenants[module+":"+rt] = n
	p, _ := authz.PrincipalFrom(ctx)
	return n == 0 && p.TenantID == nil
}

func (o *ownerTrash) restore(ctx context.Context, module, rt string, id uuid.UUID, reason string) error {
	if p, _ := authz.PrincipalFrom(ctx); p.TenantID != nil {
		return problem.New(problem.NotFound, "")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.restored = append(o.restored, module+":"+rt+":"+id.String()+":"+reason)
	return nil
}

type ownerGame struct{ *ownerTrash }

func (o ownerGame) ListTrash(ctx context.Context, req gameapi.ListTrashRequestObject) (gameapi.ListTrashResponseObject, error) {
	out := gameapi.ListTrash200JSONResponse{Data: []gameapi.CommonInternalCommonV1TrashItem{}}
	if o.list(ctx, "game", string(*req.Params.ResourceType), req.Params.TenantIds) {
		title, by := "Nhiệm vụ đã xóa", uuid.New()
		out.Data = append(out.Data, gameapi.CommonInternalCommonV1TrashItem{ResourceType: *req.Params.ResourceType, ResourceId: uuid.New(),
			DeletedAt: time.Now().UTC(), DeletedBy: &by, Title: &title})
	}
	return out, nil
}

func (o ownerGame) RestoreTrashItem(ctx context.Context, req gameapi.RestoreTrashItemRequestObject) (gameapi.RestoreTrashItemResponseObject, error) {
	if err := o.restore(ctx, "game", string(req.ResourceType), req.ResourceId, req.Body.Reason); err != nil {
		return nil, err
	}
	return gameapi.RestoreTrashItem200JSONResponse{ResourceType: req.ResourceType, ResourceId: req.ResourceId,
		RestoredAt: time.Now().UTC()}, nil
}

type ownerCompetition struct{ *ownerTrash }

func (o ownerCompetition) ListTrash(ctx context.Context, req competitionapi.ListTrashRequestObject) (competitionapi.ListTrashResponseObject, error) {
	out := competitionapi.ListTrash200JSONResponse{Data: []competitionapi.CommonInternalCommonV1TrashItem{}}
	if o.list(ctx, "competition", string(*req.Params.ResourceType), req.Params.TenantIds) {
		out.Data = append(out.Data, competitionapi.CommonInternalCommonV1TrashItem{ResourceType: *req.Params.ResourceType, ResourceId: uuid.New(),
			DeletedAt: time.Now().UTC()})
	}
	return out, nil
}

func (o ownerCompetition) RestoreTrashItem(ctx context.Context, req competitionapi.RestoreTrashItemRequestObject) (competitionapi.RestoreTrashItemResponseObject, error) {
	if err := o.restore(ctx, "competition", string(req.ResourceType), req.ResourceId, req.Body.Reason); err != nil {
		return nil, err
	}
	return competitionapi.RestoreTrashItem200JSONResponse{ResourceType: req.ResourceType,
		ResourceId: req.ResourceId, RestoredAt: time.Now().UTC()}, nil
}

type ownerChat struct{ *ownerTrash }

func (o ownerChat) ListTrash(ctx context.Context, req chatapi.ListTrashRequestObject) (chatapi.ListTrashResponseObject, error) {
	out := chatapi.ListTrash200JSONResponse{Data: []chatapi.CommonInternalCommonV1TrashItem{}}
	if o.list(ctx, "chat", string(*req.Params.ResourceType), req.Params.TenantIds) {
		out.Data = append(out.Data, chatapi.CommonInternalCommonV1TrashItem{ResourceType: *req.Params.ResourceType, ResourceId: uuid.New(),
			DeletedAt: time.Now().UTC()})
	}
	return out, nil
}

func (o ownerChat) RestoreTrashItem(ctx context.Context, req chatapi.RestoreTrashItemRequestObject) (chatapi.RestoreTrashItemResponseObject, error) {
	if err := o.restore(ctx, "chat", string(req.ResourceType), req.ResourceId, req.Body.Reason); err != nil {
		return nil, err
	}
	return chatapi.RestoreTrashItem200JSONResponse{ResourceType: req.ResourceType, ResourceId: req.ResourceId,
		RestoredAt: time.Now().UTC()}, nil
}

// F057 / F045-AC-01, AC-20, AC-22, AC-25 through the httpx chain with the real Principal resolution: the trash of every
// game, competition and chat resource type is served by its owner behind the real gateway (TEST DOUBLE owners above) —
// System Admin lists the platform items (no tenant filter) and restores one without X-Tenant-Id (audit
// <resource_type>.restored of identity), tenant_ids and the session tenant of the tenant trash are forwarded unchanged, a
// tenant principal finds no platform item (404), a role without the trash permission is 403.
func TestPlatformAdminTrashOwnerModules(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	o := &ownerTrash{tenants: map[string]int{}}
	p := platformHTTP(ctx, t, e, func(g port.OwnerServiceGateway) port.OwnerServiceGateway {
		gw := g.(modules.Gateway)
		gw.GameTrash, gw.CompetitionTrash, gw.ChatTrash = ownerGame{o}, ownerCompetition{o}, ownerChat{o}
		return gw
	})
	a := seedTenant(ctx, t, e.platform, "pgd-to-a")
	platformUsers(ctx, t, p, a, "to")
	sa, ca, ad, adNoTrash := p.token("sa.to"), p.token("ca.to"), p.token("ad.donvi.to"), p.token("ad.truong.to")
	inA := map[string]string{"X-Tenant-Id": a.tenant}
	types := map[string]string{"challenge": "game", "quest": "game", "minigame_config": "game", "item": "game", "shop_item": "game",
		"event_template": "game", "tournament_template": "competition", "challenge_template": "competition", "sticker_pack": "chat"}
	for rt, m := range types {
		r := p.do(t, "GET", "/admin/trash?resource_type="+rt, sa, nil, nil)
		if r.status != 200 || len(r.body["data"].([]any)) != 1 {
			t.Fatalf("%s: %d %s", rt, r.status, r.raw)
		}
		row := r.body["data"].([]any)[0].(map[string]any)
		if row["resource_type"] != rt || row["tenant_id"] != nil || row["restorable"] != true {
			t.Fatalf("%s row: %v", rt, row)
		}
		if n, ok := o.tenants[m+":"+rt]; !ok || n != 0 {
			t.Fatalf("%s: tenant_ids forwarded %d (%v)", rt, n, ok)
		}
		if r = p.do(t, "GET", "/admin/trash?resource_type="+rt+"&tenant_ids="+a.tenant, sa, nil, nil); r.status != 200 || len(r.body["data"].([]any)) != 0 ||
			o.tenants[m+":"+rt] != 1 {
			t.Fatalf("%s tenant filter: %d %s", rt, r.status, r.raw)
		}
		if r = p.do(t, "GET", "/tenant/trash?resource_type="+rt, ad, inA, nil); r.status != 200 || len(r.body["data"].([]any)) != 0 || o.tenants[m+":"+rt] != 1 {
			t.Fatalf("%s tenant trash: %d %s", rt, r.status, r.raw)
		}
	}
	if r := p.do(t, "GET", "/admin/trash?resource_type=quest", ca, nil, nil); r.status != 403 {
		t.Fatalf("content_admin: %d", r.status)
	}
	if r := p.do(t, "GET", "/tenant/trash?resource_type=sticker_pack", adNoTrash, inA, nil); r.status != 403 {
		t.Fatalf("school_admin: %d", r.status)
	}
	restore := func(path, tok string, hdr map[string]string, reason string) httpResp {
		h := map[string]string{"Idempotency-Key": newKey()}
		for k, v := range hdr {
			h[k] = v
		}
		return p.do(t, "POST", path, tok, h, map[string]any{"reason": reason})
	}
	for _, rt := range []string{"shop_item", "challenge_template", "sticker_pack"} {
		id := uuid.New()
		if r := restore("/admin/trash/"+rt+"/"+id.String()+":restore", sa, nil, "khôi phục dữ liệu platform"); r.status != 204 {
			t.Fatalf("restore %s: %d %s", rt, r.status, r.raw)
		}
		if got := o.restored[len(o.restored)-1]; got != types[rt]+":"+rt+":"+id.String()+":khôi phục dữ liệu platform" {
			t.Fatalf("owner call %q", got)
		}
		if outboxCount(ctx, t, p, "audit.recorded", rt+".restored") != 1 {
			t.Fatalf("audit %s.restored", rt)
		}
	}
	if r := restore("/tenant/trash/sticker_pack/"+uuid.NewString()+":restore", ad, inA, "khôi phục trong tenant"); r.status != 404 {
		t.Fatalf("tenant restore of platform data: %d %s", r.status, r.raw)
	}
	if r := restore("/admin/trash/quest/"+uuid.NewString()+":restore", ca, nil, "không có quyền"); r.status != 403 {
		t.Fatalf("content_admin restore: %d", r.status)
	}
}
