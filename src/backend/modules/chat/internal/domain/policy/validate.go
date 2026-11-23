package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
)

// Errors of the write path of a policy level (F120-AC-06…09, AC-26, AC-27).
var (
	// ErrRelax: a level is less strict than the levels above (422 CHAT_POLICY_RELAX_NOT_ALLOWED).
	ErrRelax = errors.New("policy: a lower level may only tighten the levels above")
	// ErrGrade: a document carries a grade condition (422 VALIDATION_FAILED, AGENTS R15, F120-AC-08).
	ErrGrade = errors.New("policy: no grade condition in a chat policy")
	// ErrPresetScope: chat_policy_preset outside the school level (422 VALIDATION_FAILED, F120-AC-26).
	ErrPresetScope = errors.New("policy: chat_policy_preset is a school-level setting")
)

// RelaxError tells which rule and field relax the levels above (field path, no content).
type RelaxError struct {
	ChannelType string
	Field       string
}

func (e RelaxError) Error() string { return fmt.Sprintf("%v: %s.%s", ErrRelax, e.ChannelType, e.Field) }

// Unwrap makes errors.Is(err, ErrRelax) true.
func (e RelaxError) Unwrap() error { return ErrRelax }

var gradeKey = regexp.MustCompile(`^grade(_band|_bands|_level|_levels|s)?$`)

// HasGradeKey reports a key naming a grade anywhere in a JSON document (one rule set for every grade: R15, ADR-0020).
func HasGradeKey(raw json.RawMessage) bool {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return false
	}
	return hasGrade(v)
}

func hasGrade(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if gradeKey.MatchString(k) || hasGrade(e) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if hasGrade(e) {
				return true
			}
		}
	}
	return false
}

// Lower is a document being written at a level below the platform with what it is compared to.
type Lower struct {
	Doc Document
	// Upper are the levels above (platform, then the common defaults and the tenant level when writing a school).
	Upper Levels
	// Unit reports a tenant unit (world_scope tenant allowed at the tenant level, ADR-0054).
	Unit bool
	// Curfew is the effective curfew of the level (platform_curfew, plus the school window for a school).
	Curfew Curfew
}

// ValidateLevel checks a tenant or school document against the levels above (CHAT_SYSTEM §3): every rule may only
// tighten — enabled, kinds, max length, rates, slow mode, strictness, allowed hours (also outside the curfew, ADR-0078) —
// unless a rule above sets allow_override_relax. world_scope tenant only at the tenant level of a tenant unit; the preset
// only at the school level. Always-on and the JSON schema are checked by the caller before.
func ValidateLevel(l Lower) error {
	d := l.Doc
	if d.Preset != "" && d.ScopeType != "school" {
		return ErrPresetScope
	}
	if d.WorldScope == "tenant" && (d.ScopeType != "tenant" || !l.Unit) {
		return RelaxError{ChannelType: "world", Field: "world_scope"}
	}
	if d.GlobalRate != nil {
		if up := upperGlobal(l.Upper); up > 0 && d.GlobalRate.MessagesPerMinute > up {
			return RelaxError{ChannelType: "*", Field: "global_rate"}
		}
	}
	curfew := CurfewMask(l.Curfew)
	for _, r := range d.Rules {
		if err := validateRule(r, l.Upper, curfew); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePlatformHours checks that the allowed_hours of the platform rules lie outside the platform curfew (ADR-0078,
// chat_allowed_hours "luôn nằm ngoài platform_curfew").
func ValidatePlatformHours(d Document, curfew Curfew) error {
	cm := CurfewMask(curfew)
	for _, r := range d.Rules {
		ws, ok, err := ParseHours(r.AllowedHours)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		m := Mask(ws)
		if !m.Within(nil, cm) {
			return RelaxError{ChannelType: r.ChannelType, Field: "allowed_hours"}
		}
	}
	return nil
}

func upperGlobal(u Levels) int {
	out := 0
	for _, d := range []*Document{u.Platform, u.Common, u.Tenant} {
		if d != nil && d.GlobalRate != nil {
			out = minPositive(out, d.GlobalRate.MessagesPerMinute)
		}
	}
	return out
}

func relaxable(u Levels, channelType string) bool {
	for _, d := range []*Document{u.Platform, u.Common, u.Tenant} {
		if d == nil {
			continue
		}
		if r, ok := d.rule(channelType); ok && r.AllowOverrideRelax != nil && *r.AllowOverrideRelax {
			return true
		}
	}
	return false
}

func validateRule(r Rule, upper Levels, curfew WeekMask) error {
	t := r.ChannelType
	alwaysOn := IsAlwaysOn(t)
	ws, hasHours, err := ParseHours(r.AllowedHours)
	if err != nil {
		return err
	}
	if relaxable(upper, t) {
		if hasHours && !alwaysOn { // the curfew is never relaxed (ADR-0078)
			m := Mask(ws)
			if !m.Within(nil, curfew) {
				return RelaxError{t, "allowed_hours"}
			}
		}
		return nil
	}
	enabled := r.Enabled == nil || *r.Enabled
	up, err := Intersect(t, alwaysOn, upper)
	if errors.Is(err, ErrDenied) {
		if enabled {
			return RelaxError{t, "enabled"}
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !enabled {
		return nil // turning a (non always-on) channel off is always a tightening
	}
	if !alwaysOn && r.AllowedKinds != nil {
		for _, k := range r.AllowedKinds {
			if !slices.Contains(up.AllowedKinds, k) {
				return RelaxError{t, "allowed_kinds"}
			}
		}
	}
	switch {
	case r.MaxTextLength != nil && up.MaxTextLength > 0 && *r.MaxTextLength > up.MaxTextLength:
		return RelaxError{t, "max_text_length"}
	case r.Rate != nil && up.Rate.MessagesPerMinute > 0 && r.Rate.MessagesPerMinute > up.Rate.MessagesPerMinute:
		return RelaxError{t, "rate.messages_per_minute"}
	case r.Rate != nil && up.Rate.Burst > 0 && r.Rate.Burst > up.Rate.Burst:
		return RelaxError{t, "rate.burst"}
	case r.SlowModeSeconds != nil && *r.SlowModeSeconds < up.SlowModeSeconds:
		return RelaxError{t, "slow_mode_seconds"}
	case r.Validation != nil && strictnessRank[r.Validation.Strictness] < strictnessRank[up.Strictness]:
		return RelaxError{t, "validation.strictness"}
	}
	if hasHours && !alwaysOn {
		m := Mask(ws)
		var outer *WeekMask
		if len(up.Hours) > 0 {
			o := allowedMask(up.Hours)
			outer = &o
		}
		if !m.Within(outer, curfew) {
			return RelaxError{t, "allowed_hours"}
		}
	}
	return nil
}

// allowedMask is the intersection of the windows of every level.
func allowedMask(levels [][]TimeWindow) WeekMask {
	var out WeekMask
	for i := range out {
		out[i] = true
	}
	for _, ws := range levels {
		m := Mask(ws)
		for i := range out {
			out[i] = out[i] && m[i]
		}
	}
	return out
}
