// Package settings holds the rules of the platform, tenant and school settings of module identity (F015
// US3/US4, FR-ORG-007, ADR-0020, ADR-0032, ADR-0033, ADR-0078): value shapes, defaults from configuration
// (CONFIG_DEFAULTS, never constants of the code path — AGENTS R33), effective values (a lower level may only
// tighten) and the access/curfew windows. Standard library only.
package settings

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Keys of platform.platform_settings.
const (
	KeyStudentCleanup = "student_cleanup"
	KeyChatRetention  = "chat_retention"
	KeyPlatformCurfew = "platform_curfew"
)

// StudentCleanup is the value of student_cleanup (ADR-0032). DefaultDays nil disables the cleanup.
type StudentCleanup struct {
	DefaultDays              *int `json:"default_days"`
	NoticeDays               int  `json:"notice_days"`
	AllowTenantOverride      bool `json:"allow_tenant_override"`
	ApprovalThresholdPercent int  `json:"approval_threshold_percent"`
}

// ChatRetention is the value of chat_retention (ADR-0033).
type ChatRetention struct {
	GameplayChatRetentionDays int  `json:"gameplay_chat_retention_days"`
	EphemeralRetentionHours   int  `json:"ephemeral_retention_hours"`
	AllowTenantOverride       bool `json:"allow_tenant_override"`
}

// Curfew is a daily window "HH:MM" in the platform time zone; End < Start spans midnight.
type Curfew struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Defaults are the CONFIG_DEFAULTS values used while a key is not configured.
type Defaults struct {
	StudentCleanup       StudentCleanup
	ChatRetention        ChatRetention
	ChatMinimumDays      int    // gameplay_chat_retention_days minimum (3)
	PlatformCurfew       Curfew // platform_curfew (21:30–06:30)
	CurfewWarningMinutes int    // curfew_warning_minutes (10)
	TimeZone             string // Asia/Ho_Chi_Minh
}

// ErrInvalid is a rule violation of a value (422 VALIDATION_FAILED with the field).
type ErrInvalid struct{ Field, Reason string }

func (e ErrInvalid) Error() string { return "settings: " + e.Field + ": " + e.Reason }

// ErrRetentionBelowMinimum: chat retention under the minimum (422 RETENTION_BELOW_MINIMUM).
var ErrRetentionBelowMinimum = errors.New("settings: retention below minimum")

// Validate checks a student_cleanup value.
func (v StudentCleanup) Validate() error {
	switch {
	case v.DefaultDays != nil && (*v.DefaultDays < 0 || *v.DefaultDays > 3650):
		return ErrInvalid{"default_days", "0..3650 or null"}
	case v.NoticeDays < 0 || v.NoticeDays > 90:
		return ErrInvalid{"notice_days", "0..90"}
	case v.ApprovalThresholdPercent < 1 || v.ApprovalThresholdPercent > 100:
		return ErrInvalid{"approval_threshold_percent", "1..100"}
	}
	return nil
}

// Validate checks a chat_retention value against the minimum.
func (v ChatRetention) Validate(minDays int) error {
	switch {
	case v.GameplayChatRetentionDays < minDays:
		return ErrRetentionBelowMinimum
	case v.GameplayChatRetentionDays > 365:
		return ErrInvalid{"gameplay_chat_retention_days", "at most 365"}
	case v.EphemeralRetentionHours < 1 || v.EphemeralRetentionHours > 168:
		return ErrInvalid{"ephemeral_retention_hours", "1..168"}
	}
	return nil
}

// Clock parses "HH:MM" into minutes after midnight.
func Clock(s string) (int, error) {
	h, m, ok := strings.Cut(s, ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || len(s) != 5 || err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, fmt.Errorf("settings: %q is not HH:MM", s)
	}
	return hh*60 + mm, nil
}

// Validate checks a curfew window (start ≠ end).
func (c Curfew) Validate(field string) error {
	s, err := Clock(c.Start)
	if err != nil {
		return ErrInvalid{field + ".start", "HH:MM"}
	}
	e, err := Clock(c.End)
	if err != nil {
		return ErrInvalid{field + ".end", "HH:MM"}
	}
	if s == e {
		return ErrInvalid{field, "start and end must differ"}
	}
	return nil
}

// minutes returns the set of minutes of the day covered by c (1440 booleans).
func (c Curfew) minutes() [1440]bool {
	var out [1440]bool
	s, err1 := Clock(c.Start)
	e, err2 := Clock(c.End)
	if err1 != nil || err2 != nil || s == e {
		return out
	}
	for m := s; m != e; m = (m + 1) % 1440 {
		out[m] = true
	}
	return out
}

// Contains reports whether c covers every minute of inner (a school curfew must contain the platform curfew,
// F015-AC-22: a school may only extend it).
func (c Curfew) Contains(inner Curfew) bool {
	o, i := c.minutes(), inner.minutes()
	for m := range i {
		if i[m] && !o[m] {
			return false
		}
	}
	return true
}

// Union returns the window covering both a and b when it is one contiguous window (always the case when one
// contains the other, which the school rule guarantees); otherwise a.
func Union(a, b Curfew) Curfew {
	switch {
	case b.Start == "" || a.Contains(b):
		return a
	case b.Contains(a):
		return b
	}
	return a
}

// AllowedHours is the access window of a tenant or a school (AllowedHours schema).
type AllowedHours struct {
	TimeZone string   `json:"timezone,omitempty"`
	Windows  []Window `json:"windows"`
}

// Window is one weekly window.
type Window struct {
	Days []string `json:"days"`
	From string   `json:"from"`
	To   string   `json:"to"`
}

var weekdays = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// Validate checks the windows and the time zone.
func (a AllowedHours) Validate(field string) error {
	if len(a.Windows) == 0 || len(a.Windows) > 14 {
		return ErrInvalid{field + ".windows", "1..14 windows"}
	}
	if a.TimeZone != "" {
		if _, err := time.LoadLocation(a.TimeZone); err != nil {
			return ErrInvalid{field + ".timezone", "unknown time zone"}
		}
	}
	for i, w := range a.Windows {
		f := fmt.Sprintf("%s.windows[%d]", field, i)
		if len(w.Days) == 0 || len(w.Days) > 7 {
			return ErrInvalid{f + ".days", "1..7 days"}
		}
		for _, d := range w.Days {
			if !slices.Contains(weekdays, d) {
				return ErrInvalid{f + ".days", "mon..sun"}
			}
		}
		from, err1 := Clock(w.From)
		to, err2 := Clock(w.To)
		if err1 != nil || err2 != nil || to <= from {
			return ErrInvalid{f, "from < to, HH:MM"}
		}
	}
	return nil
}

// Contains reports whether t (converted to the window time zone, default def) falls in one window.
func (a AllowedHours) Contains(t time.Time, def *time.Location) bool {
	loc := def
	if a.TimeZone != "" {
		if l, err := time.LoadLocation(a.TimeZone); err == nil {
			loc = l
		}
	}
	if loc == nil {
		loc = time.UTC
	}
	lt := t.In(loc)
	day := weekdays[lt.Weekday()]
	minute := lt.Hour()*60 + lt.Minute()
	for _, w := range a.Windows {
		from, _ := Clock(w.From)
		to, _ := Clock(w.To)
		if slices.Contains(w.Days, day) && minute >= from && minute < to {
			return true
		}
	}
	return false
}

// Within reports whether every window of a lies inside a window of outer (the school may only tighten the
// tenant hours, F015-AC-19). An outer without windows allows everything.
func (a AllowedHours) Within(outer *AllowedHours) bool {
	if outer == nil || len(outer.Windows) == 0 {
		return true
	}
	for _, w := range a.Windows {
		from, _ := Clock(w.From)
		to, _ := Clock(w.To)
		for _, d := range w.Days {
			ok := false
			for _, o := range outer.Windows {
				of, _ := Clock(o.From)
				ot, _ := Clock(o.To)
				if slices.Contains(o.Days, d) && of <= from && to <= ot {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		}
	}
	return true
}

// EffectiveCleanupDays is the cleanup days of a tenant: its override when the platform allows overrides and
// the tenant set one (0 = disabled), otherwise the platform default (ADR-0032). nil = disabled.
func EffectiveCleanupDays(p StudentCleanup, tenant *int, tenantSet bool) *int {
	if p.AllowTenantOverride && tenantSet {
		if tenant == nil || *tenant == 0 {
			return nil
		}
		return tenant
	}
	return p.DefaultDays
}
