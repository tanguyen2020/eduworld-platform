//go:build integration

package integration

import (
	"net/http"
	"sync"
	"testing"
)

// T014 — F100-AC-02 (TC-PTY-012, SC-001): about 100 students each fire createParty, acceptPartyInvitation and
// joinParty at the same moment; every student ends with exactly one active membership and the two other requests
// answer 409 (PARTY_ALREADY_IN_PARTY, or PARTY_FULL) — the advisory lock and the unique partial index hold.
func TestOneActivePartyUnderConcurrency(t *testing.T) {
	e := setupParty(t, popts{})
	const rounds, perRound = 17, 6
	students := 0
	for round := 0; round < rounds; round++ {
		open := e.kidA()
		classParty := e.create(open, http.StatusCreated, map[string]any{"name": "Lớp", "party_type": "quest", "visibility": "class",
			"max_members": 8}).str("party_id")
		host := e.kidA()
		private := e.create(host, http.StatusCreated, map[string]any{"name": "Riêng", "party_type": "quest", "max_members": 8}).str("party_id")
		kids := make([]kid, perRound)
		invs := make([]string, perRound)
		for i := range kids {
			kids[i] = e.kidA()
			invs[i] = e.invite(host, private, kids[i], http.StatusCreated).str("invitation_id")
		}
		var wg sync.WaitGroup
		results := make([][3]presp, perRound)
		for i := range kids {
			for j := 0; j < 3; j++ {
				wg.Add(1)
				go func(i, j int) {
					defer wg.Done()
					switch j {
					case 0:
						results[i][j] = e.call(kids[i], http.MethodPost, "/multiplayer/parties", key(), map[string]any{"name": "Của tôi",
							"party_type": "quest"})
					case 1:
						results[i][j] = e.call(kids[i], http.MethodPost, "/multiplayer/invitations/"+invs[i]+":accept", key(), nil)
					default:
						results[i][j] = e.call(kids[i], http.MethodPost, "/multiplayer/parties/"+classParty+":join", key(), nil)
					}
				}(i, j)
			}
		}
		wg.Wait()
		for i, rs := range results {
			ok := 0
			for _, r := range rs {
				switch {
				case r.status == http.StatusCreated || r.status == http.StatusOK:
					ok++
				case r.status == http.StatusConflict && (r.code() == "PARTY_ALREADY_IN_PARTY" || r.code() == "PARTY_FULL"):
				case r.status == http.StatusGone && r.code() == "PARTY_INVITATION_EXPIRED":
					// the invitation expired because the student entered another party first (F100-AC-14)
				default:
					t.Fatalf("round %d student %d: unexpected %d %s", round, i, r.status, r.raw)
				}
			}
			if ok != 1 {
				t.Fatalf("round %d student %d: %d successes", round, i, ok)
			}
			students++
		}
	}
	if n := e.count(`SELECT count(*) FROM (SELECT student_id FROM multiplayer.party_members WHERE left_at IS NULL AND deleted_at IS NULL
		GROUP BY student_id HAVING count(*) > 1) d`); n != 0 {
		t.Fatalf("students with two active memberships: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM (SELECT party_id FROM multiplayer.party_members WHERE left_at IS NULL AND role = 'leader'
		GROUP BY party_id HAVING count(*) > 1) d`); n != 0 {
		t.Fatalf("parties with two leaders: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.parties p WHERE (SELECT count(*) FROM multiplayer.party_members m
		WHERE m.party_id = p.party_id AND m.left_at IS NULL) > p.max_members`); n != 0 {
		t.Fatalf("parties above max_members: %d", n)
	}
	t.Logf("%d students, 0 double memberships", students)
	// Sequential: a second create with another key → 409 PARTY_ALREADY_IN_PARTY (F100-AC-02, TC-PTY-012).
	an := e.kidA()
	e.create(an, http.StatusCreated, nil)
	if r := e.create(an, http.StatusConflict, nil); r.code() != "PARTY_ALREADY_IN_PARTY" {
		t.Fatalf("second party: %s", r.raw)
	}
}
