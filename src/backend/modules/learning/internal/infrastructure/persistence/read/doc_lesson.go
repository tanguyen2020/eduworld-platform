package read

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entdi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/lesson"
	entdv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/lessonversion"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/predicate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// lessonModel reads content.lessons / content.lesson_versions. doc_question_set.go, doc_learning_material.go and
// doc_assessment_blueprint.go are copies with the type names replaced (same columns).
type lessonModel struct{}

func lessonDocOf(r *ent.Lesson) dto.Doc {
	return dto.Doc{Type: contentmodel.TypeLesson, ID: r.ID, Owner: ownerOf(r.OwnerScope, r.TenantID), CurrentVersionID: r.CurrentVersionID,
		LatestVersionNo: r.LatestVersionNo, RowVersion: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ClonedFromID: r.ClonedFromID,
		ClonedFromTenantID: r.ClonedFromTenantID, ClonedAt: r.ClonedAt, ClonedBy: r.ClonedBy}
}

func lessonVersionOf(r *ent.LessonVersion) dto.DocVersion {
	return dto.DocVersion{Type: contentmodel.TypeLesson, ID: r.ID, ItemID: r.LessonID, VersionNo: r.VersionNo, Status: contentmodel.Status(r.Status),
		Title: r.Title, Summary: r.Summary, SubjectID: r.SubjectID, CurriculumVersionID: r.CurriculumVersionID, GradeLevels: r.GradeLevels,
		TopicIDs: r.TopicIds, ObjectiveIDs: r.ObjectiveIds, MediaRefs: r.MediaRefs, WorkedExampleIDs: r.WorkedExampleIds, Body: r.Body,
		ContentLanguage: r.ContentLanguage, AuthorID: r.AuthorID, ReviewerID: r.ReviewerID, PublishedAt: r.PublishedAt, ChangeNote: r.ChangeNote,
		BasedOnID: r.BasedOnVersionID, Owner: ownerOf(r.OwnerScope, r.TenantID), RowVersion: r.Version, UpdatedAt: r.UpdatedAt, TranslationStatus: r.TranslationStatus, TranslationMeta: r.TranslationMeta, AudioExemptReason: r.AudioExemptReason,
		AudioExemptApprovedBy: r.AudioExemptApprovedBy, PublishedBy: r.PublishedBy, SnapshotSHA256: r.SnapshotSha256}
}

func (lessonModel) doc(ctx context.Context, id uuid.UUID) (dto.Doc, error) {
	r, err := client(ctx).Lesson.Query().Where(entdi.ID(id)).Only(ctx)
	if err != nil {
		return dto.Doc{}, notFound(err)
	}
	return lessonDocOf(r), nil
}

func (lessonModel) version(ctx context.Context, itemID uuid.UUID, no int) (dto.DocVersion, error) {
	q := client(ctx).LessonVersion.Query().Where(entdv.LessonID(itemID))
	if no > 0 {
		q = q.Where(entdv.VersionNo(no))
	}
	r, err := q.Order(ent.Desc(entdv.FieldVersionNo)).First(ctx)
	if err != nil {
		return dto.DocVersion{}, notFound(err)
	}
	return lessonVersionOf(r), nil
}

// search returns the newest version of every visible item matching f, newest item first.
func (lessonModel) search(ctx context.Context, f read.DocFilter) ([]read.DocHit, error) {
	ps := []predicate.LessonVersion{func(s *entsql.Selector) {
		s.Where(latestVersion(s, entdv.FieldVersionNo, entdv.FieldLessonID, entdi.Table, entdi.FieldID))
	}}
	ps = append(ps, docFilters[predicate.LessonVersion](f)...)
	if f.SubjectID != nil {
		ps = append(ps, entdv.SubjectID(*f.SubjectID))
	}
	if f.Status != "" {
		ps = append(ps, entdv.Status(f.Status))
	}
	if f.After != nil {
		ps = append(ps, entdv.LessonIDLT(*f.After))
	}
	rows, err := client(ctx).LessonVersion.Query().Where(ps...).Order(ent.Desc(entdv.FieldLessonID)).Limit(f.Limit).All(ctx)
	if err != nil {
		return nil, err
	}
	idList := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		idList[i] = r.LessonID
	}
	items, err := client(ctx).Lesson.Query().Where(entdi.IDIn(idList...)).All(ctx)
	if err != nil {
		return nil, err
	}
	by := map[uuid.UUID]*ent.Lesson{}
	for _, it := range items {
		by[it.ID] = it
	}
	out := make([]read.DocHit, 0, len(rows))
	for _, r := range rows {
		if it, ok := by[r.LessonID]; ok {
			out = append(out, read.DocHit{Doc: lessonDocOf(it), Version: lessonVersionOf(r)})
		}
	}
	return out, nil
}
