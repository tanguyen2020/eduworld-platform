package command

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MiniGamePublishRules are the checks of a mini-game config on the release core (F073 T014/T015, F073-AC-01…AC-05): the
// bilingual rule, the JSON Schema of minigame-config.schema.json, the domain rules, then the question source through
// validateMinigameQuestionSource of module learning (compatible question types, enough questions, question set
// published). The config never holds a question (AC-03: the schema has no such field).
type MiniGamePublishRules struct {
	Schema  port.PayloadValidator
	Sources port.MiniGameSources
}

// parseMiniGame reads an authored mini-game config: TRANSLATION_INCOMPLETE first, then VALIDATION_FAILED (schema,
// difficulty), then ACTIVITY_CONFIG_INVALID with the reasons of module learning. A facade that cannot answer is 503.
func parseMiniGame(ctx context.Context, it ContentItem, rules *MiniGamePublishRules) (parsedItem, error) {
	p := parsedItem{ContentItem: it, parent: map[string]any{}, columns: map[string]any{}}
	if rules == nil || rules.Schema == nil || rules.Sources == nil {
		return p, problem.New(problem.Internal, "mini-game publishing is not configured")
	}
	if err := json.Unmarshal(it.Payload, &p.doc); err != nil || p.doc == nil {
		return p, problem.Invalid("payload", problem.SubRequired, "a JSON object is required")
	}
	for _, f := range []string{"minigame_config_version_id", "version", "status"} {
		delete(p.doc, f)
	}
	stored := map[string]any{}
	for k, v := range p.doc {
		stored[k] = v
	}
	stored["minigame_config_version_id"], stored["version"], stored["status"] = placeholderVersionID, 1, "published"
	raw, err := json.Marshal(stored)
	if err != nil {
		return p, err
	}
	c, err := minigame.Parse(raw)
	if err != nil {
		return p, minigameProblem(err)
	}
	if err := c.CheckTranslations(); err != nil {
		return p, minigameProblem(err)
	}
	if err := rules.Schema.Validate(raw); err != nil {
		return p, schemaProblem(err)
	}
	if err := c.Validate(); err != nil {
		return p, minigameProblem(err)
	}
	check, err := rules.Sources.ValidateSource(ctx, c)
	if errors.Is(err, port.ErrLearningUnavailable) {
		return p, problem.New(problem.Unavailable, "module learning cannot check the question source now").Wrap(err).Retry()
	}
	if err != nil {
		return p, err
	}
	if err := c.CheckSource(check); err != nil {
		return p, minigameProblem(err)
	}
	types := check.QuestionTypes
	if types == nil {
		types = []string{}
	}
	p.minigame, p.id = &c, c.ConfigID.String()
	p.parent = map[string]any{"code": c.Code}
	p.columns = map[string]any{"code": c.Code, "minigame_type": c.Type, "grade_levels": c.GradeLevels, "subject_id": c.SubjectID,
		"question_types": types}
	return p, nil
}

// minigameProblem maps the domain errors of a config to their problem codes.
func minigameProblem(err error) error {
	var fe *minigame.FieldError
	switch {
	case errors.Is(err, minigame.ErrTranslationIncomplete):
		return problem.New(problem.TranslationIncomplete, err.Error())
	case errors.Is(err, minigame.ErrSourceInvalid):
		return problem.New(problem.ActivityConfigInvalid, truncateMsg(err.Error(), 1000))
	case errors.As(err, &fe):
		return problem.Invalid(fe.Field, problem.SubInvalid, fe.Reason)
	}
	return err
}

func truncateMsg(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// minigameStored is the stored payload of a config version (server version id, number and status).
func minigameStored(p parsedItem, vid uuid.UUID, version int) ([]byte, error) {
	doc := map[string]any{}
	for k, v := range p.doc {
		doc[k] = v
	}
	doc["minigame_config_version_id"], doc["version"], doc["status"] = vid.String(), version, "published"
	return json.Marshal(doc)
}

// minigameBlock is the minigame_config block of content.published (the immutable snapshot for the projection of module
// learning, F073-AC-04).
func minigameBlock(p parsedItem, vid uuid.UUID, version int, stored []byte) (map[string]any, error) {
	var snapshot map[string]any
	if err := json.Unmarshal(stored, &snapshot); err != nil {
		return nil, err
	}
	return map[string]any{"minigame_config_id": p.minigame.ConfigID, "version_id": vid, "version": version, "minigame_type": p.minigame.Type,
		"grade_levels": p.minigame.GradeLevels, "snapshot": snapshot}, nil
}

func minigameItem(t string) bool { return t == write.ItemMiniGameConfig }
