//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/questrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/minigamesql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	readrepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/questschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/seedfiles"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// Seed quests v1 (src/backend/modules/game/seed/quests).
var (
	questMainA  = questCh0A // main.ch0.awakening (chapter 0, first main quest)
	questMainB  = questCh0Last
	questMath3  = "0193a5f4-6c1e-7b2a-9d1f-000000072001" // math.g3.market_helper (grade 3, side, repeatable)
	testContent = "0193a5f4-6c1e-7b2a-9d1f-0000000c0001" // learning activity of the test learning quest
)

// questCfg is the quest configuration of the tests (CONFIG_DEFAULTS values).
func questCfg() command.QuestConfig {
	return command.QuestConfig{MaxActive: 10, DailyPerDay: 3, WeeklyPerWeek: 3, Reset: quest.Reset{Hour: 4, Location: ict},
		PassRule: quest.DefaultPassRule{MinCorrectRatio: 0.7, MinQuestions: 8, MaxAttemptsPerDay: 5},
		BlockMax: map[string]int{"g1_3": 3, "g4_5": 5, "g6_9": 8}, LearningRetry: 5 * time.Second, JobBatch: 100, JobInterval: time.Minute}
}

// questPush records quest.progress.updated pushes (TEST DOUBLE of port.QuestRealtime, R17).
type questPush struct {
	mu   sync.Mutex
	msgs []map[string]any
}

func (p *questPush) QuestProgressUpdated(_ context.Context, student uuid.UUID, payload map[string]any, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := map[string]any{"student": student.String()}
	for k, v := range payload {
		m[k] = v
	}
	p.msgs = append(p.msgs, m)
	return nil
}

func (p *questPush) all() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]map[string]any(nil), p.msgs...)
}

// fakeLearning is the generated TEST DOUBLE of the learning facade (learningapi.Fake, R17) with the recorded
// createAttempt calls and a switchable failure.
type fakeLearning struct {
	mu    sync.Mutex
	calls []learningapi.AttemptCreateRequest
	keys  []string
	fail  error // returned by createAttempt when set
	delay time.Duration
	f     *learningapi.Fake
	// F073: the answer of validateMinigameQuestionSource (nil = compatible, 50 questions, the types of the rule) and its calls.
	source  *learningapi.MinigameQuestionSourceResult
	sources []learningapi.MinigameQuestionSourceCheck
}

func newFakeLearning() *fakeLearning {
	l := &fakeLearning{}
	byKey := map[string]uuid.UUID{}
	l.f = &learningapi.Fake{CreateAttemptFunc: func(ctx context.Context, req learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
		l.mu.Lock()
		fail, delay := l.fail, l.delay
		key, _ := modcall.IdempotencyKeyFrom(ctx)
		l.calls, l.keys = append(l.calls, *req.Body), append(l.keys, key)
		id, ok := byKey[key]
		if !ok {
			id = uuid.New()
			byKey[key] = id
		}
		l.mu.Unlock()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if fail != nil {
			return nil, fail
		}
		cv := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-0000000c0002")
		return learningapi.CreateAttempt201JSONResponse{AttemptId: id, Status: "in_progress", QuestionCount: 3, StartedAt: time.Now().UTC(),
			ContentVersionId: &cv}, nil
	}}
	l.f.ValidateMinigameQuestionSourceFunc = func(_ context.Context, req learningapi.ValidateMinigameQuestionSourceRequestObject) (learningapi.ValidateMinigameQuestionSourceResponseObject, error) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.sources = append(l.sources, *req.Body)
		if l.source != nil {
			return learningapi.ValidateMinigameQuestionSource200JSONResponse(*l.source), nil
		}
		out := learningapi.MinigameQuestionSourceResult{Compatible: true, AvailableCount: 50, QuestionTypes: []learningapi.QuestionType{},
			Reasons: []struct {
				Code    learningapi.MinigameQuestionSourceResultReasonsCode `json:"code"`
				Message string                                              `json:"message"`
			}{}}
		if r := req.Body.QuestionSource.Rule; r != nil && r.QuestionTypes != nil {
			out.QuestionTypes = *r.QuestionTypes
		}
		return learningapi.ValidateMinigameQuestionSource200JSONResponse(out), nil
	}
	return l
}

func (l *fakeLearning) recorded() ([]learningapi.AttemptCreateRequest, []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]learningapi.AttemptCreateRequest(nil), l.calls...), append([]string(nil), l.keys...)
}

func (l *fakeLearning) setFail(err error, delay time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fail, l.delay = err, delay
}

// questsUC returns the quest use cases on the test databases with the doubles of env.
func (e *env) questsUC() command.Quests {
	return command.Quests{UoW: e.uow, AfterCommit: entx.AfterCommit, Content: questsql.Content{}, World: worldsql.Content{}, Store: writerepo.QuestRepo{},
		Sessions: writerepo.SessionRepo{}, Events: writerepo.Publisher{W: e.writer}, Audit: writerepo.Auditor{R: e.rec}, Presence: e.presence,
		Learning: modules.LearningAttempts{F: e.learning.f}, MiniGames: minigamesql.Content{}, Realtime: e.questPush, Metrics: observability.QuestMetrics{}, Config: questCfg(),
		Now: e.clock.Now}
}

// questQueries returns the quest read use cases.
func (e *env) questQueries() query.Quests {
	codec, _ := page.NewCodec([]byte(strings.Repeat("c", 32)))
	return query.Quests{Read: e.read, Content: questsql.Content{}, World: worldsql.Content{}, Pages: readrepo.Quests{}, Cursors: codec,
		Reset: questCfg().Reset, Now: e.clock.Now}
}

// questProxy calls the current quest use cases of env.
type questProxy struct{ e *env }

func (p questProxy) Accept(ctx context.Context, id uuid.UUID, party *uuid.UUID) (questrules.ProgressView, error) {
	return p.e.questsUC().Accept(ctx, id, party)
}

func (p questProxy) Abandon(ctx context.Context, id uuid.UUID) (questrules.ProgressView, error) {
	return p.e.questsUC().Abandon(ctx, id)
}

func (p questProxy) Report(ctx context.Context, id uuid.UUID, step string, in command.ReportInput) (questrules.ProgressView, error) {
	return p.e.questsUC().Report(ctx, id, step, in)
}

func (p questProxy) StartActivity(ctx context.Context, id uuid.UUID, step string, s *uuid.UUID) (command.ActivityStart, error) {
	return p.e.questsUC().StartActivity(ctx, id, step, s)
}

type questQueryProxy struct{ e *env }

func (p questQueryProxy) List(ctx context.Context, in query.ListInput) (query.QuestPage, error) {
	return p.e.questQueries().List(ctx, in)
}

func (p questQueryProxy) Get(ctx context.Context, id uuid.UUID) (query.QuestDetail, error) {
	return p.e.questQueries().Get(ctx, id)
}

// questRules are the publish checks of the quests.
func questRules() *command.QuestPublishRules {
	c := questCfg()
	return &command.QuestPublishRules{Schema: questschema.Validator{}, Validator: quest.ValidatorConfig{PassRule: c.PassRule, BlockMaxByBand: c.BlockMax}}
}

// questSeed returns the authored quests of the seed v1.
func questSeed(t *testing.T) []command.ContentItem {
	t.Helper()
	items, err := seedfiles.Quests(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "game", "seed"))
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// questItem returns a copy of the seed quest id modified by edit.
func questItem(t *testing.T, id string, edit func(m map[string]any)) command.ContentItem {
	t.Helper()
	for _, it := range questSeed(t) {
		var m map[string]any
		if err := json.Unmarshal(it.Payload, &m); err != nil {
			t.Fatal(err)
		}
		if m["quest_id"] != id {
			continue
		}
		if edit != nil {
			edit(m)
		}
		b, _ := json.Marshal(m)
		return command.ContentItem{Type: write.ItemQuest, Payload: b}
	}
	t.Fatalf("no seed quest %s", id)
	return command.ContentItem{}
}

// learningQuest is a learning quest of grade 3 in learning_zone.math (test content): talk_npc → activity (3 questions,
// pass ratio 0.75) → collect (optional) → talk_npc.
func learningQuest(id, code string, edit func(m map[string]any)) command.ContentItem {
	m := map[string]any{"schema_version": 2, "quest_id": id, "code": code, "owner_scope": "platform", "quest_type": "learning",
		"name": map[string]any{"vi": "Bài toán phiên chợ", "en": "Market problems"}, "description": map[string]any{"vi": "Giải bài toán", "en": "Solve problems"},
		"giver_npc_id": "npc.mr_numbers", "area_id": "learning_zone.math", "grade_levels": []int{3}, "subject_id": "0193a5f4-6c1e-7b2a-9d1f-0000000720f1",
		"subject_code": "math", "objective_ids": []string{"0193a5f4-6c1e-7b2a-9d1f-0000000a0001"}, "party_mode": "solo",
		"start_trigger": map[string]any{"kind": "quest_board"}, "completion_rule": map[string]any{"type": "all_required_steps"},
		"rewards": []any{map[string]any{"type": "xp", "amount": 40}},
		"steps": []any{
			map[string]any{"step_id": "meet", "order": 1, "type": "talk_npc", "title": map[string]any{"vi": "Gặp Ông Số", "en": "Meet Mr. Numbers"},
				"npc_id": "npc.mr_numbers", "area_id": "learning_zone.math"},
			map[string]any{"step_id": "solve", "order": 2, "type": "activity", "title": map[string]any{"vi": "Giải 3 bài", "en": "Solve 3 problems"},
				"hint":        map[string]any{"vi": "Ôn bảng nhân 6", "en": "Review the 6 times table"},
				"content_ref": map[string]any{"kind": "activity", "id": testContent},
				"pass_rule":   map[string]any{"min_correct_ratio": 0.75, "min_questions": 3, "max_attempts_per_day": 2}},
			map[string]any{"step_id": "apples", "order": 3, "type": "collect", "optional": true, "title": map[string]any{"vi": "Nhặt táo", "en": "Pick apples"},
				"area_id": "learning_zone.math", "interactable_ids": []string{"quest.math.apple_basket_1", "quest.math.apple_basket_2"}, "count": 2},
			map[string]any{"step_id": "back", "order": 4, "depends_on": []string{"solve"}, "type": "talk_npc",
				"title": map[string]any{"vi": "Báo lại", "en": "Report back"}, "npc_id": "npc.mr_numbers", "area_id": "learning_zone.math"},
		}}
	if edit != nil {
		edit(m)
	}
	b, _ := json.Marshal(m)
	return command.ContentItem{Type: write.ItemQuest, Payload: b}
}

// publishQuests publishes the world seed (once) and items, then assigns every quest of items to tenants.
func (e *env) publishQuests(ctx context.Context, t *testing.T, items []command.ContentItem, tenants ...uuid.UUID) {
	t.Helper()
	if _, err := e.publish(ctx, t, items, "quests"); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		var m struct {
			QuestID uuid.UUID `json:"quest_id"`
		}
		_ = json.Unmarshal(it.Payload, &m)
		for _, tenant := range tenants {
			e.assign(ctx, t, m.QuestID, tenant)
		}
	}
}

// assign writes the active assignment of a quest to tenant (as the assignment saga or the seed would).
func (e *env) assign(ctx context.Context, t *testing.T, questID, tenant uuid.UUID) {
	t.Helper()
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at, updated_at,
			resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, now(), now(), 'quest', $3, $4, now()) ON CONFLICT DO NOTHING`, uuid.New(), tenant, questID, uuid.Nil); err != nil {
		t.Fatal(err)
	}
}

// unassign soft-deletes the assignment of a quest to tenant (revoked).
func (e *env) unassign(ctx context.Context, t *testing.T, questID, tenant uuid.UUID) {
	t.Helper()
	if _, err := e.migrator.ExecContext(ctx, `UPDATE infra.tenant_resource_assignments SET deleted_at = now()
		WHERE resource_type = 'quest' AND resource_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, questID, tenant); err != nil {
		t.Fatal(err)
	}
}

// setupQuests is the environment with the world seed, the quest seed assigned to tenants A and B, and the test learning
// quest assigned to tenant A.
func setupQuests(ctx context.Context, t *testing.T) *env {
	t.Helper()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	items := questSeed(t)
	e.publishQuests(ctx, t, items, e.tenantA, e.tenantB)
	e.publishQuests(ctx, t, []command.ContentItem{learningQuest(learnQuest, "math.g3.test_learning", nil)}, e.tenantA)
	return e
}

const learnQuest = "0193a5f4-6c1e-7b2a-9d1f-0000000b0001"

func key() map[string]string { return map[string]string{"Idempotency-Key": newKey()} }

func merge(a map[string]string, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// accept calls acceptQuest with an Idempotency-Key (k = "" makes a new one).
func (e *env) accept(t *testing.T, s student, tok, questID, k string) resp {
	t.Helper()
	if k == "" {
		k = newKey()
	}
	return e.do(t, http.MethodPost, "/game/quests/"+questID+":accept", tok, merge(th(s), map[string]string{"Idempotency-Key": k}), nil)
}

// report calls reportQuestStep.
func (e *env) report(t *testing.T, s student, tok, questID, step, k string, body map[string]any) resp {
	t.Helper()
	if k == "" {
		k = newKey()
	}
	return e.do(t, http.MethodPost, "/game/quests/"+questID+"/steps/"+step+":report", tok, merge(th(s), map[string]string{"Idempotency-Key": k}), body)
}

// startActivity calls startQuestActivity.
func (e *env) startActivity(t *testing.T, s student, tok, questID, step, k string) resp {
	t.Helper()
	if k == "" {
		k = newKey()
	}
	return e.do(t, http.MethodPost, "/game/quests/"+questID+"/steps/"+step+"/activities:start", tok, merge(th(s), map[string]string{"Idempotency-Key": k}), nil)
}

func stepStatus(r resp, step string) string {
	ss, _ := r.body["step_states"].(map[string]any)
	st, _ := ss[step].(map[string]any)
	s, _ := st["status"].(string)
	return s
}

// graded delivers activity.graded of attempt to the consumer game.quest_on_activity_graded (TenantScope of the event).
func (e *env) graded(ctx context.Context, t *testing.T, s student, attempt string, correct, items int, eventID uuid.UUID) error {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"attempt_id": attempt, "student_id": s.id, "mode": "quest", "correct_count": correct, "item_count": items,
		"passed": correct*4 >= items*3})
	tt := s.tenant
	env := outbox.Envelope{EventID: eventID, EventType: "activity.graded", SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "learning",
		TenantID: &tt, Payload: b}
	c := consumer.QuestOnActivityGraded{Quests: e.questsUC()}
	return e.uow.InTenantTx(ctx, scopeOf(t, s.tenant), func(ctx context.Context) error { return c.Handle(ctx, env) })
}

// discovered delivers world.discovery_found to the consumer game.quest_on_discovery_found.
func (e *env) discovered(ctx context.Context, t *testing.T, s student, area, id string) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"student_id": s.id, "area_id": area, "interactable_id": id, "kind": "discovery"})
	tt := s.tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: "world.discovery_found", SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "game",
		TenantID: &tt, Payload: b}
	c := consumer.QuestOnDiscoveryFound{Quests: e.questsUC()}
	if err := e.uow.InTenantTx(ctx, scopeOf(t, s.tenant), func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatal(err)
	}
}

// sessionStarted delivers game_session.started to the consumer game.periodic_quests_on_session_started.
func (e *env) sessionStarted(ctx context.Context, t *testing.T, s student, at time.Time) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"game_session_id": uuid.New(), "student_id": s.id, "platform": "windows", "client_version": "1.2.3",
		"started_at": at})
	tt := s.tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: "game_session.started", SchemaVersion: 1, OccurredAt: at, Producer: "game", TenantID: &tt,
		Payload: b}
	c := consumer.PeriodicQuestsOnSessionStarted{Quests: e.questsUC()}
	if err := e.uow.InTenantTx(ctx, scopeOf(t, s.tenant), func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatal(err)
	}
}

// progressRow returns status and step_states of the progress of s on questID (latest).
func (e *env) progressRow(ctx context.Context, t *testing.T, s student, questID string) (string, map[string]quest.StepState) {
	t.Helper()
	var status string
	var raw []byte
	if err := e.platform.QueryRowContext(ctx, `SELECT status, step_states FROM game.quest_progress WHERE student_id = $1 AND quest_id = $2
		ORDER BY started_at DESC, created_at DESC LIMIT 1`, s.id, questID).Scan(&status, &raw); err != nil {
		t.Fatal(err)
	}
	var st map[string]quest.StepState
	_ = json.Unmarshal(raw, &st)
	return status, st
}

func codeIs(err error, c problem.Code) bool {
	var pe *problem.Error
	return errors.As(err, &pe) && pe.Code == c
}

// questsWith returns the quest use cases with another configuration.
func questsWith(e *env, cfg command.QuestConfig) command.Quests {
	q := e.questsUC()
	q.Config = cfg
	return q
}

// played is a student in a game session with its presence in an area.
type played struct {
	s       student
	d       device
	session string
}

// play starts a game session for s and enters area (presence of F071).
func (e *env) play(t *testing.T, s student, area string) played {
	t.Helper()
	d, session := e.session(t, s)
	if r := e.enter(t, s, d, session, area, ""); r.status != http.StatusOK {
		t.Fatalf("enter %s: %d %s", area, r.status, r.raw)
	}
	return played{s: s, d: d, session: session}
}

// startActivityAfterMeet accepts nothing: it completes the step meet of the test learning quest (talk to Mr. Numbers in
// learning_zone.math) and starts its learning step solve.
func (e *env) startActivityAfterMeet(_ context.Context, t *testing.T, s student, tok string) resp {
	t.Helper()
	p := e.play(t, s, "learning_zone.math")
	if r := e.report(t, s, p.d.token, learnQuest, "meet", "", map[string]any{"game_session_id": p.session, "npc_id": "npc.mr_numbers"}); r.status != http.StatusOK {
		t.Fatalf("report meet: %d %s", r.status, r.raw)
	}
	return e.startActivity(t, s, tok, learnQuest, "solve", "")
}

func writerepoQuest() writerepo.QuestRepo { return writerepo.QuestRepo{} }

func writerepoAuditor(e *env) writerepo.Auditor { return writerepo.Auditor{R: e.rec} }

func questContent() questsql.Content { return questsql.Content{} }

// platformScope is the system platform scope of the seed and the platform consumers.
func platformScope(t *testing.T) tenancy.Scope {
	t.Helper()
	sc, err := authz.SystemPlatformScope("game-seed")
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// seedCtx is the context of the seed command (system platform scope).
func seedCtx(t *testing.T) context.Context {
	return tenancy.WithScope(context.Background(), platformScope(t))
}
