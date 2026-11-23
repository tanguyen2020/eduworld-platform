//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

var lbFriendsXPWeekly = uuid.MustParse("01936a00-0000-7000-8000-000000106106")

// xpAwarded delivers an xp.awarded of module game (F074) for k through competition.leaderboard_xp.
func (r *renv) xpAwarded(k kid, entry uuid.UUID, amount int, reason string) {
	r.t.Helper()
	p := map[string]any{"xp_entry_id": entry, "student_id": k.id, "amount": amount, "reason_code": reason, "source_type": "quest",
		"source_code": "quest.main_completed", "source_id": uuid.New(), "capped": amount == 0, "xp_total": 1000, "level": 3,
		"awarded_at": r.clock.Now()}
	b, _ := json.Marshal(p)
	t := k.tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: "xp.awarded", OccurredAt: r.clock.Now(), TenantID: &t, Payload: b}
	r.consume(messaging.LeaderboardXPName, consumer.LeaderboardXP{H: r.ranks.RecordXP}.Handle, env)
}

// F074 T042 (received from F106): xp.awarded credits the xp metric of friends_xp_weekly once per posting; a redelivered
// posting, a 0 XP (capped) posting and a compensating entry add nothing; the board of a student lists the XP of the week
// of the student and its friends.
func TestF074FriendsXPWeekly(t *testing.T) {
	r := setupRank(t)
	a, b, c := r.kidA(), r.kidA(), r.kidA()
	r.friends(a, b)
	e1 := uuid.New()
	r.xpAwarded(a, e1, 150, "reward")
	r.xpAwarded(a, e1, 150, "reward") // the same posting under a new event id
	r.xpAwarded(a, uuid.New(), 40, "reward")
	r.xpAwarded(a, uuid.New(), 0, "reward")       // capped to 0
	r.xpAwarded(a, uuid.New(), 500, "adjustment") // merge / adjustment: not effort of the week
	r.xpAwarded(b, uuid.New(), 20, "reward")
	r.xpAwarded(c, uuid.New(), 999, "reward") // not a friend of a
	if n := r.count(`SELECT count(*) FROM competition.leaderboard_contributions WHERE metric = 'xp' AND student_id = $1`, a.id); n != 2 {
		t.Fatalf("xp contributions of a: %d", n)
	}
	if n := r.count(`SELECT coalesce(sum(amount), 0) FROM competition.leaderboard_contributions WHERE metric = 'xp' AND student_id = $1
		AND source_type = 'xp_awarded'`, a.id); n != 190 {
		t.Fatalf("xp of a: %d", n)
	}
	res := r.board(lbFriendsXPWeekly, a, "")
	es := entriesOf(res)
	if res.status != http.StatusOK || len(es) != 2 || es[0]["value"].(float64) != 190 || es[1]["value"].(float64) != 20 {
		t.Fatalf("friends_xp_weekly of a: %d %s", res.status, res.raw)
	}
}
