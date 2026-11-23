//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/progressionsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/progressionschema"
	rewardinfra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/seedfiles"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// Seed ids of F074 (src/backend/modules/game/seed/progression).
const (
	seedCurve        = "0193a5f4-6c1e-7b2a-9d1f-0000000f7400"
	ruleQuestMain    = "0193a5f4-6c1e-7b2a-9d1f-0000000f7401"
	ruleQuestSide    = "0193a5f4-6c1e-7b2a-9d1f-0000000f7402"
	ruleQuestDaily   = "0193a5f4-6c1e-7b2a-9d1f-0000000f7403"
	ruleActivity     = "0193a5f4-6c1e-7b2a-9d1f-0000000f7405"
	ruleObjective    = "0193a5f4-6c1e-7b2a-9d1f-0000000f7406"
	ruleLesson       = "0193a5f4-6c1e-7b2a-9d1f-0000000f7407"
	ruleDiscovery    = "0193a5f4-6c1e-7b2a-9d1f-0000000f7408"
	ruleAreaComplete = "0193a5f4-6c1e-7b2a-9d1f-0000000f7409"
)

// progressionRules are the publish checks of the F074 content (JSON Schemas, appliers xp/title, level table).
func progressionRules() *command.ProgressionPublishRules {
	return &command.ProgressionPublishRules{CurveSchema: progressionschema.Curve{}, RuleSchema: progressionschema.Rule{},
		Appliers: rewardinfra.Types(), Levels: writerepo.GameContentRepo{}, Curves: progressionsql.Content{}}
}

// defaultPolicy is the CONFIG_DEFAULTS policy of the progression.
func defaultPolicy() reward.Policy {
	return reward.Policy{DailyCap: 1000, Repeat: reward.Diminishing{Multipliers: []float64{1, 0.5, 0.25, 0}, WindowDays: 7},
		BelowGradeMultiplier: 0.25, BelowGradeGap: 2}
}

// progPush records progression pushes (TEST DOUBLE of port.ProgressionRealtime, R17; the Valkey adapter is the shared
// pkg/realtime publisher covered by realtime_valkey_test.go).
type progPush struct {
	mu   sync.Mutex
	msgs []map[string]any
}

func (p *progPush) add(typ string, student uuid.UUID, payload map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := map[string]any{"type": typ, "student": student.String()}
	for k, v := range payload {
		m[k] = v
	}
	p.msgs = append(p.msgs, m)
}

func (p *progPush) XPUpdated(_ context.Context, student uuid.UUID, payload map[string]any, _ time.Time) error {
	p.add("progression.xp.updated", student, payload)
	return nil
}

func (p *progPush) LevelReached(_ context.Context, student uuid.UUID, payload map[string]any, _ time.Time) error {
	p.add("progression.level.reached", student, payload)
	return nil
}

func (p *progPush) of(typ string) []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []map[string]any
	for _, m := range p.msgs {
		if m["type"] == typ {
			out = append(out, m)
		}
	}
	return out
}

var progPushes sync.Map // *env → *progPush

func (e *env) progPush() *progPush {
	v, _ := progPushes.LoadOrStore(e, &progPush{})
	return v.(*progPush)
}

// progressionUC returns the progression use cases on the test databases (policy p).
func (e *env) progressionWith(p reward.Policy) command.Progression {
	return command.Progression{UoW: e.uow, AfterCommit: entx.AfterCommit, Store: writerepo.ProgressionRepo{}, Scan: writerepo.ProgressionRepo{},
		Profiles: writerepo.ProfileRepo{}, Students: worldsql.Content{}, Rules: progressionsql.Content{}, Curves: progressionsql.Content{},
		Appliers: rewardinfra.Default(), Events: writerepo.Publisher{W: e.writer}, Audit: writerepo.Auditor{R: e.rec}, Realtime: e.progPush(),
		Quests: e.questsUC(), Metrics: observability.ProgressionMetrics{}, Policy: p, Now: e.clock.Now}
}

func (e *env) progressionUC() command.Progression { return e.progressionWith(defaultPolicy()) }

// progressionQueries returns the progression read use cases.
func (e *env) progressionQueries() query.Progression {
	codec, _ := page.NewCodec([]byte(strings.Repeat("c", 32)))
	return query.Progression{Read: e.read, Curves: progressionsql.Content{}, Data: read.Progression{}, Cursors: codec, DailyCap: 1000, Now: e.clock.Now}
}

type progressionQueryProxy struct{ e *env }

func (p progressionQueryProxy) GetProgression(ctx context.Context) (query.ProgressionStatus, error) {
	return p.e.progressionQueries().GetProgression(ctx)
}

func (p progressionQueryProxy) GetXPStatus(ctx context.Context) (query.XPStatus, error) {
	return p.e.progressionQueries().GetXPStatus(ctx)
}

func (p progressionQueryProxy) ListXPHistory(ctx context.Context, limit int, cursor, lang string) (query.XPHistoryPage, error) {
	return p.e.progressionQueries().ListXPHistory(ctx, limit, cursor, lang)
}

func (p progressionQueryProxy) ListLevels(ctx context.Context) (query.LevelsView, error) {
	return p.e.progressionQueries().ListLevels(ctx)
}

// progressionSeed returns the authored F074 seed (curve + reward rules).
func progressionSeed(t *testing.T) []command.ContentItem {
	t.Helper()
	items, err := seedfiles.Progression(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "game", "seed"))
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// setupProgression is the environment with the F074 seed published.
func setupProgression(ctx context.Context, t *testing.T) *env {
	t.Helper()
	e := setup(ctx, t, envOpts{})
	if _, err := e.publish(ctx, t, progressionSeed(t), "seed progression v1"); err != nil {
		t.Fatal(err)
	}
	return e
}

// progressionItem returns a copy of the seed item of type typ and id (curve_id / reward_rule_id) modified by edit.
func progressionItem(t *testing.T, id string, edit func(m map[string]any)) command.ContentItem {
	t.Helper()
	for _, it := range progressionSeed(t) {
		var m map[string]any
		_ = json.Unmarshal(it.Payload, &m)
		if m["curve_id"] != id && m["reward_rule_id"] != id {
			continue
		}
		if edit != nil {
			edit(m)
		}
		b, _ := json.Marshal(m)
		return command.ContentItem{Type: it.Type, Payload: b}
	}
	t.Fatalf("no seed item %s", id)
	return command.ContentItem{}
}

// smallCurve is the seed curve with xp_to_next(L) = 50 × L (levels 2 and 3 at 50 and 150 XP) and title rewards at levels
// 2 and 3.
func smallCurve(t *testing.T) command.ContentItem {
	return progressionItem(t, seedCurve, func(m map[string]any) {
		m["formula"] = map[string]any{"kind": "power_linear", "base": 50, "exponent": 1, "linear": 0}
		m["level_rewards"] = []any{
			map[string]any{"level": 2, "rewards": []any{map[string]any{"type": "title", "title_code": "rookie_star"}}},
			map[string]any{"level": 3, "rewards": []any{map[string]any{"type": "title", "title_code": "rising_star"}}},
		}
	})
}

// envelope builds a tenant event of producer for the Reward Engine (fresh event id, occurred now on the test clock).
func (e *env) envelope(typ, producer string, tenant uuid.UUID, payload map[string]any) outbox.Envelope {
	b, _ := json.Marshal(payload)
	tt := tenant
	return outbox.Envelope{EventID: uuid.Must(uuid.NewV7()), EventType: typ, SchemaVersion: 1, OccurredAt: e.clock.Now(), Producer: producer,
		TenantID: &tt, Payload: b}
}

// deliver hands env to the Reward Engine consumer in a transaction of the envelope tenant (as pkg/outbox.Consumer does,
// without the processed_events step: the replays below test the ledger keys themselves).
func (e *env) deliver(ctx context.Context, t *testing.T, uc command.Progression, env outbox.Envelope) error {
	t.Helper()
	c := consumer.RewardEngine{Awards: uc}
	return e.uow.InTenantTx(ctx, scopeOf(t, *env.TenantID), func(ctx context.Context) error { return c.Handle(ctx, env) })
}

func (e *env) mustDeliver(ctx context.Context, t *testing.T, uc command.Progression, env outbox.Envelope) {
	t.Helper()
	if err := e.deliver(ctx, t, uc, env); err != nil {
		t.Fatalf("%s: %v", env.EventType, err)
	}
}

// gradedEvent is activity.graded of a new attempt on content version cv.
func (e *env) gradedEvent(s student, cv uuid.UUID, firstTry, items int, extra map[string]any) outbox.Envelope {
	p := map[string]any{"attempt_id": uuid.New(), "student_id": s.id, "evaluation_id": uuid.New(), "activity_type": "multiple_choice",
		"mode": "practice", "source": "system", "score": firstTry, "max_score": items, "passed": true, "correct_count": firstTry,
		"item_count": items, "first_try_correct_count": firstTry, "result_release": "immediate", "objective_ids": []string{},
		"content_version_id": cv}
	for k, v := range extra {
		p[k] = v
	}
	return e.envelope("activity.graded", "learning", s.tenant, p)
}

// lessonEvent is lesson.completed of lesson version lv.
func (e *env) lessonEvent(s student, lv uuid.UUID, grades []int, mode string) outbox.Envelope {
	return e.envelope("lesson.completed", "learning", s.tenant, map[string]any{"lesson_session_id": uuid.New(), "lesson_id": uuid.New(),
		"lesson_version_id": lv, "lesson_version": 1, "student_id": s.id, "grade_levels": grades, "completion_rule": "all_required_activities",
		"mode": mode, "required_activity_count": 1, "completed_activity_count": 1, "attempt_ids": []string{uuid.NewString()},
		"objective_ids": []string{}, "started_at": e.clock.Now(), "completed_at": e.clock.Now(), "duration_ms": 60000})
}

// questEvent is quest.completed of a new quest progress.
func (e *env) questEvent(s student, questType string) outbox.Envelope {
	return e.envelope("quest.completed", "game", s.tenant, map[string]any{"quest_progress_id": uuid.New(), "quest_id": uuid.New(),
		"quest_version_id": uuid.New(), "quest_type": questType, "student_id": s.id, "objective_ids": []string{}, "completion_no": 1,
		"rewards": []any{}, "completed_at": e.clock.Now()})
}

// ledgerRow is one gamification.xp_ledger row.
type ledgerRow struct {
	amount     int
	sourceType string
	sourceCode string
	sourceID   uuid.UUID
	ruleID     uuid.NullUUID
	version    *int
	capped     bool
}

// ledger returns the ledger rows of s (oldest first; read as the migrator, RLS-free).
func (e *env) ledger(ctx context.Context, t *testing.T, s student) []ledgerRow {
	t.Helper()
	rows, err := e.platform.QueryContext(ctx, `SELECT amount, source_type, source_code, source_id, rule_id, rule_version, capped
		FROM gamification.xp_ledger WHERE student_id = $1 ORDER BY created_at, xp_entry_id`, s.id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []ledgerRow
	for rows.Next() {
		var r ledgerRow
		if err := rows.Scan(&r.amount, &r.sourceType, &r.sourceCode, &r.sourceID, &r.ruleID, &r.version, &r.capped); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// xpOf returns level and xp_total of the game profile of s.
func (e *env) xpOf(ctx context.Context, t *testing.T, s student) (int, int64) {
	t.Helper()
	var level int
	var xp int64
	if err := e.platform.QueryRowContext(ctx, `SELECT level, xp_total FROM game.game_profiles WHERE student_id = $1 AND deleted_at IS NULL`,
		s.id).Scan(&level, &xp); err != nil {
		t.Fatal(err)
	}
	return level, xp
}

func sumAmounts(rows []ledgerRow) int64 {
	var n int64
	for _, r := range rows {
		n += int64(r.amount)
	}
	return n
}

var _ = write.ItemRewardRule

// withoutKey removes key from a JSON object payload.
func withoutKey(t *testing.T, raw []byte, key string) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, key)
	b, _ := json.Marshal(m)
	return b
}

// withKey sets key in a JSON object payload.
func withKey(t *testing.T, raw []byte, key string, v any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m[key] = v
	b, _ := json.Marshal(m)
	return b
}
