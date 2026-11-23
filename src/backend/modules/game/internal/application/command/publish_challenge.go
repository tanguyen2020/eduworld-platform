package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// challengeServerFields are set by the release core, never authored (challenge.schema.json requires them stored).
var challengeServerFields = []string{"challenge_version_id", "version", "status"}

// ChallengePublishRules are the challenge checks of the release core (F076 T036, F076-AC-20): the JSON Schema of
// challenge.schema.json and the platform parameters the profiles are checked against (g1_3_challenge_rules).
type ChallengePublishRules struct {
	Schema  port.PayloadValidator
	Rules   challenge.Rules
	Content port.ChallengeContent // published challenges (references of live-ops events); nil = batch only
}

func challengeProblem(err error) error {
	var fe *challenge.FieldError
	switch {
	case errors.Is(err, challenge.ErrTranslationIncomplete):
		return problem.New(problem.TranslationIncomplete, err.Error())
	case errors.As(err, &fe):
		return problem.Invalid(fe.Field, problem.SubInvalid, fe.Reason)
	}
	return err
}

// parseChallenge reads an authored challenge (F076-AC-20): translations first (TRANSLATION_INCOMPLETE), then the JSON
// Schema of the stored form, then the domain checks (VALIDATION_FAILED with the field).
func parseChallenge(it ContentItem, rules *ChallengePublishRules) (parsedItem, error) {
	p := parsedItem{ContentItem: it, parent: map[string]any{}, columns: map[string]any{}}
	if rules == nil || rules.Schema == nil {
		return p, problem.New(problem.Internal, "challenge publishing is not configured")
	}
	if err := json.Unmarshal(it.Payload, &p.doc); err != nil || p.doc == nil {
		return p, problem.Invalid("payload", problem.SubInvalid, "a JSON object is required")
	}
	for _, f := range challengeServerFields {
		delete(p.doc, f)
	}
	v, err := challenge.Parse(it.Payload)
	if err != nil {
		return p, challengeProblem(err)
	}
	if m := v.MissingTranslations(); len(m) > 0 {
		return p, problem.New(problem.TranslationIncomplete, fmt.Sprintf("challenge %s: %v", v.Code, m)).With("missing", m)
	}
	stored := map[string]any{}
	for k, val := range p.doc {
		stored[k] = val
	}
	stored["challenge_version_id"], stored["version"], stored["status"] = placeholderVersionID, 1, "published"
	raw, err := json.Marshal(stored)
	if err != nil {
		return p, err
	}
	if err := rules.Schema.Validate(raw); err != nil {
		return p, problem.Invalid("payload", problem.SubInvalid, "challenge "+v.Code+": "+err.Error())
	}
	if err := v.Validate(rules.Rules); err != nil {
		return p, challengeProblem(err)
	}
	p.chal, p.id = &v, v.ChallengeID
	p.parent = map[string]any{"code": v.Code}
	from, to := v.Window()
	var area any
	if v.AreaID != "" {
		area = v.AreaID
	}
	p.columns = map[string]any{"code": v.Code, "challenge_type": v.Type, "mode": v.Mode, "grade_levels": v.GradeLevels,
		"subject_id": v.SubjectID, "objective_ids": v.ObjectiveIDs, "area_id": area, "available_from": from, "available_to": to}
	return p, nil
}

// challengeStored is the stored payload of a new challenge version: the authored content plus the server fields.
func challengeStored(doc map[string]any, vid uuid.UUID, versionNo int) ([]byte, error) {
	out := map[string]any{}
	for k, v := range doc {
		out[k] = v
	}
	out["challenge_version_id"], out["version"], out["status"] = vid.String(), versionNo, "published"
	return json.Marshal(out)
}

// checkChallenges checks the references of the challenges of a batch (F076-AC-20): the quests and areas of their unlock
// rule exist (published or in the batch); a code already used by another challenge is refused.
func (h PublishGameContent) checkChallenges(ctx context.Context, items []parsedItem, areas, quests map[string]bool) error {
	codes := map[string]string{}
	for _, p := range items {
		if p.chal == nil {
			continue
		}
		v := p.chal
		if other, ok := codes[v.Code]; ok && other != v.ChallengeID {
			return problem.Invalid("code", problem.SubInvalid, fmt.Sprintf("challenge code %s is used twice", v.Code))
		}
		codes[v.Code] = v.ChallengeID
		if u := v.Unlock; u != nil {
			for i, q := range u.CompletedQuestIDs {
				if !quests[q] {
					return problem.New(problem.ItemRefInvalid, fmt.Sprintf("unlock_rule.completed_quest_ids/%d is an unknown quest", i))
				}
			}
			for i, a := range u.UnlockedAreaIDs {
				if !areas[a] {
					return problem.New(problem.ItemRefInvalid, fmt.Sprintf("unlock_rule.unlocked_area_ids/%d is an unknown area", i))
				}
			}
		}
		if v.AreaID != "" && !areas[v.AreaID] {
			return problem.New(problem.ItemRefInvalid, fmt.Sprintf("challenge %s: unknown area %s", v.Code, v.AreaID))
		}
	}
	return nil
}

// knownChallenges reports which challenges of ids are published or in the batch (live-ops event components, F077
// waited for F076: an unknown challenge is 422 ITEM_REF_INVALID).
func (h PublishGameContent) knownChallenges(ctx context.Context, items []parsedItem, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	known := map[uuid.UUID]bool{}
	if h.Challenges != nil && h.Challenges.Content != nil {
		var err error
		if known, err = h.Challenges.Content.KnownChallenges(ctx, ids); err != nil {
			return nil, err
		}
	}
	for _, p := range items {
		if p.chal != nil {
			if id, err := uuid.Parse(p.chal.ChallengeID); err == nil {
				known[id] = true
			}
		}
	}
	return known, nil
}

// challengeItem reports a challenge item type.
func challengeItem(t string) bool { return t == write.ItemChallenge }
