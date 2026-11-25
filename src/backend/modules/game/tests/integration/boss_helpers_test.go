//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/bossschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/bosssql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/partyquestsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Test bosses (F103): learning activities of the requirements, the subject of the test content (module learning is the
// generated TEST DOUBLE of the facade — R17).
const (
	bossActivityG3 = "0193a5f4-6c1e-7b2a-9d1f-0000000b0301" // activity of grade 3
	bossActivityG4 = "0193a5f4-6c1e-7b2a-9d1f-0000000b0304" // per_grade activity of grade 4
	bossTwoPhase   = "0193a5f4-6c1e-7b2a-9d1f-000000010301" // party_optional, grades 3–4, p1 collective 4 (min 1), p2 all
	bossG4Only     = "0193a5f4-6c1e-7b2a-9d1f-000000010302" // solo, grade 4 only
	bossRequired   = "0193a5f4-6c1e-7b2a-9d1f-000000010303" // party_required, grade 3
	clueMeasure    = "0193a5f4-6c1e-7b2a-9d1f-0000000f1031"
	clueMap        = "0193a5f4-6c1e-7b2a-9d1f-0000000f1032"
	answerOK       = `{"option_id":"o_ok"}`
	answerWrong    = `{"option_id":"o_no"}`
	answerRapid    = `{"option_id":"o_rapid"}`
)

// bossCfg is the configuration of the tests (CONFIG_DEFAULTS values: boss_reconnect_grace 120 s, 5 per day).
func bossCfg() command.BossConfig {
	return command.BossConfig{Grace: 120 * time.Second, CheckEvery: 5 * time.Second, PlanRetries: 2, AttemptsPerDay: 5,
		HeartbeatTimeout: 45 * time.Second, Reset: quest.Reset{Hour: 4, Location: ict}, QuestionCount: 10}
}

func bossRules() *command.BossPublishRules {
	return &command.BossPublishRules{Schema: bossschema.Validator{}, Content: bosssql.Content{}}
}

// bossItem is an authored boss (boss.schema.json) of the test content, modified by edit.
func bossItem(id, code string, grades []int, mode string, edit func(m map[string]any)) command.ContentItem {
	act := map[string]any{"activity_type": "quiz", "activity_id": bossActivityG3}
	m := map[string]any{"schema_version": 1, "boss_id": id, "code": code, "owner_scope": "platform", "tenant_id": nil,
		"name": map[string]any{"vi": "Rồng Phân Số", "en": "Fraction Dragon"}, "area_id": "learning_zone.math", "grade_levels": grades,
		"subject_id": chlSubject, "mode": mode, "total_time_limit_seconds": 900, "max_phase_retries": 1, "reconnect_grace_seconds": 120,
		"phases": []any{
			map[string]any{"phase_id": "p1", "order": 1, "energy_share": 0.5, "time_limit_seconds": 120,
				"requirements": []any{map[string]any{"requirement_id": "r1", "kind": "activity", "activity_ref": act,
					"per_grade": []any{map[string]any{"grade_level": 4, "activity_ref": map[string]any{"activity_type": "quiz", "activity_id": bossActivityG4}}}}},
				"completion_rule": map[string]any{"type": "collective_count", "collective_target": 4, "min_contribution": 1}},
			map[string]any{"phase_id": "p2", "order": 2, "energy_share": 0.5, "time_limit_seconds": 120,
				"requirements": []any{map[string]any{"requirement_id": "r2", "kind": "activity", "activity_ref": act,
					"pass_rule": map[string]any{"min_correct_ratio": 0.3}}},
				"completion_rule": map[string]any{"type": "all"}},
		},
		"defeat_rewards":       []any{map[string]any{"type": "xp", "amount": 50}},
		"first_defeat_rewards": []any{map[string]any{"type": "xp", "amount": 30}}}
	if mode != "solo" {
		m["party_size"] = map[string]any{"min": 2, "max": 4}
	}
	if edit != nil {
		edit(m)
	}
	b, _ := json.Marshal(m)
	return command.ContentItem{Type: write.ItemBoss, Payload: b}
}

// publishBosses publishes the world seed, the bosses, and assigns them to tenants.
func (e *env) publishBosses(ctx context.Context, t *testing.T, items []command.ContentItem, tenants ...uuid.UUID) {
	t.Helper()
	if _, err := e.publish(ctx, t, items, "bosses"); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		var m struct {
			ID uuid.UUID `json:"boss_id"`
		}
		_ = json.Unmarshal(it.Payload, &m)
		for _, tenant := range tenants {
			if _, err := e.migrator.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at,
					updated_at, resource_type, resource_id, assigned_by, assigned_at)
				VALUES ($1, $2, now(), now(), 'boss', $3, $4, now()) ON CONFLICT DO NOTHING`, uuid.New(), tenant, m.ID, uuid.Nil); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// setupBosses is the environment with the world seed and the test bosses assigned to tenant A (and B for the
// isolation tests).
func setupBosses(ctx context.Context, t *testing.T) *env {
	t.Helper()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	e.publishBosses(ctx, t, []command.ContentItem{
		bossItem(bossTwoPhase, "math.boss.two_phase", []int{3, 4}, "party_optional", nil),
		bossItem(bossG4Only, "math.boss.g4_only", []int{4}, "solo", nil),
		bossItem(bossRequired, "math.boss.required", []int{3}, "party_required", nil),
	}, e.tenantA, e.tenantB)
	return e
}

// bossPush records the boss and co-op quest messages per member (TEST DOUBLE of port.BossRealtime/QuestMessages — the
// Valkey adapter is the one of pkg/realtime, covered by realtime_valkey_test.go; R17).
type bossPush struct {
	mu   sync.Mutex
	msgs []pushed
}

type pushed struct {
	student uuid.UUID
	typ     string
	payload map[string]any
}

func (p *bossPush) BossMessage(_ context.Context, student uuid.UUID, typ string, payload map[string]any, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	b, _ := json.Marshal(payload)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	p.msgs = append(p.msgs, pushed{student: student, typ: typ, payload: m})
	return nil
}

func (p *bossPush) QuestMessage(ctx context.Context, student uuid.UUID, typ string, payload map[string]any, at time.Time) error {
	return p.BossMessage(ctx, student, typ, payload, at)
}

// of returns the messages of type sent to student.
func (p *bossPush) of(student uuid.UUID, typ string) []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []map[string]any
	for _, m := range p.msgs {
		if m.student == student && m.typ == typ {
			out = append(out, m.payload)
		}
	}
	return out
}

// all returns every message as raw JSON text per student (privacy checks).
func (p *bossPush) raw(student uuid.UUID) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var sb strings.Builder
	for _, m := range p.msgs {
		if m.student == student {
			b, _ := json.Marshal(m.payload)
			sb.WriteString(m.typ + " " + string(b) + "\n")
		}
	}
	return sb.String()
}

// memOnline is the online state of the members (TEST DOUBLE of port.OnlineMembers — the SQL adapter on game sessions is
// covered by TestBossPresenceFromSessions; R17).
type memOnline struct {
	mu  sync.Mutex
	off map[uuid.UUID]bool
}

func (m *memOnline) Online(_ context.Context, _ uuid.UUID, students []uuid.UUID, _ time.Time) (map[uuid.UUID]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[uuid.UUID]bool{}
	for _, s := range students {
		out[s] = !m.off[s]
	}
	return out, nil
}

func (m *memOnline) set(s uuid.UUID, online bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.off[s] = !online
}

// bossLearning records the plans and scores asked to module learning (TEST DOUBLE through the generated Fake, R17): a
// plan of 3 questions without answer data; o_ok is correct, o_rapid correct but a rapid guess, anything else wrong.
type bossLearning struct {
	mu     sync.Mutex
	plans  []learningapi.QuestionPlanRequest
	scores []learningapi.PlanAnswerScoreRequest
	byKey  map[uuid.UUID]uuid.UUID
	fail   int // createQuestionPlan fails this many more times
}

func (e *env) bossFake() *bossLearning {
	if e.bossLearn != nil {
		return e.bossLearn
	}
	b := &bossLearning{byKey: map[uuid.UUID]uuid.UUID{}}
	e.bossLearn = b
	e.learning.f.CreateQuestionPlanFunc = func(ctx context.Context, req learningapi.CreateQuestionPlanRequestObject) (learningapi.CreateQuestionPlanResponseObject, error) {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.plans = append(b.plans, *req.Body)
		if b.fail > 0 {
			b.fail--
			return nil, errUnavailable
		}
		id, ok := b.byKey[req.Body.MatchId]
		if !ok {
			id = uuid.New()
			b.byKey[req.Body.MatchId] = id
		}
		out := learningapi.CreateQuestionPlan201JSONResponse{PlanId: id, MatchId: req.Body.MatchId, ExpiresAt: time.Now().Add(time.Hour)}
		for i := 0; i < 3; i++ {
			out.Items = append(out.Items, learningapi.PlanItem{Index: i, QuestionVersionId: uuid.New(), QuestionType: "single_choice",
				TimeLimitSeconds: 20, RenderPayload: map[string]interface{}{"stem": map[string]any{"text": "1/2 + 1/4 = ?"},
					"options": []any{map[string]any{"option_id": "o_ok"}, map[string]any{"option_id": "o_no"}}}})
		}
		return out, nil
	}
	e.learning.f.ScorePlanAnswerFunc = func(ctx context.Context, req learningapi.ScorePlanAnswerRequestObject) (learningapi.ScorePlanAnswerResponseObject, error) {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.scores = append(b.scores, *req.Body)
		raw, _ := req.Body.Response.MarshalJSON()
		ok, rapid := strings.Contains(string(raw), "o_ok") || strings.Contains(string(raw), "o_rapid"), strings.Contains(string(raw), "o_rapid")
		late := false
		return learningapi.ScorePlanAnswer200JSONResponse{PlanId: req.PlanId, Index: req.Body.Index, StudentId: req.Body.StudentId, IsCorrect: ok,
			Late: &late, RapidGuess: &rapid}, nil
	}
	e.learning.f.GetLearningMaterialRendersFunc = func(ctx context.Context, req learningapi.GetLearningMaterialRendersRequestObject) (learningapi.GetLearningMaterialRendersResponseObject, error) {
		var data []map[string]any
		for _, r := range req.Body.Refs {
			data = append(data, map[string]any{"id": r.Id, "version": 1, "title": map[string]any{"vi": "Thẻ", "en": "Card"},
				"blocks": []any{map[string]any{"kind": "rich_text", "id": "b1", "content": map[string]any{"text": "clue " + r.Id.String()}}}})
		}
		raw, _ := json.Marshal(map[string]any{"data": data})
		var out learningapi.GetLearningMaterialRenders200JSONResponse
		_ = json.Unmarshal(raw, &out)
		return out, nil
	}
	return b
}

func (b *bossLearning) planCalls() []learningapi.QuestionPlanRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]learningapi.QuestionPlanRequest(nil), b.plans...)
}

func (b *bossLearning) scoreCalls() []learningapi.PlanAnswerScoreRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]learningapi.PlanAnswerScoreRequest(nil), b.scores...)
}

// bossesUC returns the boss use cases on the test databases with the doubles of env.
func (e *env) bossesUC() command.Bosses {
	e.bossFake()
	return command.Bosses{UoW: e.uow, AfterCommit: entx.AfterCommit, Content: bosssql.Content{}, World: worldsql.Content{},
		Quests: questsql.Content{}, Store: bosssql.Store{}, Events: writerepo.Publisher{W: e.writer}, Audit: writerepo.Auditor{R: e.rec},
		Learning: modules.LearningBoss{F: e.learning.f}, Realtime: e.bossPush, Presence: e.online, Names: bosssql.Reads{},
		Metrics: observability.BossMetrics{}, Config: bossCfg(), Now: e.clock.Now}
}

func (e *env) bossQueries() query.Bosses {
	codec, _ := page.NewCodec([]byte(strings.Repeat("c", 32)))
	return query.Bosses{Read: e.read, Content: bosssql.Content{}, Reads: bosssql.Reads{}, World: worldsql.Content{}, Cursors: codec, Now: e.clock.Now}
}

// coopUC returns the co-op quest use cases.
func (e *env) coopUC() command.PartyQuests {
	e.bossFake()
	return command.PartyQuests{UoW: e.uow, AfterCommit: entx.AfterCommit, Content: questsql.Content{}, Party: questsql.Content{},
		World: worldsql.Content{}, Store: partyquestsql.Store{}, Quests: e.questsUC(), Events: writerepo.Publisher{W: e.writer},
		Audit: writerepo.Auditor{R: e.rec}, Messages: e.bossPush, Renders: e.clues(), Presence: e.online, Names: bosssql.Reads{},
		Metrics: observability.BossMetrics{}, Config: command.PartyQuestConfig{Grace: 120 * time.Second, CheckEvery: 5 * time.Second,
			HeartbeatTimeout: 45 * time.Second}, Now: e.clock.Now}
}

func (e *env) clues() cache.ClueCache {
	return cache.ClueCache{Source: modules.LearningBoss{F: e.learning.f}}
}

func (e *env) clueQueries() query.QuestClues {
	return query.QuestClues{Read: e.read, Reads: partyquestsql.Reads{}, Content: questsql.Content{}, Renders: e.clues(), Names: bosssql.Reads{}}
}

// partyFacade is the party-activity part of the game facade with the use cases of env.
func (e *env) partyFacade() facade.PartyActivities {
	c := bossCfg()
	return facade.PartyActivities{Queries: query.PartyActivities{Read: e.read, Bosses: bosssql.Content{}, BossReads: bosssql.Reads{},
		Party: questsql.Content{}, Quests: questsql.Content{}, World: worldsql.Content{}, AttemptsPerDay: c.AttemptsPerDay, Reset: c.Reset,
		Now: e.clock.Now}, Bosses: e.bossesUC(), Quests: e.coopUC()}
}

// competitionCtx is the context module competition calls the facade with (pkg/modcall: TenantScope of the party, the
// Principal of the leader, Idempotency-Key = party_activity_id when key ≠ "").
func competitionCtx(t *testing.T, leader student, key string) context.Context {
	t.Helper()
	ctx := ctxOf(t, nil, leader)
	if key != "" {
		ctx = modcall.WithIdempotencyKey(ctx, key)
	}
	return ctx
}

// proxies of the HTTP handlers.
type bossProxy struct{ e *env }

func (p bossProxy) CreateSolo(ctx context.Context, id uuid.UUID, mode string) (partyrules.EncounterView, error) {
	return p.e.bossesUC().CreateSolo(ctx, id, mode)
}
func (p bossProxy) Leave(ctx context.Context, id uuid.UUID) error {
	return p.e.bossesUC().Leave(ctx, id)
}

type bossQueryProxy struct{ e *env }

func (p bossQueryProxy) ListBosses(ctx context.Context, in query.BossListInput) (query.BossPage, error) {
	return p.e.bossQueries().ListBosses(ctx, in)
}
func (p bossQueryProxy) GetBossEncounter(ctx context.Context, id uuid.UUID) (partyrules.EncounterView, error) {
	return p.e.bossQueries().GetBossEncounter(ctx, id)
}

type coopProxy struct{ e *env }

func (p coopProxy) AssignRoles(ctx context.Context, q uuid.UUID, step string, as []partyquest.Assignment) (partyrules.RolesView, error) {
	return p.e.coopUC().AssignRoles(ctx, q, step, as)
}
func (p coopProxy) ShareClue(ctx context.Context, q uuid.UUID, step, role string) (partyrules.CluesView, error) {
	return p.e.coopUC().ShareClue(ctx, q, step, role)
}

type coopQueryProxy struct{ e *env }

func (p coopQueryProxy) GetClues(ctx context.Context, q uuid.UUID, step string) (partyrules.CluesView, error) {
	return p.e.clueQueries().GetClues(ctx, q, step)
}

// eventScope is the TenantScope of a consumer of tenant.
func eventScope(t *testing.T, tenant uuid.UUID) tenancy.Scope {
	t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// errUnavailable is a failure of module learning (deadline, 5xx) in the TEST DOUBLE.
var errUnavailable = errors.New("learning: deadline exceeded")
