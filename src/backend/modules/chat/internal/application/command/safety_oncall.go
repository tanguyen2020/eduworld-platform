package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/oncall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events and audit actions of the on-call rotation (F124 US3, EVENT_DESIGN §4.9): ids and levels only (R14).
const (
	EventOncallPaged        = "moderation.oncall_paged"
	EventSchoolReminded     = "moderation.school_notification_reminded"
	AuditOncallShiftCreated = "moderation.oncall_shift.created"
	AuditOncallShiftDeleted = "moderation.oncall_shift.deleted"
	AuditOncallPaged        = "moderation.oncall.paged"
	actionOncallPage        = "oncall_page"
	viaRotation             = "rotation"
)

// OncallMetrics are the metrics of the rotation (levels and roles only).
type OncallMetrics interface {
	Paged(level string)
	Coverage(role string, covered bool)
}

// OncallConfig holds oncall_shift_max_hours and the ladder (p1_unclaimed_alert_minutes, first response of report_sla).
type OncallConfig struct {
	ShiftMax time.Duration
	Ladder   oncall.Ladder
}

// DefaultOncallLadder is the ladder of the registered parameters (MODERATION §6.3, CONFIG_DEFAULTS).
func DefaultOncallLadder() oncall.Ladder {
	return oncall.Ladder{Unclaimed: safety.P1UnclaimedAlert, FirstResponse: safety.SLAOf(safety.P1).FirstResponse}
}

// ShiftInput is OncallShiftCreate.
type ShiftInput struct {
	Moderator uuid.UUID
	Role      string
	Start     time.Time
	End       time.Time
	Note      *string
}

// leadWrite authorizes a write of the rotation: moderation.case.lead (or System Admin), platform staff; platform data
// (no tenant) in the platform scope of the caller.
func leadWrite(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if err := authz.Require(ctx, p, authz.PermModerationCaseLead, authz.Resource{}); err != nil {
		return p, err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(authz.PermModerationCaseLead),
			"why": "platform_role"})
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

func (h Safety) platformScope(ctx context.Context, p authz.Principal) (context.Context, error) {
	d := p.PrincipalData
	d.TenantID = nil
	sc, err := authz.PlatformScope(authz.NewPrincipal(d))
	if err != nil {
		return ctx, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return tenancy.WithScope(ctx, sc), nil
}

// CreateOncallShift implements createModerationOncallShift (F124-AC-12): the on-call moderator is platform staff holding
// moderation.case.act (resolved through the identity facade before the transaction, fail closed), the window follows the
// rules (oncall_shift_max_hours), no live shift of the same role overlaps (advisory lock per role → 409
// ONCALL_SHIFT_OVERLAP); audit moderation.oncall_shift.created.
func (h Safety) CreateOncallShift(ctx context.Context, in ShiftInput) (out read.OncallShiftRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateOncallShift")
	defer func() { done(err) }()
	p, err := leadWrite(ctx)
	if err != nil {
		return out, err
	}
	if h.Oncall == nil || h.Principals == nil {
		return out, problem.New(problem.Unavailable, "the on-call rotation is not wired").Retry()
	}
	s := oncall.Shift{Moderator: in.Moderator, Role: in.Role, Start: in.Start.UTC(), End: in.End.UTC()}
	if err := s.Validate(h.OncallCfg.ShiftMax); err != nil {
		return out, problem.Invalid("ends_at", problem.SubInvalid, err.Error())
	}
	if in.Note != nil && len([]rune(*in.Note)) > 500 {
		return out, problem.Invalid("note", problem.SubMaxLength, "at most 500 characters")
	}
	mod, err := h.Principals.Platform(ctx, in.Moderator)
	switch {
	case errors.Is(err, port.ErrPrincipalUnknown):
		return out, problem.Invalid("moderator_user_id", problem.SubInvalid, "not a platform staff member")
	case err != nil:
		return out, problem.New(problem.Unavailable, "identity unavailable").Wrap(err).Retry()
	case mod.UserType != authz.UserTypeStaff || !mod.HasPlatformRole() || !mod.Has(authz.PermModerationCaseAct):
		return out, problem.Invalid("moderator_user_id", problem.SubInvalid, "needs moderation.case.act")
	}
	ctx, err = h.platformScope(ctx, p)
	if err != nil {
		return out, err
	}
	now := h.now()
	by := p.UserID
	out = read.OncallShiftRow{ID: ids.New(), Moderator: in.Moderator, Role: s.Role, Start: s.Start, End: s.End, Note: in.Note,
		CreatedBy: &by, CreatedAt: now, Version: 1}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Oncall.LockRole(ctx, s.Role); err != nil {
			return err
		}
		overlap, err := h.Oncall.Overlapping(ctx, s.Role, s.Start, s.End)
		if err != nil {
			return err
		}
		if overlap {
			return problem.New(problem.OncallShiftOverlap, "")
		}
		if err := h.Oncall.InsertShift(ctx, out); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditOncallShiftCreated, TargetType: "moderation_oncall_shift",
			TargetID: out.ID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"oncall_role": s.Role,
				"moderator_user_id": in.Moderator.String(), "starts_at": s.Start.Format(time.RFC3339), "ends_at": s.End.Format(time.RFC3339)}}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, service.ShiftBody(out))
	})
	return out, err
}

// DeleteOncallShift implements deleteModerationOncallShift (F124-AC-12): soft-delete (R21) + audit. A p1 case paged to
// the deleted shift stays open and the escalator pages the next level (F124 Edge Cases).
func (h Safety) DeleteOncallShift(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.DeleteOncallShift")
	defer func() { done(err) }()
	p, err := leadWrite(ctx)
	if err != nil {
		return err
	}
	if h.Oncall == nil {
		return problem.New(problem.Unavailable, "the on-call rotation is not wired").Retry()
	}
	if ctx, err = h.platformScope(ctx, p); err != nil {
		return err
	}
	now := h.now()
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Oncall.LockShift(ctx, id)
		if err != nil {
			return err
		}
		if s == nil {
			return notFound()
		}
		if err := h.Oncall.SoftDeleteShift(ctx, id, p.UserID, now); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditOncallShiftDeleted, TargetType: "moderation_oncall_shift", TargetID: id.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"oncall_role": s.Role, "moderator_user_id": s.Moderator.String()}}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, nil)
	})
}

// rotate pages the on-call moderator of a new (or newly p1) unclaimed case in the transaction of ctx (F124-AC-13): the
// case is assigned with assigned_via = rotation and stays open until the moderator claims it (assignModerationCase).
func (h Safety) rotate(ctx context.Context, c *read.CaseRow, now time.Time) error {
	_, err := h.rotated(ctx, c, now)
	return err
}

// rotated is rotate reporting whether the case changed (the caller persists it).
func (h Safety) rotated(ctx context.Context, c *read.CaseRow, now time.Time) (bool, error) {
	if h.Oncall == nil || c.Severity != safety.P1 || c.Status != safety.StatusOpen || c.Assignee != nil || c.OncallPageLevel != nil {
		return false, nil
	}
	return h.page(ctx, c, now)
}

// page pages the next due level of an open p1 case (F124-AC-13..AC-15): the current shift of primary/secondary (its
// moderator becomes the rotation assignee, so the paged moderator can claim the case), the team lead or the System
// Admin (resolved by module identity); an empty level pages the next one. It records the level on the case (the
// caller persists c), one oncall_page action, moderation.oncall_paged (platform event: the recipients are platform
// staff) and its audit. It reports whether a level was paged.
func (h Safety) page(ctx context.Context, c *read.CaseRow, now time.Time) (bool, error) {
	shifts := map[string]*read.OncallShiftRow{}
	var lookup error
	has := func(level string) bool {
		role := oncall.RoleOf(level)
		if role == "" {
			return true
		}
		if s, ok := shifts[role]; ok {
			return s != nil
		}
		s, err := h.Oncall.CurrentShift(ctx, role, now)
		if err != nil && lookup == nil {
			lookup = err
		}
		shifts[role] = s
		return s != nil
	}
	last := ""
	if c.OncallPageLevel != nil {
		last = *c.OncallPageLevel
	}
	ladder := h.OncallCfg.Ladder
	if ladder.Unclaimed <= 0 {
		ladder = DefaultOncallLadder()
	}
	level, ok := ladder.Next(last, c.CreatedAt, now, has)
	if lookup != nil {
		return false, lookup
	}
	if !ok {
		return false, nil
	}
	var moderator *uuid.UUID
	if role := oncall.RoleOf(level); role != "" {
		id := shifts[role].Moderator
		moderator = &id
		if c.Assignee == nil || *c.Assignee != id {
			via := viaRotation
			c.Assignee, c.AssignedVia = &id, &via
			if err := h.action(ctx, c, safety.ActionAssign, nil, "system", nil, nil, nil, now); err != nil {
				return false, err
			}
			if err := h.publish(ctx, EventCaseAssigned, "moderation_case", c.ID, c.TenantID, c.ID.String(), now, map[string]any{
				"case_id": c.ID, "tenant_id": c.TenantID, "severity": c.Severity, "assignee_moderator_id": id, "assigned_via": via,
				"assigned_at": now}); err != nil {
				return false, err
			}
			if err := h.record(ctx, AuditCaseAssigned, "moderation_case", c.ID, nil, map[string]any{"tenant_id": c.TenantID.String(),
				"assignee_moderator_id": id.String(), "assigned_via": via}); err != nil {
				return false, err
			}
		}
	}
	t, lv := now, level
	c.OncallPageLevel, c.LastPagedAt = &lv, &t
	if err := h.action(ctx, c, actionOncallPage, nil, "system", nil, nil, nil, now); err != nil {
		return false, err
	}
	if err := h.Events.Publish(ctx, write.Event{Type: EventOncallPaged, SubjectType: "moderation_case", SubjectID: c.ID.String(),
		PartitionKey: c.ID.String(), Platform: true, OccurredAt: now, Payload: map[string]any{"case_id": c.ID, "tenant_id": c.TenantID,
			"severity": c.Severity, "level": level, "moderator_user_id": uuidOrNil(moderator), "paged_at": now}}); err != nil {
		return false, err
	}
	meta := map[string]any{"tenant_id": c.TenantID.String(), "level": level}
	if moderator != nil {
		meta["moderator_user_id"] = moderator.String()
	}
	if err := h.record(ctx, AuditOncallPaged, "moderation_case", c.ID, nil, meta); err != nil {
		return false, err
	}
	if h.OncallMetrics != nil {
		lvl := level
		if h.AfterCommit != nil {
			if err := h.AfterCommit(ctx, func(context.Context) { h.OncallMetrics.Paged(lvl) }); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

// EscalateUnclaimedP1 is one run of the job oncall-escalator (F124-AC-14, AC-15): every open p1 case pages its next due
// level once (secondary at 1×, lead at 2× p1_unclaimed_alert_minutes, System Admin past the first response of
// report_sla); a claimed case (in_review) is never paged again. It refreshes eduworld_moderation_oncall_coverage for the
// alert ModerationOncallGap. Work list under tenancy.Bypass(platform_job); each case in the scope of its tenant.
func (h Safety) EscalateUnclaimedP1(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.EscalateUnclaimedP1")
	defer func() { done(err) }()
	if h.Oncall == nil {
		return 0, nil
	}
	if ok, err := h.lease(ctx, "oncall-escalator"); err != nil || !ok {
		return 0, err
	}
	now := h.now()
	var open []read.CaseRow
	coverage := map[string]bool{}
	if err := h.platformRead(ctx, func(ctx context.Context) error {
		for _, role := range oncall.Roles {
			s, err := h.Oncall.CurrentShift(ctx, role, now)
			if err != nil {
				return err
			}
			coverage[role] = s != nil
		}
		var e error
		open, e = h.Oncall.UnclaimedP1(ctx, 500)
		return e
	}); err != nil {
		return 0, err
	}
	if h.OncallMetrics != nil {
		for _, role := range oncall.Roles {
			h.OncallMetrics.Coverage(role, coverage[role])
		}
	}
	for _, o := range open {
		sc, err := authz.EventScope(o.TenantID)
		if err != nil {
			continue
		}
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			c, err := h.Store.LockCase(ctx, o.TenantID, o.ID)
			if err != nil || c == nil || c.Status != safety.StatusOpen || c.Severity != safety.P1 {
				return err
			}
			paged, err := h.page(ctx, c, now)
			if err != nil || !paged {
				return err
			}
			c.Version++
			if err := h.Store.UpdateCase(ctx, *c, nil); err != nil {
				return err
			}
			n++
			return nil
		})
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
