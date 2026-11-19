package dto

import (
	"encoding/json"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
)

// StudentCleanupSettings is the StudentCleanupSettings schema (reason is write-only).
type StudentCleanupSettings struct {
	settings.StudentCleanup
	Version int64 `json:"-"`
}

// ChatRetentionSettings is the ChatRetentionSettings schema.
type ChatRetentionSettings struct {
	settings.ChatRetention
	MinimumDays int   `json:"minimum_days"`
	Version     int64 `json:"-"`
}

// PlatformCurfewSettings is the PlatformCurfewSettings schema.
type PlatformCurfewSettings struct {
	Start          string     `json:"start"`
	End            string     `json:"end"`
	TimeZone       string     `json:"timezone"`
	WarningMinutes int        `json:"warning_minutes"`
	IsDefault      bool       `json:"is_default"`
	UpdatedAt      *time.Time `json:"updated_at"`
	Version        int64      `json:"-"`
}

// TenantSettings is the TenantSettings schema (stored keys plus the effective cleanup days).
type TenantSettings struct {
	AllowedHours               *settings.AllowedHours `json:"allowed_hours,omitempty"`
	InactiveStudentCleanupDays *int                   `json:"inactive_student_cleanup_days,omitempty"`
	EffectiveCleanupDays       *int                   `json:"effective_cleanup_days"`
	GameplayChatRetentionDays  *int                   `json:"gameplay_chat_retention_days,omitempty"`
	DefaultLocale              string                 `json:"default_locale,omitempty"`
	Version                    int64                  `json:"-"`
}

// StudentCleanupOf decodes student_cleanup (defaults while version is 0).
func StudentCleanupOf(d settings.Defaults, value []byte, version int64) (StudentCleanupSettings, error) {
	v := d.StudentCleanup
	if version > 0 {
		if err := json.Unmarshal(value, &v); err != nil {
			return StudentCleanupSettings{}, err
		}
	}
	return StudentCleanupSettings{StudentCleanup: v, Version: version}, nil
}

// ChatRetentionOf decodes chat_retention (defaults while version is 0).
func ChatRetentionOf(d settings.Defaults, value []byte, version int64) (ChatRetentionSettings, error) {
	v := d.ChatRetention
	if version > 0 {
		if err := json.Unmarshal(value, &v); err != nil {
			return ChatRetentionSettings{}, err
		}
	}
	return ChatRetentionSettings{ChatRetention: v, MinimumDays: d.ChatMinimumDays, Version: version}, nil
}

// PlatformCurfewOf decodes platform_curfew (CONFIG_DEFAULTS value and is_default while version is 0).
func PlatformCurfewOf(d settings.Defaults, value []byte, version int64, updatedAt *time.Time) (PlatformCurfewSettings, error) {
	c := d.PlatformCurfew
	if version > 0 {
		if err := json.Unmarshal(value, &c); err != nil {
			return PlatformCurfewSettings{}, err
		}
	}
	return PlatformCurfewSettings{Start: c.Start, End: c.End, TimeZone: d.TimeZone, WarningMinutes: d.CurfewWarningMinutes,
		IsDefault: version == 0, UpdatedAt: updatedAt, Version: version}, nil
}

// TenantSettingsOf decodes the settings JSON of a tenant and computes the effective cleanup days.
func TenantSettingsOf(value []byte, version int64, cleanup settings.StudentCleanup) (TenantSettings, error) {
	var m map[string]json.RawMessage
	if len(value) > 0 {
		if err := json.Unmarshal(value, &m); err != nil {
			return TenantSettings{}, err
		}
	}
	out := TenantSettings{Version: version}
	present := func(k string) (json.RawMessage, bool) {
		raw, ok := m[k]
		return raw, ok && string(raw) != "null"
	}
	if raw, ok := present("allowed_hours"); ok {
		out.AllowedHours = &settings.AllowedHours{}
		_ = json.Unmarshal(raw, out.AllowedHours)
	}
	_, cleanupSet := m["inactive_student_cleanup_days"]
	if raw, ok := present("inactive_student_cleanup_days"); ok {
		out.InactiveStudentCleanupDays = new(int)
		_ = json.Unmarshal(raw, out.InactiveStudentCleanupDays)
	}
	if raw, ok := present("gameplay_chat_retention_days"); ok {
		out.GameplayChatRetentionDays = new(int)
		_ = json.Unmarshal(raw, out.GameplayChatRetentionDays)
	}
	if raw, ok := present("default_locale"); ok {
		_ = json.Unmarshal(raw, &out.DefaultLocale)
	}
	out.EffectiveCleanupDays = settings.EffectiveCleanupDays(cleanup, out.InactiveStudentCleanupDays, cleanupSet)
	return out, nil
}
