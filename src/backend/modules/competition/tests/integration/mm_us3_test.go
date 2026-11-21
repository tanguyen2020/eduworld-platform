//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// competitionParty is a competition_team party of leader with members (friends of the leader, invited and accepted).
func (m *menv) competitionParty(leader kid, members ...kid) string {
	m.t.Helper()
	id := m.create(leader, http.StatusCreated, map[string]any{"name": "Đội Sao", "party_type": "competition_team", "max_members": 4}).str("party_id")
	for _, b := range members {
		m.friends(leader, b)
		inv := m.invite(leader, id, b, http.StatusCreated).str("invitation_id")
		m.accept(b, inv, http.StatusOK)
	}
	return id
}

// partyEnvelope builds the envelope of the last outbox event typ (what the relay delivers to the consumer).
func (m *menv) partyEnvelope(typ string) outbox.Envelope {
	m.t.Helper()
	var raw []byte
	if err := m.relay.QueryRowContext(context.Background(), `SELECT envelope FROM infra.outbox_events WHERE event_type = $1
		ORDER BY created_at DESC, event_id DESC LIMIT 1`, typ).Scan(&raw); err != nil {
		m.t.Fatal(err)
	}
	var e outbox.Envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		m.t.Fatal(err)
	}
	return e
}

func teamBody(party string, fill bool) map[string]any {
	b := map[string]any{"mode": "team_battle", "matchmaking_type": "team"}
	if party != "" {
		b["party_id"] = party
		b["allow_fill"] = fill
	}
	return b
}

// T028 — F104-AC-14, AC-15 (TC-MM-004): leader only (403 PARTY_NOT_LEADER), every member online (409
// PARTY_MEMBER_NOT_ONLINE), no member with its own ticket, μ_team = Σμ/n and RD_team = sqrt(ΣRD²)/n; a member leaving
// cancels the ticket for every member (party_changed) through the consumer competition.matchmaking_party.
func TestMMPartyTicket(t *testing.T) {
	m := setupMM(t)
	lead := m.kid(m.tenantA, m.schoolA, 7)
	b := m.kid(m.tenantA, m.schoolA, 7)
	c := m.kid(m.tenantA, m.schoolA, 7)
	pid := m.competitionParty(lead, b, c)
	m.ratings.set(lead, 1400)
	m.ratings.set(b, 1500)
	m.ratings.set(c, 1600)
	if r := m.ticket(b, http.StatusForbidden, teamBody(pid, false)); r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("member creates: %s", r.raw)
	}
	m.setOnline(lead, b)
	if r := m.ticket(lead, http.StatusConflict, teamBody(pid, false)); r.code() != "PARTY_MEMBER_NOT_ONLINE" {
		t.Fatalf("offline member: %s", r.raw)
	}
	m.setOnline(c)
	own := m.ticket(c, http.StatusCreated, nil).str("ticket_id")
	if r := m.ticket(lead, http.StatusConflict, teamBody(pid, false)); r.code() != "MM_TICKET_ALREADY_ACTIVE" {
		t.Fatalf("member with its own ticket: %s", r.raw)
	}
	m.call(c, http.MethodDelete, "/competition/matchmaking/tickets/"+own, "", nil)
	r := m.ticket(lead, http.StatusCreated, teamBody(pid, false))
	tk, _ := m.store.Ticket(context.Background(), uuid.MustParse(r.str("ticket_id")))
	if tk == nil || len(tk.Students) != 3 || tk.TeamSize != 3 || tk.Rating.Mu != 1500 || math.Abs(tk.Rating.RD-math.Sqrt(3*120*120)/3) > 0.01 {
		t.Fatalf("team ticket %+v", tk)
	}
	for _, k := range []kid{lead, b, c} {
		if u := m.lastUpdate(k); u == nil || u["ticket_id"] != r.str("ticket_id") {
			t.Fatalf("%s not told: %v", k.nickname, u)
		}
	}
	if x := m.ticket(b, http.StatusConflict, nil); x.code() != "MM_TICKET_ALREADY_ACTIVE" {
		t.Fatalf("member of a queued party: %s", x.raw)
	}
	if x := m.call(b, http.MethodDelete, "/competition/matchmaking/tickets/"+r.str("ticket_id"), "", nil); x.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("member cancels the team ticket: %d %s", x.status, x.raw)
	}
	// c leaves the party: party.member_left → consumer → every member told party_changed
	if x := m.call(c, http.MethodPost, "/multiplayer/parties/"+pid+":leave", "", nil); x.status != http.StatusNoContent {
		t.Fatalf("leave %d %s", x.status, x.raw)
	}
	if err := (consumer.MatchmakingParty{H: m.mm}).Handle(jobCtx(), m.partyEnvelope("party.member_left")); err != nil {
		t.Fatal(err)
	}
	for _, k := range []kid{lead, b, c} {
		u := m.lastUpdate(k)
		if u == nil || u["status"] != "cancelled" || u["cancel_reason"] != "party_changed" {
			t.Fatalf("%s: %v", k.nickname, u)
		}
	}
}

// T028 — F104-AC-15 edge case: the party changes while its ticket is proposed → the proposal is cancelled, the other
// team ticket goes back to its queue.
func TestMMPartyChangedWhileProposed(t *testing.T) {
	m := setupMM(t)
	var all []kid
	newTeam := func() (string, []kid) {
		l := m.kid(m.tenantA, m.schoolA, 7)
		x, y := m.kid(m.tenantA, m.schoolA, 7), m.kid(m.tenantA, m.schoolA, 7)
		m.setOnline(l, x, y)
		all = append(all, l, x, y)
		return m.competitionParty(l, x, y), []kid{l, x, y}
	}
	p1, t1 := newTeam()
	p2, t2 := newTeam()
	r1 := m.ticket(t1[0], http.StatusCreated, teamBody(p1, false))
	r2 := m.ticket(t2[0], http.StatusCreated, teamBody(p2, false))
	m.clock.Add(time.Second)
	m.tick()
	prop := m.proposalOf(t1[0])
	if prop == nil || m.mustProposal(t2[2]) != prop["proposal_id"] {
		t.Fatal("two full parties must meet")
	}
	teams, _ := prop["teams"].([]any)
	side := func(id uuid.UUID) int {
		for i, tm := range teams {
			for _, s := range tm.([]any) {
				if s == id.String() {
					return i
				}
			}
		}
		return -1
	}
	for _, k := range t1 {
		if side(k.id) != side(t1[0].id) {
			t.Fatal("party split")
		}
	}
	if side(t1[0].id) == side(t2[0].id) {
		t.Fatal("both parties on one team")
	}
	if x := m.call(t1[2], http.MethodPost, "/multiplayer/parties/"+p1+":leave", "", nil); x.status != http.StatusNoContent {
		t.Fatalf("leave %d", x.status)
	}
	if err := (consumer.MatchmakingParty{H: m.mm}).Handle(jobCtx(), m.partyEnvelope("party.member_left")); err != nil {
		t.Fatal(err)
	}
	if g := m.getTicket(t1[0], r1.str("ticket_id")); g.str("status") != "cancelled" || g.str("cancel_reason") != "party_changed" {
		t.Fatalf("changed party ticket %s", g.raw)
	}
	if g := m.getTicket(t2[0], r2.str("ticket_id")); g.str("status") != "queued" || g.body["requeued"] != true {
		t.Fatalf("other team not requeued %s", g.raw)
	}
	if m.matches() != 0 {
		t.Fatal("match created")
	}
}

// T028 — F104-AC-16: a party of 2 with allow_fill is completed by a solo ticket of the same queue and keeps its team;
// the solos are drafted.
func TestMMAllowFill(t *testing.T) {
	m := setupMM(t)
	l := m.kid(m.tenantA, m.schoolA, 7)
	x := m.kid(m.tenantA, m.schoolA, 7)
	m.setOnline(l, x)
	pid := m.competitionParty(l, x)
	m.ticket(l, http.StatusCreated, teamBody(pid, true))
	var solos []kid
	for range 4 {
		s := m.kid(m.tenantA, m.schoolA, 7)
		solos = append(solos, s)
		m.clock.Add(time.Second)
		m.ticket(s, http.StatusCreated, teamBody("", false))
	}
	m.tick()
	prop := m.proposalOf(l)
	if prop == nil {
		t.Fatal("no proposal")
	}
	teams := prop["teams"].([]any)
	if len(teams[0].([]any)) != 3 || len(teams[1].([]any)) != 3 {
		t.Fatalf("teams %v", teams)
	}
	for _, tm := range teams {
		ids := tm.([]any)
		if slices.Contains(ids, any(l.id.String())) != slices.Contains(ids, any(x.id.String())) {
			t.Fatal("party split")
		}
	}
	for _, s := range solos {
		if m.mustProposal(s) != prop["proposal_id"] {
			t.Fatal("solo not in the proposal")
		}
	}
}

// T029 — F104-AC-17: a grade without the mode (team_battle below grade 4) and a ranked party of mixed grades get
// 403 POLICY_DENIED.
func TestMMTeamPolicy(t *testing.T) {
	m := setupMM(t)
	g3 := m.kid(m.tenantA, m.schoolA, 3)
	if r := m.ticket(g3, http.StatusForbidden, teamBody("", false)); r.code() != "POLICY_DENIED" {
		t.Fatalf("team_battle grade 3: %s", r.raw)
	}
	l := m.kid(m.tenantA, m.schoolA, 6)
	x := m.kid(m.tenantA, m.schoolA, 7)
	m.setOnline(l, x)
	pid := m.competitionParty(l, x)
	if r := m.ticket(l, http.StatusForbidden, teamBody(pid, true)); r.code() != "POLICY_DENIED" {
		t.Fatalf("mixed party: %s", r.raw)
	}
}
