package write

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entimp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/importjob"
	entsubj "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/subject"
	enttb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/textbook"
	entsec "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/textbooksection"
	entsecobj "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/textbooksectionobjective"
	entseries "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/textbookseries"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// CatalogRepo implements write.CatalogStore (G tables: written by learning_platform under PlatformScope only).
type CatalogRepo struct{}

var _ write.CatalogStore = CatalogRepo{}

// SubjectOf maps an ent subject.
func SubjectOf(r *ent.Subject) dto.Subject {
	return dto.Subject{ID: r.ID, Code: r.Code, Name: r.Name, GradeLevels: r.GradeLevels, IconKey: r.IconKey, RowVersion: r.Version}
}

// InsertSubject implements write.CatalogStore.
func (CatalogRepo) InsertSubject(ctx context.Context, s dto.Subject) error {
	return MapCurriculumErr(client(ctx).Subject.Create().SetID(s.ID).SetCode(s.Code).SetName(s.Name).SetGradeLevels(s.GradeLevels).
		SetNillableIconKey(s.IconKey).Exec(ctx))
}

// Subject implements write.CatalogStore.
func (CatalogRepo) Subject(ctx context.Context, id uuid.UUID) (dto.Subject, error) {
	r, err := client(ctx).Subject.Query().Where(entsubj.ID(id)).Only(ctx)
	if err != nil {
		return dto.Subject{}, MapCurriculumErr(err)
	}
	return SubjectOf(r), nil
}

// SubjectsByCode implements write.CatalogStore.
func (CatalogRepo) SubjectsByCode(ctx context.Context, codes []string) (map[string]dto.Subject, error) {
	rows, err := client(ctx).Subject.Query().Where(entsubj.CodeIn(codes...)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make(map[string]dto.Subject, len(rows))
	for _, r := range rows {
		out[r.Code] = SubjectOf(r)
	}
	return out, nil
}

// UpdateSubject implements write.CatalogStore.
func (c CatalogRepo) UpdateSubject(ctx context.Context, s dto.Subject, ifRow int64) (dto.Subject, error) {
	u := client(ctx).Subject.Update().Where(entsubj.ID(s.ID), entsubj.Version(ifRow)).SetName(s.Name).SetGradeLevels(s.GradeLevels).AddVersion(1)
	if s.IconKey != nil {
		u.SetIconKey(*s.IconKey)
	} else {
		u.ClearIconKey()
	}
	n, err := u.Save(ctx)
	if err != nil {
		return s, MapCurriculumErr(err)
	}
	if n == 0 {
		return s, mismatchOrMissing(client(ctx).Subject.Query().Where(entsubj.ID(s.ID)).Exist(ctx))
	}
	return c.Subject(ctx, s.ID)
}

// InsertSeries implements write.CatalogStore.
func (CatalogRepo) InsertSeries(ctx context.Context, s dto.TextbookSeries) error {
	return MapCurriculumErr(client(ctx).TextbookSeries.Create().SetID(s.ID).SetCode(s.Code).SetName(s.Name).SetNillablePublisher(s.Publisher).Exec(ctx))
}

// Series implements write.CatalogStore.
func (CatalogRepo) Series(ctx context.Context, id uuid.UUID) (dto.TextbookSeries, error) {
	r, err := client(ctx).TextbookSeries.Query().Where(entseries.ID(id)).Only(ctx)
	if err != nil {
		return dto.TextbookSeries{}, MapCurriculumErr(err)
	}
	return dto.TextbookSeries{ID: r.ID, Code: r.Code, Name: r.Name, Publisher: r.Publisher}, nil
}

// InsertTextbook implements write.CatalogStore.
func (CatalogRepo) InsertTextbook(ctx context.Context, t dto.Textbook) error {
	return MapCurriculumErr(client(ctx).Textbook.Create().SetID(t.ID).SetSeriesID(t.SeriesID).SetSubjectID(t.SubjectID).
		SetGradeLevel(int16(t.GradeLevel)).SetVolume(int16(t.Volume)).Exec(ctx)) //nolint:gosec // 1–9 (contract)
}

// Textbook implements write.CatalogStore.
func (CatalogRepo) Textbook(ctx context.Context, id uuid.UUID) (dto.Textbook, error) {
	r, err := client(ctx).Textbook.Query().Where(enttb.ID(id)).Only(ctx)
	if err != nil {
		return dto.Textbook{}, MapCurriculumErr(err)
	}
	return dto.Textbook{ID: r.ID, SeriesID: r.SeriesID, SubjectID: r.SubjectID, GradeLevel: int(r.GradeLevel), Volume: int(r.Volume)}, nil
}

// SectionOf maps an ent section (objective ids loaded separately).
func SectionOf(r *ent.TextbookSection) dto.TextbookSection {
	return dto.TextbookSection{ID: r.ID, TextbookID: r.TextbookID, Chapter: r.Chapter, LessonNo: int(r.LessonNo), Title: r.Title,
		PageRange: r.PageRange, RowVersion: r.Version}
}

// InsertSection implements write.CatalogStore.
func (CatalogRepo) InsertSection(ctx context.Context, s dto.TextbookSection) error {
	return MapCurriculumErr(client(ctx).TextbookSection.Create().SetID(s.ID).SetTextbookID(s.TextbookID).SetNillableChapter(s.Chapter).
		SetLessonNo(int16(s.LessonNo)).SetTitle(s.Title).SetNillablePageRange(s.PageRange).Exec(ctx)) //nolint:gosec // ≤ 999 (contract)
}

// Section implements write.CatalogStore.
func (CatalogRepo) Section(ctx context.Context, id uuid.UUID) (dto.TextbookSection, error) {
	c := client(ctx)
	r, err := c.TextbookSection.Query().Where(entsec.ID(id)).Only(ctx)
	if err != nil {
		return dto.TextbookSection{}, MapCurriculumErr(err)
	}
	s := SectionOf(r)
	links, err := c.TextbookSectionObjective.Query().Where(entsecobj.SectionID(id)).Order(ent.Asc(entsecobj.FieldObjectiveID)).All(ctx)
	if err != nil {
		return s, MapCurriculumErr(err)
	}
	s.ObjectiveIDs = make([]uuid.UUID, len(links))
	for i, l := range links {
		s.ObjectiveIDs[i] = l.ObjectiveID
	}
	return s, nil
}

// UpdateSection implements write.CatalogStore.
func (c CatalogRepo) UpdateSection(ctx context.Context, s dto.TextbookSection, ifRow int64) (dto.TextbookSection, error) {
	u := client(ctx).TextbookSection.Update().Where(entsec.ID(s.ID), entsec.Version(ifRow)).SetLessonNo(int16(s.LessonNo)). //nolint:gosec // ≤ 999
																SetTitle(s.Title).AddVersion(1)
	if s.Chapter != nil {
		u.SetChapter(*s.Chapter)
	} else {
		u.ClearChapter()
	}
	if s.PageRange != nil {
		u.SetPageRange(*s.PageRange)
	} else {
		u.ClearPageRange()
	}
	n, err := u.Save(ctx)
	if err != nil {
		return s, MapCurriculumErr(err)
	}
	if n == 0 {
		return s, mismatchOrMissing(client(ctx).TextbookSection.Query().Where(entsec.ID(s.ID)).Exist(ctx))
	}
	return c.Section(ctx, s.ID)
}

// ReplaceSectionObjectives implements write.CatalogStore.
func (CatalogRepo) ReplaceSectionObjectives(ctx context.Context, sectionID uuid.UUID, objectiveIDs []uuid.UUID) error {
	c := client(ctx)
	links, err := c.TextbookSectionObjective.Query().Where(entsecobj.SectionID(sectionID)).All(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	have := map[uuid.UUID]bool{}
	for _, l := range links {
		if !slices.Contains(objectiveIDs, l.ObjectiveID) {
			if _, err := c.TextbookSectionObjective.Delete().Where(entsecobj.ID(l.ID)).Exec(entx.WithDeleteReason(ctx, "unmapped")); err != nil {
				return MapCurriculumErr(err)
			}
			continue
		}
		have[l.ObjectiveID] = true
	}
	for _, id := range objectiveIDs {
		if have[id] {
			continue
		}
		if err := c.TextbookSectionObjective.Create().SetID(ids.New()).SetSectionID(sectionID).SetObjectiveID(id).Exec(ctx); err != nil {
			return MapCurriculumErr(err)
		}
	}
	return nil
}

// ImportJobRepo implements write.ImportJobStore.
type ImportJobRepo struct{}

var _ write.ImportJobStore = ImportJobRepo{}

// ImportJobOf maps an ent import job.
func ImportJobOf(r *ent.ImportJob) dto.ImportJob {
	j := dto.ImportJob{ID: r.ID, Owner: owner(r.OwnerScope, r.TenantID), Kind: r.Kind, AssetID: r.AssetID, SourceRef: r.SourceRef, DryRun: r.DryRun,
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
	return j
}

func errorsJSON(es []dto.ImportRowError) []map[string]any {
	out := make([]map[string]any, len(es))
	for i, e := range es {
		m := map[string]any{"row": e.Row, "code": e.Code}
		if e.Field != "" {
			m["field"] = e.Field
		}
		if e.Message != "" {
			m["message"] = e.Message
		}
		out[i] = m
	}
	return out
}

// InsertImportJob implements write.ImportJobStore.
func (ImportJobRepo) InsertImportJob(ctx context.Context, j dto.ImportJob) error {
	q := client(ctx).ImportJob.Create().SetID(j.ID).SetOwnerScope(string(j.Owner.Scope)).SetKind(j.Kind).SetNillableAssetID(j.AssetID).
		SetNillableSourceRef(j.SourceRef).SetDryRun(j.DryRun).SetStatus(j.Status).SetIdempotencyKey(j.IdempotencyKey)
	if j.Owner.TenantID != nil {
		q.SetTenantID(*j.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// ImportJob implements write.ImportJobStore.
func (ImportJobRepo) ImportJob(ctx context.Context, id uuid.UUID) (dto.ImportJob, error) {
	r, err := client(ctx).ImportJob.Query().Where(entimp.ID(id)).Only(ctx)
	if err != nil {
		return dto.ImportJob{}, MapCurriculumErr(err)
	}
	return ImportJobOf(r), nil
}

// ImportJobByKey implements write.ImportJobStore.
func (ImportJobRepo) ImportJobByKey(ctx context.Context, createdBy uuid.UUID, key string) (*dto.ImportJob, error) {
	r, err := client(ctx).ImportJob.Query().Where(entimp.CreatedBy(createdBy), entimp.IdempotencyKey(key)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	j := ImportJobOf(r)
	return &j, nil
}

// UpdateImportJob implements write.ImportJobStore.
func (ImportJobRepo) UpdateImportJob(ctx context.Context, j dto.ImportJob) error {
	result := j.Result
	if result == nil {
		result = map[string]any{}
	}
	u := client(ctx).ImportJob.Update().Where(entimp.ID(j.ID)).SetStatus(j.Status).SetCreatedCount(int32(j.CreatedCount)). //nolint:gosec // bounded
																SetRejectedCount(int32(j.RejectedCount)).SetErrors(errorsJSON(j.Errors)).SetResultRefs(result).AddVersion(1) //nolint:gosec // bounded
	if j.FinishedAt != nil {
		u.SetFinishedAt(*j.FinishedAt)
	}
	n, err := u.Save(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return mismatchOrMissing(false, nil)
	}
	return nil
}

// QueuedImportJobs implements write.ImportJobStore (ctx carries tenancy.Bypass(platform_job)).
func (ImportJobRepo) QueuedImportJobs(ctx context.Context, limit int) ([]dto.ImportJob, error) {
	rows, err := client(ctx).ImportJob.Query().Where(entimp.Status("queued")).Order(ent.Asc(entimp.FieldCreatedAt), ent.Asc(entimp.FieldID)).
		Limit(limit).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]dto.ImportJob, len(rows))
	for i, r := range rows {
		out[i] = ImportJobOf(r)
	}
	return out, nil
}
