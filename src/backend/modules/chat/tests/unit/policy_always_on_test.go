package unit

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// T018 / F117-AC-08 (TC-CHAT-018): the effective policy of match/team/party is always enabled with every kind of the schema
// (text, emoji, quick, sticker, gif) whatever a lower level — or the platform — says about enabled and allowed_kinds; lower
// levels only tighten rate, slow mode (≤ 10 s) and strictness; no allowed_hours applies.
func TestApplyAlwaysOn(t *testing.T) {
	platform := seedPlatform(t)
	all := []policy.Kind{policy.KindText, policy.KindEmoji, policy.KindQuick, policy.KindSticker, policy.KindGif}
	tenant := doc(t, `{"scope_type":"tenant","rules":[{"channel_type":"party","enabled":false,"allowed_kinds":["quick"],
		"rate":{"messages_per_minute":5,"burst":1},"slow_mode_seconds":60,"validation":{"strictness":"very_strict"}}]}`)
	school := doc(t, `{"scope_type":"school","rules":[{"channel_type":"party","enabled":false,"allowed_kinds":["emoji"]}]}`)
	for _, typ := range []string{"match", "team", "party"} {
		e, err := policy.Intersect(typ, true, policy.Levels{Platform: platform, Tenant: tenant, School: school})
		if err != nil {
			t.Fatalf("%s: always-on denied: %v", typ, err)
		}
		if !reflect.DeepEqual(e.AllowedKinds, all) {
			t.Fatalf("%s: kinds %v, want every kind", typ, e.AllowedKinds)
		}
		if e.SlowModeSeconds > policy.MaxAlwaysOnSlowMode {
			t.Fatalf("%s: slow mode %d above 10 s", typ, e.SlowModeSeconds)
		}
	}
	e, _ := policy.Intersect("party", true, policy.Levels{Platform: platform, Tenant: tenant, School: school})
	if e.Rate.MessagesPerMinute != 5 || e.Rate.Burst != 1 || e.Strictness != "very_strict" || e.SlowModeSeconds != 10 {
		t.Fatalf("lower levels must still tighten rate/slow mode/strictness: %+v", e)
	}
	// The platform itself cannot turn off an always-on channel nor drop its kinds.
	off := doc(t, `{"scope_type":"platform","rules":[{"channel_type":"match","enabled":false,"allowed_kinds":["quick"],
		"max_text_length":200,"rate":{"messages_per_minute":12}}]}`)
	if e, err := policy.Intersect("match", true, policy.Levels{Platform: off}); err != nil || !reflect.DeepEqual(e.AllowedKinds, all) {
		t.Fatalf("platform disabled match: %+v %v", e, err)
	}
	// ApplyAlwaysOn leaves other channel types untouched.
	d := policy.Effective{ChannelType: "direct", AllowedKinds: []policy.Kind{policy.KindQuick}, SlowModeSeconds: 30}
	if got := policy.ApplyAlwaysOn(d); !reflect.DeepEqual(got, d) {
		t.Fatalf("direct changed: %+v", got)
	}
}

// T038 / F117-AC-24 (TC-CHAT-048): the school preset quick_message_emote_only is the only way to narrow the kinds of an
// always-on channel: quick + emoji, still enabled; a school without the preset keeps every kind.
func TestAlwaysOnPreset(t *testing.T) {
	platform := seedPlatform(t)
	preset := doc(t, `{"scope_type":"school","chat_policy_preset":"quick_message_emote_only","rules":[]}`)
	for _, typ := range []string{"match", "team", "party"} {
		e, err := policy.Intersect(typ, true, policy.Levels{Platform: platform, School: preset})
		if err != nil {
			t.Fatalf("%s: denied under the preset: %v", typ, err)
		}
		if !reflect.DeepEqual(e.AllowedKinds, []policy.Kind{policy.KindEmoji, policy.KindQuick}) || e.Preset != policy.PresetQuickEmote {
			t.Fatalf("%s: preset kinds %v", typ, e.AllowedKinds)
		}
		if e.Allows(policy.KindText) {
			t.Fatalf("%s: text allowed under the preset", typ)
		}
	}
	plain := doc(t, `{"scope_type":"school","rules":[]}`)
	if e, _ := policy.Intersect("party", true, policy.Levels{Platform: platform, School: plain}); len(e.AllowedKinds) != 5 {
		t.Fatalf("school without preset: %v", e.AllowedKinds)
	}
}

// T018 / F117-AC-09 (TC-CHAT-018): every chat policy write path refuses a rule of match/team/party that is disabled,
// drops a mandatory kind, carries allowed_hours or a slow mode above 10 s (422 CHAT_POLICY_ALWAYS_ON); the platform keeps a
// rule for each; other channel types and compliant rules pass.
func TestValidateAlwaysOn(t *testing.T) {
	for _, c := range []struct {
		name, raw, field string
	}{
		{"disabled", `{"scope_type":"school","rules":[{"channel_type":"party","enabled":false}]}`, "enabled"},
		{"kind dropped", `{"scope_type":"tenant","rules":[{"channel_type":"match","allowed_kinds":["emoji","quick"]}]}`, "allowed_kinds"},
		{"allowed_hours", `{"scope_type":"school","rules":[{"channel_type":"team","allowed_hours":[{"days":"mon-fri","from":"07:00","to":"17:00"}]}]}`, "allowed_hours"},
		{"slow mode", `{"scope_type":"tenant","rules":[{"channel_type":"party","slow_mode_seconds":11}]}`, "slow_mode_seconds"},
		{"platform without team", `{"scope_type":"platform","rules":[{"channel_type":"match"},{"channel_type":"party"}]}`, "rule"},
	} {
		err := policy.ValidateAlwaysOn(*doc(t, c.raw))
		var ae policy.AlwaysOnError
		if !errors.Is(err, policy.ErrAlwaysOn) || !errors.As(err, &ae) || ae.Field != c.field {
			t.Errorf("%s: %v, want ErrAlwaysOn on %s", c.name, err, c.field)
		}
	}
	for _, raw := range []string{
		`{"scope_type":"school","rules":[{"channel_type":"party","enabled":true,"allowed_kinds":["text","emoji","quick"],"slow_mode_seconds":10,
			"rate":{"messages_per_minute":5}}]}`,
		`{"scope_type":"school","rules":[{"channel_type":"direct","enabled":false,"allowed_hours":[{"days":"mon-sun","from":"07:00","to":"20:00"}]}]}`,
		`{"scope_type":"school","chat_policy_preset":"quick_message_emote_only","rules":[]}`,
	} {
		if err := policy.ValidateAlwaysOn(*doc(t, raw)); err != nil {
			t.Errorf("compliant document refused: %v (%s)", err, raw)
		}
	}
	if err := policy.ValidateAlwaysOn(*seedPlatform(t)); err != nil {
		t.Fatalf("platform seed refused: %v", err)
	}
}

// F117-AC-09: the write-path gate answers 422 CHAT_POLICY_ALWAYS_ON (problem code of API_DESIGN §4.1).
func TestCheckPolicyDocument(t *testing.T) {
	err := command.CheckPolicyDocument([]byte(`{"scope_type":"school","rules":[{"channel_type":"party","enabled":false}]}`))
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.ChatPolicyAlwaysOn || problem.Status(pe.Code) != 422 {
		t.Fatalf("got %v", err)
	}
	if err := command.CheckPolicyDocument([]byte(`{"scope_type":"school","rules":[{"channel_type":"party","enabled":true}]}`)); err != nil {
		t.Fatalf("compliant: %v", err)
	}
	if err := command.CheckPolicyDocument([]byte(`not json`)); !errors.As(err, &pe) || pe.Code != problem.ValidationFailed {
		t.Fatalf("invalid document: %v", err)
	}
}
