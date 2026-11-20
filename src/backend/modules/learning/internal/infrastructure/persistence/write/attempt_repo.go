package write

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entatt "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/attempt"
	entai "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/attemptitem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AttemptRepo implements write.AttemptStore: ent for attempts, items and evaluations (entx.Base filter of the scope of
// ctx), explicit SQL for the partitioned append-only assessment.attempt_answers.
type AttemptRepo struct{}

var _ write.AttemptStore = AttemptRepo{}

func attemptErr(err error) error {
	if ent.IsNotFound(err) {
		return problem.New(problem.AttemptNotFound, "").Wrap(err)
	}
	return MapCurriculumErr(err)
}

func modeOf(r *ent.Attempt) attempt.Attempt {
	return attempt.Attempt{ID: r.ID, TenantID: r.TenantID, SchoolID: r.SchoolID, StudentID: r.StudentID, Mode: attempt.Mode(r.Mode),
		ContextType: r.ContextType, ContextID: r.ContextID, ActivityID: r.ActivityID, ActivityType: r.ActivityType, SourceKind: r.SourceKind,
		SourceVersionID: r.SourceVersionID, Status: attempt.Status(r.Status), Seed: r.Seed, StartedAt: r.StartedAt.UTC(), DeadlineAt: utc(r.DeadlineAt),
		SubmittedAt: utc(r.SubmittedAt), GradedAt: utc(r.GradedAt), PausedAt: utc(r.PausedAt), PausedMS: r.PausedMs, LastActivityAt: r.LastActivityAt.UTC(),
		Feedback: attempt.FeedbackTiming(r.FeedbackTiming), DisplayLanguage: r.DisplayLanguage, SubmitReason: r.SubmitReason, Score: r.Score,
		MaxScore: r.MaxScore, DurationMS: r.DurationMs, ClientElapsedMS: r.ClientElapsedMs, ObjectiveIDs: r.ObjectiveIds,
		RecommendationID: r.RecommendationID, IdempotencyKey: r.IdempotencyKey, RowVersion: r.Version}
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func itemOf(r *ent.AttemptItem) attempt.Item {
	return attempt.Item{ID: r.ID, Position: r.Position, QuestionVersionID: r.QuestionVersionID, QuestionType: r.QuestionType,
		ObjectiveIDs: r.ObjectiveIds, Presentation: r.Presentation, PayloadHash: r.PresentedPayloadHash, MaxScore: r.MaxScore,
		AnswerCount: r.AnswerCount, FinalIsCorrect: r.FinalIsCorrect, FinalScore: r.FinalScore, HintsRevealed: r.HintsRevealed, RowVersion: r.Version}
}

// InsertAttempt implements write.AttemptStore.
func (AttemptRepo) InsertAttempt(ctx context.Context, a attempt.Attempt) error {
	cl := client(ctx)
	objectives := a.ObjectiveIDs
	if objectives == nil {
		objectives = []uuid.UUID{}
	}
	err := cl.Attempt.Create().SetID(a.ID).SetTenantID(a.TenantID).SetSchoolID(a.SchoolID).SetStudentID(a.StudentID).SetMode(string(a.Mode)).
		SetContextType(a.ContextType).SetContextID(a.ContextID).SetNillableActivityID(a.ActivityID).SetActivityType(a.ActivityType).
		SetSourceKind(a.SourceKind).SetSourceVersionID(a.SourceVersionID).SetStatus(string(a.Status)).SetSeed(a.Seed).SetStartedAt(a.StartedAt).
		SetNillableDeadlineAt(a.DeadlineAt).SetLastActivityAt(a.LastActivityAt).SetFeedbackTiming(string(a.Feedback)).
		SetDisplayLanguage(a.DisplayLanguage).SetItemCount(len(a.Items)).SetObjectiveIds(objectives).SetNillableRecommendationID(a.RecommendationID).
		SetIdempotencyKey(a.IdempotencyKey).SetCreatedBy(a.StudentID).Exec(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if len(a.Items) == 0 {
		return nil
	}
	bulk := make([]*ent.AttemptItemCreate, len(a.Items))
	for i, it := range a.Items {
		objs := it.ObjectiveIDs
		if objs == nil {
			objs = []uuid.UUID{}
		}
		bulk[i] = cl.AttemptItem.Create().SetID(it.ID).SetTenantID(a.TenantID).SetAttemptID(a.ID).SetPosition(it.Position).
			SetQuestionVersionID(it.QuestionVersionID).SetQuestionType(it.QuestionType).SetObjectiveIds(objs).SetPresentation(it.Presentation).
			SetPresentedPayloadHash(it.PayloadHash).SetMaxScore(it.MaxScore)
	}
	return MapCurriculumErr(cl.AttemptItem.CreateBulk(bulk...).Exec(ctx))
}

func (AttemptRepo) load(ctx context.Context, r *ent.Attempt) (attempt.Attempt, error) {
	a := modeOf(r)
	rows, err := client(ctx).AttemptItem.Query().Where(entai.AttemptID(r.ID)).Order(entai.ByPosition()).All(ctx)
	if err != nil {
		return a, MapCurriculumErr(err)
	}
	a.Items = make([]attempt.Item, len(rows))
	for i, it := range rows {
		a.Items[i] = itemOf(it)
	}
	return a, nil
}

// Attempt implements write.AttemptStore.
func (s AttemptRepo) Attempt(ctx context.Context, id uuid.UUID) (attempt.Attempt, error) {
	r, err := client(ctx).Attempt.Get(ctx, id)
	if err != nil {
		return attempt.Attempt{}, attemptErr(err)
	}
	return s.load(ctx, r)
}

func (s AttemptRepo) first(ctx context.Context, q *ent.AttemptQuery) (*attempt.Attempt, error) {
	r, err := q.First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	a, err := s.load(ctx, r)
	return &a, err
}

// AttemptByKey implements write.AttemptStore.
func (s AttemptRepo) AttemptByKey(ctx context.Context, studentID uuid.UUID, key string) (*attempt.Attempt, error) {
	return s.first(ctx, client(ctx).Attempt.Query().Where(entatt.StudentID(studentID), entatt.IdempotencyKey(key)))
}

// OpenPractice implements write.AttemptStore.
func (s AttemptRepo) OpenPractice(ctx context.Context, studentID, sourceVersionID uuid.UUID) (*attempt.Attempt, error) {
	return s.first(ctx, client(ctx).Attempt.Query().Where(entatt.StudentID(studentID), entatt.SourceVersionID(sourceVersionID),
		entatt.Status(string(attempt.StatusInProgress)), entatt.Mode(string(attempt.ModePractice))))
}

// OpenAdHoc implements write.AttemptStore.
func (s AttemptRepo) OpenAdHoc(ctx context.Context, studentID uuid.UUID, mode attempt.Mode, contextType string, contextID uuid.UUID) (*attempt.Attempt, error) {
	return s.first(ctx, client(ctx).Attempt.Query().Where(entatt.StudentID(studentID), entatt.Mode(string(mode)), entatt.ContextType(contextType),
		entatt.ContextID(contextID), entatt.ActivityIDIsNil(), entatt.Status(string(attempt.StatusInProgress))).
		Order(ent.Desc(entatt.FieldStartedAt)))
}

// UpdateAttempt implements write.AttemptStore: the conditional update of ACTIVITY_ENGINE §11 (row version).
func (AttemptRepo) UpdateAttempt(ctx context.Context, a attempt.Attempt, ifRow int64) error {
	u := client(ctx).Attempt.Update().Where(entatt.ID(a.ID), entatt.Version(ifRow)).SetStatus(string(a.Status)).
		SetLastActivityAt(a.LastActivityAt).SetPausedMs(a.PausedMS).SetNillableSubmittedAt(a.SubmittedAt).SetNillableGradedAt(a.GradedAt).
		SetNillableSubmitReason(a.SubmitReason).SetNillableScore(a.Score).SetNillableMaxScore(a.MaxScore).SetNillableDurationMs(a.DurationMS).
		SetNillableClientElapsedMs(a.ClientElapsedMS).AddVersion(1)
	if a.PausedAt != nil {
		u.SetPausedAt(*a.PausedAt)
	} else {
		u.ClearPausedAt()
	}
	n, err := u.Save(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return problem.New(problem.VersionMismatch, "the attempt changed meanwhile")
	}
	return nil
}

// UpdateItem implements write.AttemptStore.
func (AttemptRepo) UpdateItem(ctx context.Context, it attempt.Item) error {
	n, err := client(ctx).AttemptItem.Update().Where(entai.ID(it.ID), entai.Version(it.RowVersion)).SetAnswerCount(it.AnswerCount).
		SetNillableFinalIsCorrect(it.FinalIsCorrect).SetNillableFinalScore(it.FinalScore).SetHintsRevealed(it.HintsRevealed).AddVersion(1).Save(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return problem.New(problem.VersionMismatch, "the attempt item changed meanwhile")
	}
	return nil
}

// InsertAnswer implements write.AttemptStore (append-only, the role holds INSERT/SELECT only).
func (AttemptRepo) InsertAnswer(ctx context.Context, r write.AnswerRow) error {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return entx.ErrNoTransaction
	}
	var correct, score any = r.IsCorrect, r.Score
	if r.Ungraded {
		correct, score = nil, nil
	}
	var client *int64
	if r.ClientElapsedMS != nil {
		v := min(*r.ClientElapsedMS, int64(1<<31-1))
		client = &v
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO assessment.attempt_answers (attempt_answer_id, tenant_id, created_at, attempt_id, position,
		school_id, question_version_id, response, is_correct, score, answered_at, client_elapsed_ms, response_ms, attempt_no, is_scored,
		idempotency_key, hints_used, rapid_guess) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $3, $11, $12, $13, $14, $15, $16, $17)`,
		r.ID, r.TenantID, r.AnsweredAt.UTC(), r.AttemptID, r.Position, r.SchoolID, r.QuestionVersionID, []byte(r.Response), correct, score,
		client, min(r.ResponseMS, int64(1<<31-1)), r.AttemptNo, r.IsScored, r.IdempotencyKey, r.HintsUsed, r.RapidGuess)
	if err != nil {
		return fmt.Errorf("attempt answer: %w", MapCurriculumErr(err))
	}
	return nil
}

// InsertEvaluation implements write.AttemptStore.
func (AttemptRepo) InsertEvaluation(ctx context.Context, r write.EvaluationRow) error {
	q := client(ctx).Evaluation.Create().SetID(r.ID).SetAttemptID(r.AttemptID).SetSource(r.Source).SetStatus(r.Status).SetScore(r.Score).
		SetMaxScore(r.MaxScore).SetNillablePassed(r.Passed).SetEffective(r.Effective).SetNillableRegradeRequestID(r.RegradeRequestID).
		SetNillableSupersedesEvaluationID(r.SupersedesID).SetNillableTriggeredBy(r.TriggeredBy).SetNillableReason(r.Reason)
	if len(r.Details) > 0 {
		q.SetDetails(r.Details)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// StartedSince implements write.AttemptStore.
func (AttemptRepo) StartedSince(ctx context.Context, studentID uuid.UUID, since time.Time) (int, error) {
	n, err := client(ctx).Attempt.Query().Where(entatt.StudentID(studentID), entatt.StartedAtGTE(since)).Count(ctx)
	return n, MapCurriculumErr(err)
}

// Due implements write.AttemptStore (ctx carries tenancy.Bypass(platform_job): every tenant).
func (AttemptRepo) Due(ctx context.Context, idleBefore, resumeBefore, deadlineBefore time.Time, limit int) ([]write.DueAttempt, error) {
	rows, err := client(ctx).Attempt.Query().Where(entatt.Status(string(attempt.StatusInProgress)),
		entatt.Or(entatt.And(entatt.PausedAtIsNil(), entatt.LastActivityAtLT(idleBefore)), entatt.LastActivityAtLT(resumeBefore),
			entatt.DeadlineAtLT(deadlineBefore))).
		Order(entatt.ByLastActivityAt()).Limit(limit).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]write.DueAttempt, len(rows))
	for i, r := range rows {
		out[i] = write.DueAttempt{ID: r.ID, TenantID: r.TenantID}
	}
	return out, nil
}
