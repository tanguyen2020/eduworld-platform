package service

import (
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
)

// The JSON shapes of the switch administration (ModerationSwitchState, ModerationSwitchRequest, ModerationSwitchChange
// of specs/openapi/eduworld-api.v1.yaml). They are built here because the commands store the same body as the
// idempotent response (idempotency.SetResult) that the presentation writes. pii and grooming never appear: they have
// no switch (ADR-0078).

// StepsBody renders ModerationSwitchSteps.
func StepsBody(on func(step string) bool) map[string]bool {
	out := map[string]bool{}
	for _, s := range moderation.SwitchableSteps {
		out[s] = on(s)
	}
	return out
}

func configuredOn(steps map[string]bool) func(string) bool {
	return func(s string) bool { v, ok := steps[s]; return !ok || v }
}

func utc(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

// StateBody renders ModerationSwitchState of one level: tenant nil = platform; configured nil = not configured
// (inherits); pending nil leaves pending_requests out.
func StateBody(tenant *uuid.UUID, configured *read.SwitchRow, e moderation.Effective, pending []read.SwitchRequestRow) map[string]any {
	scope := "platform"
	if tenant != nil {
		scope = "tenant"
	}
	disabled := e.Disabled()
	if disabled == nil {
		disabled = []string{}
	}
	m := map[string]any{"scope": scope, "tenant_id": tenant, "configured": nil,
		"effective": map[string]any{"enabled_all": e.AllEnabled(), "steps": StepsBody(e.On)}, "disabled_steps": disabled}
	if c := configured; c != nil {
		m["configured"] = map[string]any{"enabled_all": c.EnabledAll, "steps": StepsBody(configuredOn(c.Steps)), "expires_at": utc(c.ExpiresAt)}
		if c.UpdatedBy != nil {
			m["updated_by"] = *c.UpdatedBy
		}
		m["updated_at"] = c.UpdatedAt.UTC()
	}
	if pending != nil {
		list := make([]map[string]any, len(pending))
		for i := range pending {
			list[i] = RequestBody(pending[i])
		}
		m["pending_requests"] = list
	}
	return m
}

// RequestBody renders ModerationSwitchRequest.
func RequestBody(r read.SwitchRequestRow) map[string]any {
	scope := "platform"
	if r.TenantID != nil {
		scope = "tenant"
	}
	return map[string]any{"request_id": r.ID, "scope": scope, "tenant_id": r.TenantID, "enabled_all": r.EnabledAll,
		"steps": StepsBody(configuredOn(r.Steps)), "reason": r.Reason, "expires_at": r.ExpiresAt.UTC(), "status": r.Status,
		"proposed_by": r.ProposedBy, "approved_by": r.ApprovedBy, "created_at": r.CreatedAt.UTC(), "decided_at": utc(r.DecidedAt),
		"request_expires_at": r.RequestExpiresAt.UTC()}
}

// ChangeBody renders ModerationSwitchChange.
func ChangeBody(c read.SwitchChangeRow) map[string]any {
	scope := "platform"
	if c.TenantID != nil {
		scope = "tenant"
	}
	reason := ""
	if c.Reason != nil {
		reason = *c.Reason
	}
	return map[string]any{"change_id": c.ID, "scope": scope, "tenant_id": c.TenantID, "before": StepsBody(configuredOn(c.Before)),
		"after": StepsBody(configuredOn(c.After)), "enabled_all_before": c.EnabledAllBefore, "enabled_all_after": c.EnabledAllAfter,
		"reason": reason, "expires_at": utc(c.ExpiresAt), "actor_id": c.ActorID, "approved_by": c.ApprovedBy, "request_id": c.RequestID,
		"source": c.Source, "changed_at": c.ChangedAt.UTC()}
}

// EffectiveOf combines the configured rows of a level into the effective switches at now (tenant nil: the platform
// level alone).
func EffectiveOf(platform, tenant *read.SwitchRow, now time.Time) moderation.Effective {
	e := moderation.Effective{Platform: ToSwitch(platform), At: now}
	if tenant != nil {
		t := ToSwitch(tenant)
		e.Tenant = &t
	}
	return e
}
