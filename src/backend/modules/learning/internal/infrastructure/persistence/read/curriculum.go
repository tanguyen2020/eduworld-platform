package read

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entcomp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/competency"
	entcur "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/curriculum"
	entver "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/curriculumversion"
	entimp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/importjob"
	entlo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/learningobjective"
	entmap "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/loversionmapping"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/predicate"
	entsubj "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/subject"
	entsg "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/subjectgrade"
	enttb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/textbook"
	entsec "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/textbooksection"
	entsecobj "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/textbooksectionobjective"
	entseries "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/textbookseries"
	enttopic "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/topic"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// CurriculumModel implements read.CurriculumReadModel with ent in the read session of ctx (entx filter: tenant rows
// and platform rows assigned to the tenant, live rows only).
type CurriculumModel struct{}

var _ read.CurriculumReadModel = CurriculumModel{}

func notFound(err error) error {
	if ent.IsNotFound(err) {
		return problem.New(problem.NotFound, "").Wrap(err)
	}
	return err
}

func ownerOf(scope string, tenant *uuid.UUID) contentmodel.Owner {
	return contentmodel.Owner{Scope: contentmodel.OwnerScope(scope), TenantID: tenant}
}

func versionOf(r *ent.CurriculumVersion) dto.CurriculumVersion {
	return dto.CurriculumVersion{ID: r.ID, CurriculumID: r.CurriculumID, VersionNo: r.VersionNo, Label: r.VersionLabel,
		Status: contentmodel.Status(r.Status), EffectiveFrom: r.EffectiveFrom, EffectiveTo: r.EffectiveTo, SupersedesID: r.SupersedesVersionID,
		BasedOnID: r.BasedOnVersionID, ChangeNote: r.ChangeNote, ContentLanguage: r.ContentLanguage, AuthorID: r.AuthorID,
		ReviewerID: r.ReviewerID, PublishedAt: r.PublishedAt, Owner: ownerOf(r.OwnerScope, r.TenantID), RowVersion: r.Version, UpdatedAt: r.UpdatedAt}
}

func refOf(v dto.CurriculumVersion) *dto.VersionRef {
	return &dto.VersionRef{ID: v.ID, VersionNo: v.VersionNo, Label: v.Label, Status: v.Status}
}

// Curricula implements read.CurriculumReadModel.
func (m CurriculumModel) Curricula(ctx context.Context, ownerScope string) ([]dto.Curriculum, error) {
	q := client(ctx).Curriculum.Query()
	if ownerScope != "" {
		q = q.Where(entcur.OwnerScope(ownerScope))
	}
	rows, err := q.Order(ent.Asc(entcur.FieldCode), ent.Asc(entcur.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	idList := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		idList[i] = r.ID
	}
	vs, err := client(ctx).CurriculumVersion.Query().Where(entver.CurriculumIDIn(idList...)).All(ctx)
	if err != nil {
		return nil, err
	}
	byCur := map[uuid.UUID][]dto.CurriculumVersion{}
	for _, v := range vs {
		byCur[v.CurriculumID] = append(byCur[v.CurriculumID], versionOf(v))
	}
	out := make([]dto.Curriculum, len(rows))
	for i, r := range rows {
		out[i] = withRefs(curriculumOf(r), byCur[r.ID])
	}
	return out, nil
}

func curriculumOf(r *ent.Curriculum) dto.Curriculum {
	return dto.Curriculum{ID: r.ID, Code: r.Code, Name: r.Name, Framework: r.Framework, Owner: ownerOf(r.OwnerScope, r.TenantID),
		CurrentVersionID: r.CurrentVersionID, LatestVersionNo: r.LatestVersionNo, RowVersion: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func withRefs(c dto.Curriculum, vs []dto.CurriculumVersion) dto.Curriculum {
	for _, v := range vs {
		if c.Latest == nil || v.VersionNo > c.Latest.VersionNo {
			c.Latest = refOf(v)
		}
		if c.CurrentVersionID != nil && *c.CurrentVersionID == v.ID {
			c.Current = refOf(v)
		}
	}
	return c
}

// Curriculum implements read.CurriculumReadModel.
func (m CurriculumModel) Curriculum(ctx context.Context, id uuid.UUID) (dto.Curriculum, error) {
	r, err := client(ctx).Curriculum.Query().Where(entcur.ID(id)).Only(ctx)
	if err != nil {
		return dto.Curriculum{}, notFound(err)
	}
	vs, err := m.versions(ctx, entver.CurriculumID(id))
	if err != nil {
		return dto.Curriculum{}, err
	}
	return withRefs(curriculumOf(r), vs), nil
}

func (m CurriculumModel) versions(ctx context.Context, ps ...predicate.CurriculumVersion) ([]dto.CurriculumVersion, error) {
	rows, err := client(ctx).CurriculumVersion.Query().Where(ps...).Order(ent.Desc(entver.FieldVersionNo)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.CurriculumVersion, len(rows))
	versionIDs := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = versionOf(r)
		versionIDs[i] = r.ID
	}
	sgs, err := client(ctx).SubjectGrade.Query().Where(entsg.CurriculumVersionIDIn(versionIDs...)).
		Order(ent.Asc(entsg.FieldGradeLevel), ent.Asc(entsg.FieldSubjectID)).All(ctx)
	if err != nil {
		return nil, err
	}
	byVersion := map[uuid.UUID][]dto.SubjectGrade{}
	for _, sg := range sgs {
		var periods *int
		if sg.PeriodsPerYear != nil {
			p := int(*sg.PeriodsPerYear)
			periods = &p
		}
		byVersion[sg.CurriculumVersionID] = append(byVersion[sg.CurriculumVersionID],
			dto.SubjectGrade{ID: sg.ID, SubjectID: sg.SubjectID, GradeLevel: int(sg.GradeLevel), PeriodsPerYear: periods})
	}
	for i := range out {
		out[i].SubjectGrades = byVersion[out[i].ID]
	}
	return out, nil
}

// liveCurricula keeps the versions whose curriculum is visible and live.
func (m CurriculumModel) liveCurricula(ctx context.Context, vs []dto.CurriculumVersion) ([]dto.CurriculumVersion, map[uuid.UUID]*ent.Curriculum, error) {
	ids := map[uuid.UUID]bool{}
	var list []uuid.UUID
	for _, v := range vs {
		if !ids[v.CurriculumID] {
			ids[v.CurriculumID] = true
			list = append(list, v.CurriculumID)
		}
	}
	cs, err := client(ctx).Curriculum.Query().Where(entcur.IDIn(list...)).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	byID := map[uuid.UUID]*ent.Curriculum{}
	for _, c := range cs {
		byID[c.ID] = c
	}
	out := vs[:0:0]
	for _, v := range vs {
		if c, ok := byID[v.CurriculumID]; ok {
			v.IsCurrent = c.CurrentVersionID != nil && *c.CurrentVersionID == v.ID
			out = append(out, v)
		}
	}
	return out, byID, nil
}

// Versions implements read.CurriculumReadModel.
func (m CurriculumModel) Versions(ctx context.Context, curriculumID uuid.UUID) ([]dto.CurriculumVersion, error) {
	if _, err := m.Curriculum(ctx, curriculumID); err != nil {
		return nil, err
	}
	vs, err := m.versions(ctx, entver.CurriculumID(curriculumID))
	if err != nil {
		return nil, err
	}
	vs, _, err = m.liveCurricula(ctx, vs)
	return vs, err
}

// Version implements read.CurriculumReadModel.
func (m CurriculumModel) Version(ctx context.Context, curriculumID uuid.UUID, versionNo int) (dto.CurriculumVersion, error) {
	vs, err := m.versions(ctx, entver.CurriculumID(curriculumID), entver.VersionNo(versionNo))
	if err != nil {
		return dto.CurriculumVersion{}, err
	}
	if vs, _, err = m.liveCurricula(ctx, vs); err != nil {
		return dto.CurriculumVersion{}, err
	}
	if len(vs) == 0 {
		return dto.CurriculumVersion{}, problem.New(problem.NotFound, "")
	}
	return vs[0], nil
}

// VersionsByID implements read.CurriculumReadModel.
func (m CurriculumModel) VersionsByID(ctx context.Context, idList []uuid.UUID) ([]dto.CurriculumVersion, error) {
	vs, err := m.versions(ctx, entver.IDIn(idList...))
	if err != nil {
		return nil, err
	}
	vs, _, err = m.liveCurricula(ctx, vs)
	return vs, err
}

// Subjects implements read.CurriculumReadModel.
func (CurriculumModel) Subjects(ctx context.Context, gradeLevel *int) ([]dto.Subject, error) {
	rows, err := client(ctx).Subject.Query().Order(ent.Asc(entsubj.FieldCode)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Subject, 0, len(rows))
	for _, r := range rows {
		if gradeLevel != nil && !containsInt(r.GradeLevels, *gradeLevel) {
			continue
		}
		out = append(out, dto.Subject{ID: r.ID, Code: r.Code, Name: r.Name, GradeLevels: r.GradeLevels, IconKey: r.IconKey, RowVersion: r.Version})
	}
	return out, nil
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func topicOf(r *ent.Topic) dto.Topic {
	return dto.Topic{ID: r.ID, CurriculumID: r.CurriculumID, VersionID: r.CurriculumVersionID, SubjectID: r.SubjectID, GradeLevel: int(r.GradeLevel),
		ParentID: r.ParentTopicID, Code: r.Code, Name: r.Name, SortOrder: int(r.SortOrder), Owner: ownerOf(r.OwnerScope, r.TenantID), RowVersion: r.Version}
}

// Topics implements read.CurriculumReadModel.
func (CurriculumModel) Topics(ctx context.Context, f read.TopicFilter) ([]dto.Topic, error) {
	q := client(ctx).Topic.Query().Where(enttopic.CurriculumVersionIDIn(f.VersionIDs...))
	if f.SubjectID != nil {
		q = q.Where(enttopic.SubjectID(*f.SubjectID))
	}
	if f.GradeLevel != nil {
		q = q.Where(enttopic.GradeLevel(int16(*f.GradeLevel))) //nolint:gosec // 1–9 (contract)
	}
	rows, err := q.Order(ent.Asc(enttopic.FieldGradeLevel), ent.Asc(enttopic.FieldSortOrder), ent.Asc(enttopic.FieldCode)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Topic, len(rows))
	for i, r := range rows {
		out[i] = topicOf(r)
	}
	return out, nil
}

// Objectives implements read.CurriculumReadModel.
func (m CurriculumModel) Objectives(ctx context.Context, f read.ObjectiveFilter) ([]dto.Objective, error) {
	c := client(ctx)
	q := c.LearningObjective.Query()
	if len(f.VersionIDs) > 0 {
		q = q.Where(entlo.CurriculumVersionIDIn(f.VersionIDs...))
	}
	if f.IDs != nil {
		q = q.Where(entlo.IDIn(f.IDs...))
	}
	if f.TopicID != nil {
		q = q.Where(entlo.TopicID(*f.TopicID))
	}
	if f.YccdRef != nil {
		q = q.Where(entlo.YccdRef(*f.YccdRef))
	}
	if f.SectionID != nil {
		links, err := c.TextbookSectionObjective.Query().Where(entsecobj.SectionID(*f.SectionID)).All(ctx)
		if err != nil {
			return nil, err
		}
		los := make([]uuid.UUID, len(links))
		for i, l := range links {
			los[i] = l.ObjectiveID
		}
		q = q.Where(entlo.IDIn(los...))
	}
	if f.SubjectID != nil || f.GradeLevel != nil {
		tq := c.Topic.Query()
		if len(f.VersionIDs) > 0 {
			tq = tq.Where(enttopic.CurriculumVersionIDIn(f.VersionIDs...))
		}
		if f.SubjectID != nil {
			tq = tq.Where(enttopic.SubjectID(*f.SubjectID))
		}
		if f.GradeLevel != nil {
			tq = tq.Where(enttopic.GradeLevel(int16(*f.GradeLevel))) //nolint:gosec // 1–9 (contract)
		}
		topicIDs, err := tq.IDs(ctx)
		if err != nil {
			return nil, err
		}
		q = q.Where(entlo.TopicIDIn(topicIDs...))
	}
	rows, err := q.Order(ent.Asc(entlo.FieldSortOrder), ent.Asc(entlo.FieldCode), ent.Asc(entlo.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	topicSet := map[uuid.UUID]bool{}
	var topicIDs []uuid.UUID
	for _, r := range rows {
		if !topicSet[r.TopicID] {
			topicSet[r.TopicID] = true
			topicIDs = append(topicIDs, r.TopicID)
		}
	}
	topics, err := c.Topic.Query().Where(enttopic.IDIn(topicIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	byTopic := map[uuid.UUID]*ent.Topic{}
	for _, t := range topics {
		byTopic[t.ID] = t
	}
	out := make([]dto.Objective, len(rows))
	for i, r := range rows {
		o := dto.Objective{ID: r.ID, CurriculumID: r.CurriculumID, VersionID: r.CurriculumVersionID, TopicID: r.TopicID, Code: r.Code,
			StableKey: r.StableKey, Description: r.Description, BloomLevel: r.BloomLevel, YccdRef: r.YccdRef, CompetencyIDs: r.CompetencyIds,
			PrerequisiteIDs: r.PrerequisiteIds, SortOrder: int(r.SortOrder), Owner: ownerOf(r.OwnerScope, r.TenantID), RowVersion: r.Version}
		if t, ok := byTopic[r.TopicID]; ok {
			o.SubjectID, o.GradeLevel = t.SubjectID, int(t.GradeLevel)
		}
		out[i] = o
	}
	return out, nil
}

// Competencies implements read.CurriculumReadModel.
func (CurriculumModel) Competencies(ctx context.Context, versionID uuid.UUID) ([]dto.Competency, error) {
	rows, err := client(ctx).Competency.Query().Where(entcomp.CurriculumVersionID(versionID)).Order(ent.Asc(entcomp.FieldCode)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Competency, len(rows))
	for i, r := range rows {
		out[i] = dto.Competency{ID: r.ID, CurriculumID: r.CurriculumID, VersionID: r.CurriculumVersionID, Code: r.Code, Name: r.Name, Kind: r.Kind,
			SubjectID: r.SubjectID, Owner: ownerOf(r.OwnerScope, r.TenantID), RowVersion: r.Version}
	}
	return out, nil
}

// Mappings implements read.CurriculumReadModel.
func (CurriculumModel) Mappings(ctx context.Context, versionID uuid.UUID) (curriculum.MappingSet, error) {
	rows, err := client(ctx).LoVersionMapping.Query().Where(entmap.CurriculumVersionID(versionID)).Order(ent.Asc(entmap.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(curriculum.MappingSet, len(rows))
	for i, r := range rows {
		out[i] = curriculum.Mapping{FromLO: r.FromLoID, ToLO: r.ToLoID, Relation: r.Relation}
	}
	return out, nil
}

// TextbookSeries implements read.CurriculumReadModel.
func (CurriculumModel) TextbookSeries(ctx context.Context) ([]dto.TextbookSeries, error) {
	rows, err := client(ctx).TextbookSeries.Query().Order(ent.Asc(entseries.FieldCode)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.TextbookSeries, len(rows))
	for i, r := range rows {
		out[i] = dto.TextbookSeries{ID: r.ID, Code: r.Code, Name: r.Name, Publisher: r.Publisher}
	}
	return out, nil
}

// Textbooks implements read.CurriculumReadModel.
func (CurriculumModel) Textbooks(ctx context.Context, f read.TextbookFilter) ([]dto.Textbook, error) {
	q := client(ctx).Textbook.Query()
	if f.SeriesID != nil {
		q = q.Where(enttb.SeriesID(*f.SeriesID))
	}
	if f.SubjectID != nil {
		q = q.Where(enttb.SubjectID(*f.SubjectID))
	}
	if f.GradeLevel != nil {
		q = q.Where(enttb.GradeLevel(int16(*f.GradeLevel))) //nolint:gosec // 1–9 (contract)
	}
	rows, err := q.Order(ent.Asc(enttb.FieldGradeLevel), ent.Asc(enttb.FieldVolume), ent.Asc(enttb.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Textbook, len(rows))
	for i, r := range rows {
		out[i] = dto.Textbook{ID: r.ID, SeriesID: r.SeriesID, SubjectID: r.SubjectID, GradeLevel: int(r.GradeLevel), Volume: int(r.Volume)}
	}
	return out, nil
}

// Sections implements read.CurriculumReadModel: the LOs listed per section are those visible to the scope.
func (CurriculumModel) Sections(ctx context.Context, textbookID uuid.UUID) ([]dto.TextbookSection, error) {
	c := client(ctx)
	rows, err := c.TextbookSection.Query().Where(entsec.TextbookID(textbookID)).Order(ent.Asc(entsec.FieldLessonNo), ent.Asc(entsec.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	secIDs := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		secIDs[i] = r.ID
	}
	links, err := c.TextbookSectionObjective.Query().Where(entsecobj.SectionIDIn(secIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	var loIDs []uuid.UUID
	for _, l := range links {
		loIDs = append(loIDs, l.ObjectiveID)
	}
	visible, err := c.LearningObjective.Query().Where(entlo.IDIn(loIDs...)).IDs(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range visible {
		seen[id] = true
	}
	bySec := map[uuid.UUID][]uuid.UUID{}
	for _, l := range links {
		if seen[l.ObjectiveID] {
			bySec[l.SectionID] = append(bySec[l.SectionID], l.ObjectiveID)
		}
	}
	out := make([]dto.TextbookSection, len(rows))
	for i, r := range rows {
		los := bySec[r.ID]
		sort.Slice(los, func(a, b int) bool { return los[a].String() < los[b].String() })
		out[i] = dto.TextbookSection{ID: r.ID, TextbookID: r.TextbookID, Chapter: r.Chapter, LessonNo: int(r.LessonNo), Title: r.Title,
			PageRange: r.PageRange, ObjectiveIDs: los, RowVersion: r.Version}
	}
	return out, nil
}

// ImportJob implements read.CurriculumReadModel.
func (CurriculumModel) ImportJob(ctx context.Context, id uuid.UUID) (dto.ImportJob, error) {
	r, err := client(ctx).ImportJob.Query().Where(entimp.ID(id)).Only(ctx)
	if err != nil {
		return dto.ImportJob{}, notFound(err)
	}
	j := dto.ImportJob{ID: r.ID, Owner: ownerOf(r.OwnerScope, r.TenantID), Kind: r.Kind, AssetID: r.AssetID, SourceRef: r.SourceRef, DryRun: r.DryRun,
		Status: r.Status, CreatedCount: int(r.CreatedCount), RejectedCount: int(r.RejectedCount), Result: r.ResultRefs,
		IdempotencyKey: r.IdempotencyKey, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, FinishedAt: r.FinishedAt}
	for _, e := range r.Errors {
		x := dto.ImportRowError{}
		if v, ok := e["row"].(float64); ok {
			x.Row = int(v)
		}
		x.Field, _ = e["field"].(string)
		x.Code, _ = e["code"].(string)
		x.Message, _ = e["message"].(string)
		j.Errors = append(j.Errors, x)
	}
	return j, nil
}

// SchoolSettings implements read.SchoolCurriculumSettings with explicit SQL on lms.school_curriculum_settings
// (owner F021 since ADR-0095, created by the F040 migration) and the projection ref.students, in the tenant of the scope.
type SchoolSettings struct{}

var _ read.SchoolCurriculumSettings = SchoolSettings{}

func sessionTenant(ctx context.Context) (uuid.UUID, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return uuid.Nil, entx.ErrMissingTenantScope
	}
	id, single := s.SessionTenantID()
	if !single {
		return uuid.Nil, errors.New("read: school settings need a session tenant")
	}
	return id, nil
}

// settingsTableExists reports whether lms.school_curriculum_settings exists (created by the F040 migration).
func settingsTableExists(ctx context.Context, q querier) (bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT to_regclass('lms.school_curriculum_settings') IS NOT NULL`)
	if err != nil {
		return false, fmt.Errorf("school settings table: %w", err)
	}
	defer func() { _ = rows.Close() }()
	exists := false
	for rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return false, err
		}
	}
	return exists, rows.Err()
}

// DefaultVersion implements read.SchoolCurriculumSettings.
func (SchoolSettings) DefaultVersion(ctx context.Context, schoolID uuid.UUID, gradeLevel *int) (*uuid.UUID, error) {
	tenant, err := sessionTenant(ctx)
	if err != nil {
		return nil, err
	}
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	exists, err := settingsTableExists(ctx, q)
	if err != nil || !exists {
		return nil, err
	}
	grade := 0
	if gradeLevel != nil {
		grade = *gradeLevel
	}
	// The academic years of the classes of the school (and grade) in the projection (OQ F021-5, F040 T044): when the
	// projection knows them, only the settings of those years apply; without academic year the previous rule stays.
	// tenancyguard:ok tenant_id of the session scope and deleted_at IS NULL are in the WHERE clause (RLS T as well); ref.classes of the same tenant
	rows, err := q.QueryContext(ctx, `WITH years AS (SELECT DISTINCT k.academic_year_id FROM ref.classes k
			WHERE k.tenant_id = $1 AND k.school_id = $2 AND k.academic_year_id IS NOT NULL AND ($3 = 0 OR k.grade_level = $3))
		SELECT curriculum_version_id FROM lms.school_curriculum_settings
		WHERE tenant_id = $1 AND school_id = $2 AND deleted_at IS NULL AND (grade_level IS NULL OR grade_level = $3)
			AND (NOT EXISTS (SELECT 1 FROM years) OR academic_year_id IN (SELECT academic_year_id FROM years))
		ORDER BY grade_level NULLS LAST, created_at DESC LIMIT 1`, tenant, schoolID, grade)
	if err != nil {
		return nil, fmt.Errorf("school settings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var id uuid.UUID
	if err := rows.Scan(&id); err != nil {
		return nil, err
	}
	return &id, rows.Err()
}

// StudentSchool implements read.SchoolCurriculumSettings.
func (SchoolSettings) StudentSchool(ctx context.Context, studentID uuid.UUID) (*uuid.UUID, *int, error) {
	tenant, err := sessionTenant(ctx)
	if err != nil {
		return nil, nil, err
	}
	q, err := txOf(ctx)
	if err != nil {
		return nil, nil, err
	}
	// tenancyguard:ok projection ref.students filtered by the session tenant and deleted_at IS NULL (RLS T as well)
	rows, err := q.QueryContext(ctx, `SELECT school_id, grade_level FROM ref.students
		WHERE student_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, studentID, tenant)
	if err != nil {
		return nil, nil, fmt.Errorf("ref.students: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, nil, rows.Err()
	}
	var school uuid.UUID
	var grade sql.NullInt16
	if err := rows.Scan(&school, &grade); err != nil {
		return nil, nil, err
	}
	if !grade.Valid {
		return &school, nil, rows.Err()
	}
	g := int(grade.Int16)
	return &school, &g, rows.Err()
}
