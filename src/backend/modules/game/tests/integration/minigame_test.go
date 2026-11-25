//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/seedfiles"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F073-AC-01…AC-05 (T010, TC-ACT-003, TC-SVC-003): a valid config is published (version, typed columns, question types of
// module learning, content.published with the minigame_config snapshot, no question in it); an incompatible or too small
// question source is 422 ACTIVITY_CONFIG_INVALID with the reasons of module learning; a name without en is 422
// TRANSLATION_INCOMPLETE; a too long string, an embedded question field or adaptive difficulty is 422 VALIDATION_FAILED;
// nothing is written by a refused publish; an identical republish is unchanged.
func TestMiniGamePublish(t *testing.T) {
	ctx := context.Background()
	e := setupMiniGames(ctx, t)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_config_versions WHERE minigame_type = 'matching'
		AND status = 'published' AND question_types = '["matching"]'::jsonb AND grade_levels = '[3]'::jsonb`); n != 1 {
		t.Fatalf("published matching version: %d", n)
	}
	pubs := e.payloads(ctx, t, "content.published")
	var block map[string]any
	for _, p := range pubs {
		if p["item_type"] == "minigame_config" && p["item_id"] == mgMatching {
			block, _ = p["minigame_config"].(map[string]any)
		}
	}
	if block == nil || block["minigame_type"] != "matching" || block["version"] != float64(1) {
		t.Fatalf("content.published minigame_config block: %v", block)
	}
	snap, _ := json.Marshal(block["snapshot"])
	for _, k := range []string{`"questions"`, `"answer`, `"correct"`, `"scoring_rule"`} {
		if strings.Contains(string(snap), k) {
			t.Fatalf("snapshot carries %s: %s", k, snap)
		}
	}
	if !strings.Contains(string(snap), `"status":"published"`) || !strings.Contains(string(snap), e.currentVersion(ctx, t, mgMatching)) {
		t.Fatalf("snapshot of the stored version: %s", snap)
	}
	if res, err := e.publish(ctx, t, []command.ContentItem{minigameItem(t, mgMatching, "math.g3.multiplication_card_match", nil)}, "again"); err != nil ||
		res.Published != 0 || res.Unchanged != 1 {
		t.Fatalf("identical republish: %+v %v", res, err)
	}
	// The question source goes to module learning as authored.
	_, _ = e.learning.recorded()
	e.learning.mu.Lock()
	src := e.learning.sources[0]
	e.learning.mu.Unlock()
	if src.MinigameType != "matching" || src.QuestionCount != 3 || src.QuestionSource.Kind != "selection_rule" || src.QuestionSource.Rule == nil {
		t.Fatalf("validateMinigameQuestionSource request: %+v", src)
	}

	before := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_config_versions`)
	reasons := func(code learningapi.MinigameQuestionSourceResultReasonsCode) []struct {
		Code    learningapi.MinigameQuestionSourceResultReasonsCode `json:"code"`
		Message string                                              `json:"message"`
	} {
		return []struct {
			Code    learningapi.MinigameQuestionSourceResultReasonsCode `json:"code"`
			Message string                                              `json:"message"`
		}{{Code: code, Message: "test"}}
	}
	newID := "0193a5f4-6c1e-7b2a-9d1f-00000073a0a1"
	for name, c := range map[string]struct {
		source *learningapi.MinigameQuestionSourceResult
		edit   func(m map[string]any)
		code   problem.Code
	}{
		"incompatible": {source: &learningapi.MinigameQuestionSourceResult{Compatible: false, QuestionTypes: []learningapi.QuestionType{"single_choice"},
			AvailableCount: 20, Reasons: reasons(learningapi.IncompatibleQuestionType)}, code: problem.ActivityConfigInvalid},
		"insufficient": {source: &learningapi.MinigameQuestionSourceResult{Compatible: true, QuestionTypes: []learningapi.QuestionType{"matching"},
			AvailableCount: 2, Reasons: reasons(learningapi.InsufficientQuestions)}, code: problem.ActivityConfigInvalid},
		"set not published": {source: &learningapi.MinigameQuestionSourceResult{Compatible: false, QuestionTypes: []learningapi.QuestionType{},
			Reasons: reasons(learningapi.QuestionSetNotPublished)}, code: problem.ActivityConfigInvalid},
		"missing en": {edit: func(m map[string]any) { m["name"] = map[string]any{"vi": "Lật thẻ"} }, code: problem.TranslationIncomplete},
		"instructions without en": {edit: func(m map[string]any) { m["instructions"] = map[string]any{"vi": "Chơi", "en": " "} },
			code: problem.TranslationIncomplete},
		"too long": {edit: func(m map[string]any) {
			m["name"] = map[string]any{"vi": strings.Repeat("a", 2001), "en": "Long"}
		}, code: problem.ValidationFailed},
		"embedded question": {edit: func(m map[string]any) {
			m["questions"] = []any{map[string]any{"stem": "6 × 7 = ?", "answer": "42"}}
		}, code: problem.ValidationFailed},
		"adaptive":     {edit: func(m map[string]any) { m["difficulty"] = "adaptive" }, code: problem.ValidationFailed},
		"unknown type": {edit: func(m map[string]any) { m["minigame_type"] = "racing" }, code: problem.ValidationFailed},
	} {
		e.learning.mu.Lock()
		e.learning.source = c.source
		e.learning.mu.Unlock()
		_, err := e.publish(ctx, t, []command.ContentItem{minigameItem(t, newID, "math.g3.refused_card_match", c.edit)}, name)
		if codeOf(err) != c.code {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if after := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_config_versions`); after != before {
		t.Fatalf("refused publishes wrote versions: %d → %d", before, after)
	}
	e.learning.mu.Lock()
	e.learning.source = nil
	e.learning.mu.Unlock()
	if got := e.checkEventSchemas(ctx, t); got["content.published"] == 0 {
		t.Fatalf("schemas: %v", got)
	}
}

// F073-AC-06, AC-07, AC-12, AC-23 server part (T017, TC-LRN-002, TC-TEN-019): startMiniGame pins the current version, calls
// createAttempt (content minigame_config, mode minigame, context minigame_session of the play) with the Idempotency-Key
// of the request, records the play and answers 201 MiniGameSession with deadline/server time and a display config
// without answer; the same key replays the same session; context assignment is 422; a config not assigned to the
// tenant, of another grade or unknown is 404; module learning not ready (projection 409) is 503 Retry-After and records
// nothing.
func TestStartMiniGame(t *testing.T) {
	ctx := context.Background()
	e := setupMiniGames(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	k := newKey()
	r := e.startMiniGame(t, an, tok, mgMatching, k, map[string]any{"context_type": "free_play", "game_score": 9999})
	if r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	version := e.currentVersion(ctx, t, mgMatching)
	att, _ := r.body["attempt"].(map[string]any)
	if r.body["minigame_type"] != "matching" || r.body["minigame_config_version_id"] != version || r.body["server_time"] == nil ||
		att["mode"] != "minigame" || att["context_type"] != "minigame_session" || att["context_id"] != r.body["play_id"] {
		t.Fatalf("session: %s", r.raw)
	}
	for _, leak := range []string{"answer_key", `"correct"`, "scoring_rule", "question_source", "wrong_penalty", "reward_rule_id", "option_feedback"} {
		if strings.Contains(r.raw, leak) {
			t.Fatalf("session leaks %s: %s", leak, r.raw)
		}
	}
	cfg, _ := r.body["config"].(map[string]any)
	sc, _ := cfg["scoring"].(map[string]any)
	if sc["time_bonus_enabled"] != false || cfg["prefetch_items"] != false || cfg["question_count"] != float64(3) {
		t.Fatalf("display config of grade 3: %v", cfg)
	}
	calls, keys := e.learning.recorded()
	c := calls[len(calls)-1]
	if c.Mode != learningapi.AttemptModeMinigame || c.Activity.ContentType != learningapi.ActivityRefContentTypeMinigameConfig ||
		c.Activity.ContentId.String() != mgMatching || c.Activity.ContentVersionId == nil || c.Activity.ContentVersionId.String() != version ||
		c.Context.ContextType != learningapi.AttemptCreateRequestContextContextTypeMinigameSession || c.Context.ContextId.String() != r.body["play_id"] || keys[len(keys)-1] != k ||
		c.StudentId != an.id {
		t.Fatalf("createAttempt: %+v key %s", c, keys[len(keys)-1])
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_plays WHERE tenant_id = $1 AND student_id = $2 AND attempt_id = $3
		AND status = 'in_progress' AND context_type = 'free_play' AND minigame_config_version_id = $4`, e.tenantA, an.id, att["attempt_id"], version); n != 1 {
		t.Fatalf("play row: %d", n)
	}
	again := e.startMiniGame(t, an, tok, mgMatching, k, map[string]any{"context_type": "free_play", "game_score": 9999})
	if again.status != http.StatusCreated || again.body["play_id"] != r.body["play_id"] {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_plays WHERE student_id = $1`, an.id); n != 1 {
		t.Fatalf("plays after replay: %d", n)
	}
	// AC-12: assignment context is 422, an unknown context too.
	for _, body := range []map[string]any{{"context_type": "assignment", "context_id": uuid.NewString()}, {"context_type": "battle"},
		{"context_type": "quest_step"}} {
		if r := e.startMiniGame(t, an, tok, mgMatching, "", body); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.ValidationFailed) {
			t.Fatalf("context %v: %d %s", body, r.status, r.raw)
		}
	}
	// AC-12: a config unknown, not assigned to the tenant or of another grade is 404.
	bo := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	if r := e.startMiniGame(t, bo, e.gtoken(bo), mgMatching, "", nil); r.status != http.StatusNotFound {
		t.Fatalf("not assigned to B: %d %s", r.status, r.raw)
	}
	if r := e.startMiniGame(t, an, tok, uuid.NewString(), "", nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown: %d %s", r.status, r.raw)
	}
	g7 := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.students SET grade_level = 7 WHERE student_id = $1`, g7.id); err != nil {
		t.Fatal(err)
	}
	if r := e.startMiniGame(t, g7, e.gtoken(g7), mgMatching, "", nil); r.status != http.StatusNotFound {
		t.Fatalf("grade 7: %d %s", r.status, r.raw)
	}
	// Edge case: the projection of module learning has not received the version → 503 Retry-After, nothing recorded.
	e.learning.setFail(problem.New(problem.Conflict, "projection not ready").Retry(), 0)
	cu := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.startMiniGame(t, cu, e.gtoken(cu), mgMatching, "", nil); r.status != http.StatusServiceUnavailable || r.header.Get("Retry-After") == "" {
		t.Fatalf("projection not ready: %d %s", r.status, r.raw)
	}
	e.learning.setFail(problem.New(problem.QuestionPoolInsufficient, "pool"), 0)
	if r := e.startMiniGame(t, cu, e.gtoken(cu), mgMatching, "", nil); r.status != http.StatusUnprocessableEntity ||
		r.code() != string(problem.QuestionPoolInsufficient) {
		t.Fatalf("pool insufficient: %d %s", r.status, r.raw)
	}
	e.learning.setFail(nil, 0)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_plays WHERE student_id = $1`, cu.id); n != 0 {
		t.Fatalf("plays of a failed start: %d", n)
	}
}

// F073-AC-13 (T021, TC-AUTH-014, TC-TEN-001): startMiniGame with the token of each staff role is 403
// GAME_ACCESS_STUDENT_ONLY (also in TestPlayerRoutesStaffDenied); the X-Tenant-Id of another tenant is 403
// TENANT_MISMATCH; the plays of tenant A are invisible under the RLS of tenant B.
func TestMiniGameAuthzIsolation(t *testing.T) {
	ctx := context.Background()
	e := setupMiniGames(ctx, t)
	authztest.Cover(t, "startMiniGame", authztest.RoleDenied, authztest.CrossTenant)
	if o := gamehttp.Operations["startMiniGame"]; o.Guard == 0 || o.Pattern != "/game/minigames/{minigame_config_id}:start" {
		t.Fatalf("operation: %+v", o)
	}
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	staff := map[string]string{
		"school_admin":     e.staff(e.tenantA, "school_admin"),
		"academic_manager": e.staff(e.tenantA, "academic_manager"),
		"tenant_admin":     e.staff(e.tenantA, "tenant_admin"),
		"system_admin":     e.platformStaff("system_admin"),
	}
	for role, sub := range staff {
		r := e.do(t, http.MethodPost, "/game/minigames/"+mgMatching+":start", e.idp.Token(sub, "eduworld-portal", nowWall()),
			map[string]string{"X-Tenant-Id": e.tenantA.String(), "Idempotency-Key": newKey()}, nil)
		if r.status != http.StatusForbidden || r.code() != string(problem.GameAccessStudentOnly) {
			t.Fatalf("staff %s: %d %s", role, r.status, r.raw)
		}
	}
	r := e.do(t, http.MethodPost, "/game/minigames/"+mgMatching+":start", e.gtoken(an), map[string]string{"X-Tenant-Id": e.tenantB.String(),
		"Idempotency-Key": newKey()}, nil)
	if r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
		t.Fatalf("tenant of B: %d %s", r.status, r.raw)
	}
	if r := e.startMiniGame(t, an, e.gtoken(an), mgMatching, "", nil); r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	var seen int
	if err := e.uow.InTenantTx(ctx, scopeOf(t, e.tenantB), func(ctx context.Context) error {
		return e.app.QueryRowContext(ctx, `SELECT count(*) FROM game.minigame_plays`).Scan(&seen)
	}); err != nil {
		t.Fatal(err)
	}
	if seen != 0 || count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_plays WHERE tenant_id = $1`, e.tenantA) != 1 {
		t.Fatalf("RLS: tenant B sees %d plays", seen)
	}
}

// F073-AC-18, AC-19, AC-20 (T030, TC-SVC-003, AC-MG-005): activity.graded with the minigame block completes the play and
// publishes minigame.completed once (a replayed event with a new id publishes nothing); over max_plays_per_day the
// play is recorded as practice with rewards_eligible = false; an unmet completion rule is never eligible; a quest step
// or assignment attempt gets its play row from the event; module game grants no XP, Xu or item here.
func TestMiniGameRecordResult(t *testing.T) {
	ctx := context.Background()
	e := setupMiniGames(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	version := e.currentVersion(ctx, t, mgMatching)
	var attempts []string
	for i := 0; i < 6; i++ { // max_plays_per_day = 5
		r := e.startMiniGame(t, an, tok, mgMatching, "", nil)
		if r.status != http.StatusCreated {
			t.Fatalf("start %d: %d %s", i, r.status, r.raw)
		}
		attempts = append(attempts, r.body["attempt"].(map[string]any)["attempt_id"].(string))
	}
	first := uuid.New()
	if err := e.minigameGraded(ctx, t, an, attempts[0], mgMatching, version, "minigame_session", uuid.New(), 3, 3, 330, true, first); err != nil {
		t.Fatal(err)
	}
	if err := e.minigameGraded(ctx, t, an, attempts[0], mgMatching, version, "minigame_session", uuid.New(), 3, 3, 330, true, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if n := e.events(ctx, t, "minigame.completed"); n != 1 {
		t.Fatalf("minigame.completed after a replay: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_plays WHERE attempt_id = $1 AND status = 'completed'
		AND learning_score = 3 AND game_score = 330 AND completion_met AND rewards_eligible`, attempts[0]); n != 1 {
		t.Fatalf("completed play: %d", n)
	}
	// Plays 2…5 are eligible, play 6 is over the limit (still recorded), an unmet completion rule never is.
	for i := 1; i < 6; i++ {
		if err := e.minigameGraded(ctx, t, an, attempts[i], mgMatching, version, "minigame_session", uuid.New(), 3, 3, 300, true, uuid.New()); err != nil {
			t.Fatal(err)
		}
	}
	pays := e.payloads(ctx, t, "minigame.completed")
	if len(pays) != 6 {
		t.Fatalf("events: %d", len(pays))
	}
	for i, p := range pays {
		want := i < 5
		if p["rewards_eligible"] != want || p["plays_today"] != float64(i+1) || p["reward_rule_id"] != mgRewardRule ||
			p["minigame_config_version_id"] != version || p["context_type"] != "free_play" {
			t.Fatalf("event %d: %v", i, p)
		}
	}
	r := e.startMiniGame(t, an, tok, mgMatching, "", nil)
	if r.status != http.StatusCreated {
		t.Fatalf("play over the limit still starts: %d %s", r.status, r.raw)
	}
	if err := e.minigameGraded(ctx, t, an, r.body["attempt"].(map[string]any)["attempt_id"].(string), mgMatching, version, "minigame_session",
		uuid.New(), 1, 3, 100, false, uuid.New()); err != nil {
		t.Fatal(err)
	}
	pays = e.payloads(ctx, t, "minigame.completed")
	if last := pays[len(pays)-1]; last["completion_met"] != false || last["rewards_eligible"] != false {
		t.Fatalf("unmet: %v", last)
	}
	// A quest step attempt and an assignment attempt have no play yet: created from the event.
	quest, assignment := uuid.New(), uuid.New()
	if err := e.minigameGraded(ctx, t, an, uuid.NewString(), mgTrueFalse, e.currentVersion(ctx, t, mgTrueFalse), "quest_step", quest, 5, 5, 500, true,
		uuid.New()); err != nil {
		t.Fatal(err)
	}
	if err := e.minigameGraded(ctx, t, an, uuid.NewString(), mgTrueFalse, e.currentVersion(ctx, t, mgTrueFalse), "assignment", assignment, 4, 5, 400, true,
		uuid.New()); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_plays WHERE minigame_config_id = $1 AND status = 'completed'
		AND ((context_type = 'quest_step' AND context_id = $2) OR (context_type = 'assignment' AND context_id = $3))`, mgTrueFalse, quest, assignment); n != 2 {
		t.Fatalf("plays from events: %d", n)
	}
	// An attempt without the minigame block is not a mini-game.
	b, _ := json.Marshal(map[string]any{"attempt_id": uuid.NewString(), "student_id": an.id, "mode": "practice", "score": 1, "max_score": 1})
	tt := an.tenant
	ev := outbox.Envelope{EventID: uuid.New(), EventType: "activity.graded", SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "learning",
		TenantID: &tt, Payload: b}
	if err := e.uow.InTenantTx(ctx, scopeOf(t, an.tenant), func(ctx context.Context) error {
		return consumer.MiniGameOnActivityGraded{MiniGames: e.miniGamesUC()}.Handle(ctx, ev)
	}); err != nil {
		t.Fatal(err)
	}
	// AC-20: no XP, Xu or item from the mini-game module.
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.xp_ledger`) + count(ctx, t, e.platform,
		`SELECT count(*) FROM gamification.currency_ledger`) + count(ctx, t, e.platform, `SELECT count(*) FROM gamification.item_ledger`); n != 0 {
		t.Fatalf("rewards written by the mini-game module: %d", n)
	}
	if got := e.checkEventSchemas(ctx, t); got["minigame.completed"] != 9 {
		t.Fatalf("minigame.completed checked: %v", got)
	}
}

// F073-AC-16 (T026, TC-ACT-002): a quest with a minigame step is published (handler registered); startQuestActivity on the
// step pins the current config version and opens a quest attempt (content minigame_config, mode quest, context
// quest_step); activity.graded with completion_met = false leaves the step active, completion_met = true completes it.
func TestQuestMiniGameStep(t *testing.T) {
	ctx := context.Background()
	e := setupMiniGames(ctx, t)
	const mgQuest = "0193a5f4-6c1e-7b2a-9d1f-00000073b001"
	item := learningQuest(mgQuest, "math.g3.brown_bear_market_test", func(m map[string]any) {
		steps := m["steps"].([]any)
		steps[1] = map[string]any{"step_id": "solve", "order": 2, "type": "minigame", "title": map[string]any{"vi": "Ghép thẻ", "en": "Match cards"},
			"minigame_config_id": mgMatching, "pass_rule": map[string]any{"min_correct_ratio": 0.7, "min_questions": 3, "max_attempts_per_day": 3}}
	})
	e.publishQuests(ctx, t, []command.ContentItem{item}, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	e.accept(t, an, tok, mgQuest, "")
	p := e.play(t, an, "learning_zone.math")
	if r := e.report(t, an, p.d.token, mgQuest, "meet", "", map[string]any{"game_session_id": p.session, "npc_id": "npc.mr_numbers"}); r.status != http.StatusOK {
		t.Fatalf("report meet: %d %s", r.status, r.raw)
	}
	first := e.startActivity(t, an, tok, mgQuest, "solve", "")
	if first.status != http.StatusCreated {
		t.Fatalf("start minigame step: %d %s", first.status, first.raw)
	}
	calls, _ := e.learning.recorded()
	c := calls[len(calls)-1]
	version := e.currentVersion(ctx, t, mgMatching)
	if c.Mode != learningapi.AttemptModeQuest || c.Activity.ContentType != learningapi.ActivityRefContentTypeMinigameConfig ||
		c.Activity.ContentId.String() != mgMatching || c.Activity.ContentVersionId == nil || c.Activity.ContentVersionId.String() != version ||
		c.Context.ContextType != learningapi.AttemptCreateRequestContextContextTypeQuestStep || c.Context.StepId == nil || *c.Context.StepId != "solve" {
		t.Fatalf("createAttempt of the step: %+v", c)
	}
	grade := func(attempt string, correct int, met bool) {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"attempt_id": attempt, "student_id": an.id, "mode": "quest", "correct_count": correct, "item_count": 3,
			"passed": correct == 3, "minigame": map[string]any{"minigame_config_id": mgMatching, "minigame_config_version_id": version, "game_score": 100,
				"completion_met": met, "context_type": "quest_step", "context_id": uuid.New()}})
		tt := an.tenant
		ev := outbox.Envelope{EventID: uuid.New(), EventType: "activity.graded", SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "learning",
			TenantID: &tt, Payload: b}
		if err := e.uow.InTenantTx(ctx, scopeOf(t, an.tenant), func(ctx context.Context) error {
			return consumer.QuestOnActivityGraded{Quests: e.questsUC()}.Handle(ctx, ev)
		}); err != nil {
			t.Fatal(err)
		}
	}
	// 3/3 correct but the completion rule of the mini-game not met: the step stays active.
	grade(first.body["attempt_id"].(string), 3, false)
	if _, steps := e.progressRow(ctx, t, an, mgQuest); steps["solve"].Status != "active" {
		t.Fatalf("completion not met: %v", steps)
	}
	second := e.startActivity(t, an, tok, mgQuest, "solve", "")
	if second.status != http.StatusCreated {
		t.Fatalf("retry: %d %s", second.status, second.raw)
	}
	grade(second.body["attempt_id"].(string), 2, true)
	if _, steps := e.progressRow(ctx, t, an, mgQuest); steps["solve"].Status != "completed" || steps["back"].Status != "active" {
		t.Fatalf("completion met: %v", steps)
	}
	// A minigame step without config is refused at publish.
	bad := learningQuest("0193a5f4-6c1e-7b2a-9d1f-00000073b002", "math.g3.bad_minigame_step", func(m map[string]any) {
		steps := m["steps"].([]any)
		steps[1] = map[string]any{"step_id": "solve", "order": 2, "type": "minigame", "title": map[string]any{"vi": "Ghép thẻ", "en": "Match cards"},
			"minigame_config_id": "not-a-uuid", "pass_rule": map[string]any{"min_correct_ratio": 0.7}}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{bad}, "bad"); err == nil {
		t.Fatal("a minigame step with an invalid config id was published")
	}
}

// F073 T001: the seed mini-game configs and the sample quest "Phiên chợ của bác Gấu Nâu" (minigame step) are published
// by the release core with the learning facade (question source checked), together.
func TestMiniGameSeedFiles(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	items, err := seedfiles.MiniGames(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "game", "seed"))
	if err != nil || len(items) != 3 {
		t.Fatalf("seed items: %d %v", len(items), err)
	}
	res, err := e.publish(ctx, t, items, "seed minigames")
	if err != nil || res.Published != 3 {
		t.Fatalf("publish: %+v %v", res, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.minigame_configs WHERE current_version_id IS NOT NULL`); n != 2 {
		t.Fatalf("configs: %d", n)
	}
}
