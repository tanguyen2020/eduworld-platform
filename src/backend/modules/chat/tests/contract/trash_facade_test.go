package contract

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	portread "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/facade"
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

// trashReader is a TEST DOUBLE of the trash reader: rows newest first, keyset and limit applied like the SQL.
type trashReader struct {
	rows  []portread.TrashRow
	last  portread.TrashFilter
	calls int
}

func (m *trashReader) DeletedPacks(_ context.Context, f portread.TrashFilter) ([]portread.TrashRow, error) {
	m.last, m.calls = f, m.calls+1
	var out []portread.TrashRow
	for _, r := range m.rows {
		after := f.AfterAt == nil || r.DeletedAt.Before(*f.AfterAt) || (r.DeletedAt.Equal(*f.AfterAt) && r.PackID.String() < f.AfterID.String())
		if after && len(out) < f.Limit {
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

func (s *trashStore) RestorePack(context.Context, uuid.UUID, time.Time) error {
	s.calls++
	return s.err
}

type trashEvents struct{ events []portwrite.Event }

func (p *trashEvents) Publish(_ context.Context, e portwrite.Event) error {
	p.events = append(p.events, e)
	return nil
}

func trashCode(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F057 (receiving side of listTrash, ADR-0028, R8/R20): System Admin lists the soft-deleted sticker packs newest first
// under tenancy.Bypass(trash) (audit cross_tenant.accessed), keyset-paged; the answer matches TrashPage of the internal
// spec; a tenant user and a tenant filter see no platform library (nothing is read); a role without the trash permission
// is 403 (safety_moderator included: R20 gives it moderation reads only); a type of another module is 422.
func TestChatTrashFacadeList(t *testing.T) {
	spy := &trashAudit{}
	tenancy.Configure(tenancy.Hooks{Recorder: spy})
	authz.Configure(authz.Hooks{Recorder: spy})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	base := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	by := uuid.New()
	reader := &trashReader{}
	for i := range 3 {
		reader.rows = append(reader.rows, portread.TrashRow{PackID: uuid.New(), Title: "Gói Mùa hè", DeletedAt: base.Add(-time.Duration(i) * time.Minute),
			DeletedBy: &by, Conflict: i == 1})
	}
	codec, err := page.NewCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	session := &trashSession{}
	f := facade.Facade{Trash: facade.Trash{TrashList: query.Trash{Read: session, Reader: reader, Cursors: codec}}}
	spec, err := contract.LoadFacade("chat")
	if err != nil {
		t.Fatal(err)
	}
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	limit := 2
	res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	page1 := res.(api.ListTrash200JSONResponse)
	if err := spec.CheckResponse("listTrash", 200, page1); err != nil {
		t.Fatalf("contract: %v", err)
	}
	if len(page1.Data) != 2 || !page1.Page.HasMore || page1.Data[0].ResourceType != chatasset.ResourceStickerPack || page1.Data[0].TenantId != nil ||
		!*page1.Data[0].Restorable || *page1.Data[1].Restorable || *page1.Data[0].Title != "Gói Mùa hè" {
		t.Fatalf("page 1: %+v", page1)
	}
	if !session.bypassed || spy.count(audit.ActionCrossTenantAccessed) != 1 || reader.last.Limit != limit+1 {
		t.Fatalf("bypass %v audits %d filter %+v", session.bypassed, spy.count(audit.ActionCrossTenantAccessed), reader.last)
	}
	res, err = f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit, Cursor: page1.Page.NextCursor}})
	if err != nil {
		t.Fatal(err)
	}
	if p2 := res.(api.ListTrash200JSONResponse); len(p2.Data) != 1 || p2.Page.HasMore || p2.Data[0].ResourceId != reader.rows[2].PackID {
		t.Fatalf("page 2: %+v", p2)
	}
	calls := reader.calls
	tenants := []uuid.UUID{authztest.TenantB}
	if res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{TenantIds: &tenants}}); err != nil ||
		len(res.(api.ListTrash200JSONResponse).Data) != 0 {
		t.Fatalf("tenant filter: %+v %v", res, err)
	}
	ta := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin"))
	if res, err := f.ListTrash(ta, api.ListTrashRequestObject{}); err != nil || len(res.(api.ListTrash200JSONResponse).Data) != 0 {
		t.Fatalf("tenant trash: %+v %v", res, err)
	}
	if reader.calls != calls {
		t.Fatal("a tenant listing must not read the platform rows")
	}
	for name, ctx := range map[string]context.Context{
		"school_admin":     authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin")),
		"safety_moderator": authztest.Ctx(context.Background(), authztest.Platform("safety_moderator")),
	} {
		if _, err := f.ListTrash(ctx, api.ListTrashRequestObject{}); trashCode(err) != problem.PermissionDenied {
			t.Fatalf("%s: %v", name, err)
		}
	}
	other := api.CommonInternalCommonV1ResourceType("quest")
	if _, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{ResourceType: &other}}); trashCode(err) != problem.ValidationFailed {
		t.Fatalf("other module type: %v", err)
	}
	if _, err := (facade.Facade{}).ListTrash(sa, api.ListTrashRequestObject{}); trashCode(err) != problem.Unavailable {
		t.Fatalf("not wired: %v", err)
	}
}

// F057 (receiving side of restoreTrashItem, ADR-0028, R10/R21): System Admin restores a sticker pack under
// tenancy.Bypass(trash) (mode write) with audit content.restored and the platform event sticker_pack.restored; not in the
// trash → 404, a live pack with the code → 409; the tenant trash never finds a pack (404); reason 3–1000; a role without
// platform.trash.restore → 403; the facade needs its idempotency store (503 otherwise).
func TestChatTrashRestore(t *testing.T) {
	spy := &trashAudit{}
	tenancy.Configure(tenancy.Hooks{Recorder: spy})
	authz.Configure(authz.Hooks{Recorder: spy})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	uow, store, events := &trashUoW{}, &trashStore{}, &trashEvents{}
	h := command.RestoreTrash{UoW: uow, Store: store, Events: events, Audit: spy}
	ctx := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	id := uuid.New()
	v, err := h.Restore(ctx, command.RestoreTrashInput{ResourceType: "sticker_pack", ID: id, Reason: "gói bị xóa nhầm"})
	if err != nil || v.ResourceID != id || !uow.bypassed || store.calls != 1 {
		t.Fatalf("restore: %+v %v", v, err)
	}
	if spy.count(command.AuditContentRestored) != 1 || spy.count(audit.ActionCrossTenantAccessed) != 1 || len(events.events) != 1 ||
		events.events[0].Type != "sticker_pack.restored" || !events.events[0].Platform {
		t.Fatalf("audit/event: %+v %+v", spy.events, events.events)
	}
	if err := contract.CheckPayload("chat", "resource.restored", 1, events.events[0].Payload); err != nil {
		t.Fatalf("payload schema: %v", err)
	}
	for name, c := range map[string]struct {
		err  error
		want problem.Code
	}{"not in trash": {chatasset.ErrNotInTrash, problem.NotFound}, "code taken": {chatasset.ErrRestoreConflict, problem.Conflict}} {
		store.err = c.err
		if _, err := h.Restore(ctx, command.RestoreTrashInput{ResourceType: "sticker_pack", ID: id, Reason: "khôi phục"}); trashCode(err) != c.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	store.err, store.calls = nil, 0
	ta := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin"))
	if _, err := h.Restore(ta, command.RestoreTrashInput{ResourceType: "sticker_pack", ID: id, Reason: "khôi phục"}); trashCode(err) != problem.NotFound {
		t.Fatalf("tenant restore: %v", err)
	}
	for name, c := range map[string]context.Context{
		"school_admin":     authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin")),
		"safety_moderator": authztest.Ctx(context.Background(), authztest.Platform("safety_moderator")),
	} {
		if _, err := h.Restore(c, command.RestoreTrashInput{ResourceType: "sticker_pack", ID: id, Reason: "khôi phục"}); trashCode(err) != problem.PermissionDenied {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for name, in := range map[string]command.RestoreTrashInput{
		"short reason": {ResourceType: "sticker_pack", ID: id, Reason: "x"},
		"gif":          {ResourceType: "gif", ID: id, Reason: "khôi phục"},
	} {
		if _, err := h.Restore(ctx, in); trashCode(err) != problem.ValidationFailed {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if store.calls != 0 {
		t.Fatal("a refused restore must not touch the store")
	}
	req := api.RestoreTrashItemRequestObject{ResourceType: "sticker_pack", ResourceId: id, Body: &api.RestoreTrashItemJSONRequestBody{Reason: "khôi phục"}}
	if _, err := (facade.Facade{Trash: facade.Trash{TrashRestore: h}}).RestoreTrashItem(ctx, req); trashCode(err) != problem.Unavailable {
		t.Fatalf("no idempotency store: %v", err)
	}
}
