package command

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// questServerFields are set by the release core, never authored (quest.schema.json requires them in the stored payload).
var questServerFields = []string{"quest_version_id", "version", "status"}

// placeholderVersionID stands for the server fields while the authored payload is checked against the JSON Schema.
const placeholderVersionID = "00000000-0000-0000-0000-000000000000"

// QuestPublishRules are the quest checks of the release core (F072 T044): the JSON Schema of quest.schema.json v2 and
// the parameters of the QuestPublishValidator.
type QuestPublishRules struct {
	Schema    port.PayloadValidator
	Validator quest.ValidatorConfig
}

// parseQuest reads an authored quest (F072-AC-27): translations first (TRANSLATION_INCOMPLETE), then the JSON Schema of
// the stored form (VALIDATION_FAILED); the references and the structure are checked with the batch in checkQuests.
func parseQuest(it ContentItem, rules *QuestPublishRules) (parsedItem, error) {
	p := parsedItem{ContentItem: it, parent: map[string]any{}, columns: map[string]any{}}
	if rules == nil || rules.Schema == nil {
		return p, problem.New(problem.Internal, "quest publishing is not configured")
	}
	if err := json.Unmarshal(it.Payload, &p.doc); err != nil || p.doc == nil {
		return p, problem.Invalid("payload", problem.SubInvalid, "a JSON object is required")
	}
	for _, f := range questServerFields {
		delete(p.doc, f)
	}
	v, err := quest.Parse(it.Payload)
	if err != nil {
		return p, questProblem(err)
	}
	if missing := v.MissingTranslations(); len(missing) > 0 {
		return p, problem.New(problem.TranslationIncomplete, fmt.Sprintf("quest %s: %v", v.Code, missing)).With("missing", missing)
	}
	stored := map[string]any{}
	for k, val := range p.doc {
		stored[k] = val
	}
	stored["quest_version_id"], stored["version"], stored["status"] = placeholderVersionID, 1, "published"
	raw, err := json.Marshal(stored)
	if err != nil {
		return p, err
	}
	if err := rules.Schema.Validate(raw); err != nil {
		return p, problem.Invalid("payload", problem.SubInvalid, "quest "+v.Code+": "+err.Error())
	}
	p.quest, p.id = &v, v.QuestID
	p.parent = map[string]any{"code": v.Code}
	from, to := v.Window()
	var subjectCode, area, giver, chapter any
	if v.SubjectCode != "" {
		subjectCode = v.SubjectCode
	}
	if v.AreaID != "" {
		area = v.AreaID
	}
	if v.GiverNPCID != "" {
		giver = v.GiverNPCID
	}
	if v.StoryChapterCode != "" {
		chapter = v.StoryChapterCode
	}
	objectives := v.ObjectiveIDs
	if objectives == nil {
		objectives = []string{}
	}
	p.columns = map[string]any{"code": v.Code, "quest_type": v.QuestType, "grade_levels": v.GradeLevels, "subject_id": v.SubjectID,
		"subject_code": subjectCode, "area_id": area, "giver_npc_id": giver, "story_chapter_code": chapter, "start_trigger": v.StartTrigger.Kind,
		"objective_ids": objectives, "available_from": from, "available_to": to}
	return p, nil
}

// questStored is the stored payload of a new quest version: the authored content plus the server fields.
func questStored(doc map[string]any, vid uuid.UUID, versionNo int) ([]byte, error) {
	out := map[string]any{}
	for k, v := range doc {
		out[k] = v
	}
	out["quest_version_id"], out["version"], out["status"] = vid.String(), versionNo, "published"
	return json.Marshal(out)
}

// checkQuests validates the quests of a batch against the published world content and quests plus the batch
// (F072-AC-27): QuestPublishValidator (structure, learning blocks, handlers) and the references; a code already used
// by another quest is refused.
func (h PublishGameContent) checkQuests(ctx context.Context, items []parsedItem, areas []port.AreaRow, chapters []port.ChapterRow) error {
	var qs []parsedItem
	for _, p := range items {
		if p.quest != nil {
			qs = append(qs, p)
		}
	}
	if len(qs) == 0 {
		return nil
	}
	if h.Quests == nil {
		return problem.New(problem.Internal, "quest publishing is not configured")
	}
	cat := quest.Catalog{Areas: map[string]quest.AreaRef{}, Chapters: map[string]bool{}, Quests: map[string]bool{}}
	addArea := func(a port.AreaRow) {
		ref := quest.AreaRef{NPCs: a.Area.NPCs(), Interactables: map[string]string{}}
		for _, it := range a.Area.Layout.Interactables {
			ref.Interactables[it.ID] = it.Kind
		}
		cat.Areas[a.Area.AreaID] = ref
	}
	for _, a := range areas {
		addArea(a)
	}
	for _, c := range chapters {
		cat.Chapters[c.Chapter.Code] = true
	}
	codes := map[string]string{}
	if h.QuestContent != nil {
		current, err := h.QuestContent.CurrentQuests(ctx)
		if err != nil {
			return err
		}
		for _, r := range current {
			cat.Quests[r.QuestID.String()] = true
			codes[r.Version.Code] = r.QuestID.String()
		}
	}
	for _, p := range items {
		switch {
		case p.area != nil:
			addArea(port.AreaRow{Area: *p.area})
		case p.chapter != nil:
			cat.Chapters[p.chapter.Code] = true
		case p.quest != nil:
			cat.Quests[p.quest.QuestID] = true
		}
	}
	for _, p := range qs {
		if other, ok := codes[p.quest.Code]; ok && other != p.quest.QuestID {
			return problem.Invalid("code", problem.SubInvalid, fmt.Sprintf("quest code %s is used by quest %s", p.quest.Code, other))
		}
		codes[p.quest.Code] = p.quest.QuestID
		if err := quest.Validate(*p.quest, h.Quests.Validator, cat); err != nil {
			return questProblem(fmt.Errorf("quest %s: %w", p.quest.Code, err))
		}
	}
	return nil
}

// questItem reports a quest item type.
func questItem(t string) bool { return t == write.ItemQuest }
