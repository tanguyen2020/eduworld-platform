package unit

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
)

// F105-AC-17 (T029, TC-MM-004): team battle from grade 4 only; 10 questions; each question one member per team answers
// the main turn (rotating), the others a support turn worth 30 %; the team total decides.
func TestBattleTeamRotation(t *testing.T) {
	if _, err := battle.DefaultConfig().RulesFor(battle.ModeTeam, 3, false); !errors.Is(err, battle.ErrModeGrade) {
		t.Fatalf("team battle in grade 3: %v", err)
	}
	r, err := battle.DefaultConfig().RulesFor(battle.ModeTeam, 6, false)
	if err != nil || r.QuestionCount != 10 || r.SupportPercent != 30 || !r.Rotation {
		t.Fatalf("team rules: %+v %v", r, err)
	}
	tr := newRoom(t, roomOpts{mode: battle.ModeTeam, grade: 4, perTeam: 3, questions: 4}) // grade 4: no speed bonus
	tr.start()
	team0, team1 := tr.players[:3], tr.players[3:]
	mains := map[uuid.UUID]int{}
	for i := 0; i < 4; i++ {
		m0, m1 := tr.r.MainTurn(0), tr.r.MainTurn(1)
		if m0 != team0[i%3] || m1 != team1[i%3] {
			t.Fatalf("question %d main turns %s/%s", i, m0, m1)
		}
		mains[m0]++
		for _, p := range tr.players {
			tr.answer(p, 2*time.Second, true)
		}
		tr.closeAndReveal()
		tr.next()
	}
	// Per team and question: main 100 + 2 supports × 30 = 160; the last question ×2 → 3 × 160 + 320 = 800.
	res := tr.r.Result()
	total := map[int]int{}
	for _, p := range res.Players {
		total[p.Team] += p.Score
	}
	if total[0] != 800 || total[1] != 800 || res.WinnerTeam != nil {
		t.Fatalf("team totals %v", total)
	}
	if mains[team0[0]] != 2 || mains[team0[1]] != 1 || mains[team0[2]] != 1 {
		t.Fatalf("rotation %v", mains)
	}
}

// F105-AC-20 (T032): the practice bot answers with the configured accuracy and delays, carries the Robot label, the
// match is never rated and the student's answers still count.
func TestBattleBot(t *testing.T) {
	profile := battle.BotProfile{Accuracy: 0.7, MinMS: 3000, MaxMS: 9000}
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 5, teams: 1, questions: 7, bot: &profile})
	tr.start()
	snap := tr.r.Snapshot(tr.players[0])
	parts := snap.Payload["participants"].([]map[string]any)
	var robot bool
	for _, p := range parts {
		if p["bot"] == true && p["player"].(map[string]any)["nickname"] == "Robot" {
			robot = true
		}
	}
	if !robot {
		t.Fatalf("no labelled bot in the snapshot: %+v", parts)
	}
	for i := 0; i < 7; i++ {
		tr.answer(tr.players[0], 4*time.Second, true)
		tr.advance(10 * time.Second) // the bot answers within 3–9 s
		if tr.r.State() == battle.StateQuestion {
			t.Fatalf("question %d not closed after the bot delay", i)
		}
		tr.closeAndReveal()
		tr.next()
	}
	res := tr.r.Result()
	if res.Rated || res.Bot == nil || len(res.Players) != 1 || res.Players[0].Score != 800 {
		t.Fatalf("bot result: %+v bot %+v", res, res.Bot)
	}
	if res.Bot.Correct < 2 || res.Bot.Correct > 7 {
		t.Fatalf("bot accuracy: %d/7", res.Bot.Correct)
	}
	// The same match gives the same bot answers (deterministic per match and question).
	if a, b := battle.BotAnswer(profile, tr.r.MatchID(), 3), battle.BotAnswer(profile, tr.r.MatchID(), 3); a != b {
		t.Fatalf("bot answer not reproducible: %+v %+v", a, b)
	}
	var correct int
	for i := 0; i < 2000; i++ {
		if battle.BotAnswer(profile, uuid.New(), i%7).Correct {
			correct++
		}
	}
	if correct < 1300 || correct > 1500 {
		t.Fatalf("bot accuracy over 2000 answers: %d", correct)
	}
}

// F105-AC-21 (T032): a tournament match uses the 1v1 rules with a 60 s grace at most twice; a third disconnection or a
// grace that runs out forfeits; a tournament match is never rated.
func TestBattleTournament(t *testing.T) {
	r, err := battle.DefaultConfig().RulesFor(battle.Mode1v1, 7, true)
	if err != nil || r.Grace != 60*time.Second || r.MaxGraces != 2 || r.OutOutcome != battle.OutcomeForfeit {
		t.Fatalf("tournament rules: %+v %v", r, err)
	}
	for _, third := range []bool{true, false} {
		tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 7, tournament: true, ranked: false})
		a, b := tr.players[0], tr.players[1]
		tr.start()
		loops := 2 // two graces used, the third disconnection forfeits
		if !third {
			loops = 1 // one grace used, the second runs out
		}
		for i := 0; i < loops; i++ {
			tr.record(tr.r.Disconnect(b, tr.now))
			tr.now = tr.now.Add(10 * time.Second)
			if _, err := tr.r.Join(b, uuid.New(), tr.now); err != nil {
				t.Fatalf("rejoin %d: %v", i, err)
			}
		}
		var s battle.Step
		if third {
			s = tr.record(tr.r.Disconnect(b, tr.now))
		} else {
			tr.record(tr.r.Disconnect(b, tr.now))
			s = tr.advance(61 * time.Second)
		}
		if !s.Finish || tr.outcome(b) != battle.OutcomeForfeit || tr.outcome(a) != battle.OutcomeWin {
			t.Fatalf("third=%v: outcome %s/%s", third, tr.outcome(a), tr.outcome(b))
		}
		res := tr.r.Result()
		if res.Rated || res.EndReason != battle.EndForfeit {
			t.Fatalf("tournament result: %+v", res)
		}
	}
}

// F105-AC-19 (T031): a timed quiz takes 1–40 students on the same questions with the 1v1 scoring (no speed bonus
// below grade 6); from grade 4 the end shows the top 5 and the student's own rank; grades 1–3 only see their result and the grade goal (ADR-0095).
func TestBattleTimedQuiz(t *testing.T) {
	if _, err := battle.New(battle.Spec{MatchID: uuid.New(), Rules: mustRules(t, battle.ModeTimedQuiz, 4), Players: make([]battle.PlayerSpec, 41)}, bt0); !errors.Is(err, battle.ErrFull) {
		t.Fatalf("41 players: %v", err)
	}
	for _, grade := range []int{4, 2} {
		tr := newRoom(t, roomOpts{mode: battle.ModeTimedQuiz, grade: grade, perTeam: 40, teams: 1, questions: 2})
		tr.start()
		for i := 0; i < 2; i++ {
			for k, p := range tr.players {
				tr.answer(p, time.Duration(1+k%5)*time.Second, k%3 != 0)
			}
			tr.closeAndReveal()
			tr.next()
		}
		for _, m := range tr.messages("battle.question.revealed") {
			if len(m.To) != 1 || len(m.Payload["scoreboard"].([]map[string]any)) != 1 {
				t.Fatalf("grade %d: a timed quiz scoreboard shows other students: %+v", grade, m)
			}
		}
		end := tr.r.Ended(tr.now)
		if len(end.Messages) != 40 {
			t.Fatalf("grade %d: %d match.ended messages, want one per student", grade, len(end.Messages))
		}
		for _, m := range end.Messages {
			_, hasTop := m.Payload["top"]
			_, hasRank := m.Payload["my_rank"]
			goal, hasGoal := m.Payload["grade_goal"].(map[string]any)
			if grade >= 4 && (!hasTop || !hasRank || len(m.Payload["top"].([]map[string]any)) != 5) {
				t.Fatalf("grade %d end: %+v", grade, m.Payload)
			}
			if grade < 4 && (hasTop || hasRank || !hasGoal || goal["target"] == nil) {
				t.Fatalf("grade %d end shows a personal rank: %+v", grade, m.Payload)
			}
		}
		for _, p := range tr.r.Result().Players {
			if p.Score%100 != 0 {
				t.Fatalf("grade %d: speed bonus in a timed quiz below grade 6: %d", grade, p.Score)
			}
		}
	}
}

// F105-AC-18 (T030, domain part): a co-op challenge has a collective goal and a server clock, no winner, no personal
// rank and no rating; every member's answers are its own.
func TestBattleCoop(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.ModeCoop, grade: 2, perTeam: 3, teams: 1, ranked: false})
	tr.start()
	if len(tr.messages("coop.task.assigned")) != 3 {
		t.Fatalf("coop tasks: %d", len(tr.messages("coop.task.assigned")))
	}
	n := tr.r.Rules().QuestionCount
	for i := 0; i < n; i++ {
		for k, p := range tr.players {
			tr.answer(p, 2*time.Second, k == 0)
		}
		tr.closeAndReveal()
		tr.next()
	}
	res := tr.r.Result()
	if res.Rated || res.WinnerTeam != nil || res.GroupGoal == nil || !res.GroupGoal.Reached || res.GroupGoal.Achieved != n {
		t.Fatalf("coop result: %+v goal %+v", res, res.GroupGoal)
	}
	for _, p := range res.Players {
		if p.Outcome != battle.OutcomeNoContest {
			t.Fatalf("coop outcome %s", p.Outcome)
		}
	}
	if len(tr.messages("coop.progress.updated")) != n {
		t.Fatalf("progress updates: %d", len(tr.messages("coop.progress.updated")))
	}
	end := tr.r.Ended(tr.now)
	if len(end.Messages) != 1 || end.Messages[0].Type != "coop.session.ended" || end.Messages[0].Payload["outcome"] != "completed" {
		t.Fatalf("coop end: %+v", end.Messages)
	}
}

// F105-AC-24 (T037): three answers in a row faster than cheat_fast_answer_flag (400 ms from grade 6, 600 ms below) raise
// one integrity signal and drop the speed bonus of those answers; the points and the result stay.
func TestBattleFastAnswerSignal(t *testing.T) {
	for _, c := range []struct {
		grade int
		fast  time.Duration
	}{{7, 350 * time.Millisecond}, {3, 550 * time.Millisecond}} {
		tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: c.grade, questions: 4, ranked: true})
		a, b := tr.players[0], tr.players[1]
		tr.start()
		for i := 0; i < 4; i++ {
			tr.answer(a, c.fast, true)
			tr.answer(b, 5*time.Second, false)
			tr.closeAndReveal()
			tr.next()
		}
		sig := tr.signals(battle.SignalFastAnswer)
		if len(sig) != 1 || sig[0].Student != a {
			t.Fatalf("grade %d: %d fast_answer signals", c.grade, len(sig))
		}
		// 3 × 100 + 200 (last ×2), no bonus at all although grade 7 answers had time left.
		if got := tr.score(a); got != 500 {
			t.Fatalf("grade %d: score %d", c.grade, got)
		}
		if tr.outcome(a) != battle.OutcomeWin {
			t.Fatalf("grade %d: a fast answer changed the result: %s", c.grade, tr.outcome(a))
		}
		for _, q := range tr.r.Result().Questions {
			for _, an := range q.Answers {
				if an.Bonus != 0 {
					t.Fatalf("grade %d question %d: bonus %d kept", c.grade, q.Index, an.Bonus)
				}
			}
		}
	}
	// Two fast answers then a normal one: no signal, bonuses kept (grade 7).
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 7, questions: 3, ranked: true})
	tr.start()
	for i, d := range []time.Duration{300 * time.Millisecond, 300 * time.Millisecond, 4 * time.Second} {
		tr.answer(tr.players[0], d, true)
		tr.answer(tr.players[1], time.Second, false)
		tr.closeAndReveal()
		tr.next()
		_ = i
	}
	if len(tr.signals(battle.SignalFastAnswer)) != 0 {
		t.Fatal("signal without three fast answers in a row")
	}
	if tr.score(tr.players[0]) <= 400 {
		t.Fatalf("bonuses dropped without a signal: %d", tr.score(tr.players[0]))
	}
}

func mustRules(t *testing.T, mode string, grade int) battle.Rules {
	t.Helper()
	r, err := battle.DefaultConfig().RulesFor(mode, grade, false)
	if err != nil {
		t.Fatal(fmt.Errorf("%s grade %d: %w", mode, grade, err))
	}
	return r
}
