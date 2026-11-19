package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ActionTenantDataCloned is the audit action of a clone step, recorded in the source and the target tenant
// (F020-AC-30, MULTITENANCY §10).
const ActionTenantDataCloned = "tenant_data.cloned"

// MaxCloneItems is maxItems of resource_ids in TenantCloneStepRequest.
const MaxCloneItems = 500

// Clone step statuses (TenantCloneStepResult.status).
const (
	CloneCompleted       = "completed"
	ClonePartiallyFailed = "partially_failed"
	CloneFailed          = "failed"
)

// CloneDeps are the collaborators of the clone step.
type CloneDeps struct {
	UoW       write.UnitOfWork
	Resources *port.Registry
	Tenants   port.TenantDirectory
	Audit     write.Auditor
	Clock     func() time.Time
}

// CloneStepInput is the request of executeTenantCloneStep.
type CloneStepInput struct {
	JobID        uuid.UUID
	SourceTenant uuid.UUID
	TargetTenant uuid.UUID
	ResourceType string
	ResourceIDs  []uuid.UUID
	IDMap        map[uuid.UUID]uuid.UUID
}

// ClonePair maps a source id to its copy.
type ClonePair struct {
	SourceID uuid.UUID `json:"source_id"`
	TargetID uuid.UUID `json:"target_id"`
}

// CloneSkip is a source id that was not cloned, with the reason.
type CloneSkip struct {
	SourceID uuid.UUID `json:"source_id"`
	Reason   string    `json:"reason"`
}

// CloneStepResult is the result of a clone step (stored for replay by (job_id, resource_type)).
type CloneStepResult struct {
	JobID        uuid.UUID   `json:"job_id"`
	ResourceType string      `json:"resource_type"`
	Status       string      `json:"status"`
	Cloned       []ClonePair `json:"cloned"`
	Skipped      []CloneSkip `json:"skipped,omitempty"`
}

// ExecuteCloneStepHandler is the use case behind facade executeTenantCloneStep (F020 US5).
type ExecuteCloneStepHandler interface {
	Handle(ctx context.Context, in CloneStepInput) (CloneStepResult, error)
}

// ExecuteCloneStep implements ExecuteCloneStepHandler.
type ExecuteCloneStep struct{ CloneDeps }

var _ ExecuteCloneStepHandler = ExecuteCloneStep{}

// Handle deep-copies each source item of the source tenant into the target tenant through the registered
// handler, under tenancy.Bypass(tenant_clone) (audit cross_tenant.accessed, mode write); each element runs in
// a savepoint so a failed element leaves no partial copy (F020-AC-29). Student data is never copied (R22,
// F020-AC-28). A type without a cloner fails every element with the reason.
func (h ExecuteCloneStep) Handle(ctx context.Context, in CloneStepInput) (out CloneStepResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ExecuteCloneStep")
	defer func() { done(err) }()
	p, err := platformOp(ctx, authz.PermPlatformTenantDataClone)
	if err != nil {
		return out, err
	}
	rt, err := resourceType(in.ResourceType)
	if err != nil {
		return out, err
	}
	switch {
	case in.JobID == uuid.Nil:
		return out, problem.Invalid("job_id", problem.SubRequired, "is required")
	case in.SourceTenant == uuid.Nil || in.TargetTenant == uuid.Nil || in.SourceTenant == in.TargetTenant:
		return out, problem.Invalid("target_tenant_id", problem.SubInvalid, "source and target must be two different tenants")
	case len(in.ResourceIDs) == 0 || len(in.ResourceIDs) > MaxCloneItems:
		return out, problem.Invalid("resource_ids", problem.SubInvalid, "must hold 1–500 ids")
	}
	out = CloneStepResult{JobID: in.JobID, ResourceType: string(rt), Cloned: []ClonePair{}}
	handler, registered := h.Resources.Get(rt)
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonTenantClone, tenancy.WithTenantIDs(in.SourceTenant, in.TargetTenant),
		tenancy.WithMode(tenancy.ModeWrite))
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		active, err := h.Tenants.ActiveTenants(ctx, []uuid.UUID{in.TargetTenant})
		if err != nil {
			return err
		}
		if len(active) == 0 {
			return problem.Invalid("target_tenant_id", problem.SubInvalid, "is not an active tenant")
		}
		at := now(h.Clock)
		seen := map[uuid.UUID]bool{}
		for _, id := range in.ResourceIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			if !registered {
				out.Skipped = append(out.Skipped, CloneSkip{SourceID: id, Reason: "resource type " + string(rt) + " has no cloner in module learning"})
				continue
			}
			var newID uuid.UUID
			err := h.UoW.Savepoint(ctx, func(ctx context.Context) error {
				var err error
				newID, err = handler.Clone(ctx, port.CloneInput{JobID: in.JobID, SourceID: id, SourceTenant: in.SourceTenant,
					TargetTenant: in.TargetTenant, Actor: p.UserID, At: at, IDMap: in.IDMap})
				return err
			})
			switch {
			case errors.Is(err, port.ErrNotInSourceTenant):
				out.Skipped = append(out.Skipped, CloneSkip{SourceID: id, Reason: "not a live resource of the source tenant"})
			case err != nil:
				out.Skipped = append(out.Skipped, CloneSkip{SourceID: id, Reason: "clone failed"})
			default:
				out.Cloned = append(out.Cloned, ClonePair{SourceID: id, TargetID: newID})
			}
		}
		switch {
		case len(out.Skipped) == 0:
			out.Status = CloneCompleted
		case len(out.Cloned) == 0:
			out.Status = CloneFailed
		default:
			out.Status = ClonePartiallyFailed
		}
		meta := map[string]any{"job_id": in.JobID.String(), "resource_type": string(rt), "source_tenant_id": in.SourceTenant.String(),
			"target_tenant_id": in.TargetTenant.String(), "cloned": fmt.Sprint(len(out.Cloned)), "skipped": fmt.Sprint(len(out.Skipped)),
			"status": out.Status}
		for _, t := range []uuid.UUID{in.SourceTenant, in.TargetTenant} {
			if err := auditIn(ctx, h.Audit, t, audit.Event{Action: ActionTenantDataCloned, TargetType: string(rt),
				TargetID: in.JobID.String(), Result: audit.ResultSuccess, Metadata: meta}); err != nil {
				return err
			}
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}
