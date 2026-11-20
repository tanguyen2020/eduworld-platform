// Package modules adapts the facades of the other modules to the ports of module identity (ADR-0074, AGENTS
// R29): each call goes in process through pkg/modcall (span, deadline, x-allowed-callers, x-context-required,
// the original Principal/scope/Idempotency-Key of ctx), never over HTTP and never with a token.
package modules

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// caller is the module name of identity in x-allowed-callers.
const caller = "identity"

// LearningPlatformOps is the part of learningapi.StrictServerInterface the gateway calls: the platform-ops
// operations implemented by F020 (the learning facade).
type LearningPlatformOps interface {
	ListTrash(ctx context.Context, req learningapi.ListTrashRequestObject) (learningapi.ListTrashResponseObject, error)
	RestoreTrashItem(ctx context.Context, req learningapi.RestoreTrashItemRequestObject) (learningapi.RestoreTrashItemResponseObject, error)
	ApplyTenantAssignments(ctx context.Context, req learningapi.ApplyTenantAssignmentsRequestObject) (learningapi.ApplyTenantAssignmentsResponseObject, error)
	RevokeTenantAssignments(ctx context.Context, req learningapi.RevokeTenantAssignmentsRequestObject) (learningapi.RevokeTenantAssignmentsResponseObject, error)
	ListResourceAssignments(ctx context.Context, req learningapi.ListResourceAssignmentsRequestObject) (learningapi.ListResourceAssignmentsResponseObject, error)
	ExecuteTenantCloneStep(ctx context.Context, req learningapi.ExecuteTenantCloneStepRequestObject) (learningapi.ExecuteTenantCloneStepResponseObject, error)
}

// LearningMergeOps is mergeUserData of module learning (F030 T056, OQ F016-2, F045-3).
type LearningMergeOps interface {
	MergeUserData(ctx context.Context, req learningapi.MergeUserDataRequestObject) (learningapi.MergeUserDataResponseObject, error)
}

// GameMergeOps is mergeUserData of module game (F070 T039, OQ F016-2, F045-3).
type GameMergeOps interface {
	MergeUserData(ctx context.Context, req gameapi.MergeUserDataRequestObject) (gameapi.MergeUserDataResponseObject, error)
}

// Gateway implements port.OwnerServiceGateway. A nil facade means the module is not wired in this binary:
// every call to it is 503 UNAVAILABLE, never a silent success (F045-AC-25). Module learning has every adapter; module game
// serves mergeUserData only (F070); competition and chat implement no platform-ops facade yet (OQ F045-3).
type Gateway struct {
	Learning      LearningPlatformOps
	LearningMerge LearningMergeOps
	GameMerge     GameMergeOps
	// Deadlines override module_call_deadline per kind (configuration, tests); zero = modcall defaults.
	Deadlines map[modcall.Kind]time.Duration
}

var _ port.OwnerServiceGateway = Gateway{}

func notWired(module, op string) error {
	return problem.New(problem.Unavailable, "").Retry().Wrap(fmt.Errorf("module %s does not serve %s in this binary", module, op))
}

func (g Gateway) learningOp(op string, kind modcall.Kind) modcall.Op {
	return modcall.Op{Caller: caller, Callee: "learning", Operation: op, Kind: kind, Deadline: g.Deadlines[kind],
		AllowedCallers: learningapi.AllowedCallers[op], ContextRequired: learningapi.ContextRequired[op]}
}

// call runs the facade method through pkg/modcall; an infrastructure failure of the callee (5xx, panic,
// deadline) is 503 UNAVAILABLE for the caller, never a guessed result (F045-AC-25); 4xx problems pass through.
func call[T any](ctx context.Context, op modcall.Op, fn func(ctx context.Context) (T, error)) (T, error) {
	v, err := modcall.Call(ctx, op, fn)
	var pe *problem.Error
	if err != nil && errors.As(err, &pe) && problem.Status(pe.Code) >= 500 && pe.Code != problem.Unavailable {
		return v, problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	return v, err
}

func unexpected(module, op string, res any) error {
	return problem.New(problem.Internal, "").Wrap(fmt.Errorf("%s.%s: unexpected response %T", module, op, res))
}

// ListTrash implements port.OwnerServiceGateway.
func (g Gateway) ListTrash(ctx context.Context, module string, q port.TrashQuery) (port.TrashPage, error) {
	if module != "learning" || g.Learning == nil {
		return port.TrashPage{}, notWired(module, "listTrash")
	}
	rt := learningapi.CommonInternalCommonV1ResourceType(q.ResourceType)
	params := learningapi.ListTrashParams{ResourceType: &rt}
	if len(q.TenantIDs) > 0 {
		ids := append([]uuid.UUID(nil), q.TenantIDs...)
		params.TenantIds = &ids
	}
	if q.Cursor != "" {
		c := q.Cursor
		params.Cursor = &c
	}
	if q.Limit > 0 {
		l := q.Limit
		params.Limit = &l
	}
	res, err := call(ctx, g.learningOp("listTrash", modcall.Read), func(ctx context.Context) (learningapi.ListTrashResponseObject, error) {
		return g.Learning.ListTrash(ctx, learningapi.ListTrashRequestObject{Params: params})
	})
	if err != nil {
		return port.TrashPage{}, err
	}
	page, ok := res.(learningapi.ListTrash200JSONResponse)
	if !ok {
		return port.TrashPage{}, unexpected(module, "listTrash", res)
	}
	out := port.TrashPage{Data: make([]port.TrashEntry, 0, len(page.Data)), HasMore: page.Page.HasMore, NextCursor: page.Page.NextCursor}
	for _, it := range page.Data {
		e := port.TrashEntry{ResourceType: string(it.ResourceType), ResourceID: it.ResourceId, TenantID: it.TenantId, DeletedAt: it.DeletedAt,
			Restorable: true}
		if it.DeletedBy != nil {
			e.DeletedBy = *it.DeletedBy
		}
		if it.Title != nil {
			e.Title = *it.Title
		}
		if it.DeleteReason != nil {
			e.DeleteReason = *it.DeleteReason
		}
		if it.Restorable != nil {
			e.Restorable = *it.Restorable
		}
		out.Data = append(out.Data, e)
	}
	return out, nil
}

// RestoreTrashItem implements port.OwnerServiceGateway.
func (g Gateway) RestoreTrashItem(ctx context.Context, module, resourceType string, id uuid.UUID, reason string) error {
	if module != "learning" || g.Learning == nil {
		return notWired(module, "restoreTrashItem")
	}
	res, err := call(ctx, g.learningOp("restoreTrashItem", modcall.Write), func(ctx context.Context) (learningapi.RestoreTrashItemResponseObject, error) {
		return g.Learning.RestoreTrashItem(ctx, learningapi.RestoreTrashItemRequestObject{
			ResourceType: learningapi.CommonInternalCommonV1ResourceTypePath(resourceType), ResourceId: id,
			Body: &learningapi.RestoreTrashItemJSONRequestBody{Reason: reason}})
	})
	if err != nil {
		return err
	}
	if _, ok := res.(learningapi.RestoreTrashItem200JSONResponse); !ok {
		return unexpected(module, "restoreTrashItem", res)
	}
	return nil
}

func assignments(list learningapi.CommonInternalCommonV1TenantAssignmentList) []port.ResourceAssignment {
	out := make([]port.ResourceAssignment, 0, len(list.Data))
	for _, a := range list.Data {
		out = append(out, port.ResourceAssignment{AssignmentID: a.AssignmentId, ResourceType: string(a.ResourceType), ResourceID: a.ResourceId,
			TenantID: a.TenantId, AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt, Active: a.Status == "active"})
	}
	return out
}

// ApplyAssignments implements port.OwnerServiceGateway.
func (g Gateway) ApplyAssignments(ctx context.Context, module, resourceType string, id uuid.UUID, tenants []uuid.UUID) ([]port.ResourceAssignment, error) {
	if module != "learning" || g.Learning == nil {
		return nil, notWired(module, "applyTenantAssignments")
	}
	res, err := call(ctx, g.learningOp("applyTenantAssignments", modcall.Write), func(ctx context.Context) (learningapi.ApplyTenantAssignmentsResponseObject, error) {
		return g.Learning.ApplyTenantAssignments(ctx, learningapi.ApplyTenantAssignmentsRequestObject{Body: &learningapi.ApplyTenantAssignmentsJSONRequestBody{
			ResourceType: learningapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id, TenantIds: tenants}})
	})
	if err != nil {
		return nil, err
	}
	list, ok := res.(learningapi.ApplyTenantAssignments200JSONResponse)
	if !ok {
		return nil, unexpected(module, "applyTenantAssignments", res)
	}
	return assignments(learningapi.CommonInternalCommonV1TenantAssignmentList(list)), nil
}

// RevokeAssignments implements port.OwnerServiceGateway.
func (g Gateway) RevokeAssignments(ctx context.Context, module, resourceType string, id uuid.UUID, tenants []uuid.UUID, reason string) ([]port.ResourceAssignment, error) {
	if module != "learning" || g.Learning == nil {
		return nil, notWired(module, "revokeTenantAssignments")
	}
	res, err := call(ctx, g.learningOp("revokeTenantAssignments", modcall.Write), func(ctx context.Context) (learningapi.RevokeTenantAssignmentsResponseObject, error) {
		return g.Learning.RevokeTenantAssignments(ctx, learningapi.RevokeTenantAssignmentsRequestObject{Body: &learningapi.RevokeTenantAssignmentsJSONRequestBody{
			ResourceType: learningapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id, TenantIds: tenants, Reason: reason}})
	})
	if err != nil {
		return nil, err
	}
	list, ok := res.(learningapi.RevokeTenantAssignments200JSONResponse)
	if !ok {
		return nil, unexpected(module, "revokeTenantAssignments", res)
	}
	return assignments(learningapi.CommonInternalCommonV1TenantAssignmentList(list)), nil
}

// ListAssignments implements port.OwnerServiceGateway.
func (g Gateway) ListAssignments(ctx context.Context, module, resourceType string, id uuid.UUID, cursor string, limit int) (port.AssignmentPage, error) {
	if module != "learning" || g.Learning == nil {
		return port.AssignmentPage{}, notWired(module, "listResourceAssignments")
	}
	params := learningapi.ListResourceAssignmentsParams{ResourceType: learningapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id}
	if cursor != "" {
		params.Cursor = &cursor
	}
	if limit > 0 {
		params.Limit = &limit
	}
	res, err := call(ctx, g.learningOp("listResourceAssignments", modcall.Read), func(ctx context.Context) (learningapi.ListResourceAssignmentsResponseObject, error) {
		return g.Learning.ListResourceAssignments(ctx, learningapi.ListResourceAssignmentsRequestObject{Params: params})
	})
	if err != nil {
		return port.AssignmentPage{}, err
	}
	list, ok := res.(learningapi.ListResourceAssignments200JSONResponse)
	if !ok {
		return port.AssignmentPage{}, unexpected(module, "listResourceAssignments", res)
	}
	l := learningapi.CommonInternalCommonV1TenantAssignmentList(list)
	return port.AssignmentPage{Data: assignments(l), HasMore: l.Page.HasMore, NextCursor: l.Page.NextCursor}, nil
}

// CloneStep implements port.OwnerServiceGateway.
func (g Gateway) CloneStep(ctx context.Context, module string, req port.CloneStepRequest) (port.CloneStepResult, error) {
	if module != "learning" || g.Learning == nil {
		return port.CloneStepResult{}, notWired(module, "executeTenantCloneStep")
	}
	body := learningapi.ExecuteTenantCloneStepJSONRequestBody{ResourceType: learningapi.CommonInternalCommonV1ResourceType(req.ResourceType),
		ResourceIds: req.ResourceIDs, SourceTenantId: req.Source, TargetTenantId: req.Target}
	if len(req.IDMap) > 0 {
		m := make(map[string]uuid.UUID, len(req.IDMap))
		for k, v := range req.IDMap {
			m[k.String()] = v
		}
		body.IdMap = &m
	}
	res, err := call(ctx, g.learningOp("executeTenantCloneStep", modcall.Saga), func(ctx context.Context) (learningapi.ExecuteTenantCloneStepResponseObject, error) {
		return g.Learning.ExecuteTenantCloneStep(ctx, learningapi.ExecuteTenantCloneStepRequestObject{JobId: req.JobID, Body: &body})
	})
	if err != nil {
		return port.CloneStepResult{}, err
	}
	r, ok := res.(learningapi.ExecuteTenantCloneStep200JSONResponse)
	if !ok {
		return port.CloneStepResult{}, unexpected(module, "executeTenantCloneStep", res)
	}
	out := port.CloneStepResult{Status: string(r.Status)}
	for _, c := range r.Cloned {
		out.Cloned = append(out.Cloned, port.ClonedPair{SourceID: c.SourceId, TargetID: c.TargetId})
	}
	if r.Skipped != nil {
		out.Skipped = len(*r.Skipped)
	}
	return out, nil
}

// MergeUserData implements port.OwnerServiceGateway.
func (g Gateway) MergeUserData(ctx context.Context, module string, from uuid.UUID, req port.MergeUserDataRequest) (port.MergeUserDataResult, error) {
	if module == "game" {
		return g.gameMerge(ctx, from, req)
	}
	if module != "learning" || g.LearningMerge == nil {
		return port.MergeUserDataResult{}, notWired(module, "mergeUserData")
	}
	res, err := call(ctx, g.learningOp("mergeUserData", modcall.Saga), func(ctx context.Context) (learningapi.MergeUserDataResponseObject, error) {
		return g.LearningMerge.MergeUserData(ctx, learningapi.MergeUserDataRequestObject{UserId: from,
			Body: &learningapi.MergeUserDataJSONRequestBody{MergeId: req.MergeID, TargetUserId: req.Into, Reason: req.Reason}})
	})
	if err != nil {
		return port.MergeUserDataResult{}, err
	}
	r, ok := res.(learningapi.MergeUserData200JSONResponse)
	if !ok {
		return port.MergeUserDataResult{}, unexpected(module, "mergeUserData", res)
	}
	out := port.MergeUserDataResult{Status: string(r.Status)}
	if r.BlockedReason != nil {
		out.BlockedReason = *r.BlockedReason
	}
	return out, nil
}

func (g Gateway) gameMerge(ctx context.Context, from uuid.UUID, req port.MergeUserDataRequest) (port.MergeUserDataResult, error) {
	if g.GameMerge == nil {
		return port.MergeUserDataResult{}, notWired("game", "mergeUserData")
	}
	op := modcall.Op{Caller: caller, Callee: "game", Operation: "mergeUserData", Kind: modcall.Saga, Deadline: g.Deadlines[modcall.Saga],
		AllowedCallers: gameapi.AllowedCallers["mergeUserData"], ContextRequired: gameapi.ContextRequired["mergeUserData"]}
	res, err := call(ctx, op, func(ctx context.Context) (gameapi.MergeUserDataResponseObject, error) {
		return g.GameMerge.MergeUserData(ctx, gameapi.MergeUserDataRequestObject{UserId: from,
			Body: &gameapi.MergeUserDataJSONRequestBody{MergeId: req.MergeID, TargetUserId: req.Into, Reason: req.Reason}})
	})
	if err != nil {
		return port.MergeUserDataResult{}, err
	}
	r, ok := res.(gameapi.MergeUserData200JSONResponse)
	if !ok {
		return port.MergeUserDataResult{}, unexpected("game", "mergeUserData", res)
	}
	out := port.MergeUserDataResult{Status: string(r.Status)}
	if r.BlockedReason != nil {
		out.BlockedReason = *r.BlockedReason
	}
	return out, nil
}
