package policy

import (
	"errors"
	"fmt"
	"slices"
)

// AlwaysOnTypes are the gameplay channel types no level can turn off (ADR-0020 §6, AGENTS R15).
var AlwaysOnTypes = []string{"match", "team", "party"}

// RequiredAlwaysOnKinds are the kinds a rule of an always-on channel must keep at every level (chat-policy.schema.json
// v5 "ADR-0020 mục 6"): a level can never remove them from its rule.
var RequiredAlwaysOnKinds = []Kind{KindText, KindEmoji, KindQuick}

// MaxAlwaysOnSlowMode is the slow-mode ceiling of always-on channels (CHAT_SYSTEM §3: ≤ 10 s).
const MaxAlwaysOnSlowMode = maxAlwaysOnSlowMode

// ErrAlwaysOn: a policy document tries to turn off or restrict an always-on channel (422 CHAT_POLICY_ALWAYS_ON).
var ErrAlwaysOn = errors.New("policy: match/team/party are always on")

// AlwaysOnError tells which rule broke the always-on rule (field path, no content).
type AlwaysOnError struct {
	ChannelType string
	Field       string
}

func (e AlwaysOnError) Error() string {
	return fmt.Sprintf("%v: %s.%s", ErrAlwaysOn, e.ChannelType, e.Field)
}

// Unwrap makes errors.Is(err, ErrAlwaysOn) true.
func (e AlwaysOnError) Unwrap() error { return ErrAlwaysOn }

// IsAlwaysOn reports a gameplay channel type of AlwaysOnTypes.
func IsAlwaysOn(channelType string) bool { return slices.Contains(AlwaysOnTypes, channelType) }

// ValidateAlwaysOn is the validator of every chat policy write path (F117-AC-09; F120/F122 call it before writing a
// document, the migration CHECK chat.f_policy_always_on_ok is the same rule in the database): a rule of match/team/party
// at any level must not be disabled, must keep text/emoji/quick, must not carry allowed_hours and its slow mode must be
// ≤ 10 s. The platform level must also keep a rule for each always-on type (a missing rule would deny the channel).
func ValidateAlwaysOn(d Document) error {
	for _, r := range d.Rules {
		if !IsAlwaysOn(r.ChannelType) {
			continue
		}
		switch {
		case r.Enabled != nil && !*r.Enabled:
			return AlwaysOnError{r.ChannelType, "enabled"}
		case r.AllowedKinds != nil && !containsAll(r.AllowedKinds, RequiredAlwaysOnKinds):
			return AlwaysOnError{r.ChannelType, "allowed_kinds"}
		case len(r.AllowedHours) > 0 && string(r.AllowedHours) != "null":
			return AlwaysOnError{r.ChannelType, "allowed_hours"}
		case r.SlowModeSeconds != nil && *r.SlowModeSeconds > MaxAlwaysOnSlowMode:
			return AlwaysOnError{r.ChannelType, "slow_mode_seconds"}
		}
	}
	if d.ScopeType == "platform" {
		for _, t := range AlwaysOnTypes {
			if _, ok := d.rule(t); !ok {
				return AlwaysOnError{t, "rule"}
			}
		}
	}
	return nil
}

func containsAll(have, want []Kind) bool {
	for _, k := range want {
		if !slices.Contains(have, k) {
			return false
		}
	}
	return true
}

// ApplyAlwaysOn enforces the always-on invariants on an effective policy of match/team/party after Intersect
// (F117-AC-08, AC-24): every kind of the schema is allowed (text, emoji, quick, sticker, gif) whatever a lower level
// says — except the school preset quick_message_emote_only, the only narrowing (quick + emoji) — and the slow mode
// stays ≤ 10 s. Rate, slow mode and strictness keep the tightening of the lower levels. Other channel types are
// returned unchanged.
func ApplyAlwaysOn(e Effective) Effective {
	if !IsAlwaysOn(e.ChannelType) {
		return e
	}
	if e.Preset == PresetQuickEmote {
		e.AllowedKinds = validKinds(presetKinds)
	} else {
		e.AllowedKinds = validKinds(AllKinds)
	}
	if e.SlowModeSeconds > MaxAlwaysOnSlowMode {
		e.SlowModeSeconds = MaxAlwaysOnSlowMode
	}
	return e
}
