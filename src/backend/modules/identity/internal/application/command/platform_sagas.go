package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Saga types of F045 (platform.saga_runs.saga_type).
const (
	SagaTenantClone = "tenant_clone"
	SagaUserMerge   = "user_merge"
)

func (d PlatformAdminDeps) inSagaScope(ctx context.Context, fn func(ctx context.Context) error) error {
	sc, err := authz.SystemPlatformScope("saga")
	if err != nil {
		return err
	}
	return d.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

// CloneDefinition is the tenant_clone saga with n step slots: step i runs plan[i] of the subject (one module
// clones one resource type with the id_map of the steps before, F045-AC-13). Clones are drafts kept for System
// Admin: no compensation.
func CloneDefinition(d PlatformAdminDeps, n int) saga.Definition {
	def := saga.Definition{Type: SagaTenantClone}
	for i := 0; i < n; i++ {
		def.Steps = append(def.Steps, saga.Step{Service: "module", Action: "clone", Do: d.cloneStep(i)})
	}
	return def
}

func (d PlatformAdminDeps) cloneStep(i int) func(ctx context.Context, run write.SagaRun) (saga.Result, error) {
	return func(ctx context.Context, run write.SagaRun) (saga.Result, error) {
		var subj cloneSubject
		if err := json.Unmarshal(run.Subject, &subj); err != nil {
			return saga.Result{}, err
		}
		if i >= len(subj.Plan) {
			return saga.Result{Outcome: saga.NothingToDo}, nil
		}
		st := subj.Plan[i]
		var job *write.CloneJob
		var idMap []platformadmin.ClonePair
		if err := d.inSagaScope(ctx, func(ctx context.Context) error {
			var err error
			if job, err = d.Repo.CloneJobBySaga(ctx, run.ID); err != nil || job == nil {
				return fmt.Errorf("clone job of saga %s: %w", run.ID, err)
			}
			if err = d.Repo.MarkCloneRunning(ctx, job.ID); err != nil {
				return err
			}
			idMap, err = d.Repo.CloneMap(ctx, job.ID)
			return err
		}); err != nil {
			return saga.Result{}, err
		}
		req := port.CloneStepRequest{JobID: job.ID, Source: job.Source, Target: job.Target, ResourceType: st.ResourceType}
		if st.Root {
			req.ResourceIDs = job.ResourceIDs
		}
		if len(idMap) > 0 {
			req.IDMap = map[uuid.UUID]uuid.UUID{}
			for _, p := range idMap {
				req.IDMap[p.SourceID] = p.CloneID
			}
		}
		actx, err := onBehalf(ctx, job.RequestedBy, authz.PermPlatformTenantDataClone)
		if err != nil {
			return saga.Result{}, err
		}
		key, _ := modcall.IdempotencyKeyFrom(ctx) // the stable step key of the engine
		res, err := d.Owners.CloneStep(modcall.WithIdempotencyKey(actx, key), st.Module, req)
		if err != nil {
			return saga.Result{}, err
		}
		pairs := make([]platformadmin.ClonePair, 0, len(res.Cloned))
		for _, c := range res.Cloned {
			pairs = append(pairs, platformadmin.ClonePair{ResourceType: st.ResourceType, SourceID: c.SourceID, CloneID: c.TargetID})
		}
		if err := d.inSagaScope(ctx, func(ctx context.Context) error { return d.Repo.AppendCloneMap(ctx, job.ID, pairs) }); err != nil {
			return saga.Result{}, err
		}
		return saga.Result{Outcome: saga.Done, Response: platformadmin.CloneStepOutcome{StepNo: i, Module: st.Module,
			ResourceType: st.ResourceType, Status: res.Status, Cloned: len(pairs), Skipped: res.Skipped}}, nil
	}
}

// MergeDefinition is the user_merge saga: step 0 is the identity step (identities, SIS links, source account,
// issue, audit user.merged), then mergeUserData of learning, game, competition and chat with Idempotency-Key =
// merge_id (F045-AC-27, AC-28). No compensation: a blocked step stops the saga, done steps never run again.
func MergeDefinition(d PlatformAdminDeps) saga.Definition {
	def := saga.Definition{Type: SagaUserMerge, Steps: []saga.Step{{Service: platformadmin.ModuleIdentity, Action: "transfer", Do: d.mergeIdentityStep}}}
	for _, m := range platformadmin.ParticipantModules {
		def.Steps = append(def.Steps, saga.Step{Service: m, Action: "merge", Do: d.mergeModuleStep(m)})
	}
	return def
}

func (d PlatformAdminDeps) mergeOf(ctx context.Context, run write.SagaRun) (*write.UserMerge, error) {
	var m *write.UserMerge
	err := d.inSagaScope(ctx, func(ctx context.Context) error {
		var err error
		m, err = d.Repo.MergeBySaga(ctx, run.ID)
		return err
	})
	if err == nil && m == nil {
		err = errNoMerge
	}
	return m, err
}

func (d PlatformAdminDeps) mergeIdentityStep(ctx context.Context, run write.SagaRun) (saga.Result, error) {
	var tr write.MergeTransfer
	err := d.inSagaScope(ctx, func(ctx context.Context) error {
		m, err := d.Repo.MergeBySaga(ctx, run.ID)
		if err != nil {
			return err
		}
		if m == nil {
			return errNoMerge
		}
		if tr, err = d.Repo.TransferIdentities(ctx, *m, d.now()); err != nil {
			return err
		}
		// user.merged in the target tenant: ids and the reason only, no PII (F045-AC-32, R14).
		return auditIn(ctx, d.Audit, m.TenantID, audit.Event{Action: "user.merged", TargetType: "user", TargetID: m.Into.String(),
			Result: audit.ResultSuccess, Reason: m.Reason, Metadata: map[string]any{"merge_id": m.ID.String(), "from_user_id": m.From.String(),
				"into_user_id": m.Into.String(), "identities_moved": tr.Identities, "identities_superseded": tr.IdentitiesSuperseded,
				"sis_links_moved": tr.SisLinks, "sis_links_superseded": tr.SisLinksSuperseded}})
	})
	if err != nil {
		return saga.Result{}, err
	}
	return saga.Result{Outcome: saga.Done, Response: tr}, nil
}

func (d PlatformAdminDeps) mergeModuleStep(module string) func(ctx context.Context, run write.SagaRun) (saga.Result, error) {
	return func(ctx context.Context, run write.SagaRun) (saga.Result, error) {
		m, err := d.mergeOf(ctx, run)
		if err != nil {
			return saga.Result{}, err
		}
		actx, err := onBehalf(ctx, m.MergedBy, authz.PermPlatformUserMerge)
		if err != nil {
			return saga.Result{}, err
		}
		sc, err := authz.EventScope(m.TenantID)
		if err != nil {
			return saga.Result{}, err
		}
		actx = modcall.WithIdempotencyKey(tenancy.WithScope(actx, sc), m.ID.String()) // Idempotency-Key = merge_id
		res, err := d.Owners.MergeUserData(actx, module, m.From, port.MergeUserDataRequest{MergeID: m.ID, Into: m.Into, Reason: m.Reason})
		if err != nil {
			return saga.Result{}, err
		}
		outcome := res.Status
		if outcome != saga.Done && outcome != saga.Blocked && outcome != saga.NothingToDo {
			return saga.Result{}, fmt.Errorf("%s.mergeUserData: unexpected status %q", module, res.Status)
		}
		return saga.Result{Outcome: outcome, Response: map[string]string{"status": res.Status, "blocked_reason": platformadmin.Bound(res.BlockedReason, 255)}}, nil
	}
}

// PlatformSagas runs the F045 sagas (job platform-saga, profile worker-jobs): resumes the due runs and closes
// the clone jobs whose saga finished (status, stats, errors, audit and tenant_data.cloned, F045-AC-17, AC-18).
type PlatformSagas struct {
	PlatformAdminDeps
	Limit      int           // runs per tick (default 50)
	StuckAfter time.Duration // SagaStuck threshold (default 1 h)
}

// Definitions are the saga definitions the job resumes.
func (h PlatformSagas) Definitions() map[string]saga.Definition {
	return map[string]saga.Definition{
		SagaTenantClone: CloneDefinition(h.PlatformAdminDeps, platformadmin.MaxPlanSteps),
		SagaUserMerge:   MergeDefinition(h.PlatformAdminDeps),
	}
}

// RunOnce executes one tick; it returns the runs advanced plus the jobs closed.
func (h PlatformSagas) RunOnce(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.job.PlatformSagas")
	defer func() { done(err) }()
	limit, stuck := h.Limit, h.StuckAfter
	if limit <= 0 {
		limit = 50
	}
	if stuck <= 0 {
		stuck = time.Hour
	}
	if n, err = h.Sagas.ResumeDue(ctx, h.Definitions(), limit, stuck); err != nil {
		return n, err
	}
	closed, err := h.closeCloneJobs(ctx, limit)
	return n + closed, err
}

// ClonedPayload is the payload of tenant_data.cloned.
type ClonedPayload struct {
	JobID          string `json:"job_id"`
	SourceTenantID string `json:"source_tenant_id"`
	TargetTenantID string `json:"target_tenant_id"`
	ResourceType   string `json:"resource_type"`
	Count          int    `json:"count"`
	Status         string `json:"status"`
}

func (h PlatformSagas) closeCloneJobs(ctx context.Context, limit int) (int, error) {
	closed := 0
	err := h.inSagaScope(ctx, func(ctx context.Context) error {
		jobs, err := h.Repo.OpenCloneJobs(ctx, limit)
		if err != nil {
			return err
		}
		for _, j := range jobs {
			if j.SagaStatus != saga.StatusSucceeded && j.SagaStatus != saga.StatusFailed && j.SagaStatus != saga.StatusBlocked {
				continue
			}
			outcomes := make([]platformadmin.CloneStepOutcome, 0, len(j.Steps))
			for _, s := range j.Steps {
				var o platformadmin.CloneStepOutcome
				if len(s.Response) > 0 {
					_ = json.Unmarshal(s.Response, &o)
				}
				o.StepNo = s.No
				if o.Module == "" {
					o.Module = s.Service
				}
				if s.Status != saga.Done && s.Status != saga.NothingToDo {
					o.Status = "failed"
					o.Error = s.Error
				}
				if s.Status == saga.NothingToDo {
					o.Status = platformadmin.CloneSucceeded
				}
				outcomes = append(outcomes, o)
			}
			res := platformadmin.SummarizeClone(outcomes, j.SagaStatus != saga.StatusSucceeded, j.SagaError)
			ok, err := h.Repo.FinishCloneJob(ctx, j.ID, res, h.now())
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			target := j.Target
			if err := h.Events.Publish(ctx, write.Event{Type: "tenant_data.cloned", SubjectType: "tenant_clone_job", SubjectID: j.ID.String(),
				TenantID: &target, Payload: ClonedPayload{JobID: j.ID.String(), SourceTenantID: j.Source.String(), TargetTenantID: j.Target.String(),
					ResourceType: j.ResourceType, Count: res.Count, Status: res.Status}}); err != nil {
				return err
			}
			for _, t := range []uuid.UUID{j.Source, j.Target} { // audit in both tenants (F045-AC-18)
				if err := auditIn(ctx, h.Audit, t, audit.Event{Action: "tenant_data.cloned", TargetType: "tenant_clone_job", TargetID: j.ID.String(),
					Result: audit.ResultSuccess, Metadata: map[string]any{"source_tenant_id": j.Source.String(), "target_tenant_id": j.Target.String(),
						"resource_type": j.ResourceType, "count": res.Count, "status": res.Status, "requested_by": j.RequestedBy.String()}}); err != nil {
					return err
				}
			}
			closed++
		}
		return nil
	})
	return closed, err
}
