package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Source scopes of TenantModerationStatus.
const (
	StatusSourcePlatform = "platform"
	StatusSourceTenant   = "tenant"
)

// ModerationStatus is TenantModerationStatus (getTenantModerationStatus, F122-AC-13/14): the switchable steps not in
// force for the tenant of the session and the earliest time they come back on. Never a reason, an actor or a pending
// request (ADR-0030): any staff member of the tenant may see the banner.
type ModerationStatus struct {
	EnabledAll    bool
	DisabledSteps []string
	ExpiresAt     *time.Time
	SourceScope   string // "" when every step runs
}

// TenantModerationStatus implements getTenantModerationStatus: tenant staff holding tenant.settings.read or
// school.settings.read in the session tenant (403 otherwise); the switches are the stored platform and tenant levels
// resolved at now (a lapsed expires_at counts as on even before the expirer job ran).
func (q Catalog) TenantModerationStatus(ctx context.Context, now time.Time) (out ModerationStatus, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetTenantModerationStatus")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID == nil {
		return out, problem.New(problem.TenantHeaderRequired, "")
	}
	tenant := *p.TenantID
	perm := authz.PermSchoolSettingsRead
	if p.Has(authz.PermTenantSettingsRead) {
		perm = authz.PermTenantSettingsRead
	}
	if err := authz.Require(ctx, p, perm, statusResource(p, perm, tenant)); err != nil {
		return out, err
	}
	var platformSw, tenantSw *read.SwitchRow
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		platformSw, tenantSw, err = q.Moderation.Switches(ctx, tenant)
		return err
	})
	if err != nil {
		return out, err
	}
	return ResolveModerationStatus(service.ToSwitch(platformSw), tenantSw, now), nil
}

// statusResource is the resource TenantModerationStatus authorizes: the session tenant, within the limits of a grant
// holding perm. The status is tenant-wide (steps off, earliest expiry — no school data, no reason, ADR-0030), so a grant
// limited to one school of the tenant is enough: the School Admin of a school sees the banner (F122-AC-13, F122
// Clarifications 2026-11-28). A tenant-wide grant is preferred; without any grant holding perm Require answers 403.
func statusResource(p authz.Principal, perm authz.Permission, tenant uuid.UUID) authz.Resource {
	res := authz.Resource{TenantID: &tenant}
	found := false
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, perm) {
			continue
		}
		if g.SchoolID == nil && g.SubjectID == nil {
			return authz.Resource{TenantID: &tenant}
		}
		if !found {
			res.SchoolID, res.SubjectID, found = g.SchoolID, g.SubjectID, true
		}
	}
	return res
}

// ResolveModerationStatus combines the platform switch and the tenant row (nil: inherits the platform) at now.
func ResolveModerationStatus(platform moderation.Switch, tenantRow *read.SwitchRow, now time.Time) ModerationStatus {
	e := moderation.Effective{Platform: platform, At: now}
	var tenant *moderation.Switch
	if tenantRow != nil {
		t := service.ToSwitch(tenantRow)
		tenant, e.Tenant = &t, &t
	}
	out := ModerationStatus{EnabledAll: e.AllEnabled(), DisabledSteps: e.Disabled()}
	if out.DisabledSteps == nil {
		out.DisabledSteps = []string{}
	}
	if out.EnabledAll && len(out.DisabledSteps) == 0 {
		return out
	}
	off := func(s moderation.Switch) bool {
		if !s.AllEnabled(now) {
			return true
		}
		for _, st := range moderation.SwitchableSteps {
			if !s.On(st, now) {
				return true
			}
		}
		return false
	}
	for _, lv := range []struct {
		sw    *moderation.Switch
		scope string
	}{{&platform, StatusSourcePlatform}, {tenant, StatusSourceTenant}} {
		if lv.sw == nil || !off(*lv.sw) {
			continue
		}
		out.SourceScope = lv.scope // the tenant level wins when both disable something
		if x := lv.sw.ExpiresAt; x != nil && (out.ExpiresAt == nil || x.Before(*out.ExpiresAt)) {
			t := x.UTC()
			out.ExpiresAt = &t
		}
	}
	return out
}
