//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/battlehost"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// presenceGone publishes the presence signal of the gateway when the connection of k ended.
func (b *benv) presenceGone(k kid) {
	b.t.Helper()
	data, _ := json.Marshal(rt.PresenceChanged{V: 1, UserID: k.id, TenantID: k.tenant, Status: rt.PresenceOffline, Reason: rt.PresenceDisconnected,
		Ts: time.Now().UnixMilli()})
	if err := b.nc.Publish(rt.PresenceSubject(k.tenant), data); err != nil {
		b.t.Fatal(err)
	}
	_ = b.nc.Flush()
}

// start opens a 1v1 match, joins both and waits for the first question.
func (b *benv) start(h *battlehost.Host, o matchOpts, sessions ...uuid.UUID) (uuid.UUID, string) {
	b.t.Helper()
	match := b.match(o)
	if err := h.Open(context.Background(), match, o.teams[0][0].tenant); err != nil {
		b.t.Fatal(err)
	}
	i := 0
	for _, team := range o.teams {
		for _, k := range team {
			if r := b.state(h, match, k, sessions[i]); !r.OK {
				b.t.Fatalf("state of %s: %+v", k.nickname, r)
			}
			i++
		}
	}
	first := o.teams[0][0]
	waitFor(b.t, 10*time.Second, "countdown", func() bool {
		for _, m := range b.out.of(first.id, "battle.countdown.started") {
			if m["match_id"] == match.String() {
				return true
			}
		}
		return false
	})
	b.clock.Add(3 * time.Second)
	return match, b.shownIn(match, first, 0)
}

// shownIn waits for question index of match shown to k.
func (b *benv) shownIn(match uuid.UUID, k kid, index int) string {
	b.t.Helper()
	var inst string
	waitFor(b.t, 10*time.Second, "question shown", func() bool {
		for _, m := range b.out.of(k.id, "battle.question.shown") {
			if m["match_id"] == match.String() && int(m["question_index"].(float64)) == index {
				inst = m["question"].(map[string]any)["question_instance_id"].(string)
			}
		}
		return inst != ""
	})
	return inst
}

// T024 — F105-AC-11, AC-14 (TC-CMP-004, TC-CMP-005): a dropped connection (presence of the gateway) keeps the seat
// with status_changed and grace_until to the opponent; the same student back on a new device (takeover, new game
// session) gets the snapshot of the current question, stays a member and its old membership closes as
// replaced_by_other_device.
func TestBattleReconnectTakeover(t *testing.T) {
	b := setupBattle(t)
	h, _ := b.host("core-1")
	an, binh := b.kid(b.tenantA, b.schoolA, 3), b.kid(b.tenantA, b.schoolA, 3)
	sa, sb := uuid.New(), uuid.New()
	match, _ := b.start(h, matchOpts{mode: battle.Mode1v1, grade: 3, ranked: true, teams: [][]kid{{an}, {binh}}}, sa, sb)
	b.presenceGone(binh)
	waitFor(t, 10*time.Second, "status_changed", func() bool {
		for _, m := range b.out.of(an.id, "battle.participant.status_changed") {
			if m["user_id"] == binh.id.String() && m["connected"] == false && m["grace_until"] != nil {
				return true
			}
		}
		return false
	})
	b.clock.Add(10 * time.Second)
	newDevice := uuid.New()
	r := b.state(h, match, binh, newDevice)
	if !r.OK {
		t.Fatalf("reconnect: %+v", r)
	}
	var snap struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(r.Result, &snap); err != nil || snap.Type != "battle.state.snapshot" || snap.Payload["state"] != battle.StateQuestion ||
		snap.Payload["current_question"] == nil {
		t.Fatalf("snapshot: %s", r.Result)
	}
	noAnswerKeys(t, "battle.state.snapshot", snap.Payload)
	waitFor(t, 10*time.Second, "memberships", func() bool {
		return b.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND student_id = $2 AND game_session_id = $3
			AND leave_reason = 'replaced_by_other_device'`, match, binh.id, sb) == 1 &&
			b.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND student_id = $2 AND game_session_id = $3
			AND left_at IS NULL`, match, binh.id, newDevice) == 1
	})
	waitFor(t, 10*time.Second, "disconnection counted on the old membership", func() bool {
		return b.count(`SELECT disconnects FROM multiplayer.session_memberships WHERE room_id = $1 AND student_id = $2 AND game_session_id = $3`,
			match, binh.id, sb) == 1
	})
}

// T025 — F105-AC-15 (AC-MP-006): the owner dies; another replica restores the room from its checkpoint and plays the
// current question again; without a checkpoint the match aborts with no result, no rating, no abandon, and the
// players are told.
func TestBattleFailover(t *testing.T) {
	b := setupBattle(t)
	h1, _ := b.host("core-1")
	an, binh := b.kid(b.tenantA, b.schoolA, 3), b.kid(b.tenantA, b.schoolA, 3)
	match, _ := b.start(h1, matchOpts{mode: battle.Mode1v1, grade: 3, ranked: true, teams: [][]kid{{an}, {binh}}}, uuid.New(), uuid.New())
	h1.Crash()
	b.clock.Add(2 * time.Second) // the lease of core-1 is not renewed any more
	h2, _ := b.host("core-2")
	waitFor(t, 15*time.Second, "takeover", func() bool { return len(h2.Hosted()) == 1 })
	waitFor(t, 10*time.Second, "resumed", func() bool {
		for _, m := range b.out.of(an.id, "battle.match.resumed") {
			if m["replay_current_question"] == true {
				return true
			}
		}
		return false
	})
	shown := 0
	for _, m := range b.out.of(an.id, "battle.question.shown") {
		if int(m["question_index"].(float64)) == 0 {
			shown++
		}
	}
	if shown != 2 {
		t.Fatalf("question 0 shown %d times, want a replay", shown)
	}
	// The players answer on the new owner, reached from another replica through NATS.
	if r := b.answer(h1, match, an, uuid.New(), b.shownIn(match, an, 0), rightOption); !r.OK {
		t.Fatalf("answer after the failover: %+v", r)
	}
	// No checkpoint: the next owner aborts.
	c, cd := b.kid(b.tenantA, b.schoolA, 3), b.kid(b.tenantA, b.schoolA, 3)
	m2, _ := b.start(h2, matchOpts{mode: battle.Mode1v1, grade: 3, ranked: true, teams: [][]kid{{c}, {cd}}}, uuid.New(), uuid.New())
	h2.Crash()
	b.clock.Add(2 * time.Second)
	if err := b.vk.Do(context.Background(), b.vk.B().Del().Key("ew:competition:rt:room:"+m2.String()+":ckpt").Build()).Error(); err != nil {
		t.Fatal(err)
	}
	h3, _ := b.host("core-3")
	_ = h3
	waitFor(t, 15*time.Second, "abort", func() bool { return b.status(m2) == "aborted" })
	if b.count(`SELECT count(*) FROM competition.match_results WHERE match_id = $1`, m2) != 0 ||
		b.count(`SELECT count(*) FROM competition.matches WHERE match_id = $1 AND end_reason = 'server_failure'`, m2) != 1 ||
		b.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1 AND outcome IS NOT NULL`, m2) != 0 {
		t.Fatal("aborted match has a result or outcomes")
	}
	if len(b.eventsOf("match.aborted", m2)) != 1 || len(b.audits(command.AuditMatchAborted)) != 1 {
		t.Fatal("match.aborted event and audit")
	}
	waitFor(t, 10*time.Second, "players told", func() bool {
		for _, m := range b.out.all() {
			if m.typ == "battle.match.ended" && m.room == m2 && m.payload["status"] == "aborted" {
				return true
			}
		}
		return false
	})
	if n := b.checkSchemasOf([]string{"match.aborted"}); n["match.aborted"] != 1 {
		t.Fatalf("match.aborted schema: %v", n)
	}
}

// T026 — F105-AC-16, AC-12: a 1v1 player past the 30 s grace abandons (the opponent wins, rated); the third abandoned
// ranked match within 24 h restricts ranked play for 15 min with an audit; an aborted match never counts.
func TestBattleAbandonPenalty(t *testing.T) {
	b := setupBattle(t)
	h, _ := b.host("core-1")
	quitter := b.kid(b.tenantA, b.schoolA, 3)
	// An aborted match (plan failure) does not count.
	b.plans.mu.Lock()
	b.plans.fail = errors.Join(port.ErrPlanRefused, errors.New("pool"))
	b.plans.mu.Unlock()
	other := b.kid(b.tenantA, b.schoolA, 3)
	aborted := b.match(matchOpts{mode: battle.Mode1v1, grade: 3, ranked: true, teams: [][]kid{{other}, {quitter}}})
	if err := h.Open(context.Background(), aborted, b.tenantA); err != nil {
		t.Fatal(err)
	}
	if b.status(aborted) != "aborted" || b.count(`SELECT count(*) FROM competition.matches WHERE match_id = $1 AND end_reason = 'plan_failure'`,
		aborted) != 1 || len(b.eventsOf("match.aborted", aborted)) != 1 {
		t.Fatal("plan failure must abort before the countdown")
	}
	b.plans.mu.Lock()
	b.plans.fail = nil
	b.plans.mu.Unlock()
	for i := 1; i <= 3; i++ {
		opp := b.kid(b.tenantA, b.schoolA, 3)
		match, _ := b.start(h, matchOpts{mode: battle.Mode1v1, grade: 3, ranked: true, teams: [][]kid{{opp}, {quitter}}}, uuid.New(), uuid.New())
		b.presenceGone(quitter)
		waitFor(t, 10*time.Second, "disconnect", func() bool {
			return len(b.out.of(opp.id, "battle.participant.status_changed")) > 0 && func() bool {
				for _, m := range b.out.of(opp.id, "battle.participant.status_changed") {
					if m["match_id"] == match.String() {
						return true
					}
				}
				return false
			}()
		})
		b.clock.Add(31 * time.Second)
		waitFor(t, 15*time.Second, "abandon recorded", func() bool { return b.status(match) == "completed" })
		if b.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1 AND student_id = $2 AND outcome = 'abandoned'`,
			match, quitter.id) != 1 || b.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1 AND student_id = $2
			AND outcome = 'win'`, match, opp.id) != 1 || b.count(`SELECT count(*) FROM competition.match_results WHERE match_id = $1 AND rated
			AND end_reason = 'abandon'`, match) != 1 {
			t.Fatalf("match %d: abandon result", i)
		}
		restricted := b.count(`SELECT count(*) FROM competition.competition_restrictions WHERE student_id = $1 AND kind = 'abandon_penalty'
			AND ends_at - starts_at = interval '15 minutes'`, quitter.id)
		if (i < 3 && restricted != 0) || (i == 3 && restricted != 1) {
			t.Fatalf("after %d abandons: %d restrictions", i, restricted)
		}
	}
	if len(b.audits(command.AuditRestrictionCreate)) != 1 {
		t.Fatal("audit competition_restriction.created")
	}
}
