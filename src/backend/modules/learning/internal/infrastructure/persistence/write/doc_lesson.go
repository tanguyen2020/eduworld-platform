package write

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entdi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/lesson"
	entdv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/lessonversion"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
)

// lessonTable is the docTable of content.lessons / content.lesson_versions. doc_question_set.go,
// doc_learning_material.go and doc_assessment_blueprint.go are copies with the type names replaced (same columns).
type lessonTable struct{}

func lessonDocOf(r *ent.Lesson) dto.Doc {
	return dto.Doc{Type: contentmodel.TypeLesson, ID: r.ID, Owner: owner(r.OwnerScope, r.TenantID), CurrentVersionID: r.CurrentVersionID,
		LatestVersionNo: r.LatestVersionNo, RowVersion: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ClonedFromID: r.ClonedFromID,
		ClonedFromTenantID: r.ClonedFromTenantID, ClonedAt: r.ClonedAt, ClonedBy: r.ClonedBy}
}

func lessonVersionOf(r *ent.LessonVersion) dto.DocVersion {
	return dto.DocVersion{Type: contentmodel.TypeLesson, ID: r.ID, ItemID: r.LessonID, VersionNo: r.VersionNo, Status: contentmodel.Status(r.Status),
		Title: r.Title, Summary: r.Summary, SubjectID: r.SubjectID, CurriculumVersionID: r.CurriculumVersionID, GradeLevels: r.GradeLevels,
		TopicIDs: r.TopicIds, ObjectiveIDs: r.ObjectiveIds, MediaRefs: r.MediaRefs, WorkedExampleIDs: r.WorkedExampleIds, Body: r.Body,
		ContentLanguage: r.ContentLanguage, AuthorID: r.AuthorID, ReviewerID: r.ReviewerID, PublishedAt: r.PublishedAt, ChangeNote: r.ChangeNote,
		BasedOnID: r.BasedOnVersionID, Owner: owner(r.OwnerScope, r.TenantID), RowVersion: r.Version, UpdatedAt: r.UpdatedAt, TranslationStatus: r.TranslationStatus, TranslationMeta: r.TranslationMeta, AudioExemptReason: r.AudioExemptReason,
		AudioExemptApprovedBy: r.AudioExemptApprovedBy}
}

func (lessonTable) insertDoc(ctx context.Context, d dto.Doc) error {
	c := client(ctx).Lesson.Create().SetID(d.ID).SetOwnerScope(string(d.Owner.Scope)).SetLatestVersionNo(d.LatestVersionNo).
		SetNillableClonedFromID(d.ClonedFromID).SetNillableClonedFromTenantID(d.ClonedFromTenantID).SetNillableClonedAt(d.ClonedAt).
		SetNillableClonedBy(d.ClonedBy)
	if d.Owner.TenantID != nil {
		c.SetTenantID(*d.Owner.TenantID)
	}
	return c.Exec(ctx)
}

func (lessonTable) doc(ctx context.Context, id uuid.UUID) (dto.Doc, error) {
	r, err := client(ctx).Lesson.Query().Where(entdi.ID(id)).Only(ctx)
	if err != nil {
		return dto.Doc{}, err
	}
	return lessonDocOf(r), nil
}

func (lessonTable) docExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return client(ctx).Lesson.Query().Where(entdi.ID(id)).Exist(ctx)
}

func (lessonTable) updateDoc(ctx context.Context, d dto.Doc, ifRow int64) (int, error) {
	u := client(ctx).Lesson.Update().Where(entdi.ID(d.ID), entdi.Version(ifRow)).SetLatestVersionNo(d.LatestVersionNo).AddVersion(1)
	if d.CurrentVersionID != nil {
		u.SetCurrentVersionID(*d.CurrentVersionID)
	} else {
		u.ClearCurrentVersionID()
	}
	return u.Save(ctx)
}

func (lessonTable) softDelete(ctx context.Context, id uuid.UUID, _ string) (int, error) {
	return client(ctx).Lesson.Delete().Where(entdi.ID(id)).Exec(ctx)
}

func (lessonTable) insertVersion(ctx context.Context, v dto.DocVersion) error {
	c := client(ctx).LessonVersion.Create().SetID(v.ID).SetLessonID(v.ItemID).SetVersionNo(v.VersionNo).SetStatus(string(v.Status)).
		SetTitle(v.Title).SetNillableSubjectID(v.SubjectID).SetNillableCurriculumVersionID(v.CurriculumVersionID).SetGradeLevels(ints(v.GradeLevels)).
		SetTopicIds(idList(v.TopicIDs)).SetObjectiveIds(idList(v.ObjectiveIDs)).SetMediaRefs(idList(v.MediaRefs)).
		SetWorkedExampleIds(strs(v.WorkedExampleIDs)).SetBody(jsonOr(v.Body, "{}")).SetContentLanguage(v.ContentLanguage).SetAuthorID(v.AuthorID).
		SetNillableChangeNote(v.ChangeNote).SetNillableBasedOnVersionID(v.BasedOnID).SetOwnerScope(string(v.Owner.Scope)).
		SetNillableTranslationStatus(v.TranslationStatus).SetNillableAudioExemptReason(v.AudioExemptReason)
	if present(v.TranslationMeta) {
		c.SetTranslationMeta(v.TranslationMeta)
	}
	if present(v.Summary) {
		c.SetSummary(v.Summary)
	}
	if v.Owner.TenantID != nil {
		c.SetTenantID(*v.Owner.TenantID)
	}
	return c.Exec(ctx)
}

func (lessonTable) version(ctx context.Context, itemID uuid.UUID, no int) (dto.DocVersion, error) {
	r, err := client(ctx).LessonVersion.Query().Where(entdv.LessonID(itemID), entdv.VersionNo(no)).Only(ctx)
	if err != nil {
		return dto.DocVersion{}, err
	}
	return lessonVersionOf(r), nil
}

func (lessonTable) versionExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return client(ctx).LessonVersion.Query().Where(entdv.ID(id)).Exist(ctx)
}

func (lessonTable) versions(ctx context.Context, itemID uuid.UUID) ([]dto.DocVersion, error) {
	rows, err := client(ctx).LessonVersion.Query().Where(entdv.LessonID(itemID)).Order(ent.Desc(entdv.FieldVersionNo)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DocVersion, len(rows))
	for i, r := range rows {
		out[i] = lessonVersionOf(r)
	}
	return out, nil
}

func (lessonTable) updateVersion(ctx context.Context, v dto.DocVersion, ifRow int64) (int, error) {
	u := client(ctx).LessonVersion.Update().Where(entdv.ID(v.ID), entdv.Version(ifRow)).SetTitle(v.Title).SetGradeLevels(ints(v.GradeLevels)).
		SetTopicIds(idList(v.TopicIDs)).SetObjectiveIds(idList(v.ObjectiveIDs)).SetMediaRefs(idList(v.MediaRefs)).
		SetWorkedExampleIds(strs(v.WorkedExampleIDs)).SetBody(jsonOr(v.Body, "{}")).SetContentLanguage(v.ContentLanguage).AddVersion(1)
	if present(v.Summary) {
		u.SetSummary(v.Summary)
	} else {
		u.ClearSummary()
	}
	if v.SubjectID != nil {
		u.SetSubjectID(*v.SubjectID)
	} else {
		u.ClearSubjectID()
	}
	if v.CurriculumVersionID != nil {
		u.SetCurriculumVersionID(*v.CurriculumVersionID)
	} else {
		u.ClearCurriculumVersionID()
	}
	if v.ChangeNote != nil {
		u.SetChangeNote(*v.ChangeNote)
	} else {
		u.ClearChangeNote()
	}
	setWorkflowDraft(v.TranslationStatus, v.TranslationMeta, v.AudioExemptReason, u.SetTranslationStatus, u.ClearTranslationStatus,
		u.SetTranslationMeta, u.ClearTranslationMeta, u.SetAudioExemptReason, u.ClearAudioExemptReason)
	return u.Save(ctx)
}

func (lessonTable) entry(r *ent.Lesson) trash.Entry {
	e := trash.Entry{ResourceType: contentmodel.ResourceType(contentmodel.TypeLesson), ResourceID: r.ID, TenantID: r.TenantID,
		Title: string(contentmodel.TypeLesson) + " " + r.ID.String(), DeletedBy: r.DeletedBy}
	if r.DeletedAt != nil {
		e.DeletedAt = *r.DeletedAt
	}
	if r.DeleteReason != nil {
		e.DeleteReason = *r.DeleteReason
	}
	return e
}

func (s lessonTable) deleted(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error) {
	q := client(ctx).Lesson.Query().Where(entdi.DeletedAtNotNil())
	if len(f.TenantIDs) > 0 {
		if f.PlatformToo {
			q = q.Where(entdi.Or(entdi.TenantIDIn(f.TenantIDs...), entdi.TenantIDIsNil()))
		} else {
			q = q.Where(entdi.TenantIDIn(f.TenantIDs...))
		}
	}
	if f.AfterAt != nil {
		q = q.Where(entdi.Or(entdi.DeletedAtLT(*f.AfterAt), entdi.And(entdi.DeletedAt(*f.AfterAt), entdi.IDLT(f.AfterID))))
	}
	rows, err := q.Order(entdi.ByDeletedAt(entsql.OrderDesc()), entdi.ByID(entsql.OrderDesc())).Limit(f.Limit + 1).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]trash.Entry, len(rows))
	for i, r := range rows {
		out[i] = s.entry(r)
	}
	return out, nil
}

func (s lessonTable) restore(ctx context.Context, id uuid.UUID) (trash.Entry, error) {
	c := client(ctx)
	r, err := c.Lesson.Query().Where(entdi.ID(id)).Only(ctx)
	if err != nil {
		return trash.Entry{}, err
	}
	e := s.entry(r)
	if r.DeletedAt == nil {
		return e, trash.ErrNotDeleted
	}
	return e, c.Lesson.Update().Where(entdi.ID(id)).ClearDeletedAt().ClearDeletedBy().ClearDeleteReason().Exec(ctx)
}
