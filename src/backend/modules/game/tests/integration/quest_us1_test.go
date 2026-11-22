//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func listCodes(r resp) map[string]map[string]any {
	out := map[string]map[string]any{}
	data, _ := r.body["data"].([]any)
	for _, d := range data {
		m := d.(map[string]any)
		out[m["code"].(string)] = m
	}
	return out
}

// F072-AC-01, AC-02, AC-03 (T012, TC-GAM-012, TC-TEN-019): listQuests shows only the published quests assigned to the
// tenant, of the grade of the student, of an enabled subject, filtered by status, quest_type and area_id; a quest of
// another grade or not assigned is 404 on getQuest and acceptQuest; the delivery has vi and en, no grading data.
func TestQuestListAndGet(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active") // grade 3
	em := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, em, 4)
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active") // grade 3, tenant B: the test learning quest is not assigned
	r := e.do(t, http.MethodGet, "/game/quests", e.gtoken(an), th(an), nil)
	got := listCodes(r)
	if r.status != http.StatusOK || got["main.ch0.awakening"] == nil || got["math.g3.market_helper"] == nil || got["math.g3.test_learning"] == nil {
		t.Fatalf("grade 3 list: %d %s", r.status, r.raw)
	}
	if got["daily.hub.lanterns"] != nil {
		t.Fatal("an auto_daily quest is listed before it is granted")
	}
	if got["main.ch0.first_steps"]["status"] != "available" || got["main.ch0.first_steps"]["can_accept"] != false ||
		got["main.ch0.first_steps"]["locked_reason"] != "prerequisite_not_met" {
		t.Fatalf("chained quest: %v", got["main.ch0.first_steps"])
	}
	if strings.Contains(r.raw, "content_ref") || strings.Contains(r.raw, "pass_rule") || strings.Contains(r.raw, "min_correct_ratio") {
		t.Fatalf("grading data in the list: %s", r.raw)
	}
	for q, want := range map[string][]string{"quest_type=learning": {"math.g3.test_learning"}, "quest_type=side": {"math.g3.market_helper"},
		"area_id=learning_zone.math": {"math.g3.market_helper", "math.g3.test_learning"}, "status=active": {}} {
		r := e.do(t, http.MethodGet, "/game/quests?"+q, e.gtoken(an), th(an), nil)
		got := listCodes(r)
		if r.status != http.StatusOK || len(got) != len(want) {
			t.Fatalf("%s: %s", q, r.raw)
		}
		for _, c := range want {
			if got[c] == nil {
				t.Fatalf("%s: no %s in %s", q, c, r.raw)
			}
		}
	}
	if r := e.do(t, http.MethodGet, "/game/quests?limit=2", e.gtoken(an), th(an), nil); len(listCodes(r)) != 2 || r.body["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("page: %s", r.raw)
	}
	// Grade 4 never sees the grade 3 quests (404, never revealed); tenant B does not see the quest assigned to A only.
	if got := listCodes(e.do(t, http.MethodGet, "/game/quests", e.gtoken(em), th(em), nil)); got["math.g3.market_helper"] != nil || got["main.ch0.awakening"] == nil {
		t.Fatalf("grade 4 list: %v", got)
	}
	for _, s := range []student{em, dung} {
		id := questMath3
		if s == dung {
			id = learnQuest
		}
		if r := e.do(t, http.MethodGet, "/game/quests/"+id, e.gtoken(s), th(s), nil); r.status != http.StatusNotFound {
			t.Fatalf("get hidden quest: %d %s", r.status, r.raw)
		}
		if r := e.accept(t, s, e.gtoken(s), id, ""); r.status != http.StatusNotFound {
			t.Fatalf("accept hidden quest: %d %s", r.status, r.raw)
		}
	}
	// A subject turned off by the school hides its quests.
	e.enableOnly(ctx, t, e.schoolA2, "vietnamese")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active")
	if got := listCodes(e.do(t, http.MethodGet, "/game/quests", e.gtoken(other), th(other), nil)); got["math.g3.market_helper"] != nil || got["main.ch0.awakening"] == nil {
		t.Fatalf("math off: %v", got)
	}
	// Delivery of the current version: bilingual texts, steps with hint, optional and depends_on, reward preview.
	d := e.do(t, http.MethodGet, "/game/quests/"+learnQuest, e.gtoken(an), th(an), nil)
	steps, _ := d.body["steps"].([]any)
	if d.status != http.StatusOK || len(steps) != 4 || d.body["name"].(map[string]any)["en"] != "Market problems" {
		t.Fatalf("detail: %d %s", d.status, d.raw)
	}
	s2 := steps[1].(map[string]any)
	s3 := steps[2].(map[string]any)
	if s2["step_type"] != "activity" || s2["hint"].(map[string]any)["vi"] == "" || s3["optional"] != true || s2["content_ref"] != nil ||
		strings.Contains(d.raw, "max_attempts_per_day") {
		t.Fatalf("steps: %v %v", s2, s3)
	}
	rw, _ := d.body["rewards_preview"].([]any)
	if len(rw) != 1 || rw[0].(map[string]any)["kind"] != "xp" {
		t.Fatalf("rewards: %v", d.body["rewards_preview"])
	}
}

// F072-AC-04, AC-05, AC-06 (T013): accept pins the current version, opens the first step and writes quest.accepted;
// the same Idempotency-Key answers the same progress; an active quest returns its progress; prerequisites missing →
// 403 QUEST_PREREQUISITE_NOT_MET with the requirements; a learning quest ignores min_level; max_active_quests → 409.
func TestQuestAccept(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	k := newKey()
	r := e.accept(t, an, tok, questMainA, k)
	if r.status != http.StatusOK || r.body["status"] != "active" || stepStatus(r, "meet_cloudy") != "active" || stepStatus(r, "read_sign") != "locked" {
		t.Fatalf("accept: %d %s", r.status, r.raw)
	}
	again := e.accept(t, an, tok, questMainA, k)
	if again.status != http.StatusOK || again.body["quest_progress_id"] != r.body["quest_progress_id"] || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %v %s", again.status, again.header, again.raw)
	}
	if other := e.accept(t, an, tok, questMainA, ""); other.body["quest_progress_id"] != r.body["quest_progress_id"] {
		t.Fatalf("active quest returns its progress: %s", other.raw)
	}
	if n := e.events(ctx, t, "quest.accepted"); n != 1 {
		t.Fatalf("quest.accepted: %d", n)
	}
	var pinned string
	if err := e.platform.QueryRowContext(ctx, `SELECT quest_version_id::text FROM game.quest_progress WHERE student_id = $1`, an.id).Scan(&pinned); err != nil ||
		pinned != r.body["quest_version_id"] {
		t.Fatalf("pinned: %s %v", pinned, err)
	}
	// Chained quest: the first main quest is not completed yet.
	pr := e.accept(t, an, tok, questMainB, "")
	if pr.status != http.StatusForbidden || pr.code() != string(problem.QuestPrerequisiteNotMet) || !strings.Contains(pr.raw, questMainA) {
		t.Fatalf("prerequisite: %d %s", pr.status, pr.raw)
	}
	// min_level never blocks a learning quest, but blocks a side quest; a learning requirement does.
	e.publishQuests(ctx, t, []command.ContentItem{learningQuest(learnQuest, "math.g3.test_learning", func(m map[string]any) {
		m["prerequisites"] = map[string]any{"min_level": 50}
	}), questItem(t, questMath3, func(m map[string]any) { m["prerequisites"] = map[string]any{"min_level": 50} })}, e.tenantA)
	if r := e.accept(t, an, tok, learnQuest, ""); r.status != http.StatusOK {
		t.Fatalf("learning quest with min_level: %d %s", r.status, r.raw)
	}
	if r := e.accept(t, an, tok, questMath3, ""); r.status != http.StatusForbidden || !strings.Contains(r.raw, `"min_level"`) {
		t.Fatalf("side quest with min_level: %d %s", r.status, r.raw)
	}
	// max_active_quests: two active, the limit is lowered to 2 for this check.
	cfg := questCfg()
	cfg.MaxActive = 2
	if _, err := (questsWith(e, cfg)).Accept(ctxOf(t, e, an), uuid.MustParse(questMath3), nil); !codeIs(err, problem.QuestPrerequisiteNotMet) {
		t.Fatalf("side quest still needs its level: %v", err)
	}
	e.publishQuests(ctx, t, []command.ContentItem{questItem(t, questMath3, nil)}, e.tenantA)
	if _, err := (questsWith(e, cfg)).Accept(ctxOf(t, e, an), uuid.MustParse(questMath3), nil); !codeIs(err, problem.QuestActiveLimit) {
		t.Fatalf("max_active_quests: %v", err)
	}
}

// F072-AC-07 (T014): abandon keeps the attempts and learning records, writes quest.abandoned (player); accepting again
// starts a new progress on the current version; abandoning without an active progress is 404.
func TestQuestAbandon(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	first := e.accept(t, an, tok, learnQuest, "")
	if r := e.startActivityAfterMeet(ctx, t, an, tok); r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	ab := e.do(t, http.MethodPost, "/game/quests/"+learnQuest+":abandon", tok, merge(th(an), key()), nil)
	if ab.status != http.StatusOK || ab.body["status"] != "abandoned" {
		t.Fatalf("abandon: %d %s", ab.status, ab.raw)
	}
	p := e.payloads(ctx, t, "quest.abandoned")
	if len(p) != 1 || p[0]["reason"] != "player" {
		t.Fatalf("quest.abandoned: %v", p)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_step_attempts WHERE student_id = $1 AND deleted_at IS NULL`, an.id); n != 1 {
		t.Fatalf("step attempts kept: %d", n)
	}
	if r := e.do(t, http.MethodPost, "/game/quests/"+learnQuest+":abandon", tok, merge(th(an), key()), nil); r.status != http.StatusNotFound {
		t.Fatalf("abandon twice: %d %s", r.status, r.raw)
	}
	again := e.accept(t, an, tok, learnQuest, "")
	if again.status != http.StatusOK || again.body["quest_progress_id"] == first.body["quest_progress_id"] || stepStatus(again, "meet") != "active" {
		t.Fatalf("accept again: %d %s", again.status, again.raw)
	}
}

// F072-AC-08 (T015, TC-AUTH-014, TC-TEN-001): every quest route is a guarded player route (TestPlayerRoutesStaffDenied
// sends the 5 staff roles to each of them → 403 GAME_ACCESS_STUDENT_ONLY); the X-Tenant-Id of another tenant →
// 403 TENANT_MISMATCH; the progress of a student of tenant B is never read in tenant A.
func TestQuestAuthzAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	for _, id := range gamehttp.QuestOperations {
		if !slices.Contains(gamehttp.PlayerOperations, id) || gamehttp.Operations[id].Guard == 0 {
			t.Fatalf("%s is not a guarded player operation", id)
		}
	}
	authztest.Cover(t, "listQuests", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "getQuest", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "acceptQuest", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "abandonQuest", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "reportQuestStep", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "startQuestActivity", authztest.RoleDenied, authztest.CrossTenant)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	if r := e.accept(t, dung, e.gtoken(dung), questMainA, ""); r.status != http.StatusOK {
		t.Fatalf("accept in B: %d %s", r.status, r.raw)
	}
	wrong := map[string]string{"X-Tenant-Id": e.tenantB.String()}
	for _, path := range []string{"/game/quests", "/game/quests/" + questMainA} {
		if r := e.do(t, http.MethodGet, path, e.gtoken(an), wrong, nil); r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
			t.Fatalf("%s tenant mismatch: %d %s", path, r.status, r.raw)
		}
	}
	if r := e.do(t, http.MethodPost, "/game/quests/"+questMainA+":accept", e.gtoken(an), merge(wrong, key()), nil); r.status != http.StatusForbidden {
		t.Fatalf("accept with the header of B: %d %s", r.status, r.raw)
	}
	if got := listCodes(e.do(t, http.MethodGet, "/game/quests?status=active", e.gtoken(an), th(an), nil)); len(got) != 0 {
		t.Fatalf("progress of tenant B listed in A: %v", got)
	}
	if r := e.do(t, http.MethodPost, "/game/quests/"+questMainA+":abandon", e.gtoken(an), merge(th(an), key()), nil); r.status != http.StatusNotFound {
		t.Fatalf("abandon the quest of another student: %d %s", r.status, r.raw)
	}
	for _, table := range []string{"quest_progress", "quest_step_attempts", "periodic_quest_grants"} {
		var n int
		err := e.uow.InTenantTx(ctx, scopeOf(t, e.tenantA), func(ctx context.Context) error {
			return txOf(ctx).QueryRowContext(ctx, `SELECT count(*) FROM game.`+table).Scan(&n)
		})
		if err != nil || n != 0 {
			t.Fatalf("%s visible in tenant A: %d %v", table, n, err)
		}
	}
}
