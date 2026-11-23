package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/predicate"
	entq "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/question"
	entqv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionversion"
	entqo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionversionobjective"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// QuestionModel implements read.QuestionReadModel with ent (entx filter of the scope of ctx).
type QuestionModel struct{}

var _ read.QuestionReadModel = QuestionModel{}

func questionOf(r *ent.Question) dto.Question {
	return dto.Question{ID: r.ID, Owner: ownerOf(r.OwnerScope, r.TenantID), CurrentVersionID: r.CurrentVersionID, LatestVersionNo: r.LatestVersionNo,
		RowVersion: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func questionVersionOf(r *ent.QuestionVersion) dto.QuestionVersion {
	v := dto.QuestionVersion{ID: r.ID, QuestionID: r.QuestionID, VersionNo: r.VersionNo, Status: contentmodel.Status(r.Status), Type: r.Type,
		Stem: r.Stem, RenderPayload: r.RenderPayload, AnswerKey: r.AnswerKey, ScoringRule: r.ScoringRule, Explanation: r.Explanation,
		OptionFeedback: r.OptionFeedback, Hints: r.Hints, WorkedExampleRef: r.WorkedExampleRef, MathAssets: r.MathAssets, MaxScore: r.MaxScore,
		ExamOnly: r.ExamOnly, EfficacyInstrument: r.EfficacyInstrument, Tags: r.Tags, Difficulty: int(r.Difficulty), GradeLevels: r.GradeLevels,
		SubjectID: r.SubjectID, TopicID: r.TopicID, CurriculumVersionID: r.CurriculumVersionID, MediaRefs: r.MediaRefs,
		ContentLanguage: r.ContentLanguage, AuthorID: r.AuthorID, ReviewerID: r.ReviewerID, PublishedAt: r.PublishedAt, ChangeNote: r.ChangeNote,
		BasedOnID: r.BasedOnVersionID, Owner: ownerOf(r.OwnerScope, r.TenantID), RowVersion: r.Version, UpdatedAt: r.UpdatedAt, TranslationStatus: r.TranslationStatus, TranslationMeta: r.TranslationMeta, AudioExemptReason: r.AudioExemptReason,
		AudioExemptApprovedBy: r.AudioExemptApprovedBy, PublishedBy: r.PublishedBy, SnapshotSHA256: r.SnapshotSha256}
	if r.MinResponseMs != nil {
		m := int(*r.MinResponseMs)
		v.MinResponseMS = &m
	}
	return v
}

func withObjectives(ctx context.Context, vs []dto.QuestionVersion) error {
	if len(vs) == 0 {
		return nil
	}
	vids := make([]uuid.UUID, len(vs))
	for i, v := range vs {
		vids[i] = v.ID
	}
	rows, err := client(ctx).QuestionVersionObjective.Query().Where(entqo.QuestionVersionIDIn(vids...)).Order(ent.Asc(entqo.FieldObjectiveID)).All(ctx)
	if err != nil {
		return err
	}
	by := map[uuid.UUID][]uuid.UUID{}
	for _, r := range rows {
		by[r.QuestionVersionID] = append(by[r.QuestionVersionID], r.ObjectiveID)
	}
	for i := range vs {
		vs[i].ObjectiveIDs = by[vs[i].ID]
	}
	return nil
}

// Question implements read.QuestionReadModel.
func (QuestionModel) Question(ctx context.Context, id uuid.UUID) (dto.Question, error) {
	r, err := client(ctx).Question.Query().Where(entq.ID(id)).Only(ctx)
	if err != nil {
		return dto.Question{}, notFound(err)
	}
	return questionOf(r), nil
}

// Version implements read.QuestionReadModel.
func (QuestionModel) Version(ctx context.Context, questionID uuid.UUID, versionNo int) (dto.QuestionVersion, error) {
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.QuestionID(questionID), entqv.VersionNo(versionNo)).Only(ctx)
	if err != nil {
		return dto.QuestionVersion{}, notFound(err)
	}
	vs := []dto.QuestionVersion{questionVersionOf(r)}
	return vs[0], withObjectives(ctx, vs)
}

// Latest implements read.QuestionReadModel.
func (QuestionModel) Latest(ctx context.Context, questionID uuid.UUID) (dto.QuestionVersion, error) {
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.QuestionID(questionID)).Order(ent.Desc(entqv.FieldVersionNo)).First(ctx)
	if err != nil {
		return dto.QuestionVersion{}, notFound(err)
	}
	vs := []dto.QuestionVersion{questionVersionOf(r)}
	return vs[0], withObjectives(ctx, vs)
}

// Search implements read.QuestionReadModel: one keyset page (question_id ASC) of the newest version of each visible
// question matching the filter. Every filter is in SQL — newest version, source, version columns, objective link,
// full-text — so the page is cut by the database (API_DESIGN §5); the ent interceptor keeps the tenant scope and live
// rows (R8, R21, R27).
func (QuestionModel) Search(ctx context.Context, f read.QuestionFilter, seek read.Seek) ([]read.QuestionHit, error) {
	c := client(ctx)
	vq := c.QuestionVersion.Query().Where(predicate.QuestionVersion(newestLiveVersion))
	switch f.Source {
	case "own":
		vq = vq.Where(entqv.TenantIDNotNil())
	case "assigned", "platform":
		vq = vq.Where(entqv.TenantIDIsNil())
	}
	if f.SubjectID != nil {
		vq = vq.Where(entqv.SubjectID(*f.SubjectID))
	}
	if f.TopicID != nil {
		vq = vq.Where(entqv.TopicID(*f.TopicID))
	}
	if len(f.Difficulties) > 0 {
		ds := make([]int16, len(f.Difficulties))
		for i, d := range f.Difficulties {
			ds[i] = int16(d) //nolint:gosec // 1–5 (contract)
		}
		vq = vq.Where(entqv.DifficultyIn(ds...))
	}
	if f.Status != "" {
		vq = vq.Where(entqv.Status(f.Status))
	}
	if f.Type != "" {
		vq = vq.Where(entqv.Type(f.Type))
	}
	if f.Efficacy != nil {
		vq = vq.Where(entqv.EfficacyInstrument(*f.Efficacy))
	}
	if f.ObjectiveID != nil {
		vq = vq.Where(predicate.QuestionVersion(linkedToObjective(*f.ObjectiveID)))
	}
	// grade (jsonb containment), content language, translation status and the accent-free full-text (F024-AC-17)
	vq = vq.Where(docFilters[predicate.QuestionVersion](read.DocFilter{GradeLevel: f.GradeLevel, ContentLanguage: f.ContentLanguage,
		TranslationStatus: f.TranslationStatus, Q: f.Q})...)
	if p := seekAfter(seek.After, false, entqv.FieldQuestionID); p != nil {
		vq = vq.Where(predicate.QuestionVersion(p))
	}
	if n := limitOf(seek); n > 0 {
		vq = vq.Limit(n)
	}
	rows, err := vq.Order(ent.Asc(entqv.FieldQuestionID)).All(ctx)
	if err != nil {
		return nil, err
	}
	vs := make([]dto.QuestionVersion, len(rows))
	qids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		vs[i], qids[i] = questionVersionOf(r), r.QuestionID
	}
	if err := withObjectives(ctx, vs); err != nil {
		return nil, err
	}
	qs, err := c.Question.Query().Where(entq.IDIn(qids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Question, len(qs))
	for _, q := range qs {
		byID[q.ID] = q
	}
	out := make([]read.QuestionHit, 0, len(vs))
	for _, v := range vs {
		if q, ok := byID[v.QuestionID]; ok {
			out = append(out, read.QuestionHit{Question: questionOf(q), Version: v})
		}
	}
	return out, nil
}

// newestLiveVersion keeps the newest version of a live question: version_no = questions.latest_version_no of the same
// question (same tenant, deleted_at IS NULL).
func newestLiveVersion(s *entsql.Selector) {
	lq := entsql.Table(entq.Table).Schema("content").As("lq")
	sub := entsql.Select(lq.C(entq.FieldLatestVersionNo)).From(lq).Where(entsql.And(
		entsql.ColumnsEQ(lq.C(entq.FieldID), s.C(entqv.FieldQuestionID)),
		entsql.Or(entsql.ColumnsEQ(lq.C(entq.FieldTenantID), s.C(entqv.FieldTenantID)),
			entsql.And(entsql.IsNull(lq.C(entq.FieldTenantID)), entsql.IsNull(s.C(entqv.FieldTenantID)))),
		entsql.IsNull(lq.C(entq.FieldDeletedAt))))
	s.Where(entsql.P(func(b *entsql.Builder) {
		b.Ident(s.C(entqv.FieldVersionNo)).WriteOp(entsql.OpEQ).Wrap(func(nb *entsql.Builder) { nb.Join(sub) })
	}))
}

// linkedToObjective keeps the versions linked to an objective (content.question_version_objectives, live links).
func linkedToObjective(objectiveID uuid.UUID) func(*entsql.Selector) {
	return func(s *entsql.Selector) {
		lo := entsql.Table(entqo.Table).Schema("content").As("qlo")
		sub := entsql.Select(lo.C(entqo.FieldQuestionVersionID)).From(lo).Where(entsql.And(
			entsql.EQ(lo.C(entqo.FieldObjectiveID), objectiveID), entsql.IsNull(lo.C(entqo.FieldDeletedAt))))
		s.Where(entsql.In(s.C(entqv.FieldID), sub))
	}
}

// Delivery implements read.DeliveryReader with explicit SQL on content.question_delivery_v (no answer column) and,
// for the grader only, on content.question_versions.
type Delivery struct{}

var _ read.DeliveryReader = Delivery{}

// Pool implements read.DeliveryReader.
func (Delivery) Pool(ctx context.Context, f read.PoolFilter) ([]selection.Candidate, error) {
	cond, args, err := HybridCondition(ctx, "q", contentmodel.ResourceQuestion, "question_id", 1)
	if err != nil {
		return nil, err
	}
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	subject := uuid.NullUUID{}
	if f.SubjectID != nil {
		subject = uuid.NullUUID{UUID: *f.SubjectID, Valid: true}
	}
	args = append(args, subject)
	// tenancyguard:ok the condition of HybridCondition on q holds tenant_id and deleted_at IS NULL; the view keeps published live rows
	rows, err := tx.QueryContext(ctx, `SELECT v.question_version_id, v.question_id, v.type, v.difficulty, v.subject_id, v.topic_id,
		v.grade_levels, v.tags, v.exam_only,
		COALESCE((SELECT jsonb_agg(o.objective_id) FROM content.question_version_objectives o
			WHERE o.question_version_id = v.question_version_id AND o.deleted_at IS NULL), '[]'::jsonb)
		FROM content.question_delivery_v v
		JOIN content.questions q ON q.question_id = v.question_id AND q.current_version_id = v.question_version_id
		WHERE `+cond+fmt.Sprintf(` AND NOT v.efficacy_instrument AND ($%d::uuid IS NULL OR v.subject_id = $%d::uuid)
		ORDER BY v.question_version_id`, len(args), len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("question pool: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []selection.Candidate
	for rows.Next() {
		var c selection.Candidate
		var grades, tags, los []byte
		if err := rows.Scan(&c.VersionID, &c.QuestionID, &c.Type, &c.Difficulty, &c.SubjectID, &c.TopicID, &grades, &tags, &c.ExamOnly, &los); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(grades, &c.GradeLevels)
		_ = json.Unmarshal(tags, &c.Tags)
		_ = json.Unmarshal(los, &c.ObjectiveIDs)
		out = append(out, c)
	}
	return out, rows.Err()
}

func uuidList(ids []uuid.UUID) string { return uuidArray(ids) }

// Deliverable implements read.DeliveryReader (view only).
func (Delivery) Deliverable(ctx context.Context, versionIDs []uuid.UUID) ([]question.Content, error) {
	cond, args, err := HybridCondition(ctx, "q", contentmodel.ResourceQuestion, "question_id", 1)
	if err != nil {
		return nil, err
	}
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	args = append(args, uuidList(versionIDs))
	// tenancyguard:ok the condition of HybridCondition on q holds tenant_id and deleted_at IS NULL; the view keeps published live rows
	rows, err := tx.QueryContext(ctx, `SELECT v.question_version_id, v.type, v.stem, v.render_payload, COALESCE(v.math_assets, '[]'::jsonb),
		v.max_score, v.content_language, (SELECT LEAST(CASE WHEN jsonb_typeof(x.hints) = 'array' THEN jsonb_array_length(x.hints) ELSE 0 END, 5)
			FROM content.question_versions x WHERE x.question_version_id = v.question_version_id) FROM content.question_delivery_v v
		JOIN content.questions q ON q.question_id = v.question_id
		WHERE (`+cond+` OR (q.tenant_id IS NULL AND q.deleted_at IS NULL AND v.efficacy_instrument))`+
		fmt.Sprintf(` AND v.question_version_id = ANY($%d::uuid[])`, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("question delivery: %w", err)
	}
	defer func() { _ = rows.Close() }()
	by := map[uuid.UUID]question.Content{}
	for rows.Next() {
		var c question.Content
		var typ string
		var math []byte
		var hints sql.NullInt32
		if err := rows.Scan(&c.VersionID, &typ, &c.Stem, &c.Render, &math, &c.MaxScore, &c.Language, &hints); err != nil {
			return nil, err
		}
		c.Type = question.Type(typ)
		_ = json.Unmarshal(math, &c.MathAssets)
		c.HintCount = int(hints.Int32)
		by[c.VersionID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]question.Content, 0, len(versionIDs))
	for _, id := range versionIDs {
		c, ok := by[id]
		if !ok {
			return nil, problem.New(problem.NotFound, "question version not deliverable")
		}
		out = append(out, c)
	}
	return out, nil
}

// Grading implements read.DeliveryReader: the only reader of answer_key and scoring_rule (ADR-0016).
func (Delivery) Grading(ctx context.Context, versionID uuid.UUID) (question.Content, error) {
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.ID(versionID), entqv.StatusIn(pinnable...)).Only(ctx)
	if err != nil {
		return question.Content{}, notFound(err)
	}
	c := question.Content{VersionID: r.ID, Type: question.Type(r.Type), Stem: r.Stem, Render: r.RenderPayload, AnswerKey: r.AnswerKey,
		MaxScore: r.MaxScore, Language: r.ContentLanguage}
	_ = json.Unmarshal(r.ScoringRule, &c.Scoring)
	if len(r.MathAssets) > 0 {
		_ = json.Unmarshal(r.MathAssets, &c.MathAssets)
	}
	return c, nil
}

// pinnable are the statuses of a version an attempt may still grade: published, or archived after it was pinned (R31).
var pinnable = []string{string(contentmodel.StatusPublished), string(contentmodel.StatusArchived)}

// OptionFeedback implements read.DeliveryReader: the server-only feedback of each option (given after grading only).
func (Delivery) OptionFeedback(ctx context.Context, versionID uuid.UUID) (map[string]question.OptionFeedback, error) {
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.ID(versionID), entqv.StatusIn(pinnable...)).Only(ctx)
	if err != nil {
		return nil, notFound(err)
	}
	out := map[string]question.OptionFeedback{}
	if len(r.OptionFeedback) > 0 {
		if err := json.Unmarshal(r.OptionFeedback, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Explanation implements read.DeliveryReader.
func (Delivery) Explanation(ctx context.Context, versionID uuid.UUID) (json.RawMessage, error) {
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.ID(versionID), entqv.StatusIn(pinnable...)).Only(ctx)
	if err != nil {
		return nil, notFound(err)
	}
	return r.Explanation, nil
}
