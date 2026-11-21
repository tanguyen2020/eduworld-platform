package unit

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
)

// bt0 is the server clock of the battle tests (TEST DOUBLE: the room never reads the wall clock).
var bt0 = time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)

// battleRoom builds a room of mode for grade with n players per team (teams 0 and 1; one team for timed quiz/coop).
type roomOpts struct {
	mode       string
	grade      int
	perTeam    int
	teams      int
	questions  int
	ranked     bool
	tournament bool
	bot        *battle.BotProfile
}

type testRoom struct {
	t       *testing.T
	r       *battle.Room
	now     time.Time
	players []uuid.UUID
	steps   []battle.Step
}

func newRoom(t *testing.T, o roomOpts) *testRoom {
	t.Helper()
	if o.teams == 0 {
		o.teams = 2
	}
	if o.perTeam == 0 {
		o.perTeam = 1
	}
	rules, err := battle.DefaultConfig().RulesFor(o.mode, o.grade, o.tournament)
	if err != nil {
		t.Fatal(err)
	}
	if o.questions > 0 {
		rules.QuestionCount = o.questions
	}
	match := uuid.New()
	tenant, school := uuid.New(), uuid.New()
	spec := battle.Spec{MatchID: match, PlanID: uuid.New(), PlanVersion: "qp1-test", Grade: o.grade, Ranked: o.ranked, Tournament: o.tournament,
		Rules: rules, Bot: o.bot, IsBot: o.bot != nil}
	tr := &testRoom{t: t, now: bt0}
	for team := 0; team < o.teams; team++ {
		for i := 0; i < o.perTeam; i++ {
			id := uuid.New()
			tr.players = append(tr.players, id)
			spec.Players = append(spec.Players, battle.PlayerSpec{ID: id, Tenant: tenant, School: school, Team: team,
				Nickname: fmt.Sprintf("Bạn %d-%d", team, i)})
		}
	}
	for i := 0; i < rules.QuestionCount; i++ {
		spec.Items = append(spec.Items, battle.Item{Index: i, Instance: battle.InstanceID(match, i), Type: "single_choice", Limit: rules.TimeLimit,
			Render: json.RawMessage(`{"stem":{"text":"7 x 8 = ?"},"options":[{"option_id":"o_3b2e"},{"option_id":"o_a91c"}]}`)})
	}
	r, err := battle.New(spec, bt0)
	if err != nil {
		t.Fatal(err)
	}
	tr.r = r
	return tr
}

func (tr *testRoom) record(s battle.Step) battle.Step {
	tr.steps = append(tr.steps, s)
	return s
}

// advance moves the clock by d and ticks.
func (tr *testRoom) advance(d time.Duration) battle.Step {
	tr.now = tr.now.Add(d)
	return tr.record(tr.r.Tick(tr.now))
}

func (tr *testRoom) joinAll() {
	tr.t.Helper()
	for _, p := range tr.players {
		s, err := tr.r.Join(p, uuid.New(), tr.now)
		if err != nil {
			tr.t.Fatal(err)
		}
		tr.record(s)
	}
}

// start joins everyone and runs lobby → countdown → question 0.
func (tr *testRoom) start() {
	tr.t.Helper()
	tr.joinAll()
	tr.advance(0)
	if tr.r.State() != battle.StateCountdown {
		tr.t.Fatalf("state after everyone joined: %s", tr.r.State())
	}
	s := tr.advance(tr.r.Rules().Countdown)
	if tr.r.State() != battle.StateQuestion || !s.Started {
		tr.t.Fatalf("state after countdown: %s (started %v)", tr.r.State(), s.Started)
	}
}

// answer submits for p the current question after d and scores it correct or not.
func (tr *testRoom) answer(p uuid.UUID, after time.Duration, correct bool) {
	tr.t.Helper()
	at := tr.r.ShownAt().Add(after)
	s, pend, err := tr.r.Submit(p, tr.r.CurrentInstance(), json.RawMessage(`{"option_id":"o_3b2e"}`), at)
	if err != nil {
		tr.t.Fatalf("submit: %v", err)
	}
	tr.record(s)
	tr.record(tr.r.Scored(pend.Index, p, correct))
}

// closeAndReveal ticks until the current question closes and reveals it (no reveal data).
func (tr *testRoom) closeAndReveal() {
	tr.t.Helper()
	idx := tr.r.Current()
	if tr.r.State() == battle.StateQuestion {
		s := tr.advance(0)
		if tr.r.State() == battle.StateQuestion { // not everyone answered: wait for the deadline
			s = tr.record(tr.r.Tick(tr.r.DeadlineAt().Add(tr.r.Rules().AnswerGrace + time.Millisecond)))
			tr.now = tr.r.DeadlineAt().Add(tr.r.Rules().AnswerGrace + time.Millisecond)
		}
		if s.Reveal == nil || *s.Reveal != idx {
			tr.t.Fatalf("question %d did not close (state %s)", idx, tr.r.State())
		}
	}
	tr.record(tr.r.Revealed(idx, battle.Reveal{CorrectResponse: json.RawMessage(`{"option_id":"o_3b2e"}`)}, tr.now))
}

// next leaves the reveal pause: the next question shows or the match finalizes.
func (tr *testRoom) next() battle.Step { return tr.advance(tr.r.Rules().RevealFor) }

// messages returns every message of type typ sent so far.
func (tr *testRoom) messages(typ string) []battle.Message {
	var out []battle.Message
	for _, s := range tr.steps {
		for _, m := range s.Messages {
			if m.Type == typ {
				out = append(out, m)
			}
		}
	}
	return out
}

func (tr *testRoom) signals(typ string) []battle.Signal {
	var out []battle.Signal
	for _, s := range tr.steps {
		for _, g := range s.Signals {
			if g.Type == typ {
				out = append(out, g)
			}
		}
	}
	return out
}

func (tr *testRoom) score(p uuid.UUID) int {
	for _, pr := range tr.r.Result().Players {
		if pr.ID == p {
			return pr.Score
		}
	}
	tr.t.Fatalf("no result for %s", p)
	return 0
}

func (tr *testRoom) outcome(p uuid.UUID) string {
	for _, pr := range tr.r.Result().Players {
		if pr.ID == p {
			return pr.Outcome
		}
	}
	return ""
}

// noAnswerData fails when a message carries answer data before its question closed.
func noAnswerData(t *testing.T, m battle.Message) {
	t.Helper()
	b, _ := json.Marshal(m.Payload)
	for _, k := range []string{"answer_key", "correct_response", "correct_option_id", "scoring_rule", "explanation", `"feedback"`} {
		if strings.Contains(string(b), k) {
			t.Fatalf("%s carries %s: %s", m.Type, k, b)
		}
	}
}
