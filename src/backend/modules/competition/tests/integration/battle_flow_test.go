//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T014, T019, T021, T035 — F105-AC-01, AC-03, AC-05, AC-07, AC-08, AC-10, AC-22, AC-26 (TC-CMP-001, TC-CMP-003,
// TC-CMP-006, TC-CMP-009, TC-TEN-012): a 1v1 grade 3 match opened from match.created pins the plan before the countdown;
// both players get the same question at the same server time without answer data; the reveal goes to both; the result
// is durable once with match.completed and its audit before battle.match.ended; rooms and memberships are recorded; a
// stranger is ROOM_NOT_MEMBER, a ticket of tenant B PERMISSION_DENIED; the result survives a Valkey flush.
func TestBattleFlow1v1(t *testing.T) {
	b := setupBattle(t)
	h, _ := b.host("core-1")
	ctx := context.Background()
	an, binh := b.kid(b.tenantA, b.schoolA, 3), b.kid(b.tenantA, b.schoolA, 3)
	chi, dung := b.kid(b.tenantA, b.schoolA, 3), b.kid(b.tenantB, b.schoolB, 3)
	match := b.match(matchOpts{mode: battle.Mode1v1, grade: 3, ranked: true, teams: [][]kid{{an}, {binh}}})
	if err := h.Open(ctx, match, b.tenantA); err != nil {
		t.Fatal(err)
	}
	if len(b.plans.creates) != 1 || b.plans.creates[0].Grade != 3 || b.plans.creates[0].Count != 7 || b.plans.creates[0].TimeS != 30 ||
		len(b.plans.creates[0].Participants) != 2 || b.plans.creates[0].SubjectID != b.subject {
		t.Fatalf("createQuestionPlan: %+v", b.plans.creates)
	}
	if b.count(`SELECT count(*) FROM competition.matches WHERE match_id = $1 AND question_plan_id IS NOT NULL AND question_plan_version IS NOT NULL
		AND room_id = match_id AND jsonb_array_length(question_plan->'items') = 7 AND status = 'lobby'`, match) != 1 {
		t.Fatal("plan not pinned before the countdown")
	}
	if b.count(`SELECT count(*) FROM multiplayer.rooms WHERE room_id = $1 AND room_type = 'battle' AND match_id = $1 AND status = 'open'`, match) != 1 {
		t.Fatal("room record")
	}
	// A second open (consumer redelivery) changes nothing.
	if err := h.Open(ctx, match, b.tenantA); err != nil || len(b.plans.creates) != 1 {
		t.Fatalf("second open: %v (%d plans)", err, len(b.plans.creates))
	}
	// Stranger of the tenant, ticket of tenant B.
	if r := b.state(h, match, chi, uuid.New()); r.OK || r.Problem.Code != "ROOM_NOT_MEMBER" {
		t.Fatalf("stranger: %+v", r)
	}
	if r := b.state(h, match, dung, uuid.New()); r.OK || r.Problem.Code != "PERMISSION_DENIED" {
		t.Fatalf("tenant B: %+v", r)
	}
	sa, sb := uuid.New(), uuid.New()
	for _, p := range []struct {
		k kid
		s uuid.UUID
	}{{an, sa}, {binh, sb}} {
		r := b.state(h, match, p.k, p.s)
		if !r.OK || len(r.Subscribe) != 1 || r.Subscribe[0] != rt.RoomOutChannel(match.String()) {
			t.Fatalf("state request: %+v", r)
		}
	}
	waitFor(t, 10*time.Second, "countdown", func() bool { return len(b.out.of(an.id, "battle.countdown.started")) == 1 })
	b.clock.Add(3 * time.Second)
	right := map[uuid.UUID]int{}
	for i := 0; i < 7; i++ {
		inst := b.shown(an, i)
		if inst != b.shown(binh, i) {
			t.Fatal("players got different questions")
		}
		for _, k := range []kid{an, binh} {
			for _, m := range b.out.of(k.id, "battle.question.shown") {
				noAnswerKeys(t, "battle.question.shown", m)
			}
		}
		if r := b.answer(h, match, an, sa, inst, rightOption); !r.OK {
			t.Fatalf("answer of an: %+v", r)
		}
		if r := b.answer(h, match, binh, sb, inst, "o_wrong"); !r.OK {
			t.Fatalf("answer of binh: %+v", r)
		}
		if i == 0 {
			if r := b.answer(h, match, binh, sb, inst, rightOption); r.OK || r.Problem.Code != "ANSWER_ALREADY_SUBMITTED" {
				t.Fatalf("second answer: %+v", r)
			}
		}
		right[an.id]++
		waitFor(t, 10*time.Second, "reveal", func() bool {
			return len(b.out.of(an.id, "battle.answer.evaluated")) == i+1 && len(b.out.of(binh.id, "battle.answer.evaluated")) == i+1
		})
		b.clock.Add(3 * time.Second)
	}
	ev := b.out.of(an.id, "battle.answer.evaluated")
	if ev[0]["correct_response"] == nil || ev[0]["is_correct"] != true || ev[0]["points"].(float64) != 100 {
		t.Fatalf("evaluation: %+v", ev[0])
	}
	if rev := b.out.of(binh.id, "battle.question.revealed"); len(rev) != 7 || len(rev[0]["scoreboard"].([]any)) != 2 {
		t.Fatalf("question.revealed: %d", len(rev))
	}
	waitFor(t, 10*time.Second, "match ended", func() bool { return len(b.out.of(an.id, "battle.match.ended")) == 1 })
	end := b.out.of(binh.id, "battle.match.ended")[0]
	if end["status"] != "completed" || end["rated"] != true || end["end_reason"] != battle.EndFinished {
		t.Fatalf("match.ended: %+v", end)
	}
	if b.status(match) != "completed" || b.count(`SELECT count(*) FROM competition.match_results WHERE match_id = $1 AND rated
		AND status = 'completed' AND winner_team = 0`, match) != 1 {
		t.Fatal("match result")
	}
	if b.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1 AND student_id = $2 AND outcome = 'win' AND score = 800
		AND correct_count = 7`, match, an.id) != 1 || b.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1
		AND student_id = $2 AND outcome = 'loss' AND score = 0`, match, binh.id) != 1 {
		t.Fatal("participant outcomes")
	}
	if len(b.eventsOf("match.started", match)) != 1 || len(b.eventsOf("match.completed", match)) != 1 {
		t.Fatalf("events: started %d completed %d", len(b.eventsOf("match.started", match)), len(b.eventsOf("match.completed", match)))
	}
	started := b.eventsOf("match.started", match)[0]
	if ps := started["participants"].([]any); len(ps) != 2 {
		t.Fatalf("match.started participants: %+v", started)
	}
	if n := b.checkSchemasOf([]string{"match.started", "match.completed"}); n["match.started"] != 1 || n["match.completed"] != 1 {
		t.Fatalf("event schemas: %v", n)
	}
	if len(b.audits(command.AuditResultRecorded)) != 1 {
		t.Fatal("audit match_result.recorded")
	}
	if b.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND match_id = $1 AND room_type = 'battle'
		AND leave_reason = 'room_closed' AND game_session_id IS NOT NULL`, match) != 2 ||
		b.count(`SELECT count(*) FROM multiplayer.rooms WHERE room_id = $1 AND status = 'closed'`, match) != 1 {
		t.Fatal("memberships and room closed")
	}
	// The scores of the result never came from a client (the commands carry none) and every answer was graded once.
	if len(b.plans.scores) != 14 {
		t.Fatalf("%d graded answers", len(b.plans.scores))
	}
	// AC-10: the result stays after the loss of Valkey.
	if err := b.vk.Do(ctx, b.vk.B().Flushall().Build()).Error(); err != nil {
		t.Fatal(err)
	}
	r := b.call(an, http.MethodGet, "/competition/matches/"+match.String(), "", nil)
	body := r.body
	if r.status != http.StatusOK || body["status"] != "completed" || body["rated"] != true {
		t.Fatalf("getMatch after FLUSHALL: %d %s", r.status, r.raw)
	}
	if strings.Contains(r.raw, "rating") {
		t.Fatalf("a rating in getMatch: %s", r.raw)
	}
	review, _ := body["review"].([]any)
	if len(review) != 7 || review[0].(map[string]any)["my_answer_correct"] != true || review[0].(map[string]any)["correct_response"] == nil {
		t.Fatalf("review: %s", r.raw)
	}
}

// T019 — F105-AC-07, AC-08 (TC-CMP-009, SC-002): RecordMatchResult twice, and from two nodes at once, writes one result,
// one match.completed and one audit; the second is MATCH_RESULT_ALREADY_RECORDED. No REST operation writes a result.
func TestBattleRecordOnce(t *testing.T) {
	b := setupBattle(t)
	an, binh := b.kid(b.tenantA, b.schoolA, 3), b.kid(b.tenantA, b.schoolA, 3)
	match := b.match(matchOpts{mode: battle.Mode1v1, grade: 3, ranked: true, teams: [][]kid{{an}, {binh}}})
	ctx := context.Background()
	info, room, err := b.battles.Open(ctx, match, b.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	room.EndReason, room.EndedAt = battle.EndFinished, b.clock.Now()
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = b.battles.Finish(ctx, info, room)
		}()
	}
	wg.Wait()
	ok, dup := 0, 0
	for _, err := range errs {
		var pe *problem.Error
		switch {
		case err == nil:
			ok++
		case errors.As(err, &pe) && pe.Code == problem.MatchResultAlreadyRecorded:
			dup++
		default:
			t.Fatalf("finish: %v", err)
		}
	}
	if ok != 1 || dup != 3 {
		t.Fatalf("%d recorded, %d duplicates", ok, dup)
	}
	if b.count(`SELECT count(*) FROM competition.match_results WHERE match_id = $1`, match) != 1 || len(b.eventsOf("match.completed", match)) != 1 ||
		len(b.audits(command.AuditResultRecorded)) != 1 {
		t.Fatal("one result, one event, one audit")
	}
	// No REST operation of the contract writes a match result (F105-AC-08).
	for _, path := range []string{"/competition/matches/" + match.String(), "/competition/matches/" + match.String() + "/result"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
			if r := b.call(an, method, path, key(), map[string]any{"score": 999}); r.status < 400 {
				t.Fatalf("%s %s accepted a result: %d", method, path, r.status)
			}
		}
	}
}
