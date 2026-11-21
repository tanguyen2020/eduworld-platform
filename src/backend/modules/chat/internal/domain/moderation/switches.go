package moderation

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Rules of a moderation switch change (MODERATION §12.2, ADR-0030, ADR-0078; CONFIG_DEFAULTS moderation_switch_*).
const (
	// SwitchReasonMin is the shortest reason of a switch change (F116-AC-21).
	SwitchReasonMin = 10
	// SwitchMaxDuration is moderation_switch_max_duration: a disabling change lasts at most 24 hours (F116-AC-31).
	SwitchMaxDuration = 24 * time.Hour
	// SwitchRequestTTL: a request waiting for the second System Admin expires after 24 hours (F116-AC-32).
	SwitchRequestTTL = 24 * time.Hour
	// SwitchApprovals is moderation_switch_approvals: the proposer and one other System Admin.
	SwitchApprovals = 2
)

// Errors of CheckSwitchChange (mapped to the MODERATION_SWITCH_* problem codes).
var (
	ErrSwitchStepNotAllowed = errors.New("moderation: pii and grooming have no switch")
	ErrSwitchUnknownStep    = errors.New("moderation: unknown switch step")
	ErrSwitchReason         = errors.New("moderation: a switch change needs a reason of at least 10 characters")
	ErrSwitchExpiry         = errors.New("moderation: a disabling switch change needs expires_at within moderation_switch_max_duration")
)

// AlwaysOnSteps are the steps without a switch (ADR-0078): a request to turn one off is refused.
var AlwaysOnSteps = []string{StepPII, StepGrooming}

// SwitchChange is a requested configuration of one level (the whole level: enabled_all and every switchable step).
type SwitchChange struct {
	EnabledAll bool
	// Steps as received: switchable steps; pii/grooming only to refuse their disabling.
	Steps     map[string]bool
	Reason    string
	ExpiresAt *time.Time
}

// Config is the normalised configuration of the change: every switchable step present.
func (c SwitchChange) Config() Switch {
	steps := map[string]bool{}
	for _, s := range SwitchableSteps {
		v, ok := c.Steps[s]
		steps[s] = !ok || v
	}
	return Switch{EnabledAll: c.EnabledAll, Steps: steps, ExpiresAt: c.ExpiresAt}
}

// CheckSwitchChange validates the shape of a change: no always-on step turned off (ADR-0078, F116-AC-30), no unknown
// step, a reason of at least SwitchReasonMin characters (F116-AC-21). The expiry is checked by Classify (only a
// disabling change needs one).
func CheckSwitchChange(c SwitchChange) error {
	for k, v := range c.Steps {
		switch {
		case k == StepPII || k == StepGrooming:
			if !v {
				return ErrSwitchStepNotAllowed
			}
		case !Switchable(k):
			return ErrSwitchUnknownStep
		}
	}
	if utf8.RuneCountInString(strings.TrimSpace(c.Reason)) < SwitchReasonMin {
		return ErrSwitchReason
	}
	return nil
}

// offValues returns enabled_all and the steps that are off in s (expired values count as on).
func offValues(s Switch, now time.Time) map[string]bool {
	if s.expired(now) {
		return map[string]bool{}
	}
	return requestedOff(s)
}

// requestedOff returns enabled_all and the steps that are off in a requested configuration (its expiry aside).
func requestedOff(s Switch) map[string]bool {
	out := map[string]bool{}
	if !s.EnabledAll {
		out["enabled_all"] = true
	}
	for _, st := range SwitchableSteps {
		if v, ok := s.Steps[st]; ok && !v {
			out[st] = true
		}
	}
	return out
}

// Classify tells a change that only re-enables (applied at once, no approval — ADR-0078) from a disabling change (a
// value turned off that is on now, or a later expires_at): the latter needs expires_at in (now, now +
// SwitchMaxDuration] (ErrSwitchExpiry, F116-AC-31) and the approval of a second System Admin (F116-AC-32). cur is
// the configured switch of the level (nil: not configured, every step on). For a re-enabling change that keeps some
// values off, expires stays the one in force.
func Classify(cur *Switch, next Switch, now time.Time) (disabling bool, expires *time.Time, err error) {
	nextOff := requestedOff(next)
	if len(nextOff) == 0 {
		return false, nil, nil
	}
	curOff := map[string]bool{}
	var curExp *time.Time
	if cur != nil {
		curOff, curExp = offValues(*cur, now), cur.ExpiresAt
	}
	for k := range nextOff {
		if !curOff[k] {
			disabling = true
		}
	}
	// Re-enabling only: the expiry in force stays (an earlier one is accepted; a later one extends the disabling).
	if !disabling && curExp != nil && (next.ExpiresAt == nil || !next.ExpiresAt.After(*curExp)) {
		e := curExp.UTC()
		if next.ExpiresAt != nil && next.ExpiresAt.After(now) {
			e = next.ExpiresAt.UTC()
		}
		return false, &e, nil
	}
	if next.ExpiresAt == nil || !next.ExpiresAt.After(now) || next.ExpiresAt.After(now.Add(SwitchMaxDuration)) {
		return true, nil, ErrSwitchExpiry
	}
	e := next.ExpiresAt.UTC()
	return true, &e, nil
}

// RequestExpiry is the end of the approval window of a request created at now for a switch disabled until expires: 24
// hours, never after expires (a request that could only apply to the past is pointless).
func RequestExpiry(now, expires time.Time) time.Time {
	end := now.Add(SwitchRequestTTL)
	if expires.Before(end) {
		return expires
	}
	return end
}

// OffSteps returns, for the gauge eduworld_chat_policy_disabled{tenant,step}, whether each switchable step is off at
// this level at now (enabled_all off turns every step off; an expired configuration counts as on).
func OffSteps(s Switch, now time.Time) map[string]bool {
	out := map[string]bool{}
	for _, st := range SwitchableSteps {
		out[st] = !s.On(st, now)
	}
	return out
}
