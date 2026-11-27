package facade

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// facadeMethod is the method column of the idempotency records written by the facade.
const facadeMethod = "FACADE"

// TrashLister is listTrash (query.Trash).
type TrashLister interface {
	List(ctx context.Context, in query.TrashInput) (query.TrashPage, error)
}

// TrashRestorer is restoreTrashItem (command.RestoreTrash).
type TrashRestorer interface {
	Restore(ctx context.Context, in command.RestoreTrashInput) (command.RestoreTrashView, error)
}

// Trash is the facade part of the trash of the sticker packs (ADR-0028, x-allowed-callers: identity); nil use cases or
// idempotency store = 503 UNAVAILABLE.
type Trash struct {
	TrashList    TrashLister
	TrashRestore TrashRestorer
	Idempotency  *idempotency.Store
}

// ListTrash implements chatapi.StrictServerInterface (x-allowed-callers: identity).
func (f Trash) ListTrash(ctx context.Context, req api.ListTrashRequestObject) (api.ListTrashResponseObject, error) {
	if f.TrashList == nil {
		return nil, problem.New(problem.Unavailable, "the trash is not served by this binary").Retry()
	}
	in := query.TrashInput{}
	if req.Params.TenantIds != nil {
		in.TenantIDs = append([]uuid.UUID(nil), *req.Params.TenantIds...)
	}
	if req.Params.ResourceType != nil {
		in.ResourceType = string(*req.Params.ResourceType)
	}
	if req.Params.Cursor != nil {
		in.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		in.Limit = *req.Params.Limit
	}
	p, err := f.TrashList.List(ctx, in)
	if err != nil {
		return nil, err
	}
	out := api.ListTrash200JSONResponse{Data: make([]api.CommonInternalCommonV1TrashItem, 0, len(p.Data)),
		Page: api.CommonInternalCommonV1PageInfo{HasMore: p.HasMore, NextCursor: p.NextCursor}}
	for _, e := range p.Data {
		restorable, title := e.Restorable, e.Title
		out.Data = append(out.Data, api.CommonInternalCommonV1TrashItem{ResourceType: api.CommonInternalCommonV1ResourceType(e.ResourceType),
			ResourceId: e.ResourceID, DeletedAt: e.DeletedAt, DeletedBy: e.DeletedBy, Restorable: &restorable, Title: &title})
	}
	return out, nil
}

// RestoreTrashItem implements chatapi.StrictServerInterface (x-allowed-callers: identity, x-context-required: tenant_scope,
// idempotency_key): a call again with the same Idempotency-Key returns the stored result.
func (f Trash) RestoreTrashItem(ctx context.Context, req api.RestoreTrashItemRequestObject) (api.RestoreTrashItemResponseObject, error) {
	if f.TrashRestore == nil || f.Idempotency == nil {
		return nil, problem.New(problem.Unavailable, "the trash is not served by this binary").Retry()
	}
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "restore request required")
	}
	in := command.RestoreTrashInput{ResourceType: string(req.ResourceType), ID: req.ResourceId, Reason: req.Body.Reason}
	v, err := idempotent(ctx, f.Idempotency, "restoreTrashItem", in, func(ctx context.Context) (command.RestoreTrashView, error) {
		return f.TrashRestore.Restore(ctx, in)
	})
	if err != nil {
		return nil, err
	}
	return api.RestoreTrashItem200JSONResponse{ResourceType: api.CommonInternalCommonV1ResourceType(v.ResourceType), ResourceId: v.ResourceID,
		RestoredAt: v.RestoredAt}, nil
}

// idempotent replays the result of a facade write called again with the same Idempotency-Key of the caller (stored with
// the result in the transaction of the use case, idempotency.SetResult); another body with the key is 409
// IDEMPOTENCY_KEY_REUSED.
func idempotent[T any](ctx context.Context, store *idempotency.Store, op string, req any, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	raw, ok := modcall.IdempotencyKeyFrom(ctx)
	if !ok {
		return zero, problem.New(problem.IdempotencyKeyRequired, "")
	}
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return zero, problem.New(problem.Unauthenticated, "")
	}
	key, issued, err := idempotency.ParseKey(raw, store.Clock().Now(), store.TTL())
	if err != nil {
		return zero, err
	}
	k := idempotency.Key{UserID: p.UserID, Method: facadeMethod, Route: "chat." + op, Key: key, IssuedOn: issued}
	body, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}
	hash := idempotency.Hash(body)
	replay := func() (T, bool, error) {
		rec, err := store.Lookup(ctx, k)
		if err != nil || rec == nil {
			return zero, false, err
		}
		if rec.Hash != hash {
			return zero, false, problem.New(problem.IdempotencyKeyReused, "")
		}
		var v T
		if err := json.Unmarshal(rec.Body, &v); err != nil {
			return zero, false, err
		}
		return v, true, nil
	}
	if v, found, err := replay(); err != nil || found {
		return v, err
	}
	v, err := fn(idempotency.WithPending(ctx, store.NewPending(k, hash, 200)))
	if errors.Is(err, idempotency.ErrDuplicate) { // a concurrent call with the same key committed first
		if v, found, rerr := replay(); rerr == nil && found {
			return v, nil
		}
	}
	return v, err
}
