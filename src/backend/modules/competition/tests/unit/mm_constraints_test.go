package unit

import (
	"errors"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"

	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
)

// seedConfig is the platform seed of the matchmaking configuration (seed/matchmaking_config.json, CONFIG_DEFAULTS
// mm_* and grade_window_g6_9).
func seedConfig(t *testing.T) mm.Config {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "seed", "matchmaking_config.json"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := mm.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func cand(grade int, mu float64, waited time.Duration, now time.Time) mm.Candidate {
	return mm.Candidate{TicketID: uuid.New(), Students: []uuid.UUID{uuid.New()}, Grade: grade, Rating: mm.Rating{Mu: mu, RD: 350},
		EnqueuedAt: now.Add(-waited)}
}

func queue(mode, typ string, grade int) mm.QueueKey {
	n := 1
	if mode == mm.ModeTeamBattle {
		n = 3
	}
	return mm.QueueKey{Mode: mode, Type: typ, Scope: mm.Scope{Kind: mm.ScopeSchool, ID: uuid.New()}, GradeKey: mm.GradeKey(typ, grade),
		Subject: uuid.New(), TeamSize: n}
}

// F104-AC-02 (TC-MM-003): the grade part of the queue key for every grade — grades 1–5 alone, 6–7 and 8–9 pooled by
// band, grade matchmaking exact everywhere.
func TestGradeKeyTable(t *testing.T) {
	want := map[int]string{1: "g1", 2: "g2", 3: "g3", 4: "g4", 5: "g5", 6: "b6_7", 7: "b6_7", 8: "b8_9", 9: "b8_9"}
	for g, k := range want {
		if got := mm.GradeKey(mm.TypeSkill, g); got != k {
			t.Errorf("grade %d: %s, want %s", g, got, k)
		}
		if got := mm.GradeKey(mm.TypeGrade, g); got != "g"+string(rune('0'+g)) {
			t.Errorf("grade matchmaking %d: %s", g, got)
		}
	}
}

// F104-AC-02: grade compatibility inside one queue — never across grades 1–5, ±1 in grades 6–9 only after both waited
// grade_window_g6_9 (30 s), never across bands.
func TestGradeCompatible(t *testing.T) {
	c := seedConfig(t)
	long, short := 31*time.Second, 5*time.Second
	cases := []struct {
		typ        string
		a, b       int
		wa, wb     time.Duration
		compatible bool
	}{
		{mm.TypeSkill, 3, 3, short, short, true},
		{mm.TypeSkill, 3, 4, 10 * time.Minute, 10 * time.Minute, false},
		{mm.TypeSkill, 1, 9, 10 * time.Minute, 10 * time.Minute, false},
		{mm.TypeSkill, 5, 6, 10 * time.Minute, 10 * time.Minute, false},
		{mm.TypeSkill, 6, 7, short, short, false},
		{mm.TypeSkill, 6, 7, long, short, false},
		{mm.TypeSkill, 6, 7, long, long, true},
		{mm.TypeSkill, 7, 8, long, long, false},
		{mm.TypeSkill, 8, 9, long, long, true},
		{mm.TypeGrade, 6, 7, 10 * time.Minute, 10 * time.Minute, false},
	}
	for _, tc := range cases {
		if got := c.GradeCompatible(tc.typ, tc.a, tc.b, tc.wa, tc.wb); got != tc.compatible {
			t.Errorf("%s %d-%d (%v/%v): %v, want %v", tc.typ, tc.a, tc.b, tc.wa, tc.wb, got, tc.compatible)
		}
	}
}

// F104-AC-02, SC-001: 10 000 random pairings with random grades and ratings, waits up to 10 minutes (past
// mm_bot_after) — zero pairs violate H1.
func TestPairingNeverBreaksGradeConstraint(t *testing.T) {
	c := seedConfig(t)
	r := rand.New(rand.NewPCG(1, 104))
	now := time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)
	pairs := 0
	for pairs < 10000 {
		typ := []string{mm.TypeRandom, mm.TypeSkill, mm.TypeGrade}[r.IntN(3)]
		// one queue per grade key: tickets of a queue share it by construction; mix grades the key allows
		key := 1 + r.IntN(9)
		k := queue(mm.ModeBattle1v1, typ, key)
		var cs []mm.Candidate
		for range 40 {
			g := key
			switch k.GradeKey {
			case "b6_7":
				g = 6 + r.IntN(2)
			case "b8_9":
				g = 8 + r.IntN(2)
			}
			cs = append(cs, cand(g, 1500+r.NormFloat64()*300, time.Duration(r.IntN(600))*time.Second, now))
		}
		for _, grp := range c.Pair(k, cs, mm.Env{Now: now}) {
			a, b := grp.Tickets[0], grp.Tickets[1]
			pairs++
			if a.Grade != b.Grade && (a.Grade <= 5 || b.Grade <= 5 || typ == mm.TypeGrade || math.Abs(float64(a.Grade-b.Grade)) > 1 ||
				mm.GradeKey(typ, a.Grade) != mm.GradeKey(typ, b.Grade)) {
				t.Fatalf("H1 violated: %s grade %d with %d", typ, a.Grade, b.Grade)
			}
			if a.Grade != b.Grade && (now.Sub(a.EnqueuedAt) < 30*time.Second || now.Sub(b.EnqueuedAt) < 30*time.Second) {
				t.Fatalf("±1 grade before grade_window_g6_9: %v %v", now.Sub(a.EnqueuedAt), now.Sub(b.EnqueuedAt))
			}
		}
	}
}

// F104-AC-18, AC-26: request validation of the matchmaking types (friend/tournament never queue, topic needs topic_id,
// party only for team tickets, mode and type must go together).
func TestCheckRequestTypes(t *testing.T) {
	c := seedConfig(t)
	subject, topic, party := uuid.New(), uuid.New(), uuid.New()
	field := func(err error) string {
		var ve mm.ValidationError
		if errors.As(err, &ve) {
			return ve.Field
		}
		return ""
	}
	cases := []struct {
		name string
		r    mm.Request
		want string
	}{
		{"skill ok", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeSkill, SubjectID: subject}, ""},
		{"random ok", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeRandom, SubjectID: subject}, ""},
		{"grade ok", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeGrade, SubjectID: subject}, ""},
		{"subject ok", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeSubject, SubjectID: subject}, ""},
		{"topic ok", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeTopic, SubjectID: subject, TopicID: &topic}, ""},
		{"team solo ok", mm.Request{Mode: mm.ModeTeamBattle, Type: mm.TypeTeam, SubjectID: subject}, ""},
		{"team party ok", mm.Request{Mode: mm.ModeTeamBattle, Type: mm.TypeTeam, SubjectID: subject, PartyID: &party, AllowFill: true}, ""},
		{"friend", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeFriend, SubjectID: subject}, "matchmaking_type"},
		{"tournament", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeTournament, SubjectID: subject}, "matchmaking_type"},
		{"unknown type", mm.Request{Mode: mm.ModeBattle1v1, Type: "abc", SubjectID: subject}, "matchmaking_type"},
		{"mode not queueable", mm.Request{Mode: "coop_challenge", Type: mm.TypeSkill, SubjectID: subject}, "mode"},
		{"unknown mode", mm.Request{Mode: "abc", Type: mm.TypeSkill, SubjectID: subject}, "mode"},
		{"topic missing", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeTopic, SubjectID: subject}, "topic_id"},
		{"topic on skill", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeSkill, SubjectID: subject, TopicID: &topic}, "topic_id"},
		{"team type in 1v1", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeTeam, SubjectID: subject}, "matchmaking_type"},
		{"skill in team mode", mm.Request{Mode: mm.ModeTeamBattle, Type: mm.TypeSkill, SubjectID: subject}, "matchmaking_type"},
		{"party in 1v1", mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeSkill, SubjectID: subject, PartyID: &party}, "party_id"},
		{"allow_fill solo", mm.Request{Mode: mm.ModeTeamBattle, Type: mm.TypeTeam, SubjectID: subject, AllowFill: true}, "allow_fill"},
	}
	for _, tc := range cases {
		err := c.CheckRequest(tc.r)
		if got := field(err); got != tc.want || (tc.want == "" && err != nil) {
			t.Errorf("%s: %v (field %q), want field %q", tc.name, err, got, tc.want)
		}
	}
	// ranked flags (MATCHMAKING §3, data-model §3): skill, subject, team ranked; random, topic, grade unranked.
	for typ, ranked := range map[string]bool{mm.TypeSkill: true, mm.TypeSubject: true, mm.TypeRandom: false, mm.TypeTopic: false, mm.TypeGrade: false} {
		if c.Ranked(mm.ModeBattle1v1, typ) != ranked {
			t.Errorf("ranked %s: want %v", typ, ranked)
		}
	}
	if !c.Ranked(mm.ModeTeamBattle, mm.TypeTeam) || c.Ranked(mm.ModeBattle1v1, mm.TypeBot) {
		t.Error("ranked team / bot")
	}
}

// F104-AC-17, AC-26: players of a ticket — team_battle needs grade 4, a ranked party one grade and a competition_team
// party, subject/topic available for the grade (configuration with a subject list).
func TestCheckPlayers(t *testing.T) {
	c := seedConfig(t)
	subject, other, topic := uuid.New(), uuid.New(), uuid.New()
	party := uuid.New()
	p := func(grade int) mm.Player {
		return mm.Player{ID: uuid.New(), Grade: grade, Active: true, PartyType: "competition_team"}
	}
	rule := func(err error) string {
		var pe mm.PolicyError
		if errors.As(err, &pe) {
			return pe.Rule
		}
		var ve mm.ValidationError
		if errors.As(err, &ve) {
			return "422:" + ve.Field
		}
		return ""
	}
	team := mm.Request{Mode: mm.ModeTeamBattle, Type: mm.TypeTeam, SubjectID: subject}
	if got := rule(c.CheckPlayers(team, []mm.Player{p(3)})); got != mm.RuleModeGrade {
		t.Errorf("team_battle grade 3: %q", got)
	}
	if got := rule(c.CheckPlayers(team, []mm.Player{p(4)})); got != "" {
		t.Errorf("team_battle grade 4: %q", got)
	}
	pt := mm.Request{Mode: mm.ModeTeamBattle, Type: mm.TypeTeam, SubjectID: subject, PartyID: &party}
	if got := rule(c.CheckPlayers(pt, []mm.Player{p(6), p(7), p(6)})); got != mm.RulePartyGrade {
		t.Errorf("mixed party: %q", got)
	}
	if got := rule(c.CheckPlayers(pt, []mm.Player{p(7), p(7), p(7)})); got != "" {
		t.Errorf("party of 3: %q", got)
	}
	friendParty := []mm.Player{p(7), p(7)}
	friendParty[0].PartyType = "friend"
	if got := rule(c.CheckPlayers(pt, friendParty)); got != mm.RulePartyType {
		t.Errorf("friend party: %q", got)
	}
	if got := rule(c.CheckPlayers(pt, []mm.Player{p(7), p(7), p(7), p(7), p(7)})); got != mm.RulePartySize {
		t.Errorf("party of 5: %q", got)
	}
	restricted := p(3)
	restricted.Restrict = true
	if got := rule(c.CheckPlayers(mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeSkill, SubjectID: subject}, []mm.Player{restricted})); got != mm.RuleRestricted {
		t.Errorf("restricted: %q", got)
	}
	c.Subjects = []mm.SubjectConfig{{SubjectID: subject, GradeLevels: []int{3}, TopicIDs: []uuid.UUID{topic}}}
	if got := rule(c.CheckPlayers(mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeSkill, SubjectID: other}, []mm.Player{p(3)})); got != "422:subject_id" {
		t.Errorf("subject not listed: %q", got)
	}
	if got := rule(c.CheckPlayers(mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeSkill, SubjectID: subject}, []mm.Player{p(4)})); got != "422:subject_id" {
		t.Errorf("subject of another grade: %q", got)
	}
	bad := uuid.New()
	if got := rule(c.CheckPlayers(mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeTopic, SubjectID: subject, TopicID: &bad}, []mm.Player{p(3)})); got != "422:topic_id" {
		t.Errorf("topic not listed: %q", got)
	}
}

// F104-AC-04 (TC-MM-002): window(t) of mm_rating_window — 100/200/350/∞ at 0/10/20/40 s.
func TestRatingWindow(t *testing.T) {
	c := seedConfig(t)
	for wait, want := range map[time.Duration]float64{0: 100, 9 * time.Second: 100, 10 * time.Second: 200, 19 * time.Second: 200,
		20 * time.Second: 350, 39 * time.Second: 350, 40 * time.Second: math.Inf(1), 10 * time.Minute: math.Inf(1)} {
		if got := c.Window(wait); got != want {
			t.Errorf("window(%v) = %v, want %v", wait, got, want)
		}
	}
	if c.Widened(9*time.Second) || !c.Widened(10*time.Second) {
		t.Error("search_widened after the first step")
	}
}
