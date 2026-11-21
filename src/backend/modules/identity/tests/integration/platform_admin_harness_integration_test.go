//go:build integration

package integration

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/facade"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// platformEnv is the F045 harness: the identity routes behind httpx with the real Principal resolution, the
// F045 use cases on the real repositories, outbox and saga engine, and the learning facade replaced by the
// in-memory TEST DOUBLE fakeLearningFacade (R17) reached through the real gateway and pkg/modcall.
type platformEnv struct {
	*httpEnv
	idp   *httpxtest.IDP
	learn *fakeLearningFacade
	deps  command.PlatformAdminDeps
	sagas command.PlatformSagas
}

// platformHTTP builds the harness; owners wraps the gateway (nil = the gateway itself).
func platformHTTP(ctx context.Context, t *testing.T, e *env, owners func(port.OwnerServiceGateway) port.OwnerServiceGateway) *platformEnv {
	t.Helper()
	// Composition-root hook: denials are audited as authz.denied through the outbox (F045-AC-04).
	authz.Configure(authz.Hooks{Recorder: e.deps.Audit})
	t.Cleanup(func() { authz.Configure(authz.Hooks{}) })
	idp := httpxtest.NewIDP(t)
	model := read.PrincipalModel{Session: e.read}
	res := facadeResolver{f: facade.Principals{Resolve: query.ResolvePrincipal{Model: model}}}
	spec, err := httpx.LoadSpec(ctx, filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: res, Spec: spec,
		Idempotency: idempotency.NewStore(e.app, 0, nil)})
	if err != nil {
		t.Fatal(err)
	}
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	learn := newFakeLearning()
	var gw port.OwnerServiceGateway = modules.Gateway{Learning: learn}
	if owners != nil {
		gw = owners(gw)
	}
	deps := command.PlatformAdminDeps{UoW: e.uow, Repo: write.PlatformAdminRepo{}, Owners: gw, Events: e.deps.Events, Audit: e.deps.Audit, Settings: write.SettingsRepo{},
		Sagas: saga.Engine{UoW: e.uow, Repo: write.SagaRepo{}, Backoff: time.Millisecond, MaxAttempts: 3}}
	if err := identityhttp.Register(srv, identityhttp.Handlers{ListTenants: query.ListTenants{Model: read.TenantModel{Session: e.read}, Cursors: codec},
		// F011 T041: the School Admin proposes a merge on the school route before mergeUsers.
		ResolveSisIssue: command.ResolveSisIssue{UoW: e.uow, Issues: write.SisSyncRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}}); err != nil {
		t.Fatal(err)
	}
	if err := identityhttp.RegisterPlatformAdmin(srv, identityhttp.PlatformAdminHandlers{
		Reads:  query.PlatformAdmin{Model: read.PlatformAdminModel{Session: e.read}, Owners: gw, Cursors: codec},
		Assign: command.AssignToTenants{PlatformAdminDeps: deps}, Unassign: command.UnassignFromTenants{PlatformAdminDeps: deps},
		Restore: command.RestoreTrashItem{PlatformAdminDeps: deps}, Clone: command.CreateTenantClone{PlatformAdminDeps: deps},
		Merge: command.MergeUsers{PlatformAdminDeps: deps}}); err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	return &platformEnv{httpEnv: &httpEnv{env: e, srv: hs, idp: idp}, idp: idp, learn: learn, deps: deps,
		sagas: command.PlatformSagas{PlatformAdminDeps: deps}}
}

func (p *platformEnv) token(user string) string {
	return p.idp.Token("sub-"+user, httpxtest.PortalClient, time.Now())
}

// runSagas runs the job platform-saga until nothing is due (bounded).
func (p *platformEnv) runSagas(ctx context.Context, t *testing.T) {
	t.Helper()
	for i := 0; i < 20; i++ {
		n, err := p.sagas.RunOnce(ctx)
		if err != nil {
			t.Fatalf("platform-saga: %v", err)
		}
		if n == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond) // backoff of the test engine
	}
}

// ---------- fakeLearningFacade: TEST DOUBLE (R17) of the learning platform-ops facade ----------

type fakeItem struct {
	id      uuid.UUID
	tenant  *uuid.UUID
	code    string
	deleted *time.Time
	by      uuid.UUID
	from    *uuid.UUID // cloned_from_id
	status  string     // draft for clones
}

type fakeAssignment struct {
	id     uuid.UUID
	active bool
	by     uuid.UUID
	at     time.Time
}

// fakeLearningFacade imitates F020's facade rules the F045 flows rely on: platform principal for assign/clone,
// 404 for unknown or deleted resources, 422 for tenant resources, idempotent apply, clone steps replayed per
// (job, resource_type), unique code on restore (409), tenant principals limited to their tenant (404).
type fakeLearningFacade struct {
	mu          sync.Mutex
	items       map[uuid.UUID]*fakeItem
	assignments map[[2]uuid.UUID]*fakeAssignment
	cloneDone   map[string]learningapi.ExecuteTenantCloneStep200JSONResponse
	cloneCalls  int
	failClone   int // transient failures to inject before succeeding (F045-AC-17)
	failAfter   int // transient failures injected after the step took effect (its replay must not clone twice)
	trashSeen   []learningapi.ListTrashParams
	trashTenant []*uuid.UUID
	down        bool // every call fails with an infrastructure error (F045-AC-25)
}

func newFakeLearning() *fakeLearningFacade {
	return &fakeLearningFacade{items: map[uuid.UUID]*fakeItem{}, assignments: map[[2]uuid.UUID]*fakeAssignment{},
		cloneDone: map[string]learningapi.ExecuteTenantCloneStep200JSONResponse{}}
}

func (f *fakeLearningFacade) add(tenant *uuid.UUID, code string, deleted bool) uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := uuid.Must(uuid.NewV7())
	it := &fakeItem{id: id, tenant: tenant, code: code, status: "published"}
	if deleted {
		at := time.Now().UTC()
		it.deleted, it.by = &at, uuid.New()
	}
	f.items[id] = it
	return id
}

func (f *fakeLearningFacade) visible(tenant, id uuid.UUID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok || it.deleted != nil {
		return false
	}
	if it.tenant != nil {
		return *it.tenant == tenant
	}
	a, ok := f.assignments[[2]uuid.UUID{id, tenant}]
	return ok && a.active
}

func (f *fakeLearningFacade) clones(target uuid.UUID) []*fakeItem {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*fakeItem
	for _, it := range f.items {
		if it.tenant != nil && *it.tenant == target && it.from != nil {
			out = append(out, it)
		}
	}
	return out
}

func platformPrincipal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return p, problem.New(problem.Unauthenticated, "")
	}
	if !p.IsPlatform() {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

func (f *fakeLearningFacade) infra() error {
	if f.down {
		return problem.New(problem.Internal, "database unavailable")
	}
	return nil
}

func (f *fakeLearningFacade) ListTrash(ctx context.Context, req learningapi.ListTrashRequestObject) (learningapi.ListTrashResponseObject, error) {
	if err := f.infra(); err != nil {
		return nil, err
	}
	p, _ := authz.PrincipalFrom(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trashSeen = append(f.trashSeen, req.Params)
	f.trashTenant = append(f.trashTenant, p.TenantID)
	var filter []uuid.UUID
	if p.TenantID != nil {
		filter = []uuid.UUID{*p.TenantID}
	} else if req.Params.TenantIds != nil {
		filter = *req.Params.TenantIds
	}
	out := learningapi.ListTrash200JSONResponse{Data: []learningapi.CommonInternalCommonV1TrashItem{}}
	for _, it := range f.items {
		if it.deleted == nil {
			continue
		}
		if len(filter) > 0 && (it.tenant == nil || !containsID(filter, *it.tenant)) {
			continue
		}
		restorable := !f.liveCode(it)
		by, title := it.by, it.code
		out.Data = append(out.Data, learningapi.CommonInternalCommonV1TrashItem{ResourceType: "question", ResourceId: it.id, TenantId: it.tenant,
			DeletedAt: *it.deleted, DeletedBy: &by, Title: &title, Restorable: &restorable})
	}
	return out, nil
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// liveCode reports a live item of the same tenant holding the code of it (restore would be 409).
func (f *fakeLearningFacade) liveCode(it *fakeItem) bool {
	for _, o := range f.items {
		if o.id != it.id && o.deleted == nil && o.code == it.code && ((o.tenant == nil && it.tenant == nil) ||
			(o.tenant != nil && it.tenant != nil && *o.tenant == *it.tenant)) {
			return true
		}
	}
	return false
}

func (f *fakeLearningFacade) RestoreTrashItem(ctx context.Context, req learningapi.RestoreTrashItemRequestObject) (learningapi.RestoreTrashItemResponseObject, error) {
	if err := f.infra(); err != nil {
		return nil, err
	}
	p, _ := authz.PrincipalFrom(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[req.ResourceId]
	if !ok || it.deleted == nil || (p.TenantID != nil && (it.tenant == nil || *it.tenant != *p.TenantID)) {
		return nil, problem.New(problem.NotFound, "")
	}
	if f.liveCode(it) {
		return nil, problem.New(problem.Conflict, "a live item already uses the unique key of this one")
	}
	it.deleted = nil
	return learningapi.RestoreTrashItem200JSONResponse{ResourceType: "question", ResourceId: it.id, RestoredAt: time.Now()}, nil
}

func (f *fakeLearningFacade) list(rt learningapi.CommonInternalCommonV1ResourceType, id uuid.UUID, tenants []uuid.UUID) learningapi.CommonInternalCommonV1TenantAssignmentList {
	out := learningapi.CommonInternalCommonV1TenantAssignmentList{Data: []learningapi.CommonInternalCommonV1TenantAssignment{}}
	for _, t := range tenants {
		a, ok := f.assignments[[2]uuid.UUID{id, t}]
		if !ok {
			continue
		}
		st := learningapi.CommonInternalCommonV1TenantAssignmentStatus("active")
		if !a.active {
			st = "revoked"
		}
		out.Data = append(out.Data, learningapi.CommonInternalCommonV1TenantAssignment{AssignmentId: a.id, ResourceType: rt, ResourceId: id,
			TenantId: t, AssignedBy: a.by, AssignedAt: a.at, Status: st})
	}
	return out
}

func (f *fakeLearningFacade) platformResource(id uuid.UUID) error {
	it, ok := f.items[id]
	switch {
	case !ok || it.deleted != nil:
		return problem.New(problem.NotFound, "")
	case it.tenant != nil:
		return problem.Invalid("resource_id", problem.SubInvalid, "is not platform data")
	}
	return nil
}

func (f *fakeLearningFacade) ApplyTenantAssignments(ctx context.Context, req learningapi.ApplyTenantAssignmentsRequestObject) (learningapi.ApplyTenantAssignmentsResponseObject, error) {
	if err := f.infra(); err != nil {
		return nil, err
	}
	p, err := platformPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.platformResource(req.Body.ResourceId); err != nil {
		return nil, err
	}
	for _, t := range req.Body.TenantIds {
		k := [2]uuid.UUID{req.Body.ResourceId, t}
		if a, ok := f.assignments[k]; !ok || !a.active {
			f.assignments[k] = &fakeAssignment{id: uuid.Must(uuid.NewV7()), active: true, by: p.UserID, at: time.Now().UTC()}
		}
	}
	return learningapi.ApplyTenantAssignments200JSONResponse(f.list(req.Body.ResourceType, req.Body.ResourceId, req.Body.TenantIds)), nil
}

func (f *fakeLearningFacade) RevokeTenantAssignments(ctx context.Context, req learningapi.RevokeTenantAssignmentsRequestObject) (learningapi.RevokeTenantAssignmentsResponseObject, error) {
	if err := f.infra(); err != nil {
		return nil, err
	}
	if _, err := platformPrincipal(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.items[req.Body.ResourceId]; !ok {
		return nil, problem.New(problem.NotFound, "")
	}
	for _, t := range req.Body.TenantIds {
		if a, ok := f.assignments[[2]uuid.UUID{req.Body.ResourceId, t}]; ok {
			a.active = false // soft-delete of the assignment
		}
	}
	return learningapi.RevokeTenantAssignments200JSONResponse(f.list(req.Body.ResourceType, req.Body.ResourceId, req.Body.TenantIds)), nil
}

func (f *fakeLearningFacade) ListResourceAssignments(ctx context.Context, req learningapi.ListResourceAssignmentsRequestObject) (learningapi.ListResourceAssignmentsResponseObject, error) {
	if err := f.infra(); err != nil {
		return nil, err
	}
	if _, err := platformPrincipal(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var tenants []uuid.UUID
	for k := range f.assignments {
		if k[0] == req.Params.ResourceId {
			tenants = append(tenants, k[1])
		}
	}
	return learningapi.ListResourceAssignments200JSONResponse(f.list(req.Params.ResourceType, req.Params.ResourceId, tenants)), nil
}

func (f *fakeLearningFacade) ExecuteTenantCloneStep(ctx context.Context, req learningapi.ExecuteTenantCloneStepRequestObject) (learningapi.ExecuteTenantCloneStepResponseObject, error) {
	if _, err := platformPrincipal(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cloneCalls++
	if f.failClone > 0 {
		f.failClone--
		return nil, problem.New(problem.Unavailable, "").Retry()
	}
	key := req.JobId.String() + ":" + string(req.Body.ResourceType)
	if prev, ok := f.cloneDone[key]; ok { // replay of the step (F020-AC-27)
		return prev, nil
	}
	out := learningapi.ExecuteTenantCloneStep200JSONResponse{JobId: req.JobId, ResourceType: req.Body.ResourceType, Status: "succeeded"}
	target := req.Body.TargetTenantId
	for _, id := range req.Body.ResourceIds {
		src, ok := f.items[id]
		if !ok || src.deleted != nil || src.tenant == nil || *src.tenant != req.Body.SourceTenantId {
			out.Status = "partially_failed"
			continue
		}
		from := src.id
		cp := &fakeItem{id: uuid.Must(uuid.NewV7()), tenant: &target, code: src.code, from: &from, status: "draft"}
		f.items[cp.id] = cp
		out.Cloned = append(out.Cloned, struct {
			SourceId uuid.UUID `json:"source_id"`
			TargetId uuid.UUID `json:"target_id"`
		}{SourceId: src.id, TargetId: cp.id})
	}
	if len(out.Cloned) == 0 {
		out.Status = "failed"
	}
	f.cloneDone[key] = out
	if f.failAfter > 0 {
		f.failAfter--
		return nil, problem.New(problem.Unavailable, "").Retry()
	}
	return out, nil
}
