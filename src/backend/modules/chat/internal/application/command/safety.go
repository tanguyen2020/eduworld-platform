package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events of the Safety Team (F121, EVENT_DESIGN §4.9): ids and codes only — never a note, a summary or message content
// (R14, F121-AC-25).
const (
	EventSanctionLifted       = "sanction.lifted"
	EventReportCreated        = "report.created"
	EventReportResolved       = "report.resolved"
	EventCaseAssigned         = "moderation.case_assigned"
	EventSchoolNotified       = "moderation.school_notified"
	EventSchoolAcknowledged   = "moderation.school_acknowledged"
	EventMessageRemoved       = "chat.message_removed"
	AuditReportCreated        = "moderation.report.created"
	AuditCaseOpened           = "moderation.case.opened"
	AuditCaseAssigned         = "moderation.case.assigned"
	AuditCaseActed            = "moderation.case.acted"
	AuditCaseEscalated        = "moderation.case.escalated"
	AuditModSanctionApplied   = "moderation.sanction.applied"
	AuditModSanctionRevoked   = "moderation.sanction.revoked"
	AuditSchoolNotified       = "moderation.school_notification.sent"
	AuditSchoolAcknowledged   = "moderation.school_notification.acknowledged"
	AuditSchoolReminded       = "moderation.school_notification.reminded"
	AuditMessageRemoved       = "moderation.message.removed"
	liftRevoked               = "revoked"
	liftExpired               = "expired"
	liftCaseResolved          = "case_resolved"
	reasonCaseHold            = "case_hold"
	reasonResolvedReleaseHold = "case_resolved"
	reasonSelfHarmSupport     = "self_harm_support"
)

// SafetyPush pushes the realtime messages of the Safety Team (best effort, after the commit).
type SafetyPush interface {
	SanctionApplied(ctx context.Context, user uuid.UUID, payload map[string]any) error
	MessageRemoved(ctx context.Context, channel, message uuid.UUID) error
}

// RecentDropper drops the recent buffer of a channel (a removed message must not be served from Valkey).
type RecentDropper interface {
	Drop(ctx context.Context, channel uuid.UUID) error
}

// SafetyMetrics are the metrics of the Safety Team (labels without user or content, R14).
type SafetyMetrics interface {
	Report(reason string)
	FirstResponse(severity string, d time.Duration)
}

// Safety are the write use cases of the Platform Safety Team (F121): reports of students, cases opened by pipeline
// signals, assignment, actions, sanctions, school notifications and the jobs (SLA, reminders, expiry). Writes of the
// Safety Team always carry the target tenant (X-Tenant-Id = tenant of the case/student, AGENTS R20); each use case
// authorizes first, then runs in one UnitOfWork with its outbox events and audit (ids only, R14).
type Safety struct {
	UoW     write.UnitOfWork
	Store   write.SafetyStore
	Events  write.EventPublisher
	Audit   write.Auditor
	Cache   port.SanctionCache
	Push    SafetyPush    // nil: none
	Recent  RecentDropper // nil: none
	Metrics SafetyMetrics // nil: none
	Lock    port.JobLock  // nil: no lease (tests)
	// AfterCommit runs the cache invalidation and realtime pushes after the commit of the transaction of ctx (also the
	// transaction of a consumer when the use case runs nested in it).
	AfterCommit write.AfterCommit
	Now         func() time.Time
}

func (h Safety) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// moderatorWrite authorizes a write of the Safety Team for the target tenant of X-Tenant-Id (platform role, staff).
func moderatorWrite(ctx context.Context, perm authz.Permission) (authz.Principal, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, uuid.Nil, err
	}
	if p.TenantID == nil {
		if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
			return p, uuid.Nil, err
		}
		return p, uuid.Nil, problem.New(problem.TenantHeaderRequired, "")
	}
	tenant := *p.TenantID
	if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: &tenant}); err != nil {
		return p, uuid.Nil, err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(perm), "why": "platform_role"})
		return p, uuid.Nil, problem.New(problem.PermissionDenied, "")
	}
	return p, tenant, nil
}

// isLead reports a team lead (moderation.case.lead) or a System Admin.
func isLead(p authz.Principal) bool {
	return p.IsSystemAdmin || p.Has(authz.PermModerationCaseLead)
}

// denyLead audits and returns the 403 of a rule reserved to the team lead.
func denyLead(ctx context.Context, p authz.Principal, why string) error {
	authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(authz.PermModerationCaseLead),
		"why": why, "user_id": p.UserID.String()})
	return problem.New(problem.PermissionDenied, "")
}

// errNotInTenant is returned inside a transaction when the case is unknown in the tenant of X-Tenant-Id; after the
// transaction resolveMissing tells TENANT_MISMATCH (the case exists in another tenant) from 404.
var errNotInTenant = errors.New("chat: case not in this tenant")

// caseProblem maps the domain errors of the case state machine to the contract.
func caseProblem(ctx context.Context, p authz.Principal, err error) error {
	switch {
	case errors.Is(err, safety.ErrClosed):
		return problem.New(problem.ModerationCaseClosed, "")
	case errors.Is(err, safety.ErrTransition):
		return problem.New(problem.Conflict, "the action is not allowed in the current status of the case")
	case errors.Is(err, safety.ErrNeedsLead):
		return denyLead(ctx, p, "team_lead_required")
	case errors.Is(err, safety.ErrNotAssignee):
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "not_assignee", "user_id": p.UserID.String()})
		return problem.New(problem.PermissionDenied, "")
	case errors.Is(err, write.ErrStale):
		return versionMismatch()
	}
	return err
}

// lockCase loads the live case of tenant for update (errNotInTenant when unknown there).
func (h Safety) lockCase(ctx context.Context, tenant, id uuid.UUID) (*read.CaseRow, error) {
	c, err := h.Store.LockCase(ctx, tenant, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, errNotInTenant
	}
	return c, nil
}

// resolveMissing answers a case unknown in the tenant of the request: TENANT_MISMATCH when it belongs to another
// tenant (looked up through tenancy.Bypass(moderation_case), audited and counted), 404 otherwise.
func (h Safety) resolveMissing(ctx context.Context, id uuid.UUID) error {
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonModerationCase)
	if err != nil {
		return err
	}
	var tenant *uuid.UUID
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var e error
		tenant, e = h.Store.CaseTenant(ctx, id)
		return e
	}); err != nil {
		return err
	}
	if tenant != nil {
		authz.RecordDenied(ctx, string(problem.TenantMismatch), map[string]any{"why": "case_tenant", "case_id": id.String()})
		return problem.New(problem.TenantMismatch, "")
	}
	return notFound()
}

// finish maps the error of a case use case after its transaction.
func (h Safety) finish(ctx context.Context, p authz.Principal, id uuid.UUID, err error) error {
	if errors.Is(err, errNotInTenant) {
		return h.resolveMissing(ctx, id)
	}
	return caseProblem(ctx, p, err)
}

func (h Safety) record(ctx context.Context, action, targetType string, target uuid.UUID, school *uuid.UUID, meta map[string]any) error {
	e := audit.Event{Action: action, TargetType: targetType, TargetID: target.String(), Result: audit.ResultSuccess, Metadata: meta}
	if school != nil {
		e.SchoolID = school.String()
	}
	return h.Audit.Record(ctx, e)
}

func (h Safety) publish(ctx context.Context, typ, subjectType string, subject, tenant uuid.UUID, key string, at time.Time, payload map[string]any) error {
	t := tenant
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: subjectType, SubjectID: subject.String(), PartitionKey: key,
		TenantID: &t, OccurredAt: at, Payload: payload})
}

func (h Safety) action(ctx context.Context, c *read.CaseRow, action string, actor *uuid.UUID, kind string, note *string,
	sanction, msg *uuid.UUID, at time.Time) error {
	return h.Store.InsertAction(ctx, write.NewAction{TenantID: c.TenantID, CaseID: c.ID, Action: action, ActorID: actor, ActorKind: kind,
		Note: note, SanctionID: sanction, MessageID: msg, At: at})
}

func timeOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

func uuidOrNil(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

// sanctionAppliedPayload is sanction.applied v1 (EVENT_DESIGN §4.9): ids and the reason code, never the reason.
func sanctionAppliedPayload(s write.SafetySanction) map[string]any {
	return map[string]any{"tenant_id": s.TenantID, "sanction_id": s.ID, "user_id": s.UserID, "case_id": uuidOrNil(s.CaseID),
		"sanction_type": s.Type, "scope": s.Scope, "channel_id": uuidOrNil(s.ChannelID), "reason_code": s.ReasonCode,
		"issued_by_kind": s.IssuedByKind, "issued_by": uuidOrNil(s.IssuedBy), "starts_at": s.StartsAt.UTC(), "until": timeOrNil(s.EndsAt),
		"source_version": s.StartsAt.UnixMicro()}
}

// sanctionLiftedPayload is sanction.lifted v1.
func sanctionLiftedPayload(s read.SanctionDetail, reason string, at time.Time) map[string]any {
	return map[string]any{"tenant_id": s.TenantID, "sanction_id": s.ID, "user_id": s.UserID, "case_id": uuidOrNil(s.CaseID),
		"sanction_type": s.Type, "lift_reason": reason, "until": timeOrNil(s.EndsAt), "lifted_at": at.UTC(), "source_version": at.UnixMicro()}
}

// applySanction inserts a sanction with sanction.applied and its audit in the transaction of ctx.
func (h Safety) applySanction(ctx context.Context, s write.SafetySanction) error {
	if err := h.Store.InsertSanction(ctx, s); err != nil {
		return err
	}
	if err := h.publish(ctx, EventSanctionApplied, "chat_sanction", s.ID, s.TenantID, s.UserID.String(), s.StartsAt,
		sanctionAppliedPayload(s)); err != nil {
		return err
	}
	meta := map[string]any{"sanction_id": s.ID.String(), "type": s.Type, "scope": s.Scope, "reason_code": s.ReasonCode,
		"issued_by_kind": s.IssuedByKind}
	if s.CaseID != nil {
		meta["case_id"] = s.CaseID.String()
	}
	return h.record(ctx, AuditModSanctionApplied, "student", s.UserID, s.SchoolID, meta)
}

// liftSanction ends an active sanction (revoked, expired or released with its case) with sanction.lifted.
func (h Safety) liftSanction(ctx context.Context, s read.SanctionDetail, by *uuid.UUID, reason, liftReason string, at time.Time) error {
	if liftReason == liftExpired {
		if err := h.Store.MarkLifted(ctx, s.TenantID, s.ID, at); err != nil {
			return err
		}
	} else if err := h.Store.Revoke(ctx, s.TenantID, s.ID, by, reason, at); err != nil {
		return err
	}
	if err := h.publish(ctx, EventSanctionLifted, "chat_sanction", s.ID, s.TenantID, s.UserID.String(), at,
		sanctionLiftedPayload(s, liftReason, at)); err != nil {
		return err
	}
	meta := map[string]any{"sanction_id": s.ID.String(), "type": s.Type, "lift_reason": liftReason}
	if s.CaseID != nil {
		meta["case_id"] = s.CaseID.String()
	}
	return h.record(ctx, AuditModSanctionRevoked, "student", s.UserID, s.SchoolID, meta)
}

// later schedules afterSanction after the commit of the transaction of ctx.
func (h Safety) later(ctx context.Context, user uuid.UUID, applied *write.SafetySanction) error {
	if h.AfterCommit == nil {
		return errors.New("chat: safety use cases need AfterCommit")
	}
	var copied *write.SafetySanction
	if applied != nil {
		c := *applied
		copied = &c
	}
	return h.AfterCommit(ctx, func(ctx context.Context) { h.afterSanction(ctx, user, copied) })
}

// afterSanction clears the cached sanctions of user and pushes chat.sanction.applied (best effort, after commit).
func (h Safety) afterSanction(ctx context.Context, user uuid.UUID, applied *write.SafetySanction) {
	bg := context.WithoutCancel(ctx)
	if h.Cache != nil {
		if err := h.Cache.Invalidate(bg, user); err != nil {
			slog.WarnContext(ctx, "chat: sanction cache not cleared", "error", err)
		}
	}
	if applied == nil || h.Push == nil {
		return
	}
	p := map[string]any{"sanction_type": applied.Type, "reason_code": applied.ReasonCode}
	if applied.EndsAt != nil {
		p["until"] = applied.EndsAt.UnixMilli()
	}
	if err := h.Push.SanctionApplied(bg, user, p); err != nil {
		slog.WarnContext(ctx, "chat: sanction not pushed", "error", err)
	}
}
