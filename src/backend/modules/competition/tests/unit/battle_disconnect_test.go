package unit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
)

// F105-AC-11, AC-12 (T023, TC-CMP-004, TC-CMP-005): a disconnected 1v1 player keeps the seat for 30 s while the clock
// runs (no answer = 0); back in time it gets the snapshot of the current question; past the grace it abandons and the
// opponent wins (rated); when both left the match is cancelled (not rated).
func TestBattleDisconnect1v1(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 3, questions: 3, ranked: true})
	a, b := tr.players[0], tr.players[1]
	tr.start()
	// b drops 20 s into question 0 (30 s per question in grade 3): the grace runs to 50 s, past the next question show.
	s := tr.record(tr.r.Disconnect(b, tr.now.Add(20*time.Second)))
	if len(s.Messages) != 1 || s.Messages[0].Type != "battle.participant.status_changed" || s.Messages[0].Payload["connected"] != false ||
		s.Messages[0].Payload["grace_until"] != tr.now.Add(50*time.Second).UnixMilli() {
		t.Fatalf("status_changed: %+v", s.Messages)
	}
	// The clock keeps running: a answers, the deadline passes, b scores 0.
	tr.answer(a, 2*time.Second, true)
	tr.closeAndReveal()
	tr.next()
	// b comes back within the grace: snapshot of the current question for b, status_changed to the others.
	s, err := tr.r.Join(b, uuid.New(), tr.now)
	if err != nil {
		t.Fatal(err)
	}
	var snap *battle.Message
	for i, m := range s.Messages {
		if m.Type == "battle.state.snapshot" {
			snap = &s.Messages[i]
		}
	}
	if snap == nil || len(snap.To) != 1 || snap.To[0] != b || snap.Payload["state"] != battle.StateQuestion || snap.Payload["question_index"] != 1 {
		t.Fatalf("snapshot after reconnect: %+v", s.Messages)
	}
	noAnswerData(t, *snap)
	if tr.score(b) != 0 || tr.score(a) != 100 {
		t.Fatalf("scores after the missed question: a %d b %d", tr.score(a), tr.score(b))
	}

	// Past the grace: b abandons, a wins, rated.
	tr.record(tr.r.Disconnect(b, tr.now))
	s = tr.advance(31 * time.Second)
	if !s.Finish || tr.r.State() != battle.StateFinalizing {
		t.Fatalf("match does not end after the grace: %s", tr.r.State())
	}
	res := tr.r.Result()
	if res.EndReason != battle.EndAbandon || !res.Rated || tr.outcome(b) != battle.OutcomeAbandoned || tr.outcome(a) != battle.OutcomeWin {
		t.Fatalf("abandon: %+v", res)
	}

	// Both gone: cancelled, not rated.
	both := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 3, ranked: true})
	both.start()
	both.record(both.r.Disconnect(both.players[0], both.now))
	both.record(both.r.Disconnect(both.players[1], both.now.Add(time.Second)))
	s = both.advance(31 * time.Second)
	res = both.r.Result()
	if !s.Finish || res.Status != "cancelled" || res.EndReason != battle.EndAllLeft || res.Rated {
		t.Fatalf("both left: %+v", res)
	}
	for _, p := range both.players {
		if both.outcome(p) != battle.OutcomeNoContest {
			t.Fatalf("outcome %s after both left", both.outcome(p))
		}
	}
}

// F105-AC-13: a team battle goes on without the member past its 45 s grace (abandoned); a timed quiz holds the seat to
// the end and keeps the answers already given.
func TestBattleDisconnectTeamAndTimedQuiz(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.ModeTeam, grade: 6, perTeam: 3, questions: 2})
	tr.start()
	gone := tr.players[1]
	tr.record(tr.r.Disconnect(gone, tr.now))
	tr.advance(46 * time.Second)
	if tr.r.State() == battle.StateFinalizing || tr.r.State() == battle.StateEnded {
		t.Fatal("team battle ended when one member left")
	}
	for tr.r.State() != battle.StateFinalizing {
		if tr.r.State() == battle.StateQuestion || tr.r.State() == battle.StateClosing {
			tr.closeAndReveal()
		}
		tr.next()
	}
	if tr.outcome(gone) != battle.OutcomeAbandoned {
		t.Fatalf("member past the grace: %s", tr.outcome(gone))
	}
	if tr.r.Result().EndReason != battle.EndFinished {
		t.Fatalf("team battle end reason %s", tr.r.Result().EndReason)
	}

	q := newRoom(t, roomOpts{mode: battle.ModeTimedQuiz, grade: 4, perTeam: 3, teams: 1, questions: 2})
	q.start()
	away := q.players[2]
	q.answer(away, 2*time.Second, true)
	q.record(q.r.Disconnect(away, q.now.Add(3*time.Second)))
	q.advance(10 * time.Minute) // far past any grace
	for q.r.State() != battle.StateFinalizing {
		if q.r.State() == battle.StateQuestion || q.r.State() == battle.StateClosing {
			q.closeAndReveal()
		}
		q.next()
	}
	if q.outcome(away) == battle.OutcomeAbandoned || q.score(away) != 100 {
		t.Fatalf("timed quiz seat: outcome %s score %d", q.outcome(away), q.score(away))
	}
}

// F105-AC-15 (T025, domain part): the room comes back from its checkpoint on another host and plays the current
// question again (replay_current_question); answers of that question are dropped, earlier scores kept.
func TestBattleCheckpointResume(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 3, questions: 3, ranked: true})
	a, b := tr.players[0], tr.players[1]
	tr.start()
	tr.answer(a, time.Second, true)
	tr.answer(b, time.Second, true)
	tr.closeAndReveal()
	tr.next()
	tr.answer(a, time.Second, true) // in flight when the host dies
	ckpt, err := json.Marshal(tr.r)
	if err != nil {
		t.Fatal(err)
	}
	if string(ckpt) == "" || json.Valid(ckpt) == false {
		t.Fatal("checkpoint")
	}
	back, err := battle.Restore(ckpt)
	if err != nil {
		t.Fatal(err)
	}
	later := tr.now.Add(12 * time.Second)
	s := back.Resume(later)
	if back.State() != battle.StateQuestion || back.Current() != 1 || !back.ShownAt().Equal(later) {
		t.Fatalf("resumed state %s question %d shown %v", back.State(), back.Current(), back.ShownAt())
	}
	var resumed bool
	for _, m := range s.Messages {
		if m.Type == "battle.match.resumed" && m.Payload["replay_current_question"] == true {
			resumed = true
		}
	}
	if !resumed {
		t.Fatalf("no battle.match.resumed: %+v", s.Messages)
	}
	if _, _, err := back.Submit(a, back.CurrentInstance(), json.RawMessage(`{"option_id":"o_3b2e"}`), later.Add(time.Second)); err != nil {
		t.Fatalf("the replayed question takes a new answer: %v", err)
	}
	for _, p := range back.Result().Players {
		if p.Score != 100 {
			t.Fatalf("score before the failover lost: %+v", p)
		}
	}
}
