package command

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
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

// Event and audit action of a rejected request (F122 T060).
const (
	EventSwitchChangeRejected  = "moderation_switch.change_rejected"
	AuditSwitchRequestRejected = "chat.moderation_switch_request.rejected"
	// RejectReasonMax is maxLength of the reason of rejectModerationSwitchChange.
	RejectReasonMax = 500
)

// RejectModerationSwitchChange implements rejectModerationSwitchChange (F122 T060, F122-AC-27, ADR-0078): a System Admin
// other than the proposer rejects a pending request of the level of X-Tenant-Id (none: a platform request) with a reason
// (1–500 characters) and If-Match = the version of the request. The proposer → 403 MODERATION_SWITCH_SELF_APPROVAL
// (audited denial); unknown in that level → 404; no longer pending or past its approval window → 409 CONFLICT; another
// version → 412 VERSION_MISMATCH. Nothing changes in force and no history row is written; the rejection is audited (proposer,
// rejecter, reason) and moderation_switch.change_rejected is published in the same transaction.
func (h SwitchAdmin) RejectModerationSwitchChange(ctx context.Context, id uuid.UUID, version int64, reason string) (out read.SwitchRequestRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RejectModerationSwitchChange")
	defer func() { done(err) }()
	p, err := switchAuthorize(ctx)
	if err != nil {
		return out, err
	}
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return out, problem.Invalid("reason", problem.SubRequired, "a reason is required")
	case utf8.RuneCountInString(reason) > RejectReasonMax:
		return out, problem.Invalid("reason", problem.SubMaxLength, "at most 500 characters")
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
		if req.Status != read.RequestPending || !now.Before(req.RequestExpiresAt) {
			return problem.New(problem.Conflict, "the request is no longer pending approval")
		}
		if req.Version != version {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := h.Repo.RejectRequest(ctx, req.ID, p.UserID, version, now); err != nil {
			return switchProblem(err)
		}
		req.Status, req.DecidedAt, req.Version = read.RequestRejected, &now, version+1
		scope := "platform"
		if target != nil {
			scope = "tenant"
		}
		sw := moderation.SwitchChange{EnabledAll: req.EnabledAll, Steps: req.Steps}.Config()
		steps := service.StepsBody(func(st string) bool { v, ok := sw.Steps[st]; return !ok || v })
		payload := map[string]any{"request_id": req.ID, "tenant_id": target, "scope": scope, "enabled_all": req.EnabledAll, "steps": steps,
			"proposed_by": req.ProposedBy, "rejected_by": p.UserID, "reason": reason, "rejected_at": now}
		if err := h.Events.Publish(ctx, eventOf(EventSwitchChangeRejected, "moderation_switch_request", req.ID.String(), target, now, payload)); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditSwitchRequestRejected, TargetType: "moderation_switch_request",
			TargetID: req.ID.String(), Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"scope": scope,
				"level": levelID(target), "disabled": offList(sw), "proposed_by": req.ProposedBy, "rejected_by": p.UserID}}); err != nil {
			return err
		}
		out = *req
		return idempotency.SetResult(ctx, service.RequestBody(out))
	})
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.ModerationSwitchSelfApproval { // audited outside the rolled-back transaction
		authz.RecordDenied(ctx, string(problem.ModerationSwitchSelfApproval), map[string]any{"request_id": id, "operation": "reject"})
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
