package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// bossServerFields are set by the release core, never authored (boss.schema.json requires them stored).
var bossServerFields = []string{"boss_version_id", "version", "status"}

// BossPublishRules are the boss checks of the release core (F103 T009): the JSON Schema of boss.schema.json and the
// domain rules (energy shares, completion rules, requirements served, party size).
type BossPublishRules struct {
	Schema  port.PayloadValidator
	Content port.BossContent // published bosses (references of quests and live-ops events); nil = batch only
}

func bossPublishProblem(err error) error {
	var fe *boss.FieldError
	switch {
	case errors.Is(err, boss.ErrTranslationIncomplete):
		return problem.New(problem.TranslationIncomplete, err.Error())
	case errors.As(err, &fe):
		return problem.Invalid(fe.Field, problem.SubInvalid, fe.Reason)
	}
	return err
}

// parseBoss reads an authored boss: translations first (TRANSLATION_INCOMPLETE), then the JSON Schema of the stored
// form, then the domain checks (VALIDATION_FAILED with the field).
func parseBoss(it ContentItem, rules *BossPublishRules) (parsedItem, error) {
	p := parsedItem{ContentItem: it, parent: map[string]any{}, columns: map[string]any{}}
	if rules == nil || rules.Schema == nil {
		return p, problem.New(problem.Internal, "boss publishing is not configured")
	}
	if err := json.Unmarshal(it.Payload, &p.doc); err != nil || p.doc == nil {
		return p, problem.Invalid("payload", problem.SubInvalid, "a JSON object is required")
	}
	for _, f := range bossServerFields {
		delete(p.doc, f)
	}
	v, err := boss.Parse(it.Payload)
	if err != nil {
		return p, bossPublishProblem(err)
	}
	if m := v.MissingTranslations(); len(m) > 0 {
		return p, problem.New(problem.TranslationIncomplete, fmt.Sprintf("boss %s: %v", v.Code, m)).With("missing", m)
	}
	stored := map[string]any{}
	for k, val := range p.doc {
		stored[k] = val
	}
	stored["boss_version_id"], stored["version"], stored["status"] = placeholderVersionID, 1, "published"
	raw, err := json.Marshal(stored)
	if err != nil {
		return p, err
	}
	if err := rules.Schema.Validate(raw); err != nil {
		return p, problem.Invalid("payload", problem.SubInvalid, "boss "+v.Code+": "+err.Error())
	}
	if err := v.Validate(); err != nil {
		return p, bossPublishProblem(err)
	}
	p.boss, p.id = &v, v.BossID
	p.parent = map[string]any{"code": v.Code}
	p.columns = map[string]any{"code": v.Code, "mode": v.Mode, "grade_levels": v.GradeLevels, "subject_id": v.SubjectID, "area_id": v.AreaID,
		"phase_count": len(v.Phases)}
	return p, nil
}

// bossStored is the stored payload of a new boss version: the authored content plus the server fields.
func bossStored(doc map[string]any, vid uuid.UUID, versionNo int) ([]byte, error) {
	out := map[string]any{}
	for k, v := range doc {
		out[k] = v
	}
	out["boss_version_id"], out["version"], out["status"] = vid.String(), versionNo, "published"
	return json.Marshal(out)
}

// checkBosses checks the references of the bosses of a batch: the area exists (published or in the batch); a code used
// by another boss is refused.
func (h PublishGameContent) checkBosses(_ context.Context, items []parsedItem, areas map[string]bool) error {
	codes := map[string]string{}
	for _, p := range items {
		if p.boss == nil {
			continue
		}
		v := p.boss
		if other, ok := codes[v.Code]; ok && other != v.BossID {
			return problem.Invalid("code", problem.SubInvalid, fmt.Sprintf("boss code %s is used twice", v.Code))
		}
		codes[v.Code] = v.BossID
		if v.AreaID != "" && !areas[v.AreaID] {
			return problem.New(problem.ItemRefInvalid, fmt.Sprintf("boss %s: unknown area %s", v.Code, v.AreaID))
		}
	}
	return nil
}

// bossItem reports a boss item type.
func bossItem(t string) bool { return t == write.ItemBoss }
