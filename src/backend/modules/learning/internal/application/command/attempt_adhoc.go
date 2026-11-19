package command

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AdHocSource is AdHocAttemptStart.source.
type AdHocSource struct {
	Kind          string     `json:"kind"`
	Interleave    bool       `json:"interleave"`
	QuestionSetID *uuid.UUID `json:"question_set_id"`
	SubjectID     *uuid.UUID `json:"subject_id"`
	TopicID       *uuid.UUID `json:"topic_id"`
	ObjectiveID   *uuid.UUID `json:"objective_id"`
	Difficulty    *int       `json:"difficulty"`
	QuestionCount *int       `json:"question_count"`
}

// StartAdHocInput is AdHocAttemptStart.
type StartAdHocInput struct {
	Mode             string      `json:"mode"`
	RecommendationID *uuid.UUID  `json:"recommendation_id"`
	Source           AdHocSource `json:"source"`
	ContextType      string      `json:"context_type"`
	ContextID        *uuid.UUID  `json:"context_id"`
	AcceptLanguage   string      `json:"-"`
}

// AdHocQuestionCount is the default number of questions of a selection practice (AdHocAttemptStart.question_count).
const AdHocQuestionCount = 10

// quizTypes are the question types of a quiz: an ad-hoc attempt made of them only is a quiz, otherwise an
// interactive_exercise [Proposed].
var quizTypes = []string{"single_choice", "multiple_choice", "true_false", "image_based"}

func adHocType(items []attempt.Item) string {
	for _, it := range items {
		if !slices.Contains(quizTypes, it.QuestionType) {
			return string(activity.InteractiveExercise)
		}
	}
	return string(activity.Quiz)
}

// StartAdHoc starts a practice without a fixed activity (F030-AC-27): the current release of a question set, or a
// selection of published questions of the tenant (own or assigned) by subject, topic, objective and difficulty with a
// stored seed; 422 QUESTION_POOL_INSUFFICIENT when the pool is too small. retry_wrong and diagnostic come with F032,
// assignment with F041.
func (h Attempts) StartAdHoc(ctx context.Context, in StartAdHocInput) (out dto.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.StartAdHocAttempt")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if in.Mode != string(attempt.ModePractice) {
		return out, problem.Invalid("/mode", problem.SubInvalid, "only practice is available in this release (retry_wrong, diagnostic: F032)")
	}
	if in.ContextType != "" && in.ContextType != "free_play" {
		return out, problem.Invalid("/context_type", problem.SubInvalid, "this context is not available in this release")
	}
	src := in.Source
	if src.Kind != activity.SourceQuestionSet && src.Kind != activity.SourceSelection {
		return out, problem.Invalid("/source/kind", problem.SubInvalid, "only question_set and selection are available in this release")
	}
	if src.Kind == activity.SourceQuestionSet && src.QuestionSetID == nil {
		return out, problem.Invalid("/source/question_set_id", problem.SubRequired, "is required with kind question_set")
	}
	if src.Kind == activity.SourceSelection && src.SubjectID == nil && src.TopicID == nil && src.ObjectiveID == nil {
		return out, problem.Invalid("/source/subject_id", problem.SubRequired, "a selection needs a subject, topic or objective")
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if prev, err := h.Store.AttemptByKey(ctx, p.UserID, key); err != nil || prev != nil {
			if err == nil {
				out, err = h.view(ctx, *prev, nil, nil, nil, nil)
			}
			return err
		}
		school, grade, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		if err := h.checkStartRate(ctx, p.UserID); err != nil {
			return err
		}
		seed, id := randomSeed(), ids.New()
		var qids []uuid.UUID
		kind, sourceVersion, ctxType, ctxID := "selection", id, "selection", id
		if src.Kind == activity.SourceQuestionSet {
			if sourceVersion, qids, err = h.questionSetItems(ctx, *src.QuestionSetID); err != nil {
				return err
			}
			kind, ctxType, ctxID = "question_set", "question_set", *src.QuestionSetID
			open, err := h.Store.OpenPractice(ctx, p.UserID, sourceVersion)
			if err != nil {
				return err
			}
			if open != nil && open.CheckActive(h.now(), h.policy()) == nil {
				if out, err = h.view(ctx, *open, nil, nil, nil, nil); err != nil {
					return err
				}
				return idempotency.SetResult(ctx, out)
			}
		} else {
			count := AdHocQuestionCount
			if src.QuestionCount != nil {
				count = *src.QuestionCount
			}
			f := selection.Filters{SubjectID: src.SubjectID}
			if src.TopicID != nil {
				f.TopicIDs, ctxID = []uuid.UUID{*src.TopicID}, *src.TopicID
			}
			if src.ObjectiveID != nil {
				f.ObjectiveIDs = []uuid.UUID{*src.ObjectiveID}
			}
			if src.Difficulty != nil {
				f.Difficulty = &selection.Range{Min: *src.Difficulty, Max: *src.Difficulty}
			}
			if grade != nil {
				f.GradeLevels = []int{*grade}
			}
			if qids, err = h.selectQuestions(ctx, f, count, seed); err != nil {
				return err
			}
		}
		items, rendered, err := h.pinItems(ctx, qids, seed, src.Kind == activity.SourceSelection || src.Interleave, nil)
		if err != nil {
			return err
		}
		a := attempt.New(attempt.Start{ID: id, TenantID: *p.TenantID, SchoolID: school, StudentID: p.UserID, Mode: attempt.ModePractice,
			ContextType: ctxType, ContextID: ctxID, ActivityType: adHocType(items), SourceKind: kind, SourceVersionID: sourceVersion, Seed: seed,
			Now: h.now(), DisplayLanguage: i18ntext.Locale(in.AcceptLanguage), RecommendationID: in.RecommendationID,
			IdempotencyKey: key, Items: items})
		if err := h.Store.InsertAttempt(ctx, a); err != nil {
			return err
		}
		if err := h.started(ctx, a); err != nil {
			return err
		}
		if out, err = h.view(ctx, a, rendered, nil, nil, nil); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}
