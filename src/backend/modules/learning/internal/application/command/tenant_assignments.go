package command

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/resourceassignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the assignment use cases (ADR-0029, plan F020 "Bảo mật").
const (
	ActionTenantDataAssigned   = "tenant_data.assigned"
	ActionTenantDataUnassigned = "tenant_data.unassigned"
)

// AssignmentDeps are the collaborators of the assignment commands.
type AssignmentDeps struct {
	UoW         write.UnitOfWork
	Assignments write.AssignmentRepository
	Tenants     port.TenantDirectory
	Resources   *port.Registry
	Audit       write.Auditor
	Cache       write.CacheInvalidator // optional
	Metrics     AssignmentMetrics      // optional
	Clock       func() time.Time
}

// AssignmentMetrics counts assignments created/revoked (eduworld_tenant_assignments_total{op}).
type AssignmentMetrics interface{ Add(op string, n int) }

// AssignmentView is one assignment as returned (and stored for Idempotency-Key replay) by the facade.
type AssignmentView struct {
	AssignmentID uuid.UUID  `json:"assignment_id"`
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	AssignedBy   uuid.UUID  `json:"assigned_by"`
	AssignedAt   time.Time  `json:"assigned_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	// Created is set by apply only: true when this call created the active assignment, false when the tenant already had
	// one (nil: list, revoke, or a result stored before the field existed — the caller treats it as a change).
	Created *bool `json:"created,omitempty"`
}

func flag(b bool) *bool { return &b }

func viewOf(a resourceassignment.Assignment) AssignmentView {
	return AssignmentView{AssignmentID: a.ID, ResourceType: string(a.ResourceType), ResourceID: a.ResourceID, TenantID: a.TenantID,
		AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt, RevokedAt: a.RevokedAt}
}

// AssignmentsInput is the request of apply/revoke.
type AssignmentsInput struct {
	ResourceType string
	ResourceID   uuid.UUID
	TenantIDs    []uuid.UUID
	Reason       string // revoke only (3–1000 characters)
}

// ApplyTenantAssignmentsHandler is the use case behind facade applyTenantAssignments (F020-AC-15, AC-16, AC-20).
type ApplyTenantAssignmentsHandler interface {
	Handle(ctx context.Context, in AssignmentsInput) ([]AssignmentView, error)
}

// RevokeTenantAssignmentsHandler is the use case behind facade revokeTenantAssignments (F020-AC-17, AC-20).
type RevokeTenantAssignmentsHandler interface {
	Handle(ctx context.Context, in AssignmentsInput) ([]AssignmentView, error)
}

// ApplyTenantAssignments implements ApplyTenantAssignmentsHandler.
type ApplyTenantAssignments struct{ AssignmentDeps }

// RevokeTenantAssignments implements RevokeTenantAssignmentsHandler.
type RevokeTenantAssignments struct{ AssignmentDeps }

var (
	_ ApplyTenantAssignmentsHandler  = ApplyTenantAssignments{}
	_ RevokeTenantAssignmentsHandler = RevokeTenantAssignments{}
)

// prepare authorizes (System Admin with platform.tenant_data.assign), validates the request and returns the
// CrossTenantScope of exactly the requested tenants (audited cross_tenant.accessed, mode write).
func (d AssignmentDeps) prepare(ctx context.Context, in AssignmentsInput) (authz.Principal, []uuid.UUID, tenancy.Scope, error) {
	p, err := platformOp(ctx, authz.PermPlatformTenantDataAssign)
	if err != nil {
		return p, nil, nil, err
	}
	if _, err := resourceType(in.ResourceType); err != nil {
		return p, nil, nil, err
	}
	if in.ResourceID == uuid.Nil {
		return p, nil, nil, problem.Invalid("resource_id", problem.SubRequired, "is required")
	}
	tenants, err := resourceassignment.Tenants(in.TenantIDs)
	if err != nil {
		return p, nil, nil, invalid(err)
	}
	sc, err := authz.CrossTenantScope(p, tenants, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return p, nil, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return p, tenants, sc, nil
}

func (d AssignmentDeps) invalidateAfterCommit(ctx context.Context, tenants []uuid.UUID) error {
	if d.Cache == nil || len(tenants) == 0 {
		return nil
	}
	return entx.AfterCommit(ctx, func(ctx context.Context) {
		for _, t := range tenants {
			_ = d.Cache.InvalidateTenant(ctx, t) // best effort: cache entries also expire (ADR-0004)
		}
	})
}

// Handle assigns the platform resource to every requested tenant that has no active assignment yet; one
// invalid element (resource not a live platform resource, tenant unknown or not active) fails the whole
// request with each element listed and nothing written.
func (h ApplyTenantAssignments) Handle(ctx context.Context, in AssignmentsInput) (out []AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ApplyTenantAssignments")
	defer func() { done(err) }()
	p, tenants, sc, err := h.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
	rt, _ := resourceType(in.ResourceType)
	handler, ok := h.Resources.Assignable(rt)
	if !ok {
		return nil, problem.Invalid("resource_type", problem.SubInvalid, "resource type cannot be assigned yet")
	}
	ps, err := authz.PlatformScope(p)
	if err != nil {
		return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	var created []uuid.UUID
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		// The resource must be a live platform row: read under PlatformScope (tenant_id IS NULL rows only).
		owners, err := handler.Owners(tenancy.WithScope(ctx, ps), []uuid.UUID{in.ResourceID})
		if err != nil {
			return err
		}
		pe := &problem.Error{Code: problem.ValidationFailed, Detail: "request validation failed"}
		owner, found := owners[in.ResourceID]
		if !found || !owner.Platform() {
			pe.AddField("resource_id", problem.SubInvalid, "is not a live platform resource")
		} else if ef, ok := handler.(port.EfficacyFlagged); ok {
			// F035-AC-04: an efficacy instrument is scheduled by F035 only, never assigned to a tenant.
			flagged, err := ef.EfficacyInstrument(tenancy.WithScope(ctx, ps), in.ResourceID)
			if err != nil {
				return err
			}
			if flagged {
				pe.AddField("resource_id", problem.SubInvalid, "an efficacy instrument is never assigned to a tenant")
			}
		}
		active, err := h.Tenants.ActiveTenants(ctx, tenants)
		if err != nil {
			return err
		}
		for i, t := range tenants {
			if !containsID(active, t) {
				pe.AddField(fmt.Sprintf("tenant_ids[%d]", i), problem.SubInvalid, "is not an active tenant")
			}
		}
		if len(pe.Fields) > 0 {
			return pe
		}
		existing, err := h.Assignments.Active(ctx, rt, in.ResourceID, tenants)
		if err != nil {
			return err
		}
		byTenant := map[uuid.UUID]resourceassignment.Assignment{}
		for _, a := range existing {
			byTenant[a.TenantID] = a
		}
		at := now(h.Clock)
		for _, t := range resourceassignment.Missing(tenants, existing) {
			a, err := resourceassignment.New(ids.New(), rt, in.ResourceID, owner, t, p.UserID, at)
			if err != nil {
				return invalid(err)
			}
			if err := h.Assignments.Create(ctx, a); err != nil {
				return err
			}
			if err := auditIn(ctx, h.Audit, t, audit.Event{Action: ActionTenantDataAssigned, TargetType: string(rt),
				TargetID: in.ResourceID.String(), Result: audit.ResultSuccess,
				Metadata: map[string]any{"assignment_id": a.ID.String(), "tenant_id": t.String()}}); err != nil {
				return err
			}
			byTenant[t] = a
			created = append(created, t)
		}
		out = make([]AssignmentView, 0, len(tenants))
		for _, t := range tenants {
			v := viewOf(byTenant[t])
			v.Created = flag(containsID(created, t))
			out = append(out, v)
		}
		if err := h.invalidateAfterCommit(ctx, created); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	if err != nil {
		return nil, err
	}
	if h.Metrics != nil {
		h.Metrics.Add("apply", len(created))
	}
	return out, nil
}

// Handle soft-deletes the active assignments of the requested tenants (tenants without one are skipped): the
// tenant no longer sees the resource at its next read; attempts and history pinned to a version are untouched.
func (h RevokeTenantAssignments) Handle(ctx context.Context, in AssignmentsInput) (out []AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RevokeTenantAssignments")
	defer func() { done(err) }()
	if n := utf8.RuneCountInString(strings.TrimSpace(in.Reason)); n < 3 || utf8.RuneCountInString(in.Reason) > 1000 {
		if _, err := platformOp(ctx, authz.PermPlatformTenantDataAssign); err != nil {
			return nil, err
		}
		return nil, problem.Invalid("reason", problem.SubMaxLength, "must be 3–1000 characters")
	}
	_, tenants, sc, err := h.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
	rt, _ := resourceType(in.ResourceType)
	var revoked []uuid.UUID
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		existing, err := h.Assignments.Active(ctx, rt, in.ResourceID, tenants)
		if err != nil {
			return err
		}
		out = make([]AssignmentView, 0, len(existing))
		for _, a := range existing {
			r, err := h.Assignments.Revoke(ctx, a, in.Reason)
			if err != nil {
				return err
			}
			if err := auditIn(ctx, h.Audit, a.TenantID, audit.Event{Action: ActionTenantDataUnassigned, TargetType: string(rt),
				TargetID: in.ResourceID.String(), Result: audit.ResultSuccess, Reason: in.Reason,
				Metadata: map[string]any{"assignment_id": a.ID.String(), "tenant_id": a.TenantID.String()}}); err != nil {
				return err
			}
			out = append(out, viewOf(r))
			revoked = append(revoked, a.TenantID)
		}
		if err := h.invalidateAfterCommit(ctx, revoked); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	if err != nil {
		return nil, err
	}
	if h.Metrics != nil {
		h.Metrics.Add("revoke", len(revoked))
	}
	return out, nil
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
