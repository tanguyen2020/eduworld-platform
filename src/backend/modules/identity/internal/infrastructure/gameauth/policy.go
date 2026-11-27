package gameauth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SettingsPolicy implements port.CurfewSource, port.AccessHoursSource and port.DailyPlayLimitSource from the settings of F015: the
// effective curfew is platform_curfew united with the curfew_window of the school (ADR-0078, F015-AC-22; the
// common tenant's virtual school has none); the access hours are those of the school, else of the tenant.
type SettingsPolicy struct {
	Reader   read.SettingsReader
	PlayTime read.PlayTimeReader // OQ-17: time played per day (nil = no daily play limit)
	Defaults settings.Defaults
}

var (
	_ port.SocialSettingsSource = SettingsPolicy{}
	_ port.CurfewSource         = SettingsPolicy{}
	_ port.AccessHoursSource    = SettingsPolicy{}
	_ port.DailyPlayLimitSource = SettingsPolicy{}
)

// Location returns the platform time zone of the settings (Asia/Ho_Chi_Minh; fixed UTC+7 when unknown).
func (p SettingsPolicy) Location() *time.Location { return p.location() }

func (p SettingsPolicy) location() *time.Location {
	if loc, err := time.LoadLocation(p.Defaults.TimeZone); err == nil {
		return loc
	}
	return time.FixedZone("ICT", 7*3600)
}

func (p SettingsPolicy) school(ctx context.Context, tenant uuid.UUID, school *uuid.UUID, platform settings.Curfew) (*dto.SchoolSettings, error) {
	if school == nil {
		return nil, nil
	}
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return nil, err
	}
	s, found, err := p.Reader.SchoolSettings(tenancy.WithScope(ctx, sc), *school)
	if err != nil || !found {
		return nil, err
	}
	out, err := dto.SchoolSettingsOf(*school, s.Value, s.Version, s.UpdatedAt, platform)
	return &out, err
}

func (p SettingsPolicy) platformCurfew(ctx context.Context) (settings.Curfew, error) {
	s, err := p.Reader.PlatformSetting(ctx, settings.KeyPlatformCurfew)
	if err != nil {
		return settings.Curfew{}, err
	}
	c, err := dto.PlatformCurfewOf(p.Defaults, s.Value, s.Version, s.UpdatedAt)
	return settings.Curfew{Start: c.Start, End: c.End}, err
}

// EffectiveCurfew implements port.CurfewSource.
func (p SettingsPolicy) EffectiveCurfew(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (gamelogin.Curfew, error) {
	pc, err := p.platformCurfew(ctx)
	if err != nil {
		return gamelogin.Curfew{}, err
	}
	eff := pc
	s, err := p.school(ctx, tenant, school, pc)
	if err != nil {
		return gamelogin.Curfew{}, err
	}
	if s != nil {
		eff = s.EffectiveCurfew
	}
	return gamelogin.ParseCurfew(eff.Start+"-"+eff.End, p.location())
}

// AllowedHours implements port.AccessHoursSource.
func (p SettingsPolicy) AllowedHours(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (*settings.AllowedHours, *time.Location, error) {
	s, err := p.school(ctx, tenant, school, settings.Curfew{})
	if err != nil {
		return nil, nil, err
	}
	if s != nil && s.AllowedHours != nil {
		return s.AllowedHours, p.location(), nil
	}
	ts, found, err := p.Reader.TenantSettings(ctx, tenant)
	if err != nil || !found {
		return nil, nil, err
	}
	t, err := dto.TenantSettingsOf(ts.Value, ts.Version, settings.StudentCleanup{})
	if err != nil {
		return nil, nil, err
	}
	return t.AllowedHours, p.location(), nil
}

// DailyPlayLimit implements port.DailyPlayLimitSource: daily_play_limit_minutes of the school (OQ-17, F015-AC-30)
// and the time played on the ICT day of now, from the projection identity.daily_play_time (F013-AC-33).
func (p SettingsPolicy) DailyPlayLimit(ctx context.Context, tenant uuid.UUID, school *uuid.UUID, user uuid.UUID, now time.Time) (*int,
	time.Duration, *time.Location, error) {
	loc := p.location()
	if p.PlayTime == nil {
		return nil, 0, loc, nil
	}
	s, err := p.school(ctx, tenant, school, settings.Curfew{})
	if err != nil || s == nil || s.DailyPlayLimitMinutes == nil {
		return nil, 0, loc, err
	}
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return nil, 0, loc, err
	}
	played, err := p.PlayTime.PlayedOn(tenancy.WithScope(ctx, sc), tenant, user, settings.DateOf(now, loc))
	return s.DailyPlayLimitMinutes, played, loc, err
}

// SocialSettings implements port.SocialSettingsSource (delta F013/F015): student_club_creation of the school (absent =
// false) and cross_school_social_enabled of the tenant (absent = false).
func (p SettingsPolicy) SocialSettings(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (bool, bool, error) {
	s, err := p.school(ctx, tenant, school, settings.Curfew{})
	if err != nil {
		return false, false, err
	}
	club := s != nil && s.StudentClubCreation
	ts, found, err := p.Reader.TenantSettings(ctx, tenant)
	if err != nil || !found {
		return club, false, err
	}
	t, err := dto.TenantSettingsOf(ts.Value, ts.Version, settings.StudentCleanup{})
	if err != nil {
		return club, false, err
	}
	return club, t.CrossSchoolSocialEnabled, nil
}

// ChatDirectPolicy implements port.ChatDirectSource from the projections of the chat policy levels (delta F112-7) under
// the TenantScope of the student.
type ChatDirectPolicy struct {
	Reader read.ChatDirectPolicyReader
}

var _ port.ChatDirectSource = ChatDirectPolicy{}

// DirectMessageEnabled implements port.ChatDirectSource.
func (p ChatDirectPolicy) DirectMessageEnabled(ctx context.Context, tenant uuid.UUID, school *uuid.UUID, commonTenant bool) (*bool, error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return nil, err
	}
	levels, err := p.Reader.ChatDirectLevels(tenancy.WithScope(ctx, sc), tenant, school)
	if err != nil {
		return nil, err
	}
	return settings.DirectMessageEnabled(levels, commonTenant), nil
}
