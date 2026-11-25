package quest

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// AreaRef is what the validator knows about a published area: its NPCs and its interactables by kind.
type AreaRef struct {
	NPCs          []string
	Interactables map[string]string // interactable_id → kind
}

// Catalog is the published world content (plus the batch being published) the references of a quest must exist in.
type Catalog struct {
	Areas    map[string]AreaRef
	Chapters map[string]bool
	Quests   map[string]bool
}

// ValidatorConfig holds the platform parameters of the publish checks.
type ValidatorConfig struct {
	PassRule       DefaultPassRule
	BlockMaxByBand map[string]int // learning_block_max_items: g1_3 3, g4_5 5, g6_9 8
	Registry       Registry
}

var (
	codeRe    = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9_]+)+$`)
	stepIDRe  = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
	uuidRe    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	subjectRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
)

func refInvalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrRefInvalid, fmt.Sprintf(format, a...))
}

// Validate is the QuestPublishValidator (F072-AC-27, LEARNING_EXPERIENCE §2.1): translations first (ErrTranslationIncomplete),
// then the structure (ErrInvalid: step ids, handlers, dependencies, completion rule, objectives, learning blocks),
// then the references against the catalog (ErrRefInvalid). The JSON Schema check runs before it (application).
func Validate(v Version, cfg ValidatorConfig, cat Catalog) error {
	if missing := v.MissingTranslations(); len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrTranslationIncomplete, strings.Join(missing, ", "))
	}
	if err := validateStructure(v, cfg); err != nil {
		return err
	}
	return validateRefs(v, cat)
}

func validateStructure(v Version, cfg ValidatorConfig) error {
	switch {
	case v.SchemaVersion != SchemaVersion:
		return invalid("schema_version must be %d", SchemaVersion)
	case !uuidRe.MatchString(v.QuestID):
		return invalid("quest_id")
	case !codeRe.MatchString(v.Code):
		return invalid("code %q", v.Code)
	case v.OwnerScope != "platform":
		return invalid("owner_scope must be platform")
	case !slices.Contains(Types, v.QuestType):
		return invalid("quest_type %q", v.QuestType)
	case v.SubjectCode != "" && !subjectRe.MatchString(v.SubjectCode):
		return invalid("subject_code %q", v.SubjectCode)
	case slices.Contains(objectiveTypes, v.QuestType) && v.SubjectCode == "":
		return invalid("quest_type %s needs subject_code (the school enabled subjects filter, F072)", v.QuestType)
	case len(v.GradeLevels) == 0:
		return invalid("grade_levels")
	case slices.Contains(objectiveTypes, v.QuestType) && len(v.ObjectiveIDs) == 0:
		return invalid("quest_type %s needs objective_ids", v.QuestType)
	case len(v.Steps) == 0:
		return invalid("steps")
	}
	for _, g := range v.GradeLevels {
		if g < 1 || g > 9 {
			return invalid("grade level %d", g)
		}
	}
	reg := cfg.Registry
	if reg == nil {
		reg = DefaultRegistry()
	}
	seen := map[string]bool{}
	for _, s := range v.Steps {
		if !stepIDRe.MatchString(s.ID) || seen[s.ID] {
			return invalid("step_id %q duplicated or invalid", s.ID)
		}
		seen[s.ID] = true
		h, ok := reg.Handler(s.Type)
		if !ok {
			return invalid("step %s: step type %s has no handler in this release", s.ID, s.Type)
		}
		if err := h.Validate(s); err != nil {
			return err
		}
	}
	for _, s := range v.Steps {
		for _, d := range v.Deps(s) {
			if !seen[d] || d == s.ID {
				return invalid("step %s: depends_on %s", s.ID, d)
			}
		}
	}
	if err := acyclic(v); err != nil {
		return err
	}
	if v.CompletionRule.Type == CompleteAnyN {
		if v.CompletionRule.N < 1 || v.CompletionRule.N > len(v.CompletionRule.StepIDs) {
			return invalid("completion_rule: 1 ≤ n ≤ len(step_ids)")
		}
		for _, id := range v.CompletionRule.StepIDs {
			if !seen[id] {
				return invalid("completion_rule: unknown step %s", id)
			}
		}
	} else if v.CompletionRule.Type != CompleteAllRequired {
		return invalid("completion_rule.type")
	}
	required := false
	for _, s := range v.Steps {
		required = required || !s.Optional
	}
	if v.CompletionRule.Type == CompleteAllRequired && !required {
		return invalid("completion_rule all_required_steps needs a required step")
	}
	for _, f := range v.FailureConditions {
		if f.StepID != "" && !seen[f.StepID] {
			return invalid("failure_conditions: unknown step %s", f.StepID)
		}
	}
	if err := validateParty(v, reg); err != nil {
		return err
	}
	return learningBlocks(v, cfg, reg)
}

// acyclic rejects a dependency cycle.
func acyclic(v Version) error {
	state := map[string]int{}
	var visit func(id string) bool
	visit = func(id string) bool {
		switch state[id] {
		case 1:
			return false
		case 2:
			return true
		}
		state[id] = 1
		s, _ := v.Step(id)
		for _, d := range v.Deps(s) {
			if !visit(d) {
				return false
			}
		}
		state[id] = 2
		return true
	}
	for _, s := range v.Steps {
		if !visit(s.ID) {
			return invalid("depends_on has a cycle through %s", s.ID)
		}
	}
	return nil
}

// learningBlocks checks learning_block_max_items (LEARNING_EXPERIENCE §2.1): the questions of consecutive learning
// steps (by order, without a game beat between them) never exceed the limit of the lowest grade band of the quest.
func learningBlocks(v Version, cfg ValidatorConfig, reg Registry) error {
	low := slices.Min(v.GradeLevels)
	limit, ok := cfg.BlockMaxByBand[GradeBand(low)]
	if !ok || limit <= 0 {
		return nil
	}
	block := 0
	for _, s := range v.SortedSteps() {
		h, _ := reg.Handler(s.Type)
		if h == nil || !h.Learning() {
			block = 0
			continue
		}
		block += cfg.PassRule.Effective(s.PassRule).MinQuestions
		if block > limit {
			return invalid("step %s: %d questions in a row exceed learning_block_max_items %d of %s", s.ID, block, limit, GradeBand(low))
		}
	}
	return nil
}

func validateRefs(v Version, cat Catalog) error {
	area := func(id, what string) (AreaRef, error) {
		a, ok := cat.Areas[id]
		if !ok {
			return a, refInvalid("%s: unknown area %s", what, id)
		}
		return a, nil
	}
	npcPlaced := func(npc, areaID, what string) error {
		if areaID != "" {
			a, err := area(areaID, what)
			if err != nil {
				return err
			}
			if !slices.Contains(a.NPCs, npc) {
				return refInvalid("%s: npc %s is not in area %s", what, npc, areaID)
			}
			return nil
		}
		for _, a := range cat.Areas {
			if slices.Contains(a.NPCs, npc) {
				return nil
			}
		}
		return refInvalid("%s: npc %s is placed in no area", what, npc)
	}
	if v.AreaID != "" {
		if _, err := area(v.AreaID, "area_id"); err != nil {
			return err
		}
	}
	if v.GiverNPCID != "" {
		if err := npcPlaced(v.GiverNPCID, "", "giver_npc_id"); err != nil {
			return err
		}
	}
	if v.StoryChapterCode != "" && !cat.Chapters[v.StoryChapterCode] {
		return refInvalid("story_chapter_code: unknown chapter %s", v.StoryChapterCode)
	}
	t := v.StartTrigger
	if t.NPCID != "" {
		if err := npcPlaced(t.NPCID, t.AreaID, "start_trigger"); err != nil {
			return err
		}
	} else if t.AreaID != "" {
		if _, err := area(t.AreaID, "start_trigger"); err != nil {
			return err
		}
	}
	for _, id := range t.InteractableIDs {
		found := false
		for _, a := range cat.Areas {
			k := a.Interactables[id]
			found = found || k == "discovery" || k == "collectible"
		}
		if !found {
			return refInvalid("start_trigger: %s is no discovery of a published area", id)
		}
	}
	if pr := v.Prerequisites; pr != nil {
		for _, q := range pr.CompletedQuestIDs {
			if !cat.Quests[q] {
				return refInvalid("prerequisites: unknown quest %s", q)
			}
		}
		for _, a := range pr.UnlockedAreaIDs {
			if _, err := area(a, "prerequisites"); err != nil {
				return err
			}
		}
	}
	for _, s := range v.Steps {
		what := "step " + s.ID
		switch s.Type {
		case StepTalkNPC:
			if err := npcPlaced(s.NPCID, s.AreaID, what); err != nil {
				return err
			}
		case StepGoTo, StepCollect, StepDiscover:
			a, err := area(s.AreaID, what)
			if err != nil {
				return err
			}
			for _, id := range s.Targets() {
				k, ok := a.Interactables[id]
				if !ok || (s.Type == StepDiscover && k != "discovery" && k != "collectible") {
					return refInvalid("%s: interactable %s is not in area %s", what, id, s.AreaID)
				}
			}
		case StepMinigame:
			if !uuidRe.MatchString(s.MiniGameConfigID) {
				return refInvalid("%s: minigame_config_id", what)
			}
		case StepActivity, StepInterdep:
			if !uuidRe.MatchString(s.ContentRef.ID) || (s.ContentRef.VersionID != "" && !uuidRe.MatchString(s.ContentRef.VersionID)) {
				return refInvalid("%s: content_ref", what)
			}
		}
	}
	return nil
}
