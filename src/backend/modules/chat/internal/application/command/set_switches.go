package command

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Event and audit actions of the moderation switches (F116 US3, MODERATION §12.2).
const (
	EventSwitchChanged = "chat_policy.moderation_switch_changed"

	AuditSwitchChanged        = "chat.moderation_switch.changed"
	AuditSwitchRequested      = "chat.moderation_switch.requested"
	AuditSwitchAutoExpired    = "chat.moderation_switch.auto_expired"
	AuditSwitchRequestExpired = "chat.moderation_switch_request.expired"
)

// GaugeSyncer re-reads the switches into eduworld_chat_policy_disabled (service.SwitchGauge).
type GaugeSyncer interface {
	Sync(ctx context.Context) error
}

// SwitchAdmin are the write use cases of the moderation switches (F116 US3; ADR-0030, ADR-0078): only System Admin
// (chat.policy.toggle, internal gateway). A change that only re-enables applies at once; a disabling change is a
// request pending the approval of another System Admin. Each use case authorizes first, then runs in one UnitOfWork
// with the history row, the outbox event and the audit; after the commit the cached switches are dropped and every
// instance is told (ew:chat:switches:changed), so the change is in force everywhere within the 30 s cache freshness.
type SwitchAdmin struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.SwitchRepository
	Reader      read.ModerationReader
	Events      write.EventPublisher
	Audit       write.Auditor
	Cache       port.Cache // facts cache of the pipeline (switches:<tenant>)
	Notifier    port.CatalogNotifier
	Gauge       GaugeSyncer
	Now         func() time.Time
}

func (h SwitchAdmin) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// SwitchInput is ModerationSwitchUpdate: the whole configuration of one level.
type SwitchInput struct {
	Scope      string
	TenantID   *uuid.UUID
	EnabledAll bool
	Steps      map[string]bool
	Reason     string
	ExpiresAt  *time.Time
}

// SwitchState is ModerationSwitchState of one level after a change.
type SwitchState struct {
	TenantID   *uuid.UUID
	Configured *read.SwitchRow
	Effective  moderation.Effective
}

// Body renders the state (ModerationSwitchState).
func (s SwitchState) Body() map[string]any {
	return service.StateBody(s.TenantID, s.Configured, s.Effective, nil)
}

// SwitchResult is the outcome of setModerationSwitches: the new state (200, re-enabling) or the pending request (202,
// disabling).
type SwitchResult struct {
	State   *SwitchState
	Request *read.SwitchRequestRow
}

// switchAuthorize checks chat.policy.toggle of platform staff (System Admin only holds it, R20).
func switchAuthorize(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if err := authz.Require(ctx, p, authz.PermChatPolicyToggle, authz.Resource{}); err != nil {
		return p, err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

// levelScope returns the scope of a write on one level (AGENTS R20: a System Admin write on tenant data has exactly
// one target tenant): the platform row in the platform scope; a tenant row in the session tenant when X-Tenant-Id is
// that tenant (another one → 404), otherwise in a cross-tenant scope of exactly the target (audited).
func levelScope(p authz.Principal, target *uuid.UUID) (tenancy.Scope, error) {
	if target == nil {
		d := p.PrincipalData
		d.TenantID = nil
		sc, err := authz.PlatformScope(authz.NewPrincipal(d))
		if err != nil {
			return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return sc, nil
	}
	if p.TenantID != nil {
		if *p.TenantID != *target {
			return nil, problem.New(problem.NotFound, "")
		}
		sc, err := authz.ScopeFor(p)
		if err != nil {
			return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return sc, nil
	}
	sc, err := authz.CrossTenantScope(p, []uuid.UUID{*target}, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return sc, nil
}

func switchProblem(err error) error {
	switch {
	case errors.Is(err, moderation.ErrSwitchStepNotAllowed):
		return problem.New(problem.ModerationSwitchStepNotAllowed, "pii and grooming always run (ADR-0078)")
	case errors.Is(err, moderation.ErrSwitchUnknownStep):
		return problem.Invalid("steps", problem.SubInvalid, "unknown step")
	case errors.Is(err, moderation.ErrSwitchReason):
		return problem.New(problem.ModerationSwitchReasonRequired, "")
	case errors.Is(err, moderation.ErrSwitchExpiry):
		return problem.New(problem.ModerationSwitchExpiryRequired, "")
	case errors.Is(err, write.ErrStale), errors.Is(err, write.ErrDuplicate):
		return problem.New(problem.Conflict, "the switch changed meanwhile")
	}
	return err
}

// offList lists enabled_all (when off) and the switchable steps off in s (audit metadata, no content).
func offList(s moderation.Switch) string {
	var out []string
	if !s.EnabledAll {
		out = append(out, "enabled_all")
	}
	for _, st := range moderation.SwitchableSteps {
		if v, ok := s.Steps[st]; ok && !v {
			out = append(out, st)
		}
	}
	return strings.Join(out, ",")
}

func levelID(tenant *uuid.UUID) string {
	if tenant == nil {
		return service.PlatformLabel
	}
	return tenant.String()
}

// SetModerationSwitches implements setModerationSwitches (F116-AC-20, AC-21, AC-30, AC-31, AC-32): pii/grooming off →
// MODERATION_SWITCH_STEP_NOT_ALLOWED; reason < 10 characters → MODERATION_SWITCH_REASON_REQUIRED; a disabling change
// without expires_at in (now, now + 24 h] → MODERATION_SWITCH_EXPIRY_REQUIRED, otherwise a pending_approval request
// (no effect yet); a change that only re-enables applies at once.
func (h SwitchAdmin) SetModerationSwitches(ctx context.Context, in SwitchInput) (out SwitchResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.SetModerationSwitches")
	defer func() { done(err) }()
	p, err := switchAuthorize(ctx)
	if err != nil {
		return out, err
	}
	var target *uuid.UUID
	switch in.Scope {
	case "platform":
		if in.TenantID != nil {
			return out, problem.Invalid("tenant_id", problem.SubInvalid, "must be absent for scope platform")
		}
	case "tenant":
		if in.TenantID == nil || *in.TenantID == uuid.Nil {
			return out, problem.Invalid("tenant_id", problem.SubInvalid, "required for scope tenant")
		}
		t := *in.TenantID
		target = &t
	default:
		return out, problem.Invalid("scope", problem.SubInvalid, "platform or tenant")
	}
	sc, err := levelScope(p, target)
	if err != nil {
		return out, err
	}
	change := moderation.SwitchChange{EnabledAll: in.EnabledAll, Steps: in.Steps, Reason: in.Reason, ExpiresAt: in.ExpiresAt}
	if err := moderation.CheckSwitchChange(change); err != nil {
		return out, switchProblem(err)
	}
	reason := strings.TrimSpace(in.Reason)
	next := change.Config()
	now := h.now()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.Repo.LockSwitch(ctx, target)
		if err != nil {
			return err
		}
		var curSw *moderation.Switch
		if cur != nil {
			s := service.ToSwitch(cur)
			curSw = &s
		}
		disabling, exp, err := moderation.Classify(curSw, next, now)
		if err != nil {
			return switchProblem(err)
		}
		if disabling {
			req := read.SwitchRequestRow{ID: ids.New(), TenantID: target, EnabledAll: next.EnabledAll, Steps: next.Steps, Reason: reason,
				ExpiresAt: *exp, Status: read.RequestPending, ProposedBy: p.UserID, CreatedAt: now,
				RequestExpiresAt: moderation.RequestExpiry(now, *exp)}
			if err := h.Repo.InsertRequest(ctx, req); err != nil {
				return err
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: AuditSwitchRequested, TargetType: "moderation_switch_request",
				TargetID: req.ID.String(), Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"scope": in.Scope,
					"level": levelID(target), "disabled": offList(next), "expires_at": exp.Format(time.RFC3339), "proposed_by": p.UserID}}); err != nil {
				return err
			}
			out.Request = &req
			if pd, ok := idempotency.PendingFrom(ctx); ok {
				pd.SuccessStatus = 202
			}
			return idempotency.SetResult(ctx, service.RequestBody(req))
		}
		next.ExpiresAt = exp
		st, err := h.apply(ctx, target, cur, next, &reason, &p.UserID, nil, nil, write.SourceManual, now)
		if err != nil {
			return err
		}
		out.State = st
		return idempotency.SetResult(ctx, st.Body())
	})
	return out, err
}

// apply writes the configuration next of a level (in the transaction of ctx): the switch row, the history row, the
// event and the audit; after the commit the cached switches are dropped, the instances told and the gauge synced.
func (h SwitchAdmin) apply(ctx context.Context, tenant *uuid.UUID, cur *read.SwitchRow, next moderation.Switch, reason *string,
	actor, approver, request *uuid.UUID, source string, now time.Time) (*SwitchState, error) {
	before := moderation.AllOn()
	if cur != nil {
		before = service.ToSwitch(cur)
	}
	row, err := h.Repo.SaveSwitch(ctx, tenant, cur, next, reason, actor, now)
	if err != nil {
		return nil, switchProblem(err)
	}
	change := write.SwitchChangeRecord{ID: ids.New(), TenantID: tenant, Before: before, After: next, Reason: reason,
		ExpiresAt: next.ExpiresAt, ActorID: actor, ApprovedBy: approver, RequestID: request, Source: source, At: now}
	if err := h.Repo.InsertChange(ctx, change); err != nil {
		return nil, err
	}
	scope := "platform"
	if tenant != nil {
		scope = "tenant"
	}
	stepsOf := func(s moderation.Switch) map[string]bool {
		return service.StepsBody(func(st string) bool { v, ok := s.Steps[st]; return !ok || v })
	}
	payload := map[string]any{"tenant_id": tenant, "scope": scope, "change_id": change.ID, "request_id": request,
		"enabled_all_before": before.EnabledAll, "enabled_all_after": next.EnabledAll, "before": stepsOf(before), "after": stepsOf(next),
		"reason": reason, "expires_at": utc(next.ExpiresAt), "actor_id": actor, "approved_by": approver, "source": source, "changed_at": now}
	if err := h.Events.Publish(ctx, eventOf(EventSwitchChanged, "moderation_switch", row.ID.String(), tenant, now, payload)); err != nil {
		return nil, err
	}
	action := AuditSwitchChanged
	if source == write.SourceAutoExpire {
		action = AuditSwitchAutoExpired
	}
	meta := map[string]any{"scope": scope, "level": levelID(tenant), "source": source, "disabled": offList(next),
		"change_id": change.ID}
	if next.ExpiresAt != nil {
		meta["expires_at"] = next.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if source == write.SourceApprovedRequest {
		meta["proposed_by"], meta["approved_by"], meta["request_id"] = *actor, *approver, *request
	}
	r := ""
	if reason != nil {
		r = *reason
	}
	if err := h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "moderation_switch", TargetID: row.ID.String(),
		Result: audit.ResultSuccess, Reason: r, Metadata: meta}); err != nil {
		return nil, err
	}
	platform, tenantRow := row, (*read.SwitchRow)(nil)
	if tenant != nil {
		if platform, _, err = h.Reader.Switches(ctx, *tenant); err != nil {
			return nil, err
		}
		tenantRow = row
	}
	st := &SwitchState{TenantID: tenant, Configured: row, Effective: service.EffectiveOf(platform, tenantRow, now)}
	if h.AfterCommit != nil {
		if err := h.AfterCommit(ctx, func(ctx context.Context) { h.announce(context.WithoutCancel(ctx), tenant) }); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// announce makes the change effective on every instance: the cached switches of the tenant are dropped at once (a
// platform change is dropped by each instance for the tenants it serves on the notification), then the gauge is synced.
func (h SwitchAdmin) announce(ctx context.Context, tenant *uuid.UUID) {
	if tenant != nil && h.Cache != nil {
		if err := h.Cache.Invalidate(ctx, service.SwitchKey(*tenant)); err != nil {
			slog.WarnContext(ctx, "chat: cached switches not dropped (freshness bounds the effect)", "error", err)
		}
	}
	if h.Notifier != nil {
		if err := h.Notifier.SwitchesChanged(ctx, tenant); err != nil {
			slog.WarnContext(ctx, "chat: switch change not announced (cache freshness bounds the effect)", "error", err)
		}
	}
	if h.Gauge != nil {
		if err := h.Gauge.Sync(ctx); err != nil {
			slog.WarnContext(ctx, "chat: moderation switch gauge not synced", "error", err)
		}
	}
}

func utc(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}
