package unit

import (
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
)

// Delta F119 (CHAT_SYSTEM §7.1): Channel.features of listMyChannels are the features of the effective policy (seed of the
// platform, always-on forced on match/party/team) narrowed to what the caller can send now: the school preset
// quick_message_emote_only leaves no sticker, GIF, edit nor mention; a mute leaves no content feature at all; World/Area
// never offer forward, pin, typing nor receipts. max_text_length comes from the policy (§3.1: 200 in party, 120 in
// world), never from a client constant.
func TestFeaturesForCaller(t *testing.T) {
	platform := seedPlatform(t)
	party, err := policy.Intersect("party", true, policy.Levels{Platform: platform})
	if err != nil {
		t.Fatal(err)
	}
	if party.MaxTextLength != 200 {
		t.Fatalf("party max_text_length: %d", party.MaxTextLength)
	}
	full := party.Features.ForCaller(party.AllowedKinds, false)
	if !full.Edit || !full.Recall || !full.Reply || !full.Quote || !full.Sticker || !full.Gif || !full.Reaction || !full.Mention ||
		!full.Typing || full.ReceiptMode() != policy.ReceiptsPerMember {
		t.Fatalf("always-on party features: %+v", full)
	}
	if full.EditWindow().Minutes() != 15 || full.RecallWindow().Hours() != 24 {
		t.Fatalf("windows: %v %v", full.EditWindow(), full.RecallWindow())
	}
	// School preset quick_message_emote_only (ADR-0078): only quick + emoji are sendable.
	school := doc(t, `{"scope_type":"school","chat_policy_preset":"quick_message_emote_only","rules":[]}`)
	preset, err := policy.Intersect("party", true, policy.Levels{Platform: platform, School: school})
	if err != nil {
		t.Fatal(err)
	}
	f := preset.Features.ForCaller(preset.AllowedKinds, false)
	if f.Sticker || f.Gif || f.Edit || f.Mention || !f.Reply || !f.Reaction || !f.Recall {
		t.Fatalf("preset features: %+v (kinds %v)", f, preset.AllowedKinds)
	}
	// Muted: nothing sendable → no content feature; recall and reactions keep the policy value (not gated by a mute).
	muted := party.Features.ForCaller(nil, false)
	if muted.Sticker || muted.Gif || muted.Edit || muted.Mention || muted.Reply || muted.Quote || muted.Forward || !muted.Recall {
		t.Fatalf("muted features: %+v", muted)
	}
	// World (implicit): no forward, pin, typing, receipts whatever the document says.
	world, err := policy.Intersect("world", false, policy.Levels{Platform: platform})
	if err != nil {
		t.Fatal(err)
	}
	w := world.Features
	w.Forward, w.Pin, w.Typing, w.Receipts = true, true, true, policy.ReceiptsCount
	if got := w.ForCaller(world.AllowedKinds, true); got.Forward || got.Pin || got.Typing || got.ReceiptMode() != policy.ReceiptsNone {
		t.Fatalf("world features: %+v", got)
	}
	if world.MaxTextLength != 120 {
		t.Fatalf("world max_text_length: %d", world.MaxTextLength)
	}
}
