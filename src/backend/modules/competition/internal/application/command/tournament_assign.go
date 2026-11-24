package command

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// TemplateAssignments are the facade use cases applyTenantAssignments, revokeTenantAssignments and
// listResourceAssignments of module competition for the tournament and challenge templates (F107-AC-29, ADR-0029): the
// saga of module identity (F045) assigns a published platform template to tenants; unassigning soft-deletes the row and
// never touches the tournaments already created (pinned template version).
type TemplateAssignments struct {
	UoW   write.UnitOfWork
	Repo  write.AssignmentRepository
	Audit write.Auditor
	Now   func() time.Time
}

// AssignmentView is one assignment as returned (and replayed) by the facade.
type AssignmentView struct {
	AssignmentID uuid.UUID  `json:"assignment_id"`
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	AssignedBy   uuid.UUID  `json:"assigned_by"`
	AssignedAt   time.Time  `json:"assigned_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

func assignmentView(a write.AssignmentRow) AssignmentView {
	return AssignmentView{AssignmentID: a.ID, ResourceType: a.ResourceType, ResourceID: a.ResourceID, TenantID: a.Tenant, AssignedBy: a.AssignedBy,
		AssignedAt: a.AssignedAt, RevokedAt: a.RevokedAt}
}

func (h TemplateAssignments) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// assigner is the original principal of the saga: platform staff with platform.tenant_data.assign.
func assigner(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTenantDataAssign, authz.Resource{}); err != nil {
		return p, err
	}
	if !p.IsPlatform() {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

// AssignInput is TenantAssignmentApplyRequest / TenantAssignmentRevokeRequest.
type AssignInput struct {
	ResourceType string
	ResourceID   uuid.UUID
	TenantIDs    []uuid.UUID
	Reason       string
}

func (in AssignInput) check() ([]uuid.UUID, error) {
	if challenge.ConfigType(in.ResourceType) == "" {
		return nil, problem.Invalid("resource_type", problem.SubInvalid, "module competition assigns tournament_template and challenge_template")
	}
	if in.ResourceID == uuid.Nil {
		return nil, problem.Invalid("resource_id", problem.SubRequired, "is required")
	}
	if len(in.TenantIDs) == 0 || len(in.TenantIDs) > 100 {
		return nil, problem.Invalid("tenant_ids", problem.SubInvalid, "has 1 to 100 tenants")
	}
	seen := map[uuid.UUID]bool{}
	out := make([]uuid.UUID, 0, len(in.TenantIDs))
	for _, t := range in.TenantIDs {
		if t != uuid.Nil && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out, nil
}

func (h TemplateAssignments) scope(ctx context.Context, p authz.Principal, tenants []uuid.UUID) (context.Context, error) {
	sc, err := authz.CrossTenantScope(p, tenants, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return ctx, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return tenancy.WithScope(ctx, sc), nil
}

func auditTenant(ctx context.Context, a write.Auditor, tenant uuid.UUID, e audit.Event) error {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return a.Record(tenancy.WithScope(ctx, sc), e)
}

// Apply assigns a live published platform template to every requested active tenant without an active assignment
// (F107-AC-29): a template that is none, or a tenant that is not active, fails the whole request with each element
// listed; audit tenant_data.assigned per tenant. Idempotent by Idempotency-Key (the facade replays the result).
func (h TemplateAssignments) Apply(ctx context.Context, in AssignInput) (out []AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.ApplyTenantAssignments")
	defer func() { done(err) }()
	p, err := assigner(ctx)
	if err != nil {
		return nil, err
	}
	tenants, err := in.check()
	if err != nil {
		return nil, err
	}
	sctx, err := h.scope(ctx, p, tenants)
	if err != nil {
		return nil, err
	}
	err = h.UoW.Do(sctx, func(ctx context.Context) error {
		pe := &problem.Error{Code: problem.ValidationFailed, Detail: "request validation failed"}
		typ, err := h.Repo.TemplateConfig(ctx, in.ResourceID)
		if err != nil {
			return err
		}
		if challenge.ResourceType(typ) != in.ResourceType {
			pe.AddField("resource_id", problem.SubInvalid, "is not a live published platform template of this resource type")
		}
		active, err := h.Repo.ActiveTenants(ctx, tenants)
		if err != nil {
			return err
		}
		for i, t := range tenants {
			if !active[t] {
				pe.AddField(fmt.Sprintf("tenant_ids[%d]", i), problem.SubInvalid, "is not an active tenant")
			}
		}
		if len(pe.Fields) > 0 {
			return pe
		}
		existing, err := h.Repo.Active(ctx, in.ResourceType, in.ResourceID, tenants)
		if err != nil {
			return err
		}
		have := map[uuid.UUID]write.AssignmentRow{}
		for _, a := range existing {
			have[a.Tenant] = a
		}
		now := h.now()
		out = make([]AssignmentView, 0, len(tenants))
		for _, t := range tenants {
			a, ok := have[t]
			if !ok {
				a = write.AssignmentRow{ID: ids.New(), ResourceType: in.ResourceType, ResourceID: in.ResourceID, Tenant: t, AssignedBy: p.UserID,
					AssignedAt: now}
				if err := h.Repo.Insert(ctx, a); err != nil {
					return err
				}
				if err := auditTenant(ctx, h.Audit, t, audit.Event{Action: AuditTenantDataAssigned, TargetType: in.ResourceType,
					TargetID: in.ResourceID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"assignment_id": a.ID.String(),
						"tenant_id": t.String()}}); err != nil {
					return err
				}
			}
			out = append(out, assignmentView(a))
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// Revoke soft-deletes the active assignments of the requested tenants (F107-AC-29): tenants without one are skipped,
// the tournaments created from the template keep running on their pinned version; audit tenant_data.unassigned.
func (h TemplateAssignments) Revoke(ctx context.Context, in AssignInput) (out []AssignmentView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RevokeTenantAssignments")
	defer func() { done(err) }()
	p, err := assigner(ctx)
	if err != nil {
		return nil, err
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(in.Reason)); n < 3 || utf8.RuneCountInString(in.Reason) > 1000 {
		return nil, problem.Invalid("reason", problem.SubMaxLength, "must be 3–1000 characters")
	}
	tenants, err := in.check()
	if err != nil {
		return nil, err
	}
	sctx, err := h.scope(ctx, p, tenants)
	if err != nil {
		return nil, err
	}
	err = h.UoW.Do(sctx, func(ctx context.Context) error {
		existing, err := h.Repo.Active(ctx, in.ResourceType, in.ResourceID, tenants)
		if err != nil {
			return err
		}
		now := h.now()
		out = make([]AssignmentView, 0, len(existing))
		for _, a := range existing {
			if err := h.Repo.Revoke(ctx, a.ID, p.UserID, now); err != nil {
				return err
			}
			at := now
			a.RevokedAt = &at
			if err := auditTenant(ctx, h.Audit, a.Tenant, audit.Event{Action: AuditTenantDataUnassigned, TargetType: in.ResourceType,
				TargetID: in.ResourceID.String(), Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{
					"assignment_id": a.ID.String(), "tenant_id": a.Tenant.String()}}); err != nil {
				return err
			}
			out = append(out, assignmentView(a))
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}
