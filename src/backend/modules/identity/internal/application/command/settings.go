package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the settings (F015 US3/US4).
const (
	AuditPlatformSettingsUpdated = "platform_settings.updated"
	AuditTenantSettingsUpdated   = "tenant.settings_updated"
	AuditSchoolSettingsUpdated   = "school.settings_updated"
)

// Settings implements the settings use cases of F015 (platform: student cleanup, chat retention, curfew;
// tenant settings; school settings in school_settings.go). Every write needs If-Match and (platform) a
// reason, records an audit with before/after and publishes platform_settings.changed (and tenant.updated /
// school.updated) in the same transaction.
type Settings struct {
	UoW      write.UnitOfWork
	Repo     write.SettingsRepository
	Events   write.EventPublisher
	Audit    write.Auditor
	Defaults settings.Defaults
}

// StudentCleanup returns student_cleanup (defaults when not configured).
func (h Settings) StudentCleanup(s write.StoredSetting) (dto.StudentCleanupSettings, error) {
	return dto.StudentCleanupOf(h.Defaults, s.Value, s.Version)
}

// ChatRetention returns chat_retention (defaults when not configured).
func (h Settings) ChatRetention(s write.StoredSetting) (dto.ChatRetentionSettings, error) {
	return dto.ChatRetentionOf(h.Defaults, s.Value, s.Version)
}

// PlatformCurfew returns platform_curfew (CONFIG_DEFAULTS value with is_default when not configured).
func (h Settings) PlatformCurfew(s write.StoredSetting) (dto.PlatformCurfewSettings, error) {
	return dto.PlatformCurfewOf(h.Defaults, s.Value, s.Version, s.UpdatedAt)
}

func platformWriter(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if p.TenantID != nil {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, authz.Require(ctx, p, authz.PermPlatformSettingsWrite, authz.Resource{})
}

func checkReason(reason string) error {
	if n := len(strings.TrimSpace(reason)); n < 3 || n > 500 {
		return problem.Invalid("reason", problem.SubInvalid, "3..500 characters")
	}
	return nil
}

func settingsInvalid(err error) error {
	var ie settings.ErrInvalid
	switch {
	case errors.As(err, &ie):
		return problem.Invalid(ie.Field, problem.SubInvalid, ie.Reason)
	case errors.Is(err, settings.ErrRetentionBelowMinimum):
		return problem.New(problem.RetentionBelowMinimum, "")
	}
	return err
}

// auditValue renders a settings value for audit metadata (strings ≤ 255).
func auditValue(v any) string {
	b, _ := json.Marshal(v)
	if len(b) > 255 {
		b = b[:255]
	}
	return string(b)
}

// putPlatform writes one platform key with audit and platform_settings.changed.
func (h Settings) putPlatform(ctx context.Context, key string, value any, reason string, expected int64, curfew *settings.Curfew,
	before func(write.StoredSetting) (any, error)) (int64, error) {
	p, err := platformWriter(ctx)
	if err != nil {
		return 0, err
	}
	if err := checkReason(reason); err != nil {
		return 0, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	sc, err := authz.SystemPlatformScope("platform-settings")
	if err != nil {
		return 0, err
	}
	var version int64
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		cur, err := h.Repo.Platform(ctx, key)
		if err != nil {
			return err
		}
		if cur.Version != expected {
			return problem.New(problem.VersionMismatch, "")
		}
		old, err := before(cur)
		if err != nil {
			return err
		}
		if version, err = h.Repo.PutPlatform(ctx, key, raw, expected, p.UserID); err != nil {
			return err
		}
		payload := map[string]any{"key": key, "scope": "platform", "tenant_id": nil, "school_id": nil, "flag_key": nil,
			"before": old, "after": value, "reason": reason, "actor_id": p.UserID}
		if curfew != nil {
			payload["curfew_window"] = curfew
		} else {
			payload["curfew_window"] = nil
		}
		if err := h.Events.Publish(ctx, write.Event{Type: "platform_settings.changed", SubjectType: "platform_setting", SubjectID: key,
			Platform: true, Payload: payload}); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditPlatformSettingsUpdated, TargetType: "platform_setting", TargetID: key,
			Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"before": auditValue(old), "after": auditValue(value)}})
	})
	return version, err
}

// PutStudentCleanup implements putStudentCleanupSettings (F015-AC-11).
func (h Settings) PutStudentCleanup(ctx context.Context, v settings.StudentCleanup, reason string, expected int64) (out dto.StudentCleanupSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.PutStudentCleanupSettings")
	defer func() { done(err) }()
	if err := v.Validate(); err != nil {
		return out, settingsInvalid(err)
	}
	ver, err := h.putPlatform(ctx, settings.KeyStudentCleanup, v, reason, expected, nil, func(s write.StoredSetting) (any, error) {
		o, err := h.StudentCleanup(s)
		return o.StudentCleanup, err
	})
	return dto.StudentCleanupSettings{StudentCleanup: v, Version: ver}, err
}

// PutChatRetention implements putChatRetentionSettings (F015-AC-11, AC-12).
func (h Settings) PutChatRetention(ctx context.Context, v settings.ChatRetention, reason string, expected int64) (out dto.ChatRetentionSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.PutChatRetentionSettings")
	defer func() { done(err) }()
	if err := v.Validate(h.Defaults.ChatMinimumDays); err != nil {
		return out, settingsInvalid(err)
	}
	ver, err := h.putPlatform(ctx, settings.KeyChatRetention, v, reason, expected, nil, func(s write.StoredSetting) (any, error) {
		o, err := h.ChatRetention(s)
		return o.ChatRetention, err
	})
	return dto.ChatRetentionSettings{ChatRetention: v, MinimumDays: h.Defaults.ChatMinimumDays, Version: ver}, err
}

// PutPlatformCurfew implements putPlatformCurfewSettings (F015-AC-21).
func (h Settings) PutPlatformCurfew(ctx context.Context, c settings.Curfew, reason string, expected int64) (out dto.PlatformCurfewSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.PutPlatformCurfewSettings")
	defer func() { done(err) }()
	if err := c.Validate("start"); err != nil {
		return out, settingsInvalid(err)
	}
	ver, err := h.putPlatform(ctx, settings.KeyPlatformCurfew, c, reason, expected, &c, func(s write.StoredSetting) (any, error) {
		o, err := h.PlatformCurfew(s)
		return settings.Curfew{Start: o.Start, End: o.End}, err
	})
	if err != nil {
		return out, err
	}
	return dto.PlatformCurfewSettings{Start: c.Start, End: c.End, TimeZone: h.Defaults.TimeZone, WarningMinutes: h.Defaults.CurfewWarningMinutes,
		Version: ver}, nil
}

// Keys of the tenant settings JSON written through updateTenantSettings.
var tenantKeys = map[string]bool{"allowed_hours": true, "inactive_student_cleanup_days": true, "gameplay_chat_retention_days": true,
	"default_locale": true, settings.KeyCrossSchoolSocial: true}

// TenantSettingsOf decodes the stored tenant settings and computes the effective cleanup days.
func (h Settings) TenantSettingsOf(ctx context.Context, s write.StoredSetting) (dto.TenantSettings, error) {
	ps, err := h.Repo.Platform(ctx, settings.KeyStudentCleanup)
	if err != nil {
		return dto.TenantSettings{}, err
	}
	pc, err := h.StudentCleanup(ps)
	if err != nil {
		return dto.TenantSettings{}, err
	}
	return dto.TenantSettingsOf(s.Value, s.Version, pc.StudentCleanup)
}

func isNull(raw json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

// UpdateTenantSettings implements updateTenantSettings (merge patch, F015-AC-13..AC-15): only the tenant of
// the session; cleanup days need tenant.student_cleanup.configure and a platform allowing overrides; chat
// retention needs a platform allowing overrides and at least the minimum.
func (h Settings) UpdateTenantSettings(ctx context.Context, patch map[string]json.RawMessage, expected int64) (out dto.TenantSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateTenantSettings")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID == nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermTenantSettingsWrite, authz.Resource{TenantID: p.TenantID}); err != nil {
		return out, err
	}
	for k := range patch {
		if !tenantKeys[k] {
			return out, problem.Invalid(k, problem.SubInvalid, "not a tenant setting")
		}
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	tenant := *p.TenantID
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		cur, found, err := h.Repo.Tenant(ctx, tenant)
		if err != nil {
			return err
		}
		if !found {
			return problem.New(problem.NotFound, "")
		}
		if cur.Version != expected {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := h.checkTenantPatch(ctx, p, patch); err != nil {
			return err
		}
		if raw, ok := patch[settings.KeyCrossSchoolSocial]; ok {
			if err := settings.ValidateCrossSchoolSocial(cur.TenantType, raw); err != nil {
				return settingsInvalid(err)
			}
		}
		var m map[string]json.RawMessage
		if len(cur.Value) > 0 {
			if err := json.Unmarshal(cur.Value, &m); err != nil {
				return err
			}
		}
		if m == nil {
			m = map[string]json.RawMessage{}
		}
		before, _ := h.TenantSettingsOf(ctx, cur)
		changed := make([]string, 0, len(patch))
		for k, v := range patch {
			if isNull(v) && k != "inactive_student_cleanup_days" {
				delete(m, k)
			} else {
				m[k] = v
			}
			changed = append(changed, k)
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return err
		}
		v, err := h.Repo.PutTenant(ctx, tenant, raw, expected, p.UserID)
		if err != nil {
			return err
		}
		out, err = h.TenantSettingsOf(ctx, write.StoredSetting{Value: raw, Version: v})
		if err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: "tenant.updated", SubjectType: "tenant", SubjectID: tenant.String(),
			TenantID: &tenant, Payload: map[string]any{"tenant_id": tenant, "tenant_type": cur.TenantType, "status": cur.Status,
				"changed_fields": prefixed("settings.", changed), "settings_subset": settings.TenantSubset(raw), "source_version": v}}); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: "platform_settings.changed", SubjectType: "tenant", SubjectID: tenant.String(),
			TenantID: &tenant, Payload: map[string]any{"key": "tenant_settings", "scope": "tenant", "tenant_id": tenant, "school_id": nil,
				"flag_key": nil, "before": before, "after": out, "reason": nil, "actor_id": p.UserID, "curfew_window": nil}}); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditTenantSettingsUpdated, TargetType: "tenant", TargetID: tenant.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"before": auditValue(before), "after": auditValue(out),
				"changed_fields": strings.Join(changed, ",")}})
	})
	return out, err
}

func prefixed(prefix string, xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = prefix + x
	}
	return out
}

// checkTenantPatch validates the patched values against the platform limits (only tighter, ADR-0020).
func (h Settings) checkTenantPatch(ctx context.Context, p authz.Principal, patch map[string]json.RawMessage) error {
	if raw, ok := patch["allowed_hours"]; ok && !isNull(raw) {
		var a settings.AllowedHours
		if err := json.Unmarshal(raw, &a); err != nil {
			return problem.Invalid("allowed_hours", problem.SubInvalid, "invalid")
		}
		if err := a.Validate("allowed_hours"); err != nil {
			return settingsInvalid(err)
		}
	}
	if raw, ok := patch["inactive_student_cleanup_days"]; ok {
		ps, err := h.Repo.Platform(ctx, settings.KeyStudentCleanup)
		if err != nil {
			return err
		}
		pc, err := h.StudentCleanup(ps)
		if err != nil {
			return err
		}
		if !pc.AllowTenantOverride || !p.Has(authz.PermTenantStudentCleanupConfigure) {
			return problem.New(problem.PermissionDenied, "")
		}
		var d *int
		if err := json.Unmarshal(raw, &d); err != nil || (d != nil && (*d < 0 || *d > 3650)) {
			return problem.Invalid("inactive_student_cleanup_days", problem.SubInvalid, "0..3650 or null")
		}
	}
	if raw, ok := patch["gameplay_chat_retention_days"]; ok && !isNull(raw) {
		ps, err := h.Repo.Platform(ctx, settings.KeyChatRetention)
		if err != nil {
			return err
		}
		pc, err := h.ChatRetention(ps)
		if err != nil {
			return err
		}
		if !pc.AllowTenantOverride {
			return problem.New(problem.PermissionDenied, "")
		}
		var d int
		if err := json.Unmarshal(raw, &d); err != nil {
			return problem.Invalid("gameplay_chat_retention_days", problem.SubInvalid, "integer")
		}
		if d < h.Defaults.ChatMinimumDays {
			return problem.New(problem.RetentionBelowMinimum, "")
		}
	}
	if raw, ok := patch["default_locale"]; ok && !isNull(raw) {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || len(s) > 16 {
			return problem.Invalid("default_locale", problem.SubInvalid, "locale")
		}
	}
	return nil
}
