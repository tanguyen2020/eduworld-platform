package command

import (
	"context"
	"errors"
	mrand "math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// errProjectionNotReady is 409 CONFLICT: the MiniGameConfigVersion is not in the projection yet (module game answers
// 503 UNAVAILABLE with Retry-After, F073 edge case).
func errProjectionNotReady() error {
	return problem.New(problem.Conflict, "the mini-game configuration version is not projected yet; retry shortly").Retry()
}

// projected loads a version of the projection ref.minigame_config_versions (409 CONFLICT when absent).
func (h Attempts) projected(ctx context.Context, versionID uuid.UUID) (dto.MinigameConfigVersion, error) {
	if h.Minigames == nil {
		return dto.MinigameConfigVersion{}, problem.New(problem.Internal, "the mini-game projection is not wired")
	}
	row, err := h.Minigames.MinigameConfigVersion(ctx, versionID)
	if err != nil {
		return dto.MinigameConfigVersion{}, err
	}
	if row == nil {
		return dto.MinigameConfigVersion{}, errProjectionNotReady()
	}
	return *row, nil
}

// activityConfigVersion is the configuration version a minigame learning activity plays (activities/minigame.schema.json):
// its version number, else the version published last in the projection; pinned by the attempt (ADR-0043).
func (h Attempts) activityConfigVersion(ctx context.Context, src ActivitySource) (dto.MinigameConfigVersion, minigame.ActivityContent, error) {
	ac, err := minigame.ParseActivity(src.Config.Raw)
	if err != nil {
		return dto.MinigameConfigVersion{}, ac, problem.New(problem.ActivityConfigInvalid, "the published minigame config cannot be read")
	}
	if h.Minigames == nil {
		return dto.MinigameConfigVersion{}, ac, problem.New(problem.Internal, "the mini-game projection is not wired")
	}
	var row *dto.MinigameConfigVersion
	if ac.MinigameConfigVersion != nil {
		row, err = h.Minigames.MinigameConfigVersionNo(ctx, ac.MinigameConfigID, *ac.MinigameConfigVersion)
	} else {
		row, err = h.Minigames.CurrentMinigameConfigVersion(ctx, ac.MinigameConfigID)
	}
	if err != nil {
		return dto.MinigameConfigVersion{}, ac, err
	}
	if row == nil {
		return dto.MinigameConfigVersion{}, ac, errProjectionNotReady()
	}
	return *row, ac, nil
}

// gradeOf is the grade of the student of an attempt (ref.students; nil when unknown).
func (h Attempts) gradeOf(ctx context.Context, a attempt.Attempt) (*int, error) {
	if h.Students == nil {
		return nil, nil
	}
	_, grade, err := h.Students.StudentSchool(ctx, a.StudentID)
	return grade, err
}

// minigameVersion returns the plugin and version of a mini-game attempt (F073-AC-17): the pinned MiniGameConfigVersion of
// the projection — alone for a play started by module game, on the pinned activity version for a minigame learning
// activity — with the grade of its student.
func (h Attempts) minigameVersion(ctx context.Context, a attempt.Attempt) (activity.Plugin, activity.Version, *ActivitySource, error) {
	grade, err := h.gradeOf(ctx, a)
	if err != nil {
		return nil, activity.Version{}, nil, err
	}
	pinned := a.SourceVersionID
	if a.MinigameConfigVersionID != nil {
		pinned = *a.MinigameConfigVersionID
	}
	row, err := h.projected(ctx, pinned)
	if err != nil {
		return nil, activity.Version{}, nil, err
	}
	if a.SourceKind == attemptview.SourceKindMinigameConfig || a.ActivityID == nil {
		p, err := ActivityPlugin(h.Registry, string(activity.Minigame))
		if err != nil {
			return nil, activity.Version{}, nil, err
		}
		src, err := attemptview.MinigameSource(row, p, grade)
		if err != nil {
			return nil, activity.Version{}, nil, problem.New(problem.ActivityConfigInvalid, "the pinned mini-game configuration cannot be read")
		}
		return src.Plugin, src.DomainVersion(), &src, nil
	}
	src, err := LoadActivityVersion(ctx, h.Docs, h.Registry, *a.ActivityID, &a.SourceVersionID)
	if err != nil {
		return nil, activity.Version{}, nil, err
	}
	attemptview.Pin(&src, row, grade)
	return src.Plugin, src.DomainVersion(), &src, nil
}

// minigameQuestions resolves the question versions of a play (F073-AC-03, AC-06): the question set of the source (its
// current release, or the fixed version number) drawn to question_count by the seed of the attempt, or question_count
// questions of the published pool of the tenant matching the selection rule — the pool is narrowed to the question
// types of the compatibility table before the selection, so no relaxation can return another type. A source with fewer
// questions than question_count is 422 QUESTION_POOL_INSUFFICIENT (no attempt).
func (h Attempts) minigameQuestions(ctx context.Context, rules minigame.Rules, src minigame.Source, seed int64) ([]uuid.UUID, error) {
	count := rules.QuestionCount
	allowed := minigame.Allowed(rules.Type)
	switch src.Kind {
	case minigame.SourceQuestionSet:
		if src.QuestionSetID == nil {
			return nil, problem.New(problem.ActivityConfigInvalid, "question_set_id is missing")
		}
		var pinned *uuid.UUID
		if src.Version != nil {
			v, err := h.Docs.DocVersion(ctx, contentmodel.TypeQuestionSet, *src.QuestionSetID, *src.Version)
			if err != nil {
				return nil, err
			}
			if v.PublishedAt == nil {
				return nil, problem.New(problem.ContentNotPublished, "the question set version is not published")
			}
			pinned = &v.ID
		}
		_, qids, err := h.questionSetItems(ctx, *src.QuestionSetID, pinned, seed)
		if err != nil {
			return nil, err
		}
		if len(qids) < count {
			return nil, problem.New(problem.QuestionPoolInsufficient, "the question set has fewer questions than question_count")
		}
		r := mrand.New(mrand.NewPCG(uint64(seed), uint64(seed)>>1|1)) //nolint:gosec // reproducible draw of the attempt, not a secret
		r.Shuffle(len(qids), func(i, j int) { qids[i], qids[j] = qids[j], qids[i] })
		return qids[:count], nil
	case minigame.SourceSelectionRule:
		if src.Rule == nil {
			return nil, problem.New(problem.ActivityConfigInvalid, "the selection rule is missing")
		}
		rule := src.Rule
		types := allowed
		if len(rule.QuestionTypes) > 0 {
			types = nil
			for _, t := range rule.QuestionTypes {
				if slices.Contains(allowed, t) {
					types = append(types, t)
				}
			}
		}
		pool, err := h.Delivery.Pool(ctx, rule.SubjectID)
		if err != nil {
			return nil, err
		}
		pool = slices.DeleteFunc(slices.Clone(pool), func(c selection.Candidate) bool { return !slices.Contains(types, c.Type) })
		f := selection.Filters{SubjectID: rule.SubjectID, GradeLevels: rule.GradeLevels, TopicIDs: rule.TopicIDs, ObjectiveIDs: rule.ObjectiveIDs,
			Types: types}
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
		cs, err := selection.Select(pool, selection.Request{Rule: selection.Rule{Filters: f, Count: count}, Seed: uint64(seed)})
		if errors.Is(err, selection.ErrPoolInsufficient) || len(cs) < count {
			return nil, problem.New(problem.QuestionPoolInsufficient, "the question pool cannot satisfy the mini-game selection rule")
		}
		if err != nil {
			return nil, err
		}
		out := make([]uuid.UUID, len(cs))
		for i, c := range cs {
			out[i] = c.VersionID
		}
		return out, nil
	}
	return nil, problem.New(problem.ActivityConfigInvalid, "unknown question source")
}

// minigameStart is what a mini-game play pins at start.
type minigameStart struct {
	src ActivitySource // a minigame learning activity with its pinned configuration, or MinigameSource
	// activity is true for a minigame learning activity (activity_id set), false for a play of module game.
	activity       bool
	override       *minigame.SetOverride
	mode           attempt.Mode
	ctxType        string
	ctxID          uuid.UUID
	recommendation *uuid.UUID
	acceptLanguage string
	key            string
	now            time.Time
}

// newMinigameAttempt starts a mini-game play (F073-AC-06, AC-17, AC-26): items from the question source of the pinned
// configuration (or the question set override of the activity), every question type inside the compatibility table,
// deadline from the time limit, feedback timing of the activity or of the configuration; the attempt pins the
// configuration version and starts with a game score of 0. Every item is delivered at start (timed_challenge prefetches
// the whole batch); each answer is still graded by the server.
func (h Attempts) newMinigameAttempt(ctx context.Context, p authz.Principal, school uuid.UUID, in minigameStart) (attempt.Attempt, []question.Rendered, error) {
	rules, ok := attemptview.Minigame(&in.src)
	if !ok {
		return attempt.Attempt{}, nil, problem.New(problem.ActivityConfigInvalid, "the pinned mini-game configuration cannot be read")
	}
	seed := randomSeed()
	qids, err := h.minigameQuestions(ctx, rules, rules.Effective(in.override), seed)
	if err != nil {
		return attempt.Attempt{}, nil, err
	}
	items, rendered, err := h.pinItems(ctx, qids, seed, false, minigame.Allowed(rules.Type))
	if err != nil {
		return attempt.Attempt{}, nil, err
	}
	var activityID *uuid.UUID
	kind := attemptview.SourceKindMinigameConfig
	if in.activity {
		id := in.src.Doc.ID
		activityID, kind = &id, "activity"
	}
	pinned := in.src.Config.Pinned.VersionID
	a := attempt.New(attempt.Start{ID: ids.New(), TenantID: *p.TenantID, SchoolID: school, StudentID: p.UserID, Mode: in.mode, ContextType: in.ctxType,
		ContextID: in.ctxID, ActivityID: activityID, ActivityType: string(activity.Minigame), SourceKind: kind, SourceVersionID: in.src.Version.ID,
		Seed: seed, Now: in.now, TimeLimitSeconds: rules.TimeLimit(in.src.Body.TimeLimitSeconds), ActivityFeedback: in.src.Config.Feedback.Timing,
		DisplayLanguage: displayLanguage(in.acceptLanguage, in.src.Version.Title, in.src.Version.ContentLanguage), RecommendationID: in.recommendation,
		IdempotencyKey: in.key, Items: items, MinigameConfigVersionID: &pinned})
	if err := h.Store.InsertAttempt(ctx, a); err != nil {
		return a, nil, err
	}
	return a, rendered, h.started(ctx, a)
}

// startMinigameActivity starts a play of a minigame learning activity (school assignment, practice, F073-AC-14): the
// configuration version is pinned (the version number of the activity or the current one of the projection) and the
// question set override replaces the question source.
func (h Attempts) startMinigameActivity(ctx context.Context, p authz.Principal, school uuid.UUID, src ActivitySource, in minigameStart) (attempt.Attempt,
	[]question.Rendered, error) {
	row, ac, err := h.activityConfigVersion(ctx, src)
	if err != nil {
		return attempt.Attempt{}, nil, err
	}
	_, grade, err := h.studentSchool(ctx, p)
	if err != nil {
		return attempt.Attempt{}, nil, err
	}
	attemptview.Pin(&src, row, grade)
	in.src, in.override, in.activity = src, ac.QuestionSetOverride, true
	return h.newMinigameAttempt(ctx, p, school, in)
}

// minigameOutcome is the game result of a graded mini-game attempt.
type minigameOutcome struct {
	rules         minigame.Rules
	versionID     uuid.UUID
	gameScore     int
	completionMet bool
}

// outcomeOf computes the game result of a mini-game attempt from its items (nil for another plugin): the game score kept
// by the answers and the completion rule of the pinned configuration.
func outcomeOf(plugin activity.Plugin, v activity.Version, a attempt.Attempt, items []activity.ItemResult) *minigameOutcome {
	gs, ok := plugin.(activity.GameScored)
	if !ok || !a.GameScored() || v.Config.Pinned == nil {
		return nil
	}
	rules, ok := minigame.RulesOf(v.Config)
	if !ok {
		return nil
	}
	return &minigameOutcome{rules: rules, versionID: v.Config.Pinned.VersionID, gameScore: *a.GameScore, completionMet: gs.CompletionMet(v, items)}
}

// details is the minigame part of the evaluation details (read back by the evaluation views).
func (o *minigameOutcome) details() map[string]any {
	return map[string]any{"game_score": o.gameScore, "completion_met": o.completionMet}
}

// payload is the minigame block of activity.graded (F073-AC-18): never in the learning event nor the mastery evidence.
func (o *minigameOutcome) payload(a attempt.Attempt) map[string]any {
	return map[string]any{"minigame_config_id": o.rules.MinigameConfigID, "minigame_config_version_id": o.versionID, "minigame_type": o.rules.Type,
		"game_score": o.gameScore, "completion_met": o.completionMet, "context_type": a.ContextType, "context_id": a.ContextID}
}

// view adds the game result to an evaluation view.
func (o *minigameOutcome) view(ev *dto.EvaluationView) {
	if o == nil || ev == nil {
		return
	}
	gs, cm := o.gameScore, o.completionMet
	ev.GameScore, ev.CompletionMet = &gs, &cm
}
