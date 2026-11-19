package write

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entq "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/question"
	entqv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionversion"
	entqo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionversionobjective"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// QuestionRepo implements write.QuestionStore with ent (entx.HybridContent filter of the scope of ctx).
type QuestionRepo struct{}

var _ write.QuestionStore = QuestionRepo{}

// QuestionOf maps an ent question.
func QuestionOf(r *ent.Question) dto.Question {
	return dto.Question{ID: r.ID, Owner: owner(r.OwnerScope, r.TenantID), CurrentVersionID: r.CurrentVersionID, LatestVersionNo: r.LatestVersionNo,
		RowVersion: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ClonedFromID: r.ClonedFromID, ClonedFromTenantID: r.ClonedFromTenantID,
		ClonedAt: r.ClonedAt, ClonedBy: r.ClonedBy}
}

// QuestionVersionOf maps an ent version (objectives loaded separately).
func QuestionVersionOf(r *ent.QuestionVersion) dto.QuestionVersion {
	v := dto.QuestionVersion{ID: r.ID, QuestionID: r.QuestionID, VersionNo: r.VersionNo, Status: contentmodel.Status(r.Status), Type: r.Type,
		Stem: r.Stem, RenderPayload: r.RenderPayload, AnswerKey: r.AnswerKey, ScoringRule: r.ScoringRule, Explanation: r.Explanation,
		OptionFeedback: r.OptionFeedback, Hints: r.Hints, WorkedExampleRef: r.WorkedExampleRef, MathAssets: r.MathAssets, MaxScore: r.MaxScore,
		ExamOnly: r.ExamOnly, EfficacyInstrument: r.EfficacyInstrument, Tags: r.Tags, Difficulty: int(r.Difficulty), GradeLevels: r.GradeLevels,
		SubjectID: r.SubjectID, TopicID: r.TopicID, CurriculumVersionID: r.CurriculumVersionID, MediaRefs: r.MediaRefs,
		ContentLanguage: r.ContentLanguage, AuthorID: r.AuthorID, ReviewerID: r.ReviewerID, PublishedAt: r.PublishedAt, ChangeNote: r.ChangeNote,
		BasedOnID: r.BasedOnVersionID, ContentHash: r.ContentHash, Owner: owner(r.OwnerScope, r.TenantID), RowVersion: r.Version, UpdatedAt: r.UpdatedAt, TranslationStatus: r.TranslationStatus, TranslationMeta: r.TranslationMeta, AudioExemptReason: r.AudioExemptReason,
		AudioExemptApprovedBy: r.AudioExemptApprovedBy}
	if r.MinResponseMs != nil {
		m := int(*r.MinResponseMs)
		v.MinResponseMS = &m
	}
	return v
}

// present reports a JSON value that is neither absent nor null.
func present(v json.RawMessage) bool { return len(v) > 0 && string(v) != "null" }

// InsertQuestion implements write.QuestionStore.
func (QuestionRepo) InsertQuestion(ctx context.Context, q dto.Question) error {
	c := client(ctx).Question.Create().SetID(q.ID).SetOwnerScope(string(q.Owner.Scope)).SetLatestVersionNo(q.LatestVersionNo).
		SetNillableClonedFromID(q.ClonedFromID).SetNillableClonedFromTenantID(q.ClonedFromTenantID).SetNillableClonedAt(q.ClonedAt).
		SetNillableClonedBy(q.ClonedBy)
	if q.Owner.TenantID != nil {
		c.SetTenantID(*q.Owner.TenantID)
	}
	return MapCurriculumErr(c.Exec(ctx))
}

// Question implements write.QuestionStore.
func (QuestionRepo) Question(ctx context.Context, id uuid.UUID) (dto.Question, error) {
	r, err := client(ctx).Question.Query().Where(entq.ID(id)).Only(ctx)
	if err != nil {
		return dto.Question{}, MapCurriculumErr(err)
	}
	return QuestionOf(r), nil
}

// UpdateQuestion implements write.QuestionStore.
func (s QuestionRepo) UpdateQuestion(ctx context.Context, q dto.Question, ifRow int64) (dto.Question, error) {
	u := client(ctx).Question.Update().Where(entq.ID(q.ID), entq.Version(ifRow)).SetLatestVersionNo(q.LatestVersionNo).AddVersion(1)
	if q.CurrentVersionID != nil {
		u.SetCurrentVersionID(*q.CurrentVersionID)
	} else {
		u.ClearCurrentVersionID()
	}
	n, err := u.Save(ctx)
	if err != nil {
		return q, MapCurriculumErr(err)
	}
	if n == 0 {
		return q, mismatchOrMissing(client(ctx).Question.Query().Where(entq.ID(q.ID)).Exist(ctx))
	}
	return s.Question(ctx, q.ID)
}

// SoftDeleteQuestion implements write.QuestionStore.
func (QuestionRepo) SoftDeleteQuestion(ctx context.Context, id uuid.UUID, reason string) error {
	n, err := client(ctx).Question.Delete().Where(entq.ID(id)).Exec(entx.WithDeleteReason(ctx, reason))
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return problem.New(problem.NotFound, "")
	}
	return nil
}

func (QuestionRepo) setObjectives(ctx context.Context, v dto.QuestionVersion) error {
	c := client(ctx)
	have, err := c.QuestionVersionObjective.Query().Where(entqo.QuestionVersionID(v.ID)).All(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	kept := map[uuid.UUID]bool{}
	for _, h := range have {
		if slices.Contains(v.ObjectiveIDs, h.ObjectiveID) {
			kept[h.ObjectiveID] = true
			continue
		}
		if _, err := c.QuestionVersionObjective.Delete().Where(entqo.ID(h.ID)).Exec(entx.WithDeleteReason(ctx, "objective_removed")); err != nil {
			return MapCurriculumErr(err)
		}
	}
	for _, lo := range v.ObjectiveIDs {
		if kept[lo] {
			continue
		}
		b := c.QuestionVersionObjective.Create().SetID(ids.New()).SetQuestionID(v.QuestionID).SetQuestionVersionID(v.ID).SetObjectiveID(lo).
			SetOwnerScope(string(v.Owner.Scope))
		if v.Owner.TenantID != nil {
			b.SetTenantID(*v.Owner.TenantID)
		}
		if err := b.Exec(ctx); err != nil {
			return MapCurriculumErr(err)
		}
	}
	return nil
}

func minResponse(v *int) *int32 {
	if v == nil {
		return nil
	}
	x := int32(*v) //nolint:gosec // 0–600000 (contract, CHECK)
	return &x
}

// InsertQuestionVersion implements write.QuestionStore.
func (s QuestionRepo) InsertQuestionVersion(ctx context.Context, v dto.QuestionVersion) error {
	c := client(ctx).QuestionVersion.Create().SetID(v.ID).SetQuestionID(v.QuestionID).SetVersionNo(v.VersionNo).SetStatus(string(v.Status)).
		SetType(v.Type).SetStem(v.Stem).SetRenderPayload(v.RenderPayload).SetAnswerKey(v.AnswerKey).SetScoringRule(v.ScoringRule).
		SetNillableMinResponseMs(minResponse(v.MinResponseMS)).SetMaxScore(v.MaxScore).SetExamOnly(v.ExamOnly).SetEfficacyInstrument(v.EfficacyInstrument).
		SetTags(nonNilTags(v.Tags)).SetDifficulty(int16(v.Difficulty)).SetGradeLevels(v.GradeLevels). //nolint:gosec // 1–5
		SetSubjectID(v.SubjectID).SetTopicID(v.TopicID).SetCurriculumVersionID(v.CurriculumVersionID).SetMediaRefs(idList(v.MediaRefs)).
		SetContentLanguage(v.ContentLanguage).SetAuthorID(v.AuthorID).SetNillableChangeNote(v.ChangeNote).SetNillableBasedOnVersionID(v.BasedOnID).
		SetContentHash(v.ContentHash).SetOwnerScope(string(v.Owner.Scope)).SetNillableTranslationStatus(v.TranslationStatus).
		SetNillableAudioExemptReason(v.AudioExemptReason)
	if present(v.TranslationMeta) {
		c.SetTranslationMeta(v.TranslationMeta)
	}
	if v.Owner.TenantID != nil {
		c.SetTenantID(*v.Owner.TenantID)
	}
	// Optional JSON columns stay SQL NULL when absent (a JSON null would break the CHECKs on jsonb_typeof).
	for _, f := range []struct {
		val json.RawMessage
		set func(json.RawMessage) *ent.QuestionVersionCreate
	}{{v.Explanation, c.SetExplanation}, {v.OptionFeedback, c.SetOptionFeedback}, {v.Hints, c.SetHints},
		{v.WorkedExampleRef, c.SetWorkedExampleRef}, {v.MathAssets, c.SetMathAssets}} {
		if present(f.val) {
			f.set(f.val)
		}
	}
	if err := c.Exec(ctx); err != nil {
		return MapCurriculumErr(err)
	}
	return s.setObjectives(ctx, v)
}

func nonNilTags(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}

func (QuestionRepo) objectives(ctx context.Context, vs []dto.QuestionVersion) error {
	if len(vs) == 0 {
		return nil
	}
	vids := make([]uuid.UUID, len(vs))
	for i, v := range vs {
		vids[i] = v.ID
	}
	rows, err := client(ctx).QuestionVersionObjective.Query().Where(entqo.QuestionVersionIDIn(vids...)).Order(ent.Asc(entqo.FieldObjectiveID)).All(ctx)
	if err != nil {
		return MapCurriculumErr(err)
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

// QuestionVersion implements write.QuestionStore.
func (s QuestionRepo) QuestionVersion(ctx context.Context, questionID uuid.UUID, versionNo int) (dto.QuestionVersion, error) {
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.QuestionID(questionID), entqv.VersionNo(versionNo)).Only(ctx)
	if err != nil {
		return dto.QuestionVersion{}, MapCurriculumErr(err)
	}
	vs := []dto.QuestionVersion{QuestionVersionOf(r)}
	return vs[0], s.objectivesInto(ctx, vs)
}

func (s QuestionRepo) objectivesInto(ctx context.Context, vs []dto.QuestionVersion) error {
	return s.objectives(ctx, vs)
}

// QuestionVersionByID implements write.QuestionStore.
func (s QuestionRepo) QuestionVersionByID(ctx context.Context, id uuid.UUID) (dto.QuestionVersion, error) {
	r, err := client(ctx).QuestionVersion.Query().Where(entqv.ID(id)).Only(ctx)
	if err != nil {
		return dto.QuestionVersion{}, MapCurriculumErr(err)
	}
	vs := []dto.QuestionVersion{QuestionVersionOf(r)}
	return vs[0], s.objectivesInto(ctx, vs)
}

// QuestionVersions implements write.QuestionStore (newest first).
func (s QuestionRepo) QuestionVersions(ctx context.Context, questionID uuid.UUID) ([]dto.QuestionVersion, error) {
	rows, err := client(ctx).QuestionVersion.Query().Where(entqv.QuestionID(questionID)).Order(ent.Desc(entqv.FieldVersionNo)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]dto.QuestionVersion, len(rows))
	for i, r := range rows {
		out[i] = QuestionVersionOf(r)
	}
	return out, s.objectives(ctx, out)
}

// UpdateQuestionVersion implements write.QuestionStore.
func (s QuestionRepo) UpdateQuestionVersion(ctx context.Context, v dto.QuestionVersion, ifRow int64) (dto.QuestionVersion, error) {
	u := client(ctx).QuestionVersion.Update().Where(entqv.ID(v.ID), entqv.Version(ifRow)).SetType(v.Type).SetStem(v.Stem).
		SetRenderPayload(v.RenderPayload).SetAnswerKey(v.AnswerKey).SetScoringRule(v.ScoringRule).SetMaxScore(v.MaxScore).SetExamOnly(v.ExamOnly).
		SetEfficacyInstrument(v.EfficacyInstrument).SetTags(nonNilTags(v.Tags)).SetDifficulty(int16(v.Difficulty)). //nolint:gosec // 1–5
		SetGradeLevels(v.GradeLevels).SetSubjectID(v.SubjectID).SetTopicID(v.TopicID).SetCurriculumVersionID(v.CurriculumVersionID).
		SetMediaRefs(idList(v.MediaRefs)).SetContentLanguage(v.ContentLanguage).SetContentHash(v.ContentHash).AddVersion(1)
	for _, f := range []struct {
		val  json.RawMessage
		set  func(json.RawMessage) *ent.QuestionVersionUpdate
		clr  func() *ent.QuestionVersionUpdate
		name string
	}{{v.Explanation, u.SetExplanation, u.ClearExplanation, "explanation"}, {v.OptionFeedback, u.SetOptionFeedback, u.ClearOptionFeedback, "feedback"},
		{v.Hints, u.SetHints, u.ClearHints, "hints"}, {v.WorkedExampleRef, u.SetWorkedExampleRef, u.ClearWorkedExampleRef, "worked"},
		{v.MathAssets, u.SetMathAssets, u.ClearMathAssets, "math"}} {
		if present(f.val) {
			f.set(f.val)
		} else {
			f.clr()
		}
	}
	if v.MinResponseMS != nil {
		u.SetMinResponseMs(*minResponse(v.MinResponseMS))
	} else {
		u.ClearMinResponseMs()
	}
	if v.ChangeNote != nil {
		u.SetChangeNote(*v.ChangeNote)
	} else {
		u.ClearChangeNote()
	}
	setWorkflowDraft(v.TranslationStatus, v.TranslationMeta, v.AudioExemptReason, u.SetTranslationStatus, u.ClearTranslationStatus,
		u.SetTranslationMeta, u.ClearTranslationMeta, u.SetAudioExemptReason, u.ClearAudioExemptReason)
	n, err := u.Save(ctx)
	if err != nil {
		return v, MapCurriculumErr(err)
	}
	if n == 0 {
		return v, mismatchOrMissing(client(ctx).QuestionVersion.Query().Where(entqv.ID(v.ID)).Exist(ctx))
	}
	if err := s.setObjectives(ctx, v); err != nil {
		return v, err
	}
	return s.QuestionVersion(ctx, v.QuestionID, v.VersionNo)
}

func (QuestionRepo) entry(r *ent.Question) trash.Entry {
	e := trash.Entry{ResourceType: contentmodel.ResourceQuestion, ResourceID: r.ID, TenantID: r.TenantID, Title: "question " + r.ID.String(),
		DeletedBy: r.DeletedBy}
	if r.DeletedAt != nil {
		e.DeletedAt = *r.DeletedAt
	}
	if r.DeleteReason != nil {
		e.DeleteReason = *r.DeleteReason
	}
	return e
}

// DeletedQuestions implements write.QuestionStore.
func (s QuestionRepo) DeletedQuestions(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error) {
	q := client(ctx).Question.Query().Where(entq.DeletedAtNotNil())
	if len(f.TenantIDs) > 0 {
		if f.PlatformToo {
			q = q.Where(entq.Or(entq.TenantIDIn(f.TenantIDs...), entq.TenantIDIsNil()))
		} else {
			q = q.Where(entq.TenantIDIn(f.TenantIDs...))
		}
	}
	if f.AfterAt != nil {
		q = q.Where(entq.Or(entq.DeletedAtLT(*f.AfterAt), entq.And(entq.DeletedAt(*f.AfterAt), entq.IDLT(f.AfterID))))
	}
	rows, err := q.Order(entq.ByDeletedAt(entsql.OrderDesc()), entq.ByID(entsql.OrderDesc())).Limit(f.Limit + 1).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]trash.Entry, len(rows))
	for i, r := range rows {
		out[i] = s.entry(r)
	}
	slices.SortStableFunc(out, func(a, b trash.Entry) int {
		if c := b.DeletedAt.Compare(a.DeletedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ResourceID.String(), a.ResourceID.String())
	})
	return out, nil
}

// RestoreQuestion implements write.QuestionStore (questions have no unique business key: no conflict).
func (s QuestionRepo) RestoreQuestion(ctx context.Context, id uuid.UUID) (trash.Entry, error) {
	c := client(ctx)
	r, err := c.Question.Query().Where(entq.ID(id)).Only(ctx)
	if err != nil {
		return trash.Entry{}, MapCurriculumErr(err)
	}
	e := s.entry(r)
	if r.DeletedAt == nil {
		return e, trash.ErrNotDeleted
	}
	return e, MapCurriculumErr(c.Question.Update().Where(entq.ID(id)).ClearDeletedAt().ClearDeletedBy().ClearDeleteReason().Exec(ctx))
}
