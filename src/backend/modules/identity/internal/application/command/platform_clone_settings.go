package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// errSettingsSource: the source of a settings clone is not a live tenant/school of the source tenant.
var errSettingsSource = errors.New("settings clone: the source is not a live tenant or school of the source tenant")

// cloneSettings is the identity step of a clone job (OQ F045-4): tenant_settings copies the settings of the source
// tenant onto the target tenant; school_settings copies the settings of one source school onto every live school of
// the target tenant, without the access hours when they are wider than the hours of the target tenant (a school
// may only tighten them, F015-AC-19). Each write bumps the row version, publishes tenant.updated /
// platform_settings.changed or school.updated (projections of the other modules) and the settings audit in the target
// tenant, in one transaction. Student data is never touched (R22).
func (d PlatformAdminDeps) cloneSettings(ctx context.Context, job write.CloneJob, rt string) (port.CloneStepResult, error) {
	if d.Settings == nil {
		return port.CloneStepResult{}, problem.New(problem.Unavailable, "").Retry().Wrap(errors.New("settings clone is not wired"))
	}
	var res port.CloneStepResult
	err := d.inSagaScope(ctx, func(ctx context.Context) error {
		var err error
		switch rt {
		case "tenant_settings":
			res, err = d.cloneTenantSettings(ctx, job)
		case "school_settings":
			res, err = d.cloneSchoolSettings(ctx, job)
		default:
			err = fmt.Errorf("settings clone: %q is not an identity settings type", rt)
		}
		if errors.Is(err, errSettingsSource) {
			res, err = port.CloneStepResult{Status: platformadmin.CloneFailed, Skipped: 1}, nil
		}
		return err
	})
	return res, err
}

// settingsAudit records the settings change made by a clone job in the target tenant (the job itself is audited
// tenant_data.cloned when it finishes): tenant.settings_updated or school.settings_updated, like the settings API.
func (d PlatformAdminDeps) settingsAudit(ctx context.Context, job write.CloneJob, action, targetType, targetID string, dropped []string) error {
	e := audit.Event{Action: action, TargetType: targetType, TargetID: targetID, Result: audit.ResultSuccess, Reason: "tenant_clone",
		Metadata: map[string]any{"job_id": job.ID.String(), "source_tenant_id": job.Source.String(), "dropped_keys": strings.Join(dropped, ",")}}
	if targetType == "school" {
		e.SchoolID = targetID
	}
	return auditIn(ctx, d.Audit, job.Target, e)
}

func (d PlatformAdminDeps) cloneTenantSettings(ctx context.Context, job write.CloneJob) (port.CloneStepResult, error) {
	src, found, err := d.Settings.Tenant(ctx, job.Source)
	if err != nil {
		return port.CloneStepResult{}, err
	}
	if !found {
		return port.CloneStepResult{}, errSettingsSource
	}
	dst, found, err := d.Settings.Tenant(ctx, job.Target)
	if err != nil || !found {
		return port.CloneStepResult{}, errors.Join(err, fmt.Errorf("settings clone: target tenant %s not found", job.Target))
	}
	raw := src.Value
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if _, err := d.Settings.PutTenant(ctx, job.Target, raw, dst.Version, job.RequestedBy); err != nil {
		return port.CloneStepResult{}, err
	}
	t := job.Target
	if err := d.Events.Publish(ctx, write.Event{Type: "tenant.updated", SubjectType: "tenant", SubjectID: t.String(), TenantID: &t,
		Payload: map[string]any{"tenant_id": t, "tenant_type": dst.TenantType, "status": dst.Status, "changed_fields": []string{"settings"}}}); err != nil {
		return port.CloneStepResult{}, err
	}
	var before, after map[string]any
	_ = json.Unmarshal(dst.Value, &before)
	_ = json.Unmarshal(raw, &after)
	if err := d.Events.Publish(ctx, write.Event{Type: "platform_settings.changed", SubjectType: "tenant", SubjectID: t.String(), TenantID: &t,
		Payload: map[string]any{"key": "tenant_settings", "scope": "tenant", "tenant_id": t, "school_id": nil, "flag_key": nil, "before": before,
			"after": after, "reason": nil, "actor_id": job.RequestedBy, "curfew_window": nil}}); err != nil {
		return port.CloneStepResult{}, err
	}
	cloned := []port.ClonedPair{{SourceID: job.Source, TargetID: job.Target}}
	if err := d.settingsAudit(ctx, job, AuditTenantSettingsUpdated, "tenant", t.String(), nil); err != nil {
		return port.CloneStepResult{}, err
	}
	return port.CloneStepResult{Status: platformadmin.CloneSucceeded, Cloned: cloned}, nil
}

func (d PlatformAdminDeps) cloneSchoolSettings(ctx context.Context, job write.CloneJob) (port.CloneStepResult, error) {
	if len(job.ResourceIDs) != 1 {
		return port.CloneStepResult{}, errSettingsSource
	}
	srcSchool := job.ResourceIDs[0]
	src, owner, found, err := d.Settings.School(ctx, srcSchool)
	if err != nil {
		return port.CloneStepResult{}, err
	}
	if !found || owner != job.Source {
		return port.CloneStepResult{}, errSettingsSource
	}
	var value map[string]json.RawMessage
	if len(src.Value) > 0 {
		if err := json.Unmarshal(src.Value, &value); err != nil {
			return port.CloneStepResult{}, err
		}
	}
	if value == nil {
		value = map[string]json.RawMessage{}
	}
	var dropped []string
	if raw, ok := value["allowed_hours"]; ok && !isNull(raw) {
		ts, found, err := d.Settings.Tenant(ctx, job.Target)
		if err != nil {
			return port.CloneStepResult{}, err
		}
		var hours settings.AllowedHours
		var tenantHours *settings.AllowedHours
		if found {
			t, err := dto.TenantSettingsOf(ts.Value, ts.Version, settings.StudentCleanup{})
			if err != nil {
				return port.CloneStepResult{}, err
			}
			tenantHours = t.AllowedHours
		}
		if json.Unmarshal(raw, &hours) != nil || !hours.Within(tenantHours) {
			delete(value, "allowed_hours")
			dropped = append(dropped, "allowed_hours")
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return port.CloneStepResult{}, err
	}
	targets, err := d.Repo.TenantSchools(ctx, job.Target)
	if err != nil {
		return port.CloneStepResult{}, err
	}
	t := job.Target
	cloned := make([]port.ClonedPair, 0, len(targets))
	for _, school := range targets {
		cur, _, found, err := d.Settings.School(ctx, school)
		if err != nil {
			return port.CloneStepResult{}, err
		}
		if !found {
			continue
		}
		v, err := d.Settings.PutSchool(ctx, school, raw, cur.Version, job.RequestedBy)
		if err != nil {
			return port.CloneStepResult{}, err
		}
		out, err := dto.SchoolSettingsOf(school, raw, v, nil, settings.Curfew{})
		if err != nil {
			return port.CloneStepResult{}, err
		}
		if err := d.Events.Publish(ctx, write.Event{Type: "school.updated", SubjectType: "school", SubjectID: school.String(), TenantID: &t,
			Payload: map[string]any{"school_id": school, "tenant_id": t, "status": cur.Status, "settings_subset": schoolSubset(out),
				"changed_fields": []string{"settings"}, "source_version": v}}); err != nil {
			return port.CloneStepResult{}, err
		}
		if err := d.settingsAudit(ctx, job, AuditSchoolSettingsUpdated, "school", school.String(), dropped); err != nil {
			return port.CloneStepResult{}, err
		}
		cloned = append(cloned, port.ClonedPair{SourceID: srcSchool, TargetID: school})
	}
	status := platformadmin.CloneSucceeded
	if len(cloned) == 0 {
		status = platformadmin.CloneFailed
	}
	return port.CloneStepResult{Status: status, Cloned: cloned}, nil
}
