// Package policy resolves the effective ChatPolicy of a channel (F115-AC-08, CHAT_SYSTEM §3): the levels platform →
// tenant → school are intersected (no class level since ADR-0095), the stricter value wins, and a channel type without a platform rule is
// denied. There is deliberately no grade field anywhere in this package: one rule set for every grade (ADR-0020, R15,
// F115-AC-09 static test).
package policy

import (
	"encoding/json"
	"errors"
	"slices"
)

// Kind is a message kind a policy can allow.
type Kind string

// Message kinds of ChatPolicy v5.
const (
	KindText    Kind = "text"
	KindEmoji   Kind = "emoji"
	KindQuick   Kind = "quick"
	KindSticker Kind = "sticker"
	KindGif     Kind = "gif"
)

// AllKinds are the kinds of the schema, in order.
var AllKinds = []Kind{KindText, KindEmoji, KindQuick, KindSticker, KindGif}

// Valid reports a kind of the schema.
func (k Kind) Valid() bool { return slices.Contains(AllKinds, k) }

// PresetQuickEmote is chat_policy_preset quick_message_emote_only (ADR-0078, CHILD_SAFETY §3.5).
const PresetQuickEmote = "quick_message_emote_only"

// presetKinds are the kinds left by PresetQuickEmote, for every channel of the school, gameplay included.
var presetKinds = []Kind{KindQuick, KindEmoji}

// ErrDenied: no rule allows the channel type (no platform rule, or disabled at some level) — CHAT_NOT_ALLOWED.
var ErrDenied = errors.New("policy: channel type not allowed")

// ErrDocument: a stored policy document is not ChatPolicy JSON.
var ErrDocument = errors.New("policy: invalid policy document")

// Rate is messages_per_minute + burst.
type Rate struct {
	MessagesPerMinute int `json:"messages_per_minute"`
	Burst             int `json:"burst,omitempty"`
}

// Validation is the part of rule.validation the resolver intersects (the pipeline of F116 reads the rest).
type Validation struct {
	Strictness string `json:"strictness,omitempty"`
}

// Rule is one channel rule of a level.
type Rule struct {
	ChannelType     string      `json:"channel_type"`
	Enabled         *bool       `json:"enabled,omitempty"`
	AllowedKinds    []Kind      `json:"allowed_kinds,omitempty"`
	MaxTextLength   *int        `json:"max_text_length,omitempty"`
	Rate            *Rate       `json:"rate,omitempty"`
	SlowModeSeconds *int        `json:"slow_mode_seconds,omitempty"`
	Validation      *Validation `json:"validation,omitempty"`
	// AllowedHours is kept raw: the resolver never applies it (curfew is ADR-0078); ValidateAlwaysOn refuses it on
	// match/team/party.
	AllowedHours json.RawMessage `json:"allowed_hours,omitempty"`
}

// Document is the policy document of one level (chat-policy.schema.json v5).
type Document struct {
	ScopeType  string `json:"scope_type"`
	GlobalRate *Rate  `json:"global_rate,omitempty"`
	Rules      []Rule `json:"rules"`
	Preset     string `json:"chat_policy_preset,omitempty"`
}

// Parse decodes a stored document.
func Parse(raw []byte) (Document, error) {
	var d Document
	if err := json.Unmarshal(raw, &d); err != nil || d.ScopeType == "" {
		return Document{}, ErrDocument
	}
	return d, nil
}

func (d Document) rule(channelType string) (Rule, bool) {
	for _, r := range d.Rules {
		if r.ChannelType == channelType {
			return r, true
		}
	}
	return Rule{}, false
}

// Levels are the documents of one resolution: Platform is required; Tenant and School are nil when the level has no
// policy (platform → tenant → school, ADR-0095).
type Levels struct {
	Platform *Document
	Tenant   *Document
	School   *Document
}

// Effective is the policy in force for a channel of one school: the same for every member, whatever the grade.
type Effective struct {
	ChannelType     string `json:"channel_type"`
	AllowedKinds    []Kind `json:"allowed_kinds"`
	MaxTextLength   int    `json:"max_text_length"`
	Rate            Rate   `json:"rate"`
	GlobalPerMinute int    `json:"global_per_minute,omitempty"`
	SlowModeSeconds int    `json:"slow_mode_seconds,omitempty"`
	Strictness      string `json:"strictness,omitempty"`
	Preset          string `json:"preset,omitempty"`
}

// Allows reports a kind allowed by the effective policy.
func (e Effective) Allows(k Kind) bool { return slices.Contains(e.AllowedKinds, k) }

var strictnessRank = map[string]int{"": 0, "standard": 1, "strict": 2, "very_strict": 3}

// maxAlwaysOnSlowMode is the slow-mode ceiling of always-on channels (CHAT_SYSTEM §3: ≤ 10 s).
const maxAlwaysOnSlowMode = 10

func minPositive(a, b int) int {
	switch {
	case a <= 0:
		return b
	case b <= 0:
		return a
	}
	return min(a, b)
}

// Intersect resolves the effective policy of channelType over the levels (CHAT_SYSTEM §3): allowed_kinds intersect,
// numbers take the minimum, enabled is an AND, strictness the highest, slow mode the longest. A channel type without a
// platform rule, or disabled at any level, is ErrDenied. Always-on channels (match/party/team) ignore enabled,
// allowed_kinds of lower levels (ADR-0020 §6); only the school preset quick_message_emote_only narrows their kinds
// (ADR-0078), for every channel of the school alike.
func Intersect(channelType string, alwaysOn bool, l Levels) (Effective, error) {
	if l.Platform == nil {
		return Effective{}, ErrDenied
	}
	base, ok := l.Platform.rule(channelType)
	// An always-on channel ignores enabled at every level, the platform included (ADR-0020 §6, F117-AC-08); a missing
	// platform rule is refused by ValidateAlwaysOn on every policy write.
	if !ok || (!alwaysOn && base.Enabled != nil && !*base.Enabled) {
		return Effective{}, ErrDenied
	}
	e := Effective{ChannelType: channelType, AllowedKinds: validKinds(base.AllowedKinds)}
	apply(&e, base, true)
	if l.Platform.GlobalRate != nil {
		e.GlobalPerMinute = l.Platform.GlobalRate.MessagesPerMinute
	}
	for _, d := range []*Document{l.Tenant, l.School} {
		if d == nil {
			continue
		}
		if d.GlobalRate != nil {
			e.GlobalPerMinute = minPositive(e.GlobalPerMinute, d.GlobalRate.MessagesPerMinute)
		}
		r, ok := d.rule(channelType)
		if !ok {
			continue // no rule at a lower level: nothing to tighten
		}
		if !alwaysOn {
			if r.Enabled != nil && !*r.Enabled {
				return Effective{}, ErrDenied
			}
			if r.AllowedKinds != nil {
				e.AllowedKinds = intersectKinds(e.AllowedKinds, r.AllowedKinds)
			}
		}
		apply(&e, r, false)
	}
	if l.School != nil && l.School.Preset == PresetQuickEmote {
		e.Preset = PresetQuickEmote
		e.AllowedKinds = intersectKinds(e.AllowedKinds, presetKinds)
	}
	if alwaysOn {
		e = ApplyAlwaysOn(e)
	}
	if len(e.AllowedKinds) == 0 && e.Preset == "" {
		return Effective{}, ErrDenied
	}
	return e, nil
}

func apply(e *Effective, r Rule, first bool) {
	if r.MaxTextLength != nil {
		if first {
			e.MaxTextLength = *r.MaxTextLength
		} else {
			e.MaxTextLength = minPositive(e.MaxTextLength, *r.MaxTextLength)
		}
	}
	if r.Rate != nil {
		e.Rate.MessagesPerMinute = minPositive(e.Rate.MessagesPerMinute, r.Rate.MessagesPerMinute)
		e.Rate.Burst = minPositive(e.Rate.Burst, r.Rate.Burst)
	}
	if r.SlowModeSeconds != nil && *r.SlowModeSeconds > e.SlowModeSeconds {
		e.SlowModeSeconds = *r.SlowModeSeconds
	}
	if r.Validation != nil && strictnessRank[r.Validation.Strictness] > strictnessRank[e.Strictness] {
		e.Strictness = r.Validation.Strictness
	}
}

func validKinds(ks []Kind) []Kind {
	out := make([]Kind, 0, len(ks))
	for _, k := range AllKinds {
		if slices.Contains(ks, k) {
			out = append(out, k)
		}
	}
	return out
}

func intersectKinds(a, b []Kind) []Kind {
	out := make([]Kind, 0, len(a))
	for _, k := range a {
		if slices.Contains(b, k) {
			out = append(out, k)
		}
	}
	return out
}
