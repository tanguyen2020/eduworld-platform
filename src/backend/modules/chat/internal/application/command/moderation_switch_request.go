package command

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ApproveModerationSwitchChange implements approveModerationSwitchChange (F116-AC-32, ADR-0078): a second System Admin
// approves a pending request of the level of X-Tenant-Id (none: a platform request). The proposer cannot approve
// (403 MODERATION_SWITCH_SELF_APPROVAL, audited as a denial); a request no longer pending or past its approval window
// is 409 CONFLICT. The requested values apply with the reason and expires_at of the request; the history row has the
// proposer as actor and the approver, the audit both.
func (h SwitchAdmin) ApproveModerationSwitchChange(ctx context.Context, id uuid.UUID) (out SwitchState, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ApproveModerationSwitchChange")
	defer func() { done(err) }()
	p, err := switchAuthorize(ctx)
	if err != nil {
		return out, err
	}
	var target *uuid.UUID
	if p.TenantID != nil {
		t := *p.TenantID
		target = &t
	}
	sc, err := levelScope(p, target)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		req, err := h.Repo.LockRequest(ctx, target, id)
		if err != nil {
			return err
		}
		if req == nil {
			return notFound()
		}
		if req.ProposedBy == p.UserID {
			return problem.New(problem.ModerationSwitchSelfApproval, "")
		}
		if req.Status != read.RequestPending || !now.Before(req.RequestExpiresAt) || !now.Before(req.ExpiresAt) {
			return problem.New(problem.Conflict, "the request is no longer pending approval")
		}
		cur, err := h.Repo.LockSwitch(ctx, target)
		if err != nil {
			return err
		}
		if err := h.Repo.DecideRequest(ctx, req.ID, read.RequestApproved, &p.UserID, now); err != nil {
			return switchProblem(err)
		}
		exp := req.ExpiresAt
		next := moderation.SwitchChange{EnabledAll: req.EnabledAll, Steps: req.Steps}.Config()
		next.ExpiresAt = &exp
		reason := req.Reason
		st, err := h.apply(ctx, target, cur, next, &reason, &req.ProposedBy, &p.UserID, &req.ID, write.SourceApprovedRequest, now)
		if err != nil {
			return err
		}
		out = *st
		return idempotency.SetResult(ctx, st.Body())
	})
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.ModerationSwitchSelfApproval { // audited outside the rolled-back transaction
		authz.RecordDenied(ctx, string(problem.ModerationSwitchSelfApproval), map[string]any{"request_id": id})
	}
	return out, err
}

// ModerationSwitchExpirerJob is the job name and Valkey lock of the expirer (ew:lock:job:moderation-switch-expirer).
const ModerationSwitchExpirerJob = "moderation-switch-expirer"

// SwitchExpiry is one run of the job moderation-switch-expirer (worker jobs, F116-AC-25, F116-AC-32): every level whose
// disabled values reached expires_at is switched back on (history source auto_expire, audit, event, cache dropped,
// gauge 0); every request still pending past its approval window becomes expired. One replica runs a cycle (lock);
// each level is re-checked under its row lock, so a concurrent run or a manual change is never undone.
type SwitchExpiry struct {
	Admin SwitchAdmin
	Lock  port.JobLock // nil: no lease (the row locks still keep a run correct)
	Batch int
}

// Run runs one cycle and returns the number of switches and requests expired.
func (h SwitchExpiry) Run(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ExpireModerationSwitches")
	defer func() { done(err) }()
	if h.Lock != nil {
		ok, err := h.Lock.Acquire(ctx, ModerationSwitchExpirerJob, moderation.PropagationMax/2)
		if err != nil {
			slog.WarnContext(ctx, "chat: expirer lease unavailable, running under row locks", "error", err)
		} else if !ok {
			return 0, nil
		}
	}
	a := h.Admin
	batch := h.Batch
	if batch <= 0 {
		batch = 500
	}
	ps, err := authz.SystemPlatformScope(ModerationSwitchExpirerJob)
	if err != nil {
		return 0, err
	}
	now := a.now()
	var levels []write.SwitchLevel
	var reqs []write.ExpiredRequest
	if err := a.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		if levels, err = a.Repo.ExpiredSwitches(ctx, now, batch); err != nil {
			return err
		}
		reqs, err = a.Repo.ExpiredRequests(ctx, now, batch)
		return err
	}); err != nil {
		return 0, err
	}
	scopeOf := func(tenant *uuid.UUID) (tenancy.Scope, error) {
		if tenant == nil {
			return ps, nil
		}
		return authz.EventScope(*tenant)
	}
	var errs []error
	for _, l := range levels {
		sc, err := scopeOf(l.TenantID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		done := false
		err = a.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			cur, err := a.Repo.LockSwitch(ctx, l.TenantID)
			if err != nil || cur == nil || cur.ExpiresAt == nil || cur.ExpiresAt.After(now) {
				return err
			}
			_, err = a.apply(ctx, l.TenantID, cur, moderation.AllOn(), nil, nil, nil, nil, write.SourceAutoExpire, now)
			done = err == nil
			return err
		})
		if err != nil {
			errs = append(errs, err)
		} else if done {
			n++
		}
	}
	for _, r := range reqs {
		sc, err := scopeOf(r.TenantID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		expired := false
		err = a.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			req, err := a.Repo.LockRequest(ctx, r.TenantID, r.ID)
			if err != nil || req == nil || req.Status != read.RequestPending || req.RequestExpiresAt.After(now) {
				return err
			}
			if err := a.Repo.DecideRequest(ctx, req.ID, read.RequestExpired, nil, now); err != nil {
				return err
			}
			expired = true
			return a.Audit.Record(ctx, audit.Event{Action: AuditSwitchRequestExpired, TargetType: "moderation_switch_request",
				TargetID: req.ID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"level": levelID(req.TenantID),
					"proposed_by": req.ProposedBy, "disabled": offList(moderation.SwitchChange{EnabledAll: req.EnabledAll, Steps: req.Steps}.Config())}})
		})
		if err != nil {
			errs = append(errs, err)
		} else if expired {
			n++
		}
	}
	if a.Gauge != nil {
		if err := a.Gauge.Sync(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return n, errors.Join(errs...)
}

// ExpiryTick is the interval of the expirer: half of moderation_config_propagation_max, so a switch is back on within
// 30 s of its expires_at (the pipeline already treats a lapsed value as on, F116-AC-22).
const ExpiryTick = moderation.PropagationMax / 2
