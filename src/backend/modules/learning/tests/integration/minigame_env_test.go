//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// hsGiang is a grade 7 student of tenant A (speed bonus and wrong penalty of the mini-games, F073-AC-09).
const hsGiang = "hs.giang.7a"

// mgConfig is a published MiniGameConfigVersion of module game (minigame-config.schema.json) as the projection receives it.
type mgConfig struct {
	configID, versionID uuid.UUID
	version             int
	typ                 string
	body                map[string]any
}

// mgSnapshot builds a configuration of type typ over source (question_source) with count questions: 100 points per
// correct answer, a speed bonus of 50 within 20 s, a streak bonus of 30 every 3 correct answers, a wrong penalty of 10,
// 180 s, immediate feedback, completion when 70 % of the items are correct (the example of
// specs/examples/minigame.math-g3-matching.json); edit changes it.
func mgSnapshot(configID uuid.UUID, version int, typ string, source map[string]any, count int, edit func(m map[string]any)) mgConfig {
	c := mgConfig{configID: configID, versionID: uuid.New(), version: version, typ: typ}
	c.body = map[string]any{"schema_version": 1, "minigame_config_id": configID, "minigame_config_version_id": c.versionID,
		"code": "math.g3.test_minigame", "version": version, "status": "published", "owner_scope": "platform", "tenant_id": nil, "minigame_type": typ,
		"name": map[string]string{"vi": "Hái quả", "en": "Fruit picking"}, "instructions": map[string]string{"vi": "Chọn quả đúng", "en": "Pick the right fruit"},
		"grade_levels": []int{3, 7}, "subject_id": uuid.New(), "question_source": source, "difficulty": 2, "time_limit_seconds": 180,
		"per_item_time_seconds": nil, "question_count": count,
		"scoring": map[string]any{"points_per_correct": 100, "wrong_penalty": 10,
			"time_bonus":   map[string]any{"enabled": true, "max_bonus": 50, "full_bonus_within_ms": 20000},
			"streak_bonus": map[string]any{"enabled": true, "every_n_correct": 3, "bonus": 30}},
		"result_calculation": map[string]any{"learning_score": "sum_question_scores", "game_score": "points_plus_bonus"},
		"feedback_policy":    map[string]any{"timing": "immediate", "show_explanation": true, "show_correct_answer": true},
		"completion_rule":    map[string]any{"type": "min_correct_ratio", "threshold": 0.7},
		"limits":             map[string]any{"max_plays_per_day": 5, "retry_policy": "new_questions", "resumable": true},
		"reward_rule_id":     uuid.New(),
		"presentation":       map[string]any{"scene_variant": "multiple_choice.fruit_tree", "theme": "garden", "mascot_npc_id": "npc.ong_so"}}
	if edit != nil {
		edit(c.body)
	}
	return c
}

// selectionSource is a selection rule over the questions of subject of the given types.
func selectionSource(subject string, types ...string) map[string]any {
	return map[string]any{"kind": "selection_rule", "rule": map[string]any{"subject_id": subject, "question_types": types}}
}

// contentPublished is the payload of a content.published of module game for c (item_type minigame_config).
func (c mgConfig) contentPublished(at time.Time) map[string]any {
	return map[string]any{"item_type": "minigame_config", "item_id": c.configID.String(), "from_version_id": nil, "to_version_id": c.versionID,
		"to_version": c.version, "release_id": uuid.New(), "owner_scope": "platform", "tenant_id": nil, "scheduled": false,
		"published_by": uuid.New(), "released_at": at.UTC(),
		"minigame_config": map[string]any{"minigame_config_id": c.configID, "version_id": c.versionID, "version": c.version,
			"minigame_type": c.typ, "grade_levels": []int{3, 7}, "snapshot": c.body}}
}

// project stores the version in ref.minigame_config_versions through the command of the consumer (PlatformScope of the
// consumer learning.minigame_config_projection, role learning_platform).
func (e *env) project(ctx context.Context, t *testing.T, c mgConfig) bool {
	t.Helper()
	snap, _ := json.Marshal(c.body)
	ps, err := authz.SystemPlatformScope("consumer-learning-minigame-config-projection")
	if err != nil {
		t.Fatal(err)
	}
	var applied bool
	if err := e.uow.InTenantTx(tenancy.WithModule(ctx, "learning"), ps, func(ctx context.Context) error {
		var err error
		applied, err = command.UpsertMiniGameConfigSnapshot{Store: write.RefRepo{}}.Upsert(ctx, command.MiniGameConfigSnapshot{ConfigID: c.configID,
			VersionID: c.versionID, Version: c.version, Type: c.typ, GradeLevels: []int{3, 7}, Snapshot: snap, PublishedAt: time.Now().UTC(),
			SourceVersion: time.Now().UnixMicro()})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return applied
}

// mgFacade is the learning facade of the mini-game operations (createAttempt, validateMinigameQuestionSource).
func (c *curEnv) mgFacade() facade.Facade {
	return facade.Facade{AttemptOpener: c.att, AttemptResults: c.attq, Idempotency: idempotency.NewStore(c.app, 0, nil),
		MinigameSources: query.ValidateMinigameQuestionSource{Session: c.read, Docs: read.DocModel{}, Sources: read.MinigameSourceModel{}}}
}

// mgStart opens a mini-game play of sub as module game does (createAttempt: content_type minigame_config, the pinned
// version, mode minigame / context minigame_session or mode quest / context quest_step).
func (c *curEnv) mgStart(ctx context.Context, sub string, cfg mgConfig, mode learningapi.AttemptMode) (learningapi.CreateAttempt201JSONResponse, error) {
	p := authz.NewPrincipal(c.users[sub])
	v := cfg.versionID
	b := learningapi.CreateAttemptJSONRequestBody{StudentId: p.UserID, Mode: mode,
		Activity: learningapi.ActivityRef{ContentType: learningapi.ActivityRefContentTypeMinigameConfig, ContentId: cfg.configID, ContentVersionId: &v}}
	b.Context.ContextType, b.Context.ContextId = "minigame_session", uuid.New()
	if mode == learningapi.AttemptModeQuest {
		b.Context.ContextType = "quest_step"
	}
	f := c.mgFacade()
	res, err := asModule(authztest.Ctx(ctx, p), "game", "createAttempt", newKey(), func(ctx context.Context) (learningapi.CreateAttemptResponseObject, error) {
		return f.CreateAttempt(ctx, learningapi.CreateAttemptRequestObject{Body: &b})
	})
	if err != nil {
		return learningapi.CreateAttempt201JSONResponse{}, err
	}
	return res.(learningapi.CreateAttempt201JSONResponse), nil
}

// getAttempt reads an attempt as its student.
func (c *curEnv) getAttempt(t *testing.T, sub string, id uuid.UUID) resp {
	t.Helper()
	return must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+id.String(), sub, nil, nil), http.StatusOK, "get attempt")
}

// mgNum reads a number at path of a JSON answer (NaN-free: -1 when absent).
func mgNum(r resp, path ...string) float64 {
	var cur any = r.body
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return -1
		}
		cur = m[p]
	}
	if f, ok := cur.(float64); ok {
		return f
	}
	return -1
}
