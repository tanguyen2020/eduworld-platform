//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// party seeds an open competition_team party of members (the first is the leader) and returns its id.
func (e *tenv) party(members ...kid) uuid.UUID {
	e.t.Helper()
	pid := uuid.New()
	l := members[0]
	e.exec(`INSERT INTO multiplayer.parties (party_id, tenant_id, created_at, updated_at, school_id, name, leader_id, party_type, max_members,
		grade_band, status, state_version) VALUES ($1, $2, now(), now(), $3, 'Đội', $4, 'competition_team', 4, 'g6_7', 'open', 1)`,
		pid, l.tenant, l.school, l.id)
	for i, m := range members {
		role := "member"
		if i == 0 {
			role = "leader"
		}
		e.exec(`INSERT INTO multiplayer.party_members (membership_id, tenant_id, created_at, updated_at, party_id, student_id, school_id, role,
			joined_at) VALUES ($1, $2, now(), now(), $3, $4, $5, $6, now() + $7::interval)`, uuid.New(), m.tenant, pid, m.id, m.school, role,
			fmt.Sprintf("%d seconds", i))
	}
	return pid
}

// teamCompleted delivers match.completed of a team match: team 0 (entry A) wins.
func (e *tenv) teamCompleted(match string, a, b []kid) {
	e.t.Helper()
	var results []map[string]any
	for _, k := range a {
		results = append(results, map[string]any{"student_id": k.id, "tenant_id": k.tenant, "school_id": k.school, "team_no": 0, "outcome": "win",
			"score": 200, "correct_count": 2})
	}
	for _, k := range b {
		results = append(results, map[string]any{"student_id": k.id, "tenant_id": k.tenant, "school_id": k.school, "team_no": 1, "outcome": "loss",
			"score": 100, "correct_count": 1})
	}
	e.deliver("match.completed", e.tenantA, map[string]any{"match_id": match, "room_id": match, "mode": "team_battle", "matchmaking_type": "tournament",
		"ranked": false, "rated": false, "is_bot": false, "subject_id": e.subject, "grade_level": 6, "status": "completed", "end_reason": "finished",
		"winner_team": 0, "rule_version_id": uuid.New(), "tournament_id": uuid.New(), "tournament_node_id": uuid.New(), "ended_at": e.clock.Now().UTC(),
		"results": results})
}

// T031 / F107-AC-23, AC-24: a team tournament takes parties (the leader registers; every member eligible and of the
// school; team_size − 1 to team_size members), a non-leader 403 PARTY_NOT_LEADER, a member of another school 422; 8 teams
// make 2 snake groups of 4 played in round robin, then the top 2 of each group meet in a knockout; a team left with fewer
// than team_size − 1 active members forfeits.
func TestF107TeamTournament(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplTeamGroups)
	admin := e.adminOf(e.tenantA, e.schoolA)
	ch := e.createCh(admin, e.tenantA, e.challengeBody(tplTeamGroups, e.schoolA, 6, func(b map[string]any) {
		b["starts_at"] = e.clock.Now().Add(26 * time.Hour).UTC().Format(time.RFC3339)
		b["ends_at"] = e.clock.Now().Add(26*time.Hour + 14*24*time.Hour).UTC().Format(time.RFC3339)
	}), 201)
	tid := tournamentOf(t, ch)
	path := "/competition/tournaments/" + tid
	e.staff(admin, e.tenantA, http.MethodPost, path+":publish", key(), nil)
	teams := make([][]kid, 8)
	for i := range teams {
		teams[i] = e.kidsOf(3, 6)
	}
	parties := make([]uuid.UUID, 8)
	for i, tm := range teams {
		parties[i] = e.party(tm...)
	}
	if r := e.call(teams[0][1], http.MethodPost, path+"/entries", key(), map[string]any{"party_id": parties[0]}); r.status != 403 || r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("a member registers the party: %d %s", r.status, r.raw)
	}
	mixed := []kid{e.kid(e.tenantA, e.schoolA, 6), e.kid(e.tenantA, e.schoolA2, 6), e.kid(e.tenantA, e.schoolA, 6)}
	if r := e.call(mixed[0], http.MethodPost, path+"/entries", key(), map[string]any{"party_id": e.party(mixed...)}); r.status != 422 {
		t.Fatalf("a member of another school: %d %s", r.status, r.raw)
	}
	solo := e.kid(e.tenantA, e.schoolA, 6)
	if r := e.call(solo, http.MethodPost, path+"/entries", key(), map[string]any{"party_id": e.party(solo)}); r.status != 422 {
		t.Fatalf("a party of 1 for teams of 3: %d %s", r.status, r.raw)
	}
	if r := e.call(solo, http.MethodPost, path+"/entries", key(), nil); r.status != 422 {
		t.Fatalf("no party in a team tournament: %d", r.status)
	}
	for i, tm := range teams {
		r := e.call(tm[0], http.MethodPost, path+"/entries", key(), map[string]any{"party_id": parties[i]})
		if r.status != 201 {
			t.Fatalf("team %d: %d %s", i, r.status, r.raw)
		}
	}
	e.clock.Add(26*time.Hour - 14*time.Minute)
	e.tick()
	e.staff(admin, e.tenantA, http.MethodPost, path+"/entries:confirm", key(), nil)
	if r := e.staff(admin, e.tenantA, http.MethodPost, path+":generate-bracket", key(), nil); r.status != 200 {
		t.Fatalf("generate: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM competition.tournament_groups WHERE tournament_id = $1`, tid); n != 2 {
		t.Fatalf("8 teams → 2 groups, got %d", n)
	}
	for _, tm := range teams {
		e.checkIn(tid, tm[0])
	}
	// team 7 keeps only its leader active: 1 < team_size − 1 = 2 → it forfeits its matches.
	e.exec(`UPDATE ref.students SET status = 'inactive' WHERE student_id = ANY($1::uuid[])`, []string{teams[7][1].id.String(), teams[7][2].id.String()})
	byEntry := map[string][]kid{}
	for _, tm := range teams {
		byEntry[e.entryOf(tid, tm[0])] = tm
	}
	gone := e.entryOf(tid, teams[7][0])
	e.clock.Add(15 * time.Minute)
	for round := 0; round < 3; round++ {
		e.tick()
		for _, n := range e.nodes(tid) {
			if n.status != "in_match" || n.bracket != "group" {
				continue
			}
			a, b := byEntry[*n.a], byEntry[*n.b]
			e.accept(*n.match, append(append([]kid{}, a...), b...)...)
			e.teamCompleted(*n.match, a, b)
		}
		for _, n := range e.nodes(tid) {
			if n.bracket == "group" && (*n.a == gone || *n.b == gone) && n.status != "pending" && n.status != "scheduled" {
				if n.winner == nil || *n.winner == gone {
					t.Fatalf("the short team must forfeit: %+v", n)
				}
			}
		}
		e.clock.Add(10*time.Minute + time.Second)
		e.tick()
		e.clock.Add(24 * time.Hour)
	}
	ko := 0
	for _, n := range e.nodes(tid) {
		if n.bracket == "winners" {
			ko++
		}
	}
	if ko != 3 {
		t.Fatalf("top 2 of 2 groups → a knockout of 4 (3 nodes), got %d", ko)
	}
	var standings string
	_ = e.su.QueryRow(`SELECT standings::text FROM competition.tournament_groups WHERE tournament_id = $1 ORDER BY code LIMIT 1`, tid).Scan(&standings)
	if standings == "" || standings == "[]" {
		t.Fatal("group standings are stored")
	}
}
