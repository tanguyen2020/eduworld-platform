//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// T015 — F100-AC-05 (MODERATION §1, R15): a typed party name goes through the moderation of module chat (MOCK of
// moderateDisplayText until F116 — fakeModerator, R17): blocked → 422 on name without the matched words; module chat
// not answering → the generated default name "Nhóm của <nickname>" and 201. Same at updatePartySettings.
func TestPartyNameModeration(t *testing.T) {
	e := setupParty(t, popts{})
	an := e.kidA()
	r := e.call(an, http.MethodPost, "/multiplayer/parties", key(), map[string]any{"name": "Nhóm " + blockedName, "party_type": "friend"})
	if r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" || !strings.Contains(r.raw, `"name"`) {
		t.Fatalf("blocked: %d %s", r.status, r.raw)
	}
	if strings.Contains(r.raw, blockedName) {
		t.Fatalf("the matched text is echoed: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.parties`); n != 0 {
		t.Fatalf("written: %d", n)
	}
	e.mod.down = true
	r = e.create(an, http.StatusCreated, map[string]any{"name": "Tên tự đặt", "party_type": "friend"})
	if want := party.DefaultName(an.nickname); r.str("name") != want {
		t.Fatalf("fallback name %q, want %q", r.str("name"), want)
	}
	id := r.str("party_id")
	// updatePartySettings: allowed name stored; blocked → 422; chat down → default name again.
	e.mod.down = false
	if u := e.call(an, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"name": "Đội Mèo"}); u.status != http.StatusOK ||
		u.str("name") != "Đội Mèo" {
		t.Fatalf("rename: %d %s", u.status, u.raw)
	}
	if u := e.call(an, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"name": blockedName}); u.status !=
		http.StatusUnprocessableEntity || strings.Contains(u.raw, blockedName) {
		t.Fatalf("blocked rename: %d %s", u.status, u.raw)
	}
	e.mod.down = true
	if u := e.call(an, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"name": "Tên khác"}); u.status != http.StatusOK ||
		u.str("name") != party.DefaultName(an.nickname) {
		t.Fatalf("fallback rename: %d %s", u.status, u.raw)
	}
	if e.mod.calls < 4 {
		t.Fatalf("moderation calls: %d", e.mod.calls)
	}
	// The default name is cut at 40 characters.
	if n := party.DefaultName(strings.Repeat("x", 60)); len([]rune(n)) != 40 {
		t.Fatalf("default name length %d", len([]rune(n)))
	}
}
