package unit

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
)

// F105-AC-04 (T012, TC-CMP-007, SC-003): 100 per right answer, 0 otherwise, the last question ×2; a speed bonus ≤ 50,
// linear in the server time left, only from speed_bonus_min_grade (6); never below it.
func TestBattlePointsByGrade(t *testing.T) {
	cfg := battle.DefaultConfig()
	for grade := 1; grade <= 9; grade++ {
		r, err := cfg.RulesFor(battle.Mode1v1, grade, false)
		if err != nil {
			t.Fatal(err)
		}
		wantLimit := 20 * time.Second
		if grade <= 3 {
			wantLimit = 30 * time.Second
		}
		if r.QuestionCount != 7 || r.TimeLimit != wantLimit || r.SpeedBonus != (grade >= 6) {
			t.Fatalf("grade %d rules: %+v", grade, r)
		}
		elapsed := 2 * time.Second
		p, bonus := battle.Points(r, true, elapsed, false, false, false)
		if grade < 6 && (p != 100 || bonus != 0) {
			t.Fatalf("grade %d: %d points, bonus %d (no speed bonus below grade 6)", grade, p, bonus)
		}
		if grade >= 6 {
			// remaining = 20 s − (2 s − 150 ms RTT) = 18.15 s → 50 × 18.15/20 = 45.4 → 45
			if bonus != 45 || p != 145 {
				t.Fatalf("grade %d: %d points, bonus %d", grade, p, bonus)
			}
		}
		if p, b := battle.Points(r, false, elapsed, false, false, false); p != 0 || b != 0 {
			t.Fatalf("grade %d wrong answer: %d %d", grade, p, b)
		}
		last, _ := battle.Points(r, true, elapsed, true, false, false)
		if last != 2*p {
			t.Fatalf("grade %d last question: %d, want ×2 of %d", grade, last, p)
		}
		if _, b := battle.Points(r, true, r.TimeLimit, false, false, false); b != 0 {
			t.Fatalf("grade %d: bonus %d at the deadline", grade, b)
		}
		if p, b := battle.Points(r, true, 50*time.Millisecond, false, false, true); b != 0 || p != 100 {
			t.Fatalf("grade %d flagged fast answer keeps its points but no bonus: %d %d", grade, p, b)
		}
	}
}

// F105-AC-06: the winner has the higher total, equal totals are a draw; the commands carry no score at all.
func TestBattleWinnerAndDraw(t *testing.T) {
	tr := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 3, questions: 2, ranked: true})
	a, b := tr.players[0], tr.players[1]
	tr.start()
	tr.answer(a, 3*time.Second, true)
	tr.answer(b, 4*time.Second, false)
	tr.closeAndReveal()
	tr.next()
	tr.answer(a, 3*time.Second, false)
	tr.answer(b, 3*time.Second, true) // last question ×2: b leads 200 to 100
	tr.closeAndReveal()
	s := tr.next()
	if !s.Finish || tr.r.State() != battle.StateFinalizing {
		t.Fatalf("the match does not finalize: %s", tr.r.State())
	}
	res := tr.r.Result()
	if res.Status != "completed" || res.EndReason != battle.EndFinished || res.WinnerTeam == nil || *res.WinnerTeam != 1 || !res.Rated {
		t.Fatalf("result: %+v", res)
	}
	if tr.score(a) != 100 || tr.score(b) != 200 || tr.outcome(a) != battle.OutcomeLoss || tr.outcome(b) != battle.OutcomeWin {
		t.Fatalf("scores %d/%d outcomes %s/%s", tr.score(a), tr.score(b), tr.outcome(a), tr.outcome(b))
	}

	d := newRoom(t, roomOpts{mode: battle.Mode1v1, grade: 4, questions: 1, ranked: true})
	d.start()
	d.answer(d.players[0], time.Second, true)
	d.answer(d.players[1], 9*time.Second, true) // grade 4: no speed bonus, same points
	d.closeAndReveal()
	d.next()
	res = d.r.Result()
	if res.WinnerTeam != nil || d.outcome(d.players[0]) != battle.OutcomeDraw || d.outcome(d.players[1]) != battle.OutcomeDraw {
		t.Fatalf("draw: %+v", res)
	}
}
