//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/minigameschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/minigamesql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// Test mini-game configs (F073): the matching example of the spec (grade 3, 3 questions, 5 plays a day) and a true/false.
const (
	mgMatching   = "01936b2e-7c4a-7d10-9a3e-5b1c2d3e5a01"
	mgTrueFalse  = "0193a5f4-6c1e-7b2a-9d1f-00000073a002"
	mgRewardRule = "01936b2e-7c4a-7d10-9a3e-5b1c2d3e7c02"
)

// minigameRules are the F073 checks of the release core with the learning facade double of env (R17).
func (e *env) minigameRules() *command.MiniGamePublishRules {
	return &command.MiniGamePublishRules{Schema: minigameschema.Validator{}, Sources: modules.LearningAttempts{F: e.learning.f}}
}

// miniGamesUC returns the mini-game use cases on the test databases with the doubles of env.
func (e *env) miniGamesUC() command.MiniGames {
	return command.MiniGames{UoW: e.uow, Content: minigamesql.Content{}, World: worldsql.Content{}, Plays: writerepo.MiniGamePlayRepo{},
		Events: writerepo.Publisher{W: e.writer}, Learning: modules.LearningAttempts{F: e.learning.f}, Metrics: observability.MiniGameMetrics{},
		LearningRetry: 2 * time.Second, Now: e.clock.Now}
}

type miniGameProxy struct{ e *env }

func (p miniGameProxy) Start(ctx context.Context, in command.StartMiniGameInput) (command.MiniGameSession, error) {
	return p.e.miniGamesUC().Start(ctx, in)
}

// minigameItem is the example config of the spec with id, code and edit applied.
func minigameItem(t *testing.T, id, code string, edit func(m map[string]any)) command.ContentItem {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "specs", "examples", "minigame.math-g3-matching.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["minigame_config_id"], m["code"] = id, code
	if edit != nil {
		edit(m)
	}
	b, _ := json.Marshal(m)
	return command.ContentItem{Type: write.ItemMiniGameConfig, Payload: b}
}

// trueFalseItem is a true/false config of grade 3 from a question set.
func trueFalseItem(t *testing.T) command.ContentItem {
	return minigameItem(t, mgTrueFalse, "math.g3.true_false_race", func(m map[string]any) {
		m["minigame_type"] = "true_false"
		m["question_source"] = map[string]any{"kind": "question_set", "question_set_id": "0193a5f4-6c1e-7b2a-9d1f-00000073a0f1"}
		m["question_count"] = 5
		delete(m, "limits")
	})
}

// assignMiniGame writes the active assignment of a config to tenant (as the assignment saga or the seed would).
func (e *env) assignMiniGame(ctx context.Context, t *testing.T, configID string, tenant uuid.UUID) {
	t.Helper()
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at, updated_at,
			resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, now(), now(), 'minigame_config', $3, $4, now()) ON CONFLICT DO NOTHING`, uuid.New(), tenant, configID, uuid.Nil); err != nil {
		t.Fatal(err)
	}
}

// setupMiniGames is the environment with the world seed and the two test configs published, assigned to tenant A.
func setupMiniGames(ctx context.Context, t *testing.T) *env {
	t.Helper()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	if _, err := e.publish(ctx, t, []command.ContentItem{minigameItem(t, mgMatching, "math.g3.multiplication_card_match", nil), trueFalseItem(t)},
		"minigames"); err != nil {
		t.Fatal(err)
	}
	e.assignMiniGame(ctx, t, mgMatching, e.tenantA)
	e.assignMiniGame(ctx, t, mgTrueFalse, e.tenantA)
	return e
}

// startMiniGame calls startMiniGame.
func (e *env) startMiniGame(t *testing.T, s student, tok, config, k string, body any) resp {
	t.Helper()
	if k == "" {
		k = newKey()
	}
	return e.do(t, http.MethodPost, "/game/minigames/"+config+":start", tok, merge(th(s), map[string]string{"Idempotency-Key": k}), body)
}

// currentVersion is the current published version of a config.
func (e *env) currentVersion(ctx context.Context, t *testing.T, config string) string {
	t.Helper()
	var v string
	if err := e.migrator.QueryRowContext(ctx, `SELECT current_version_id::text FROM game.minigame_configs WHERE minigame_config_id = $1`, config).
		Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// minigameGraded delivers activity.graded of a minigame attempt to the consumer game.minigame_on_activity_graded.
func (e *env) minigameGraded(ctx context.Context, t *testing.T, s student, attempt, config, version, ctxType string, ctxID uuid.UUID, correct,
	items, gameScore int, met bool, eventID uuid.UUID) error {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"attempt_id": attempt, "student_id": s.id, "mode": "minigame", "score": correct, "max_score": items,
		"correct_count": correct, "item_count": items, "minigame": map[string]any{"minigame_config_id": config, "minigame_config_version_id": version,
			"game_score": gameScore, "completion_met": met, "context_type": ctxType, "context_id": ctxID}})
	tt := s.tenant
	env := outbox.Envelope{EventID: eventID, EventType: "activity.graded", SchemaVersion: 1, OccurredAt: e.clock.Now(), Producer: "learning",
		TenantID: &tt, Payload: b}
	c := consumer.MiniGameOnActivityGraded{MiniGames: e.miniGamesUC()}
	return e.uow.InTenantTx(ctx, scopeOf(t, s.tenant), func(ctx context.Context) error { return c.Handle(ctx, env) })
}
