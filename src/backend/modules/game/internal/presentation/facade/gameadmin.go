package facade

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Assignments are the facade use cases of the tenant assignments of game resources (command.GameAssignments).
type Assignments interface {
	Apply(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID) ([]command.AssignmentView, error)
	Revoke(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID, reason string) ([]command.AssignmentView, error)
	List(ctx context.Context, resourceType string, resourceID uuid.UUID, cursor string, limit int) (command.AssignmentPage, error)
}

// Catalog is listAssignedGameActivities (query.GameAdmin.Assigned) with the CDN URL of a render thumbnail.
type Catalog interface {
	Assigned(ctx context.Context, tenant uuid.UUID, cursor string, limit int) (query.AssignedPage, error)
	ThumbnailURL(sha *string) string
}

// GameAdmin is the facade part of the Game Operations Console (F078): applyTenantAssignments, revokeTenantAssignments,
// listResourceAssignments (x-allowed-callers: identity) and listAssignedGameActivities (x-allowed-callers: learning).
type GameAdmin struct {
	Assignments Assignments
	Catalog     Catalog
}

func assignmentList(views []command.AssignmentView) api.CommonInternalCommonV1TenantAssignmentList {
	out := api.CommonInternalCommonV1TenantAssignmentList{Data: make([]api.CommonInternalCommonV1TenantAssignment, 0, len(views))}
	for _, v := range views {
		out.Data = append(out.Data, api.CommonInternalCommonV1TenantAssignment{AssignmentId: v.AssignmentID, AssignedAt: v.AssignedAt,
			AssignedBy: v.AssignedBy, ResourceId: v.ResourceID, ResourceType: api.CommonInternalCommonV1ResourceType(v.ResourceType),
			TenantId: v.TenantID, Status: api.CommonInternalCommonV1TenantAssignmentStatus(v.Status), RevokedAt: v.RevokedAt, Created: v.Created})
	}
	return out
}

// ApplyTenantAssignments implements gameapi.StrictServerInterface (x-allowed-callers: identity, F078-AC-21).
func (f GameAdmin) ApplyTenantAssignments(ctx context.Context, req api.ApplyTenantAssignmentsRequestObject) (api.ApplyTenantAssignmentsResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "assignment request required")
	}
	views, err := f.Assignments.Apply(ctx, string(req.Body.ResourceType), req.Body.ResourceId, req.Body.TenantIds)
	if err != nil {
		return nil, err
	}
	return api.ApplyTenantAssignments200JSONResponse(assignmentList(views)), nil
}

// RevokeTenantAssignments implements gameapi.StrictServerInterface (x-allowed-callers: identity, F078-AC-21).
func (f GameAdmin) RevokeTenantAssignments(ctx context.Context, req api.RevokeTenantAssignmentsRequestObject) (api.RevokeTenantAssignmentsResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "revoke request required")
	}
	views, err := f.Assignments.Revoke(ctx, string(req.Body.ResourceType), req.Body.ResourceId, req.Body.TenantIds, req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return api.RevokeTenantAssignments200JSONResponse(assignmentList(views)), nil
}

// ListResourceAssignments implements gameapi.StrictServerInterface (x-allowed-callers: identity).
func (f GameAdmin) ListResourceAssignments(ctx context.Context, req api.ListResourceAssignmentsRequestObject) (api.ListResourceAssignmentsResponseObject, error) {
	cursor, limit := "", 0
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	page, err := f.Assignments.List(ctx, string(req.Params.ResourceType), req.Params.ResourceId, cursor, limit)
	if err != nil {
		return nil, err
	}
	out := assignmentList(page.Data)
	out.Page = api.CommonInternalCommonV1PageInfo{HasMore: page.Page.HasMore, NextCursor: page.Page.NextCursor}
	return api.ListResourceAssignments200JSONResponse(out), nil
}

// ListAssignedGameActivities implements gameapi.StrictServerInterface (x-allowed-callers: learning, F078-AC-23): the
// published quests and mini-game configs assigned to the tenant with their catalog metadata.
func (f GameAdmin) ListAssignedGameActivities(ctx context.Context, req api.ListAssignedGameActivitiesRequestObject) (api.ListAssignedGameActivitiesResponseObject, error) {
	cursor, limit := "", 0
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	page, err := f.Catalog.Assigned(ctx, req.Params.TenantId, cursor, limit)
	if err != nil {
		return nil, err
	}
	out := api.ListAssignedGameActivities200JSONResponse{Data: make([]api.GameActivityCatalogEntry, 0, len(page.Data)),
		Page: api.CommonInternalCommonV1PageInfo{HasMore: page.HasMore, NextCursor: page.NextCursor}}
	for _, a := range page.Data {
		var doc map[string]any
		_ = json.Unmarshal(a.Payload, &doc)
		c := command.CatalogBlock(a.ItemType, doc)
		e := api.GameActivityCatalogEntry{ContentType: api.GameActivityCatalogEntryContentType(a.ItemType), ContentId: a.ConfigID,
			ContentVersionId: a.VersionID, Version: a.VersionNo, PublishedAt: a.PublishedAt, GradeLevels: []int{}}
		if t, ok := c["title"].(map[string]any); ok {
			e.Title.Vi, _ = t["vi"].(string)
			if en, ok := t["en"].(string); ok {
				e.Title.En = &en
			}
		}
		if g, ok := c["grade_levels"].([]int); ok {
			e.GradeLevels = g
		}
		if s, ok := c["subject_id"].(string); ok {
			if id, err := uuid.Parse(s); err == nil {
				e.SubjectId = &id
			}
		}
		if os, ok := c["objective_ids"].([]string); ok {
			ids := make([]uuid.UUID, 0, len(os))
			for _, o := range os {
				if id, err := uuid.Parse(o); err == nil {
					ids = append(ids, id)
				}
			}
			e.ObjectiveIds = &ids
		}
		if m, ok := c["estimated_minutes"].(int); ok {
			e.EstimatedMinutes = &m
		}
		if u := f.Catalog.ThumbnailURL(a.ThumbnailSHA); u != "" {
			e.ThumbnailUrl = &u
		}
		out.Data = append(out.Data, e)
	}
	return out, nil
}
