package command

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the trash use cases (ADR-0028, F020-AC-21, F020-AC-25).
const (
	ActionContentDeleted  = "content.deleted"
	ActionContentRestored = "content.restored"
)

// TrashDeps are the collaborators of the soft-delete and restore commands.
type TrashDeps struct {
	UoW       write.UnitOfWork
	Resources *port.Registry
	Events    write.EventPublisher
	Audit     write.Auditor
	Clock     func() time.Time
}

// ResourceEventPayload is the payload of <resource_type>.deleted / .restored
// (specs/events/payloads/learning/resource.{deleted,restored}.v1.schema.json).
type ResourceEventPayload struct {
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	TenantID     *uuid.UUID `json:"tenant_id"`
	DeletedBy    *uuid.UUID `json:"deleted_by,omitempty"`
	RestoredBy   *uuid.UUID `json:"restored_by,omitempty"`
	Reason       string     `json:"reason,omitempty"`
}

func (d TrashDeps) handler(s string) (contentmodel.ResourceType, port.ResourceHandler, error) {
	rt, err := resourceType(s)
	if err != nil {
		return "", nil, err
	}
	h, ok := d.Resources.Get(rt)
	if !ok {
		return "", nil, problem.Invalid("resource_type", problem.SubInvalid, "resource type has no trash yet")
	}
	return rt, h, nil
}

func reasonOK(r string, minLen int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(r))
	return n >= minLen && utf8.RuneCountInString(r) <= 1000
}

// publishResource writes <resource_type>.<verb> for the owner tenant (platform event for platform content).
func (d TrashDeps) publishResource(ctx context.Context, rt contentmodel.ResourceType, verb string, p ResourceEventPayload) error {
	e := write.Event{Type: string(rt) + "." + verb, SubjectType: string(rt), SubjectID: p.ResourceID.String(), Payload: p}
	if p.TenantID == nil {
		e.Platform = true
	} else {
		t := *p.TenantID
		e.TenantID = &t
	}
	return d.Events.Publish(ctx, e)
}

// recordFor audits e attributed to the owner tenant of the content (platform audit for platform content).
func (d TrashDeps) recordFor(ctx context.Context, tenant *uuid.UUID, e audit.Event) error {
	if tenant == nil {
		return d.Audit.Record(ctx, e)
	}
	return auditIn(ctx, d.Audit, *tenant, e)
}

// SoftDeleteInput is the request of SoftDeleteResource.
type SoftDeleteInput struct {
	ResourceType string
	ID           uuid.UUID
	Reason       string // ≤ 1000; delete_reason keeps the first 64 characters, audit and event the whole reason
}

// SoftDeleteResourceHandler soft-deletes one learning item (F020-AC-21): used by the delete endpoints of each
// content type (F021+) with the scope chosen by scope.ForWrite.
type SoftDeleteResourceHandler interface {
	Handle(ctx context.Context, in SoftDeleteInput) error
}

// SoftDeleteResource implements SoftDeleteResourceHandler.
type SoftDeleteResource struct{ TrashDeps }

var _ SoftDeleteResourceHandler = SoftDeleteResource{}

// Handle sets deleted_at/deleted_by/delete_reason through the entx hook, records audit content.deleted and
// writes <resource_type>.deleted in the same transaction.
func (h SoftDeleteResource) Handle(ctx context.Context, in SoftDeleteInput) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SoftDeleteResource")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	rt, handler, err := h.handler(in.ResourceType)
	if err != nil {
		return err
	}
	if err := authz.Require(ctx, p, handler.WritePermission(), authz.Resource{}); err != nil {
		return err
	}
	if utf8.RuneCountInString(in.Reason) > 1000 {
		return problem.Invalid("reason", problem.SubMaxLength, "is longer than 1000 characters")
	}
	if _, ok := tenancy.ScopeFrom(ctx); !ok {
		return problem.New(problem.Internal, "soft-delete needs the scope of the request")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		owners, err := handler.Owners(ctx, []uuid.UUID{in.ID})
		if err != nil {
			return err
		}
		owner, ok := owners[in.ID]
		if !ok {
			return problem.New(problem.NotFound, "")
		}
		if err := RequireWritable(p, owner); err != nil {
			return err
		}
		if err := authz.Require(ctx, p, handler.WritePermission(), authz.Resource{TenantID: owner.TenantID}); err != nil {
			return err
		}
		if _, err := handler.SoftDelete(ctx, in.ID, in.Reason); err != nil {
			if errors.Is(err, contentmodel.ErrItemPublished) {
				return problem.New(problem.Conflict, "an item with a current release cannot be deleted; archive or unpublish it first")
			}
			return err
		}
		if err := h.recordFor(ctx, owner.TenantID, audit.Event{Action: ActionContentDeleted, TargetType: string(rt), TargetID: in.ID.String(),
			Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"resource_type": string(rt)}}); err != nil {
			return err
		}
		by := p.UserID
		return h.publishResource(ctx, rt, "deleted", ResourceEventPayload{ResourceType: string(rt), ResourceID: in.ID, TenantID: owner.TenantID,
			DeletedBy: &by, Reason: in.Reason})
	})
}

// RestoreInput is the request of restoreTrashItem.
type RestoreInput struct {
	ResourceType string
	ID           uuid.UUID
	Reason       string // 3–1000
}

// RestoreView is the result of a restore (stored for Idempotency-Key replay).
type RestoreView struct {
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	RestoredAt   time.Time `json:"restored_at"`
}

// RestoreResourceHandler is the use case behind facade restoreTrashItem (F020-AC-25, AC-26).
type RestoreResourceHandler interface {
	Handle(ctx context.Context, in RestoreInput) (RestoreView, error)
}

// RestoreResource implements RestoreResourceHandler: System Admin (platform.trash.restore) restores any item;
// a tenant user with tenant.trash.restore [Proposed] only items of its session tenant.
type RestoreResource struct{ TrashDeps }

var _ RestoreResourceHandler = RestoreResource{}

// Handle clears deleted_* under tenancy.Bypass(trash) (audited cross_tenant.accessed, mode write), records
// audit content.restored and writes <resource_type>.restored in the same transaction; a live item holding
// the unique key is 409 CONFLICT and nothing changes.
func (h RestoreResource) Handle(ctx context.Context, in RestoreInput) (out RestoreView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RestoreResource")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	perm := authz.PermPlatformTrashRestore
	if p.TenantID != nil {
		perm = authz.PermTenantTrashRestore
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return out, err
	}
	if p.TenantID == nil && !p.IsPlatform() {
		return out, problem.New(problem.PermissionDenied, "")
	}
	rt, handler, err := h.handler(in.ResourceType)
	if err != nil {
		return out, err
	}
	if !reasonOK(in.Reason, 3) {
		return out, problem.Invalid("reason", problem.SubInvalid, "must be 3–1000 characters")
	}
	var opts []tenancy.BypassOption
	if p.TenantID != nil {
		opts = append(opts, tenancy.WithTenantIDs(*p.TenantID))
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonTrash, append(opts, tenancy.WithMode(tenancy.ModeWrite))...)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		if p.TenantID != nil {
			// Under the trash bypass Owners sees deleted rows of every tenant: another tenant's item is 404
			// before anything else (no conflict or state leaks across tenants).
			owners, err := handler.Owners(ctx, []uuid.UUID{in.ID})
			if err != nil {
				return err
			}
			if o, ok := owners[in.ID]; !ok || o.TenantID == nil || *o.TenantID != *p.TenantID {
				return problem.New(problem.NotFound, "")
			}
		}
		e, err := handler.Restore(ctx, in.ID)
		switch {
		case errors.Is(err, trash.ErrNotDeleted):
			return problem.New(problem.NotFound, "the item is not in the trash")
		case errors.Is(err, trash.ErrConflict):
			return problem.New(problem.Conflict, "a live item already uses the unique key of this one")
		case err != nil:
			return err
		}
		if p.TenantID != nil && (e.TenantID == nil || *e.TenantID != *p.TenantID) {
			return problem.New(problem.NotFound, "") // rolls the restore back: another tenant's item does not exist here
		}
		if err := h.recordFor(ctx, e.TenantID, audit.Event{Action: ActionContentRestored, TargetType: string(rt), TargetID: in.ID.String(),
			Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"resource_type": string(rt)}}); err != nil {
			return err
		}
		by := p.UserID
		if err := h.publishResource(ctx, rt, "restored", ResourceEventPayload{ResourceType: string(rt), ResourceID: in.ID, TenantID: e.TenantID,
			RestoredBy: &by, Reason: in.Reason}); err != nil {
			return err
		}
		out = RestoreView{ResourceType: string(rt), ResourceID: in.ID, RestoredAt: now(h.Clock)}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}
