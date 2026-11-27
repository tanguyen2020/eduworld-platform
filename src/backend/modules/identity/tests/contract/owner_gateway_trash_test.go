package contract

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	competitionapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// fakeOwnerTrash is a TEST DOUBLE (R17) of listTrash/restoreTrashItem of the game, competition and chat facades: it
// records the call context and the parameters and answers TrashPage/RestoreResult of their internal specs (checked with
// contract.LoadFacade). The real facades are tested by each module (tests/contract/trash_facade_test.go,
// tests/integration/trash_test.go).
type fakeOwnerTrash struct {
	fakeLearning
	params  []string // "<module>:<resource_type>:<cursor>:<limit>:<tenants>"
	restore []string // "<module>:<resource_type>:<id>:<reason>"
	no      bool
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func ptrLen(p *[]uuid.UUID) int {
	if p == nil {
		return 0
	}
	return len(*p)
}

func (f *fakeOwnerTrash) note(module, rt string, cursor *string, limit *int, tenants *[]uuid.UUID) {
	f.params = append(f.params, fmt.Sprintf("%s:%s:%s:%d:%d", module, rt, ptrStr(cursor), ptrInt(limit), ptrLen(tenants)))
}

type fakeGameTrash struct{ *fakeOwnerTrash }

func (f fakeGameTrash) ListTrash(ctx context.Context, req gameapi.ListTrashRequestObject) (gameapi.ListTrashResponseObject, error) {
	if err := f.record(ctx, "listTrash"); err != nil {
		return nil, err
	}
	f.note("game", string(*req.Params.ResourceType), req.Params.Cursor, req.Params.Limit, req.Params.TenantIds)
	next, title, reason := "g2", "Nhiệm vụ phiên chợ", "nhầm"
	return gameapi.ListTrash200JSONResponse{Data: []gameapi.CommonInternalCommonV1TrashItem{{ResourceType: *req.Params.ResourceType,
		ResourceId: uuid.New(), DeletedAt: time.Now().UTC(), Title: &title, DeleteReason: &reason, Restorable: &f.no}},
		Page: gameapi.CommonInternalCommonV1PageInfo{HasMore: true, NextCursor: &next}}, nil
}

func (f fakeGameTrash) RestoreTrashItem(ctx context.Context, req gameapi.RestoreTrashItemRequestObject) (gameapi.RestoreTrashItemResponseObject, error) {
	if err := f.record(ctx, "restoreTrashItem"); err != nil {
		return nil, err
	}
	f.restore = append(f.restore, "game:"+string(req.ResourceType)+":"+req.ResourceId.String()+":"+req.Body.Reason)
	return gameapi.RestoreTrashItem200JSONResponse{ResourceType: req.ResourceType, ResourceId: req.ResourceId,
		RestoredAt: time.Now().UTC()}, nil
}

type fakeCompetitionTrash struct{ *fakeOwnerTrash }

func (f fakeCompetitionTrash) ListTrash(ctx context.Context, req competitionapi.ListTrashRequestObject) (competitionapi.ListTrashResponseObject, error) {
	if err := f.record(ctx, "listTrash"); err != nil {
		return nil, err
	}
	f.note("competition", string(*req.Params.ResourceType), req.Params.Cursor, req.Params.Limit, req.Params.TenantIds)
	return competitionapi.ListTrash200JSONResponse{Data: []competitionapi.CommonInternalCommonV1TrashItem{{ResourceType: *req.Params.ResourceType,
		ResourceId: uuid.New(), DeletedAt: time.Now().UTC()}}}, nil
}

func (f fakeCompetitionTrash) RestoreTrashItem(ctx context.Context, req competitionapi.RestoreTrashItemRequestObject) (competitionapi.RestoreTrashItemResponseObject, error) {
	if err := f.record(ctx, "restoreTrashItem"); err != nil {
		return nil, err
	}
	f.restore = append(f.restore, "competition:"+string(req.ResourceType)+":"+req.ResourceId.String()+":"+req.Body.Reason)
	return competitionapi.RestoreTrashItem200JSONResponse{ResourceType: req.ResourceType,
		ResourceId: req.ResourceId, RestoredAt: time.Now().UTC()}, nil
}

type fakeChatTrash struct{ *fakeOwnerTrash }

func (f fakeChatTrash) ListTrash(ctx context.Context, req chatapi.ListTrashRequestObject) (chatapi.ListTrashResponseObject, error) {
	if err := f.record(ctx, "listTrash"); err != nil {
		return nil, err
	}
	f.note("chat", string(*req.Params.ResourceType), req.Params.Cursor, req.Params.Limit, req.Params.TenantIds)
	return chatapi.ListTrash200JSONResponse{Data: []chatapi.CommonInternalCommonV1TrashItem{}}, nil
}

func (f fakeChatTrash) RestoreTrashItem(ctx context.Context, req chatapi.RestoreTrashItemRequestObject) (chatapi.RestoreTrashItemResponseObject, error) {
	if err := f.record(ctx, "restoreTrashItem"); err != nil {
		return nil, err
	}
	f.restore = append(f.restore, "chat:"+string(req.ResourceType)+":"+req.ResourceId.String()+":"+req.Body.Reason)
	return chatapi.RestoreTrashItem200JSONResponse{ResourceType: req.ResourceType, ResourceId: req.ResourceId,
		RestoredAt: time.Now().UTC()}, nil
}

// F057 / F045-AC-24 (caller side): the trash of every game, competition and chat resource type of the routing table is
// served by its owner through pkg/modcall — caller identity in x-allowed-callers, the original Principal, the
// Idempotency-Key of a restore, the deadline of its kind — with resource_type, cursor, limit and tenant_ids passed through
// unchanged (one owner per listing: the public API requires resource_type, each owner pages with its own keyset cursor).
// The fake answers validate against the owners' internal specs; an owner not wired is 503 UNAVAILABLE, a 4xx passes.
func TestOwnerGatewayTrashFanout(t *testing.T) {
	byModule := map[string][]string{}
	for _, rt := range []string{"challenge", "quest", "minigame_config", "item", "shop_item", "event_template", "tournament_template",
		"challenge_template", "sticker_pack"} {
		m, err := platformadmin.Owner(rt)
		if err != nil || !platformadmin.RestorableThroughTrash(rt) {
			t.Fatalf("%s: %q %v", rt, m, err)
		}
		byModule[m] = append(byModule[m], rt)
	}
	if len(byModule["game"]) != 6 || len(byModule["competition"]) != 2 || len(byModule["chat"]) != 1 {
		t.Fatalf("routing: %v", byModule)
	}
	for _, ac := range []map[string][]string{gameapi.AllowedCallers, competitionapi.AllowedCallers, chatapi.AllowedCallers} {
		for _, op := range []string{"listTrash", "restoreTrashItem"} {
			if !slices.Equal(ac[op], []string{"identity"}) {
				t.Fatalf("%s: x-allowed-callers %v", op, ac[op])
			}
		}
	}
	fake := &fakeOwnerTrash{}
	g := modules.Gateway{GameTrash: fakeGameTrash{fake}, CompetitionTrash: fakeCompetitionTrash{fake}, ChatTrash: fakeChatTrash{fake}}
	sa := authztest.Platform("system_admin")
	key := uuid.NewString()
	ctx := modcall.WithIdempotencyKey(authztest.Ctx(context.Background(), sa), key)
	tenant := uuid.New()
	for m, types := range byModule {
		if m == "learning" || m == "identity" {
			continue
		}
		for _, rt := range types {
			p, err := g.ListTrash(ctx, m, port.TrashQuery{ResourceType: rt, Cursor: "c1", Limit: 7, TenantIDs: []uuid.UUID{tenant}})
			if err != nil {
				t.Fatalf("%s/%s list: %v", m, rt, err)
			}
			if m == "game" && (len(p.Data) != 1 || p.Data[0].Restorable || p.Data[0].Title == "" || p.Data[0].DeleteReason != "nhầm" || !p.HasMore ||
				p.NextCursor == nil || *p.NextCursor != "g2") {
				t.Fatalf("game page: %+v", p)
			}
			if m == "competition" && (len(p.Data) != 1 || !p.Data[0].Restorable || p.Data[0].TenantID != nil) {
				t.Fatalf("competition page (restorable defaults to true): %+v", p)
			}
			id := uuid.New()
			if err := g.RestoreTrashItem(ctx, m, rt, id, "khôi phục "+rt); err != nil {
				t.Fatalf("%s/%s restore: %v", m, rt, err)
			}
			want := m + ":" + rt + ":" + id.String() + ":khôi phục " + rt
			if fake.restore[len(fake.restore)-1] != want {
				t.Fatalf("restore call %q, want %q", fake.restore[len(fake.restore)-1], want)
			}
			if got := fake.params[len(fake.params)-1]; got != m+":"+rt+":c1:7:1" {
				t.Fatalf("list params %q", got)
			}
		}
	}
	for _, c := range fake.seen {
		if c.caller != "identity" || c.principal.UserID != sa.UserID || c.key != key {
			t.Fatalf("%s: call context %+v", c.op, c)
		}
		limit := modcall.DefaultDeadlines[modcall.Write]
		if c.op == "listTrash" {
			limit = modcall.DefaultDeadlines[modcall.Read]
		}
		if c.deadline <= 0 || c.deadline > limit {
			t.Fatalf("%s: deadline %v", c.op, c.deadline)
		}
	}
	// The answers of the fakes are valid answers of the owners' contracts.
	checks := map[string]func() (any, error){
		"game": func() (any, error) {
			return fakeGameTrash{fake}.ListTrash(ctx, gameapi.ListTrashRequestObject{Params: gameapi.ListTrashParams{ResourceType: ptrRT[gameapi.CommonInternalCommonV1ResourceType]("quest")}})
		},
		"competition": func() (any, error) {
			return fakeCompetitionTrash{fake}.ListTrash(ctx, competitionapi.ListTrashRequestObject{Params: competitionapi.ListTrashParams{
				ResourceType: ptrRT[competitionapi.CommonInternalCommonV1ResourceType]("tournament_template")}})
		},
		"chat": func() (any, error) {
			return fakeChatTrash{fake}.ListTrash(ctx, chatapi.ListTrashRequestObject{Params: chatapi.ListTrashParams{ResourceType: ptrRT[chatapi.CommonInternalCommonV1ResourceType]("sticker_pack")}})
		},
	}
	for m, call := range checks {
		res, err := call()
		if err != nil {
			t.Fatal(err)
		}
		spec, err := contract.LoadFacade(m)
		if err != nil {
			t.Fatal(err)
		}
		if err := spec.CheckResponse("listTrash", 200, res); err != nil {
			t.Fatalf("%s fake answer: %v", m, err)
		}
	}
	// Not wired → 503 UNAVAILABLE (F045-AC-25); a 4xx of the owner passes through.
	for _, m := range []string{"game", "competition", "chat"} {
		if _, err := (modules.Gateway{}).ListTrash(ctx, m, port.TrashQuery{ResourceType: byModule[m][0]}); probCode(err) != problem.Unavailable {
			t.Fatalf("%s list not wired: %v", m, err)
		}
		if err := (modules.Gateway{}).RestoreTrashItem(ctx, m, byModule[m][0], uuid.New(), "khôi phục"); probCode(err) != problem.Unavailable {
			t.Fatalf("%s restore not wired: %v", m, err)
		}
	}
	conflict := &fakeOwnerTrash{fakeLearning: fakeLearning{err: problem.New(problem.Conflict, "")}}
	if err := (modules.Gateway{ChatTrash: fakeChatTrash{conflict}}).RestoreTrashItem(ctx, "chat", "sticker_pack", uuid.New(), "khôi phục"); probCode(err) != problem.Conflict {
		t.Fatalf("409 pass-through: %v", err)
	}
	down := &fakeOwnerTrash{fakeLearning: fakeLearning{err: context.DeadlineExceeded}}
	if _, err := (modules.Gateway{CompetitionTrash: fakeCompetitionTrash{down}}).ListTrash(ctx, "competition", port.TrashQuery{ResourceType: "challenge_template"}); probCode(err) != problem.Unavailable {
		t.Fatalf("owner down: %v", err)
	}
}

func ptrRT[T ~string](s string) *T {
	v := T(s)
	return &v
}
