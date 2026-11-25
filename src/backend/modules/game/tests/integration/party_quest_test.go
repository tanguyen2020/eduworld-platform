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

const (
	coopShared     = "0193a5f4-6c1e-7b2a-9d1f-0000000b0201" // shared_steps: go_to → activity
	coopCollective = "0193a5f4-6c1e-7b2a-9d1f-0000000b0202" // collective_count: activity 20 correct (min 5)
	coopInterdep   = "0193a5f4-6c1e-7b2a-9d1f-0000000b0203" // interdependent 2 roles, all_members
)

// coopQuest is a party quest of grade 3 (party_optional, party_size 2–4) with the policy and steps given.
func coopQuest(id, code, policy string, steps []any, edit func(m map[string]any)) command.ContentItem {
	return learningQuest(id, code, func(m map[string]any) {
		m["quest_type"], m["party_mode"], m["party_size"], m["shared_progress_policy"] = "party", "party_optional", map[string]any{"min": 2, "max": 4}, policy
		m["steps"] = steps
		if edit != nil {
			edit(m)
		}
	})
}

func title(s string) map[string]any { return map[string]any{"vi": s, "en": s} }

func coopSet() []command.ContentItem {
	activity := func(id string, order int, extra map[string]any) map[string]any {
		m := map[string]any{"step_id": id, "order": order, "type": "activity", "title": title("Giải bài"),
			"content_ref": map[string]any{"kind": "activity", "id": testContent},
			"pass_rule":   map[string]any{"min_correct_ratio": 0.75, "min_questions": 3, "max_attempts_per_day": 5}}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	return []command.ContentItem{
		coopQuest(coopShared, "math.g3.coop_shared", "shared_steps", []any{
			map[string]any{"step_id": "walk", "order": 1, "type": "go_to", "title": title("Tới giỏ táo"), "area_id": "learning_zone.math",
				"interactable_id": "quest.math.apple_basket_1"},
			activity("solve", 2, nil)}, nil),
		coopQuest(coopCollective, "math.g3.coop_count", "collective_count", []any{
			activity("count", 1, map[string]any{"collective_target": map[string]any{"metric": "correct_answers", "count": 20, "min_contribution": 5}})}, nil),
		coopQuest(coopInterdep, "math.g3.coop_puzzle", "shared_steps", []any{
			activity("puzzle", 1, map[string]any{"type": "interdependent", "roles": []any{
				map[string]any{"role": "measurer", "title": title("Người đo"), "clue_content_ref": map[string]any{"kind": "learning_material", "id": clueMeasure}},
				map[string]any{"role": "map_reader", "title": title("Người đọc bản đồ"), "clue_content_ref": map[string]any{"kind": "learning_material", "id": clueMap}},
			}})}, nil),
	}
}

func setupCoop(ctx context.Context, t *testing.T) *env {
	t.Helper()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	e.publishQuests(ctx, t, coopSet(), e.tenantA)
	return e
}

// pump delivers every quest.step_completed of the outbox to the consumer of the co-op quests (replays are no-ops).
func (e *env) pump(ctx context.Context, t *testing.T) {
	t.Helper()
	c := consumer.PartyQuestOnStep{Quests: e.coopUC()}
	for _, p := range e.payloads(ctx, t, "quest.step_completed") {
		b, _ := json.Marshal(p)
		tt := e.tenantA
		env := outbox.Envelope{EventID: uuid.New(), EventType: "quest.step_completed", SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "game",
			TenantID: &tt, Payload: b}
		if err := e.uow.InTenantTx(ctx, eventScope(t, e.tenantA), func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
			t.Fatalf("quest.step_completed: %v", err)
		}
	}
}

// gradedCoop delivers activity.graded to the quest engine (F072) and the co-op quests (F103), then pumps the steps.
func (e *env) gradedCoop(ctx context.Context, t *testing.T, s student, attempt string, correct, items int) {
	t.Helper()
	if err := e.graded(ctx, t, s, attempt, correct, items, uuid.New()); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"attempt_id": attempt, "student_id": s.id, "mode": "quest", "correct_count": correct, "item_count": items,
		"passed": correct*4 >= items*3})
	tt := s.tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: "activity.graded", SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "learning",
		TenantID: &tt, Payload: b}
	c := consumer.PartyQuestOnGraded{Quests: e.coopUC()}
	if err := e.uow.InTenantTx(ctx, eventScope(t, s.tenant), func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatal(err)
	}
	e.pump(ctx, t)
}

// F103-AC-13, AC-14, AC-16, AC-17 (TC-PTY-006, TC-PTY-007, TC-PTY-010): startPartyActivity of a party quest pins one
// version for every member's QuestProgress (party_id) and one shared session (private_instance, idempotent); A
// completing the frame step completes it for B; a learning step completes only for the member who passed its own
// attempt (attempts never copied); quest.completed per member and one quest.party_completed.
func TestPartyQuestShared(t *testing.T) {
	ctx := context.Background()
	e := setupCoop(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	activity, party := uuid.New(), uuid.New()
	st, err := e.startParty(t, gameapi.PartyActivityTypeQuest, "math.g3.coop_shared", activity, party, an, an, binh)
	if err != nil || st.RoomType != "private_instance" || st.AreaId == nil || *st.AreaId != "learning_zone.math" {
		t.Fatalf("start: %v %+v", err, st)
	}
	if again, err := e.startParty(t, gameapi.PartyActivityTypeQuest, "math.g3.coop_shared", activity, party, an, an, binh); err != nil ||
		again.ExternalRef != st.ExternalRef {
		t.Fatalf("idempotent: %v", err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(DISTINCT quest_version_id) FROM game.quest_progress WHERE quest_id = $1 AND party_id = $2
		AND status = 'active'`, coopShared, party); n != 1 {
		t.Fatalf("one pinned version: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress WHERE quest_id = $1 AND party_id = $2`, coopShared, party); n != 2 {
		t.Fatalf("two personal progress with party_id: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.party_quest_sessions WHERE party_activity_id = $1`, activity); n != 1 {
		t.Fatalf("one shared session: %d", n)
	}
	pa := e.play(t, an, "learning_zone.math")
	if r := e.report(t, an, pa.d.token, coopShared, "walk", "", body(pa, "interactable_id", "quest.math.apple_basket_1")); r.status != http.StatusOK {
		t.Fatalf("A reports the frame step: %d %s", r.status, r.raw)
	}
	e.pump(ctx, t)
	if _, steps := e.progressRow(ctx, t, binh, coopShared); steps["walk"].Status != "completed" || steps["solve"].Status != "active" {
		t.Fatalf("the frame step counts for B: %v", steps)
	}
	if got := e.questPush.of(binh.id); len(got) == 0 {
		t.Fatal("B is told (quest.progress.updated)")
	}
	tokB := e.device(binh).token
	ra := e.startActivity(t, an, pa.d.token, coopShared, "solve", "")
	rb := e.startActivity(t, binh, tokB, coopShared, "solve", "")
	if ra.status != http.StatusCreated || rb.status != http.StatusCreated {
		t.Fatalf("own attempts: %d %d", ra.status, rb.status)
	}
	e.gradedCoop(ctx, t, an, ra.body["attempt_id"].(string), 1, 3)
	if _, steps := e.progressRow(ctx, t, an, coopShared); steps["solve"].Status != "active" {
		t.Fatal("A did not pass: still active")
	}
	e.gradedCoop(ctx, t, binh, rb.body["attempt_id"].(string), 3, 3)
	if st, steps := e.progressRow(ctx, t, binh, coopShared); st != "completed" || steps["solve"].Status != "completed" {
		t.Fatalf("B passed its own attempt: %s %v", st, steps)
	}
	if _, steps := e.progressRow(ctx, t, an, coopShared); steps["solve"].Status != "active" {
		t.Fatal("B's result is never copied to A (FR-PTY-013)")
	}
	calls, _ := e.learning.recorded()
	for _, c := range calls {
		if c.StudentId != an.id && c.StudentId != binh.id {
			t.Fatalf("attempt of another student: %v", c.StudentId)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_step_attempts WHERE student_id = $1`, an.id); n != 1 {
		t.Fatalf("A has its own attempt only: %d", n)
	}
	if e.events(ctx, t, "quest.party_completed") != 0 {
		t.Fatal("A has not completed: the session goes on")
	}
	ra2 := e.startActivity(t, an, pa.d.token, coopShared, "solve", "")
	e.gradedCoop(ctx, t, an, ra2.body["attempt_id"].(string), 3, 3)
	e.pump(ctx, t)
	if e.events(ctx, t, "quest.completed") != 2 || e.events(ctx, t, "quest.party_completed") != 1 {
		t.Fatalf("quest.completed per member, one quest.party_completed: %d %d", e.events(ctx, t, "quest.completed"),
			e.events(ctx, t, "quest.party_completed"))
	}
	pc := e.payloads(ctx, t, "quest.party_completed")[0]
	if pc["party_activity_id"] != activity.String() || pc["outcome"] != "completed" {
		t.Fatalf("party completion: %v", pc)
	}
}

// F103-AC-15 (TC-PTY-007): the collective step adds the correct answers of the members' own attempts; the step completes
// when the total reaches count, only for members with min_contribution — B with 3 of 5 waits, then completes with its
// own next attempt; quest.progress.updated carries the shared progress and the own contribution only.
func TestPartyQuestCollective(t *testing.T) {
	ctx := context.Background()
	e := setupCoop(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if _, err := e.startParty(t, gameapi.PartyActivityTypeQuest, "math.g3.coop_count", uuid.New(), uuid.New(), an, an, binh); err != nil {
		t.Fatal(err)
	}
	ta, tb := e.device(an).token, e.device(binh).token
	attempt := func(s student, tok string) string {
		r := e.startActivity(t, s, tok, coopCollective, "count", "")
		if r.status != http.StatusCreated {
			t.Fatalf("start: %d %s", r.status, r.raw)
		}
		return r.body["attempt_id"].(string)
	}
	e.gradedCoop(ctx, t, an, attempt(an, ta), 15, 15)
	e.gradedCoop(ctx, t, binh, attempt(binh, tb), 3, 4)
	if _, s := e.progressRow(ctx, t, an, coopCollective); s["count"].Status != "active" {
		t.Fatal("18 of 20: not done")
	}
	e.gradedCoop(ctx, t, an, attempt(an, ta), 5, 5)
	if st, s := e.progressRow(ctx, t, an, coopCollective); st != "completed" || s["count"].Status != "completed" {
		t.Fatalf("23 ≥ 20 and A ≥ 5: done for A: %s %v", st, s)
	}
	if _, s := e.progressRow(ctx, t, binh, coopCollective); s["count"].Status != "active" {
		t.Fatal("B has 3 < min_contribution 5: not counted")
	}
	var last map[string]any
	for _, m := range e.questPush.of(binh.id) {
		if pp, ok := m["party_progress"].(map[string]any); ok {
			last = pp
		}
	}
	if last == nil || last["collective_progress"] != float64(1) || last["my_contribution_met"] != false {
		t.Fatalf("B sees the shared progress and its own contribution only: %v", last)
	}
	for _, m := range e.questPush.of(binh.id) {
		raw, _ := json.Marshal(m)
		if strings.Contains(string(raw), an.id.String()) {
			t.Fatalf("no data of A in the pushes to B: %s", raw)
		}
	}
	e.gradedCoop(ctx, t, binh, attempt(binh, tb), 2, 2)
	if st, _ := e.progressRow(ctx, t, binh, coopCollective); st != "completed" {
		t.Fatalf("B reaches 5 with its own attempt: %s", st)
	}
	if e.events(ctx, t, "quest.party_completed") != 1 {
		t.Fatal("one party completion")
	}
}

// F103-AC-19 (TC-PTY-008): a member who leaves keeps its progress and the shared steps; party_optional goes on; a
// disbanded party applies failure_conditions party_disbanded (fail_quest here) and closes the session once.
func TestPartyQuestLeaveDisband(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	items := coopSet()
	items = append(items, coopQuest("0193a5f4-6c1e-7b2a-9d1f-0000000b0204", "math.g3.coop_fail", "shared_steps", []any{
		map[string]any{"step_id": "walk", "order": 1, "type": "go_to", "title": title("Tới giỏ táo"), "area_id": "learning_zone.math",
			"interactable_id": "quest.math.apple_basket_1"}}, func(m map[string]any) {
		m["failure_conditions"] = []any{map[string]any{"kind": "party_disbanded", "on_fail": "fail_quest"}}
	}))
	e.publishQuests(ctx, t, items, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	party := uuid.New()
	if _, err := e.startParty(t, gameapi.PartyActivityTypeQuest, "math.g3.coop_shared", uuid.New(), party, an, an, binh); err != nil {
		t.Fatal(err)
	}
	pa := e.play(t, an, "learning_zone.math")
	e.report(t, an, pa.d.token, coopShared, "walk", "", body(pa, "interactable_id", "quest.math.apple_basket_1"))
	e.pump(ctx, t)
	pm := consumer.PartyMembership{Bosses: e.bossesUC(), Quests: e.coopUC()}
	e.partyEvent(ctx, t, pm.Handle, "party.member_left", party, binh.id)
	if st, steps := e.progressRow(ctx, t, binh, coopShared); st != "active" || steps["walk"].Status != "completed" {
		t.Fatalf("B keeps its progress and the shared step: %s %v", st, steps)
	}
	if e.events(ctx, t, "quest.party_completed") != 0 {
		t.Fatal("party_optional with one member left goes on")
	}
	party2 := uuid.New()
	if _, err := e.startParty(t, gameapi.PartyActivityTypeQuest, "math.g3.coop_fail", uuid.New(), party2, an, an, binh); err != nil {
		t.Fatal(err)
	}
	e.partyEvent(ctx, t, pm.Handle, "party.disbanded", party2, uuid.Nil)
	e.partyEvent(ctx, t, pm.Handle, "party.disbanded", party2, uuid.Nil)
	if st, _ := e.progressRow(ctx, t, an, "0193a5f4-6c1e-7b2a-9d1f-0000000b0204"); st != "failed" {
		t.Fatalf("fail_quest on party_disbanded: %s", st)
	}
	var outcomes []string
	for _, p := range e.payloads(ctx, t, "quest.party_completed") {
		outcomes = append(outcomes, p["outcome"].(string)+"/"+p["end_reason"].(string))
	}
	if len(outcomes) != 1 || outcomes[0] != "failed/disbanded" {
		t.Fatalf("the session closes once: %v", outcomes)
	}
}

// F103-AC-20…AC-24 (TC-PTY-008, TC-PTY-010): roles are assigned in join order when the interdependent step becomes active
// (quest.roles.assigned); a member reads its own card and the shared ones only (never an answer, never a card not
// shared); only the holder shares (403 otherwise), once; only the leader assigns (403 PARTY_NOT_LEADER); each member
// answers the final question in its own attempt and the step completes for all members once everybody passed; a member
// past coop_quest_reconnect_grace hands its role over; a solo player of the party_optional quest holds every role.
func TestInterdependentStep(t *testing.T) {
	ctx := context.Background()
	e := setupCoop(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if _, err := e.startParty(t, gameapi.PartyActivityTypeQuest, "math.g3.coop_puzzle", uuid.New(), uuid.New(), an, an, binh); err != nil {
		t.Fatal(err)
	}
	ra := e.bossPush.of(an.id, "quest.roles.assigned")
	if len(ra) != 1 || ra[0]["my_roles"].([]any)[0] != "measurer" || e.bossPush.of(binh.id, "quest.roles.assigned")[0]["my_roles"].([]any)[0] != "map_reader" {
		t.Fatalf("roles in join order: %v", ra)
	}
	ta, tb := e.device(an).token, e.device(binh).token
	ha, hb := map[string]string{"X-Tenant-Id": an.tenant.String()}, map[string]string{"X-Tenant-Id": binh.tenant.String()}
	base := "/game/quests/" + coopInterdep + "/steps/puzzle"
	cb := e.do(t, http.MethodGet, base+"/clues", tb, hb, nil)
	if cb.status != http.StatusOK || !strings.Contains(cb.raw, clueMap) || strings.Contains(cb.raw, clueMeasure) {
		t.Fatalf("B reads its own card only: %d %s", cb.status, cb.raw)
	}
	if r := e.do(t, http.MethodPost, base+"/clues/measurer:share", tb, merge(key(), hb), nil); r.status != http.StatusForbidden ||
		r.code() != string(problem.PermissionDenied) {
		t.Fatalf("B shares the card of A: %d %s", r.status, r.raw)
	}
	k := key()
	if r := e.do(t, http.MethodPost, base+"/clues/measurer:share", ta, merge(k, ha), nil); r.status != http.StatusOK {
		t.Fatalf("A shares: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, base+"/clues/measurer:share", ta, merge(key(), ha), nil); r.status != http.StatusOK {
		t.Fatalf("shared again: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT jsonb_array_length(shared_clues) FROM game.party_quest_sessions WHERE quest_id = $1`, coopInterdep); n != 1 {
		t.Fatalf("no duplicate card: %d", n)
	}
	if got := e.bossPush.of(binh.id, "quest.clue.shared"); len(got) != 1 || got[0]["role"] != "measurer" {
		t.Fatalf("B is told once: %v", got)
	}
	cb = e.do(t, http.MethodGet, base+"/clues", tb, hb, nil)
	if !strings.Contains(cb.raw, clueMeasure) || !strings.Contains(cb.raw, clueMap) {
		t.Fatalf("B reads both now: %s", cb.raw)
	}
	assign := map[string]any{"assignments": []any{map[string]any{"role": "measurer", "student_id": binh.id}, map[string]any{"role": "map_reader", "student_id": an.id}}}
	if r := e.do(t, http.MethodPut, base+"/roles", tb, merge(key(), hb), assign); r.status != http.StatusForbidden || r.code() != string(problem.PartyNotLeader) {
		t.Fatalf("B is not the leader: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPut, base+"/roles", ta, merge(key(), ha), assign); r.status != http.StatusOK || len(r.body["assignments"].([]any)) != 2 {
		t.Fatalf("the leader assigns: %d %s", r.status, r.raw)
	}
	// Final question: own attempts; the step completes for everybody once all passed (all_members).
	aa := e.startActivity(t, an, ta, coopInterdep, "puzzle", "")
	ab := e.startActivity(t, binh, tb, coopInterdep, "puzzle", "")
	e.gradedCoop(ctx, t, an, aa.body["attempt_id"].(string), 3, 3)
	if _, s := e.progressRow(ctx, t, an, coopInterdep); s["puzzle"].Status != "active" {
		t.Fatal("A passed but B has not: all_members waits")
	}
	e.gradedCoop(ctx, t, binh, ab.body["attempt_id"].(string), 1, 3)
	if _, s := e.progressRow(ctx, t, binh, coopInterdep); s["puzzle"].Status != "active" {
		t.Fatal("B below pass_rule: not completed")
	}
	ab2 := e.startActivity(t, binh, tb, coopInterdep, "puzzle", "")
	e.gradedCoop(ctx, t, binh, ab2.body["attempt_id"].(string), 3, 3)
	for _, s := range []student{an, binh} {
		if st, _ := e.progressRow(ctx, t, s, coopInterdep); st != "completed" {
			t.Fatalf("both completed: %s", st)
		}
	}
	// Disconnect past the grace: the role moves to the member left (allow_multi_role), shared cards stay.
	chi := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dao := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	st, err := e.startParty(t, gameapi.PartyActivityTypeQuest, "math.g3.coop_puzzle", uuid.New(), uuid.New(), chi, chi, dao)
	if err != nil {
		t.Fatal(err)
	}
	if r := e.do(t, http.MethodPost, base+"/clues/map_reader:share", e.device(dao).token, merge(key(), map[string]string{"X-Tenant-Id": dao.tenant.String()}),
		nil); r.status != http.StatusOK {
		t.Fatalf("dao shares: %d %s", r.status, r.raw)
	}
	e.online.set(dao.id, false)
	if _, err := e.coopUC().OnTimer(ctx, e.tenantA, st.ExternalRef); err != nil {
		t.Fatal(err)
	}
	e.clock.Add(121 * time.Second)
	if _, err := e.coopUC().OnTimer(ctx, e.tenantA, st.ExternalRef); err != nil {
		t.Fatal(err)
	}
	cc := e.do(t, http.MethodGet, base+"/clues", e.device(chi).token, map[string]string{"X-Tenant-Id": chi.tenant.String()}, nil)
	if cc.status != http.StatusOK || len(cc.body["my_roles"].([]any)) != 2 || !strings.Contains(cc.raw, clueMap) {
		t.Fatalf("chi holds both roles, the shared card stays: %d %s", cc.status, cc.raw)
	}
	// Solo player of a party_optional quest: every role and every card.
	em := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	te := e.device(em).token
	if r := e.accept(t, em, te, coopInterdep, ""); r.status != http.StatusOK {
		t.Fatalf("solo accept: %d %s", r.status, r.raw)
	}
	cs := e.do(t, http.MethodGet, base+"/clues", te, map[string]string{"X-Tenant-Id": em.tenant.String()}, nil)
	if cs.status != http.StatusOK || len(cs.body["my_roles"].([]any)) != 2 || !strings.Contains(cs.raw, clueMeasure) || !strings.Contains(cs.raw, clueMap) {
		t.Fatalf("solo holds every role: %d %s", cs.status, cs.raw)
	}
	// Isolation (F103-AC-27): a student of another tenant or without the quest gets 404.
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	if r := e.do(t, http.MethodGet, base+"/clues", e.device(dung).token, map[string]string{"X-Tenant-Id": dung.tenant.String()}, nil); r.status != http.StatusNotFound {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "getQuestStepClues", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "shareQuestStepClue", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "assignQuestStepRoles", authztest.CrossTenant, authztest.RoleDenied)
}

// of returns the quest.progress.updated pushes to student.
func (p *questPush) of(student uuid.UUID) []map[string]any {
	var out []map[string]any
	for _, m := range p.all() {
		if m["student"] == student.String() {
			out = append(out, m)
		}
	}
	return out
}
