package command

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CaseState is a case after a write with its history (ModerationCase / ModerationCaseDetail without content).
type CaseState struct {
	Case    read.CaseRow
	Actions []read.ActionRow
}

// ActInput is ModerationActionRequest.
type ActInput struct {
	Action          string
	TargetUserID    *uuid.UUID
	MessageID       *uuid.UUID
	SanctionType    *string
	DurationMinutes *int
	Note            string
}

// AssignCase implements assignModerationCase (F121-AC-08, MODERATION §6.2): a moderator claims a case (open, reopened)
// or a team lead assigns it (also escalated, in_review) → in_review, moderation.case_assigned; resolved → 409
// MODERATION_CASE_CLOSED. Writes run in the tenant of the case (X-Tenant-Id).
func (h Safety) AssignCase(ctx context.Context, id, assignee uuid.UUID) (out CaseState, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.AssignCase")
	defer func() { done(err) }()
	p, tenant, err := moderatorWrite(ctx, authz.PermModerationCaseAct)
	if err != nil {
		return out, err
	}
	now := h.now()
	var firstResponse *time.Duration
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.lockCase(ctx, tenant, id)
		if err != nil {
			return err
		}
		lead := isLead(p)
		if c.Assignee != nil && *c.Assignee != p.UserID && c.Status != safety.StatusResolved && !lead {
			return safety.ErrNeedsLead // taking the case of another moderator
		}
		status, err := safety.Assign(c.Status, assignee == p.UserID, lead)
		if err != nil {
			return err
		}
		via := "manual"
		c.Status, c.Assignee, c.AssignedVia = status, &assignee, &via
		if c.FirstResponseAt == nil {
			t := now
			c.FirstResponseAt = &t
			d := now.Sub(c.CreatedAt)
			firstResponse = &d
		}
		c.Version++
		if err := h.Store.UpdateCase(ctx, *c, &p.UserID); err != nil {
			return err
		}
		if err := h.action(ctx, c, safety.ActionAssign, &p.UserID, "moderator", nil, nil, nil, now); err != nil {
			return err
		}
		if err := h.publish(ctx, EventCaseAssigned, "moderation_case", c.ID, tenant, c.ID.String(), now, map[string]any{
			"case_id": c.ID, "tenant_id": tenant, "severity": c.Severity, "assignee_moderator_id": assignee, "assigned_via": via,
			"assigned_at": now}); err != nil {
			return err
		}
		if err := h.record(ctx, AuditCaseAssigned, "moderation_case", c.ID, nil, map[string]any{"tenant_id": tenant.String(),
			"assignee_moderator_id": assignee.String()}); err != nil {
			return err
		}
		out.Case = *c
		if out.Actions, err = h.Store.CaseActions(ctx, tenant, c.ID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, service.CaseBody(out.Case))
	})
	if err != nil {
		return out, h.finish(ctx, p, id, err)
	}
	if firstResponse != nil && h.Metrics != nil {
		h.Metrics.FirstResponse(out.Case.Severity, *firstResponse)
	}
	return out, nil
}

// ActOnCase implements actOnModerationCase (F121-AC-11, AC-12, MODERATION §6.2, §7): the assignee or a team lead acts
// with If-Match (version) — warn/mute/restrict apply a sanction to a student of the case, remove_message removes a
// message of the case channel (placeholder for users, original kept as evidence), escalate, dismiss/close resolve the
// case (holds released, reporters notified), reopen reopens a resolved case. Every action is appended to the history
// with its audit; a resolved case accepts only reopen (409 MODERATION_CASE_CLOSED); a stale version is 412.
func (h Safety) ActOnCase(ctx context.Context, id uuid.UUID, version int64, in ActInput) (out CaseState, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ActOnCase")
	defer func() { done(err) }()
	p, tenant, err := moderatorWrite(ctx, authz.PermModerationCaseAct)
	if err != nil {
		return out, err
	}
	if !slices.Contains(safety.ModeratorActions, in.Action) {
		return out, problem.Invalid("action", problem.SubInvalid, "unknown action")
	}
	if n := len([]rune(in.Note)); n < 3 || n > 1000 {
		return out, problem.Invalid("note", problem.SubInvalid, "3 to 1000 characters")
	}
	now := h.now()
	var applied []write.SafetySanction
	var lifted []uuid.UUID
	var removed *write.ReportedMessage
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.lockCase(ctx, tenant, id)
		if err != nil {
			return err
		}
		status, err := safety.Act(c.Status, in.Action)
		if err != nil {
			return err
		}
		if !safety.CanRead(c.Assignee, p.UserID, isLead(p)) {
			return safety.ErrNotAssignee
		}
		if c.Version != version {
			return write.ErrStale
		}
		note := in.Note
		var sanctionID, messageID *uuid.UUID
		switch in.Action {
		case safety.ActionWarn, safety.ActionMute, safety.ActionRestrict:
			s, err := h.caseSanction(ctx, p, c, in, now)
			if err != nil {
				return err
			}
			applied = append(applied, s)
			sid := s.ID
			sanctionID = &sid
		case safety.ActionRemoveMessage:
			if in.MessageID == nil {
				return problem.Invalid("message_id", problem.SubRequired, "required for remove_message")
			}
			m, err := h.Store.Message(ctx, tenant, *in.MessageID)
			if err != nil {
				return err
			}
			if m == nil || c.ChannelID == nil || m.ChannelID != *c.ChannelID {
				return problem.Invalid("message_id", problem.SubInvalid, "not a message of the case channel")
			}
			ok, err := h.Store.RemoveMessage(ctx, tenant, m.ID, p.UserID, now)
			if err != nil {
				return err
			}
			if ok {
				removed = m
				if err := h.publish(ctx, EventMessageRemoved, "chat_message", m.ID, tenant, m.ChannelID.String(), now, map[string]any{
					"tenant_id": tenant, "message_id": m.ID, "channel_id": m.ChannelID, "by": p.UserID, "case_id": c.ID,
					"removed_at": now}); err != nil {
					return err
				}
				if err := h.record(ctx, AuditMessageRemoved, "chat_message", m.ID, m.SchoolID, map[string]any{"case_id": c.ID.String(),
					"channel_id": m.ChannelID.String()}); err != nil {
					return err
				}
			}
			mid := m.ID
			messageID = &mid
		case safety.ActionClose, safety.ActionDismiss:
			if lifted, err = h.resolve(ctx, c, in.Action, p.UserID, now); err != nil {
				return err
			}
		case safety.ActionReopen:
			c.ResolvedAt = nil
			if err := h.Store.SetReportStatus(ctx, tenant, c.ID, "open"); err != nil {
				return err
			}
		}
		c.Status = status
		if c.FirstResponseAt == nil {
			t := now
			c.FirstResponseAt = &t
		}
		c.Version++
		if err := h.Store.UpdateCase(ctx, *c, &p.UserID); err != nil {
			return err
		}
		if err := h.action(ctx, c, in.Action, &p.UserID, "moderator", &note, sanctionID, messageID, now); err != nil {
			return err
		}
		meta := map[string]any{"tenant_id": tenant.String(), "action": in.Action, "status": c.Status}
		if sanctionID != nil {
			meta["sanction_id"] = sanctionID.String()
		}
		if messageID != nil {
			meta["message_id"] = messageID.String()
		}
		if err := h.record(ctx, AuditCaseActed, "moderation_case", c.ID, nil, meta); err != nil {
			return err
		}
		for i := range applied {
			if err := h.later(ctx, applied[i].UserID, &applied[i]); err != nil {
				return err
			}
		}
		for _, u := range lifted {
			if err := h.later(ctx, u, nil); err != nil {
				return err
			}
		}
		if removed != nil && h.AfterCommit != nil {
			m := *removed
			if err := h.AfterCommit(ctx, func(ctx context.Context) {
				bg := context.WithoutCancel(ctx)
				if h.Recent != nil {
					_ = h.Recent.Drop(bg, m.ChannelID)
				}
				if h.Push != nil {
					_ = h.Push.MessageRemoved(bg, m.ChannelID, m.ID)
				}
			}); err != nil {
				return err
			}
		}
		out.Case = *c
		if out.Actions, err = h.Store.CaseActions(ctx, tenant, c.ID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, service.CaseStateBody(out.Case, out.Actions))
	})
	if err != nil {
		return out, h.finish(ctx, p, id, err)
	}
	return out, nil
}

// caseSanction applies the sanction of warn/mute/restrict to a student of the case (MODERATION §7).
func (h Safety) caseSanction(ctx context.Context, p authz.Principal, c *read.CaseRow, in ActInput, now time.Time) (write.SafetySanction, error) {
	typ := safety.SanctionWarning
	switch in.Action {
	case safety.ActionMute:
		typ = safety.SanctionMute
	case safety.ActionRestrict:
		if in.SanctionType == nil || !slices.Contains(safety.RestrictTypes, *in.SanctionType) {
			return write.SafetySanction{}, problem.Invalid("sanction_type", problem.SubRequired, "required for restrict")
		}
		typ = *in.SanctionType
	}
	user := uuid.Nil
	if in.TargetUserID != nil {
		user = *in.TargetUserID
	} else if len(c.Subjects) > 0 {
		user = c.Subjects[0]
	}
	if !slices.Contains(c.Subjects, user) {
		return write.SafetySanction{}, problem.Invalid("target_user_id", problem.SubInvalid, "not a student of the case")
	}
	caseID := c.ID
	return h.newSanction(ctx, p, c.TenantID, user, &caseID, typ, in.DurationMinutes, nil, in.Note, now)
}

// newSanction validates and applies a sanction of the Safety Team: student of the tenant (404 otherwise), default
// duration of §7, team-lead rule (403), scope channel for mute only.
func (h Safety) newSanction(ctx context.Context, p authz.Principal, tenant, user uuid.UUID, caseID *uuid.UUID, typ string,
	minutes *int, channel *uuid.UUID, reason string, now time.Time) (write.SafetySanction, error) {
	if !slices.Contains(safety.SanctionTypes, typ) {
		return write.SafetySanction{}, problem.Invalid("sanction_type", problem.SubInvalid, "unknown sanction type")
	}
	st, err := h.Store.Student(ctx, tenant, user)
	if err != nil {
		return write.SafetySanction{}, err
	}
	if st == nil || st.Deleted {
		return write.SafetySanction{}, notFound()
	}
	if channel != nil && typ != safety.SanctionMute {
		return write.SafetySanction{}, problem.Invalid("scope.channel_id", problem.SubInvalid, "only a mute is limited to a channel")
	}
	var end *time.Time
	var d time.Duration
	if minutes != nil {
		d = time.Duration(*minutes) * time.Minute
	} else if def, ok := safety.DefaultDuration(typ); ok {
		d = def
	}
	if d > 0 && typ != safety.SanctionWarning {
		e := now.Add(d)
		end = &e
	}
	if safety.RequiresLead(typ, d, end != nil) && !isLead(p) {
		return write.SafetySanction{}, safety.ErrNeedsLead
	}
	scope := "all"
	if channel != nil {
		scope = "channel"
	}
	school := st.SchoolID
	issuer := p.UserID
	s := write.SafetySanction{ID: ids.New(), TenantID: tenant, SchoolID: &school, UserID: user, Type: typ, Scope: scope, ChannelID: channel,
		Reason: reason, ReasonCode: safety.CodeModeration, CaseID: caseID, IssuedByKind: "moderator", IssuedBy: &issuer, StartsAt: now,
		EndsAt: end}
	return s, h.applySanction(ctx, s)
}

// resolve closes a case: releases its holds (sanction.lifted case_resolved, F121-AC-04), marks the reports
// (dismissed/linked) and tells each reporter "your report was handled" without any detail (report.resolved,
// F121-AC-16). It returns the users whose sanction cache must be cleared.
func (h Safety) resolve(ctx context.Context, c *read.CaseRow, action string, actor uuid.UUID, now time.Time) ([]uuid.UUID, error) {
	t := now
	c.ResolvedAt = &t
	var users []uuid.UUID
	sanctions, err := h.Store.CaseSanctions(ctx, c.TenantID, c.ID)
	if err != nil {
		return nil, err
	}
	for _, s := range sanctions {
		if !safety.Holds(s.ReasonCode) || s.RevokedAt != nil || s.LiftedAt != nil {
			continue
		}
		if err := h.liftSanction(ctx, s, &actor, reasonResolvedReleaseHold, liftCaseResolved, now); err != nil {
			return nil, err
		}
		sid := s.ID
		if err := h.action(ctx, c, safety.ActionReleaseHold, nil, "system", nil, &sid, nil, now); err != nil {
			return nil, err
		}
		users = append(users, s.UserID)
	}
	status := "linked"
	if action == safety.ActionDismiss {
		status = "dismissed"
	}
	if err := h.Store.SetReportStatus(ctx, c.TenantID, c.ID, status); err != nil {
		return nil, err
	}
	reports, err := h.Store.CaseReports(ctx, c.TenantID, c.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range reports {
		if err := h.publish(ctx, EventReportResolved, "chat_report", r.ID, c.TenantID, r.ReporterID.String(), now, map[string]any{
			"tenant_id": c.TenantID, "report_id": r.ID, "case_id": c.ID, "reporter_id": r.ReporterID, "kind": "moderation_notice",
			"message_code": "moderation.report_handled", "resolved_at": now}); err != nil {
			return nil, err
		}
	}
	return users, nil
}
