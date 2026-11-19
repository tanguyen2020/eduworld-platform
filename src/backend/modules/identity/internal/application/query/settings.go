package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Settings implements the settings reads of F015 (getStudentCleanupSettings, getChatRetentionSettings,
// getPlatformCurfewSettings, getTenantSettings, getSchoolSettings).
type Settings struct {
	Reader   read.SettingsReader
	Defaults settings.Defaults
}

// platformReader: platform staff with platform.settings.write (the catalog has no separate read permission
// for system settings [Proposed]).
func platformReader(ctx context.Context) (context.Context, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return ctx, err
	}
	if p.TenantID != nil {
		return ctx, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformSettingsWrite, authz.Resource{}); err != nil {
		return ctx, err
	}
	sc, err := authz.SystemPlatformScope("platform-settings")
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(ctx, sc), nil
}

// StudentCleanup is getStudentCleanupSettings.
func (q Settings) StudentCleanup(ctx context.Context) (out dto.StudentCleanupSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetStudentCleanupSettings")
	defer func() { done(err) }()
	if ctx, err = platformReader(ctx); err != nil {
		return out, err
	}
	s, err := q.Reader.PlatformSetting(ctx, settings.KeyStudentCleanup)
	if err != nil {
		return out, err
	}
	return dto.StudentCleanupOf(q.Defaults, s.Value, s.Version)
}

// ChatRetention is getChatRetentionSettings.
func (q Settings) ChatRetention(ctx context.Context) (out dto.ChatRetentionSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetChatRetentionSettings")
	defer func() { done(err) }()
	if ctx, err = platformReader(ctx); err != nil {
		return out, err
	}
	s, err := q.Reader.PlatformSetting(ctx, settings.KeyChatRetention)
	if err != nil {
		return out, err
	}
	return dto.ChatRetentionOf(q.Defaults, s.Value, s.Version)
}

// PlatformCurfew is getPlatformCurfewSettings.
func (q Settings) PlatformCurfew(ctx context.Context) (out dto.PlatformCurfewSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetPlatformCurfewSettings")
	defer func() { done(err) }()
	if ctx, err = platformReader(ctx); err != nil {
		return out, err
	}
	s, err := q.Reader.PlatformSetting(ctx, settings.KeyPlatformCurfew)
	if err != nil {
		return out, err
	}
	return dto.PlatformCurfewOf(q.Defaults, s.Value, s.Version, s.UpdatedAt)
}

// TenantSettings is getTenantSettings (tenant.settings.read): the session tenant with its effective values.
func (q Settings) TenantSettings(ctx context.Context) (out dto.TenantSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetTenantSettings")
	defer func() { done(err) }()
	ctx, p, err := tenantRead(ctx, authz.PermTenantSettingsRead, nil)
	if err != nil {
		return out, err
	}
	s, found, err := q.Reader.TenantSettings(ctx, *p.TenantID)
	if err != nil {
		return out, err
	}
	if !found {
		return out, problem.New(problem.NotFound, "")
	}
	pc, err := q.Reader.PlatformSetting(ctx, settings.KeyStudentCleanup)
	if err != nil {
		return out, err
	}
	cleanup, err := dto.StudentCleanupOf(q.Defaults, pc.Value, pc.Version)
	if err != nil {
		return out, err
	}
	return dto.TenantSettingsOf(s.Value, s.Version, cleanup.StudentCleanup)
}

// SchoolSettings is getSchoolSettings (school.settings.read on that school; another tenant → 404).
func (q Settings) SchoolSettings(ctx context.Context, schoolID uuid.UUID) (out dto.SchoolSettings, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetSchoolSettings")
	defer func() { done(err) }()
	ctx, p, err := tenantRead(ctx, authz.PermSchoolSettingsRead, &schoolID)
	if err != nil {
		return out, err
	}
	s, found, err := q.Reader.SchoolSettings(ctx, schoolID)
	if err != nil {
		return out, err
	}
	if !found || s.TenantID != *p.TenantID {
		return out, problem.New(problem.NotFound, "")
	}
	pc, err := q.Reader.PlatformSetting(ctx, settings.KeyPlatformCurfew)
	if err != nil {
		return out, err
	}
	c, err := dto.PlatformCurfewOf(q.Defaults, pc.Value, pc.Version, pc.UpdatedAt)
	if err != nil {
		return out, err
	}
	return dto.SchoolSettingsOf(schoolID, s.Value, s.Version, s.UpdatedAt, settings.Curfew{Start: c.Start, End: c.End})
}
