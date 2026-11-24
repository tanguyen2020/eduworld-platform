package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/breakglass"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Events and audit actions of break-glass (F124 US4, MODERATION §6.1, §9): ids only — never the reason in an event and
// never message content anywhere (R14, F124-AC-22).
const (
	EventBreakGlassRequested  = "break_glass.requested"
	AuditBreakGlassRequested  = "break_glass.requested"
	AuditBreakGlassApproved   = "break_glass.approved"
	AuditBreakGlassRejected   = "break_glass.rejected"
	AuditBreakGlassRevoked    = "break_glass.revoked"
	AuditBreakGlassExpired    = "break_glass.expired"
	AuditBreakGlassViewed     = "break_glass.messages_viewed"
	BreakGlassExpirerJob      = "break-glass-expirer"
	maxBreakGlassDecisionNote = 1000
)

// BreakGlassMetrics counts the transitions of the grants.
type BreakGlassMetrics interface{ Grant(status string) }

// BreakGlassInput is BreakGlassRequestCreate.
type BreakGlassInput = breakglass.Request

func grantOf(r read.GrantRow) breakglass.Grant {
	return breakglass.Grant{ID: r.ID, TenantID: r.TenantID, ScopeType: r.ScopeType, ScopeID: r.ScopeID, WindowFrom: r.WindowFrom,
		WindowTo: r.WindowTo, DurationMinutes: r.DurationMinutes, Status: r.Status, RequestedBy: r.RequestedBy, DecidedBy: r.DecidedBy,
		DecidedAt: r.DecidedAt, ExpiresAt: r.ExpiresAt, RevokedBy: r.RevokedBy, RevokedAt: r.RevokedAt}
}

func applyGrant(r *read.GrantRow, g breakglass.Grant) {
	r.Status, r.DecidedBy, r.DecidedAt, r.ExpiresAt, r.RevokedBy, r.RevokedAt = g.Status, g.DecidedBy, g.DecidedAt, g.ExpiresAt, g.RevokedBy, g.RevokedAt
}

// breakGlassWrite authorizes a write of break-glass for the target tenant of X-Tenant-Id: platform staff holding one of
// perms (safety_moderator, tenant roles and tokens without such a platform permission → 403 PERMISSION_DENIED; no MFA —
// ADR-0094).
func breakGlassWrite(ctx context.Context, perms ...authz.Permission) (authz.Principal, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, uuid.Nil, err
	}
	allowed := false
	for _, perm := range perms {
		allowed = allowed || p.Has(perm)
	}
	if !allowed || p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(perms[0]), "why": "break_glass"})
		return p, uuid.Nil, problem.New(problem.PermissionDenied, "")
	}
	if p.TenantID == nil {
		return p, uuid.Nil, problem.New(problem.TenantHeaderRequired, "")
	}
	return p, *p.TenantID, nil
}

func (h Safety) grantMetric(status string) {
	if h.BreakGlassMetrics != nil {
		h.BreakGlassMetrics.Grant(status)
	}
}

func (h Safety) grantAudit(ctx context.Context, action string, g read.GrantRow, reason string, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["tenant_id"], meta["scope_type"], meta["status"] = g.TenantID.String(), g.ScopeType, g.Status
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "break_glass_grant", TargetID: g.ID.String(),
		Result: audit.ResultSuccess, Reason: reason, Metadata: meta})
}

// RequestBreakGlass implements createBreakGlassRequest (F124-AC-17, AC-21): validates the limits (422), stores a pending
// request of the target tenant, audits break_glass.requested (with the reason, staff audit only) and publishes
// break_glass.requested (platform event, ids only) so module identity emails the other approvers.
func (h Safety) RequestBreakGlass(ctx context.Context, in BreakGlassInput) (out read.GrantRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RequestBreakGlass")
	defer func() { done(err) }()
	p, tenant, err := breakGlassWrite(ctx, authz.PermPlatformBreakGlassRequest)
	if err != nil {
		return out, err
	}
	if h.Grants == nil {
		return out, problem.New(problem.Unavailable, "break-glass is not wired").Retry()
	}
	in.WindowFrom, in.WindowTo = in.WindowFrom.UTC(), in.WindowTo.UTC()
	if err := in.Validate(h.BreakGlassLimits); err != nil {
		var fe breakglass.FieldError
		if errors.As(err, &fe) {
			return out, problem.Invalid(fe.Field, problem.SubInvalid, fe.Reason)
		}
		return out, err
	}
	now := h.now()
	out = read.GrantRow{ID: ids.New(), TenantID: tenant, ScopeType: in.ScopeType, ScopeID: in.ScopeID, WindowFrom: in.WindowFrom,
		WindowTo: in.WindowTo, DurationMinutes: in.DurationMinutes, Reason: in.Reason, Status: breakglass.StatusPending,
		RequestedBy: p.UserID, RequestedAt: now, Version: 1}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Grants.InsertGrant(ctx, out); err != nil {
			return err
		}
		if err := h.grantAudit(ctx, AuditBreakGlassRequested, out, in.Reason, map[string]any{"scope_id": in.ScopeID.String(),
			"window_from": in.WindowFrom.Format(time.RFC3339), "window_to": in.WindowTo.Format(time.RFC3339),
			"duration_minutes": in.DurationMinutes}); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventBreakGlassRequested, SubjectType: "break_glass_grant", SubjectID: out.ID.String(),
			PartitionKey: out.ID.String(), Platform: true, OccurredAt: now, Payload: map[string]any{"grant_id": out.ID, "tenant_id": tenant,
				"requested_by": p.UserID, "requested_at": now}}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, service.GrantBody(out))
	})
	if err == nil {
		h.grantMetric(breakglass.StatusPending)
	}
	return out, err
}

// DecideBreakGlass implements approveBreakGlassRequest / rejectBreakGlassRequest (F124-AC-18): a holder of
// platform.break_glass.approve other than the requester (self decision → 403 PERMISSION_DENIED, audited), If-Match on
// the version (412), pending only (409 BREAK_GLASS_NOT_PENDING); approval sets expires_at = now + duration.
func (h Safety) DecideBreakGlass(ctx context.Context, id uuid.UUID, version int64, approve bool, note *string) (out read.GrantRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.DecideBreakGlass")
	defer func() { done(err) }()
	p, tenant, err := breakGlassWrite(ctx, authz.PermPlatformBreakGlassApprove)
	if err != nil {
		return out, err
	}
	if h.Grants == nil {
		return out, problem.New(problem.Unavailable, "break-glass is not wired").Retry()
	}
	if note != nil && len([]rune(*note)) > maxBreakGlassDecisionNote {
		return out, problem.Invalid("decision_note", problem.SubMaxLength, "at most 1000 characters")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		r, err := h.Grants.LockGrant(ctx, tenant, id)
		if err != nil {
			return err
		}
		if r == nil {
			return notFound()
		}
		g, err := grantOf(*r).Decide(p.UserID, approve, now)
		switch {
		case errors.Is(err, breakglass.ErrSelfDecision):
			authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(authz.PermPlatformBreakGlassApprove),
				"why": "two_person_rule", "grant_id": id.String()})
			return problem.New(problem.PermissionDenied, "")
		case errors.Is(err, breakglass.ErrNotPending):
			return problem.New(problem.BreakGlassNotPending, "")
		case err != nil:
			return err
		}
		if r.Version != version {
			return versionMismatch()
		}
		applyGrant(r, g)
		r.DecisionNote = note
		r.Version++
		if err := h.Grants.UpdateGrant(ctx, *r, &p.UserID); err != nil {
			if errors.Is(err, write.ErrStale) {
				return versionMismatch()
			}
			return err
		}
		action := AuditBreakGlassRejected
		if approve {
			action = AuditBreakGlassApproved
		}
		meta := map[string]any{"requested_by": r.RequestedBy.String()}
		if r.ExpiresAt != nil {
			meta["expires_at"] = r.ExpiresAt.Format(time.RFC3339)
		}
		reason := ""
		if note != nil {
			reason = *note
		}
		if err := h.grantAudit(ctx, action, *r, reason, meta); err != nil {
			return err
		}
		out = *r
		return idempotency.SetResult(ctx, service.GrantBody(out))
	})
	if err == nil {
		h.grantMetric(out.Status)
	}
	return out, err
}

// RevokeBreakGlass implements revokeBreakGlassGrant (F124-AC-20): a holder of platform.break_glass.request or .approve
// ends a pending or approved grant at once (revoked); an ended grant → 409 BREAK_GLASS_NOT_PENDING.
func (h Safety) RevokeBreakGlass(ctx context.Context, id uuid.UUID, reason string) (out read.GrantRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RevokeBreakGlass")
	defer func() { done(err) }()
	p, tenant, err := breakGlassWrite(ctx, authz.PermPlatformBreakGlassRequest, authz.PermPlatformBreakGlassApprove)
	if err != nil {
		return out, err
	}
	if h.Grants == nil {
		return out, problem.New(problem.Unavailable, "break-glass is not wired").Retry()
	}
	if n := len([]rune(reason)); n < 3 || n > maxBreakGlassDecisionNote {
		return out, problem.Invalid("reason", problem.SubInvalid, "3 to 1000 characters")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		r, err := h.Grants.LockGrant(ctx, tenant, id)
		if err != nil {
			return err
		}
		if r == nil {
			return notFound()
		}
		g, err := grantOf(*r).Revoke(p.UserID, now)
		if errors.Is(err, breakglass.ErrNotPending) {
			return problem.New(problem.BreakGlassNotPending, "")
		}
		if err != nil {
			return err
		}
		applyGrant(r, g)
		rr := reason
		r.RevokeReason = &rr
		r.Version++
		if err := h.Grants.UpdateGrant(ctx, *r, &p.UserID); err != nil {
			return err
		}
		if err := h.grantAudit(ctx, AuditBreakGlassRevoked, *r, reason, map[string]any{"requested_by": r.RequestedBy.String()}); err != nil {
			return err
		}
		out = *r
		return idempotency.SetResult(ctx, service.GrantBody(out))
	})
	if err == nil {
		h.grantMetric(breakglass.StatusRevoked)
	}
	return out, err
}

// RecordBreakGlassView records one read of the messages of a grant (F124-AC-19, AC-20): messages_viewed_count += n and
// the audit break_glass.messages_viewed (counts and ids only, never content) in the scope of the grant's tenant. The
// presentation calls it after query.ListBreakGlassMessages and fails the request when it fails (no unaudited read).
func (h Safety) RecordBreakGlassView(ctx context.Context, g read.GrantRow, n int) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RecordBreakGlassView")
	defer func() { done(err) }()
	if h.Grants == nil {
		return problem.New(problem.Unavailable, "break-glass is not wired").Retry()
	}
	sc, err := authz.EventScope(g.TenantID)
	if err != nil {
		return err
	}
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		if err := h.Grants.AddViewed(ctx, g.TenantID, g.ID, n); err != nil {
			return err
		}
		return h.grantAudit(ctx, AuditBreakGlassViewed, g, "", map[string]any{"messages": n, "scope_id": g.ScopeID.String(),
			"bypass_reason": "break_glass"})
	})
}

// ExpireBreakGlass is one run of the job break-glass-expirer (F124-AC-20): an approved grant past expires_at becomes
// expired (audit). Reads already stop at expires_at (ReadableBy); this records the end. Work list under
// tenancy.Bypass(platform_job); each grant in the scope of its tenant.
func (h Safety) ExpireBreakGlass(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ExpireBreakGlass")
	defer func() { done(err) }()
	if h.Grants == nil {
		return 0, nil
	}
	if ok, err := h.lease(ctx, BreakGlassExpirerJob); err != nil || !ok {
		return 0, err
	}
	now := h.now()
	var due []read.GrantRow
	if err := h.platformRead(ctx, func(ctx context.Context) error {
		var e error
		due, e = h.Grants.ExpiredGrants(ctx, now, 500)
		return e
	}); err != nil {
		return 0, err
	}
	for _, d := range due {
		sc, err := authz.EventScope(d.TenantID)
		if err != nil {
			continue
		}
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			r, err := h.Grants.LockGrant(ctx, d.TenantID, d.ID)
			if err != nil || r == nil || !grantOf(*r).ExpiredAt(now) {
				return err
			}
			r.Status = breakglass.StatusExpired
			r.Version++
			if err := h.Grants.UpdateGrant(ctx, *r, nil); err != nil {
				return err
			}
			n++
			return h.grantAudit(ctx, AuditBreakGlassExpired, *r, "", map[string]any{"messages_viewed": r.MessagesViewedCount})
		})
		if err != nil {
			return n, err
		}
		h.grantMetric(breakglass.StatusExpired)
	}
	return n, nil
}
