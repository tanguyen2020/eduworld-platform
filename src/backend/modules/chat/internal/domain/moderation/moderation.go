// Package moderation holds the rules of the synchronous moderation pipeline of module chat (F116, MODERATION §3, §11,
// §12, ADR-0030, ADR-0078): actions and their order, the severity bump of the channel strictness, the effective
// moderation switches (platform ∧ tenant, pii and grooming always on), the content spam rules and the bullying rule.
// There is deliberately no grade anywhere in this package: one rule set for every grade (ADR-0020, R15, F116-AC-11).
package moderation

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Actions of a hit, in increasing severity (MODERATION §11.2).
const (
	ActionAllow         = "allow"
	ActionMask          = "mask"
	ActionBlock         = "block"
	ActionHoldForReview = "hold_for_review"
	ActionBlockAndAlert = "block_and_alert"
)

var actionRank = map[string]int{ActionAllow: 0, ActionMask: 1, ActionBlock: 2, ActionHoldForReview: 3, ActionBlockAndAlert: 4}

var rankAction = []string{ActionAllow, ActionMask, ActionBlock, ActionHoldForReview, ActionBlockAndAlert}

// ValidAction reports a known action.
func ValidAction(a string) bool { _, ok := actionRank[a]; return ok }

// Rank returns the order of an action (an unknown action ranks as block: fail closed).
func Rank(a string) int {
	if r, ok := actionRank[a]; ok {
		return r
	}
	return actionRank[ActionBlock]
}

// Heavier returns the heavier of a and b.
func Heavier(a, b string) string {
	if Rank(b) > Rank(a) {
		return b
	}
	return a
}

// Strictness levels of rule.validation.strictness (chat-policy.schema.json v5).
const (
	StrictnessStandard   = "standard"
	StrictnessStrict     = "strict"
	StrictnessVeryStrict = "very_strict"
)

// Bump raises a non-allow action one level when the channel strictness asks for it (very_strict: mask → block, …;
// ADR-0030, F116-AC-03). allow stays allow (a watch term only feeds the risk score).
func Bump(action, strictness string) string {
	if strictness != StrictnessVeryStrict || action == ActionAllow {
		return action
	}
	r := Rank(action)
	if r+1 < len(rankAction) {
		return rankAction[r+1]
	}
	return action
}

// Steps of the pipeline (decision log steps_run / steps_skipped).
const (
	StepTechnical      = "technical"
	StepNormalize      = "normalize"
	StepFormat         = "format"
	StepSensitiveTerms = "sensitive_terms"
	StepPII            = "pii"
	StepGrooming       = "grooming"
	StepSpam           = "spam"
	StepBullying       = "bullying"
	StepContext        = "context"
	StepAsyncAnalysis  = "async_analysis"
)

// SwitchableSteps are the steps a moderation switch can turn off (MODERATION §12.1); pii and grooming are not among
// them (ADR-0078, F116-AC-30).
var SwitchableSteps = []string{StepSensitiveTerms, StepSpam, StepBullying, StepAsyncAnalysis}

// Switchable reports a step that has a switch.
func Switchable(step string) bool {
	for _, s := range SwitchableSteps {
		if s == step {
			return true
		}
	}
	return false
}

// Switch is the configured switch of one level (platform or tenant): Steps holds only switchable steps (an unknown or
// legacy key such as pii is ignored when resolved, ADR-0078); a disabled value past ExpiresAt counts as enabled.
type Switch struct {
	EnabledAll bool
	Steps      map[string]bool
	ExpiresAt  *time.Time
}

// AllOn is the default switch: every step on (moderation_switches, CONFIG_DEFAULTS).
func AllOn() Switch {
	steps := map[string]bool{}
	for _, s := range SwitchableSteps {
		steps[s] = true
	}
	return Switch{EnabledAll: true, Steps: steps}
}

// expired reports a switch whose disabled values lapsed (the expirer job may not have run yet: never default off).
func (s Switch) expired(now time.Time) bool { return s.ExpiresAt != nil && !now.Before(*s.ExpiresAt) }

// On reports a step on at this level.
func (s Switch) On(step string, now time.Time) bool {
	if s.expired(now) {
		return true
	}
	if !s.EnabledAll {
		return false
	}
	v, ok := s.Steps[step]
	return !ok || v
}

// AllEnabled reports enabled_all at this level.
func (s Switch) AllEnabled(now time.Time) bool { return s.expired(now) || s.EnabledAll }

// Disabled reports any disabled value (then reason and expires_at are required, ADR-0078).
func (s Switch) Disabled() bool {
	if !s.EnabledAll {
		return true
	}
	for _, st := range SwitchableSteps {
		if v, ok := s.Steps[st]; ok && !v {
			return true
		}
	}
	return false
}

// Effective is the switch state in force for a tenant: platform ∧ tenant (a tenant without configuration inherits the
// platform; F116-AC-22).
type Effective struct {
	Platform Switch
	Tenant   *Switch
	At       time.Time
}

// On reports whether a step runs. pii and grooming always run (ADR-0078); a step outside the known list runs.
func (e Effective) On(step string) bool {
	if !Switchable(step) {
		return true
	}
	if !e.Platform.On(step, e.At) {
		return false
	}
	return e.Tenant == nil || e.Tenant.On(step, e.At)
}

// AllEnabled reports platform.enabled_all ∧ tenant.enabled_all.
func (e Effective) AllEnabled() bool {
	return e.Platform.AllEnabled(e.At) && (e.Tenant == nil || e.Tenant.AllEnabled(e.At))
}

// Disabled returns the switchable steps not in force (metric eduworld_chat_policy_disabled{tenant,step}).
func (e Effective) Disabled() []string {
	var out []string
	for _, s := range SwitchableSteps {
		if !e.On(s) {
			out = append(out, s)
		}
	}
	return out
}

// Spam thresholds (chat_spam_thresholds, MODERATION §3.4, CONFIG_DEFAULTS [Confirmed]).
const (
	SpamRepeatCount     = 3                // ≥ 3 identical messages in a channel …
	SpamRepeatWindow    = 60 * time.Second // … within 60 s → block the repeat
	SpamChannelCount    = 4                // the same content in ≥ 4 channels …
	SpamChannelWindow   = 2 * time.Minute  // … within 2 minutes → block
	SpamCapsRatio       = 0.7              // > 70% capitals → reduced (lowercased)
	SpamCapsMinLetters  = 10               // only for texts of at least 10 letters
	SpamEmojiRun        = 10               // > 10 emoji in a row → reduced to 10
	SpamViolationCount  = 3                // 3 blocks …
	SpamViolationWindow = 10 * time.Minute // … within 10 minutes → system mute (auto_mute_spam)
	AutoMuteFirst       = 10 * time.Minute // auto_mute_spam: 10 minutes …
	AutoMuteRepeat      = time.Hour        // … 1 hour when repeated within AutoMuteRepeatWindow
	AutoMuteRepeatSince = 24 * time.Hour
)

// Other engine limits (sensitive_terms_engine_limits, moderation_config_propagation_max).
const (
	PatternBudget     = 5 * time.Millisecond // RE2 time budget per pattern
	MaxTenantPatterns = 200                  // patterns per tenant
	PropagationMax    = 60 * time.Second     // catalogue/switch change in force on every instance
)

// filter_quality_gate (MODERATION §11.4, ADR-0030, F116-AC-14): a catalogue version is published only when the
// regression suite reaches this recall on the high/critical/PII samples and stays at or under this false-positive rate
// on the clean samples.
const (
	RegressionMinRecall = 0.95
	RegressionMaxFPRate = 0.03
)

// PassesQualityGate reports a regression result within filter_quality_gate.
func PassesQualityGate(recall, fpRate float64) bool {
	return recall >= RegressionMinRecall && fpRate <= RegressionMaxFPRate
}

// ToggleReasonMin is the shortest reason of a tenant category toggle (F116-AC-17).
const ToggleReasonMin = 10

// Detected reports a decision that stops a message (block or heavier): a high/critical/PII sample of the regression
// suite counts as detected only then.
func Detected(decision string) bool {
	return Rank(decision) >= Rank(ActionBlock) && decision != DecisionUnchecked
}

// FalsePositive reports a clean sample that was not delivered as is (mask included).
func FalsePositive(decision string) bool {
	return decision != DecisionAllow && decision != DecisionUnchecked
}

// Reduce applies the formatting spam rules: a shouting text is lowercased, a run of more than 10 emoji is cut to 10.
// changed reports a reduction (decision mask, code spam).
func Reduce(text string) (string, bool) {
	letters, upper := 0, 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	out := text
	changed := false
	if letters >= SpamCapsMinLetters && float64(upper)/float64(letters) > SpamCapsRatio {
		out, changed = strings.ToLower(out), true
	}
	var b strings.Builder
	run := 0
	for _, r := range out {
		if isEmoji(r) {
			run++
			if run > SpamEmojiRun {
				changed = true
				continue
			}
		} else if !isJoiner(r) {
			run = 0
		}
		b.WriteRune(r)
	}
	return b.String(), changed
}

func isEmoji(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) || (r >= 0x1F000 && r <= 0x1F2FF)
}

func isJoiner(r rune) bool { return r == 0x200D || r == 0xFE0F || unicode.Is(unicode.Mn, r) }

// Mask replaces the given byte spans of text with "***" (overlapping spans merged); only matched spans are hidden
// (F116-AC-03).
func Mask(text string, spans [][2]int) string {
	if len(spans) == 0 {
		return text
	}
	sorted := append([][2]int(nil), spans...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j][0] < sorted[j-1][0]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	var b strings.Builder
	pos := 0
	for _, s := range sorted {
		start, end := max(s[0], pos), min(s[1], len(text))
		if start >= end {
			continue
		}
		if !utf8.ValidString(text[start:end]) {
			continue
		}
		b.WriteString(text[pos:start])
		b.WriteString("***")
		pos = end
	}
	b.WriteString(text[pos:])
	return b.String()
}
