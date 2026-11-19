package write

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entcomp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/competency"
	entcur "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/curriculum"
	entver "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/curriculumversion"
	entlo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/learningobjective"
	entmap "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/loversionmapping"
	entsg "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/subjectgrade"
	enttopic "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/topic"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CurriculumRepo implements write.CurriculumStore with ent (entx.HybridContent filter of the scope of ctx).
type CurriculumRepo struct{}

var _ write.CurriculumStore = CurriculumRepo{}

// MapCurriculumErr maps database errors of the curriculum tables to problems (shared with the catalog and
// import repositories and the resource handler).
func MapCurriculumErr(err error) error {
	if err == nil {
		return nil
	}
	var pe *problem.Error
	if errors.As(err, &pe) {
		return err
	}
	if ent.IsNotFound(err) {
		return problem.New(problem.NotFound, "").Wrap(err)
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch {
		case pg.Code == "P0001" && pg.Message == "CONTENT_NOT_EDITABLE":
			return problem.New(problem.ContentNotEditable, "the curriculum version is not a draft").Wrap(err)
		case pg.Code == "23505":
			return problem.New(problem.Conflict, "a record with the same code already exists").Wrap(err)
		case pg.Code == "23503":
			return problem.Invalid("reference", problem.SubInvalid, "a referenced record does not exist").Wrap(err)
		case pg.Code == "23514":
			return problem.Invalid(pg.ConstraintName, problem.SubInvalid, "violates a rule of the curriculum").Wrap(err)
		}
	}
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, "").Wrap(err)
	}
	var ve *ent.ValidationError
	if errors.As(err, &ve) {
		return problem.Invalid(ve.Name, problem.SubInvalid, "is invalid").Wrap(err)
	}
	return err
}

func owner(scope string, tenant *uuid.UUID) contentmodel.Owner {
	o := contentmodel.Owner{Scope: contentmodel.OwnerScope(scope)}
	if tenant != nil {
		t := *tenant
		o.TenantID = &t
	}
	return o
}

func status(s string) contentmodel.Status { return contentmodel.Status(s) }

// mismatchOrMissing tells a lost optimistic update apart from a row the scope cannot see.
func mismatchOrMissing(exists bool, err error) error {
	if err != nil {
		return MapCurriculumErr(err)
	}
	if !exists {
		return problem.New(problem.NotFound, "")
	}
	return problem.New(problem.VersionMismatch, "")
}

func intPtr16(p *int16) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

// ---------- curricula

func curriculumOf(r *ent.Curriculum) dto.Curriculum {
	return dto.Curriculum{ID: r.ID, Code: r.Code, Name: r.Name, Framework: r.Framework, Owner: owner(r.OwnerScope, r.TenantID),
		CurrentVersionID: r.CurrentVersionID, LatestVersionNo: r.LatestVersionNo, RowVersion: r.Version, CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt}
}

// InsertCurriculum implements write.CurriculumStore.
func (CurriculumRepo) InsertCurriculum(ctx context.Context, c dto.Curriculum) error {
	q := client(ctx).Curriculum.Create().SetID(c.ID).SetCode(c.Code).SetName(c.Name).SetNillableFramework(c.Framework).
		SetOwnerScope(string(c.Owner.Scope)).SetLatestVersionNo(c.LatestVersionNo).SetNillableClonedFromID(c.ClonedFromID).
		SetNillableClonedFromTenantID(c.ClonedFromTenantID).SetNillableClonedAt(c.ClonedAt).SetNillableClonedBy(c.ClonedBy)
	if c.Owner.TenantID != nil {
		q.SetTenantID(*c.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// Curriculum implements write.CurriculumStore.
func (CurriculumRepo) Curriculum(ctx context.Context, id uuid.UUID) (dto.Curriculum, error) {
	r, err := client(ctx).Curriculum.Query().Where(entcur.ID(id)).Only(ctx)
	if err != nil {
		return dto.Curriculum{}, MapCurriculumErr(err)
	}
	return curriculumOf(r), nil
}

// CurriculumByCode implements write.CurriculumStore.
func (CurriculumRepo) CurriculumByCode(ctx context.Context, o contentmodel.Owner, code string) (*dto.Curriculum, error) {
	q := client(ctx).Curriculum.Query().Where(entcur.Code(code))
	if o.TenantID == nil {
		q = q.Where(entcur.TenantIDIsNil())
	} else {
		q = q.Where(entcur.TenantID(*o.TenantID))
	}
	r, err := q.Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	c := curriculumOf(r)
	return &c, nil
}

// UpdateCurriculum implements write.CurriculumStore.
func (s CurriculumRepo) UpdateCurriculum(ctx context.Context, c dto.Curriculum, ifRow int64) (dto.Curriculum, error) {
	u := client(ctx).Curriculum.Update().Where(entcur.ID(c.ID), entcur.Version(ifRow)).SetName(c.Name).
		SetLatestVersionNo(c.LatestVersionNo).AddVersion(1)
	if c.Framework != nil {
		u.SetFramework(*c.Framework)
	} else {
		u.ClearFramework()
	}
	if c.CurrentVersionID != nil {
		u.SetCurrentVersionID(*c.CurrentVersionID)
	} else {
		u.ClearCurrentVersionID()
	}
	n, err := u.Save(ctx)
	if err != nil {
		return c, MapCurriculumErr(err)
	}
	if n == 0 {
		return c, mismatchOrMissing(client(ctx).Curriculum.Query().Where(entcur.ID(c.ID)).Exist(ctx))
	}
	return s.Curriculum(ctx, c.ID)
}

// ---------- versions

func versionOf(r *ent.CurriculumVersion) dto.CurriculumVersion {
	return dto.CurriculumVersion{ID: r.ID, CurriculumID: r.CurriculumID, VersionNo: r.VersionNo, Label: r.VersionLabel, Status: status(r.Status),
		EffectiveFrom: r.EffectiveFrom, EffectiveTo: r.EffectiveTo, SupersedesID: r.SupersedesVersionID, BasedOnID: r.BasedOnVersionID,
		ChangeNote: r.ChangeNote, ContentLanguage: r.ContentLanguage, AuthorID: r.AuthorID, ReviewerID: r.ReviewerID,
		PublishedAt: r.PublishedAt, Owner: owner(r.OwnerScope, r.TenantID), RowVersion: r.Version, UpdatedAt: r.UpdatedAt}
}

// InsertVersion implements write.CurriculumStore.
func (CurriculumRepo) InsertVersion(ctx context.Context, v dto.CurriculumVersion) error {
	q := client(ctx).CurriculumVersion.Create().SetID(v.ID).SetCurriculumID(v.CurriculumID).SetVersionNo(v.VersionNo).
		SetVersionLabel(v.Label).SetStatus(string(v.Status)).SetNillableEffectiveFrom(v.EffectiveFrom).SetNillableEffectiveTo(v.EffectiveTo).
		SetNillableSupersedesVersionID(v.SupersedesID).SetNillableBasedOnVersionID(v.BasedOnID).SetAuthorID(v.AuthorID).
		SetNillableChangeNote(v.ChangeNote).SetContentLanguage(v.ContentLanguage).SetOwnerScope(string(v.Owner.Scope))
	if v.Owner.TenantID != nil {
		q.SetTenantID(*v.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

func (s CurriculumRepo) withGrades(ctx context.Context, v dto.CurriculumVersion) (dto.CurriculumVersion, error) {
	sgs, err := s.SubjectGrades(ctx, v.ID)
	v.SubjectGrades = sgs
	return v, err
}

// Version implements write.CurriculumStore.
func (s CurriculumRepo) Version(ctx context.Context, curriculumID uuid.UUID, versionNo int) (dto.CurriculumVersion, error) {
	r, err := client(ctx).CurriculumVersion.Query().Where(entver.CurriculumID(curriculumID), entver.VersionNo(versionNo)).Only(ctx)
	if err != nil {
		return dto.CurriculumVersion{}, MapCurriculumErr(err)
	}
	return s.withGrades(ctx, versionOf(r))
}

// VersionByID implements write.CurriculumStore.
func (s CurriculumRepo) VersionByID(ctx context.Context, id uuid.UUID) (dto.CurriculumVersion, error) {
	r, err := client(ctx).CurriculumVersion.Query().Where(entver.ID(id)).Only(ctx)
	if err != nil {
		return dto.CurriculumVersion{}, MapCurriculumErr(err)
	}
	return s.withGrades(ctx, versionOf(r))
}

// Versions implements write.CurriculumStore.
func (CurriculumRepo) Versions(ctx context.Context, curriculumID uuid.UUID) ([]dto.CurriculumVersion, error) {
	rows, err := client(ctx).CurriculumVersion.Query().Where(entver.CurriculumID(curriculumID)).Order(ent.Desc(entver.FieldVersionNo)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]dto.CurriculumVersion, len(rows))
	for i, r := range rows {
		out[i] = versionOf(r)
	}
	return out, nil
}

// UpdateVersion implements write.CurriculumStore.
func (s CurriculumRepo) UpdateVersion(ctx context.Context, v dto.CurriculumVersion, ifRow int64) (dto.CurriculumVersion, error) {
	u := client(ctx).CurriculumVersion.Update().Where(entver.ID(v.ID), entver.Version(ifRow)).SetVersionLabel(v.Label).
		SetStatus(string(v.Status)).AddVersion(1)
	if v.EffectiveFrom != nil {
		u.SetEffectiveFrom(*v.EffectiveFrom)
	} else {
		u.ClearEffectiveFrom()
	}
	if v.EffectiveTo != nil {
		u.SetEffectiveTo(*v.EffectiveTo)
	} else {
		u.ClearEffectiveTo()
	}
	if v.ChangeNote != nil {
		u.SetChangeNote(*v.ChangeNote)
	} else {
		u.ClearChangeNote()
	}
	if v.ReviewerID != nil {
		u.SetReviewerID(*v.ReviewerID)
	} else {
		u.ClearReviewerID()
	}
	if v.PublishedAt != nil {
		u.SetPublishedAt(*v.PublishedAt)
	}
	n, err := u.Save(ctx)
	if err != nil {
		return v, MapCurriculumErr(err)
	}
	if n == 0 {
		return v, mismatchOrMissing(client(ctx).CurriculumVersion.Query().Where(entver.ID(v.ID)).Exist(ctx))
	}
	return s.VersionByID(ctx, v.ID)
}

// ---------- subject grades

// SubjectGrades implements write.CurriculumStore.
func (CurriculumRepo) SubjectGrades(ctx context.Context, versionID uuid.UUID) ([]dto.SubjectGrade, error) {
	rows, err := client(ctx).SubjectGrade.Query().Where(entsg.CurriculumVersionID(versionID)).
		Order(ent.Asc(entsg.FieldGradeLevel), ent.Asc(entsg.FieldSubjectID)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]dto.SubjectGrade, len(rows))
	for i, r := range rows {
		out[i] = dto.SubjectGrade{ID: r.ID, SubjectID: r.SubjectID, GradeLevel: int(r.GradeLevel), PeriodsPerYear: intPtr16(r.PeriodsPerYear)}
	}
	return out, nil
}

// InsertSubjectGrade implements write.CurriculumStore.
func (CurriculumRepo) InsertSubjectGrade(ctx context.Context, v dto.CurriculumVersion, sg dto.SubjectGrade) error {
	q := client(ctx).SubjectGrade.Create().SetID(sg.ID).SetCurriculumID(v.CurriculumID).SetCurriculumVersionID(v.ID).
		SetSubjectID(sg.SubjectID).SetGradeLevel(int16(sg.GradeLevel)).SetOwnerScope(string(v.Owner.Scope)) //nolint:gosec // grade 1–9
	if sg.PeriodsPerYear != nil {
		q.SetPeriodsPerYear(int16(*sg.PeriodsPerYear)) //nolint:gosec // ≤ 1000 (contract)
	}
	if v.Owner.TenantID != nil {
		q.SetTenantID(*v.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// DeleteSubjectGrade implements write.CurriculumStore.
func (CurriculumRepo) DeleteSubjectGrade(ctx context.Context, id uuid.UUID) error {
	_, err := client(ctx).SubjectGrade.Delete().Where(entsg.ID(id)).Exec(entx.WithDeleteReason(ctx, "subject_grade_removed"))
	return MapCurriculumErr(err)
}

// ---------- topics

func topicOf(r *ent.Topic) dto.Topic {
	return dto.Topic{ID: r.ID, CurriculumID: r.CurriculumID, VersionID: r.CurriculumVersionID, SubjectID: r.SubjectID, GradeLevel: int(r.GradeLevel),
		ParentID: r.ParentTopicID, Code: r.Code, Name: r.Name, SortOrder: int(r.SortOrder), Owner: owner(r.OwnerScope, r.TenantID), RowVersion: r.Version}
}

// InsertTopic implements write.CurriculumStore.
func (CurriculumRepo) InsertTopic(ctx context.Context, t dto.Topic) error {
	q := client(ctx).Topic.Create().SetID(t.ID).SetCurriculumID(t.CurriculumID).SetCurriculumVersionID(t.VersionID).SetSubjectID(t.SubjectID).
		SetGradeLevel(int16(t.GradeLevel)).SetNillableParentTopicID(t.ParentID).SetCode(t.Code).SetName(t.Name). //nolint:gosec // grade 1–9
		SetSortOrder(int32(t.SortOrder)).SetOwnerScope(string(t.Owner.Scope))                                    //nolint:gosec // bounded by the contract
	if t.Owner.TenantID != nil {
		q.SetTenantID(*t.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// Topic implements write.CurriculumStore.
func (CurriculumRepo) Topic(ctx context.Context, id uuid.UUID) (dto.Topic, error) {
	r, err := client(ctx).Topic.Query().Where(enttopic.ID(id)).Only(ctx)
	if err != nil {
		return dto.Topic{}, MapCurriculumErr(err)
	}
	return topicOf(r), nil
}

// Topics implements write.CurriculumStore.
func (CurriculumRepo) Topics(ctx context.Context, versionID uuid.UUID) ([]dto.Topic, error) {
	rows, err := client(ctx).Topic.Query().Where(enttopic.CurriculumVersionID(versionID)).
		Order(ent.Asc(enttopic.FieldSortOrder), ent.Asc(enttopic.FieldCode)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]dto.Topic, len(rows))
	for i, r := range rows {
		out[i] = topicOf(r)
	}
	return out, nil
}

// UpdateTopic implements write.CurriculumStore.
func (s CurriculumRepo) UpdateTopic(ctx context.Context, t dto.Topic, ifRow int64) (dto.Topic, error) {
	u := client(ctx).Topic.Update().Where(enttopic.ID(t.ID), enttopic.Version(ifRow)).SetCode(t.Code).SetName(t.Name).
		SetSortOrder(int32(t.SortOrder)).AddVersion(1) //nolint:gosec // bounded by the contract
	if t.ParentID != nil {
		u.SetParentTopicID(*t.ParentID)
	} else {
		u.ClearParentTopicID()
	}
	n, err := u.Save(ctx)
	if err != nil {
		return t, MapCurriculumErr(err)
	}
	if n == 0 {
		return t, mismatchOrMissing(client(ctx).Topic.Query().Where(enttopic.ID(t.ID)).Exist(ctx))
	}
	return s.Topic(ctx, t.ID)
}

// DeleteTopic implements write.CurriculumStore.
func (CurriculumRepo) DeleteTopic(ctx context.Context, id uuid.UUID, ifRow int64, reason string) error {
	n, err := client(ctx).Topic.Delete().Where(enttopic.ID(id), enttopic.Version(ifRow)).Exec(entx.WithDeleteReason(ctx, reason))
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return mismatchOrMissing(client(ctx).Topic.Query().Where(enttopic.ID(id)).Exist(ctx))
	}
	return nil
}

// ---------- objectives

func objectiveOf(r *ent.LearningObjective) dto.Objective {
	return dto.Objective{ID: r.ID, CurriculumID: r.CurriculumID, VersionID: r.CurriculumVersionID, TopicID: r.TopicID, Code: r.Code,
		StableKey: r.StableKey, Description: r.Description, BloomLevel: r.BloomLevel, YccdRef: r.YccdRef, CompetencyIDs: r.CompetencyIds,
		PrerequisiteIDs: r.PrerequisiteIds, SortOrder: int(r.SortOrder), Owner: owner(r.OwnerScope, r.TenantID), RowVersion: r.Version}
}

func idList(v []uuid.UUID) []uuid.UUID {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

// InsertObjective implements write.CurriculumStore.
func (CurriculumRepo) InsertObjective(ctx context.Context, o dto.Objective) error {
	q := client(ctx).LearningObjective.Create().SetID(o.ID).SetCurriculumID(o.CurriculumID).SetCurriculumVersionID(o.VersionID).
		SetTopicID(o.TopicID).SetCode(o.Code).SetStableKey(o.StableKey).SetDescription(o.Description).SetNillableBloomLevel(o.BloomLevel).
		SetNillableYccdRef(o.YccdRef).SetCompetencyIds(idList(o.CompetencyIDs)).SetPrerequisiteIds(idList(o.PrerequisiteIDs)).
		SetSortOrder(int32(o.SortOrder)).SetOwnerScope(string(o.Owner.Scope)) //nolint:gosec // bounded by the contract
	if o.Owner.TenantID != nil {
		q.SetTenantID(*o.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// Objective implements write.CurriculumStore.
func (CurriculumRepo) Objective(ctx context.Context, id uuid.UUID) (dto.Objective, error) {
	r, err := client(ctx).LearningObjective.Query().Where(entlo.ID(id)).Only(ctx)
	if err != nil {
		return dto.Objective{}, MapCurriculumErr(err)
	}
	return objectiveOf(r), nil
}

// Objectives implements write.CurriculumStore.
func (CurriculumRepo) Objectives(ctx context.Context, versionID uuid.UUID) ([]dto.Objective, error) {
	rows, err := client(ctx).LearningObjective.Query().Where(entlo.CurriculumVersionID(versionID)).
		Order(ent.Asc(entlo.FieldSortOrder), ent.Asc(entlo.FieldCode)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]dto.Objective, len(rows))
	for i, r := range rows {
		out[i] = objectiveOf(r)
	}
	return out, nil
}

// UpdateObjective implements write.CurriculumStore.
func (s CurriculumRepo) UpdateObjective(ctx context.Context, o dto.Objective, ifRow int64) (dto.Objective, error) {
	u := client(ctx).LearningObjective.Update().Where(entlo.ID(o.ID), entlo.Version(ifRow)).SetTopicID(o.TopicID).SetCode(o.Code).
		SetDescription(o.Description).SetCompetencyIds(idList(o.CompetencyIDs)).SetPrerequisiteIds(idList(o.PrerequisiteIDs)).
		SetSortOrder(int32(o.SortOrder)).AddVersion(1) //nolint:gosec // bounded by the contract
	if o.BloomLevel != nil {
		u.SetBloomLevel(*o.BloomLevel)
	} else {
		u.ClearBloomLevel()
	}
	if o.YccdRef != nil {
		u.SetYccdRef(*o.YccdRef)
	} else {
		u.ClearYccdRef()
	}
	n, err := u.Save(ctx)
	if err != nil {
		return o, MapCurriculumErr(err)
	}
	if n == 0 {
		return o, mismatchOrMissing(client(ctx).LearningObjective.Query().Where(entlo.ID(o.ID)).Exist(ctx))
	}
	return s.Objective(ctx, o.ID)
}

// DeleteObjective implements write.CurriculumStore.
func (CurriculumRepo) DeleteObjective(ctx context.Context, id uuid.UUID, ifRow int64, reason string) error {
	n, err := client(ctx).LearningObjective.Delete().Where(entlo.ID(id), entlo.Version(ifRow)).Exec(entx.WithDeleteReason(ctx, reason))
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return mismatchOrMissing(client(ctx).LearningObjective.Query().Where(entlo.ID(id)).Exist(ctx))
	}
	return nil
}

// ---------- competencies

func competencyOf(r *ent.Competency) dto.Competency {
	return dto.Competency{ID: r.ID, CurriculumID: r.CurriculumID, VersionID: r.CurriculumVersionID, Code: r.Code, Name: r.Name, Kind: r.Kind,
		SubjectID: r.SubjectID, Owner: owner(r.OwnerScope, r.TenantID), RowVersion: r.Version}
}

// InsertCompetency implements write.CurriculumStore.
func (CurriculumRepo) InsertCompetency(ctx context.Context, c dto.Competency) error {
	q := client(ctx).Competency.Create().SetID(c.ID).SetCurriculumID(c.CurriculumID).SetCurriculumVersionID(c.VersionID).SetCode(c.Code).
		SetName(c.Name).SetKind(c.Kind).SetNillableSubjectID(c.SubjectID).SetOwnerScope(string(c.Owner.Scope))
	if c.Owner.TenantID != nil {
		q.SetTenantID(*c.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// Competency implements write.CurriculumStore.
func (CurriculumRepo) Competency(ctx context.Context, id uuid.UUID) (dto.Competency, error) {
	r, err := client(ctx).Competency.Query().Where(entcomp.ID(id)).Only(ctx)
	if err != nil {
		return dto.Competency{}, MapCurriculumErr(err)
	}
	return competencyOf(r), nil
}

// Competencies implements write.CurriculumStore.
func (CurriculumRepo) Competencies(ctx context.Context, versionID uuid.UUID) ([]dto.Competency, error) {
	rows, err := client(ctx).Competency.Query().Where(entcomp.CurriculumVersionID(versionID)).Order(ent.Asc(entcomp.FieldCode)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]dto.Competency, len(rows))
	for i, r := range rows {
		out[i] = competencyOf(r)
	}
	return out, nil
}

// UpdateCompetency implements write.CurriculumStore.
func (s CurriculumRepo) UpdateCompetency(ctx context.Context, c dto.Competency, ifRow int64) (dto.Competency, error) {
	n, err := client(ctx).Competency.Update().Where(entcomp.ID(c.ID), entcomp.Version(ifRow)).SetName(c.Name).AddVersion(1).Save(ctx)
	if err != nil {
		return c, MapCurriculumErr(err)
	}
	if n == 0 {
		return c, mismatchOrMissing(client(ctx).Competency.Query().Where(entcomp.ID(c.ID)).Exist(ctx))
	}
	return s.Competency(ctx, c.ID)
}

// ---------- mappings

// Mappings implements write.CurriculumStore.
func (CurriculumRepo) Mappings(ctx context.Context, versionID uuid.UUID) (curriculum.MappingSet, error) {
	rows, err := client(ctx).LoVersionMapping.Query().Where(entmap.CurriculumVersionID(versionID)).Order(ent.Asc(entmap.FieldID)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make(curriculum.MappingSet, len(rows))
	for i, r := range rows {
		out[i] = curriculum.Mapping{FromLO: r.FromLoID, ToLO: r.ToLoID, Relation: r.Relation}
	}
	return out, nil
}

// ReplaceMappings implements write.CurriculumStore.
func (CurriculumRepo) ReplaceMappings(ctx context.Context, v dto.CurriculumVersion, set curriculum.MappingSet) error {
	c := client(ctx)
	if _, err := c.LoVersionMapping.Delete().Where(entmap.CurriculumVersionID(v.ID)).Exec(entx.WithDeleteReason(ctx, "mappings_replaced")); err != nil {
		return MapCurriculumErr(err)
	}
	if len(set) == 0 {
		return nil
	}
	builders := make([]*ent.LoVersionMappingCreate, len(set))
	for i, m := range set {
		b := c.LoVersionMapping.Create().SetID(ids.New()).SetCurriculumID(v.CurriculumID).SetCurriculumVersionID(v.ID).
			SetFromLoID(m.FromLO).SetNillableToLoID(m.ToLO).SetRelation(m.Relation).SetOwnerScope(string(v.Owner.Scope))
		if v.Owner.TenantID != nil {
			b.SetTenantID(*v.Owner.TenantID)
		}
		builders[i] = b
	}
	return MapCurriculumErr(c.LoVersionMapping.CreateBulk(builders...).Exec(ctx))
}

// ---------- history

// RecordTransition implements write.CurriculumStore.
func (CurriculumRepo) RecordTransition(ctx context.Context, v dto.CurriculumVersion, from, to contentmodel.Status, actor uuid.UUID, note string) error {
	q := client(ctx).ContentTransition.Create().SetID(ids.New()).SetContentType(string(contentmodel.TypeCurriculum)).
		SetContentVersionID(v.ID).SetFromStatus(string(from)).SetToStatus(string(to)).SetActorID(actor).SetAt(time.Now().UTC())
	if note != "" {
		q.SetNote(note)
	}
	if v.Owner.TenantID != nil {
		q.SetTenantID(*v.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// RecordRelease implements write.CurriculumStore.
func (CurriculumRepo) RecordRelease(ctx context.Context, r write.Release) (uuid.UUID, error) {
	id := ids.New()
	q := client(ctx).ContentRelease.Create().SetID(id).SetItemType(r.ItemType).SetItemID(r.ItemID).SetAction("publish").
		SetNillableFromVersionID(r.FromVersionID).SetToVersionID(r.ToVersionID).SetReleasedBy(r.ReleasedBy).SetReleasedAt(r.ReleasedAt).
		SetIdempotencyKey(r.IdempotencyKey)
	if r.Owner.TenantID != nil {
		q.SetTenantID(*r.Owner.TenantID)
	}
	return id, MapCurriculumErr(q.Exec(ctx))
}
