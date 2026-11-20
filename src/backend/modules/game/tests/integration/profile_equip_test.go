//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// T026 (F070-AC-13, AC-14, AC-15, TC-ECO-007): an item neither free nor in the inventory is 422 ITEM_NOT_OWNED; an
// item in another slot, two items in one slot or more emotes than emote_wheel_slots is 422 EQUIP_SLOT_INVALID; an
// owned item is accepted; a change gives a new hash and publishes avatar.updated with game_profile.updated in the
// same transaction.
func TestAvatarEquipment(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{emoteWheel: 4})
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	first := e.putAvatar(t, s, with(avatar(), "emote_slots", []any{"emote_wave", "emote_clap"}))
	if first.status != http.StatusOK {
		t.Fatalf("first: %d %s", first.status, first.raw)
	}
	rejected := testutil.ToFloat64(observability.AvatarRejections().WithLabelValues("item_not_owned"))
	knight := with(avatar(), "outfit", map[string]any{"item_id": "outfit_knight_01"})
	cases := []struct {
		name string
		cfg  map[string]any
		code problem.Code
	}{
		{"outfit not owned", knight, problem.ItemNotOwned},
		{"unknown item", with(avatar(), "pet", "pet_unknown"), problem.ItemNotOwned},
		{"emote not owned", with(avatar(), "emote_slots", []any{"emote_bow"}), problem.ItemNotOwned},
		{"accessory in another slot", with(avatar(), "accessories", []any{map[string]any{"slot": "back", "item_id": "acc_cap_star"}}),
			problem.EquipSlotInvalid},
		{"two items in one slot", with(avatar(), "accessories", []any{map[string]any{"slot": "head", "item_id": "acc_cap_star"},
			map[string]any{"slot": "head", "item_id": "acc_cap_star"}}), problem.EquipSlotInvalid},
		{"outfit as hair", with(avatar(), "hair", map[string]any{"item_id": "outfit_sport_01", "color": "hair_col_01"}), problem.EquipSlotInvalid},
		{"more emotes than the wheel", with(avatar(), "emote_slots", []any{"emote_wave", "emote_clap", "emote_dance", "emote_celebrate",
			"emote_thank"}), problem.EquipSlotInvalid},
		{"accessory as emote", with(avatar(), "emote_slots", []any{"acc_cap_star"}), problem.EquipSlotInvalid},
	}
	for _, c := range cases {
		r := e.putAvatar(t, s, c.cfg)
		if r.status != http.StatusUnprocessableEntity || r.code() != string(c.code) {
			t.Fatalf("%s: %d %s", c.name, r.status, r.raw)
		}
	}
	if got := testutil.ToFloat64(observability.AvatarRejections().WithLabelValues("item_not_owned")); got < rejected+3 {
		t.Fatalf("rejection metric %v", got)
	}
	if e.events(ctx, t, "avatar.updated") != 0 {
		t.Fatal("refused avatar published")
	}
	e.own(ctx, t, s, "outfit_knight_01")
	e.own(ctx, t, s, "emote_bow")
	r := e.putAvatar(t, s, with(knight, "emote_slots", []any{"emote_bow", "emote_wave"}))
	if r.status != http.StatusOK || r.body["outfit"].(map[string]any)["item_id"] != "outfit_knight_01" {
		t.Fatalf("owned: %d %s", r.status, r.raw)
	}
	if n := e.events(ctx, t, "avatar.updated"); n != 1 {
		t.Fatalf("avatar.updated: %d", n)
	}
	up := e.payloads(ctx, t, "avatar.updated")[0]
	created := e.payloads(ctx, t, "avatar.created")[0]
	if up["avatar_config_hash"] == created["avatar_config_hash"] {
		t.Fatal("hash unchanged")
	}
	// avatar.updated and game_profile.updated of the change are written by the same transaction.
	if n := count(ctx, t, e.relay, `SELECT count(DISTINCT xmin::text) FROM infra.outbox_events WHERE event_type IN ('avatar.updated',
		'game_profile.updated') AND envelope->'payload'->>'avatar_config_hash' = $1`, up["avatar_config_hash"]); n != 1 {
		t.Fatalf("transactions: %d", n)
	}
	gp := e.payloads(ctx, t, "game_profile.updated")
	last := gp[len(gp)-1]
	if last["avatar_config_hash"] != up["avatar_config_hash"] || last["avatar_config"] == nil || last["level"] != float64(1) {
		t.Fatalf("game_profile.updated %v", last)
	}
	// Absent emote_slots keeps the current wheel.
	r = e.putAvatar(t, s, knight)
	if emotes := r.body["emote_slots"].([]any); r.status != http.StatusOK || len(emotes) != 2 || emotes[0] != "emote_bow" {
		t.Fatalf("absent emotes: %d %s", r.status, r.raw)
	}
	e.checkEventSchemas(ctx, t)
}

// T027 (F070-AC-16, AC-17): saveAvatarPreset overwrites slots 1..3 and listAvatarPresets answers only the student's
// own outfits; slot 4, a name over 64, an item code over 64 or a body over 16 KB is 422 VALIDATION_FAILED; a preset
// with an item not owned is ITEM_NOT_OWNED.
func TestAvatarPresets(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(s)
	put := func(slot string, body any) resp {
		return e.do(t, http.MethodPut, "/game/avatar-presets/"+slot, tok, th(s), body)
	}
	for _, slot := range []string{"1", "2", "3"} {
		if r := put(slot, map[string]any{"name": "Bộ " + slot, "avatar_config": avatar()}); r.status != http.StatusOK || r.body["slot"] != float64(slot[0]-'0') {
			t.Fatalf("slot %s: %d %s", slot, r.status, r.raw)
		}
	}
	if r := put("1", map[string]any{"name": "Đi học", "avatar_config": with(avatar(), "skin_tone", "tone_05")}); r.status != http.StatusOK {
		t.Fatalf("overwrite: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPut, "/game/avatar-presets/1", e.gtoken(other), th(other), map[string]any{"avatar_config": avatar()}); r.status != http.StatusOK {
		t.Fatalf("other: %d %s", r.status, r.raw)
	}
	r := e.do(t, http.MethodGet, "/game/avatar-presets", tok, th(s), nil)
	data, _ := r.body["data"].([]any)
	if r.status != http.StatusOK || len(data) != 3 {
		t.Fatalf("list: %d %s", r.status, r.raw)
	}
	one := data[0].(map[string]any)
	if one["slot"] != float64(1) || one["name"] != "Đi học" || one["avatar_config"].(map[string]any)["skin_tone"] != "tone_05" {
		t.Fatalf("slot 1: %v", one)
	}
	long := strings.Repeat("a", 65)
	for name, c := range map[string]struct {
		slot string
		body any
	}{
		"slot 4":       {"4", map[string]any{"avatar_config": avatar()}},
		"long name":    {"2", map[string]any{"name": long, "avatar_config": avatar()}},
		"long item id": {"2", map[string]any{"avatar_config": with(avatar(), "outfit", map[string]any{"item_id": long})}},
	} {
		if r := put(c.slot, c.body); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.ValidationFailed) {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if r := put("2", map[string]any{"avatar_config": with(avatar(), "outfit", map[string]any{"item_id": "outfit_knight_01"})}); r.status != http.StatusUnprocessableEntity ||
		r.code() != string(problem.ItemNotOwned) {
		t.Fatalf("not owned: %d %s", r.status, r.raw)
	}
	// avatar_config over 16 KB (valid JSON padded with whitespace) is refused by the size limit, not stored.
	big := `{"schema_version": 1,` + strings.Repeat(" ", 17000) + `"body_type": "kid_a", "skin_tone": "tone_02",
		"hair": {"item_id": "hair_short_01", "color": "hair_col_01"}, "outfit": {"item_id": "outfit_explorer_01"}}`
	if r := e.doRaw(t, http.MethodPut, "/game/profile/avatar", tok, th(s), big); r.status != http.StatusUnprocessableEntity ||
		r.code() != string(problem.ValidationFailed) {
		t.Fatalf("16 KB: %d %s", r.status, r.raw)
	}
}
