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

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	competitionapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
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

// GameAssignmentOps are the tenant assignment operations of module game (F078-AC-21).
type GameAssignmentOps interface {
	ApplyTenantAssignments(ctx context.Context, req gameapi.ApplyTenantAssignmentsRequestObject) (gameapi.ApplyTenantAssignmentsResponseObject, error)
	RevokeTenantAssignments(ctx context.Context, req gameapi.RevokeTenantAssignmentsRequestObject) (gameapi.RevokeTenantAssignmentsResponseObject, error)
	ListResourceAssignments(ctx context.Context, req gameapi.ListResourceAssignmentsRequestObject) (gameapi.ListResourceAssignmentsResponseObject, error)
}

// CompetitionMergeOps is mergeUserData of module competition (F100 T042, OQ F016-2, F045-3).
type CompetitionMergeOps interface {
	MergeUserData(ctx context.Context, req competitionapi.MergeUserDataRequestObject) (competitionapi.MergeUserDataResponseObject, error)
}

// CompetitionAssignmentOps are the template assignments of module competition (tournament_template, challenge_template —
// F107-AC-29).
type CompetitionAssignmentOps interface {
	ApplyTenantAssignments(ctx context.Context, req competitionapi.ApplyTenantAssignmentsRequestObject) (competitionapi.ApplyTenantAssignmentsResponseObject, error)
	RevokeTenantAssignments(ctx context.Context, req competitionapi.RevokeTenantAssignmentsRequestObject) (competitionapi.RevokeTenantAssignmentsResponseObject, error)
	ListResourceAssignments(ctx context.Context, req competitionapi.ListResourceAssignmentsRequestObject) (competitionapi.ListResourceAssignmentsResponseObject, error)
}

// ChatAssignmentOps are the sticker pack assignments of module chat (F082 T040).
type ChatAssignmentOps interface {
	ApplyTenantAssignments(ctx context.Context, req chatapi.ApplyTenantAssignmentsRequestObject) (chatapi.ApplyTenantAssignmentsResponseObject, error)
	RevokeTenantAssignments(ctx context.Context, req chatapi.RevokeTenantAssignmentsRequestObject) (chatapi.RevokeTenantAssignmentsResponseObject, error)
	ListResourceAssignments(ctx context.Context, req chatapi.ListResourceAssignmentsRequestObject) (chatapi.ListResourceAssignmentsResponseObject, error)
}

// GameMergeOps is mergeUserData of module game (F070 T039, OQ F016-2, F045-3).
type GameMergeOps interface {
	MergeUserData(ctx context.Context, req gameapi.MergeUserDataRequestObject) (gameapi.MergeUserDataResponseObject, error)
}

// Gateway implements port.OwnerServiceGateway. A nil facade means the module is not wired in this binary:
// every call to it is 503 UNAVAILABLE, never a silent success (F045-AC-25). Module learning has every adapter; module game
// serves mergeUserData (F070) and its assignments (F078); competition serves mergeUserData (F100) and the template assignments
// (F107); chat serves the sticker pack assignments (F082 T040).
type Gateway struct {
	Learning      LearningPlatformOps
	LearningMerge LearningMergeOps
	GameMerge     GameMergeOps
	// GameAssign are the tenant assignments of the game resources (F078).
	GameAssign GameAssignmentOps
	// CompetitionMerge is mergeUserData of module competition (F100 T042).
	CompetitionMerge CompetitionMergeOps
	// CompetitionAssign assigns the tournament and challenge templates of module competition (F107).
	CompetitionAssign CompetitionAssignmentOps
	// ChatAssign assigns the sticker packs of module chat (F082 T040).
	ChatAssign ChatAssignmentOps
	// Deadlines override module_call_deadline per kind (configuration, tests); zero = modcall defaults.
	Deadlines map[modcall.Kind]time.Duration
}

var _ port.OwnerServiceGateway = Gateway{}

func notWired(module, op string) error {
	return problem.New(problem.Unavailable, "").Retry().Wrap(fmt.Errorf("module %s does not serve %s in this binary", module, op))
}

func (g Gateway) gameOp(op string, kind modcall.Kind) modcall.Op {
	return modcall.Op{Caller: caller, Callee: "game", Operation: op, Kind: kind, Deadline: g.Deadlines[kind],
		AllowedCallers: gameapi.AllowedCallers[op], ContextRequired: gameapi.ContextRequired[op]}
}

func gameAssignments(l gameapi.CommonInternalCommonV1TenantAssignmentList) []port.ResourceAssignment {
	out := make([]port.ResourceAssignment, 0, len(l.Data))
	for _, a := range l.Data {
		out = append(out, port.ResourceAssignment{AssignmentID: a.AssignmentId, ResourceType: string(a.ResourceType), ResourceID: a.ResourceId,
			TenantID: a.TenantId, AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt, Active: a.Status == "active"})
	}
	return out
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

func (g Gateway) competitionOp(op string, kind modcall.Kind) modcall.Op {
	return modcall.Op{Caller: caller, Callee: "competition", Operation: op, Kind: kind, Deadline: g.Deadlines[kind],
		AllowedCallers: competitionapi.AllowedCallers[op], ContextRequired: competitionapi.ContextRequired[op]}
}

func competitionAssignments(list competitionapi.CommonInternalCommonV1TenantAssignmentList) []port.ResourceAssignment {
	out := make([]port.ResourceAssignment, 0, len(list.Data))
	for _, a := range list.Data {
		out = append(out, port.ResourceAssignment{AssignmentID: a.AssignmentId, ResourceType: string(a.ResourceType), ResourceID: a.ResourceId,
			TenantID: a.TenantId, AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt, Active: a.Status == "active"})
	}
	return out
}

// competitionApply assigns a template of module competition (F107-AC-29).
func (g Gateway) competitionApply(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID) ([]port.ResourceAssignment, error) {
	if g.CompetitionAssign == nil {
		return nil, notWired("competition", "applyTenantAssignments")
	}
	res, err := call(ctx, g.competitionOp("applyTenantAssignments", modcall.Write), func(ctx context.Context) (competitionapi.ApplyTenantAssignmentsResponseObject, error) {
		return g.CompetitionAssign.ApplyTenantAssignments(ctx, competitionapi.ApplyTenantAssignmentsRequestObject{Body: &competitionapi.ApplyTenantAssignmentsJSONRequestBody{
			ResourceType: competitionapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id, TenantIds: tenants}})
	})
	if err != nil {
		return nil, err
	}
	list, ok := res.(competitionapi.ApplyTenantAssignments200JSONResponse)
	if !ok {
		return nil, unexpected("competition", "applyTenantAssignments", res)
	}
	return competitionAssignments(competitionapi.CommonInternalCommonV1TenantAssignmentList(list)), nil
}

// competitionRevoke unassigns a template of module competition (F107-AC-29).
func (g Gateway) competitionRevoke(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID, reason string) ([]port.ResourceAssignment, error) {
	if g.CompetitionAssign == nil {
		return nil, notWired("competition", "revokeTenantAssignments")
	}
	res, err := call(ctx, g.competitionOp("revokeTenantAssignments", modcall.Write), func(ctx context.Context) (competitionapi.RevokeTenantAssignmentsResponseObject, error) {
		return g.CompetitionAssign.RevokeTenantAssignments(ctx, competitionapi.RevokeTenantAssignmentsRequestObject{Body: &competitionapi.RevokeTenantAssignmentsJSONRequestBody{
			ResourceType: competitionapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id, TenantIds: tenants, Reason: reason}})
	})
	if err != nil {
		return nil, err
	}
	list, ok := res.(competitionapi.RevokeTenantAssignments200JSONResponse)
	if !ok {
		return nil, unexpected("competition", "revokeTenantAssignments", res)
	}
	return competitionAssignments(competitionapi.CommonInternalCommonV1TenantAssignmentList(list)), nil
}

// competitionList lists the assignments of a template of module competition (F107-AC-29).
func (g Gateway) competitionList(ctx context.Context, resourceType string, id uuid.UUID, cursor string, limit int) (port.AssignmentPage, error) {
	if g.CompetitionAssign == nil {
		return port.AssignmentPage{}, notWired("competition", "listResourceAssignments")
	}
	params := competitionapi.ListResourceAssignmentsParams{ResourceType: competitionapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id}
	if cursor != "" {
		params.Cursor = &cursor
	}
	if limit > 0 {
		params.Limit = &limit
	}
	res, err := call(ctx, g.competitionOp("listResourceAssignments", modcall.Read), func(ctx context.Context) (competitionapi.ListResourceAssignmentsResponseObject, error) {
		return g.CompetitionAssign.ListResourceAssignments(ctx, competitionapi.ListResourceAssignmentsRequestObject{Params: params})
	})
	if err != nil {
		return port.AssignmentPage{}, err
	}
	list, ok := res.(competitionapi.ListResourceAssignments200JSONResponse)
	if !ok {
		return port.AssignmentPage{}, unexpected("competition", "listResourceAssignments", res)
	}
	l := competitionapi.CommonInternalCommonV1TenantAssignmentList(list)
	return port.AssignmentPage{Data: competitionAssignments(l), HasMore: l.Page.HasMore, NextCursor: l.Page.NextCursor}, nil
}

func (g Gateway) chatOp(op string, kind modcall.Kind) modcall.Op {
	return modcall.Op{Caller: caller, Callee: "chat", Operation: op, Kind: kind, Deadline: g.Deadlines[kind],
		AllowedCallers: chatapi.AllowedCallers[op], ContextRequired: chatapi.ContextRequired[op]}
}

func chatAssignments(list chatapi.CommonInternalCommonV1TenantAssignmentList) []port.ResourceAssignment {
	out := make([]port.ResourceAssignment, 0, len(list.Data))
	for _, a := range list.Data {
		out = append(out, port.ResourceAssignment{AssignmentID: a.AssignmentId, ResourceType: string(a.ResourceType), ResourceID: a.ResourceId,
			TenantID: a.TenantId, AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt, Active: a.Status == "active"})
	}
	return out
}

// chatApply assigns a sticker pack of module chat (F082 T040).
func (g Gateway) chatApply(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID) ([]port.ResourceAssignment, error) {
	if g.ChatAssign == nil {
		return nil, notWired("chat", "applyTenantAssignments")
	}
	res, err := call(ctx, g.chatOp("applyTenantAssignments", modcall.Write), func(ctx context.Context) (chatapi.ApplyTenantAssignmentsResponseObject, error) {
		return g.ChatAssign.ApplyTenantAssignments(ctx, chatapi.ApplyTenantAssignmentsRequestObject{Body: &chatapi.ApplyTenantAssignmentsJSONRequestBody{
			ResourceType: chatapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id, TenantIds: tenants}})
	})
	if err != nil {
		return nil, err
	}
	list, ok := res.(chatapi.ApplyTenantAssignments200JSONResponse)
	if !ok {
		return nil, unexpected("chat", "applyTenantAssignments", res)
	}
	return chatAssignments(chatapi.CommonInternalCommonV1TenantAssignmentList(list)), nil
}

// chatRevoke unassigns a sticker pack of module chat (F082 T040).
func (g Gateway) chatRevoke(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID, reason string) ([]port.ResourceAssignment, error) {
	if g.ChatAssign == nil {
		return nil, notWired("chat", "revokeTenantAssignments")
	}
	res, err := call(ctx, g.chatOp("revokeTenantAssignments", modcall.Write), func(ctx context.Context) (chatapi.RevokeTenantAssignmentsResponseObject, error) {
		return g.ChatAssign.RevokeTenantAssignments(ctx, chatapi.RevokeTenantAssignmentsRequestObject{Body: &chatapi.RevokeTenantAssignmentsJSONRequestBody{
			ResourceType: chatapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id, TenantIds: tenants, Reason: reason}})
	})
	if err != nil {
		return nil, err
	}
	list, ok := res.(chatapi.RevokeTenantAssignments200JSONResponse)
	if !ok {
		return nil, unexpected("chat", "revokeTenantAssignments", res)
	}
	return chatAssignments(chatapi.CommonInternalCommonV1TenantAssignmentList(list)), nil
}

// chatList lists the assignments of a sticker pack of module chat (F082 T040).
func (g Gateway) chatList(ctx context.Context, resourceType string, id uuid.UUID, cursor string, limit int) (port.AssignmentPage, error) {
	if g.ChatAssign == nil {
		return port.AssignmentPage{}, notWired("chat", "listResourceAssignments")
	}
	params := chatapi.ListResourceAssignmentsParams{ResourceType: chatapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id}
	if cursor != "" {
		params.Cursor = &cursor
	}
	if limit > 0 {
		params.Limit = &limit
	}
	res, err := call(ctx, g.chatOp("listResourceAssignments", modcall.Read), func(ctx context.Context) (chatapi.ListResourceAssignmentsResponseObject, error) {
		return g.ChatAssign.ListResourceAssignments(ctx, chatapi.ListResourceAssignmentsRequestObject{Params: params})
	})
	if err != nil {
		return port.AssignmentPage{}, err
	}
	list, ok := res.(chatapi.ListResourceAssignments200JSONResponse)
	if !ok {
		return port.AssignmentPage{}, unexpected("chat", "listResourceAssignments", res)
	}
	l := chatapi.CommonInternalCommonV1TenantAssignmentList(list)
	return port.AssignmentPage{Data: chatAssignments(l), HasMore: l.Page.HasMore, NextCursor: l.Page.NextCursor}, nil
}

// ApplyAssignments implements port.OwnerServiceGateway.
func (g Gateway) ApplyAssignments(ctx context.Context, module, resourceType string, id uuid.UUID, tenants []uuid.UUID) ([]port.ResourceAssignment, error) {
	if module == "chat" {
		return g.chatApply(ctx, resourceType, id, tenants)
	}
	if module == "game" && g.GameAssign != nil {
		res, err := call(ctx, g.gameOp("applyTenantAssignments", modcall.Write), func(ctx context.Context) (gameapi.ApplyTenantAssignmentsResponseObject, error) {
			return g.GameAssign.ApplyTenantAssignments(ctx, gameapi.ApplyTenantAssignmentsRequestObject{Body: &gameapi.ApplyTenantAssignmentsJSONRequestBody{
				ResourceType: gameapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id, TenantIds: tenants}})
		})
		if err != nil {
			return nil, err
		}
		list, ok := res.(gameapi.ApplyTenantAssignments200JSONResponse)
		if !ok {
			return nil, unexpected(module, "applyTenantAssignments", res)
		}
		return gameAssignments(gameapi.CommonInternalCommonV1TenantAssignmentList(list)), nil
	}
	if module == "competition" {
		return g.competitionApply(ctx, resourceType, id, tenants)
	}
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
	if module == "chat" {
		return g.chatRevoke(ctx, resourceType, id, tenants, reason)
	}
	if module == "game" && g.GameAssign != nil {
		res, err := call(ctx, g.gameOp("revokeTenantAssignments", modcall.Write), func(ctx context.Context) (gameapi.RevokeTenantAssignmentsResponseObject, error) {
			return g.GameAssign.RevokeTenantAssignments(ctx, gameapi.RevokeTenantAssignmentsRequestObject{Body: &gameapi.RevokeTenantAssignmentsJSONRequestBody{
				ResourceType: gameapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id, TenantIds: tenants, Reason: reason}})
		})
		if err != nil {
			return nil, err
		}
		list, ok := res.(gameapi.RevokeTenantAssignments200JSONResponse)
		if !ok {
			return nil, unexpected(module, "revokeTenantAssignments", res)
		}
		return gameAssignments(gameapi.CommonInternalCommonV1TenantAssignmentList(list)), nil
	}
	if module == "competition" {
		return g.competitionRevoke(ctx, resourceType, id, tenants, reason)
	}
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
	if module == "chat" {
		return g.chatList(ctx, resourceType, id, cursor, limit)
	}
	if module == "game" && g.GameAssign != nil {
		params := gameapi.ListResourceAssignmentsParams{ResourceType: gameapi.CommonInternalCommonV1ResourceType(resourceType), ResourceId: id}
		if cursor != "" {
			params.Cursor = &cursor
		}
		if limit > 0 {
			params.Limit = &limit
		}
		res, err := call(ctx, g.gameOp("listResourceAssignments", modcall.Read), func(ctx context.Context) (gameapi.ListResourceAssignmentsResponseObject, error) {
			return g.GameAssign.ListResourceAssignments(ctx, gameapi.ListResourceAssignmentsRequestObject{Params: params})
		})
		if err != nil {
			return port.AssignmentPage{}, err
		}
		list, ok := res.(gameapi.ListResourceAssignments200JSONResponse)
		if !ok {
			return port.AssignmentPage{}, unexpected(module, "listResourceAssignments", res)
		}
		l := gameapi.CommonInternalCommonV1TenantAssignmentList(list)
		return port.AssignmentPage{Data: gameAssignments(l), HasMore: l.Page.HasMore, NextCursor: l.Page.NextCursor}, nil
	}
	if module == "competition" {
		return g.competitionList(ctx, resourceType, id, cursor, limit)
	}
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
	if module == "competition" {
		return g.competitionMerge(ctx, from, req)
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

func (g Gateway) competitionMerge(ctx context.Context, from uuid.UUID, req port.MergeUserDataRequest) (port.MergeUserDataResult, error) {
	if g.CompetitionMerge == nil {
		return port.MergeUserDataResult{}, notWired("competition", "mergeUserData")
	}
	op := modcall.Op{Caller: caller, Callee: "competition", Operation: "mergeUserData", Kind: modcall.Saga, Deadline: g.Deadlines[modcall.Saga],
		AllowedCallers: competitionapi.AllowedCallers["mergeUserData"], ContextRequired: competitionapi.ContextRequired["mergeUserData"]}
	res, err := call(ctx, op, func(ctx context.Context) (competitionapi.MergeUserDataResponseObject, error) {
		return g.CompetitionMerge.MergeUserData(ctx, competitionapi.MergeUserDataRequestObject{UserId: from,
			Body: &competitionapi.MergeUserDataJSONRequestBody{MergeId: req.MergeID, TargetUserId: req.Into, Reason: req.Reason}})
	})
	if err != nil {
		return port.MergeUserDataResult{}, err
	}
	r, ok := res.(competitionapi.MergeUserData200JSONResponse)
	if !ok {
		return port.MergeUserDataResult{}, unexpected("competition", "mergeUserData", res)
	}
	out := port.MergeUserDataResult{Status: string(r.Status)}
	if r.BlockedReason != nil {
		out.BlockedReason = *r.BlockedReason
	}
	return out, nil
}
