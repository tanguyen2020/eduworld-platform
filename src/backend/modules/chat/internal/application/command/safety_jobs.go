package command

import (
	"context"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// SafetyTick is the period of the Safety Team jobs (SLA escalation, reminders, sanction expiry; ≤ 60 s so an expired
// sanction is lifted within a minute, F121-AC-15).
const SafetyTick = 30 * time.Second

// SafetyGauges publishes the queue gauges (eduworld_moderation_cases_open, oldest unclaimed age, overdue notifications).
type SafetyGauges interface {
	Cases(counts []read.OpenCount, now time.Time)
	OverdueNotifications(n int)
}

// EscalateOverdueCases is one run of the job moderation-sla-escalator (F121-AC-22, AC-23, MODERATION §6.3): a case past
// its first-response deadline without response, or past its resolution deadline, becomes escalated once (system action,
// audit; the team lead sees it in the queue) — never an automatic long sanction. It also refreshes the queue gauges for
// the alert ModerationP1Unclaimed. Work list under tenancy.Bypass(platform_job); each case in the scope of its tenant.
func (h Safety) EscalateOverdueCases(ctx context.Context, gauges SafetyGauges, counts func(ctx context.Context) ([]read.OpenCount, int, error)) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.EscalateOverdueCases")
	defer func() { done(err) }()
	if ok, err := h.lease(ctx, "moderation-sla-escalator"); err != nil || !ok {
		return 0, err
	}
	now := h.now()
	var overdue []read.CaseRow
	if err := h.platformRead(ctx, func(ctx context.Context) error {
		var e error
		overdue, e = h.Store.OverdueCases(ctx, now, 500)
		return e
	}); err != nil {
		return 0, err
	}
	for _, o := range overdue {
		sc, err := authz.EventScope(o.TenantID)
		if err != nil {
			continue
		}
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			c, err := h.Store.LockCase(ctx, o.TenantID, o.ID)
			if err != nil || c == nil || c.EscalatedAt != nil || c.Status == safety.StatusResolved || c.Status == safety.StatusEscalate {
				return err
			}
			late := (c.FirstResponseAt == nil && !now.Before(c.FirstResponseDue)) || !now.Before(c.SLADue)
			if !late {
				return nil
			}
			t := now
			c.Status, c.EscalatedAt = safety.StatusEscalate, &t
			c.Version++
			if err := h.Store.UpdateCase(ctx, *c, nil); err != nil {
				return err
			}
			if err := h.action(ctx, c, safety.ActionSystemEscalate, nil, "system", nil, nil, nil, now); err != nil {
				return err
			}
			n++
			return h.record(ctx, AuditCaseEscalated, "moderation_case", c.ID, nil, map[string]any{"tenant_id": c.TenantID.String(),
				"severity": c.Severity, "reason": "sla"})
		})
		if err != nil {
			return n, err
		}
	}
	if gauges != nil && counts != nil {
		list, overdueN, err := counts(ctx)
		if err != nil {
			return n, err
		}
		gauges.Cases(list, now)
		gauges.OverdueNotifications(overdueN)
	}
	return n, nil
}
