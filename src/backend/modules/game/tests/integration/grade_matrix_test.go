//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// Grade matrix 1–9 of module game on PostgreSQL (TEST_STRATEGY §6.10): the read routes of a student of every grade,
// with the band boundaries 3→4, 4→5, 5→6 and 7→8.

const chlMatrixAll = "0193a5f4-6c1e-7b2a-9d1f-000000076900" // survival, grades 1–9, 900 s

func chlMatrixOf(g int) string { return fmt.Sprintf("0193a5f4-6c1e-7b2a-9d1f-00000007691%d", g) } // time, grade g only

// TestGradeMatrixChallengeRead (F076-AC-01/02, delta F076 for F069-AC-14/20, CHALLENGE_BOSS_SYSTEM §2.2, AC-UX-0601): for
// a student of each grade 1–9, listGameChallenges holds the challenge of every grade and the challenge of its own grade
// only; available_difficulties are easy–hard for 1–3, + expert for 4–5, all five for 6–9; getGameChallenge gives the
// time × 1.5 of grades 1–3 and difficulties[] (grades 1–3 never receive expert/legendary; a level locked for the grade
// carries grade_not_allowed); a challenge of another grade is 404; active_attempt_id is null without a run and the id of
// the run in progress while it lasts.
func TestGradeMatrixChallengeRead(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	items := []command.ContentItem{challengeItem(chlMatrixAll, "chl.matrix.all", "survival", grades19(), func(m map[string]any) {
		m["goal"], m["time_limit_seconds"] = map[string]any{"metric": "rounds_survived", "target": 10}, 900
	})}
	for g := 1; g <= 9; g++ {
		items = append(items, challengeItem(chlMatrixOf(g), fmt.Sprintf("chl.matrix.g%d", g), "time", []int{g}, nil))
	}
	e.publishChallenges(ctx, t, items, e.tenantA)
	for g := 1; g <= 9; g++ {
		s := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, g)
		r := e.do(t, http.MethodGet, "/game/challenges", e.gtoken(s), th(s), nil)
		ids := challengeIDs(r)
		slices.Sort(ids)
		if r.status != http.StatusOK || !slices.Equal(ids, []string{chlMatrixAll, chlMatrixOf(g)}) {
			t.Fatalf("grade %d list: %d %s", g, r.status, r.raw)
		}
		want := []string{"easy", "normal", "hard"}
		switch {
		case g >= 6:
			want = append(want, "expert", "legendary")
		case g >= 4:
			want = append(want, "expert")
		}
		if got := levelsOf(r, chlMatrixAll); !slices.Equal(got, want) {
			t.Fatalf("grade %d levels %v, want %v", g, got, want)
		}
		for _, it := range r.items("data") {
			if v, ok := it["active_attempt_id"]; !ok || v != nil {
				t.Fatalf("grade %d: active_attempt_id %v (present %v) without a run", g, v, ok)
			}
		}
		d := e.do(t, http.MethodGet, "/game/challenges/"+chlMatrixAll, e.gtoken(s), th(s), nil)
		limit := 900.0
		if g <= 3 {
			limit = 1350 // 900 s × 1.5 (g1_3_challenge_rules)
		}
		if d.status != http.StatusOK || d.body["time_limit_seconds"] != limit {
			t.Fatalf("grade %d detail: %d %s", g, d.status, d.raw)
		}
		var levels []string
		for _, x := range d.body["difficulties"].([]any) {
			o := x.(map[string]any)
			l := o["difficulty"].(string)
			levels = append(levels, l)
			available := slices.Contains(want, l)
			reason, hasReason := o["locked_reason_code"]
			if o["available"] != available || !hasReason || (available && reason != nil) || (!available && reason != "grade_not_allowed") {
				t.Fatalf("grade %d %s: %v", g, l, o)
			}
			if o["time_limit_seconds"] != limit || o["lives"] == nil || o["reward_multiplier"] == nil || o["time_per_activity_seconds"].(float64) < 10 {
				t.Fatalf("grade %d %s option: %v", g, l, o)
			}
		}
		wantLevels := []string{"easy", "normal", "hard", "expert", "legendary"}
		if g <= 3 {
			wantLevels = []string{"easy", "normal", "hard"}
		}
		if !slices.Equal(levels, wantLevels) {
			t.Fatalf("grade %d difficulties %v, want %v", g, levels, wantLevels)
		}
		for _, h := range []int{g - 1, g + 1} {
			if h < 1 || h > 9 {
				continue
			}
			if r := e.do(t, http.MethodGet, "/game/challenges/"+chlMatrixOf(h), e.gtoken(s), th(s), nil); r.status != http.StatusNotFound {
				t.Fatalf("grade %d reads the challenge of grade %d: %d", g, h, r.status)
			}
		}
	}
	// active_attempt_id (F069-AC-20): the run in progress is found again from the board and the detail, then null.
	for _, g := range []int{3, 4, 7} {
		s := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, g)
		st := e.startChallenge(t, s, chlMatrixAll, "easy", "")
		if st.status != http.StatusCreated {
			t.Fatalf("grade %d start: %d %s", g, st.status, st.raw)
		}
		attempt := st.str("challenge_attempt_id")
		for _, it := range e.do(t, http.MethodGet, "/game/challenges", e.gtoken(s), th(s), nil).items("data") {
			want := any(nil)
			if it["challenge_id"] == chlMatrixAll {
				want = attempt
			}
			if it["active_attempt_id"] != want {
				t.Fatalf("grade %d %s: active_attempt_id %v, want %v", g, it["challenge_id"], it["active_attempt_id"], want)
			}
		}
		if d := e.do(t, http.MethodGet, "/game/challenges/"+chlMatrixAll, e.gtoken(s), th(s), nil); d.body["active_attempt_id"] != attempt {
			t.Fatalf("grade %d detail active_attempt_id %v", g, d.body["active_attempt_id"])
		}
		if r := e.abandonChallenge(t, s, attempt, ""); r.status != http.StatusOK {
			t.Fatalf("abandon: %d %s", r.status, r.raw)
		}
		if d := e.do(t, http.MethodGet, "/game/challenges/"+chlMatrixAll, e.gtoken(s), th(s), nil); d.body["active_attempt_id"] != nil {
			t.Fatalf("grade %d after abandon: %v", g, d.body["active_attempt_id"])
		}
	}
}

func grades19() []int { return []int{1, 2, 3, 4, 5, 6, 7, 8, 9} }

// TestGradeMatrixQuestsAndWorld (F072-AC-01/02, F071-AC-02, MULTIPLAYER_GAMEPLAY §6): for a student of each grade, the
// learning quest of its own grade only (the quest of a neighbour grade is 404); learning_zone.history from grade 4; the
// main_hub room is shared inside a grade band and never across 3→4, 5→6, 7→8.
func TestGradeMatrixQuestsAndWorld(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	var items []command.ContentItem
	for g := 1; g <= 9; g++ {
		items = append(items, learningQuest(fmt.Sprintf("0193a5f4-6c1e-7b2a-9d1f-0000000b091%d", g), fmt.Sprintf("math.g%d.matrix", g),
			func(m map[string]any) { m["grade_levels"] = []int{g} }))
	}
	e.publishQuests(ctx, t, items, e.tenantA)
	rooms := map[int]any{}
	for g := 1; g <= 9; g++ {
		s := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, g)
		got := listCodes(e.do(t, http.MethodGet, "/game/quests", e.gtoken(s), th(s), nil))
		for h := 1; h <= 9; h++ {
			code := fmt.Sprintf("math.g%d.matrix", h)
			if (got[code] != nil) != (h == g) {
				t.Fatalf("grade %d lists %s: %v", g, code, got[code] != nil)
			}
			id := fmt.Sprintf("0193a5f4-6c1e-7b2a-9d1f-0000000b091%d", h)
			r := e.do(t, http.MethodGet, "/game/quests/"+id, e.gtoken(s), th(s), nil)
			if (r.status == http.StatusOK) != (h == g) || (h != g && r.status != http.StatusNotFound) {
				t.Fatalf("grade %d reads the quest of grade %d: %d", g, h, r.status)
			}
		}
		areas := e.listAreas(t, s)
		if h := areas["learning_zone.history"]; (h["accessible"] == true) != (g >= 4) || (g <= 3 && h["lock_reason"] != "grade_not_allowed") {
			t.Fatalf("grade %d history: %v", g, h)
		}
		d, session := e.session(t, s)
		r := e.enter(t, s, d, session, "main_hub", "")
		if r.status != http.StatusOK {
			t.Fatalf("grade %d enter: %d %s", g, r.status, r.raw)
		}
		rooms[g] = r.body["room_id"]
	}
	for _, same := range [][2]int{{1, 2}, {2, 3}, {4, 5}, {6, 7}, {8, 9}} {
		if rooms[same[0]] != rooms[same[1]] {
			t.Errorf("grades %d and %d in different hub rooms", same[0], same[1])
		}
	}
	for _, apart := range [][2]int{{3, 4}, {5, 6}, {7, 8}} {
		if rooms[apart[0]] == rooms[apart[1]] {
			t.Errorf("grades %d and %d share a hub room", apart[0], apart[1])
		}
	}
}
