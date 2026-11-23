package command

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/questionset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// minigameChecker checks a minigame learning activity against the projection and the question bank (F073-AC-15).
type minigameChecker struct {
	Minigames write.MinigameConfigs
	Docs      write.DocStore
	Questions write.QuestionStore
}

// check applies F073-AC-15 on draft save and before publish: the wrapped configuration (its version number, or a
// published version when the activity follows the current one) is in the projection ref.minigame_config_versions, and
// the question set override is published and returns only question types of the compatibility table of the configuration
// type, with enough questions for question_count. Any failure is 422 ACTIVITY_CONFIG_INVALID with the JSON pointer.
func (c minigameChecker) check(ctx context.Context, raw json.RawMessage) error {
	ac, err := minigame.ParseActivity(raw)
	if err != nil {
		return configProblem(contentmodel.Violations{{Field: "/config/content/minigame_config_id", Reason: "is required", Required: true}})
	}
	if c.Minigames == nil {
		return configProblem(contentmodel.Violations{{Field: "/config/content/minigame_config_id", Reason: "the mini-game projection is not available"}})
	}
	field := "/config/content/minigame_config_id"
	var row *dto.MinigameConfigVersion
	if ac.MinigameConfigVersion != nil {
		field = "/config/content/minigame_config_version"
		row, err = c.Minigames.MinigameConfigVersionNo(ctx, ac.MinigameConfigID, *ac.MinigameConfigVersion)
	} else {
		row, err = c.Minigames.CurrentMinigameConfigVersion(ctx, ac.MinigameConfigID)
	}
	if err != nil {
		return err
	}
	if row == nil {
		return configProblem(contentmodel.Violations{{Field: field, Reason: "is not a published mini-game configuration version"}})
	}
	rules, err := minigame.Parse(row.Snapshot)
	if err != nil {
		return configProblem(contentmodel.Violations{{Field: field, Reason: "the mini-game configuration cannot be played"}})
	}
	if ac.QuestionSetOverride == nil {
		return nil
	}
	s, err := c.setSource(ctx, *ac.QuestionSetOverride)
	if err != nil {
		return err
	}
	s.QuestionCount = rules.QuestionCount
	var vs contentmodel.Violations
	for _, r := range minigame.Check(rules.Type, s) {
		vs = append(vs, contentmodel.Violation{Field: "/config/content/question_set_override", Reason: r.Code + ": " + r.Message})
	}
	if len(vs) > 0 {
		return configProblem(vs)
	}
	return nil
}

// setSource is what a question set override can return: the types of its fixed items, its selection rules (unbounded
// without type filter) and its question count; SetNotFound when it is invisible or not published.
func (c minigameChecker) setSource(ctx context.Context, o minigame.SetOverride) (minigame.SourceCheck, error) {
	var out minigame.SourceCheck
	v, ok, err := c.setVersion(ctx, o.QuestionSetID, o.QuestionSetVersion)
	if err != nil || !ok {
		out.SetNotFound = !ok
		return out, err
	}
	var body struct {
		Definition questionset.Definition `json:"definition"`
	}
	_ = json.Unmarshal(v.Body, &body)
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(v.Items)+len(body.Definition.Items))
	for _, it := range v.Items {
		ids = append(ids, it.QuestionVersionID)
	}
	for _, it := range body.Definition.Items {
		ids = append(ids, it.QuestionVersionID)
	}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		qv, err := c.Questions.QuestionVersionByID(ctx, id)
		if err != nil {
			return out, err
		}
		out.Available++
		if !slices.Contains(out.Types, qv.Type) {
			out.Types = append(out.Types, qv.Type)
		}
	}
	for _, r := range body.Definition.SelectionRules {
		rule := r.Selection()
		out.Available += rule.Count
		if len(rule.Filters.Types) == 0 {
			out.Unbounded = true
		}
		for _, t := range rule.Filters.Types {
			if !slices.Contains(out.Types, t) {
				out.Types = append(out.Types, t)
			}
		}
	}
	return out, nil
}

// setVersion is the published version of a question set (the version number given, else the current release).
func (c minigameChecker) setVersion(ctx context.Context, setID uuid.UUID, no *int) (dto.DocVersion, bool, error) {
	d, err := c.Docs.Doc(ctx, contentmodel.TypeQuestionSet, setID)
	if err != nil {
		if pe, ok := problemOf(err); ok && (pe.Code == problem.NotFound || pe.Code == problem.ContentNotPublished) {
			return dto.DocVersion{}, false, nil
		}
		return dto.DocVersion{}, false, err
	}
	vs, err := c.Docs.DocVersions(ctx, contentmodel.TypeQuestionSet, setID)
	if err != nil {
		return dto.DocVersion{}, false, err
	}
	for _, v := range vs {
		switch {
		case no != nil && v.VersionNo == *no:
			return v, v.PublishedAt != nil, nil
		case no == nil && d.CurrentVersionID != nil && v.ID == *d.CurrentVersionID:
			return v, true, nil
		}
	}
	return dto.DocVersion{}, false, nil
}
