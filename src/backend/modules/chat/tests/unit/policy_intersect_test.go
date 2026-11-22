package unit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
)

func doc(t *testing.T, raw string) *policy.Document {
	t.Helper()
	d, err := policy.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return &d
}

func seedPlatform(t *testing.T) *policy.Document {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "seed", "chat_policies.platform.json"))
	if err != nil {
		t.Fatal(err)
	}
	return doc(t, string(b))
}

// T020 / F115-AC-08: levels platform → tenant → school intersect (kinds ∩, numbers min, enabled AND, strictness max);
// a channel type without a platform rule is denied whatever the lower levels say. ADR-0095: no class level and no
// class/study_group channel (no platform rule → denied).
func TestPolicyIntersect(t *testing.T) {
	platform := seedPlatform(t)
	e, err := policy.Intersect("direct", false, policy.Levels{Platform: platform})
	if err != nil || e.MaxTextLength != 300 || e.Rate.MessagesPerMinute != 15 || e.GlobalPerMinute != 30 || len(e.AllowedKinds) != 5 {
		t.Fatalf("platform default direct: %+v %v", e, err)
	}
	// F118: features come from the platform level only; the tenant and school documents never change them.
	platformFeatures := e.Features
	if !platformFeatures.Edit || platformFeatures.ReceiptMode() != policy.ReceiptsPerMember {
		t.Fatalf("platform features of direct: %+v", platformFeatures)
	}
	tenant := doc(t, `{"scope_type":"tenant","global_rate":{"messages_per_minute":20},"rules":[{"channel_type":"direct","enabled":true,
		"allowed_kinds":["text","emoji","quick"],"max_text_length":500,"rate":{"messages_per_minute":8}}]}`)
	school := doc(t, `{"scope_type":"school","rules":[{"channel_type":"direct","enabled":true,"allowed_kinds":["text","quick","gif"],
		"max_text_length":150,"slow_mode_seconds":3,"rate":{"burst":2},"validation":{"strictness":"very_strict"}}]}`)
	e, err = policy.Intersect("direct", false, policy.Levels{Platform: platform, Tenant: tenant, School: school})
	if err != nil {
		t.Fatal(err)
	}
	want := policy.Effective{ChannelType: "direct", AllowedKinds: []policy.Kind{"text", "quick"}, MaxTextLength: 150,
		Rate: policy.Rate{MessagesPerMinute: 8, Burst: 2}, GlobalPerMinute: 20, SlowModeSeconds: 3, Strictness: "very_strict",
		Features: platformFeatures}
	if !reflect.DeepEqual(e, want) {
		t.Fatalf("intersection:\n got %+v\nwant %+v", e, want)
	}
	// enabled false at the school level denies a non always-on channel.
	off := doc(t, `{"scope_type":"school","rules":[{"channel_type":"direct","enabled":false,"allowed_kinds":[]}]}`)
	if _, err := policy.Intersect("direct", false, policy.Levels{Platform: platform, School: off}); !errors.Is(err, policy.ErrDenied) {
		t.Fatalf("school disabled direct: %v", err)
	}
	// ADR-0095: the platform seed has no class/study_group rule, so such a channel is denied even with a lower-level rule.
	for _, ct := range []string{"class", "study_group"} {
		lower := doc(t, `{"scope_type":"school","rules":[{"channel_type":"`+ct+`","enabled":true,"allowed_kinds":["text"]}]}`)
		if _, err := policy.Intersect(ct, false, policy.Levels{Platform: platform, School: lower}); !errors.Is(err, policy.ErrDenied) {
			t.Fatalf("%s channel not denied: %v", ct, err)
		}
	}
	// No platform rule = deny, even with a tenant rule (quickstart #9: party rule removed at platform level).
	var noParty policy.Document
	b, _ := json.Marshal(platform)
	_ = json.Unmarshal(b, &noParty)
	rules := noParty.Rules[:0]
	for _, r := range platform.Rules {
		if r.ChannelType != "party" {
			rules = append(rules, r)
		}
	}
	noParty.Rules = rules
	tenantParty := doc(t, `{"scope_type":"tenant","rules":[{"channel_type":"party","enabled":true,"allowed_kinds":["text"]}]}`)
	if _, err := policy.Intersect("party", true, policy.Levels{Platform: &noParty, Tenant: tenantParty}); !errors.Is(err, policy.ErrDenied) {
		t.Fatalf("party without platform rule: %v", err)
	}
	if _, err := policy.Intersect("party", true, policy.Levels{}); !errors.Is(err, policy.ErrDenied) {
		t.Fatalf("no platform document: %v", err)
	}
	// Always-on (match/party/team): lower levels cannot disable nor narrow kinds, only tighten numbers (ADR-0020 §6).
	partyOff := doc(t, `{"scope_type":"school","rules":[{"channel_type":"party","enabled":false,"allowed_kinds":[],
		"rate":{"messages_per_minute":5},"slow_mode_seconds":60}]}`)
	e, err = policy.Intersect("party", true, policy.Levels{Platform: platform, School: partyOff})
	if err != nil || len(e.AllowedKinds) != 5 || e.Rate.MessagesPerMinute != 5 || e.SlowModeSeconds != 10 {
		t.Fatalf("always-on party: %+v %v", e, err)
	}
	// The preset quick_message_emote_only narrows every channel of the school to quick + emoji, gameplay included.
	preset := doc(t, `{"scope_type":"school","chat_policy_preset":"quick_message_emote_only","rules":[]}`)
	for _, ct := range []string{"party", "team", "match", "direct"} {
		e, err := policy.Intersect(ct, ct == "party" || ct == "match" || ct == "team", policy.Levels{Platform: platform, School: preset})
		if err != nil || !reflect.DeepEqual(e.AllowedKinds, []policy.Kind{"emoji", "quick"}) || e.Preset != policy.PresetQuickEmote {
			t.Fatalf("preset %s: %+v %v", ct, e, err)
		}
	}
}
