package read

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entact "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/activity"
	entactv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/activityversion"
	entatt "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/attempt"
	entai "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/attemptitem"
	enteval "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/evaluation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AttemptModel implements read.AttemptReadModel with ent (entx.Base filter of the scope of ctx) and explicit SQL on the
// partitioned assessment.attempt_answers.
type AttemptModel struct{}

var _ read.AttemptReadModel = AttemptModel{}

func utcp(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func attemptOf(r *ent.Attempt) attempt.Attempt {
	a := attempt.Attempt{ID: r.ID, TenantID: r.TenantID, SchoolID: r.SchoolID, StudentID: r.StudentID, Mode: attempt.Mode(r.Mode),
		ContextType: r.ContextType, ContextID: r.ContextID, ActivityID: r.ActivityID, ActivityType: r.ActivityType, SourceKind: r.SourceKind,
		SourceVersionID: r.SourceVersionID, Status: attempt.Status(r.Status), Seed: r.Seed, StartedAt: r.StartedAt.UTC(), DeadlineAt: utcp(r.DeadlineAt),
		SubmittedAt: utcp(r.SubmittedAt), GradedAt: utcp(r.GradedAt), PausedAt: utcp(r.PausedAt), PausedMS: r.PausedMs,
		LastActivityAt: r.LastActivityAt.UTC(), Feedback: attempt.FeedbackTiming(r.FeedbackTiming), DisplayLanguage: r.DisplayLanguage,
		SubmitReason: r.SubmitReason, Score: r.Score, MaxScore: r.MaxScore, DurationMS: r.DurationMs, ClientElapsedMS: r.ClientElapsedMs,
		ObjectiveIDs: r.ObjectiveIds, RecommendationID: r.RecommendationID, IdempotencyKey: r.IdempotencyKey, RowVersion: r.Version,
		GameScore: r.GameScore, MinigameConfigVersionID: r.MinigameConfigVersionID}
	if r.GameStreak != nil {
		a.GameStreak = *r.GameStreak
	}
	return a
}

// Attempt implements read.AttemptReadModel.
func (AttemptModel) Attempt(ctx context.Context, id uuid.UUID) (attempt.Attempt, error) {
	r, err := client(ctx).Attempt.Get(ctx, id)
	if ent.IsNotFound(err) {
		return attempt.Attempt{}, problem.New(problem.AttemptNotFound, "")
	}
	if err != nil {
		return attempt.Attempt{}, err
	}
	a := attemptOf(r)
	rows, err := client(ctx).AttemptItem.Query().Where(entai.AttemptID(id)).Order(entai.ByPosition()).All(ctx)
	if err != nil {
		return a, err
	}
	for _, it := range rows {
		a.Items = append(a.Items, attempt.Item{ID: it.ID, Position: it.Position, QuestionVersionID: it.QuestionVersionID, QuestionType: it.QuestionType,
			ObjectiveIDs: it.ObjectiveIds, Presentation: it.Presentation, PayloadHash: it.PresentedPayloadHash, MaxScore: it.MaxScore,
			AnswerCount: it.AnswerCount, FinalIsCorrect: it.FinalIsCorrect, FinalScore: it.FinalScore, HintsRevealed: it.HintsRevealed,
			RowVersion: it.Version})
	}
	return a, nil
}

// List implements read.AttemptReadModel.
func (AttemptModel) List(ctx context.Context, studentID uuid.UUID, f read.AttemptFilter) ([]attempt.Attempt, error) {
	q := client(ctx).Attempt.Query().Where(entatt.StudentID(studentID))
	if f.Mode != "" {
		q.Where(entatt.Mode(f.Mode))
	}
	if f.Status != "" {
		q.Where(entatt.Status(f.Status))
	}
	if f.ActivityType != "" {
		q.Where(entatt.ActivityType(f.ActivityType))
	}
	if f.After != nil {
		q.Where(entatt.IDLT(*f.After))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := q.Order(ent.Desc(entatt.FieldID)).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]attempt.Attempt, len(rows))
	for i, r := range rows {
		out[i] = attemptOf(r)
	}
	return out, nil
}

// LastAnswers implements read.AttemptReadModel.
func (AttemptModel) LastAnswers(ctx context.Context, attemptID uuid.UUID) ([]read.SavedAnswer, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	tenant, err := sessionTenant(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only table without deleted_at, filtered by the session tenant (RLS T as well)
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT ON (position) position, response, is_correct, score, is_scored
		FROM assessment.attempt_answers WHERE attempt_id = $1 AND tenant_id = $2 ORDER BY position, answered_at DESC, attempt_no DESC`,
		attemptID, tenant)
	if err != nil {
		return nil, fmt.Errorf("attempt answers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SavedAnswer
	for rows.Next() {
		var a read.SavedAnswer
		var resp []byte
		if err := rows.Scan(&a.Position, &resp, &a.IsCorrect, &a.Score, &a.IsScored); err != nil {
			return nil, err
		}
		a.Response = json.RawMessage(resp)
		out = append(out, a)
	}
	return out, rows.Err()
}

// Evaluation implements read.AttemptReadModel.
func (AttemptModel) Evaluation(ctx context.Context, attemptID uuid.UUID) (*dto.EvaluationView, error) {
	r, err := client(ctx).Evaluation.Query().Where(enteval.AttemptID(attemptID), enteval.Effective(true)).First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ev := &dto.EvaluationView{EvaluationID: r.ID, AttemptID: r.AttemptID, Source: r.Source, Status: r.Status, Score: r.Score, MaxScore: r.MaxScore,
		Passed: r.Passed, Effective: r.Effective, CreatedAt: r.CreatedAt.UTC()}
	// F073-AC-10: the game result of a mini-game attempt is kept in the details of its evaluation.
	var d struct {
		Minigame *struct {
			GameScore     int  `json:"game_score"`
			CompletionMet bool `json:"completion_met"`
		} `json:"minigame"`
	}
	if len(r.Details) > 0 && json.Unmarshal(r.Details, &d) == nil && d.Minigame != nil {
		gs, cm := d.Minigame.GameScore, d.Minigame.CompletionMet
		ev.GameScore, ev.CompletionMet = &gs, &cm
	}
	return ev, nil
}

// ActivityVersion implements read.AttemptReadModel.
func (AttemptModel) ActivityVersion(ctx context.Context, activityID, versionID uuid.UUID) (dto.Doc, dto.DocVersion, error) {
	d, err := client(ctx).Activity.Query().Where(entact.ID(activityID)).Only(ctx)
	if err != nil {
		return dto.Doc{}, dto.DocVersion{}, notFound(err)
	}
	v, err := client(ctx).ActivityVersion.Query().Where(entactv.ID(versionID), entactv.ActivityID(activityID)).Only(ctx)
	if err != nil {
		return dto.Doc{}, dto.DocVersion{}, notFound(err)
	}
	return activityDocOf(d), activityVersionOf(v), nil
}
