// Package facade implements the in-process facade of module identity (identityapi.StrictServerInterface,
// generated from specs/openapi/modules/identity.internal.v1.yaml): other modules call it through
// pkg/modcall, never over HTTP (ADR-0074, AGENTS R29). F010 implements listProjectionSnapshot; the
// Principal and tenant operations come with F012.
package facade

import (
	"context"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
)

// Snapshots is the facade part implemented by F010.
type Snapshots interface {
	ListProjectionSnapshot(ctx context.Context, req identityapi.ListProjectionSnapshotRequestObject) (identityapi.ListProjectionSnapshotResponseObject, error)
}

// Facade serves the identity facade operations of F010.
type Facade struct {
	Snapshot query.ProjectionSnapshot
}

var _ Snapshots = Facade{}

// ListProjectionSnapshot implements listProjectionSnapshot (F010-AC-27).
func (f Facade) ListProjectionSnapshot(ctx context.Context, req identityapi.ListProjectionSnapshotRequestObject) (identityapi.ListProjectionSnapshotResponseObject, error) {
	in := query.SnapshotInput{EntityType: string(req.EntityType)}
	if req.Params.TenantId != nil {
		t := uuid.UUID(*req.Params.TenantId)
		in.TenantID = &t
	}
	if req.Params.Cursor != nil {
		in.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		in.Limit = *req.Params.Limit
	}
	p, err := f.Snapshot.Handle(ctx, in)
	if err != nil {
		return nil, err
	}
	out := identityapi.ListProjectionSnapshot200JSONResponse{Items: make([]identityapi.ProjectionSnapshotItem, 0, len(p.Items)),
		Page: identityapi.CommonInternalCommonV1PageInfo{HasMore: p.HasMore, NextCursor: p.NextCursor}}
	for _, it := range p.Items {
		out.Items = append(out.Items, identityapi.ProjectionSnapshotItem{EntityType: identityapi.ProjectionSnapshotItemEntityType(it.EntityType),
			EntityId: openapi_types.UUID(it.EntityID), TenantId: openapi_types.UUID(it.TenantID), SourceVersion: int(it.SourceVersion),
			Deleted: it.Deleted, Data: it.Data})
	}
	return out, nil
}
