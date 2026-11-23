package query

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/questionset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MinigameSourceInput is MinigameQuestionSourceCheck of validateMinigameQuestionSource.
type MinigameSourceInput struct {
	MinigameType  string
	Source        minigame.Source
	QuestionCount int
}

// ValidateMinigameQuestionSourceHandler is the typed interface of the query (ADR-0087).
type ValidateMinigameQuestionSourceHandler interface {
	Validate(ctx context.Context, in MinigameSourceInput) (dto.MinigameSourceResult, error)
}

// ValidateMinigameQuestionSource is the facade query validateMinigameQuestionSource (x-allowed-callers: game, F073-AC-02,
// TC-ACT-003): module game asks it only when it publishes a mini-game configuration. It answers which question types the
// source may return, how many questions are available and why the source is refused (compatibility table of
// MINIGAME_ENGINE §2, unbounded types, too few questions, question set not published) — never a question, an answer or
// a scoring rule (R6). It reads in the scope of the call (the PlatformScope of the publish: platform content only).
type ValidateMinigameQuestionSource struct {
	Session read.Session
	Docs    read.DocReadModel
	Sources read.MinigameSourceReadModel
}

var _ ValidateMinigameQuestionSourceHandler = ValidateMinigameQuestionSource{}

// Validate implements ValidateMinigameQuestionSourceHandler.
func (q ValidateMinigameQuestionSource) Validate(ctx context.Context, in MinigameSourceInput) (out dto.MinigameSourceResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.ValidateMinigameQuestionSource")
	defer func() { done(err) }()
	if _, err := principal(ctx); err != nil {
		return out, err
	}
	if !minigame.Valid(in.MinigameType) {
		return out, problem.Invalid("/minigame_type", problem.SubInvalid, "is not a mini-game type")
	}
	if in.QuestionCount < 1 || in.QuestionCount > 50 {
		return out, problem.Invalid("/question_count", problem.SubInvalid, "must be between 1 and 50")
	}
	check := minigame.SourceCheck{QuestionCount: in.QuestionCount}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		switch in.Source.Kind {
		case minigame.SourceQuestionSet:
			if in.Source.QuestionSetID == nil {
				return problem.Invalid("/question_source/question_set_id", problem.SubRequired, "is required for a question set")
			}
			return q.questionSet(ctx, *in.Source.QuestionSetID, in.Source.Version, &check)
		case minigame.SourceSelectionRule:
			if in.Source.Rule == nil {
				return problem.Invalid("/question_source/rule", problem.SubRequired, "is required for a selection rule")
			}
			return q.selection(ctx, in.MinigameType, *in.Source.Rule, &check)
		}
		return problem.Invalid("/question_source/kind", problem.SubInvalid, "is not question_set or selection_rule")
	})
	if err != nil {
		return out, err
	}
	out = dto.MinigameSourceResult{QuestionTypes: minigame.SortTypes(check.Types), AvailableCount: check.Available, Reasons: []dto.MinigameSourceReason{}}
	for _, r := range minigame.Check(in.MinigameType, check) {
		out.Reasons = append(out.Reasons, dto.MinigameSourceReason{Code: r.Code, Message: r.Message})
	}
	out.Compatible = len(out.Reasons) == 0
	return out, nil
}

// publishedSetVersion returns the version of a question set a source plays: the version number given (once published)
// or the current release; found = false when the set is invisible or not published.
func publishedSetVersion(ctx context.Context, docs read.DocReadModel, setID uuid.UUID, no *int) (dto.DocVersion, bool, error) {
	d, err := docs.Doc(ctx, contentmodel.TypeQuestionSet, setID)
	if isNotFound(err) {
		return dto.DocVersion{}, false, nil
	}
	if err != nil {
		return dto.DocVersion{}, false, err
	}
	if no != nil {
		v, err := docs.Version(ctx, contentmodel.TypeQuestionSet, setID, *no)
		if isNotFound(err) {
			return v, false, nil
		}
		return v, err == nil && v.PublishedAt != nil, err
	}
	if d.CurrentVersionID == nil {
		return dto.DocVersion{}, false, nil
	}
	for n := d.LatestVersionNo; n >= 1; n-- {
		v, err := docs.Version(ctx, contentmodel.TypeQuestionSet, setID, n)
		if isNotFound(err) {
			continue
		}
		if err != nil {
			return v, false, err
		}
		if v.ID == *d.CurrentVersionID {
			return v, true, nil
		}
	}
	return dto.DocVersion{}, false, nil
}

func isNotFound(err error) bool {
	var pe *problem.Error
	return errors.As(err, &pe) && (pe.Code == problem.NotFound || pe.Code == problem.ContentNotPublished)
}

// questionSet adds the fixed items of the set version (types of their published question versions) and its selection
// rules (count; a rule without type filter is unbounded).
func (q ValidateMinigameQuestionSource) questionSet(ctx context.Context, setID uuid.UUID, no *int, check *minigame.SourceCheck) error {
	v, found, err := publishedSetVersion(ctx, q.Docs, setID, no)
	if err != nil || !found {
		check.SetNotFound = !found
		return err
	}
	var body struct {
		Definition questionset.Definition `json:"definition"`
	}
	_ = json.Unmarshal(v.Body, &body)
	ids := make([]uuid.UUID, 0, len(v.Items))
	for _, it := range v.Items {
		ids = append(ids, it.QuestionVersionID)
	}
	for _, it := range body.Definition.Items {
		if !slices.Contains(ids, it.QuestionVersionID) {
			ids = append(ids, it.QuestionVersionID)
		}
	}
	if len(ids) > 0 {
		types, err := q.Sources.QuestionTypes(ctx, ids)
		if err != nil {
			return err
		}
		for _, t := range types {
			check.Available++
			if !slices.Contains(check.Types, t) {
				check.Types = append(check.Types, t)
			}
		}
	}
	for _, r := range body.Definition.SelectionRules {
		rule := r.Selection()
		check.Available += rule.Count
		if len(rule.Filters.Types) == 0 {
			check.Unbounded = true
		}
		for _, t := range rule.Filters.Types {
			if !slices.Contains(check.Types, t) {
				check.Types = append(check.Types, t)
			}
		}
	}
	return nil
}

// selection counts the questions of the published pool matching the rule, narrowed to the question types the mini-game
// accepts (as a play draws them); a rule without type filter is unbounded.
func (q ValidateMinigameQuestionSource) selection(ctx context.Context, minigameType string, rule minigame.Rule, check *minigame.SourceCheck) error {
	check.Types = slices.Clone(rule.QuestionTypes)
	check.Unbounded = len(rule.QuestionTypes) == 0
	types := minigame.Allowed(minigameType)
	if !check.Unbounded {
		types = slices.DeleteFunc(slices.Clone(rule.QuestionTypes), func(t string) bool { return !minigame.Accepts(minigameType, t) })
	}
	pool, err := q.Sources.Pool(ctx, read.PoolFilter{SubjectID: rule.SubjectID})
	if err != nil {
		return err
	}
	f := selection.Filters{SubjectID: rule.SubjectID, GradeLevels: rule.GradeLevels, TopicIDs: rule.TopicIDs, ObjectiveIDs: rule.ObjectiveIDs, Types: types}
	if rule.DifficultyMin != nil || rule.DifficultyMax != nil {
		d := selection.Range{Min: 1, Max: 5}
		if rule.DifficultyMin != nil {
			d.Min = *rule.DifficultyMin
		}
		if rule.DifficultyMax != nil {
			d.Max = *rule.DifficultyMax
		}
		f.Difficulty = &d
	}
	if len(types) > 0 {
		check.Available = selection.Available(pool, selection.Rule{Filters: f})
	}
	return nil
}
