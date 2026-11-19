// Package settingscfg reads the defaults of the platform settings from configuration (F015): the values
// registered in CONFIG_DEFAULTS apply while a key of platform.platform_settings is not configured. Nothing is
// a constant of the code path (AGENTS R33); the fallbacks below are the CONFIG_DEFAULTS values.
package settingscfg

import (
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// DefaultsFrom reads EW_STUDENT_CLEANUP_DEFAULT_DAYS (inactive_student_cleanup_days, 180),
// EW_STUDENT_CLEANUP_NOTICE_DAYS (14), EW_STUDENT_CLEANUP_APPROVAL_PERCENT (10),
// EW_GAMEPLAY_CHAT_RETENTION_DAYS (7), EW_GAMEPLAY_CHAT_RETENTION_MIN_DAYS (3), EW_EPHEMERAL_CHAT_RETENTION_HOURS
// (24), EW_PLATFORM_CURFEW (21:30-06:30), EW_CURFEW_WARNING_MINUTES (10) and EW_PLATFORM_TIMEZONE
// (Asia/Ho_Chi_Minh).
func DefaultsFrom(c *config.Config) (settings.Defaults, error) {
	days := c.Int("EW_STUDENT_CLEANUP_DEFAULT_DAYS", 180)
	start, end, ok := cut(c.StringOr("EW_PLATFORM_CURFEW", "21:30-06:30"))
	d := settings.Defaults{
		StudentCleanup: settings.StudentCleanup{DefaultDays: &days, NoticeDays: c.Int("EW_STUDENT_CLEANUP_NOTICE_DAYS", 14),
			AllowTenantOverride: true, ApprovalThresholdPercent: c.Int("EW_STUDENT_CLEANUP_APPROVAL_PERCENT", 10)},
		ChatRetention: settings.ChatRetention{GameplayChatRetentionDays: c.Int("EW_GAMEPLAY_CHAT_RETENTION_DAYS", 7),
			EphemeralRetentionHours: c.Int("EW_EPHEMERAL_CHAT_RETENTION_HOURS", 24), AllowTenantOverride: true},
		ChatMinimumDays:      c.Int("EW_GAMEPLAY_CHAT_RETENTION_MIN_DAYS", 3),
		PlatformCurfew:       settings.Curfew{Start: start, End: end},
		CurfewWarningMinutes: c.Int("EW_CURFEW_WARNING_MINUTES", 10),
		TimeZone:             c.StringOr("EW_PLATFORM_TIMEZONE", "Asia/Ho_Chi_Minh"),
	}
	if !ok || d.PlatformCurfew.Validate("EW_PLATFORM_CURFEW") != nil {
		return d, errors.New("settingscfg: EW_PLATFORM_CURFEW must be HH:MM-HH:MM")
	}
	if d.StudentCleanup.Validate() != nil || d.ChatRetention.Validate(d.ChatMinimumDays) != nil {
		return d, errors.New("settingscfg: invalid student cleanup or chat retention defaults")
	}
	return d, nil
}

func cut(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
