package unit

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/chatid"
)

// Delta F109 2026-11-27 (AsyncAPI): speed_bonus_enabled, primary_user_ids, chat_channel_ids, the option key alias and
// the encouragement after a loss streak — all decided by the server from the pinned rules, never by the client.

// speed_bonus_enabled follows speed_bonus_min_grade (6): no bonus hint for grades 1–5 (F109-AC-09).
func TestBattleDeltaSpeedBonusEnabled(t *testing.T) {
	for _, c := range []struct {
		grade int
		want  bool
	}{{3, false}, {5, false}, {6, true}, {9, true}} {
		tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: c.grade, questions: 2})
		tr.start()
		cd := tr.messages("battle.countdown.started")
		if len(cd) != 1 || cd[0].Payload["speed_bonus_enabled"] != c.want {
			t.Fatalf("grade %d countdown: %+v", c.grade, cd)
		}
		snap := tr.r.Snapshot(tr.players[0])
		if snap.Payload["speed_bonus_enabled"] != c.want {
			t.Fatalf("grade %d snapshot speed_bonus_enabled %v", c.grade, snap.Payload["speed_bonus_enabled"])
		}
	}
}

// primary_user_ids: in a team battle each question names the main turn of every team (rotation), the snapshot too; a
// 1v1 never has it.
func TestBattleDeltaPrimaryTurns(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.ModeTeam, grade: 5, perTeam: 2, questions: 3})
	tr.start()
	for q := 0; q < 2; q++ {
		shown := tr.messages("battle.question.shown")
		last := shown[len(shown)-1]
		ids, _ := last.Payload["primary_user_ids"].([]string)
		want := []string{tr.r.MainTurn(0).String(), tr.r.MainTurn(1).String()}
		if !slices.Equal(ids, want) {
			t.Fatalf("question %d primary %v, want %v", q, ids, want)
		}
		snap := tr.r.Snapshot(tr.players[1])
		if got, _ := snap.Payload["primary_user_ids"].([]string); !slices.Equal(got, want) {
			t.Fatalf("question %d snapshot primary %v", q, got)
		}
		noAnswerData(t, last)
		for _, p := range tr.players {
			tr.answer(p, 2*time.Second, true)
		}
		tr.closeAndReveal()
		tr.next()
	}
	shown := tr.messages("battle.question.shown")
	first, _ := shown[0].Payload["primary_user_ids"].([]string)
	second, _ := shown[1].Payload["primary_user_ids"].([]string)
	if slices.Equal(first, second) {
		t.Fatalf("the main turn does not rotate: %v %v", first, second)
	}
	solo := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 5, questions: 1})
	solo.start()
	if _, ok := solo.messages("battle.question.shown")[0].Payload["primary_user_ids"]; ok {
		t.Fatal("a 1v1 has no primary turn")
	}
	if _, ok := solo.r.Snapshot(solo.players[0]).Payload["primary_user_ids"]; ok {
		t.Fatal("a 1v1 snapshot has no primary turn")
	}
}

// chat_channel_ids: the deterministic ids of module chat (F117) — the match channel for everyone, the own team
// channel only in a team battle; never a team channel in a 1v1 or a timed quiz.
func TestBattleDeltaChatChannels(t *testing.T) {
	team := newRoom(t, roomOpts{mode: battle.ModeTeam, grade: 5, perTeam: 2, questions: 1})
	match := team.r.MatchID()
	for i, p := range team.players {
		c, _ := team.r.Snapshot(p).Payload["chat_channel_ids"].(map[string]any)
		wantTeam := chatid.Team(match, i/2).String()
		if c["match"] != chatid.Match(match).String() || c["team"] != wantTeam {
			t.Fatalf("team player %d: %v (team %s)", i, c, wantTeam)
		}
	}
	solo := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 5, questions: 1})
	c, _ := solo.r.Snapshot(solo.players[0]).Payload["chat_channel_ids"].(map[string]any)
	if c["match"] != chatid.Match(solo.r.MatchID()).String() || c["team"] != nil {
		t.Fatalf("1v1 chat channels: %v", c)
	}
	quiz := newRoom(t, roomOpts{mode: battle.ModeTimedQuiz, grade: 4, perTeam: 6, teams: 1, questions: 1})
	if c, _ := quiz.r.Snapshot(quiz.players[0]).Payload["chat_channel_ids"].(map[string]any); c["team"] != nil {
		t.Fatalf("timed quiz chat channels: %v", c)
	}
	if got := battle.ChatChannelsOf(uuid.New(), map[uuid.UUID]int{uuid.New(): 0}); len(got) != 1 {
		t.Fatalf("bot match: %v", got)
	}
	if _, ok := solo.r.Snapshot(uuid.New()).Payload["chat_channel_ids"]; ok {
		t.Fatal("a stranger gets no chat channel")
	}
}

// Option key: the QuestionRender carries the canonical `id` and the `option_id` alias with the same value, whichever
// one the render payload of module learning had.
func TestBattleDeltaOptionAlias(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 5, questions: 1})
	tr.start()
	q := tr.messages("battle.question.shown")[0].Payload["question"].(map[string]any)
	b, _ := json.Marshal(q["options"])
	var opts []map[string]any
	if err := json.Unmarshal(b, &opts); err != nil || len(opts) != 2 {
		t.Fatalf("options %s", b)
	}
	for _, o := range opts {
		if o["id"] == nil || o["id"] != o["option_id"] {
			t.Fatalf("option keys %v", o)
		}
	}
	// a render payload with `id` only (module learning) gets the alias too.
	match := uuid.New()
	rules := mustRules(t, battle.Mode1v1, 5)
	r, err := battle.New(battle.Spec{MatchID: match, PlanID: uuid.New(), Grade: 5, Rules: rules,
		Players: []battle.PlayerSpec{{ID: uuid.New(), Team: 0}, {ID: uuid.New(), Team: 1}},
		Items: []battle.Item{{Index: 0, Instance: battle.InstanceID(match, 0), Type: "single_choice", Limit: rules.TimeLimit,
			Render: json.RawMessage(`{"stem":{},"options":[{"id":"o_x1"},{"id":"o_y2"}]}`)}}}, bt0)
	if err != nil {
		t.Fatal(err)
	}
	r.St, r.Cur = battle.StateQuestion, 0
	cq := r.Snapshot(r.Players[0].ID).Payload["current_question"].(map[string]any)
	for _, o := range cq["options"].([]any) {
		m := o.(map[string]any)
		if m["option_id"] != m["id"] || m["id"] == nil {
			t.Fatalf("alias of a learning render: %v", m)
		}
	}
}

// LossStreak counts lost matches in a row, newest first: a win or a draw breaks it, no_contest is skipped.
func TestBattleDeltaLossStreak(t *testing.T) {
	L, W, D, F, A, N := battle.OutcomeLoss, battle.OutcomeWin, battle.OutcomeDraw, battle.OutcomeForfeit, battle.OutcomeAbandoned, battle.OutcomeNoContest
	for _, c := range []struct {
		in   []string
		n    int
		want bool
	}{
		{[]string{L, L, L}, 3, true},
		{[]string{L, N, F, A}, 3, true},
		{[]string{L, L}, 3, false},
		{[]string{L, W, L, L}, 3, false},
		{[]string{L, L, D}, 3, false},
		{[]string{W, L, L, L}, 3, false},
		{[]string{L, L, L}, 0, false},
		{nil, 3, false},
	} {
		if got := battle.LossStreak(c.in, c.n); got != c.want {
			t.Fatalf("LossStreak(%v, %d) = %v", c.in, c.n, got)
		}
	}
	if battle.DefaultConfig().LossStreak != 3 {
		t.Fatalf("loss_streak_encouragement default %d (CONFIG_DEFAULTS 3)", battle.DefaultConfig().LossStreak)
	}
	cfg, err := battle.ParseConfig([]byte(`{"battle":{"loss_streak_encouragement":4}}`))
	if err != nil || cfg.LossStreak != 4 {
		t.Fatalf("configured loss streak %d %v", cfg.LossStreak, err)
	}
}

// The encouragement is private: the player who ended a loss streak gets its own battle.match.ended with it (a locale key,
// the subject and topic of the match), the other player the room message without it.
func TestBattleDeltaEncouragementPrivate(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 4, questions: 1})
	tr.start()
	a, b := tr.players[0], tr.players[1]
	tr.answer(a, 2*time.Second, true)
	tr.answer(b, 2*time.Second, false)
	tr.closeAndReveal()
	tr.next()
	if tr.r.State() != battle.StateFinalizing || tr.outcome(b) != battle.OutcomeLoss {
		t.Fatalf("state %s outcome %s", tr.r.State(), tr.outcome(b))
	}
	subject, topic := uuid.New(), uuid.New()
	end := tr.r.EndedWith(tr.now, map[uuid.UUID]battle.Encouragement{b: battle.LossStreakEncouragement(subject, &topic)})
	var mine, others *battle.Message
	for i := range end.Messages {
		m := &end.Messages[i]
		if m.Type != "battle.match.ended" {
			continue
		}
		if slices.Equal(m.To, []uuid.UUID{b}) {
			mine = m
		} else {
			others = m
		}
	}
	if mine == nil || others == nil || !slices.Equal(others.To, []uuid.UUID{a}) {
		t.Fatalf("messages %+v", end.Messages)
	}
	e, _ := mine.Payload["encouragement"].(map[string]any)
	if e["code"] != battle.EncouragementLossStreak || e["message_code"] != battle.EncouragementLossStreakMessage ||
		e["subject_id"] != subject.String() || e["topic_id"] != topic.String() {
		t.Fatalf("encouragement %v", e)
	}
	if _, ok := others.Payload["encouragement"]; ok {
		t.Fatal("the other player sees the encouragement")
	}
	if len(e) != 4 {
		t.Fatalf("the encouragement carries more than the code, key, subject and topic: %v", e)
	}
	// without encouragement the room message stays one message to everyone.
	tr2 := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 4, questions: 1})
	tr2.start()
	for _, p := range tr2.players {
		tr2.answer(p, 2*time.Second, true)
	}
	tr2.closeAndReveal()
	tr2.next()
	if end := tr2.r.Ended(tr2.now); len(end.Messages) != 1 || end.Messages[0].To != nil {
		t.Fatalf("plain end: %+v", end.Messages)
	}
}
