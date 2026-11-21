package unit

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
)

// F105-AC-01, AC-03, AC-05 (T013, TC-CMP-003, TC-CMP-007): one question.shown for everyone (same shown_at and
// deadline_at, no answer data); the server time decides lateness; one answer per player per question (the first stays);
// an answer to a question not shown, already closed or not in the plan is refused and recorded as an integrity signal;
// a stranger is not a member; the reveal sends each player its own evaluation and everyone the scoreboard.
func TestBattleAnswerRules(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 3, questions: 3, ranked: true})
	a, b := tr.players[0], tr.players[1]
	tr.start()
	shown := tr.messages("battle.question.shown")
	if len(shown) != 1 || shown[0].To != nil {
		t.Fatalf("question.shown must go once to the whole room: %+v", shown)
	}
	noAnswerData(t, shown[0])
	q := shown[0].Payload["question"].(map[string]any)
	if q["question_instance_id"] != tr.r.CurrentInstance() || shown[0].Payload["shown_at"] != tr.r.ShownAt().UnixMilli() ||
		shown[0].Payload["deadline_at"] != tr.r.DeadlineAt().UnixMilli() {
		t.Fatalf("question.shown payload: %+v", shown[0].Payload)
	}
	resp := json.RawMessage(`{"option_id":"o_3b2e"}`)
	cur := tr.r.CurrentInstance()
	// A stranger.
	if _, _, err := tr.r.Submit(uuid.New(), cur, resp, tr.r.ShownAt().Add(time.Second)); !errors.Is(err, battle.ErrNotMember) {
		t.Fatalf("stranger: %v", err)
	}
	// The next question is not shown yet; an instance outside the plan.
	next := battle.InstanceID(tr.r.MatchID(), 1)
	if s, _, err := tr.r.Submit(a, next, resp, tr.r.ShownAt().Add(time.Second)); !errors.Is(err, battle.ErrNotOpen) || len(s.Signals) != 1 ||
		s.Signals[0].Type != battle.SignalBeforeShown {
		t.Fatalf("answer before shown: %v %+v", err, s.Signals)
	}
	if s, _, err := tr.r.Submit(a, "qi_forged", resp, tr.r.ShownAt().Add(time.Second)); !errors.Is(err, battle.ErrNotOpen) || len(s.Signals) != 1 {
		t.Fatalf("instance outside the plan: %v %+v", err, s.Signals)
	}
	// Valid, then a second answer of the same player.
	s, pend, err := tr.r.Submit(a, cur, resp, tr.r.ShownAt().Add(2*time.Second))
	if err != nil || pend.Index != 0 || pend.Student != a || !pend.ReceivedAt.Equal(tr.r.ShownAt().Add(2*time.Second)) {
		t.Fatalf("valid answer: %v %+v", err, pend)
	}
	if len(s.Messages) != 1 || s.Messages[0].Type != "battle.answer.recorded" || s.Messages[0].Payload["user_id"] != a.String() {
		t.Fatalf("answer.recorded: %+v", s.Messages)
	}
	noAnswerData(t, s.Messages[0])
	if _, _, err := tr.r.Submit(a, cur, json.RawMessage(`{"option_id":"o_a91c"}`), tr.r.ShownAt().Add(3*time.Second)); !errors.Is(err, battle.ErrAlreadySubmitted) {
		t.Fatalf("second answer: %v", err)
	}
	tr.record(tr.r.Scored(0, a, true))
	// Late: after deadline_at + 300 ms (server time only).
	late := tr.r.DeadlineAt().Add(301 * time.Millisecond)
	if _, _, err := tr.r.Submit(b, cur, resp, late); !errors.Is(err, battle.ErrLate) {
		t.Fatalf("late answer: %v", err)
	}
	onTime := tr.r.DeadlineAt().Add(299 * time.Millisecond)
	if _, _, err := tr.r.Submit(b, cur, resp, onTime); err != nil {
		t.Fatalf("answer inside the 300 ms grace: %v", err)
	}
	tr.record(tr.r.Scored(0, b, false))
	tr.now = onTime
	tr.closeAndReveal()
	// Reveal: one evaluation per player (to that player only), the scoreboard to everyone, at the same time.
	ev := tr.messages("battle.answer.evaluated")
	if len(ev) != 2 {
		t.Fatalf("%d evaluations", len(ev))
	}
	for _, m := range ev {
		if len(m.To) != 1 || m.Payload["correct_response"] == nil {
			t.Fatalf("evaluation: %+v", m)
		}
		if m.To[0] == a && (m.Payload["is_correct"] != true || m.Payload["points"] != 100) {
			t.Fatalf("evaluation of a: %+v", m.Payload)
		}
		if m.To[0] == b && (m.Payload["is_correct"] != false || m.Payload["points"] != 0) {
			t.Fatalf("evaluation of b: %+v", m.Payload)
		}
	}
	rev := tr.messages("battle.question.revealed")
	if len(rev) != 1 || rev[0].To != nil || len(rev[0].Payload["scoreboard"].([]map[string]any)) != 2 {
		t.Fatalf("question.revealed: %+v", rev)
	}
	// After the close: the closed question is late and signalled.
	tr.next()
	if s, _, err := tr.r.Submit(b, cur, resp, tr.now.Add(time.Second)); !errors.Is(err, battle.ErrLate) || len(s.Signals) != 1 ||
		s.Signals[0].Type != battle.SignalClosedQuestion {
		t.Fatalf("answer to a closed question: %v %+v", err, s.Signals)
	}
	// Before the match runs (lobby) no answer is taken.
	lobby := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 3})
	if _, _, err := lobby.r.Submit(lobby.players[0], battle.InstanceID(lobby.r.MatchID(), 0), resp, bt0); !errors.Is(err, battle.ErrNotActive) {
		t.Fatalf("answer in the lobby: %v", err)
	}
}

// Scoring failures (edge case, REALTIME_ARCHITECTURE §7.1): a question whose answer could not be scored is played again;
// after two replays the match aborts (no result, no rating, no abandon).
func TestBattleScoreFailureReplaysThenAborts(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 3, questions: 2, ranked: true})
	a, b := tr.players[0], tr.players[1]
	tr.start()
	for replay := 1; replay <= 3; replay++ {
		at := tr.r.ShownAt().Add(time.Second)
		_, pend, err := tr.r.Submit(a, tr.r.CurrentInstance(), json.RawMessage(`{"option_id":"o_3b2e"}`), at)
		if err != nil {
			t.Fatalf("replay %d submit: %v", replay, err)
		}
		tr.record(tr.r.ScoreFailed(pend.Index, a))
		tr.answer(b, time.Second, true)
		s := tr.advance(time.Second)
		if replay < 3 {
			if tr.r.State() != battle.StateQuestion || tr.r.Current() != 0 || s.Reveal != nil {
				t.Fatalf("replay %d: state %s question %d", replay, tr.r.State(), tr.r.Current())
			}
			continue
		}
		if s.Abort != battle.EndServerFailure || tr.r.State() != battle.StateAborted {
			t.Fatalf("third failure: abort %q state %s", s.Abort, tr.r.State())
		}
	}
	if tr.r.Result().Rated {
		t.Fatal("an aborted match is rated")
	}
}
