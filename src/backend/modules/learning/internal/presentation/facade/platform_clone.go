package facade

import (
	"context"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ExecuteTenantCloneStep implements executeTenantCloneStep (F020-AC-27 … AC-30): idempotent by
// (job_id, resource_type) — a retried saga step gets the first result.
func (f Facade) ExecuteTenantCloneStep(ctx context.Context, req learningapi.ExecuteTenantCloneStepRequestObject) (learningapi.ExecuteTenantCloneStepResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if _, ok := modcall.IdempotencyKeyFrom(ctx); !ok {
		return nil, problem.New(problem.IdempotencyKeyRequired, "")
	}
	if _, ok := authz.PrincipalFrom(ctx); !ok {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	b := *req.Body
	in := command.CloneStepInput{JobID: req.JobId, SourceTenant: b.SourceTenantId, TargetTenant: b.TargetTenantId,
		ResourceType: string(b.ResourceType), ResourceIDs: b.ResourceIds}
	if b.IdMap != nil {
		in.IDMap = map[uuid.UUID]uuid.UUID{}
		for k, v := range *b.IdMap {
			src, err := uuid.Parse(k)
			if err != nil {
				return nil, problem.Invalid("id_map", problem.SubInvalid, "keys must be UUIDs")
			}
			in.IDMap[src] = v
		}
	}
	r, err := replayOr(ctx, f.Idempotency, stepKey(req.JobId, string(b.ResourceType)), in, func(ctx context.Context) (command.CloneStepResult, error) {
		return f.Clone.Handle(ctx, in)
	})
	if err != nil {
		return nil, err
	}
	out := learningapi.ExecuteTenantCloneStep200JSONResponse{JobId: r.JobID, ResourceType: learningapi.CommonInternalCommonV1ResourceType(r.ResourceType),
		Status: learningapi.CommonInternalCommonV1TenantCloneStepResultStatus(r.Status)}
	for _, c := range r.Cloned {
		out.Cloned = append(out.Cloned, struct {
			SourceId uuid.UUID `json:"source_id"`
			TargetId uuid.UUID `json:"target_id"`
		}{SourceId: c.SourceID, TargetId: c.TargetID})
	}
	if out.Cloned == nil {
		out.Cloned = []struct {
			SourceId uuid.UUID `json:"source_id"`
			TargetId uuid.UUID `json:"target_id"`
		}{}
	}
	if len(r.Skipped) > 0 {
		skipped := make([]struct {
			Reason   string    `json:"reason"`
			SourceId uuid.UUID `json:"source_id"`
		}, len(r.Skipped))
		for i, s := range r.Skipped {
			skipped[i].Reason, skipped[i].SourceId = s.Reason, s.SourceID
		}
		out.Skipped = &skipped
	}
	return out, nil
}
