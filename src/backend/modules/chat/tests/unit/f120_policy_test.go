package unit

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
)

var ict = time.FixedZone("ICT", 7*3600)

func curfew(t *testing.T, s string) policy.Curfew {
	t.Helper()
	w, err := policy.ParseWindow(s, ict)
	if err != nil {
		t.Fatal(err)
	}
	return policy.Curfew{w}
}

func school(t *testing.T, rules string) policy.Document {
	t.Helper()
	return *doc(t, `{"scope_type":"school","rules":`+rules+`}`)
}

func relax(t *testing.T, err error, field string) {
	t.Helper()
	var re policy.RelaxError
	if !errors.As(err, &re) || re.Field != field {
		t.Fatalf("want relax %s, got %v", field, err)
	}
}

// T022 — F120-AC-07 (AC-CHS-011): a lower level only tightens: a higher rate, a longer text, a kind or a day the platform
// does not allow, a lower strictness or a shorter slow mode is CHAT_POLICY_RELAX_NOT_ALLOWED; tightening passes; a rule
// above with allow_override_relax lets it through.
func TestPolicyValidateLevelRelax(t *testing.T) {
	up := policy.Levels{Platform: seedPlatform(t)}
	c := curfew(t, "21:30-06:30")
	ok := school(t, `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text","emoji"],"max_text_length":200,
		"rate":{"messages_per_minute":10,"burst":2},"slow_mode_seconds":5,"validation":{"strictness":"very_strict"},
		"allowed_hours":[{"days":"mon-fri","from":"07:00","to":"20:00"}]},{"channel_type":"world","enabled":false,"allowed_kinds":[]}]`)
	if err := policy.ValidateLevel(policy.Lower{Doc: ok, Upper: up, Curfew: c}); err != nil {
		t.Fatalf("tightening refused: %v", err)
	}
	for field, rules := range map[string]string{
		"rate.messages_per_minute": `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],"rate":{"messages_per_minute":60}}]`,
		"max_text_length":          `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],"max_text_length":500}]`,
		"validation.strictness":    `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],"validation":{"strictness":"standard"}}]`,
		"slow_mode_seconds":        `[{"channel_type":"world","enabled":true,"allowed_kinds":["text"],"slow_mode_seconds":1}]`,
		"allowed_hours":            `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],"allowed_hours":[{"days":"mon-sun","from":"05:00","to":"20:00"}]}]`,
	} {
		relax(t, policy.ValidateLevel(policy.Lower{Doc: school(t, rules), Upper: up, Curfew: c}), field)
	}
	// A kind removed by the tenant level cannot come back at the school level.
	tenant := doc(t, `{"scope_type":"tenant","rules":[{"channel_type":"direct","enabled":true,"allowed_kinds":["emoji","quick"]}]}`)
	relax(t, policy.ValidateLevel(policy.Lower{Doc: school(t, `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"]}]`),
		Upper: policy.Levels{Platform: up.Platform, Tenant: tenant}, Curfew: c}), "allowed_kinds")
	// A channel disabled above cannot be enabled below.
	off := doc(t, `{"scope_type":"tenant","rules":[{"channel_type":"world","enabled":false,"allowed_kinds":[]}]}`)
	relax(t, policy.ValidateLevel(policy.Lower{Doc: school(t, `[{"channel_type":"world","enabled":true,"allowed_kinds":["emoji"]}]`),
		Upper: policy.Levels{Platform: up.Platform, Tenant: off}, Curfew: c}), "enabled")
	// allow_override_relax on the tenant rule lets the school relax the rate (never the curfew).
	relaxable := doc(t, `{"scope_type":"tenant","rules":[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],
		"rate":{"messages_per_minute":5},"allow_override_relax":true}]}`)
	if err := policy.ValidateLevel(policy.Lower{Doc: school(t, `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],
		"rate":{"messages_per_minute":20}}]`), Upper: policy.Levels{Platform: up.Platform, Tenant: relaxable}, Curfew: c}); err != nil {
		t.Fatalf("allow_override_relax: %v", err)
	}
	// world_scope tenant: only the tenant level of a tenant unit.
	ws := *doc(t, `{"scope_type":"tenant","world_scope":"tenant","rules":[{"channel_type":"world","enabled":true,"allowed_kinds":["text"]}]}`)
	relax(t, policy.ValidateLevel(policy.Lower{Doc: ws, Upper: up, Unit: false, Curfew: c}), "world_scope")
	if err := policy.ValidateLevel(policy.Lower{Doc: ws, Upper: up, Unit: true, Curfew: c}); err != nil {
		t.Fatalf("unit world_scope tenant: %v", err)
	}
	// The preset belongs to the school level only (F120-AC-26).
	pt := *doc(t, `{"scope_type":"tenant","chat_policy_preset":"quick_message_emote_only","rules":[{"channel_type":"direct","enabled":true}]}`)
	if err := policy.ValidateLevel(policy.Lower{Doc: pt, Upper: up, Curfew: c}); !errors.Is(err, policy.ErrPresetScope) {
		t.Fatalf("tenant preset: %v", err)
	}
}

// T056 — F120-AC-27 (TC-CHAT-046): allowed_hours of a persistent channel reaching into the platform curfew (open until
// 22:00 when platform_curfew starts 21:30, or from 06:00) is CHAT_POLICY_RELAX_NOT_ALLOWED; 07:00–21:00 is fine; the
// platform level itself must stay outside platform_curfew.
func TestPolicyAllowedHoursCurfew(t *testing.T) {
	up := policy.Levels{Platform: seedPlatform(t)}
	c := curfew(t, "21:30-06:30")
	relax(t, policy.ValidateLevel(policy.Lower{Doc: school(t, `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],
		"allowed_hours":[{"days":"mon-sun","from":"06:00","to":"22:00"}]}]`), Upper: up, Curfew: c}), "allowed_hours")
	if err := policy.ValidateLevel(policy.Lower{Doc: school(t, `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],
		"allowed_hours":[{"days":"mon-sun","from":"07:00","to":"21:00"}]}]`), Upper: up, Curfew: c}); err != nil {
		t.Fatalf("07:00-21:00 refused: %v", err)
	}
	// The school curfew window extends the platform curfew (ADR-0078): 20:00–21:00 is now inside it.
	wide := append(curfew(t, "21:30-06:30"), curfew(t, "20:00-07:00")...)
	relax(t, policy.ValidateLevel(policy.Lower{Doc: school(t, `[{"channel_type":"direct","enabled":true,"allowed_kinds":["text"],
		"allowed_hours":[{"days":"mon-sun","from":"07:00","to":"21:00"}]}]`), Upper: up, Curfew: wide}), "allowed_hours")
	if err := policy.ValidatePlatformHours(*seedPlatform(t), c); err != nil {
		t.Fatalf("platform seed hours inside curfew: %v", err)
	}
	bad := doc(t, `{"scope_type":"platform","rules":[{"channel_type":"direct","enabled":true,"allowed_hours":[{"days":"mon-sun","from":"06:30","to":"22:00"}]}]}`)
	relax(t, policy.ValidatePlatformHours(*bad, c), "allowed_hours")
}

// T022 — F120-AC-08 (TC-CHAT-010): any key naming a grade, at any depth, is a grade condition (422 VALIDATION_FAILED); the
// same document without it is not.
func TestPolicyGradeKeys(t *testing.T) {
	for _, raw := range []string{`[{"channel_type":"direct","grade_band":"1-3"}]`, `[{"channel_type":"direct","x":{"grade_level":3}}]`,
		`[{"channel_type":"direct","grades":[1,2]}]`, `{"grade":4}`} {
		if !policy.HasGradeKey(json.RawMessage(raw)) {
			t.Errorf("grade key not found in %s", raw)
		}
	}
	if policy.HasGradeKey(json.RawMessage(`[{"channel_type":"direct","enabled":true,"upgrade":1}]`)) {
		t.Fatal("false positive")
	}
}

// T023 — F120-AC-10: the effective policy is platform ∩ common defaults ∩ tenant ∩ school (no class level); no platform rule
// = deny; allowed_hours of every level intersect and are checked at send time (persistent channels only, never
// match/team/party); the common defaults keep direct/world on and only quick + emoji in area, and the System Admin
// override switches each off again (F120-AC-28 as changed by the owner 2026-11-28).
func TestPolicyEffectiveThreeLevels(t *testing.T) {
	p := seedPlatform(t)
	tenant := doc(t, `{"scope_type":"tenant","rules":[{"channel_type":"direct","enabled":true,"allowed_kinds":["text","emoji","quick"],
		"rate":{"messages_per_minute":8},"allowed_hours":[{"days":"mon-fri","from":"07:00","to":"21:00"}]}]}`)
	sch := doc(t, `{"scope_type":"school","rules":[{"channel_type":"direct","enabled":true,"allowed_kinds":["text","emoji"],
		"allowed_hours":[{"days":"mon-sun","from":"08:00","to":"20:00"}]}]}`)
	e, err := policy.Intersect("direct", false, policy.Levels{Platform: p, Tenant: tenant, School: sch})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.AllowedKinds) != 2 || e.Rate.MessagesPerMinute != 8 || len(e.Hours) != 3 {
		t.Fatalf("effective %+v", e)
	}
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, ict) // a Monday
	if !e.OpenAt(monday.Add(9*time.Hour), ict) || e.OpenAt(monday.Add(20*time.Hour+30*time.Minute), ict) ||
		e.OpenAt(monday.Add(5*24*time.Hour+10*time.Hour), ict) { // Saturday: outside the tenant window
		t.Fatal("hours intersection")
	}
	party, err := policy.Intersect("party", true, policy.Levels{Platform: p, Tenant: tenant, School: sch})
	if err != nil || len(party.Hours) != 0 || !party.OpenAt(monday.Add(23*time.Hour), ict) {
		t.Fatalf("always-on hours: %+v %v", party, err)
	}
	if _, err := policy.Intersect("direct", false, policy.Levels{}); !errors.Is(err, policy.ErrDenied) {
		t.Fatalf("no platform: %v", err)
	}
	// Owner 2026-11-28 (common_tenant_social_defaults): direct, world and friends on by default, Area quick + emoji only.
	def := policy.DefaultCommonOverride()
	if !def.DirectEnabled || !def.WorldEnabled || !def.FriendsEnabled || def.AreaFreeTextEnabled {
		t.Fatalf("common defaults: %+v", def)
	}
	common := policy.CommonDefaults(def)
	for _, ty := range []string{"direct", "world"} {
		if _, err := policy.Intersect(ty, false, policy.Levels{Platform: p, Common: common}); err != nil {
			t.Fatalf("common %s by default: %v", ty, err)
		}
	}
	area, err := policy.Intersect("area", false, policy.Levels{Platform: p, Common: common})
	if err != nil || area.Allows(policy.KindText) || !area.Allows(policy.KindQuick) || !area.Allows(policy.KindEmoji) {
		t.Fatalf("common area: %+v %v", area, err)
	}
	if policy.CommonDefaults(policy.CommonOverride{DirectEnabled: true, WorldEnabled: true, AreaFreeTextEnabled: true, FriendsEnabled: true}) != nil {
		t.Fatal("full override keeps a level")
	}
	// The System Admin incident switch turns direct and world off again.
	off := def
	off.DirectEnabled, off.WorldEnabled = false, false
	switched := policy.CommonDefaults(off)
	for _, ty := range []string{"direct", "world"} {
		if _, err := policy.Intersect(ty, false, policy.Levels{Platform: p, Common: switched}); !errors.Is(err, policy.ErrDenied) {
			t.Fatalf("common %s switched off: %v", ty, err)
		}
	}
	// world_scope tenant only from the tenant level of a unit.
	wide := doc(t, `{"scope_type":"tenant","world_scope":"tenant","rules":[]}`)
	if (policy.Levels{Platform: p, Tenant: wide}).WorldScope(false) != "school" || (policy.Levels{Platform: p, Tenant: wide}).WorldScope(true) != "tenant" {
		t.Fatal("world scope")
	}
}

// Hours windows across midnight and day ranges (helper of F120-AC-10, AC-27).
func TestPolicyHoursMask(t *testing.T) {
	ws, ok, err := policy.ParseHours(json.RawMessage(`[{"days":"fri-sun","from":"22:00","to":"02:00"}]`))
	if err != nil || !ok {
		t.Fatal(err)
	}
	sat := time.Date(2026, 11, 28, 1, 0, 0, 0, ict) // Saturday 01:00 (Friday window spills over)
	if !policy.HoursOpen([][]policy.TimeWindow{ws}, sat, ict) || policy.HoursOpen([][]policy.TimeWindow{ws}, sat.Add(2*time.Hour), ict) {
		t.Fatal("midnight window")
	}
	if _, _, err := policy.ParseHours(json.RawMessage(`[{"days":"funday","from":"07:00","to":"08:00"}]`)); err == nil {
		t.Fatal("bad day accepted")
	}
}
