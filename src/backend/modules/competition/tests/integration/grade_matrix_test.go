//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Grade matrix 1–9 of module competition on PostgreSQL + Valkey (TEST_STRATEGY §6.10), with the band boundaries 3→4,
// 4→5, 5→6 and 7→8.

// TestGradeMatrixMatchmakingQueues (F104-AC-02, MATCHMAKING §2.1 H1, COMPETITION_SYSTEM §3.2): for every grade the
// server puts the ticket in its grade queue (g1…g5, b6_7, b8_9 — never what the client sends); two students of the same
// grade meet at once; neighbour grades meet only inside g6_7 / g8_9 after 30 s of waiting, never across 1–5, 5→6 or
// 7→8; team_battle opens from grade 4.
func TestGradeMatrixMatchmakingQueues(t *testing.T) {
	m := setupMM(t)
	wantKey := map[int]string{1: "g1", 2: "g2", 3: "g3", 4: "g4", 5: "g5", 6: "b6_7", 7: "b6_7", 8: "b8_9", 9: "b8_9"}
	cleanup := func(ks []kid, tickets []string) {
		for i, k := range ks {
			if p := m.proposalOf(k); p != nil {
				m.declineProposal(k, p["proposal_id"].(string), 0)
			}
			m.call(k, http.MethodDelete, "/competition/matchmaking/tickets/"+tickets[i], "", nil)
		}
	}
	for g := 1; g <= 9; g++ {
		a, b := m.kid(m.tenantA, m.schoolA, g), m.kid(m.tenantA, m.schoolA, g)
		m.setOnline(a, b)
		ta := m.ticket(a, http.StatusCreated, map[string]any{"grade_level": 10 - g}).str("ticket_id")
		tk, err := m.store.Ticket(context.Background(), uuid.MustParse(ta))
		if err != nil || tk == nil || tk.Grade != g || tk.GradeKey != wantKey[g] {
			t.Fatalf("grade %d ticket %+v %v", g, tk, err)
		}
		tb := m.ticket(b, http.StatusCreated, nil).str("ticket_id")
		m.clock.Add(time.Second)
		m.tick()
		if m.proposalOf(a) == nil || m.proposalOf(a)["proposal_id"] != m.mustProposal(b) {
			t.Fatalf("grade %d: two students of the grade not proposed", g)
		}
		cleanup([]kid{a, b}, []string{ta, tb})
	}
	for _, p := range []struct {
		lo, hi int
		after  bool // meet once both waited grade_window_g6_9.after_s
	}{{1, 2, false}, {2, 3, false}, {3, 4, false}, {4, 5, false}, {5, 6, false}, {6, 7, true}, {7, 8, false}, {8, 9, true}} {
		a, b := m.kid(m.tenantA, m.schoolA, p.lo), m.kid(m.tenantA, m.schoolA, p.hi)
		m.setOnline(a, b)
		ta := m.ticket(a, http.StatusCreated, nil).str("ticket_id")
		tb := m.ticket(b, http.StatusCreated, nil).str("ticket_id")
		m.clock.Add(29 * time.Second)
		m.tick()
		if m.proposalOf(a) != nil || m.proposalOf(b) != nil {
			t.Fatalf("grades %d and %d proposed after 29 s", p.lo, p.hi)
		}
		m.clock.Add(time.Second)
		m.tick()
		if got := m.proposalOf(a) != nil && m.proposalOf(b) != nil; got != p.after {
			t.Fatalf("grades %d and %d after 30 s: proposed %v, want %v", p.lo, p.hi, got, p.after)
		}
		if !p.after {
			m.clock.Add(2 * time.Minute)
			m.tick()
			if m.proposalOf(a) != nil || m.proposalOf(b) != nil {
				t.Fatalf("grades %d and %d proposed after a long wait", p.lo, p.hi)
			}
		}
		cleanup([]kid{a, b}, []string{ta, tb})
	}
	for g := 1; g <= 9; g++ {
		a, b := m.kid(m.tenantA, m.schoolA, g), m.kid(m.tenantA, m.schoolA, g)
		m.setOnline(a, b)
		if g <= 3 {
			if r := m.ticket(a, http.StatusForbidden, teamBody("", false)); r.code() != "POLICY_DENIED" {
				t.Fatalf("team_battle grade %d: %s", g, r.raw)
			}
			continue
		}
		pid := m.competitionParty(a, b)
		tk := m.ticket(a, http.StatusCreated, teamBody(pid, true)).str("ticket_id")
		m.call(a, http.MethodDelete, "/competition/matchmaking/tickets/"+tk, "", nil)
	}
}

// TestGradeMatrixLeaderboards (F106-AC-16/17, COMPETITION_SYSTEM §9.2, §9.4): 12 students of each grade with points;
// grades 1–3 list only the grade goal and every ranked board is 404; grades 4–5 the top 10 with "nhóm X%" outside it
// and never the last member; grades 6–9 every entry and ±3 around themselves, plus the team board.
func TestGradeMatrixLeaderboards(t *testing.T) {
	r := setupRank(t)
	at := r.clock.Now().Add(-time.Hour)
	base := []string{"grade_goal_weekly"}
	ranked := []string{"friends_effort_weekly", "friends_points_weekly", "friends_xp_weekly", "grade_goal_weekly", "school_arena_season",
		"school_arena_weekly"}
	for g := 1; g <= 9; g++ {
		var ks []kid
		for i := 0; i < 12; i++ {
			k := r.kid(r.tenantA, r.schoolA, g)
			ks = append(ks, k)
			r.points(k, 500-i*10, at.Add(time.Duration(i)*time.Second))
		}
		want := base
		switch {
		case g >= 6:
			want = append(slices.Clone(ranked), "team_season")
		case g >= 4:
			want = ranked
		}
		var codes []string
		for _, d := range r.get(ks[0], "/leaderboards").data() {
			codes = append(codes, d["code"].(string))
		}
		slices.Sort(codes)
		slices.Sort(want)
		if !slices.Equal(codes, want) {
			t.Fatalf("grade %d boards %v, want %v", g, codes, want)
		}
		goal := r.board(lbGradeGoal, ks[0], "")
		if goal.status != http.StatusOK || goal.body["visibility"] != "grade_goal" || len(entriesOf(goal)) != 0 {
			t.Fatalf("grade %d goal: %d %s", g, goal.status, goal.raw)
		}
		last := r.board(lbArenaWeekly, ks[11], "")
		switch {
		case g <= 3:
			if last.status != http.StatusNotFound {
				t.Fatalf("grade %d opens a ranked board: %d", g, last.status)
			}
		case g <= 5:
			me, _ := last.body["me"].(map[string]any)
			if last.status != http.StatusOK || last.body["visibility"] != "top_n_percentile" || len(entriesOf(last)) != 10 || me == nil ||
				me["rank"] != nil || me["percentile_bucket"] == nil {
				t.Fatalf("grade %d last: %s", g, last.raw)
			}
			first := r.board(lbArenaWeekly, ks[0], "")
			for _, e := range entriesOf(first) {
				if e["rank"].(float64) == 12 {
					t.Fatalf("grade %d: the last member is shown to the first", g)
				}
			}
			if first.body["me"].(map[string]any)["rank"].(float64) != 1 {
				t.Fatalf("grade %d first: %s", g, first.raw)
			}
		default:
			mid := r.board(lbArenaWeekly, ks[8], "")
			around, _ := mid.body["around_me"].([]any)
			if mid.status != http.StatusOK || mid.body["visibility"] != "ranked" || len(entriesOf(mid)) != 12 ||
				mid.body["me"].(map[string]any)["rank"].(float64) != 9 || len(around) != 7 || around[0].(map[string]any)["rank"].(float64) != 6 {
				t.Fatalf("grade %d rank 9: %s", g, mid.raw)
			}
		}
	}
}
