//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// caller is the identity the realtime gateway puts in the Ew-* headers of a command of student s.
func caller(s student) rt.Caller {
	return rt.Caller{TenantID: s.tenant, UserID: s.id, GameSessionID: uuid.New(), DeviceID: uuid.New(), ConnID: "c1", NodeID: "n1"}
}

func (e *env) bossReady(t *testing.T, s student, enc string) rt.Reply {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"encounter_id": enc, "ready": true})
	return natscmd.BossReady(e.bossesUC())(context.Background(), caller(s), b)
}

func (e *env) bossAnswer(t *testing.T, s student, enc, phase, req, inst, response string) rt.Reply {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"encounter_id": enc, "phase_id": phase, "requirement_id": req, "question_instance_id": inst,
		"response": json.RawMessage(response)})
	return natscmd.BossAnswer(e.bossesUC())(context.Background(), caller(s), b)
}

type question struct{ inst, req, phase string }

// questions returns the questions of the latest boss.question.assigned of phase sent to s.
func (e *env) questions(t *testing.T, s student, phase string) []question {
	t.Helper()
	var out []question
	for _, m := range e.bossPush.of(s.id, "boss.question.assigned") {
		if m["phase_id"] != phase {
			continue
		}
		out = nil
		for _, q := range m["questions"].([]any) {
			out = append(out, question{inst: q.(map[string]any)["question_instance_id"].(string), req: m["requirement_id"].(string), phase: phase})
		}
	}
	if len(out) == 0 {
		t.Fatalf("no questions of %s for %s", phase, s.id)
	}
	return out
}

func (e *env) createSolo(t *testing.T, s student, tok, bossID string) resp {
	t.Helper()
	return e.do(t, http.MethodPost, "/game/bosses/"+bossID+"/encounters", tok, merge(key(), map[string]string{"X-Tenant-Id": s.tenant.String()}),
		map[string]any{"mode": "solo"})
}

func replyOK(t *testing.T, r rt.Reply, what string) map[string]any {
	t.Helper()
	if !r.OK {
		t.Fatalf("%s: %+v", what, r.Problem)
	}
	var m map[string]any
	_ = json.Unmarshal(r.Result, &m)
	return m
}

func replyCode(r rt.Reply) string {
	if r.Problem == nil {
		return ""
	}
	return r.Problem.Code
}

// encounterRow reads an encounter row (status, phase, retries, outcome, end reason, energy).
func (e *env) encounterRow(ctx context.Context, t *testing.T, id string) (status, phase string, retries int, outcome, reason string, energy float64) {
	t.Helper()
	var ph, oc, rs *string
	if err := e.platform.QueryRowContext(ctx, `SELECT status, current_phase, phase_retries, outcome, end_reason, energy_remaining
		FROM game.boss_encounters WHERE encounter_id = $1`, id).Scan(&status, &ph, &retries, &oc, &rs, &energy); err != nil {
		t.Fatal(err)
	}
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return status, str(ph), retries, str(oc), str(rs), energy
}

// deliver hands an outbox event of module game (the newest of typ) to a consumer handler in the TenantScope of tenant.
func (e *env) deliverGame(ctx context.Context, t *testing.T, tenant uuid.UUID, typ string, h func(ctx context.Context, env outbox.Envelope) error) {
	t.Helper()
	ps := e.payloads(ctx, t, typ)
	if len(ps) == 0 {
		t.Fatalf("no %s event", typ)
	}
	b, _ := json.Marshal(ps[len(ps)-1])
	tt := tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: typ, SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "game", TenantID: &tt,
		Payload: b}
	if err := e.uow.InTenantTx(ctx, eventScope(t, tenant), func(ctx context.Context) error { return h(ctx, env) }); err != nil {
		t.Fatalf("deliver %s: %v", typ, err)
	}
}

// soloBoss is a solo-friendly two-phase boss: p1 collective 2 (min 1), p2 any_n 1 (1 correct of 3).
func soloBossEdit(m map[string]any) {
	ph := m["phases"].([]any)
	ph[0].(map[string]any)["completion_rule"] = map[string]any{"type": "collective_count", "collective_target": 2, "min_contribution": 1}
	ph[1].(map[string]any)["completion_rule"] = map[string]any{"type": "any_n", "n": 1}
}

const bossSolo = "0193a5f4-6c1e-7b2a-9d1f-000000010304"

// F103-AC-01 (TC-CHL-001), F103-AC-28: a solo encounter of an available boss is created in lobby, pinned to the current
// version, its ready deadline on the server clock; mode party → 422; a boss of another grade or party_required → 403
// CHALLENGE_NOT_ELIGIBLE; past challenge_attempts_per_day → 409 CHALLENGE_ATTEMPT_LIMIT; no Idempotency-Key → 428;
// listBosses pages in the database (keyset, limit + 1).
func TestBossCreateSolo(t *testing.T) {
	ctx := context.Background()
	e := setupBosses(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.device(an).token
	r := e.createSolo(t, an, tok, bossTwoPhase)
	if r.status != http.StatusCreated || r.body["status"] != "lobby" || r.body["mode"] != "solo" || r.body["boss_version"] != float64(1) {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	var deadline time.Time
	var version string
	if err := e.platform.QueryRowContext(ctx, `SELECT ready_deadline_at, boss_version_id FROM game.boss_encounters WHERE encounter_id = $1`,
		r.body["encounter_id"]).Scan(&deadline, &version); err != nil || !deadline.Equal(e.clock.Now().Add(120*time.Second)) || version == "" {
		t.Fatalf("pinned version and server ready deadline: %v %v %s", err, deadline, version)
	}
	h := merge(key(), map[string]string{"X-Tenant-Id": an.tenant.String()})
	if r := e.do(t, http.MethodPost, "/game/bosses/"+bossTwoPhase+"/encounters", tok, h, map[string]any{"mode": "party"}); r.status != http.StatusUnprocessableEntity ||
		r.code() != string(problem.ValidationFailed) {
		t.Fatalf("mode party: %d %s", r.status, r.raw)
	}
	for _, id := range []string{bossG4Only, bossRequired} {
		if r := e.createSolo(t, an, tok, id); r.status != http.StatusForbidden || r.code() != string(problem.ChallengeNotEligible) {
			t.Fatalf("not eligible %s: %d %s", id, r.status, r.raw)
		}
	}
	if r := e.createSolo(t, an, tok, uuid.NewString()); r.status != http.StatusNotFound {
		t.Fatalf("unknown boss: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/game/bosses/"+bossTwoPhase+"/encounters", tok, map[string]string{"X-Tenant-Id": an.tenant.String()},
		map[string]any{"mode": "solo"}); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no Idempotency-Key: %d %s", r.status, r.raw)
	}
	for i := 0; i < 4; i++ {
		if r := e.createSolo(t, an, tok, bossTwoPhase); r.status != http.StatusCreated {
			t.Fatalf("encounter %d: %d %s", i+2, r.status, r.raw)
		}
	}
	if r := e.createSolo(t, an, tok, bossTwoPhase); r.status != http.StatusConflict || r.code() != string(problem.ChallengeAttemptLimit) {
		t.Fatalf("6th of the day: %d %s", r.status, r.raw)
	}
	l := e.do(t, http.MethodGet, "/game/bosses?limit=1", tok, map[string]string{"X-Tenant-Id": an.tenant.String()}, nil)
	page, _ := l.body["page"].(map[string]any)
	if l.status != http.StatusOK || len(l.body["data"].([]any)) != 1 || page["has_more"] != true {
		t.Fatalf("page 1: %d %s", l.status, l.raw)
	}
	l2 := e.do(t, http.MethodGet, "/game/bosses?limit=5&cursor="+page["next_cursor"].(string), tok, map[string]string{"X-Tenant-Id": an.tenant.String()}, nil)
	if l2.status != http.StatusOK || len(l2.body["data"].([]any)) != 1 {
		t.Fatalf("page 2 (grade 3: two-phase and required, not g4 only): %d %s", l2.status, l2.raw)
	}
	authztest.Cover(t, "createBossEncounter", authztest.RoleDenied)
	authztest.Cover(t, "listBosses", authztest.RoleDenied)
}

// F103-AC-04, AC-05, AC-06, AC-07, AC-09, AC-10 (TC-CHL-002…004, TC-CHL-006): a solo encounter starts on ready with the
// question plan of the student (per grade, mode challenge, context boss_encounter) and questions without answer data;
// an answer is graded by module learning for the student of the session (never the payload), counted once; a closed
// phase is BOSS_PHASE_CLOSED; the collective target clears p1 (energy 50 %, p2 plans); p2 any_n defeats the boss; the
// result is fixed once with eligible and first_defeat; the Reward Engine grants defeat + first defeat rewards once even
// when the event is delivered three times; a wrong answer takes nothing away.
func TestBossSoloPhasesAndRewards(t *testing.T) {
	ctx := context.Background()
	e := setupBosses(ctx, t)
	e.publishBosses(ctx, t, []command.ContentItem{bossItem(bossSolo, "math.boss.solo", []int{3}, "party_optional", soloBossEdit)}, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.device(an).token
	r := e.createSolo(t, an, tok, bossSolo)
	enc := r.body["encounter_id"].(string)
	v := replyOK(t, e.bossReady(t, an, enc), "ready")
	if v["status"] != "phase_active" || v["phase_id"] != "p1" {
		t.Fatalf("phase 1 starts: %v", v)
	}
	plans := e.bossFake().planCalls()
	if len(plans) != 1 || plans[0].Participants[0] != an.id || plans[0].GradeLevel != 3 || plans[0].ActivityRef.ContentId.String() != bossActivityG3 ||
		string(*plans[0].ContextType) != "boss_encounter" || plans[0].ContextId.String() != enc || string(*plans[0].AttemptMode) != "challenge" ||
		string(plans[0].Mode) != "boss" {
		t.Fatalf("one plan of the student, grade 3, challenge: %+v", plans)
	}
	qs := e.questions(t, an, "p1")
	if raw := e.bossPush.raw(an.id); strings.Contains(raw, "correct_response") || strings.Contains(raw, "answer_key") ||
		strings.Contains(raw, "is_correct\":true") {
		t.Fatalf("no answer data in the questions: %s", raw)
	}
	if e.events(ctx, t, "boss.encounter_started") != 1 {
		t.Fatal("boss.encounter_started once")
	}
	g := e.do(t, http.MethodGet, "/game/boss-encounters/"+enc, tok, map[string]string{"X-Tenant-Id": an.tenant.String()}, nil)
	if g.status != http.StatusOK || len(g.body["my_questions"].([]any)) != 3 {
		t.Fatalf("snapshot with my questions: %d %s", g.status, g.raw)
	}
	a1 := replyOK(t, e.bossAnswer(t, an, enc, "p1", qs[0].req, qs[0].inst, answerOK), "answer 1")
	if a1["is_correct"] != true || a1["counted"] != true {
		t.Fatalf("correct counted: %v", a1)
	}
	if d := replyOK(t, e.bossAnswer(t, an, enc, "p1", qs[0].req, qs[0].inst, answerOK), "again"); d["duplicate"] != true {
		t.Fatalf("the same question counts once: %v", d)
	}
	if sc := e.bossFake().scoreCalls(); len(sc) != 1 || sc[0].StudentId != an.id {
		t.Fatalf("graded once for the student of the session: %+v", sc)
	}
	if w := replyOK(t, e.bossAnswer(t, an, enc, "p1", qs[1].req, qs[1].inst, answerWrong), "wrong"); w["is_correct"] != false {
		t.Fatalf("wrong: %v", w)
	}
	if st, ph, _, _, _, en := e.encounterRow(ctx, t, enc); st != "phase_active" || ph != "p1" || en != 1 {
		t.Fatalf("one correct of 2 needed, wrong takes nothing: %s %s %v", st, ph, en)
	}
	replyOK(t, e.bossAnswer(t, an, enc, "p1", qs[2].req, qs[2].inst, answerOK), "answer 3")
	if st, ph, _, _, _, en := e.encounterRow(ctx, t, enc); st != "phase_active" || ph != "p2" || en != 0.5 {
		t.Fatalf("p1 cleared: energy 0.5, p2: %s %s %v", st, ph, en)
	}
	if e.events(ctx, t, "boss.phase_completed") != 1 || len(e.bossPush.of(an.id, "boss.phase.completed")) != 1 {
		t.Fatal("boss.phase_completed event and push")
	}
	if c := replyCode(e.bossAnswer(t, an, enc, "p1", qs[1].req, qs[1].inst, answerOK)); c != string(problem.BossPhaseClosed) {
		t.Fatalf("an answer of the closed phase: %s", c)
	}
	q2 := e.questions(t, an, "p2")
	replyOK(t, e.bossAnswer(t, an, enc, "p2", q2[0].req, q2[0].inst, answerOK), "p2 answer")
	if st, _, _, oc, rs, en := e.encounterRow(ctx, t, enc); st != "defeated" || oc != "defeated" || rs != "defeated" || en != 0 {
		t.Fatalf("defeated: %s %s %s %v", st, oc, rs, en)
	}
	ps := e.payloads(ctx, t, "boss.encounter_completed")
	if len(ps) != 1 || ps[0]["result"] != "defeated" || ps[0]["party_activity_id"] != nil {
		t.Fatalf("one result: %v", ps)
	}
	cs := ps[0]["contributions"].([]any)
	if c := cs[0].(map[string]any); c["eligible"] != true || c["first_defeat"] != true {
		t.Fatalf("eligible, first defeat: %v", cs)
	}
	if e.audits(ctx, t, "boss.encounter_completed") != 1 {
		t.Fatal("audit of the result")
	}
	if ended := e.bossPush.of(an.id, "boss.encounter.ended"); len(ended) != 1 || ended[0]["eligible"] != true || ended[0]["outcome"] != "defeated" {
		t.Fatalf("boss.encounter.ended to the member: %v", ended)
	}
	if c := replyCode(e.bossAnswer(t, an, enc, "p2", q2[1].req, q2[1].inst, answerOK)); c != string(problem.BossPhaseClosed) {
		t.Fatalf("after the end: %s", c)
	}
	// Rewards through the Reward Engine: the event delivered 3 times grants once (TC-CHL-006, SC-002).
	re := consumer.RewardEngine{Awards: e.progressionUC()}
	for i := 0; i < 3; i++ {
		e.deliverGame(ctx, t, e.tenantA, "boss.encounter_completed", re.Handle)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.xp_ledger WHERE student_id = $1 AND source_type = 'boss'`, an.id); n != 2 {
		t.Fatalf("defeat + first defeat rewards once: %d ledger rows", n)
	}
	if n := count(ctx, t, e.platform, `SELECT COALESCE(sum(amount), 0) FROM gamification.xp_ledger WHERE student_id = $1 AND source_type = 'boss'`,
		an.id); n != 80 {
		t.Fatalf("50 + 30 XP: %d", n)
	}
	authztest.Cover(t, "getBossEncounter", authztest.RoleDenied)
}

// F103-AC-08 (TC-CHL-005), F103-AC-12 and the edge case of learning failures: the phase deadline restarts the phase with
// new questions (the old ones are closed) up to max_phase_retries, then the encounter ends time_up with its phases
// cleared; a lobby nobody readies within boss_reconnect_grace is abandoned; plans failing three times abandon the
// encounter without counting an attempt.
func TestBossRetryTimeouts(t *testing.T) {
	ctx := context.Background()
	e := setupBosses(ctx, t)
	e.publishBosses(ctx, t, []command.ContentItem{bossItem(bossSolo, "math.boss.solo", []int{3}, "party_optional", soloBossEdit)}, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.device(an).token
	enc := e.createSolo(t, an, tok, bossSolo).body["encounter_id"].(string)
	replyOK(t, e.bossReady(t, an, enc), "ready")
	old := e.questions(t, an, "p1")
	e.clock.Add(121 * time.Second)
	if done, err := e.bossesUC().OnTimer(ctx, an.tenant, uuid.MustParse(enc)); err != nil || done {
		t.Fatalf("retry: %v %v", done, err)
	}
	if st, ph, retries, _, _, _ := e.encounterRow(ctx, t, enc); st != "phase_active" || ph != "p1" || retries != 1 {
		t.Fatalf("p1 again: %s %s %d", st, ph, retries)
	}
	started := e.bossPush.of(an.id, "boss.phase.started")
	if len(started) != 2 || started[1]["retry"] != true || len(e.bossFake().planCalls()) != 2 {
		t.Fatalf("new questions: %v %d", started, len(e.bossFake().planCalls()))
	}
	if c := replyCode(e.bossAnswer(t, an, enc, "p1", old[0].req, old[0].inst, answerOK)); c != string(problem.BossPhaseClosed) {
		t.Fatalf("question of the previous round: %s", c)
	}
	e.clock.Add(121 * time.Second)
	if done, err := e.bossesUC().OnTimer(ctx, an.tenant, uuid.MustParse(enc)); err != nil || !done {
		t.Fatalf("end: %v %v", done, err)
	}
	if st, _, _, oc, rs, _ := e.encounterRow(ctx, t, enc); st != "ended" || oc != "time_up" || rs != "retries_exhausted" {
		t.Fatalf("time up past max_phase_retries: %s %s %s", st, oc, rs)
	}
	if ended := e.bossPush.of(an.id, "boss.encounter.ended"); len(ended) != 1 || ended[0]["phases_cleared"] != float64(0) ||
		ended[0]["phase_count"] != float64(2) {
		t.Fatalf("summary of the phases cleared: %v", ended)
	}
	// A lobby nobody readies within the grace is abandoned, without reward.
	lobby := e.createSolo(t, an, tok, bossSolo).body["encounter_id"].(string)
	e.clock.Add(121 * time.Second)
	if done, err := e.bossesUC().OnTimer(ctx, an.tenant, uuid.MustParse(lobby)); err != nil || !done {
		t.Fatalf("orphan lobby: %v %v", done, err)
	}
	if st, _, _, oc, rs, _ := e.encounterRow(ctx, t, lobby); st != "ended" || oc != "abandoned" || rs != "no_ready" {
		t.Fatalf("abandoned: %s %s %s", st, oc, rs)
	}
	// Learning failures: retried, then abandoned (plan_failure) and not counted as an attempt of the day.
	e.bossFake().mu.Lock()
	e.bossFake().fail = 100
	e.bossFake().mu.Unlock()
	failing := e.createSolo(t, an, tok, bossSolo).body["encounter_id"].(string)
	replyOK(t, e.bossReady(t, an, failing), "ready")
	for i := 0; i < 2; i++ {
		if _, err := e.bossesUC().OnTimer(ctx, an.tenant, uuid.MustParse(failing)); err != nil {
			t.Fatal(err)
		}
	}
	if st, _, _, oc, rs, _ := e.encounterRow(ctx, t, failing); st != "ended" || oc != "abandoned" || rs != "plan_failure" {
		t.Fatalf("plan failures: %s %s %s", st, oc, rs)
	}
	for i := 0; i < 3; i++ { // 2 counted encounters (time up, lobby) + 3 = 5: the plan failure is not counted
		if r := e.createSolo(t, an, tok, bossSolo); r.status != http.StatusCreated {
			t.Fatalf("plan failure not counted: %d %s", r.status, r.raw)
		}
	}
	if r := e.createSolo(t, an, tok, bossSolo); r.status != http.StatusConflict {
		t.Fatalf("5 per day: %d %s", r.status, r.raw)
	}
}
