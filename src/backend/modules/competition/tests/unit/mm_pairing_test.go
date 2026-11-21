package unit

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
)

var t0 = time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)

// F104-AC-04: a pair needs |Δμ| ≤ window(t) of the longest wait; past 40 s rating is ignored, hard constraints kept.
func TestPairWithinWindow(t *testing.T) {
	c := seedConfig(t)
	k := queue(mm.ModeBattle1v1, mm.TypeSkill, 3)
	a, b := cand(3, 1500, 2*time.Second, t0), cand(3, 1550, time.Second, t0)
	if g := c.Pair(k, []mm.Candidate{a, b}, mm.Env{Now: t0}); len(g) != 1 {
		t.Fatalf("Δμ 50 at 2 s: %d groups", len(g)) // SC-002: grade 3, Δμ 50 → paired at once
	}
	far := cand(3, 1650, time.Second, t0)
	if g := c.Pair(k, []mm.Candidate{a, far}, mm.Env{Now: t0}); len(g) != 0 {
		t.Fatalf("Δμ 150 at 2 s: paired")
	}
	a.EnqueuedAt = t0.Add(-11 * time.Second)
	if g := c.Pair(k, []mm.Candidate{a, far}, mm.Env{Now: t0}); len(g) != 1 {
		t.Fatalf("Δμ 150 at 11 s (window 200): not paired")
	}
	huge := cand(3, 2600, time.Second, t0)
	a.EnqueuedAt = t0.Add(-41 * time.Second)
	if g := c.Pair(k, []mm.Candidate{a, huge}, mm.Env{Now: t0}); len(g) != 1 {
		t.Fatalf("Δμ 1100 at 41 s (window ∞): not paired")
	}
	other := cand(4, 1500, 10*time.Minute, t0) // grade 4 in the grade-3 queue (never by construction): never paired
	if g := c.Pair(k, []mm.Candidate{a, other}, mm.Env{Now: t0}); len(g) != 0 {
		t.Fatal("grade 3 paired with grade 4 after 10 minutes")
	}
}

// F104-AC-04: the oldest ticket chooses first and takes the partner of smallest cost (rating, recent opponent; no
// class term since ADR-0095).
func TestPairOldestFirstLowestCost(t *testing.T) {
	c := seedConfig(t)
	k := queue(mm.ModeBattle1v1, mm.TypeSkill, 3)
	old := cand(3, 1500, 30*time.Second, t0)
	near := cand(3, 1520, 2*time.Second, t0)
	mid := cand(3, 1600, 3*time.Second, t0)
	g := c.Pair(k, []mm.Candidate{mid, near, old}, mm.Env{Now: t0})
	if len(g) != 1 || g[0].Tickets[0].TicketID != old.TicketID || g[0].Tickets[1].TicketID != near.TicketID {
		t.Fatalf("oldest should take the nearest rating: %+v", g)
	}
	// recent opponent: the nearest was met in the last 30 minutes → the other one is cheaper
	recent := func(x, y uuid.UUID) bool {
		return (x == old.Students[0] && y == near.Students[0]) || (y == old.Students[0] && x == near.Students[0])
	}
	g = c.Pair(k, []mm.Candidate{mid, near, old}, mm.Env{Now: t0, Recent: recent})
	if len(g) != 1 || g[0].Tickets[1].TicketID != mid.TicketID {
		t.Fatalf("recent opponent not avoided: %+v", g)
	}
	// ADR-0095: no class term in ranked play — the nearest rating still wins.
	other := cand(3, 1530, 2*time.Second, t0)
	g = c.Pair(k, []mm.Candidate{near, other, old}, mm.Env{Now: t0})
	if len(g) != 1 || g[0].Tickets[1].TicketID != near.TicketID {
		t.Fatalf("ranked: nearest rating expected: %+v", g)
	}
}

// F104-AC-08 (H6): two students who blocked each other are never paired.
func TestPairSkipsBlocked(t *testing.T) {
	c := seedConfig(t)
	k := queue(mm.ModeBattle1v1, mm.TypeSkill, 3)
	a, b, x := cand(3, 1500, 50*time.Second, t0), cand(3, 1500, 40*time.Second, t0), cand(3, 1800, time.Second, t0)
	blocked := func(p, q uuid.UUID) bool {
		return (p == a.Students[0] && q == b.Students[0]) || (q == a.Students[0] && p == b.Students[0])
	}
	g := c.Pair(k, []mm.Candidate{a, b, x}, mm.Env{Now: t0, Blocked: blocked})
	for _, grp := range g {
		ids := grp.TicketIDs()
		if slices.Contains(ids, a.TicketID) && slices.Contains(ids, b.TicketID) {
			t.Fatal("blocked pair in a proposal")
		}
	}
	if len(g) != 1 {
		t.Fatalf("a must pair with x: %d groups", len(g))
	}
}

// F104-AC-18: random matchmaking is FIFO and ignores rating.
func TestRandomIsFIFO(t *testing.T) {
	c := seedConfig(t)
	k := queue(mm.ModeBattle1v1, mm.TypeRandom, 3)
	a, b, x := cand(3, 900, 3*time.Second, t0), cand(3, 2500, 2*time.Second, t0), cand(3, 905, time.Second, t0)
	g := c.Pair(k, []mm.Candidate{x, b, a}, mm.Env{Now: t0})
	if len(g) != 1 || g[0].Tickets[0].TicketID != a.TicketID || g[0].Tickets[1].TicketID != b.TicketID {
		t.Fatalf("random must pair the two oldest: %+v", g)
	}
}

// Edge case: above bucket_threshold (500) tickets the bucketed search gives valid pairs in < 1 s.
func TestBucketedQueue(t *testing.T) {
	c := seedConfig(t)
	k := queue(mm.ModeBattle1v1, mm.TypeSkill, 3)
	var cs []mm.Candidate
	for i := range 1200 {
		cs = append(cs, cand(3, 1000+float64(i%700), time.Duration(i%9)*time.Second, t0))
	}
	start := time.Now()
	g := c.Pair(k, cs, mm.Env{Now: t0})
	if time.Since(start) > time.Second {
		t.Fatalf("pairing 1200 tickets took %v", time.Since(start))
	}
	if len(g) < 500 {
		t.Fatalf("only %d pairs", len(g))
	}
	seen := map[uuid.UUID]bool{}
	for _, grp := range g {
		a, b := grp.Tickets[0], grp.Tickets[1]
		if a.Rating.Mu-b.Rating.Mu > 200 || b.Rating.Mu-a.Rating.Mu > 200 {
			t.Fatalf("pair outside window: %v %v", a.Rating.Mu, b.Rating.Mu)
		}
		for _, id := range grp.TicketIDs() {
			if seen[id] {
				t.Fatal("ticket in two pairs")
			}
			seen[id] = true
		}
	}
}

func teamParty(size int, mu float64, waited time.Duration, fill bool) mm.Candidate {
	p := uuid.New()
	c := mm.Candidate{TicketID: uuid.New(), Grade: 7, Rating: mm.Rating{Mu: mu, RD: 200}, EnqueuedAt: t0.Add(-waited), PartyID: &p, AllowFill: fill}
	for range size {
		c.Students = append(c.Students, uuid.New())
	}
	return c
}

// F104-AC-14: μ_team = Σμ/n, RD_team = sqrt(ΣRD²)/n.
func TestTeamRating(t *testing.T) {
	r := mm.TeamRating([]mm.Rating{{Mu: 1400, RD: 300}, {Mu: 1600, RD: 400}})
	if r.Mu != 1500 || r.RD != 250 {
		t.Fatalf("team rating %+v", r)
	}
}

// F104-AC-16 (TC-MM-004): parties keep their team; solo tickets are spread by snake draft (A1, B1, B2, A2…); a party
// without allow_fill only plays as a full team; with allow_fill solos complete it.
func TestTeamDraft(t *testing.T) {
	c := seedConfig(t)
	k := queue(mm.ModeTeamBattle, mm.TypeTeam, 7)
	p1 := teamParty(3, 1500, 20*time.Second, false)
	var solos []mm.Candidate
	for i, mu := range []float64{1600, 1550, 1500} {
		s := cand(7, mu, time.Duration(10-i)*time.Second, t0)
		solos = append(solos, s)
	}
	g := c.Pair(k, append([]mm.Candidate{p1}, solos...), mm.Env{Now: t0})
	if len(g) != 1 {
		t.Fatalf("party of 3 + 3 solos: %d groups", len(g))
	}
	team := func(id uuid.UUID) int {
		for i, tm := range g[0].Teams {
			if slices.Contains(tm, id) {
				return i
			}
		}
		return -1
	}
	pt := team(p1.Students[0])
	for _, s := range p1.Students {
		if team(s) != pt {
			t.Fatal("party split across teams")
		}
	}
	for _, s := range solos {
		if team(s.Students[0]) == pt {
			t.Fatal("solo in the full party team")
		}
	}
	// two half parties with allow_fill and two solos: each party keeps its team, solos drafted
	a, b := teamParty(2, 1500, 30*time.Second, true), teamParty(2, 1500, 25*time.Second, true)
	s1, s2 := cand(7, 1700, 5*time.Second, t0), cand(7, 1300, 4*time.Second, t0)
	g = c.Pair(k, []mm.Candidate{a, b, s1, s2}, mm.Env{Now: t0})
	if len(g) != 1 || len(g[0].Teams[0]) != 3 || len(g[0].Teams[1]) != 3 {
		t.Fatalf("allow_fill parties: %+v", g)
	}
	if team(s1.Students[0]) == team(s2.Students[0]) {
		t.Fatal("snake draft must split the best and the next solo")
	}
	// a half party without allow_fill never plays
	half := teamParty(2, 1500, 40*time.Second, false)
	g = c.Pair(k, []mm.Candidate{half, cand(7, 1500, 3*time.Second, t0), cand(7, 1500, 2*time.Second, t0), cand(7, 1500, time.Second, t0),
		cand(7, 1500, time.Second, t0)}, mm.Env{Now: t0})
	for _, grp := range g {
		if slices.Contains(grp.TicketIDs(), half.TicketID) {
			t.Fatal("half party without allow_fill was filled")
		}
	}
	// pure snake order on six solos: A1 B1 B2 A2 A3 B3
	var six []mm.Candidate
	for i, mu := range []float64{1600, 1500, 1400, 1300, 1200, 1100} {
		six = append(six, cand(7, mu, time.Duration(10-i)*time.Second, t0))
	}
	teams, ok := mm.Draft(six, 3)
	if !ok {
		t.Fatal("draft of six solos")
	}
	want := [2][]uuid.UUID{{six[0].Students[0], six[3].Students[0], six[4].Students[0]}, {six[1].Students[0], six[2].Students[0], six[5].Students[0]}}
	if !slices.Equal(teams[0], want[0]) || !slices.Equal(teams[1], want[1]) {
		t.Fatalf("snake order: %v", teams)
	}
}

// F104-AC-06: ready check outcome — decline cancels the decliner's ticket and requeues the others; timeout requeues
// the tickets whose students all accepted.
func TestProposalResolution(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	ta, tb := uuid.New(), uuid.New()
	p := mm.Proposal{ID: uuid.New(), Tickets: []mm.ProposalTicket{{TicketID: ta, Students: []uuid.UUID{a}}, {TicketID: tb, Students: []uuid.UUID{b}}},
		Accepted: map[uuid.UUID]bool{a: true}}
	r := p.Decline(b)
	if !slices.Equal(r.Cancel, []uuid.UUID{tb}) || !slices.Equal(r.Requeue, []uuid.UUID{ta}) || r.Reason != mm.CancelDeclined {
		t.Fatalf("decline: %+v", r)
	}
	r = p.Timeout()
	if !slices.Equal(r.Cancel, []uuid.UUID{tb}) || !slices.Equal(r.Requeue, []uuid.UUID{ta}) || !slices.Equal(r.Blamed, []uuid.UUID{b}) {
		t.Fatalf("timeout: %+v", r)
	}
	if p.AllAccepted() {
		t.Fatal("not all accepted")
	}
	p.Accepted[b] = true
	if !p.AllAccepted() {
		t.Fatal("all accepted")
	}
}

// F104-AC-19, AC-20: the duel aggregate — 60 s, only the opponent answers, expired duels are gone.
func TestDuelLifecycle(t *testing.T) {
	ch, op := uuid.New(), uuid.New()
	d := mm.NewDuel(uuid.New(), uuid.New(), uuid.New(), ch, op, uuid.New(), nil, t0)
	if !d.ExpiresAt.Equal(t0.Add(60 * time.Second)) {
		t.Fatal("duel TTL")
	}
	if err := d.Accept(ch, uuid.New(), t0); !errors.Is(err, mm.ErrNotDuelOpponent) {
		t.Fatalf("challenger accepts: %v", err)
	}
	late := d
	if err := late.Accept(op, uuid.New(), t0.Add(61*time.Second)); !errors.Is(err, mm.ErrDuelGone) {
		t.Fatalf("accept after 61 s: %v", err)
	}
	if err := d.Accept(op, uuid.New(), t0.Add(59*time.Second)); err != nil || d.Status != mm.DuelAccepted || d.MatchID == nil {
		t.Fatalf("accept: %v %+v", err, d)
	}
	e := mm.NewDuel(uuid.New(), uuid.New(), uuid.New(), ch, op, uuid.New(), nil, t0)
	if e.Expire(t0.Add(59*time.Second)) || !e.Expire(t0.Add(60*time.Second)) || e.Status != mm.DuelExpired {
		t.Fatal("expire")
	}
	if err := e.Decline(op, t0.Add(61*time.Second)); !errors.Is(err, mm.ErrDuelGone) {
		t.Fatalf("decline expired: %v", err)
	}
}

// H5: allowed hours of the school (Asia/Ho_Chi_Minh by default; to exclusive; windows across midnight).
func TestAllowedHours(t *testing.T) {
	h := mm.ParseAllowedHours([]byte(`{"allowed_hours":{"windows":[{"days":["mon","tue","wed","thu","fri"],"from":"07:00","to":"17:00"}]}}`))
	if h == nil {
		t.Fatal("parse")
	}
	// 2026-11-23 is a Monday; 03:00 UTC = 10:00 ICT
	mon10 := time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)
	if !h.Contains(mon10) || h.Contains(mon10.Add(7*time.Hour)) || h.Contains(mon10.Add(-48*time.Hour)) {
		t.Fatal("window")
	}
	if mm.ParseAllowedHours([]byte(`{}`)) != nil || mm.ParseAllowedHours([]byte(`{"allowed_hours":null}`)) != nil {
		t.Fatal("no window = no limit")
	}
}
