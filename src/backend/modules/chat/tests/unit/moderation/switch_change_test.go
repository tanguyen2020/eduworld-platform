package moderation

import (
	"errors"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
)

func allSteps(off ...string) map[string]bool {
	m := map[string]bool{"sensitive_terms": true, "spam": true, "bullying": true, "async_analysis": true}
	for _, s := range off {
		m[s] = false
	}
	return m
}

// T038 / F116-AC-30, T039 / F116-AC-21, F116-AC-31: the shape of a switch change — pii and grooming have no switch,
// unknown steps are refused, the reason has at least 10 characters.
func TestCheckSwitchChange(t *testing.T) {
	ok := "Sự kiện thi đấu cấp trường"
	for name, tc := range map[string]struct {
		c    moderation.SwitchChange
		want error
	}{
		"pii off":         {moderation.SwitchChange{EnabledAll: true, Steps: map[string]bool{"pii": false}, Reason: ok}, moderation.ErrSwitchStepNotAllowed},
		"grooming off":    {moderation.SwitchChange{EnabledAll: true, Steps: map[string]bool{"grooming": false}, Reason: ok}, moderation.ErrSwitchStepNotAllowed},
		"pii on accepted": {moderation.SwitchChange{EnabledAll: true, Steps: map[string]bool{"pii": true, "spam": false}, Reason: ok}, nil},
		"unknown step":    {moderation.SwitchChange{EnabledAll: true, Steps: map[string]bool{"zqstep": false}, Reason: ok}, moderation.ErrSwitchUnknownStep},
		"short reason":    {moderation.SwitchChange{EnabledAll: true, Steps: allSteps(), Reason: " ngắn  "}, moderation.ErrSwitchReason},
		"missing reason":  {moderation.SwitchChange{EnabledAll: false, Steps: allSteps()}, moderation.ErrSwitchReason},
		"valid":           {moderation.SwitchChange{EnabledAll: false, Steps: allSteps("spam"), Reason: ok}, nil},
	} {
		if err := moderation.CheckSwitchChange(tc.c); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", name, err, tc.want)
		}
	}
	// the normalised configuration has the four switchable steps only (pii never stored).
	cfg := moderation.SwitchChange{EnabledAll: true, Steps: map[string]bool{"pii": true, "spam": false}}.Config()
	if len(cfg.Steps) != 4 || cfg.Steps["spam"] || !cfg.Steps["bullying"] {
		t.Fatalf("config: %+v", cfg)
	}
	if _, ok := cfg.Steps["pii"]; ok {
		t.Fatal("pii stored in the configuration")
	}
}

// T039 / F116-AC-31, F116-AC-32: a disabling change needs expires_at in (now, now + 24 h] and an approval; a change
// that only re-enables applies at once and keeps the expiry in force.
func TestClassifySwitchChange(t *testing.T) {
	now := time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	sw := func(all bool, exp *time.Time, off ...string) moderation.Switch {
		return moderation.Switch{EnabledAll: all, Steps: allSteps(off...), ExpiresAt: exp}
	}
	type out struct {
		disabling bool
		exp       *time.Time
		err       error
	}
	cases := map[string]struct {
		cur  *moderation.Switch
		next moderation.Switch
		want out
	}{
		"all on, not configured":        {nil, sw(true, nil), out{}},
		"spam off without expiry":       {nil, sw(true, nil, "spam"), out{true, nil, moderation.ErrSwitchExpiry}},
		"spam off beyond 24 h":          {nil, sw(true, at(25*time.Hour), "spam"), out{true, nil, moderation.ErrSwitchExpiry}},
		"spam off in the past":          {nil, sw(true, at(-time.Minute), "spam"), out{true, nil, moderation.ErrSwitchExpiry}},
		"spam off for 5 minutes":        {nil, sw(true, at(5*time.Minute), "spam"), out{true, at(5 * time.Minute), nil}},
		"spam off exactly 24 h":         {nil, sw(true, at(24*time.Hour), "spam"), out{true, at(24 * time.Hour), nil}},
		"enabled_all off for 1 h":       {nil, sw(false, at(time.Hour)), out{true, at(time.Hour), nil}},
		"re-enable one of two":          {ptr(sw(true, at(2*time.Hour), "spam", "bullying")), sw(true, nil, "spam"), out{false, at(2 * time.Hour), nil}},
		"re-enable, earlier expiry":     {ptr(sw(true, at(2*time.Hour), "spam", "bullying")), sw(true, at(time.Hour), "spam"), out{false, at(time.Hour), nil}},
		"extend the expiry":             {ptr(sw(true, at(2*time.Hour), "spam")), sw(true, at(3*time.Hour), "spam"), out{true, at(3 * time.Hour), nil}},
		"one more step off":             {ptr(sw(true, at(2*time.Hour), "spam")), sw(true, at(2*time.Hour), "spam", "bullying"), out{true, at(2 * time.Hour), nil}},
		"expired value counts as on":    {ptr(sw(true, at(-time.Second), "spam")), sw(true, at(time.Hour), "spam"), out{true, at(time.Hour), nil}},
		"enable everything from off":    {ptr(sw(false, at(time.Hour))), sw(true, nil), out{}},
		"enabled_all off from step off": {ptr(sw(true, at(time.Hour), "spam")), sw(false, at(time.Hour)), out{true, at(time.Hour), nil}},
	}
	for name, tc := range cases {
		d, exp, err := moderation.Classify(tc.cur, tc.next, now)
		if d != tc.want.disabling || !errors.Is(err, tc.want.err) || (exp == nil) != (tc.want.exp == nil) ||
			(exp != nil && !exp.Equal(*tc.want.exp)) {
			t.Errorf("%s: disabling=%v exp=%v err=%v, want %+v", name, d, exp, err, tc.want)
		}
	}
	// the approval window: 24 hours, never past the switch expiry.
	if got := moderation.RequestExpiry(now, now.Add(5*time.Minute)); !got.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("request expiry (short switch): %v", got)
	}
	if got := moderation.RequestExpiry(now, now.Add(30*time.Hour)); !got.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("request expiry: %v", got)
	}
}

func ptr(s moderation.Switch) *moderation.Switch { return &s }

// T027 / F116-AC-22, T038 / F116-AC-30: effective(step) = platform.enabled_all ∧ platform.steps[x] ∧
// tenant.enabled_all ∧ tenant.steps[x]; a stored legacy steps.pii = false is ignored; nothing resolves to off without
// data.
func TestEffectiveSwitches(t *testing.T) {
	now := time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)
	exp := now.Add(time.Hour)
	row := func(all bool, off ...string) *read.SwitchRow {
		return &read.SwitchRow{EnabledAll: all, Steps: allSteps(off...), ExpiresAt: &exp}
	}
	cases := []struct {
		name             string
		platform, tenant *read.SwitchRow
		off              []string
	}{
		{"no data: every step on", nil, nil, nil},
		{"platform spam off", row(true, "spam"), nil, []string{"spam"}},
		{"tenant bullying off", row(true), row(true, "bullying"), []string{"bullying"}},
		{"platform and tenant", row(true, "spam"), row(true, "bullying"), []string{"spam", "bullying"}},
		{"tenant cannot re-enable", row(true, "spam"), row(true), []string{"spam"}},
		{"tenant enabled_all off", row(true), row(false), []string{"sensitive_terms", "spam", "bullying", "async_analysis"}},
		{"platform enabled_all off", row(false), row(true), []string{"sensitive_terms", "spam", "bullying", "async_analysis"}},
	}
	for _, tc := range cases {
		e := service.EffectiveOf(tc.platform, tc.tenant, now)
		got := e.Disabled()
		if len(got) != len(tc.off) {
			t.Errorf("%s: off %v, want %v", tc.name, got, tc.off)
			continue
		}
		for i := range got {
			if got[i] != tc.off[i] && !contains(tc.off, got[i]) {
				t.Errorf("%s: off %v, want %v", tc.name, got, tc.off)
			}
		}
		if !e.On("pii") || !e.On("grooming") {
			t.Errorf("%s: pii/grooming not always on", tc.name)
		}
	}
	// legacy stored pii = false (ADR-0078): dropped when resolved, never in the effective state.
	legacy := &read.SwitchRow{EnabledAll: true, Steps: map[string]bool{"pii": false, "sensitive_terms": true}, ExpiresAt: &exp}
	sw := service.ToSwitch(legacy)
	if _, ok := sw.Steps["pii"]; ok {
		t.Fatal("legacy pii key kept")
	}
	if e := service.EffectiveOf(legacy, legacy, now); !e.On("pii") || len(e.Disabled()) != 0 {
		t.Fatalf("legacy pii: %+v", e)
	}
	// gauge view: enabled_all off turns every step off; an expired configuration is on.
	if off := moderation.OffSteps(moderation.Switch{EnabledAll: false, Steps: allSteps(), ExpiresAt: &exp}, now); !off["spam"] ||
		!off["async_analysis"] {
		t.Fatalf("off steps: %v", off)
	}
	if off := moderation.OffSteps(moderation.Switch{EnabledAll: false, Steps: allSteps(), ExpiresAt: &exp}, exp); off["spam"] {
		t.Fatalf("expired: %v", off)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
