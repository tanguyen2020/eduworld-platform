package read

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

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
		AudioExemptApprovedBy: r.AudioExemptApprovedBy}
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

// Search implements read.QuestionReadModel: the latest version of each visible question matching the filter.
func (QuestionModel) Search(ctx context.Context, f read.QuestionFilter) ([]read.QuestionHit, error) {
	c := client(ctx)
	qq := c.Question.Query()
	switch f.Source {
	case "own":
		qq = qq.Where(entq.TenantIDNotNil())
	case "assigned", "platform":
		qq = qq.Where(entq.TenantIDIsNil())
	}
	qs, err := qq.Order(ent.Asc(entq.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	qids := make([]uuid.UUID, len(qs))
	for i, q := range qs {
		qids[i] = q.ID
	}
	rows, err := c.QuestionVersion.Query().Where(entqv.QuestionIDIn(qids...)).Order(ent.Desc(entqv.FieldVersionNo)).All(ctx)
	if err != nil {
		return nil, err
	}
	latest := map[uuid.UUID]dto.QuestionVersion{}
	for _, r := range rows {
		if _, ok := latest[r.QuestionID]; !ok {
			latest[r.QuestionID] = questionVersionOf(r)
		}
	}
	vs := make([]dto.QuestionVersion, 0, len(latest))
	for _, q := range qs {
		if v, ok := latest[q.ID]; ok {
			vs = append(vs, v)
		}
	}
	if f.Q != "" && len(vs) > 0 {
		// Full-text search without accents on the newest versions (F024-AC-17): search_tsv of the F024 migration.
		vids := make([]uuid.UUID, len(vs))
		for i, v := range vs {
			vids[i] = v.ID
		}
		ps := docFilters[predicate.QuestionVersion](read.DocFilter{Q: f.Q})
		hits, err := c.QuestionVersion.Query().Where(append(ps, entqv.IDIn(vids...))...).IDs(ctx)
		if err != nil {
			return nil, err
		}
		kept := vs[:0]
		for _, v := range vs {
			if slices.Contains(hits, v.ID) {
				kept = append(kept, v)
			}
		}
		vs = kept
	}
	if err := withObjectives(ctx, vs); err != nil {
		return nil, err
	}
	byQ := map[uuid.UUID]dto.QuestionVersion{}
	for _, v := range vs {
		byQ[v.QuestionID] = v
	}
	var out []read.QuestionHit
	for _, q := range qs {
		v, ok := byQ[q.ID]
		if !ok || !matches(v, f) {
			continue
		}
		out = append(out, read.QuestionHit{Question: questionOf(q), Version: v})
	}
	return out, nil
}

func matches(v dto.QuestionVersion, f read.QuestionFilter) bool {
	switch {
	case f.SubjectID != nil && v.SubjectID != *f.SubjectID,
		f.GradeLevel != nil && !slices.Contains(v.GradeLevels, *f.GradeLevel),
		f.TopicID != nil && v.TopicID != *f.TopicID,
		f.ObjectiveID != nil && !slices.Contains(v.ObjectiveIDs, *f.ObjectiveID),
		len(f.Difficulties) > 0 && !slices.Contains(f.Difficulties, v.Difficulty),
		f.Status != "" && string(v.Status) != f.Status,
		f.Type != "" && v.Type != f.Type,
		f.Efficacy != nil && v.EfficacyInstrument != *f.Efficacy:
		return false
	}
	return true
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
		v.max_score, v.content_language FROM content.question_delivery_v v
		JOIN content.questions q ON q.question_id = v.question_id
		WHERE `+cond+fmt.Sprintf(` AND v.question_version_id = ANY($%d::uuid[])`, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("question delivery: %w", err)
	}
	defer func() { _ = rows.Close() }()
	by := map[uuid.UUID]question.Content{}
	for rows.Next() {
		var c question.Content
		var typ string
		var math []byte
		if err := rows.Scan(&c.VersionID, &typ, &c.Stem, &c.Render, &math, &c.MaxScore, &c.Language); err != nil {
			return nil, err
		}
		c.Type = question.Type(typ)
		_ = json.Unmarshal(math, &c.MathAssets)
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
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.ID(versionID), entqv.Status(string(contentmodel.StatusPublished))).Only(ctx)
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

// Explanation implements read.DeliveryReader.
func (Delivery) Explanation(ctx context.Context, versionID uuid.UUID) (json.RawMessage, error) {
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.ID(versionID), entqv.Status(string(contentmodel.StatusPublished))).Only(ctx)
	if err != nil {
		return nil, notFound(err)
	}
	return r.Explanation, nil
}
