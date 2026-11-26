package command

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ResourceStickerPack is the only assignable resource of module chat (GIFs are a shared library — Clarifications F082
// 2026-11-26).
const ResourceStickerPack = "sticker_pack"

// Audit actions of the assignments (the domain events tenant_data.assigned/unassigned are published by module identity
// after the facade returns, owner F045).
const (
	AuditTenantDataAssigned   = "tenant_data.assigned"
	AuditTenantDataUnassigned = "tenant_data.unassigned"
)

// ChatAssignments are applyTenantAssignments, revokeTenantAssignments and listResourceAssignments of module chat (F082
// T040, ADR-0029; caller module identity, x-allowed-callers [identity]): a published sticker pack is assigned to tenants
// (students of an assigned tenant see it) or revoked (soft delete). Messages already sent keep their sticker.
type ChatAssignments struct {
	UoW     write.UnitOfWork
	Store   write.AssignmentStore
	Audit   write.Auditor
	Cursors *page.Codec
	Now     func() time.Time
}

// AssignmentView is TenantAssignment of the internal contract.
type AssignmentView struct {
	AssignmentID uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	TenantID     uuid.UUID
	AssignedBy   uuid.UUID
	AssignedAt   time.Time
	Status       string
	RevokedAt    *time.Time
}

func assignmentView(a write.Assignment) AssignmentView {
	v := AssignmentView{AssignmentID: a.ID, ResourceType: a.ResourceType, ResourceID: a.ResourceID, TenantID: a.TenantID, AssignedBy: a.AssignedBy,
		AssignedAt: a.AssignedAt.UTC(), Status: "active", RevokedAt: a.RevokedAt}
	if a.RevokedAt != nil {
		v.Status = "revoked"
	}
	return v
}

func (h ChatAssignments) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// prepare authorizes the original principal (platform staff with platform.tenant_data.assign), checks the resource type
// and the tenants and returns the CrossTenantScope of exactly the requested tenants.
func (h ChatAssignments) prepare(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID) (authz.Principal, []uuid.UUID, tenancy.Scope, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, nil, nil, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTenantDataAssign, authz.Resource{}); err != nil {
		return p, nil, nil, err
	}
	if !p.IsPlatform() {
		return p, nil, nil, problem.New(problem.PermissionDenied, "")
	}
	if resourceType != ResourceStickerPack {
		return p, nil, nil, problem.Invalid("resource_type", problem.SubInvalid, "not a chat resource that can be assigned")
	}
	if id == uuid.Nil {
		return p, nil, nil, problem.Invalid("resource_id", problem.SubRequired, "is required")
	}
	uniq := make([]uuid.UUID, 0, len(tenants))
	seen := map[uuid.UUID]bool{}
	for _, t := range tenants {
		if t != uuid.Nil && !seen[t] {
			seen[t] = true
			uniq = append(uniq, t)
		}
	}
	if len(uniq) == 0 || len(uniq) > 500 {
		return p, nil, nil, problem.Invalid("tenant_ids", problem.SubInvalid, "1 to 500 tenants")
	}
	sc, err := authz.CrossTenantScope(p, uniq, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return p, nil, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return p, uniq, sc, nil
}

// Apply is applyTenantAssignments: a pack that is not published is 409 CONTENT_NOT_PUBLISHED; an inactive or unknown
// tenant fails the request (422, each element listed); missing active assignments are created. Idempotent.
func (h ChatAssignments) Apply(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID) (out []AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ApplyTenantAssignments")
	defer func() { done(err) }()
	p, tenants, sc, err := h.prepare(ctx, resourceType, id, tenants)
	if err != nil {
		return nil, err
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		exists, published, err := h.Store.PackPublished(ctx, id)
		if err != nil {
			return err
		}
		if !exists {
			return problem.Invalid("resource_id", problem.SubInvalid, "is not a live sticker pack")
		}
		if !published {
			return problem.New(problem.ContentNotPublished, "only a published sticker pack can be assigned to a tenant")
		}
		active, err := h.Store.ActiveTenants(ctx, tenants)
		if err != nil {
			return err
		}
		isActive := map[uuid.UUID]bool{}
		for _, a := range active {
			isActive[a] = true
		}
		pe := &problem.Error{Code: problem.ValidationFailed, Detail: "request validation failed"}
		for i, t := range tenants {
			if !isActive[t] {
				pe.AddField(fmt.Sprintf("tenant_ids[%d]", i), problem.SubInvalid, "is not an active tenant")
			}
		}
		if len(pe.Fields) > 0 {
			return pe
		}
		existing, err := h.Store.Active(ctx, resourceType, id, tenants)
		if err != nil {
			return err
		}
		by := map[uuid.UUID]write.Assignment{}
		for _, a := range existing {
			by[a.TenantID] = a
		}
		for _, t := range tenants {
			if _, ok := by[t]; ok {
				continue
			}
			a := write.Assignment{ID: ids.New(), ResourceType: resourceType, ResourceID: id, TenantID: t, AssignedBy: p.UserID, AssignedAt: now}
			if err := h.Store.Create(ctx, a); err != nil {
				return err
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: AuditTenantDataAssigned, TargetType: resourceType, TargetID: id.String(),
				Result: audit.ResultSuccess, Metadata: map[string]any{"assignment_id": a.ID.String(), "tenant_id": t.String()}}); err != nil {
				return err
			}
			by[t] = a
		}
		out = make([]AssignmentView, 0, len(tenants))
		for _, t := range tenants {
			out = append(out, assignmentView(by[t]))
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// Revoke is revokeTenantAssignments: the active assignments of the requested tenants are soft-deleted (tenants without
// one are skipped); old messages keep showing the sticker (assets are immutable).
func (h ChatAssignments) Revoke(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID, reason string) (out []AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RevokeTenantAssignments")
	defer func() { done(err) }()
	p, tenants, sc, err := h.prepare(ctx, resourceType, id, tenants)
	if err != nil {
		return nil, err
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(reason)); n < 3 || utf8.RuneCountInString(reason) > 1000 {
		return nil, problem.Invalid("reason", problem.SubInvalid, "must be 3–1000 characters")
	}
	now := h.now()
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		existing, err := h.Store.Active(ctx, resourceType, id, tenants)
		if err != nil {
			return err
		}
		out = make([]AssignmentView, 0, len(existing))
		for _, a := range existing {
			r, err := h.Store.Revoke(ctx, a, p.UserID, now)
			if err != nil {
				return err
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: AuditTenantDataUnassigned, TargetType: resourceType, TargetID: id.String(),
				Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"assignment_id": a.ID.String(), "tenant_id": a.TenantID.String()}}); err != nil {
				return err
			}
			out = append(out, assignmentView(r))
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// AssignmentPage is TenantAssignmentList.
type AssignmentPage struct {
	Data       []AssignmentView
	HasMore    bool
	NextCursor *string
}

// List is listResourceAssignments: the active assignments of a pack, keyset by tenant id in the query.
func (h ChatAssignments) List(ctx context.Context, resourceType string, id uuid.UUID, cursor string, limit int) (out AssignmentPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListResourceAssignments")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTenantDataAssign, authz.Resource{}); err != nil {
		return out, err
	}
	if resourceType != ResourceStickerPack {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "not a chat resource that can be assigned")
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
			a := cur.ID
			after = &a
		}
		rows, err := h.Store.Page(ctx, resourceType, id, after, n+1)
		if err != nil {
			return err
		}
		out = AssignmentPage{Data: []AssignmentView{}}
		if len(rows) > n {
			rows = rows[:n]
			next := h.Cursors.Encode(page.Cursor{Key: "t", ID: rows[n-1].TenantID})
			out.HasMore, out.NextCursor = true, &next
		}
		for _, a := range rows {
			out.Data = append(out.Data, assignmentView(a))
		}
		return nil
	})
	return out, err
}
