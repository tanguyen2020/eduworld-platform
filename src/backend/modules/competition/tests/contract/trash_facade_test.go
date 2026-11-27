package contract

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	portread "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
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

func (m *trashReader) DeletedTemplates(_ context.Context, f portread.TrashFilter) ([]portread.TrashRow, error) {
	m.last, m.calls = f, m.calls+1
	var out []portread.TrashRow
	for _, r := range m.rows {
		typeOK := false
		for _, t := range f.ConfigTypes {
			typeOK = typeOK || t == r.ConfigType
		}
		after := f.After == nil || r.DeletedAt.Before(f.After.At) || (r.DeletedAt.Equal(f.After.At) && r.ConfigID.String() < f.After.ID.String())
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

// trashStore is a TEST DOUBLE of the restore store; configType records the config type asked for.
type trashStore struct {
	err        error
	calls      int
	configType string
}

func (s *trashStore) RestoreTemplate(_ context.Context, configType string, _, _ uuid.UUID, _ time.Time) error {
	s.calls, s.configType = s.calls+1, configType
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

// F057 (receiving side of listTrash, ADR-0028, R8/R20): System Admin lists the soft-deleted tournament and challenge
// templates newest first under tenancy.Bypass(trash) (audit cross_tenant.accessed), keyset-paged; the answer matches
// TrashPage of the internal spec; a tenant user and a tenant filter see no platform configuration (nothing is read); a role
// without the trash permission is 403; a type of another module is 422; a bad cursor is 422.
func TestCompetitionTrashFacadeList(t *testing.T) {
	spy := &trashAudit{}
	tenancy.Configure(tenancy.Hooks{Recorder: spy})
	authz.Configure(authz.Hooks{Recorder: spy})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	base := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	by := uuid.New()
	reader := &trashReader{}
	for i := range 3 {
		ct := challenge.ConfigTournamentTemplates
		if i == 1 {
			ct = challenge.ConfigChallengeTemplates
		}
		reader.rows = append(reader.rows, portread.TrashRow{ConfigType: ct, ConfigID: uuid.New(), Title: "Mẫu", DeletedAt: base.Add(-time.Duration(i) * time.Hour),
			DeletedBy: &by, Conflict: i == 2})
	}
	session := &trashSession{}
	f := facade.Facade{TrashList: &query.Trash{Read: session, Reader: reader}}
	spec, err := contract.LoadFacade("competition")
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
	if len(page1.Data) != 2 || !page1.Page.HasMore || page1.Data[0].ResourceType != "tournament_template" || page1.Data[1].ResourceType != "challenge_template" ||
		page1.Data[0].TenantId != nil || !*page1.Data[0].Restorable {
		t.Fatalf("page 1: %+v", page1)
	}
	if !session.bypassed || spy.count(audit.ActionCrossTenantAccessed) != 1 || len(reader.last.ConfigTypes) != 2 || reader.last.Limit != limit+1 {
		t.Fatalf("bypass %v audits %d filter %+v", session.bypassed, spy.count(audit.ActionCrossTenantAccessed), reader.last)
	}
	res, err = f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Limit: &limit, Cursor: page1.Page.NextCursor}})
	if err != nil {
		t.Fatal(err)
	}
	if p2 := res.(api.ListTrash200JSONResponse); len(p2.Data) != 1 || p2.Page.HasMore || *p2.Data[0].Restorable {
		t.Fatalf("page 2: %+v", p2)
	}
	rt := api.CommonInternalCommonV1ResourceType("challenge_template")
	if res, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{ResourceType: &rt}}); err != nil ||
		len(res.(api.ListTrash200JSONResponse).Data) != 1 || reader.last.ConfigTypes[0] != challenge.ConfigChallengeTemplates {
		t.Fatalf("one type: %+v %v", res, err)
	}
	calls := reader.calls
	tenants := []uuid.UUID{authztest.TenantA}
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
		"school_admin":      authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin")),
		"competition_admin": authztest.Ctx(context.Background(), authztest.Platform("competition_admin")),
	} {
		if _, err := f.ListTrash(ctx, api.ListTrashRequestObject{}); trashCode(err) != problem.PermissionDenied {
			t.Fatalf("%s: %v", name, err)
		}
	}
	other := api.CommonInternalCommonV1ResourceType("quest")
	if _, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{ResourceType: &other}}); trashCode(err) != problem.ValidationFailed {
		t.Fatalf("other module type: %v", err)
	}
	bad := "%%%"
	if _, err := f.ListTrash(sa, api.ListTrashRequestObject{Params: api.ListTrashParams{Cursor: &bad}}); trashCode(err) != problem.ValidationFailed {
		t.Fatalf("bad cursor: %v", err)
	}
	if _, err := (facade.Facade{}).ListTrash(sa, api.ListTrashRequestObject{}); trashCode(err) != problem.Unavailable {
		t.Fatalf("not wired: %v", err)
	}
}

// F057 (receiving side of restoreTrashItem, ADR-0028, R10/R21): System Admin restores a template under
// tenancy.Bypass(trash) (mode write) with audit content.restored and the platform event <resource_type>.restored; not in
// the trash → 404, a live configuration with the code → 409; the tenant trash never finds a template (404); reason 3–1000;
// a role without platform.trash.restore → 403; the facade needs an Idempotency-Key and its store.
func TestCompetitionTrashRestore(t *testing.T) {
	spy := &trashAudit{}
	tenancy.Configure(tenancy.Hooks{Recorder: spy})
	authz.Configure(authz.Hooks{Recorder: spy})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	uow, store, events := &trashUoW{}, &trashStore{}, &trashEvents{}
	h := command.RestoreTrash{UoW: uow, Store: store, Events: events, Audit: spy, Now: func() time.Time { return now }}
	ctx := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	id := uuid.New()
	v, err := h.Restore(ctx, command.RestoreTrashInput{ResourceType: "challenge_template", ID: id, Reason: "khôi phục mẫu thử thách"})
	if err != nil || v.ResourceID != id || !uow.bypassed || store.configType != challenge.ConfigChallengeTemplates {
		t.Fatalf("restore: %+v %v %+v", v, err, store)
	}
	if spy.count(command.ActionContentRestored) != 1 || spy.count(audit.ActionCrossTenantAccessed) != 1 || len(events.events) != 1 ||
		events.events[0].Type != "challenge_template.restored" || !events.events[0].Platform {
		t.Fatalf("audit/event: %+v %+v", spy.events, events.events)
	}
	if err := contract.CheckPayload("competition", "resource.restored", 1, events.events[0].Payload); err != nil {
		t.Fatalf("payload schema: %v", err)
	}
	for name, c := range map[string]struct {
		err  error
		want problem.Code
	}{"not in trash": {challenge.ErrNotInTrash, problem.NotFound}, "code taken": {challenge.ErrRestoreConflict, problem.Conflict}} {
		store.err = c.err
		if _, err := h.Restore(ctx, command.RestoreTrashInput{ResourceType: "tournament_template", ID: id, Reason: "khôi phục"}); trashCode(err) != c.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	store.err, store.calls = nil, 0
	ta := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin"))
	if _, err := h.Restore(ta, command.RestoreTrashInput{ResourceType: "tournament_template", ID: id, Reason: "khôi phục"}); trashCode(err) != problem.NotFound {
		t.Fatalf("tenant restore: %v", err)
	}
	for name, c := range map[string]context.Context{
		"school_admin":      authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin")),
		"competition_admin": authztest.Ctx(context.Background(), authztest.Platform("competition_admin")),
	} {
		if _, err := h.Restore(c, command.RestoreTrashInput{ResourceType: "tournament_template", ID: id, Reason: "khôi phục"}); trashCode(err) != problem.PermissionDenied {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for name, in := range map[string]command.RestoreTrashInput{
		"short reason":      {ResourceType: "tournament_template", ID: id, Reason: "ab"},
		"other module type": {ResourceType: "quest", ID: id, Reason: "khôi phục"},
	} {
		if _, err := h.Restore(ctx, in); trashCode(err) != problem.ValidationFailed {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if store.calls != 0 {
		t.Fatal("a refused restore must not touch the store")
	}
	f := facade.Facade{TrashRestore: &h}
	req := api.RestoreTrashItemRequestObject{ResourceType: "tournament_template", ResourceId: id, Body: &api.RestoreTrashItemJSONRequestBody{Reason: "khôi phục"}}
	if _, err := f.RestoreTrashItem(ctx, req); trashCode(err) != problem.IdempotencyKeyRequired {
		t.Fatalf("no key: %v", err)
	}
	if _, err := (facade.Facade{}).RestoreTrashItem(ctx, req); trashCode(err) != problem.Unavailable {
		t.Fatalf("not wired: %v", err)
	}
}
