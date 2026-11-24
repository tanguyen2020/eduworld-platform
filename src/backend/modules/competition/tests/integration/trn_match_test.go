//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// started prepares a single elimination of kids, checks everyone in and starts it (status in_progress, round 1
// proposed). It returns the tournament id and the kid of each entry.
func (e *tenv) started(kids []kid) (string, map[string]kid) {
	e.t.Helper()
	admin := e.adminOf(e.tenantA, e.schoolA)
	tid := e.prepared(tplSingleElim, admin, kids, nil)
	e.checkIn(tid, kids...)
	e.clock.Add(15 * time.Minute)
	e.tick()
	if e.tstatus(tid) != "in_progress" {
		e.t.Fatalf("not started: %s", e.tstatus(tid))
	}
	by := map[string]kid{}
	for _, k := range kids {
		by[e.entryOf(tid, k)] = k
	}
	return tid, by
}

// inMatch returns the nodes of tid in a match.
func (e *tenv) inMatch(tid string) []nodeRow {
	var out []nodeRow
	for _, n := range e.nodes(tid) {
		if n.status == "in_match" {
			out = append(out, n)
		}
	}
	return out
}

// T024 / F107-AC-15, AC-16: notices 10 and 1 minutes before the first matches (once each), the direct proposals at the
// match time (a second pass proposes nothing more), a ready check accepted by both creates the match of the node; a
// no-show forfeits (the present entry wins); both absent voids the node (its next node gets a bye).
func TestF107Scheduler(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	admin := e.adminOf(e.tenantA, e.schoolA)
	kids := e.kidsOf(8, 3)
	tid := e.prepared(tplSingleElim, admin, kids, nil)
	e.checkIn(tid, kids...)
	e.clock.Add(4 * time.Minute) // starts_at − 10 min
	e.tick()
	e.tick()
	if n := len(e.events("tournament.match_scheduled")); n != 4 {
		t.Fatalf("%d notices at −10 min (4 round-1 matches, once each)", n)
	}
	e.clock.Add(9 * time.Minute) // −1 min
	e.tick()
	if n := len(e.events("tournament.match_scheduled")); n != 8 {
		t.Fatalf("%d notices after −1 min", n)
	}
	if len(e.push.of(kids[0].id, "tournament.match.scheduled")) != 2 {
		t.Fatal("tournament.match.scheduled pushed to the student at −10 and −1 min")
	}
	e.clock.Add(time.Minute) // starts_at
	e.tick()
	matches := e.inMatch(tid)
	if len(matches) != 4 {
		t.Fatalf("%d nodes proposed", len(matches))
	}
	e.tick()
	again := e.inMatch(tid)
	for i := range matches {
		if *again[i].match != *matches[i].match {
			t.Fatal("a second pass proposed again")
		}
	}
	by := map[string]kid{}
	for _, k := range kids {
		by[e.entryOf(tid, k)] = k
	}
	n0, n1, n2 := matches[0], matches[1], matches[2]
	e.accept(*n0.match, by[*n0.a], by[*n0.b])
	if e.count(`SELECT count(*) FROM competition.matches WHERE match_id = $1 AND tournament_node_id = $2 AND tournament_id = $3 AND ranked = false`,
		*n0.match, n0.id, tid) != 1 {
		t.Fatal("the match of the node exists (unranked, F107-AC-21)")
	}
	e.accept(*n1.match, by[*n1.a]) // only entry A of node 1 shows up; nobody of node 2
	e.clock.Add(3*time.Minute + 10*time.Second)
	e.tick()
	var st1, st2 string
	var w1 *string
	_ = e.su.QueryRow(`SELECT status, winner_entry_id::text FROM competition.bracket_nodes WHERE node_id = $1`, n1.id).Scan(&st1, &w1)
	_ = e.su.QueryRow(`SELECT status FROM competition.bracket_nodes WHERE node_id = $1`, n2.id).Scan(&st2)
	if st1 != "awaiting_confirm" || w1 == nil || *w1 != *n1.a {
		t.Fatalf("no-show: the present entry wins: %s %v", st1, w1)
	}
	if st2 != "void" {
		t.Fatalf("both absent: void, got %s", st2)
	}
	var st0 string
	_ = e.su.QueryRow(`SELECT status FROM competition.bracket_nodes WHERE node_id = $1`, n0.id).Scan(&st0)
	if st0 != "in_match" {
		t.Fatalf("a running match is not a no-show: %s", st0)
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_entries WHERE entry_id = ANY($1::uuid[]) AND status = 'eliminated'`,
		[]string{*n1.b, *n2.a, *n2.b}); n != 3 {
		t.Fatalf("%d eliminated (forfeit loser + both absent)", n)
	}
	authztest.Cover(t, "getTournament", authztest.CrossTenant, authztest.RoleDenied)
}

// T024 / F107-AC-19: a tournament match aborted by a server failure is played again 2 minutes later with a new ready
// check, at most twice; then the node waits for the organizer, who decides with an override.
func TestF107Reschedule(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	kids := e.kidsOf(4, 3)
	tid, by := e.started(kids)
	n := e.inMatch(tid)[0]
	match := *n.match
	for i := 1; i <= 3; i++ {
		e.accept(match, by[*n.a], by[*n.b])
		e.deliver("match.aborted", e.tenantA, map[string]any{"match_id": match, "room_id": match, "mode": "battle_1v1", "reason": "server_failure",
			"aborted_at": e.clock.Now().UTC(), "participants": []map[string]any{}})
		var status string
		var resched int
		var decision bool
		_ = e.su.QueryRow(`SELECT status, reschedule_count, decision_required FROM competition.bracket_nodes WHERE node_id = $1`, n.id).
			Scan(&status, &resched, &decision)
		if i <= 2 {
			if status != "scheduled" || resched != i {
				t.Fatalf("abort %d: %s %d", i, status, resched)
			}
			e.clock.Add(2*time.Minute + time.Second)
			e.tick()
			var m string
			_ = e.su.QueryRow(`SELECT match_id::text FROM competition.bracket_nodes WHERE node_id = $1`, n.id).Scan(&m)
			if m == match || m == "" {
				t.Fatalf("reschedule %d: a new ready check (%s)", i, m)
			}
			match = m
		} else if status != "awaiting_confirm" || !decision {
			t.Fatalf("after 2 reschedules the organizer decides: %s %v", status, decision)
		}
	}
	admin := e.adminOf(e.tenantA, e.schoolA)
	r := e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+tid+"/nodes/"+n.id+":override", key(),
		map[string]any{"winner_entry_id": *n.b, "reason": "Sự cố máy chủ, xét kết quả luyện tập"})
	if r.status != 200 || r.str("status") != "overridden" || r.str("winner_entry_id") != *n.b {
		t.Fatalf("decision: %d %s", r.status, r.raw)
	}
}

// T025 / F107-AC-17, AC-18 (TC-CMP-009): match.completed moves the winner on once (a replayed event, another delivery of
// the same match change nothing); the node is confirmed when the complaint window ends; the organizer overrides in the
// window with a reason (audit before/after; the final gets the new winner), without a reason 422, after the next node
// started 422 INVALID_STATE_TRANSITION.
func TestF107AdvanceOverride(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	kids := e.kidsOf(4, 3)
	tid, by := e.started(kids)
	semis := e.inMatch(tid)
	s0, s1 := semis[0], semis[1]
	e.accept(*s0.match, by[*s0.a], by[*s0.b])
	e.accept(*s1.match, by[*s1.a], by[*s1.b])
	p := map[string]any{"match_id": *s0.match, "room_id": *s0.match, "mode": "battle_1v1", "matchmaking_type": "tournament", "ranked": false,
		"rated": false, "is_bot": false, "subject_id": e.subject, "grade_level": 3, "status": "completed", "end_reason": "finished", "winner_team": 0,
		"rule_version_id": uuid.New(), "tournament_id": tid, "tournament_node_id": s0.id, "ended_at": e.clock.Now().UTC(),
		"results": []map[string]any{{"student_id": by[*s0.a].id, "tenant_id": e.tenantA, "school_id": e.schoolA, "team_no": 0, "outcome": "win",
			"score": 300, "correct_count": 3}, {"student_id": by[*s0.b].id, "tenant_id": e.tenantA, "school_id": e.schoolA, "team_no": 1,
			"outcome": "loss", "score": 100, "correct_count": 1}}}
	b, _ := json.Marshal(p)
	ev := uuid.New()
	e.redeliver("match.completed", e.tenantA, ev, b)
	e.redeliver("match.completed", e.tenantA, ev, b)         // the same event twice (processed_events)
	e.redeliver("match.completed", e.tenantA, uuid.New(), b) // another delivery of the same match (idempotent by match id)
	final := e.nodes(tid)[2]
	if final.a == nil || *final.a != *s0.a || final.b != nil {
		t.Fatalf("the winner moves on once: %+v", final)
	}
	e.completed(*s1.match, by[*s1.a], by[*s1.b], false) // entry B of semi 1 wins
	admin := e.adminOf(e.tenantA, e.schoolA)
	path := "/competition/tournaments/" + tid + "/nodes/"
	if r := e.staff(admin, e.tenantA, http.MethodPost, path+s1.id+":override", key(), map[string]any{"winner_entry_id": *s1.a}); r.status != 422 {
		t.Fatalf("override without a reason: %d", r.status)
	}
	ov := e.staff(admin, e.tenantA, http.MethodPost, path+s1.id+":override", key(), map[string]any{"winner_entry_id": *s1.a, "reason": "Phát hiện gian lận"})
	if ov.status != 200 || ov.str("winner_entry_id") != *s1.a {
		t.Fatalf("override: %d %s", ov.status, ov.raw)
	}
	final = e.nodes(tid)[2]
	if final.b == nil || *final.b != *s1.a {
		t.Fatalf("the final gets the new winner: %+v", final)
	}
	aud := e.audits("tournament.node_overridden")
	if len(aud) != 1 || aud[0]["reason"] != "Phát hiện gian lận" {
		t.Fatalf("override audit: %v", aud)
	}
	meta, _ := aud[0]["metadata"].(map[string]any)
	if meta["winner_before"] != *s1.b || meta["winner_after"] != *s1.a {
		t.Fatalf("audit before/after: %v", meta)
	}
	// the complaint window of semi 0 ends: confirmed.
	e.clock.Add(10*time.Minute + time.Second)
	e.tick()
	var st string
	_ = e.su.QueryRow(`SELECT status FROM competition.bracket_nodes WHERE node_id = $1`, s0.id).Scan(&st)
	if st != "confirmed" {
		t.Fatalf("complaint window over: %s", st)
	}
	// the final starts at its time; overriding semi 0 now is too late.
	e.clock.Add(5 * time.Minute)
	e.tick()
	if r := e.staff(admin, e.tenantA, http.MethodPost, path+s0.id+":override", key(), map[string]any{"winner_entry_id": *s0.b, "reason": "muộn"}); r.status != 422 ||
		r.code() != "INVALID_STATE_TRANSITION" {
		t.Fatalf("override after the next node started: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "confirmBracketNode", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "overrideBracketNode", authztest.CrossTenant, authztest.RoleDenied)
}

// T026 / F107-AC-17, AC-20, AC-21 (SC-002): the confirmed final completes the tournament (tournament_results, champion,
// tournament.completed) and grants the reward rules of the pinned template once per (tournament, entry, rule) with
// tournament.rewarded and the Competition Points source_type tournament; a second reward run grants nothing; no rating
// changes (tournament matches are unranked).
func TestF107Rewards(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	kids := e.kidsOf(4, 3)
	tid, by := e.started(kids)
	admin := e.adminOf(e.tenantA, e.schoolA)
	for _, n := range e.inMatch(tid) {
		e.accept(*n.match, by[*n.a], by[*n.b])
		e.completed(*n.match, by[*n.a], by[*n.b], true)
	}
	for _, n := range e.nodes(tid) {
		if n.round == 1 {
			if r := e.staff(admin, e.tenantA, http.MethodPost, "/competition/tournaments/"+tid+"/nodes/"+n.id+":confirm", "", nil); r.status != 200 {
				t.Fatalf("confirm early: %d %s", r.status, r.raw)
			}
		}
	}
	e.clock.Add(5 * time.Minute)
	e.tick()
	final := e.inMatch(tid)
	if len(final) != 1 {
		t.Fatalf("the final is proposed: %d", len(final))
	}
	f := final[0]
	e.accept(*f.match, by[*f.a], by[*f.b])
	e.completed(*f.match, by[*f.a], by[*f.b], true)
	e.clock.Add(10*time.Minute + time.Second)
	e.tick()
	if s := e.tstatus(tid); s != "rewarded" {
		t.Fatalf("completed and rewarded: %s", s)
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_results WHERE tournament_id = $1`, tid); n != 4 {
		t.Fatalf("%d results", n)
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_results WHERE tournament_id = $1 AND entry_id = $2 AND final_rank = 1`, tid, *f.a); n != 1 {
		t.Fatal("the winner of the final is champion")
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_entries WHERE entry_id = $1 AND status = 'champion'`, *f.a); n != 1 {
		t.Fatal("champion entry")
	}
	// rules: champion, runner_up, top4 × 2, participation × 4 (everyone completed at least one match) = 8.
	rewards := e.count(`SELECT count(*) FROM competition.tournament_rewards WHERE tournament_id = $1`, tid)
	if rewards != 8 || len(e.events("tournament.rewarded")) != 8 || len(e.events("tournament.completed")) != 1 {
		t.Fatalf("%d rewards, %d events", rewards, len(e.events("tournament.rewarded")))
	}
	champ := by[*f.a]
	if n := e.count(`SELECT COALESCE(sum(amount), 0) FROM competition.competition_point_ledger WHERE source_type = 'tournament' AND source_id = $1
		AND student_id = $2`, tid, champ.id); n != 35 {
		t.Fatalf("champion points (30 + 5 participation): %d", n)
	}
	e.exec(`UPDATE competition.tournaments SET status = 'completed' WHERE tournament_id = $1`, tid) // run the rewards again
	e.tick()
	if e.tstatus(tid) != "rewarded" {
		t.Fatal("the job grants the rewards of a completed tournament")
	}
	if e.count(`SELECT count(*) FROM competition.tournament_rewards WHERE tournament_id = $1`, tid) != 8 || len(e.events("tournament.rewarded")) != 8 ||
		e.count(`SELECT count(*) FROM competition.competition_point_ledger WHERE source_type = 'tournament' AND source_id = $1`, tid) != 4 {
		t.Fatal("a second reward run grants nothing")
	}
	if n := e.count(`SELECT count(*) FROM competition.ratings WHERE student_id = ANY($1::uuid[])`, []string{kids[0].id.String(), kids[1].id.String(),
		kids[2].id.String(), kids[3].id.String()}); n != 0 {
		t.Fatalf("tournament matches change no rating (%d)", n)
	}
	seen := e.checkSchemasOf([]string{"tournament.match_scheduled", "tournament.completed", "tournament.rewarded", "competition_points.awarded"})
	if seen["tournament.rewarded"] != 8 || seen["tournament.completed"] != 1 {
		t.Fatalf("event schemas: %v", seen)
	}
	br := e.call(champ, http.MethodGet, "/competition/tournaments/"+tid+"/bracket", "", nil)
	if br.status != 200 {
		t.Fatalf("bracket: %d %s", br.status, br.raw)
	}
	authztest.Cover(t, "listTournaments", authztest.CrossTenant, authztest.RoleDenied)
}
