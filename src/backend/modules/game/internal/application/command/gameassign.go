package command

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the assignments (ADR-0029). The domain events tenant_data.assigned/unassigned are published by module
// identity after this facade returns (owner F045); module game never publishes them (F078-AC-21).
const (
	ActionTenantDataAssigned   = "tenant_data.assigned"
	ActionTenantDataUnassigned = "tenant_data.unassigned"
)

// TenantRebuilder rebuilds the content manifest of tenants (command.GameManifest.RebuildTenants).
type TenantRebuilder interface {
	RebuildTenants(ctx context.Context, tenants []uuid.UUID, cause uuid.UUID, itemType string) error
}

// GameAssignments are the facade use cases applyTenantAssignments, revokeTenantAssignments and listResourceAssignments
// of module game (F078-AC-21, ADR-0029; caller module identity, x-allowed-callers [identity]): a published game resource
// (quest, mini-game config, item, shop item, event template) is assigned to tenants or revoked (soft delete), and the
// manifest of each changed tenant is rebuilt in the same transaction.
type GameAssignments struct {
	UoW       write.UnitOfWork
	Store     write.TenantAssignmentStore
	Manifests TenantRebuilder
	Audit     write.Auditor
	Cursors   *page.Codec
	Now       func() time.Time
}

// AssignmentView is TenantAssignment of the internal contract.
type AssignmentView struct {
	AssignmentID uuid.UUID  `json:"assignment_id"`
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	AssignedBy   uuid.UUID  `json:"assigned_by"`
	AssignedAt   time.Time  `json:"assigned_at"`
	Status       string     `json:"status"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

func assignmentView(a write.Assignment) AssignmentView {
	v := AssignmentView{AssignmentID: a.ID, ResourceType: a.ResourceType, ResourceID: a.ResourceID, TenantID: a.TenantID, AssignedBy: a.AssignedBy,
		AssignedAt: a.AssignedAt.UTC(), Status: "active", RevokedAt: a.RevokedAt}
	if a.RevokedAt != nil {
		v.Status = "revoked"
	}
	return v
}

func (h GameAssignments) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// prepare authorizes the original principal (platform staff with platform.tenant_data.assign), checks the resource type
// and the tenants and returns the CrossTenantScope of exactly the requested tenants.
func (h GameAssignments) prepare(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID) (authz.Principal,
	gameconfig.Type, []uuid.UUID, tenancy.Scope, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, gameconfig.Type{}, nil, nil, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTenantDataAssign, authz.Resource{}); err != nil {
		return p, gameconfig.Type{}, nil, nil, err
	}
	if !p.IsPlatform() {
		return p, gameconfig.Type{}, nil, nil, problem.New(problem.PermissionDenied, "")
	}
	t, ok := gameconfig.ByAssignable(resourceType)
	if !ok {
		return p, t, nil, nil, problem.Invalid("resource_type", problem.SubInvalid, "not a game resource that can be assigned")
	}
	if resourceID == uuid.Nil {
		return p, t, nil, nil, problem.Invalid("resource_id", problem.SubRequired, "is required")
	}
	uniq := make([]uuid.UUID, 0, len(tenants))
	seen := map[uuid.UUID]bool{}
	for _, id := range tenants {
		if id != uuid.Nil && !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}
	if len(uniq) == 0 || len(uniq) > 500 {
		return p, t, nil, nil, problem.Invalid("tenant_ids", problem.SubInvalid, "1 to 500 tenants")
	}
	sc, err := authz.CrossTenantScope(p, uniq, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return p, t, nil, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return p, t, uniq, sc, nil
}

// Apply is applyTenantAssignments: a resource that is not published is 409 CONTENT_NOT_PUBLISHED; an inactive or
// unknown tenant fails the request (422, each element listed); missing active assignments are created, the manifests
// of the newly assigned tenants rebuilt. Idempotent by Idempotency-Key.
func (h GameAssignments) Apply(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID) (out []AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ApplyTenantAssignments")
	defer func() { done(err) }()
	p, t, tenants, sc, err := h.prepare(ctx, resourceType, resourceID, tenants)
	if err != nil {
		return nil, err
	}
	ps, err := authz.PlatformScope(p)
	if err != nil {
		return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		exists, published, err := h.Store.Published(tenancy.WithScope(ctx, ps), t.ItemType, resourceID)
		if err != nil {
			return err
		}
		if !exists {
			return problem.Invalid("resource_id", problem.SubInvalid, "is not a live platform resource of module game")
		}
		if !published {
			return problem.New(problem.ContentNotPublished, "only published game content can be assigned to a tenant")
		}
		active, err := h.Store.ActiveTenants(ctx, tenants)
		if err != nil {
			return err
		}
		pe := &problem.Error{Code: problem.ValidationFailed, Detail: "request validation failed"}
		for i, id := range tenants {
			if !containsUUID(active, id) {
				pe.AddField(fmt.Sprintf("tenant_ids[%d]", i), problem.SubInvalid, "is not an active tenant")
			}
		}
		if len(pe.Fields) > 0 {
			return pe
		}
		existing, err := h.Store.Active(ctx, resourceType, resourceID, tenants)
		if err != nil {
			return err
		}
		by := map[uuid.UUID]write.Assignment{}
		for _, a := range existing {
			by[a.TenantID] = a
		}
		var created []uuid.UUID
		for _, id := range tenants {
			if _, ok := by[id]; ok {
				continue
			}
			a := write.Assignment{ID: ids.New(), ResourceType: resourceType, ResourceID: resourceID, TenantID: id, AssignedBy: p.UserID, AssignedAt: now}
			if err := h.Store.Create(ctx, a); err != nil {
				return err
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: ActionTenantDataAssigned, TargetType: resourceType,
				TargetID: resourceID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"assignment_id": a.ID.String(),
					"tenant_id": id.String()}}); err != nil {
				return err
			}
			by[id] = a
			created = append(created, id)
		}
		if len(created) > 0 && h.Manifests != nil {
			if err := h.Manifests.RebuildTenants(ctx, created, ids.New(), t.ItemType); err != nil {
				return err
			}
		}
		out = make([]AssignmentView, 0, len(tenants))
		for _, id := range tenants {
			out = append(out, assignmentView(by[id]))
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// Revoke is revokeTenantAssignments: the active assignments of the requested tenants are soft-deleted (tenants without
// one are skipped) and their manifests rebuilt; plays pinned to a version are untouched.
func (h GameAssignments) Revoke(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID, reason string) (out []AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RevokeTenantAssignments")
	defer func() { done(err) }()
	p, t, tenants, sc, err := h.prepare(ctx, resourceType, resourceID, tenants)
	if err != nil {
		return nil, err
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(reason)); n < 3 || utf8.RuneCountInString(reason) > 1000 {
		return nil, problem.Invalid("reason", problem.SubInvalid, "must be 3–1000 characters")
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		existing, err := h.Store.Active(ctx, resourceType, resourceID, tenants)
		if err != nil {
			return err
		}
		out = make([]AssignmentView, 0, len(existing))
		var revoked []uuid.UUID
		for _, a := range existing {
			r, err := h.Store.Revoke(ctx, a, p.UserID, reason, now)
			if err != nil {
				return err
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: ActionTenantDataUnassigned, TargetType: resourceType,
				TargetID: resourceID.String(), Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"assignment_id": a.ID.String(),
					"tenant_id": a.TenantID.String()}}); err != nil {
				return err
			}
			out = append(out, assignmentView(r))
			revoked = append(revoked, a.TenantID)
		}
		if len(revoked) > 0 && h.Manifests != nil {
			if err := h.Manifests.RebuildTenants(ctx, revoked, ids.New(), t.ItemType); err != nil {
				return err
			}
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// AssignmentPage is TenantAssignmentList.
type AssignmentPage struct {
	Data []AssignmentView
	Page page.Info
}

// List is listResourceAssignments: the active assignments of a resource, keyset by tenant id.
func (h GameAssignments) List(ctx context.Context, resourceType string, resourceID uuid.UUID, cursor string, limit int) (out AssignmentPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListResourceAssignments")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTenantDataAssign, authz.Resource{}); err != nil {
		return out, err
	}
	if _, ok := gameconfig.ByAssignable(resourceType); !ok {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "not a game resource that can be assigned")
	}
	ps, err := authz.PlatformScope(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	n := page.Limit(limit)
	cur, err := h.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	err = h.UoW.Do(tenancy.WithScope(ctx, ps), func(ctx context.Context) error {
		var after *uuid.UUID
		if cur != nil {
			id := cur.ID
			after = &id
		}
		rows, err := h.Store.Page(ctx, resourceType, resourceID, after, n+1)
		if err != nil {
			return err
		}
		out = AssignmentPage{Data: []AssignmentView{}, Page: page.Info{Limit: n}}
		if len(rows) > n {
			rows = rows[:n]
			next := h.Cursors.Encode(page.Cursor{Key: "t", ID: rows[n-1].TenantID})
			out.Page.HasMore, out.Page.NextCursor = true, &next
		}
		for _, a := range rows {
			out.Data = append(out.Data, assignmentView(a))
		}
		return nil
	})
	return out, err
}

func containsUUID(list []uuid.UUID, id uuid.UUID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}
