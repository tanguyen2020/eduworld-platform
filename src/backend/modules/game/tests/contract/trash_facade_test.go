package contract

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	portread "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// trashAudit is a TEST DOUBLE (R17) of the audit recorder of the tenancy/authz hooks and of write.Auditor.
type trashAudit struct {
	mu     sync.Mutex
	events []audit.Event
}

func (a *trashAudit) Record(_ context.Context, e audit.Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, e)
	return nil
}

func (a *trashAudit) count(action string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, e := range a.events {
		if e.Action == action {
			n++
		}
	}
	return n
}

// trashSession is a TEST DOUBLE of the read session: it records whether ctx carried tenancy.Bypass(trash).
type trashSession struct{ bypassed bool }

func (s *trashSession) Read(ctx context.Context, fn func(ctx context.Context) error) error {
	r, ok := tenancy.Bypassed(ctx)
	s.bypassed = ok && r == tenancy.ReasonTrash
	return fn(ctx)
}

// trashModel is a TEST DOUBLE of the trash read model: rows newest first, keyset and limit applied like the SQL.
type trashModel struct {
	rows []portread.TrashRow
	last portread.TrashFilter
}

func (m *trashModel) Deleted(_ context.Context, f portread.TrashFilter) ([]portread.TrashRow, error) {
	m.last = f
	var out []portread.TrashRow
	for _, r := range m.rows {
		typeOK := false
		for _, t := range f.ItemTypes {
			typeOK = typeOK || t == r.ItemType
		}
		after := f.AfterAt == nil || r.DeletedAt.Before(*f.AfterAt) || (r.DeletedAt.Equal(*f.AfterAt) && r.ConfigID.String() < f.AfterID.String())
		if typeOK && after && len(out) < f.Limit {
			out = append(out, r)
		}
	}
	return out, nil
}

// trashUoW runs fn in place (TEST DOUBLE of the UnitOfWork) and records whether ctx carried tenancy.Bypass(trash).
type trashUoW struct{ bypassed bool }

func (u *trashUoW) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	r, ok := tenancy.Bypassed(ctx)
	u.bypassed = ok && r == tenancy.ReasonTrash
	return fn(ctx)
}

func (u *trashUoW) InTenantTx(ctx context.Context, _ tenancy.Scope, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// trashStore is a TEST DOUBLE of the restore store.
type trashStore struct {
	err   error
	calls int
}

func (s *trashStore) RestoreConfig(context.Context, string, uuid.UUID, uuid.UUID, time.Time) error {
	s.calls++
	return s.err
}

type trashEvents struct{ events []portwrite.Event }

func (p *trashEvents) Publish(_ context.Context, e portwrite.Event) error {
	p.events = append(p.events, e)
	return nil
}

func trashCodec(t *testing.T) *page.Codec {
	t.Helper()
	c, err := page.NewCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F057 (receiving side of listTrash, ADR-0028, R8/R20): System Admin lists the soft-deleted console items of the six game
// resource types newest first under tenancy.Bypass(trash) (audit cross_tenant.accessed), keyset-paged; the answer matches
// TrashPage of the internal spec; a tenant user (tenant_admin) and a tenant filter see no platform content; a role without
// the trash permission is 403; a type of another module is 422.
func TestGameTrashFacadeList(t *testing.T) {
	spy := &trashAudit{}
	tenancy.Configure(tenancy.Hooks{Recorder: spy})
	authz.Configure(authz.Hooks{Recorder: spy})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	base := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	by := uuid.New()
	model := &trashModel{}
	for i, rt := range []string{"quest", "minigame_config", "item", "shop_item", "event_template", "challenge"} {
		model.rows = append(model.rows, portread.TrashRow{ItemType: rt, ConfigID: uuid.New(), Title: "Mục " + rt,
			DeletedAt: base.Add(-time.Duration(i) * time.Hour), DeletedBy: &by, DeleteReason: "nhầm", Conflict: rt == "shop_item"})
	}
	session := &trashSession{}
	f := facade.Facade{Trash: facade.Trash{TrashList: query.Trash{Read: session, Model: model, Cursors: trashCodec(t)}}}
	spec, err := contract.LoadFacade("game")
	if err != nil {
		t.Fatal(err)
	}
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	limit := 4
	res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	page1 := res.(api.ListTrash200JSONResponse)
	if err := spec.CheckResponse("listTrash", 200, page1); err != nil {
		t.Fatalf("contract: %v", err)
	}
	if len(page1.Data) != 4 || !page1.Page.HasMore || page1.Page.NextCursor == nil || page1.Data[0].ResourceType != "quest" ||
		page1.Data[0].TenantId != nil || *page1.Data[3].Restorable || page1.Data[0].Title == nil || *page1.Data[0].DeleteReason != "nhầm" {
		t.Fatalf("page 1: %+v", page1)
	}
	if !session.bypassed || spy.count(audit.ActionCrossTenantAccessed) != 1 || len(model.last.ItemTypes) != len(gameconfig.TrashTypes()) ||
		model.last.Limit != limit+1 {
		t.Fatalf("bypass %v audits %d filter %+v", session.bypassed, spy.count(audit.ActionCrossTenantAccessed), model.last)
	}
	res, err = f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit, Cursor: page1.Page.NextCursor}})
	if err != nil {
		t.Fatal(err)
	}
	page2 := res.(api.ListTrash200JSONResponse)
	if len(page2.Data) != 2 || page2.Page.HasMore || page2.Data[0].ResourceId != model.rows[4].ConfigID {
		t.Fatalf("page 2: %+v", page2)
	}
	rt := api.CommonInternalCommonV1ResourceType("shop_item")
	if res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{ResourceType: &rt}}); err != nil ||
		len(res.(api.ListTrash200JSONResponse).Data) != 1 || len(model.last.ItemTypes) != 1 {
		t.Fatalf("one type: %+v %v", res, err)
	}
	// R8: a tenant filter or the tenant trash never lists platform content (and reads nothing).
	calls := model.last
	tenants := []uuid.UUID{authztest.TenantA}
	if res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{TenantIds: &tenants}}); err != nil ||
		len(res.(api.ListTrash200JSONResponse).Data) != 0 {
		t.Fatalf("tenant filter: %+v %v", res, err)
	}
	ta := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin"))
	if res, err := f.ListTrash(ta, api.ListTrashRequestObject{}); err != nil || len(res.(api.ListTrash200JSONResponse).Data) != 0 {
		t.Fatalf("tenant trash: %+v %v", res, err)
	}
	if model.last.Limit != calls.Limit || len(model.last.ItemTypes) != len(calls.ItemTypes) {
		t.Fatal("a tenant listing must not read the platform rows")
	}
	// Authorization: no trash permission → 403 (tenant and platform roles); a type of another module → 422; bad cursor → 400.
	for name, ctx := range map[string]context.Context{
		"school_admin":  authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin")),
		"content_admin": authztest.Ctx(context.Background(), authztest.Platform("content_admin")),
	} {
		if _, err := f.ListTrash(ctx, api.ListTrashRequestObject{}); codeOf(err) != problem.PermissionDenied {
			t.Fatalf("%s: %v", name, err)
		}
	}
	other := api.CommonInternalCommonV1ResourceType("question")
	if _, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{ResourceType: &other}}); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("other module type: %v", err)
	}
	bad := "not-a-cursor"
	if _, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Cursor: &bad}}); codeOf(err) != problem.BadRequest {
		t.Fatalf("bad cursor: %v", err)
	}
	if _, err := f.ListTrash(context.Background(), api.ListTrashRequestObject{}); codeOf(err) != problem.Unauthenticated {
		t.Fatalf("no principal: %v", err)
	}
	if _, err := (facade.Facade{}).ListTrash(sa, api.ListTrashRequestObject{}); codeOf(err) != problem.Unavailable {
		t.Fatalf("not wired: %v", err)
	}
}

// F057 (receiving side of restoreTrashItem, ADR-0028, R10/R21): System Admin restores under tenancy.Bypass(trash) (mode
// write), with audit content.restored and the platform event <resource_type>.restored (payload schema of module game);
// not in the trash → 404, a live item with the key → 409; the tenant trash never finds platform content (404); reason
// 3–1000; a role without platform.trash.restore → 403; the facade needs an Idempotency-Key and its store (503 otherwise).
func TestGameTrashRestore(t *testing.T) {
	spy := &trashAudit{}
	tenancy.Configure(tenancy.Hooks{Recorder: spy})
	authz.Configure(authz.Hooks{Recorder: spy})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	uow, store, events := &trashUoW{}, &trashStore{}, &trashEvents{}
	h := command.RestoreTrash{UoW: uow, Store: store, Events: events, Audit: spy, Now: func() time.Time { return now }}
	sa := authztest.Platform("system_admin")
	ctx := authztest.Ctx(context.Background(), sa)
	id := uuid.New()
	v, err := h.Restore(ctx, command.RestoreTrashInput{ResourceType: "event_template", ID: id, Reason: "khôi phục mẫu sự kiện"})
	if err != nil || v.ResourceID != id || !v.RestoredAt.Equal(now) || !uow.bypassed || store.calls != 1 {
		t.Fatalf("restore: %+v %v bypass=%v", v, err, uow.bypassed)
	}
	if spy.count(command.ActionContentRestored) != 1 || spy.count(audit.ActionCrossTenantAccessed) != 1 || len(events.events) != 1 {
		t.Fatalf("audit/event: %+v %+v", spy.events, events.events)
	}
	ev := events.events[0]
	if ev.Type != "event_template.restored" || !ev.Platform || ev.TenantID != nil {
		t.Fatalf("event: %+v", ev)
	}
	if err := contract.CheckPayload("game", "resource.restored", 1, ev.Payload); err != nil {
		t.Fatalf("payload schema: %v", err)
	}
	for name, c := range map[string]struct {
		err  error
		want problem.Code
	}{"not in trash": {gameconfig.ErrNotInTrash, problem.NotFound}, "key taken": {gameconfig.ErrRestoreConflict, problem.Conflict}} {
		store.err = c.err
		if _, err := h.Restore(ctx, command.RestoreTrashInput{ResourceType: "quest", ID: id, Reason: "khôi phục"}); codeOf(err) != c.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	store.err, store.calls = nil, 0
	ta := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin"))
	if _, err := h.Restore(ta, command.RestoreTrashInput{ResourceType: "quest", ID: id, Reason: "khôi phục"}); codeOf(err) != problem.NotFound {
		t.Fatalf("tenant restore: %v", err)
	}
	for name, ctx := range map[string]context.Context{
		"school_admin":  authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin")),
		"content_admin": authztest.Ctx(context.Background(), authztest.Platform("content_admin")),
	} {
		if _, err := h.Restore(ctx, command.RestoreTrashInput{ResourceType: "quest", ID: id, Reason: "khôi phục"}); codeOf(err) != problem.PermissionDenied {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for name, in := range map[string]command.RestoreTrashInput{
		"short reason":         {ResourceType: "quest", ID: id, Reason: "  a "},
		"other module type":    {ResourceType: "sticker_pack", ID: id, Reason: "khôi phục"},
		"boss (no trash type)": {ResourceType: "boss", ID: id, Reason: "khôi phục"},
		"reason over 1000 c":   {ResourceType: "quest", ID: id, Reason: string(make([]rune, 1001))},
	} {
		if _, err := h.Restore(ctx, in); codeOf(err) != problem.ValidationFailed {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if store.calls != 0 {
		t.Fatal("a refused restore must not touch the store")
	}
	f := facade.Facade{Trash: facade.Trash{TrashRestore: h}}
	req := api.RestoreTrashItemRequestObject{ResourceType: "quest", ResourceId: id, Body: &api.RestoreTrashItemJSONRequestBody{Reason: "khôi phục"}}
	if _, err := f.RestoreTrashItem(ctx, req); codeOf(err) != problem.Unavailable {
		t.Fatalf("no idempotency store: %v", err)
	}
}
