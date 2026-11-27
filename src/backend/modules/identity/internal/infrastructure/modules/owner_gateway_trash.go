package modules

import (
	"context"
	"time"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	competitionapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// GameTrashOps are listTrash and restoreTrashItem of module game: the soft-deleted console items of challenge, quest,
// minigame_config, item, shop_item and event_template (F057, ADR-0028).
type GameTrashOps interface {
	ListTrash(ctx context.Context, req gameapi.ListTrashRequestObject) (gameapi.ListTrashResponseObject, error)
	RestoreTrashItem(ctx context.Context, req gameapi.RestoreTrashItemRequestObject) (gameapi.RestoreTrashItemResponseObject, error)
}

// CompetitionTrashOps are listTrash and restoreTrashItem of module competition: the soft-deleted tournament and challenge
// templates (F057, ADR-0028).
type CompetitionTrashOps interface {
	ListTrash(ctx context.Context, req competitionapi.ListTrashRequestObject) (competitionapi.ListTrashResponseObject, error)
	RestoreTrashItem(ctx context.Context, req competitionapi.RestoreTrashItemRequestObject) (competitionapi.RestoreTrashItemResponseObject, error)
}

// ChatTrashOps are listTrash and restoreTrashItem of module chat: the soft-deleted sticker packs (F057, ADR-0028).
type ChatTrashOps interface {
	ListTrash(ctx context.Context, req chatapi.ListTrashRequestObject) (chatapi.ListTrashResponseObject, error)
	RestoreTrashItem(ctx context.Context, req chatapi.RestoreTrashItemRequestObject) (chatapi.RestoreTrashItemResponseObject, error)
}

// trashParams are the query parameters of listTrash, common to every owner (the generated types differ per module).
type trashParams struct {
	tenants *[]uuid.UUID
	cursor  *string
	limit   *int
}

func paramsOf(q port.TrashQuery) trashParams {
	var p trashParams
	if len(q.TenantIDs) > 0 {
		ids := append([]uuid.UUID(nil), q.TenantIDs...)
		p.tenants = &ids
	}
	if q.Cursor != "" {
		c := q.Cursor
		p.cursor = &c
	}
	if q.Limit > 0 {
		l := q.Limit
		p.limit = &l
	}
	return p
}

// trashEntry maps one TrashItem of an owner (restorable absent = true, as the contract default).
func trashEntry(rt string, id uuid.UUID, tenant *uuid.UUID, at time.Time, by *uuid.UUID, title, reason *string, restorable *bool) port.TrashEntry {
	e := port.TrashEntry{ResourceType: rt, ResourceID: id, TenantID: tenant, DeletedAt: at, Restorable: true}
	if by != nil {
		e.DeletedBy = *by
	}
	if title != nil {
		e.Title = *title
	}
	if reason != nil {
		e.DeleteReason = *reason
	}
	if restorable != nil {
		e.Restorable = *restorable
	}
	return e
}

func (g Gateway) gameListTrash(ctx context.Context, q port.TrashQuery) (port.TrashPage, error) {
	if g.GameTrash == nil {
		return port.TrashPage{}, notWired("game", "listTrash")
	}
	p := paramsOf(q)
	rt := gameapi.CommonInternalCommonV1ResourceType(q.ResourceType)
	params := gameapi.ListTrashParams{ResourceType: &rt, TenantIds: p.tenants, Cursor: p.cursor, Limit: p.limit}
	res, err := call(ctx, g.gameOp("listTrash", modcall.Read), func(ctx context.Context) (gameapi.ListTrashResponseObject, error) {
		return g.GameTrash.ListTrash(ctx, gameapi.ListTrashRequestObject{Params: params})
	})
	if err != nil {
		return port.TrashPage{}, err
	}
	page, ok := res.(gameapi.ListTrash200JSONResponse)
	if !ok {
		return port.TrashPage{}, unexpected("game", "listTrash", res)
	}
	out := port.TrashPage{Data: make([]port.TrashEntry, 0, len(page.Data)), HasMore: page.Page.HasMore, NextCursor: page.Page.NextCursor}
	for _, it := range page.Data {
		out.Data = append(out.Data, trashEntry(string(it.ResourceType), it.ResourceId, it.TenantId, it.DeletedAt, it.DeletedBy, it.Title,
			it.DeleteReason, it.Restorable))
	}
	return out, nil
}

func (g Gateway) gameRestoreTrash(ctx context.Context, resourceType string, id uuid.UUID, reason string) error {
	if g.GameTrash == nil {
		return notWired("game", "restoreTrashItem")
	}
	res, err := call(ctx, g.gameOp("restoreTrashItem", modcall.Write), func(ctx context.Context) (gameapi.RestoreTrashItemResponseObject, error) {
		return g.GameTrash.RestoreTrashItem(ctx, gameapi.RestoreTrashItemRequestObject{ResourceType: gameapi.CommonInternalCommonV1ResourceTypePath(resourceType),
			ResourceId: id, Body: &gameapi.RestoreTrashItemJSONRequestBody{Reason: reason}})
	})
	if err != nil {
		return err
	}
	if _, ok := res.(gameapi.RestoreTrashItem200JSONResponse); !ok {
		return unexpected("game", "restoreTrashItem", res)
	}
	return nil
}

func (g Gateway) competitionListTrash(ctx context.Context, q port.TrashQuery) (port.TrashPage, error) {
	if g.CompetitionTrash == nil {
		return port.TrashPage{}, notWired("competition", "listTrash")
	}
	p := paramsOf(q)
	rt := competitionapi.CommonInternalCommonV1ResourceType(q.ResourceType)
	params := competitionapi.ListTrashParams{ResourceType: &rt, TenantIds: p.tenants, Cursor: p.cursor, Limit: p.limit}
	res, err := call(ctx, g.competitionOp("listTrash", modcall.Read), func(ctx context.Context) (competitionapi.ListTrashResponseObject, error) {
		return g.CompetitionTrash.ListTrash(ctx, competitionapi.ListTrashRequestObject{Params: params})
	})
	if err != nil {
		return port.TrashPage{}, err
	}
	page, ok := res.(competitionapi.ListTrash200JSONResponse)
	if !ok {
		return port.TrashPage{}, unexpected("competition", "listTrash", res)
	}
	out := port.TrashPage{Data: make([]port.TrashEntry, 0, len(page.Data)), HasMore: page.Page.HasMore, NextCursor: page.Page.NextCursor}
	for _, it := range page.Data {
		out.Data = append(out.Data, trashEntry(string(it.ResourceType), it.ResourceId, it.TenantId, it.DeletedAt, it.DeletedBy, it.Title,
			it.DeleteReason, it.Restorable))
	}
	return out, nil
}

func (g Gateway) competitionRestoreTrash(ctx context.Context, resourceType string, id uuid.UUID, reason string) error {
	if g.CompetitionTrash == nil {
		return notWired("competition", "restoreTrashItem")
	}
	res, err := call(ctx, g.competitionOp("restoreTrashItem", modcall.Write), func(ctx context.Context) (competitionapi.RestoreTrashItemResponseObject, error) {
		return g.CompetitionTrash.RestoreTrashItem(ctx, competitionapi.RestoreTrashItemRequestObject{
			ResourceType: competitionapi.CommonInternalCommonV1ResourceTypePath(resourceType), ResourceId: id,
			Body: &competitionapi.RestoreTrashItemJSONRequestBody{Reason: reason}})
	})
	if err != nil {
		return err
	}
	if _, ok := res.(competitionapi.RestoreTrashItem200JSONResponse); !ok {
		return unexpected("competition", "restoreTrashItem", res)
	}
	return nil
}

func (g Gateway) chatListTrash(ctx context.Context, q port.TrashQuery) (port.TrashPage, error) {
	if g.ChatTrash == nil {
		return port.TrashPage{}, notWired("chat", "listTrash")
	}
	p := paramsOf(q)
	rt := chatapi.CommonInternalCommonV1ResourceType(q.ResourceType)
	params := chatapi.ListTrashParams{ResourceType: &rt, TenantIds: p.tenants, Cursor: p.cursor, Limit: p.limit}
	res, err := call(ctx, g.chatOp("listTrash", modcall.Read), func(ctx context.Context) (chatapi.ListTrashResponseObject, error) {
		return g.ChatTrash.ListTrash(ctx, chatapi.ListTrashRequestObject{Params: params})
	})
	if err != nil {
		return port.TrashPage{}, err
	}
	page, ok := res.(chatapi.ListTrash200JSONResponse)
	if !ok {
		return port.TrashPage{}, unexpected("chat", "listTrash", res)
	}
	out := port.TrashPage{Data: make([]port.TrashEntry, 0, len(page.Data)), HasMore: page.Page.HasMore, NextCursor: page.Page.NextCursor}
	for _, it := range page.Data {
		out.Data = append(out.Data, trashEntry(string(it.ResourceType), it.ResourceId, it.TenantId, it.DeletedAt, it.DeletedBy, it.Title,
			it.DeleteReason, it.Restorable))
	}
	return out, nil
}

func (g Gateway) chatRestoreTrash(ctx context.Context, resourceType string, id uuid.UUID, reason string) error {
	if g.ChatTrash == nil {
		return notWired("chat", "restoreTrashItem")
	}
	res, err := call(ctx, g.chatOp("restoreTrashItem", modcall.Write), func(ctx context.Context) (chatapi.RestoreTrashItemResponseObject, error) {
		return g.ChatTrash.RestoreTrashItem(ctx, chatapi.RestoreTrashItemRequestObject{ResourceType: chatapi.CommonInternalCommonV1ResourceTypePath(resourceType),
			ResourceId: id, Body: &chatapi.RestoreTrashItemJSONRequestBody{Reason: reason}})
	})
	if err != nil {
		return err
	}
	if _, ok := res.(chatapi.RestoreTrashItem200JSONResponse); !ok {
		return unexpected("chat", "restoreTrashItem", res)
	}
	return nil
}
