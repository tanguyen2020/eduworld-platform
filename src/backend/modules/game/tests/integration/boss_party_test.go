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

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// partyBossEdit: p2 any_n 1 so that a member who did nothing in p2 is not eligible when the boss falls.
func partyBossEdit(m map[string]any) {
	m["phases"].([]any)[1].(map[string]any)["completion_rule"] = map[string]any{"type": "any_n", "n": 1}
}

const bossParty = "0193a5f4-6c1e-7b2a-9d1f-000000010305"

func members(ss ...student) []gameapi.PartyActivityMember {
	out := make([]gameapi.PartyActivityMember, len(ss))
	for i, s := range ss {
		out[i] = gameapi.PartyActivityMember{StudentId: s.id}
	}
	return out
}

// startParty calls startPartyActivity as module competition would (Idempotency-Key = party_activity_id).
func (e *env) startParty(t *testing.T, typ gameapi.PartyActivityType, ref string, activity, party uuid.UUID, leader student, ss ...student) (gameapi.StartPartyActivity201JSONResponse, error) {
	t.Helper()
	b := gameapi.StartPartyActivityJSONRequestBody{PartyActivityId: activity, PartyId: party, LeaderId: leader.id, ActivityType: typ, ActivityRef: ref}
	for _, s := range ss {
		b.Members = append(b.Members, gameapi.PartyActivityStartMember{StudentId: s.id})
	}
	r, err := e.partyFacade().StartPartyActivity(competitionCtx(t, leader, activity.String()), gameapi.StartPartyActivityRequestObject{Body: &b})
	if err != nil {
		return gameapi.StartPartyActivity201JSONResponse{}, err
	}
	return r.(gameapi.StartPartyActivity201JSONResponse), nil
}

// F103-AC-02, AC-03, AC-26, AC-28 (TC-PTY-006, TC-SVC-005): the candidates and the eligibility per member carry reason
// codes and the requirement of the activity — never the grade or school of a student; startPartyActivity creates one
// party encounter per party_activity_id (same encounter again), refuses a member of another tenant (422) and needs the
// Idempotency-Key (428).
func TestBossPartyFacade(t *testing.T) {
	ctx := context.Background()
	e := setupBosses(ctx, t)
	e.publishBosses(ctx, t, []command.ContentItem{bossItem(bossParty, "math.boss.party", []int{3, 4}, "party_optional", partyBossEdit)}, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active")
	e.setGrade(ctx, t, binh, 4)
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	f := e.partyFacade()
	ctxA := competitionCtx(t, an, "")
	party := uuid.New()
	c, err := f.ListPartyActivityCandidates(ctxA, gameapi.ListPartyActivityCandidatesRequestObject{Body: &gameapi.ListPartyActivityCandidatesJSONRequestBody{
		PartyId: party, Members: members(an, binh)}})
	if err != nil {
		t.Fatal(err)
	}
	byRef := map[string]gameapi.PartyActivityCandidate{}
	for _, a := range c.(gameapi.ListPartyActivityCandidates200JSONResponse).Data {
		byRef[a.ActivityRef] = a
	}
	if a := byRef["math.boss.party"]; !a.AllEligible || a.MinMembers != 2 || a.MaxMembers != 4 {
		t.Fatalf("grades 3 and 4 eligible: %+v", a)
	}
	req := byRef["math.boss.required"]
	if req.AllEligible || *req.Members[1].ReasonCode != "grade_not_allowed" || *req.Members[1].Requirement != "grade_levels:3" {
		t.Fatalf("grade 3 only: reason code + requirement of the boss: %+v", req)
	}
	raw, _ := json.Marshal(c)
	if strings.Contains(string(raw), e.schoolA2.String()) || strings.Contains(string(raw), "grade_level\"") {
		t.Fatalf("no school or grade of a student: %s", raw)
	}
	if _, ok := byRef["math.boss.g4_only"]; ok {
		t.Fatal("a solo boss is no party activity")
	}
	chk, err := f.CheckPartyActivityEligibility(ctxA, gameapi.CheckPartyActivityEligibilityRequestObject{Body: &gameapi.CheckPartyActivityEligibilityJSONRequestBody{
		PartyId: party, ActivityType: gameapi.PartyActivityTypeBoss, ActivityRef: "math.boss.party", Members: members(an, binh)}})
	if v, ok := chk.(gameapi.CheckPartyActivityEligibility200JSONResponse); err != nil || !ok || !v.Eligible || !v.AllowContinueOnDisconnect {
		t.Fatalf("check: %v %+v", err, chk)
	}
	if _, err := f.CheckPartyActivityEligibility(ctxA, gameapi.CheckPartyActivityEligibilityRequestObject{Body: &gameapi.CheckPartyActivityEligibilityJSONRequestBody{
		PartyId: party, ActivityType: gameapi.PartyActivityTypeBoss, ActivityRef: "math.boss.party", Members: members(an, dung)}}); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("a member of another tenant: %v", err)
	}
	activity := uuid.New()
	s1, err := e.startParty(t, gameapi.PartyActivityTypeBoss, "math.boss.party", activity, party, an, an, binh)
	if err != nil || s1.RoomType != "boss" || s1.SceneRef == nil {
		t.Fatalf("start: %v %+v", err, s1)
	}
	s2, err := e.startParty(t, gameapi.PartyActivityTypeBoss, "math.boss.party", activity, party, an, an, binh)
	if err != nil || s2.ExternalRef != s1.ExternalRef {
		t.Fatalf("same party_activity_id → same encounter: %v %v %v", err, s1.ExternalRef, s2.ExternalRef)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.boss_encounters WHERE party_activity_id = $1 AND mode = 'party'`, activity); n != 1 {
		t.Fatalf("one encounter: %d", n)
	}
	b := gameapi.StartPartyActivityJSONRequestBody{PartyActivityId: uuid.New(), PartyId: party, LeaderId: an.id, ActivityType: gameapi.PartyActivityTypeBoss,
		ActivityRef: "math.boss.party", Members: []gameapi.PartyActivityStartMember{{StudentId: an.id}, {StudentId: binh.id}}}
	if _, err := f.StartPartyActivity(ctxA, gameapi.StartPartyActivityRequestObject{Body: &b}); problem.Status(codeOf(err)) != http.StatusPreconditionRequired {
		t.Fatalf("no Idempotency-Key: %v", err)
	}
	if _, err := e.startParty(t, gameapi.PartyActivityTypeBoss, "math.boss.party", uuid.New(), party, an, an, dung); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("member of another tenant: %v", err)
	}
	if _, err := e.startParty(t, gameapi.PartyActivityTypeBoss, "math.boss.required", uuid.New(), party, an, an, binh); codeOf(err) != problem.PartyActivityNotEligible {
		t.Fatalf("ineligible member: %v", err)
	}
}

// F103-AC-04, AC-05, AC-07, AC-09, AC-10, AC-18 (TC-CHL-002…004, TC-PTY-010): a party encounter starts when both are
// ready; each member gets questions of its own grade (per_grade); answers count for their author only; p1 needs the
// collective target and min_contribution of every present member; nobody receives the correct/wrong of another member
// (only contribution_met); the boss falls with A eligible and B not (no p2 contribution); the boss step of a quest
// completes for A only; the Reward Engine rewards A only.
func TestBossPartyFlow(t *testing.T) {
	ctx := context.Background()
	e := setupBosses(ctx, t)
	e.publishBosses(ctx, t, []command.ContentItem{bossItem(bossParty, "math.boss.party", []int{3, 4}, "party_optional", partyBossEdit)}, e.tenantA)
	bossQuest := learningQuest("0193a5f4-6c1e-7b2a-9d1f-0000000b0103", "math.g3.boss_quest", func(m map[string]any) {
		m["quest_type"], m["grade_levels"] = "boss", []int{3, 4}
		m["steps"] = []any{map[string]any{"step_id": "slay", "order": 1, "type": "boss", "title": map[string]any{"vi": "Hạ rồng", "en": "Defeat"},
			"boss_id": bossParty}}
	})
	e.publishQuests(ctx, t, []command.ContentItem{bossQuest}, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, binh, 4)
	for _, s := range []student{an, binh} {
		if r := e.accept(t, s, e.device(s).token, "0193a5f4-6c1e-7b2a-9d1f-0000000b0103", ""); r.status != http.StatusOK {
			t.Fatalf("accept boss quest: %d %s", r.status, r.raw)
		}
	}
	activity := uuid.New()
	st, err := e.startParty(t, gameapi.PartyActivityTypeBoss, "math.boss.party", activity, uuid.New(), an, an, binh)
	if err != nil {
		t.Fatal(err)
	}
	enc := st.ExternalRef.String()
	if v := replyOK(t, e.bossReady(t, an, enc), "ready A"); v["status"] != "lobby" {
		t.Fatalf("waits for B: %v", v)
	}
	if v := replyOK(t, e.bossReady(t, binh, enc), "ready B"); v["status"] != "phase_active" {
		t.Fatalf("starts: %v", v)
	}
	grades := map[uuid.UUID]string{}
	for _, p := range e.bossFake().planCalls() {
		grades[p.Participants[0]] = p.ActivityRef.ContentId.String()
	}
	if grades[an.id] != bossActivityG3 || grades[binh.id] != bossActivityG4 {
		t.Fatalf("questions of the grade of each member (per_grade): %v", grades)
	}
	qa, qb := e.questions(t, an, "p1"), e.questions(t, binh, "p1")
	for _, q := range qa {
		replyOK(t, e.bossAnswer(t, an, enc, "p1", q.req, q.inst, answerOK), "A p1")
	}
	if _, ph, _, _, _, _ := e.encounterRow(ctx, t, enc); ph != "p1" {
		t.Fatal("collective 3 of 4 and B below min_contribution: p1 goes on")
	}
	if c := replyCode(e.bossAnswer(t, binh, enc, "p1", qa[0].req, qa[0].inst, answerOK)); c != string(problem.PermissionDenied) {
		t.Fatalf("B answering a question of A: %s", c)
	}
	replyOK(t, e.bossAnswer(t, binh, enc, "p1", qb[0].req, qb[0].inst, answerWrong), "B wrong")
	replyOK(t, e.bossAnswer(t, binh, enc, "p1", qb[1].req, qb[1].inst, answerOK), "B right")
	if _, ph, _, _, _, en := e.encounterRow(ctx, t, enc); ph != "p2" || en != 0.5 {
		t.Fatalf("p1 cleared: %s %v", ph, en)
	}
	// Privacy (F103-AC-09, SC-004): B never receives a result of A — only contribution_met in the shared state.
	rawB := e.bossPush.raw(binh.id)
	for _, q := range qa {
		if strings.Contains(rawB, q.inst) {
			t.Fatalf("B received a question or result of A: %s", rawB)
		}
	}
	for _, m := range e.bossPush.of(binh.id, "boss.encounter.updated") {
		for _, mem := range m["members"].([]any) {
			for k := range mem.(map[string]any) {
				if k != "student_id" && k != "nickname" && k != "ready" && k != "connected" && k != "contribution_met" {
					t.Fatalf("member field %s in boss.encounter.updated", k)
				}
			}
		}
	}
	q2 := e.questions(t, an, "p2")
	replyOK(t, e.bossAnswer(t, an, enc, "p2", q2[0].req, q2[0].inst, answerOK), "A p2")
	if s, _, _, oc, _, _ := e.encounterRow(ctx, t, enc); s != "defeated" || oc != "defeated" {
		t.Fatalf("defeated: %s %s", s, oc)
	}
	ps := e.payloads(ctx, t, "boss.encounter_completed")
	if len(ps) != 1 || ps[0]["party_activity_id"] != activity.String() {
		t.Fatalf("result with party_activity_id: %v", ps)
	}
	el := map[string]bool{}
	for _, c := range ps[0]["contributions"].([]any) {
		m := c.(map[string]any)
		el[m["student_id"].(string)] = m["eligible"].(bool)
	}
	if !el[an.id.String()] || el[binh.id.String()] {
		t.Fatalf("A eligible, B not (no contribution in p2): %v", el)
	}
	// F103-AC-18: the boss step completes for the eligible member only.
	qb2 := consumer.QuestOnBoss{Quests: e.questsUC()}
	e.deliverGame(ctx, t, e.tenantA, "boss.encounter_completed", qb2.Handle)
	e.deliverGame(ctx, t, e.tenantA, "boss.encounter_completed", qb2.Handle)
	if st, _ := e.progressRow(ctx, t, an, "0193a5f4-6c1e-7b2a-9d1f-0000000b0103"); st != "completed" {
		t.Fatalf("A completed the boss quest: %s", st)
	}
	if st, steps := e.progressRow(ctx, t, binh, "0193a5f4-6c1e-7b2a-9d1f-0000000b0103"); st != "active" || steps["slay"].Status != "active" {
		t.Fatalf("B keeps its boss step: %s %v", st, steps)
	}
	re := consumer.RewardEngine{Awards: e.progressionUC()}
	e.deliverGame(ctx, t, e.tenantA, "boss.encounter_completed", re.Handle)
	if a, b := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.xp_ledger WHERE student_id = $1 AND source_type = 'boss'`, an.id),
		count(ctx, t, e.platform, `SELECT count(*) FROM gamification.xp_ledger WHERE student_id = $1 AND source_type = 'boss'`, binh.id); a == 0 || b != 0 {
		t.Fatalf("rewards for the eligible member only: A %d, B %d", a, b)
	}
}

// F103-AC-11, AC-12 (TC-CHL-007): an offline member is held for the reconnect grace, gets the full state back from
// another device, leaves past the grace with its contribution kept; below party_size.min the encounter is abandoned;
// leaveBossEncounter and party.member_left / party.disbanded remove members (contributions kept).
func TestBossDisconnectLeave(t *testing.T) {
	ctx := context.Background()
	e := setupBosses(ctx, t)
	e.publishBosses(ctx, t, []command.ContentItem{bossItem(bossParty, "math.boss.party", []int{3, 4}, "party_optional", partyBossEdit)}, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	carl := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	start := func(ss ...student) string {
		st, err := e.startParty(t, gameapi.PartyActivityTypeBoss, "math.boss.party", uuid.New(), uuid.New(), ss[0], ss...)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range ss {
			replyOK(t, e.bossReady(t, s, st.ExternalRef.String()), "ready")
		}
		return st.ExternalRef.String()
	}
	enc := start(an, binh)
	qb := e.questions(t, binh, "p1")
	replyOK(t, e.bossAnswer(t, binh, enc, "p1", qb[0].req, qb[0].inst, answerOK), "B answers")
	e.online.set(binh.id, false)
	if _, err := e.bossesUC().OnTimer(ctx, an.tenant, uuid.MustParse(enc)); err != nil {
		t.Fatal(err)
	}
	var disc *time.Time
	if err := e.platform.QueryRowContext(ctx, `SELECT disconnected_at FROM game.boss_encounter_members WHERE encounter_id = $1 AND student_id = $2`,
		enc, binh.id).Scan(&disc); err != nil || disc == nil {
		t.Fatalf("B held: %v %v", err, disc)
	}
	e.clock.Add(60 * time.Second)
	e.online.set(binh.id, true)
	if _, err := e.bossesUC().OnTimer(ctx, an.tenant, uuid.MustParse(enc)); err != nil {
		t.Fatal(err)
	}
	other := e.device(binh) // back from another device: the full state, its own questions
	g := e.do(t, http.MethodGet, "/game/boss-encounters/"+enc, other.token, map[string]string{"X-Tenant-Id": binh.tenant.String()}, nil)
	if g.status != http.StatusOK || g.body["status"] != "phase_active" || len(g.body["my_questions"].([]any)) != 3 {
		t.Fatalf("state back: %d %s", g.status, g.raw)
	}
	e.online.set(binh.id, false)
	if _, err := e.bossesUC().OnTimer(ctx, an.tenant, uuid.MustParse(enc)); err != nil {
		t.Fatal(err)
	}
	e.clock.Add(121 * time.Second)
	if done, err := e.bossesUC().OnTimer(ctx, an.tenant, uuid.MustParse(enc)); err != nil || !done {
		t.Fatalf("B past the grace: %v %v", done, err)
	}
	if s, _, _, oc, rs, _ := e.encounterRow(ctx, t, enc); s != "ended" || oc != "abandoned" || rs != "below_min" {
		t.Fatalf("1 < party_size.min 2: %s %s %s", s, oc, rs)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.boss_answer_results WHERE encounter_id = $1 AND student_id = $2`, enc, binh.id); n != 1 {
		t.Fatalf("contribution kept: %d", n)
	}
	// leaveBossEncounter: idempotent 204, the other member alone → abandoned.
	e.online.set(binh.id, true)
	enc2 := start(an, binh)
	h := merge(key(), map[string]string{"X-Tenant-Id": an.tenant.String()})
	tok := e.device(an).token
	if r := e.do(t, http.MethodPost, "/game/boss-encounters/"+enc2+":leave", tok, h, nil); r.status != http.StatusNoContent {
		t.Fatalf("leave: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/game/boss-encounters/"+enc2+":leave", tok, h, nil); r.status != http.StatusNoContent {
		t.Fatalf("leave replayed: %d %s", r.status, r.raw)
	}
	if s, _, _, oc, _, _ := e.encounterRow(ctx, t, enc2); s != "ended" || oc != "abandoned" {
		t.Fatalf("abandoned after the leave: %s %s", s, oc)
	}
	// party.member_left of one of three: the encounter goes on; party.disbanded: everybody leaves.
	st, err := e.startParty(t, gameapi.PartyActivityTypeBoss, "math.boss.party", uuid.New(), uuid.New(), an, an, binh, carl)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []student{an, binh, carl} {
		replyOK(t, e.bossReady(t, s, st.ExternalRef.String()), "ready")
	}
	var party uuid.UUID
	if err := e.platform.QueryRowContext(ctx, `SELECT party_id FROM game.boss_encounters WHERE encounter_id = $1`, st.ExternalRef).Scan(&party); err != nil {
		t.Fatal(err)
	}
	pm := consumer.PartyMembership{Bosses: e.bossesUC(), Quests: e.coopUC()}
	e.partyEvent(ctx, t, pm.Handle, "party.member_left", party, carl.id)
	if s, _, _, _, _, _ := e.encounterRow(ctx, t, st.ExternalRef.String()); s != "phase_active" {
		t.Fatalf("2 ≥ min: goes on: %s", s)
	}
	e.partyEvent(ctx, t, pm.Handle, "party.disbanded", party, uuid.Nil)
	if s, _, _, oc, _, _ := e.encounterRow(ctx, t, st.ExternalRef.String()); s != "ended" || oc != "abandoned" {
		t.Fatalf("disbanded: %s %s", s, oc)
	}
	authztest.Cover(t, "leaveBossEncounter", authztest.RoleDenied)
}

// partyEvent delivers a competition party event to a consumer handler of module game.
func (e *env) partyEvent(ctx context.Context, t *testing.T, h func(context.Context, outbox.Envelope) error, typ string, party, student uuid.UUID) {
	t.Helper()
	p := map[string]any{"party_id": party, "tenant_id": e.tenantA, "reason": "left", "state_version": 3}
	if student != uuid.Nil {
		p["student_id"] = student
	} else {
		p["reason"], p["disbanded_at"] = "leader_disbanded", time.Now().UTC()
	}
	b, _ := json.Marshal(p)
	tt := e.tenantA
	env := outbox.Envelope{EventID: uuid.New(), EventType: typ, SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "competition", TenantID: &tt,
		Payload: b}
	if err := e.uow.InTenantTx(ctx, eventScope(t, e.tenantA), func(ctx context.Context) error { return h(ctx, env) }); err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
}

// F103-AC-25, AC-27, AC-28, AC-06 (TC-TEN-001, TC-TEN-012, TC-TEN-024): the encounter of A is 404 for a student of
// another tenant and for a student of the same tenant who is not a member (REST); over WS another tenant gets
// PERMISSION_DENIED and a non-member ROOM_NOT_MEMBER, the encounter unchanged; a phase id over its maxLength is
// VALIDATION_FAILED; the clue routes refuse a step id or role over their maxLength.
func TestBossIsolationValidation(t *testing.T) {
	ctx := context.Background()
	e := setupBosses(ctx, t)
	e.publishBosses(ctx, t, []command.ContentItem{bossItem(bossSolo, "math.boss.solo", []int{3}, "party_optional", soloBossEdit)}, e.tenantA, e.tenantB)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	chi := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	enc := e.createSolo(t, an, e.device(an).token, bossSolo).body["encounter_id"].(string)
	replyOK(t, e.bossReady(t, an, enc), "ready")
	q := e.questions(t, an, "p1")
	for _, s := range []student{chi, dung} {
		h := map[string]string{"X-Tenant-Id": s.tenant.String()}
		if r := e.do(t, http.MethodGet, "/game/boss-encounters/"+enc, e.device(s).token, h, nil); r.status != http.StatusNotFound {
			t.Fatalf("read by %v: %d %s", s.id, r.status, r.raw)
		}
		if r := e.do(t, http.MethodPost, "/game/boss-encounters/"+enc+":leave", e.device(s).token, merge(key(), h), nil); r.status != http.StatusNotFound {
			t.Fatalf("leave by %v: %d %s", s.id, r.status, r.raw)
		}
	}
	if c := replyCode(e.bossAnswer(t, dung, enc, "p1", q[0].req, q[0].inst, answerOK)); c != string(problem.PermissionDenied) {
		t.Fatalf("another tenant over WS: %s", c)
	}
	if c := replyCode(e.bossAnswer(t, chi, enc, "p1", q[0].req, q[0].inst, answerOK)); c != string(problem.RoomNotMember) {
		t.Fatalf("not a member over WS: %s", c)
	}
	if c := replyCode(e.bossAnswer(t, an, enc, strings.Repeat("p", 65), q[0].req, q[0].inst, answerOK)); c != string(problem.ValidationFailed) {
		t.Fatalf("phase_id over 64: %s", c)
	}
	if st, _, _, _, _, _ := e.encounterRow(ctx, t, enc); st != "phase_active" || len(e.bossFake().scoreCalls()) != 0 {
		t.Fatal("the encounter is unchanged and nothing was graded")
	}
	tok := e.device(an).token
	h := merge(key(), map[string]string{"X-Tenant-Id": an.tenant.String()})
	quest := uuid.NewString()
	if r := e.do(t, http.MethodGet, "/game/quests/"+quest+"/steps/"+strings.Repeat("s", 65)+"/clues", tok, h, nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("step_id over 64: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/game/quests/"+quest+"/steps/puzzle/clues/"+strings.Repeat("r", 33)+":share", tok, h, nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("role over 32: %d %s", r.status, r.raw)
	}
	for _, c := range []struct{ method, path string }{{http.MethodPost, "/game/boss-encounters/" + enc + ":leave"},
		{http.MethodPost, "/game/quests/" + quest + "/steps/puzzle/clues/measurer:share"}, {http.MethodPut, "/game/quests/" + quest + "/steps/puzzle/roles"}} {
		var body any
		if c.method == http.MethodPut {
			body = map[string]any{"assignments": []any{map[string]any{"role": "measurer", "student_id": an.id}}}
		}
		if r := e.do(t, c.method, c.path, tok, map[string]string{"X-Tenant-Id": an.tenant.String()}, body); r.status != http.StatusPreconditionRequired {
			t.Fatalf("%s without Idempotency-Key: %d %s", c.path, r.status, r.raw)
		}
	}
	authztest.Cover(t, "getBossEncounter", authztest.CrossTenant)
	authztest.Cover(t, "leaveBossEncounter", authztest.CrossTenant)
	authztest.Cover(t, "createBossEncounter", authztest.CrossTenant)
	authztest.Cover(t, "listBosses", authztest.CrossTenant)
}
