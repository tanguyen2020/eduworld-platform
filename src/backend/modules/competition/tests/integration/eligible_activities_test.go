//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
)

// T013 — F101-AC-05 (TC-PTY-005, module game MOCK — R17): listPartyEligibleActivities returns the activities every member can
// play and those only some can, with per member a reason code and the requirement of the activity — never the grade, class or
// school of another student (R16); module game not answering leaves explore only; non-members get 404.
func TestEligibleActivities(t *testing.T) {
	game := &fakeGame{}
	e := setupParty(t, popts{game: game})
	e.area("forest", []string{"g6_7"}, true)
	e.area("meadow", []string{"g6_7"}, false)
	e.area("mountain", []string{"g8_9"}, false)
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	e.unlock(a, "forest")
	e.unlock(b, "forest")
	id := e.party(a, b, c)

	r := e.call(b, http.MethodGet, "/multiplayer/parties/"+id+"/eligible-activities", "", nil)
	if r.status != http.StatusOK {
		t.Fatalf("list: %d %s", r.status, r.raw)
	}
	noPII(t, r)
	if strings.Contains(r.raw, "grade_level\"") {
		t.Fatalf("grade of a member leaked: %s", r.raw)
	}
	byRef := map[string]map[string]any{}
	for _, d := range r.data() {
		byRef[str(d["activity_type"])+"/"+str(d["activity_ref"])] = d
	}
	reason := func(key, student string) string {
		for _, m := range byRef[key]["members"].([]any) {
			mm := m.(map[string]any)
			if mm["student_id"] == student {
				if mm["eligible"] == true {
					return "ok"
				}
				return str(mm["reason_code"]) + "|" + str(mm["requirement"])
			}
		}
		return "missing"
	}
	if byRef["explore/meadow"]["all_eligible"] != true || byRef["explore/forest"]["all_eligible"] != false {
		t.Fatalf("all_eligible: %v", byRef)
	}
	if got := reason("explore/forest", c.id.String()); got != "area_locked|area:forest" {
		t.Fatalf("C forest: %s", got)
	}
	if got := reason("explore/forest", a.id.String()); got != "ok" {
		t.Fatalf("A forest: %s", got)
	}
	if got := reason("explore/mountain", a.id.String()); got != "grade_band_mismatch|grade_bands:g8_9" {
		t.Fatalf("A mountain: %s", got)
	}
	if byRef["quest/q-bridge"]["all_eligible"] != true || byRef["quest/q-bridge"]["min_members"] != float64(2) {
		t.Fatalf("quest: %v", byRef["quest/q-bridge"])
	}
	if got := reason("boss/b-golem", b.id.String()); got != "grade_not_allowed|grade_levels:4" {
		t.Fatalf("boss: %s", got)
	}
	if len(game.scopes) == 0 || game.scopes[0] != e.tenantA {
		t.Fatalf("facade game called without the TenantScope of the party: %v", game.scopes)
	}

	game.mu.Lock()
	game.down = true
	game.mu.Unlock()
	r = e.call(a, http.MethodGet, "/multiplayer/parties/"+id+"/eligible-activities", "", nil)
	for _, d := range r.data() {
		if d["activity_type"] != "explore" {
			t.Fatalf("module game down: only explore, got %v", d)
		}
	}
	if len(r.data()) != 3 {
		t.Fatalf("explore areas: %s", r.raw)
	}
	for _, k := range []kid{e.kidA(), e.kid(e.tenantB, e.schoolB, 6)} {
		if r := e.call(k, http.MethodGet, "/multiplayer/parties/"+id+"/eligible-activities", "", nil); r.status != http.StatusNotFound ||
			r.code() != "PARTY_NOT_FOUND" {
			t.Fatalf("outsider: %d %s", r.status, r.raw)
		}
	}
}
