//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// staffAt is a token of school staff of tenant limited to school (school_admin holds lms.class.read).
func (b *benv) staffAt(tenant, school uuid.UUID, client string) string {
	sub := "kc-staff-" + uuid.NewString()
	b.res.Set(sub, authztest.Staff(tenant, "school_admin", authztest.AtSchool(school)).PrincipalData)
	return b.idp.Token(sub, client, time.Now())
}

// finished plays a 1v1 match where a answers everything right and b everything wrong (results recorded).
func (b *benv) finished(a, c kid) uuid.UUID {
	b.t.Helper()
	h, stop := b.host("core-" + uuid.NewString()[:6])
	defer stop()
	sa, sc := uuid.New(), uuid.New()
	match, _ := b.start(h, matchOpts{mode: battle.Mode1v1, grade: 3, ranked: true, teams: [][]kid{{a}, {c}}}, sa, sc)
	for i := 0; i < 7; i++ {
		inst := b.shownIn(match, a, i)
		b.answer(h, match, a, sa, inst, rightOption)
		b.answer(h, match, c, sc, inst, "o_wrong")
		waitFor(b.t, 10*time.Second, "reveal", func() bool {
			n := 0
			for _, m := range b.out.of(a.id, "battle.answer.evaluated") {
				if m["match_id"] == match.String() {
					n++
				}
			}
			return n == i+1
		})
		b.clock.Add(3 * time.Second)
	}
	waitFor(b.t, 10*time.Second, "result", func() bool { return b.status(match) == "completed" })
	return match
}

// T036 — F105-AC-23, AC-25 (TC-TEN-001, TC-AUTH-014, TC-TEN-011): getMatch for a participant (review after the match,
// opponent as nickname only, no rating), another student of the tenant → 404, staff with lms.class.read of the school of
// a participant → 200, of another school → 404, staff with a game token → 403 GAME_ACCESS_STUDENT_ONLY, X-Tenant-Id of
// another tenant → 403 TENANT_MISMATCH; listMyMatches lists the student's own matches only.
func TestBattleMatchAuthzTenant(t *testing.T) {
	b := setupBattle(t)
	an, binh, chi := b.kid(b.tenantA, b.schoolA, 3), b.kid(b.tenantA, b.schoolA, 3), b.kid(b.tenantA, b.schoolA, 3)
	match := b.finished(an, binh)
	path := "/competition/matches/" + match.String()
	r := b.call(an, http.MethodGet, path, "", nil)
	if r.status != http.StatusOK || len(r.body["participants"].([]any)) != 2 || r.body["review"] == nil {
		t.Fatalf("participant: %d %s", r.status, r.raw)
	}
	for _, p := range r.body["participants"].([]any) {
		player := p.(map[string]any)["player"].(map[string]any)
		if player["nickname"] == "" || player["user_id"] == nil {
			t.Fatalf("player card: %+v", player)
		}
	}
	if strings.Contains(r.raw, "rating") || strings.Contains(r.raw, "school_id") || strings.Contains(r.raw, "tenant_id") {
		t.Fatalf("getMatch leaks rating or school: %s", r.raw)
	}
	if r := b.call(chi, http.MethodGet, path, "", nil); r.status != http.StatusNotFound {
		t.Fatalf("other student: %d %s", r.status, r.raw)
	}
	h := map[string]string{"X-Tenant-Id": b.tenantA.String()}
	if r := b.do(http.MethodGet, path, b.staffAt(b.tenantA, b.schoolA, httpxtest.PortalClient), h, nil, ""); r.status != http.StatusOK ||
		r.body["review"] != nil && len(r.body["review"].([]any)) != 7 {
		t.Fatalf("staff of the school: %d %s", r.status, r.raw)
	}
	if r := b.do(http.MethodGet, path, b.staffAt(b.tenantA, b.schoolA2, httpxtest.PortalClient), h, nil, ""); r.status != http.StatusNotFound {
		t.Fatalf("staff of another school: %d %s", r.status, r.raw)
	}
	if r := b.do(http.MethodGet, path, b.staffAt(b.tenantA, b.schoolA, httpxtest.GameClient), h, nil, ""); r.status != http.StatusForbidden ||
		r.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff with a game token: %d %s", r.status, r.raw)
	}
	if r := b.do(http.MethodGet, path, b.staffToken(b.tenantB, "school_admin"), map[string]string{"X-Tenant-Id": b.tenantB.String()}, nil, ""); r.status != http.StatusNotFound {
		t.Fatalf("staff of tenant B: %d %s", r.status, r.raw)
	}
	if r := b.do(http.MethodGet, path, an.token, map[string]string{"X-Tenant-Id": b.tenantB.String()}, nil, ""); r.status != http.StatusForbidden ||
		r.code() != "TENANT_MISMATCH" {
		t.Fatalf("X-Tenant-Id of tenant B: %d %s", r.status, r.raw)
	}
	l := b.call(an, http.MethodGet, "/competition/matches?limit=5", "", nil)
	if l.status != http.StatusOK || len(l.data()) != 1 || l.data()[0]["match_id"] != match.String() {
		t.Fatalf("listMyMatches: %d %s", l.status, l.raw)
	}
	if l := b.call(chi, http.MethodGet, "/competition/matches", "", nil); l.status != http.StatusOK || len(l.data()) != 0 {
		t.Fatalf("listMyMatches of a student without match: %s", l.raw)
	}
	if r := b.do(http.MethodGet, "/competition/matches", b.staffAt(b.tenantA, b.schoolA, httpxtest.PortalClient), h, nil, ""); r.status != http.StatusForbidden {
		t.Fatalf("listMyMatches by staff: %d", r.status)
	}
	authztest.Cover(t, "getMatch", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listMyMatches", authztest.CrossTenant, authztest.RoleDenied)
	_ = authz.PermLmsClassRead
}

// T038 — F105-AC-26, AC-27 (TC-TEN-024, R5): an answer with a score field, an instance id over maxLength, a response of
// another question type or unknown fields is VALIDATION_FAILED and leaves the room unchanged; match events carry the
// participants and teams for the chat channels, and no setting can turn match chat off.
func TestBattleCommandValidation(t *testing.T) {
	b := setupBattle(t)
	h, _ := b.host("core-1")
	an, binh := b.kid(b.tenantA, b.schoolA, 6), b.kid(b.tenantA, b.schoolA, 6)
	sa, sb := uuid.New(), uuid.New()
	match, inst := b.start(h, matchOpts{mode: battle.Mode1v1, grade: 6, ranked: true, teams: [][]kid{{an}, {binh}}}, sa, sb)
	handler := natscmd.BattleAnswer(h, nil)
	for name, payload := range map[string]string{
		"score field":     fmt.Sprintf(`{"match_id":%q,"question_instance_id":%q,"response":{"option_id":"o_right"},"score":999}`, match, inst),
		"time field":      fmt.Sprintf(`{"match_id":%q,"question_instance_id":%q,"response":{"option_id":"o_right"},"received_at":1}`, match, inst),
		"long instance":   fmt.Sprintf(`{"match_id":%q,"question_instance_id":%q,"response":{"option_id":"o_right"}}`, match, strings.Repeat("q", 65)),
		"other type":      fmt.Sprintf(`{"match_id":%q,"question_instance_id":%q,"response":{"value":12}}`, match, inst),
		"extra in answer": fmt.Sprintf(`{"match_id":%q,"question_instance_id":%q,"response":{"option_id":"o_right","points":100}}`, match, inst),
		"not json":        `{"match_id":`,
	} {
		r := handler(context.Background(), caller(an, sa), json.RawMessage(payload))
		if r.OK || r.Problem.Code != "VALIDATION_FAILED" {
			t.Fatalf("%s: %+v", name, r)
		}
	}
	for _, m := range b.out.all() {
		if m.typ == "battle.answer.recorded" {
			t.Fatal("a refused answer changed the room")
		}
	}
	if r := b.answer(h, match, an, sa, inst, rightOption); !r.OK {
		t.Fatalf("the valid answer after the refused ones: %+v", r)
	}
	// Answer to a question not shown yet → refused and an integrity signal.
	next := battle.InstanceID(match, 1)
	if r := b.answer(h, match, binh, sb, next, rightOption); r.OK || r.Problem.Code != "VALIDATION_FAILED" {
		t.Fatalf("answer before shown: %+v", r)
	}
	waitFor(t, 10*time.Second, "integrity signal", func() bool {
		return b.count(`SELECT count(*) FROM competition.integrity_signals WHERE student_id = $1 AND signal_type = 'answer_before_shown'
			AND match_id = $2`, binh.id, match) == 1
	})
	ev := b.eventsOf("match.started", match)
	waitFor(t, 10*time.Second, "match.started", func() bool { ev = b.eventsOf("match.started", match); return len(ev) == 1 })
	teams := map[float64]bool{}
	for _, p := range ev[0]["participants"].([]any) {
		teams[p.(map[string]any)["team_no"].(float64)] = true
	}
	if len(teams) != 2 {
		t.Fatalf("match.started teams: %+v", ev[0])
	}
	// No flag, config key or parameter of the battle engine turns match chat off (ADR-0020 §6, R15).
	for _, k := range []string{"chat_enabled", "disable_chat", "match_chat"} {
		if b.count(`SELECT count(*) FROM competition.game_config_versions WHERE payload::text LIKE '%' || $1 || '%'`, k) != 0 {
			t.Fatalf("configuration key %s", k)
		}
	}
}

// T030, T031, T032 — F105-AC-18, AC-19, AC-20: a co-op challenge runs a coop room (tasks, own answers, collective
// progress, no winner, never rated); a practice bot match labels the Robot, is not rated and still grades the student's
// answers (the timed quiz top 5 / grade goal rules are unit tests: TestBattleTimedQuiz).
func TestBattleModes(t *testing.T) {
	b := setupBattle(t)
	h, _ := b.host("core-1")
	ctx := context.Background()
	// Co-op challenge (party of grade 2).
	a, c, d := b.kid(b.tenantA, b.schoolA, 2), b.kid(b.tenantA, b.schoolA, 2), b.kid(b.tenantA, b.schoolA, 2)
	coop := b.match(matchOpts{mode: battle.ModeCoop, grade: 2, teams: [][]kid{{a, c, d}}})
	if err := h.Open(ctx, coop, b.tenantA); err != nil {
		t.Fatal(err)
	}
	if b.count(`SELECT count(*) FROM multiplayer.rooms WHERE room_id = $1 AND room_type = 'coop'`, coop) != 1 {
		t.Fatal("coop room")
	}
	sessions := map[uuid.UUID]uuid.UUID{a.id: uuid.New(), c.id: uuid.New(), d.id: uuid.New()}
	for _, k := range []kid{a, c, d} {
		if r := b.state(h, coop, k, sessions[k.id]); !r.OK {
			t.Fatalf("coop join: %+v", r)
		}
	}
	time.Sleep(200 * time.Millisecond) // the lobby sees everyone and starts the countdown
	b.clock.Add(3 * time.Second)
	coopAnswer := natscmd.CoopAnswer(h, nil)
	for i := 0; i < 12; i++ {
		var task string
		waitFor(t, 10*time.Second, "coop task", func() bool {
			for _, m := range b.out.of(a.id, "coop.task.assigned") {
				if m["room_id"] == coop.String() {
					q := m["question"].(map[string]any)
					task = q["question_instance_id"].(string)
				}
			}
			n := 0
			for _, m := range b.out.of(a.id, "coop.task.assigned") {
				if m["room_id"] == coop.String() {
					n++
				}
			}
			return n == i+1
		})
		for _, k := range []kid{a, c, d} {
			opt := "o_wrong"
			if k.id == a.id {
				opt = rightOption
			}
			r := coopAnswer(ctx, caller(k, sessions[k.id]), json.RawMessage(fmt.Sprintf(`{"room_id":%q,"task_id":%q,"question_instance_id":%q,
				"response":{"option_id":%q}}`, coop, task, task, opt)))
			if !r.OK {
				t.Fatalf("coop answer: %+v", r)
			}
		}
		waitFor(t, 10*time.Second, "coop progress", func() bool {
			n := 0
			for _, m := range b.out.of(c.id, "coop.progress.updated") {
				if m["room_id"] == coop.String() {
					n++
				}
			}
			return n == i+1
		})
		b.clock.Add(3 * time.Second)
	}
	waitFor(t, 10*time.Second, "coop end", func() bool { return b.status(coop) == "completed" })
	if b.count(`SELECT count(*) FROM competition.match_results WHERE match_id = $1 AND NOT rated AND winner_team IS NULL`, coop) != 1 ||
		b.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1 AND outcome = 'no_contest'`, coop) != 3 {
		t.Fatal("coop result")
	}
	if len(b.plans.scores) < 36 {
		t.Fatalf("every member's answers are graded on its own: %d", len(b.plans.scores))
	}
	// coop.action.perform: a member gets VALIDATION_FAILED (no puzzle in a co-op challenge), nobody else is a member.
	if r := natscmd.CoopAction(h, nil)(ctx, caller(a, sessions[a.id]), json.RawMessage(fmt.Sprintf(`{"room_id":%q,"state_version":1,"action_type":"push"}`,
		uuid.New()))); r.OK {
		t.Fatalf("coop action: %+v", r)
	}

	// Practice bot (grade 5): the student alone against the Robot.
	s := b.kid(b.tenantA, b.schoolA, 5)
	sess := uuid.New()
	botMatch, inst := b.start(h, matchOpts{mode: battle.Mode1v1, grade: 5, isBot: true, teams: [][]kid{{s}}}, sess)
	r := b.state(h, botMatch, s, sess)
	if !strings.Contains(string(r.Result), `"nickname":"Robot"`) || !strings.Contains(string(r.Result), `"bot":true`) {
		t.Fatalf("no Robot label: %s", r.Result)
	}
	for i := 0; i < 7; i++ {
		if i > 0 {
			inst = b.shownIn(botMatch, s, i)
		}
		b.answer(h, botMatch, s, sess, inst, rightOption)
		b.clock.Add(20 * time.Second) // the bot answers within its delay
		waitFor(t, 10*time.Second, "bot reveal", func() bool {
			n := 0
			for _, m := range b.out.of(s.id, "battle.answer.evaluated") {
				if m["match_id"] == botMatch.String() {
					n++
				}
			}
			return n == i+1
		})
		b.clock.Add(3 * time.Second)
	}
	waitFor(t, 10*time.Second, "bot match end", func() bool { return b.status(botMatch) == "completed" })
	if b.count(`SELECT count(*) FROM competition.match_results WHERE match_id = $1 AND NOT rated AND summary ? 'bot'`, botMatch) != 1 ||
		b.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1`, botMatch) != 1 {
		t.Fatal("bot match result")
	}
	done := b.eventsOf("match.completed", botMatch)
	if len(done) != 1 || done[0]["rated"] != false || done[0]["is_bot"] != true {
		t.Fatalf("bot match.completed: %+v", done)
	}
}
