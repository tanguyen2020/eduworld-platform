package contract

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// fakeLearning is a TEST DOUBLE (R17) of the learning facade (learningapi.StrictServerInterface subset): it
// records what the call context carried and answers like the spec examples of
// specs/openapi/modules/learning.internal.v1.yaml. The real facade is tested by module learning (F020).
type fakeLearning struct {
	seen     []seenCall
	err      error
	sleep    time.Duration
	tenantID uuid.UUID
}

type seenCall struct {
	op, caller, key string
	principal       authz.Principal
	scope           bool
	deadline        time.Duration
}

func (f *fakeLearning) record(ctx context.Context, op string) error {
	p, _ := authz.PrincipalFrom(ctx)
	k, _ := modcall.IdempotencyKeyFrom(ctx)
	_, sc := tenancy.ScopeFrom(ctx)
	var d time.Duration
	if dl, ok := ctx.Deadline(); ok {
		d = time.Until(dl)
	}
	f.seen = append(f.seen, seenCall{op: op, caller: modcall.CallerFrom(ctx), key: k, principal: p, scope: sc, deadline: d})
	if f.sleep > 0 {
		select {
		case <-time.After(f.sleep):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.err
}

func (f *fakeLearning) ListTrash(ctx context.Context, req learningapi.ListTrashRequestObject) (learningapi.ListTrashResponseObject, error) {
	if err := f.record(ctx, "listTrash"); err != nil {
		return nil, err
	}
	no := false
	reason := "deleted"
	return learningapi.ListTrash200JSONResponse{Data: []learningapi.CommonInternalCommonV1TrashItem{{ResourceType: "question",
		ResourceId: uuid.New(), TenantId: &f.tenantID, DeletedAt: time.Now(), Restorable: &no, DeleteReason: &reason}}}, nil
}

func (f *fakeLearning) RestoreTrashItem(ctx context.Context, req learningapi.RestoreTrashItemRequestObject) (learningapi.RestoreTrashItemResponseObject, error) {
	if err := f.record(ctx, "restoreTrashItem"); err != nil {
		return nil, err
	}
	return learningapi.RestoreTrashItem200JSONResponse{ResourceType: req.ResourceType,
		ResourceId: req.ResourceId, RestoredAt: time.Now()}, nil
}

func (f *fakeLearning) assignments(rt learningapi.CommonInternalCommonV1ResourceType, id uuid.UUID, tenants []uuid.UUID) learningapi.CommonInternalCommonV1TenantAssignmentList {
	out := learningapi.CommonInternalCommonV1TenantAssignmentList{}
	for _, t := range tenants {
		out.Data = append(out.Data, learningapi.CommonInternalCommonV1TenantAssignment{AssignmentId: uuid.New(), ResourceType: rt, ResourceId: id,
			TenantId: t, AssignedAt: time.Now(), Status: "active"})
	}
	return out
}

func (f *fakeLearning) ApplyTenantAssignments(ctx context.Context, req learningapi.ApplyTenantAssignmentsRequestObject) (learningapi.ApplyTenantAssignmentsResponseObject, error) {
	if err := f.record(ctx, "applyTenantAssignments"); err != nil {
		return nil, err
	}
	return learningapi.ApplyTenantAssignments200JSONResponse(f.assignments(req.Body.ResourceType, req.Body.ResourceId, req.Body.TenantIds)), nil
}

func (f *fakeLearning) RevokeTenantAssignments(ctx context.Context, req learningapi.RevokeTenantAssignmentsRequestObject) (learningapi.RevokeTenantAssignmentsResponseObject, error) {
	if err := f.record(ctx, "revokeTenantAssignments"); err != nil {
		return nil, err
	}
	return learningapi.RevokeTenantAssignments200JSONResponse(f.assignments(req.Body.ResourceType, req.Body.ResourceId, req.Body.TenantIds)), nil
}

func (f *fakeLearning) ListResourceAssignments(ctx context.Context, req learningapi.ListResourceAssignmentsRequestObject) (learningapi.ListResourceAssignmentsResponseObject, error) {
	if err := f.record(ctx, "listResourceAssignments"); err != nil {
		return nil, err
	}
	return learningapi.ListResourceAssignments200JSONResponse(f.assignments(req.Params.ResourceType, req.Params.ResourceId, []uuid.UUID{f.tenantID})), nil
}

func (f *fakeLearning) ExecuteTenantCloneStep(ctx context.Context, req learningapi.ExecuteTenantCloneStepRequestObject) (learningapi.ExecuteTenantCloneStepResponseObject, error) {
	if err := f.record(ctx, "executeTenantCloneStep"); err != nil {
		return nil, err
	}
	out := learningapi.ExecuteTenantCloneStep200JSONResponse{JobId: req.JobId, ResourceType: req.Body.ResourceType, Status: "succeeded"}
	for _, id := range req.Body.ResourceIds {
		out.Cloned = append(out.Cloned, struct {
			SourceId uuid.UUID `json:"source_id"`
			TargetId uuid.UUID `json:"target_id"`
		}{SourceId: id, TargetId: uuid.New()})
	}
	return out, nil
}

func (f *fakeLearning) MergeUserData(ctx context.Context, req learningapi.MergeUserDataRequestObject) (learningapi.MergeUserDataResponseObject, error) {
	if err := f.record(ctx, "mergeUserData"); err != nil {
		return nil, err
	}
	return learningapi.MergeUserData200JSONResponse{UserId: req.UserId, Status: "done"}, nil
}

func probCode(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F045-AC-24 (TC-SVC-006): the gateway calls the learning facade in process through pkg/modcall: caller
// identity (in x-allowed-callers), the original Principal, the scope and the Idempotency-Key of the context and
// the deadline of module_call_deadline per kind; never over HTTP, never with a token.
func TestOwnerGatewayCallContext(t *testing.T) {
	for _, op := range []string{"listTrash", "restoreTrashItem", "applyTenantAssignments", "revokeTenantAssignments", "listResourceAssignments",
		"executeTenantCloneStep", "mergeUserData"} {
		if !slices.Equal(learningapi.AllowedCallers[op], []string{"identity"}) {
			t.Fatalf("%s: x-allowed-callers %v", op, learningapi.AllowedCallers[op])
		}
	}
	tenant := uuid.New()
	fake := &fakeLearning{tenantID: tenant}
	g := modules.Gateway{Learning: fake, LearningMerge: fake}
	sa := authztest.Platform("system_admin")
	sc, err := authz.PlatformScope(sa)
	if err != nil {
		t.Fatal(err)
	}
	ctx := modcall.WithIdempotencyKey(tenancy.WithScope(authztest.Ctx(context.Background(), sa), sc), "0193a5f4-6c1e-7b2a-9d1f-000000000001")
	id := uuid.New()
	if _, err := g.ApplyAssignments(ctx, "learning", "question_set", id, []uuid.UUID{tenant}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.RevokeAssignments(ctx, "learning", "question_set", id, []uuid.UUID{tenant}, "no longer used"); err != nil {
		t.Fatal(err)
	}
	if err := g.RestoreTrashItem(ctx, "learning", "question", id, "restore it"); err != nil {
		t.Fatal(err)
	}
	if page, err := g.ListTrash(ctx, "learning", port.TrashQuery{ResourceType: "question"}); err != nil || len(page.Data) != 1 ||
		page.Data[0].Restorable || page.Data[0].DeleteReason != "deleted" {
		t.Fatalf("list trash: %+v %v", page, err)
	}
	if res, err := g.CloneStep(ctx, "learning", port.CloneStepRequest{JobID: uuid.New(), Source: tenant, Target: uuid.New(), ResourceType: "question",
		ResourceIDs: []uuid.UUID{id}, IDMap: map[uuid.UUID]uuid.UUID{uuid.New(): uuid.New()}}); err != nil || len(res.Cloned) != 1 || res.Status != "succeeded" {
		t.Fatalf("clone: %+v %v", res, err)
	}
	if res, err := g.MergeUserData(ctx, "learning", id, port.MergeUserDataRequest{MergeID: uuid.New(), Into: uuid.New(), Reason: "same person"}); err != nil || res.Status != "done" {
		t.Fatalf("merge: %+v %v", res, err)
	}
	for _, c := range fake.seen {
		if c.caller != "identity" || c.principal.UserID != sa.UserID || !c.scope {
			t.Fatalf("%s: caller %q principal %v scope %v", c.op, c.caller, c.principal.UserID, c.scope)
		}
		if c.key != "0193a5f4-6c1e-7b2a-9d1f-000000000001" {
			t.Fatalf("%s: idempotency key %q", c.op, c.key)
		}
		limit := modcall.DefaultDeadlines[modcall.Write]
		switch c.op {
		case "listTrash", "listResourceAssignments":
			limit = modcall.DefaultDeadlines[modcall.Read]
		case "executeTenantCloneStep", "mergeUserData":
			limit = modcall.DefaultDeadlines[modcall.Saga]
		}
		if c.deadline <= 0 || c.deadline > limit {
			t.Fatalf("%s: deadline %v, want ≤ %v", c.op, c.deadline, limit)
		}
	}
}

// F045-AC-25 (TC-SVC-004): a module not wired, an infrastructure failure or a missed deadline is 503
// UNAVAILABLE; a 4xx problem of the callee passes through unchanged.
func TestOwnerGatewayUnavailable(t *testing.T) {
	sa := authztest.Platform("system_admin")
	sc, _ := authz.PlatformScope(sa)
	ctx := modcall.WithIdempotencyKey(tenancy.WithScope(authztest.Ctx(context.Background(), sa), sc), "0193a5f4-6c1e-7b2a-9d1f-000000000002")
	id := uuid.New()

	none := modules.Gateway{}
	if _, err := none.ListTrash(ctx, "learning", port.TrashQuery{ResourceType: "question"}); probCode(err) != problem.Unavailable {
		t.Fatalf("not wired: %v", err)
	}
	for _, m := range []string{"game", "competition", "chat"} { // no adapter yet (OQ F045-3)
		if _, err := (modules.Gateway{Learning: &fakeLearning{}}).ApplyAssignments(ctx, m, "quest", id, []uuid.UUID{uuid.New()}); probCode(err) != problem.Unavailable {
			t.Fatalf("%s: %v", m, err)
		}
	}
	if _, err := (modules.Gateway{Learning: &fakeLearning{}}).MergeUserData(ctx, "learning", id, port.MergeUserDataRequest{}); probCode(err) != problem.Unavailable {
		t.Fatalf("merge not wired: %v", err)
	}

	broken := modules.Gateway{Learning: &fakeLearning{err: errors.New("connection refused")}}
	if err := broken.RestoreTrashItem(ctx, "learning", "question", id, "restore"); probCode(err) != problem.Unavailable {
		t.Fatalf("infrastructure error: %v", err)
	}
	slow := modules.Gateway{Learning: &fakeLearning{sleep: time.Second}, Deadlines: map[modcall.Kind]time.Duration{modcall.Read: 20 * time.Millisecond}}
	if _, err := slow.ListTrash(ctx, "learning", port.TrashQuery{ResourceType: "question"}); probCode(err) != problem.Unavailable {
		t.Fatalf("deadline: %v", err)
	}
	notFound := modules.Gateway{Learning: &fakeLearning{err: problem.New(problem.NotFound, "")}}
	if _, err := notFound.ApplyAssignments(ctx, "learning", "question_set", id, []uuid.UUID{uuid.New()}); probCode(err) != problem.NotFound {
		t.Fatalf("4xx pass-through: %v", err)
	}
}
