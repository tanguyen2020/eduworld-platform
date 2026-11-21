package command

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Keys of the school settings JSON written through updateSchoolSettings.
// class_leaderboard_enabled is gone with the classes (ADR-0095 D5).
var schoolKeys = map[string]bool{"allowed_hours": true, "school_leaderboard_enabled": true,
	"student_club_creation": true, "curfew_window": true, settings.KeyDailyPlayLimit: true, settings.KeyEnabledSubjects: true}

// CurrentPlatformCurfew returns platform_curfew in force (configured value or CONFIG_DEFAULTS).
func (h Settings) CurrentPlatformCurfew(ctx context.Context) (settings.Curfew, error) {
	s, err := h.Repo.Platform(ctx, settings.KeyPlatformCurfew)
	if err != nil {
		return settings.Curfew{}, err
	}
	c, err := h.PlatformCurfew(s)
	return settings.Curfew{Start: c.Start, End: c.End}, err
}

// UpdateSchoolSettings implements updateSchoolSettings (F015-AC-16, AC-17, AC-19, AC-22, AC-30): school.settings.write
// on that school (another school → 403, another tenant → 404 through RLS); the access hours only tighten the
// tenant hours; the curfew window must contain platform_curfew and needs a reason; school.updated and, for the
// curfew, platform_settings.changed (scope school) are published with the audit in the same transaction.
func (h Settings) UpdateSchoolSettings(ctx context.Context, schoolID uuid.UUID, patch map[string]json.RawMessage, reason string, expected int64) (out dto.SchoolSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateSchoolSettings")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID == nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermSchoolSettingsWrite, authz.Resource{TenantID: p.TenantID, SchoolID: &schoolID}); err != nil {
		return out, err
	}
	for k := range patch {
		if !schoolKeys[k] {
			return out, problem.Invalid(k, problem.SubInvalid, "not a school setting")
		}
	}
	_, curfewChanged := patch["curfew_window"]
	_, limitChanged := patch[settings.KeyDailyPlayLimit]
	if curfewChanged || limitChanged { // F015-AC-22, AC-30: audited with a reason
		if err := checkReason(reason); err != nil {
			return out, err
		}
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	tenant := *p.TenantID
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		cur, owner, found, err := h.Repo.School(ctx, schoolID)
		if err != nil {
			return err
		}
		if !found || owner != tenant {
			return problem.New(problem.NotFound, "")
		}
		if cur.Version != expected {
			return problem.New(problem.VersionMismatch, "")
		}
		platformCurfew, err := h.CurrentPlatformCurfew(ctx)
		if err != nil {
			return err
		}
		if err := h.checkSchoolPatch(ctx, tenant, patch, platformCurfew); err != nil {
			return err
		}
		before, err := dto.SchoolSettingsOf(schoolID, cur.Value, cur.Version, cur.UpdatedAt, platformCurfew)
		if err != nil {
			return err
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
		changed := make([]string, 0, len(patch))
		for k, v := range patch {
			if isNull(v) && k != "curfew_window" {
				delete(m, k)
			} else {
				m[k] = v
			}
			changed = append(changed, "settings."+k)
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return err
		}
		v, err := h.Repo.PutSchool(ctx, schoolID, raw, expected, p.UserID)
		if err != nil {
			return err
		}
		if out, err = dto.SchoolSettingsOf(schoolID, raw, v, nil, platformCurfew); err != nil {
			return err
		}
		subset := schoolSubset(out)
		if err := h.Events.Publish(ctx, write.Event{Type: "school.updated", SubjectType: "school", SubjectID: schoolID.String(), TenantID: &tenant,
			Payload: map[string]any{"school_id": schoolID, "tenant_id": tenant, "status": cur.Status, "settings_subset": subset,
				"changed_fields": changed, "source_version": v}}); err != nil {
			return err
		}
		if curfewChanged {
			if err := h.Events.Publish(ctx, write.Event{Type: "platform_settings.changed", SubjectType: "school", SubjectID: schoolID.String(),
				TenantID: &tenant, Payload: map[string]any{"key": "curfew_window", "scope": "school", "tenant_id": tenant, "school_id": schoolID,
					"flag_key": nil, "before": curfewMap(before.CurfewWindow), "after": curfewMap(out.CurfewWindow),
					"curfew_window": out.CurfewWindow, "reason": reason, "actor_id": p.UserID}}); err != nil {
				return err
			}
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditSchoolSettingsUpdated, TargetType: "school", TargetID: schoolID.String(),
			SchoolID: schoolID.String(), Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"before": auditValue(before),
				"after": auditValue(out), "changed_fields": strings.Join(changed, ",")}})
	})
	return out, err
}

// schoolSubset is settings_subset of school.updated: the school settings the other modules project.
func schoolSubset(out dto.SchoolSettings) map[string]any {
	subset := map[string]any{"allowed_hours": out.AllowedHours, "curfew_window": out.CurfewWindow, "student_club_creation": out.StudentClubCreation,
		settings.KeyDailyPlayLimit: out.DailyPlayLimitMinutes} // module game enforces it (F014-AC-27)
	if out.SchoolLeaderboardEnabled != nil {
		subset["school_leaderboard_enabled"] = *out.SchoolLeaderboardEnabled
	}
	if out.EnabledSubjectCodes != nil { // absent = every subject on (module game F071-AC-03, F015 T054)
		subset[settings.KeyEnabledSubjects] = out.EnabledSubjectCodes
	}
	return subset
}

func curfewMap(c *settings.Curfew) map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{"start": c.Start, "end": c.End}
}

// checkSchoolPatch: values valid; hours within the tenant hours (AC-19); curfew contains platform_curfew (AC-22).
func (h Settings) checkSchoolPatch(ctx context.Context, tenant uuid.UUID, patch map[string]json.RawMessage, platformCurfew settings.Curfew) error {
	if raw, ok := patch["allowed_hours"]; ok && !isNull(raw) {
		var a settings.AllowedHours
		if err := json.Unmarshal(raw, &a); err != nil {
			return problem.Invalid("allowed_hours", problem.SubInvalid, "invalid")
		}
		if err := a.Validate("allowed_hours"); err != nil {
			return settingsInvalid(err)
		}
		ts, found, err := h.Repo.Tenant(ctx, tenant)
		if err != nil {
			return err
		}
		if found {
			t, err := dto.TenantSettingsOf(ts.Value, ts.Version, settings.StudentCleanup{})
			if err != nil {
				return err
			}
			if !a.Within(t.AllowedHours) {
				return problem.Invalid("allowed_hours", problem.SubInvalid, "a school may only tighten the tenant access hours")
			}
		}
	}
	for _, k := range []string{"school_leaderboard_enabled", "student_club_creation"} {
		if raw, ok := patch[k]; ok && !isNull(raw) {
			var b bool
			if err := json.Unmarshal(raw, &b); err != nil {
				return problem.Invalid(k, problem.SubInvalid, "boolean")
			}
		}
	}
	if raw, ok := patch[settings.KeyDailyPlayLimit]; ok && !isNull(raw) {
		var minutes int
		if err := json.Unmarshal(raw, &minutes); err != nil {
			return problem.Invalid(settings.KeyDailyPlayLimit, problem.SubInvalid, "integer minutes or null")
		}
		if err := settings.ValidateDailyPlayLimit(minutes); err != nil {
			return settingsInvalid(err)
		}
	}
	if raw, ok := patch[settings.KeyEnabledSubjects]; ok && !isNull(raw) {
		var codes []string
		if err := json.Unmarshal(raw, &codes); err != nil {
			return problem.Invalid(settings.KeyEnabledSubjects, problem.SubInvalid, "array of subject codes or null")
		}
		if err := settings.ValidateEnabledSubjects(codes); err != nil {
			return settingsInvalid(err)
		}
	}
	if raw, ok := patch["curfew_window"]; ok && !isNull(raw) {
		var c settings.Curfew
		if err := json.Unmarshal(raw, &c); err != nil {
			return problem.Invalid("curfew_window", problem.SubInvalid, "start/end HH:MM")
		}
		if err := c.Validate("curfew_window"); err != nil {
			return settingsInvalid(err)
		}
		if !c.Contains(platformCurfew) {
			return problem.Invalid("curfew_window", problem.SubInvalid, "the school curfew must contain platform_curfew "+
				platformCurfew.Start+"-"+platformCurfew.End)
		}
	}
	return nil
}
