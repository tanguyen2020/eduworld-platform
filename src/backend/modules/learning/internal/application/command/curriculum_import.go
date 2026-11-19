package command

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// CsvImportMax is csv_import_max (CONFIG_DEFAULTS: 10 MB, owner SECURITY_ARCHITECTURE): the largest import file.
const CsvImportMax = 10 << 20

// Import job statuses and the audit actions of the import (F021 US5).
const (
	ImportQueued              = "queued"
	ImportRunning             = "running"
	ImportSucceeded           = "succeeded"
	ImportFailed              = "failed"
	ActionContentImportQueued = "content.import_requested"
	ActionContentImported     = "content.imported"
)

// Row error codes of an import (ContentImportJob.errors[].code).
const (
	ImportErrFileTooLarge  = "FILE_TOO_LARGE"
	ImportErrAsset         = "ASSET_NOT_FOUND"
	ImportErrVersionExists = "VERSION_EXISTS"
	ImportErrUnknownCode   = "UNKNOWN_CODE"
)

// CreateImportInput is the request of createContentImport.
type CreateImportInput struct {
	Kind    string
	AssetID uuid.UUID
	DryRun  bool
}

// CreateImport queues an import job (202, F021-AC-22): curriculum kinds need curriculum.write; other kinds belong
// to F024. The owner of the import is the session tenant, or the platform for platform staff.
func (h Curriculum) CreateImport(ctx context.Context, in CreateImportInput) (out dto.ImportJob, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateContentImport")
	defer func() { done(err) }()
	content := slices.Contains(QuestionImportKinds, in.Kind)
	switch {
	case in.Kind == "lessons_docx":
		return out, problem.Invalid("kind", SubImportKindNotSupported, "lessons_docx is not supported in this release")
	case in.Kind != port.KindCurriculumYAML && in.Kind != port.KindCurriculumCSV && !content:
		return out, problem.Invalid("kind", problem.SubInvalid, "unknown import kind")
	case content && h.Content == nil:
		return out, problem.Invalid("kind", SubImportKindNotSupported, "question imports are not enabled")
	}
	perm := authz.PermCurriculumWrite
	if content {
		perm = authz.PermLearningContentImport
	}
	p, err := authorize(ctx, perm)
	if err != nil {
		return out, err
	}
	o := contentmodel.Owner{Scope: contentmodel.OwnerPlatform}
	if p.TenantID != nil {
		o = contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: ptrClone(p.TenantID)}
	} else if !p.IsPlatform() {
		return out, denied(ctx, "platform_import")
	}
	key := ids.New().String()
	if pend, ok := idempotency.PendingFrom(ctx); ok {
		key = pend.Key.Key.String()
	}
	asset := in.AssetID
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		j := dto.ImportJob{ID: ids.New(), Owner: o, Kind: in.Kind, AssetID: &asset, DryRun: in.DryRun, Status: ImportQueued, IdempotencyKey: key}
		if err := h.Imports.InsertImportJob(ctx, j); err != nil {
			return err
		}
		if err := h.recordCurriculum(ctx, o, audit.Event{Action: ActionContentImportQueued, TargetID: j.ID.String(),
			Metadata: map[string]any{"kind": in.Kind, "dry_run": in.DryRun}}); err != nil {
			return err
		}
		if out, err = h.Imports.ImportJob(ctx, j.ID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.ImportJobJSON(out))
	})
	return out, err
}

// jobScope is the data scope of a job of owner o: the tenant of the job, or the platform.
func jobScope(ctx context.Context, o contentmodel.Owner) (context.Context, error) {
	if o.TenantID != nil {
		sc, err := authz.EventScope(*o.TenantID)
		if err != nil {
			return ctx, err
		}
		return tenancy.WithScope(ctx, sc), nil
	}
	sc, err := authz.SystemPlatformScope("curriculum-import")
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(ctx, sc), nil
}

// RunPendingImports runs up to limit queued import jobs, oldest first (worker profile jobs). It lists the queue
// under tenancy.Bypass(platform_job) and runs each job in the scope of its owner.
func (h Curriculum) RunPendingImports(ctx context.Context, limit int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.RunPendingImports")
	defer func() { done(err) }()
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return 0, err
	}
	var jobs []dto.ImportJob
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		jobs, err = h.Imports.QueuedImportJobs(ctx, limit)
		return err
	}); err != nil {
		return 0, err
	}
	for _, j := range jobs {
		if err := h.RunImportJob(ctx, j.ID, j.Owner); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// RunImportJob runs one queued job: reads the file (≤ csv_import_max), parses it, imports it with the validators of
// the CMS (nothing written on dry_run or on any rejected row) and records the report (F021-AC-22, AC-25).
func (h Curriculum) RunImportJob(ctx context.Context, jobID uuid.UUID, o contentmodel.Owner) error {
	ctx, err := jobScope(ctx, o)
	if err != nil {
		return err
	}
	var job dto.ImportJob
	claimed := false
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		j, err := h.Imports.ImportJob(ctx, jobID)
		if err != nil || j.Status != ImportQueued {
			return err
		}
		j.Status = ImportRunning
		if err := h.Imports.UpdateImportJob(ctx, j); err != nil {
			return err
		}
		job, claimed = j, true
		return nil
	}); err != nil || !claimed {
		return err
	}
	if slices.Contains(QuestionImportKinds, job.Kind) {
		return h.runContentImport(ctx, job)
	}
	file, rowErrs := h.readImport(ctx, job)
	var res ImportResult
	if len(rowErrs) == 0 {
		actor := uuid.Nil
		if job.CreatedBy != nil {
			actor = *job.CreatedBy
		}
		res, err = h.ImportCurriculum(ctx, ImportCurriculumInput{Owner: job.Owner, Actor: actor, File: file, DryRun: job.DryRun})
		if err != nil {
			return err
		}
		rowErrs = res.Errors
	}
	at := h.now()
	job.FinishedAt = &at
	job.Errors = rowErrs
	job.RejectedCount = len(rowErrs)
	job.Status = ImportSucceeded
	if len(rowErrs) > 0 {
		job.Status = ImportFailed
	} else {
		job.CreatedCount = res.Created
		if !job.DryRun {
			job.Result = map[string]any{"curriculum_id": res.CurriculumID.String(), "curriculum_version_id": res.VersionID.String()}
		}
	}
	if h.Metrics != nil {
		h.Metrics.ImportFinished(job.Kind, job.Status, job.RejectedCount)
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Imports.UpdateImportJob(ctx, job); err != nil {
			return err
		}
		r := audit.ResultSuccess
		if job.Status == ImportFailed {
			r = audit.ResultFailure
		}
		return h.recordCurriculum(ctx, job.Owner, audit.Event{Action: ActionContentImported, TargetID: job.ID.String(), Result: r,
			Metadata: map[string]any{"kind": job.Kind, "dry_run": job.DryRun, "status": job.Status, "created": job.CreatedCount,
				"rejected": job.RejectedCount}})
	})
}

// runContentImport runs a question import job (F024-AC-13…16): valid rows become drafts, rejected rows are listed;
// the job succeeds when at least one row was imported (or would be on dry run) and records its audit.
func (h Curriculum) runContentImport(ctx context.Context, job dto.ImportJob) error {
	data, rowErrs := h.readBytes(ctx, job)
	created := 0
	if len(rowErrs) == 0 && h.Content != nil {
		var err error
		created, rowErrs, err = h.Content.Run(ctx, job, data)
		if err != nil {
			return err
		}
	}
	at := h.now()
	job.FinishedAt, job.Errors, job.RejectedCount, job.CreatedCount = &at, rowErrs, len(rowErrs), created
	job.Status = ImportSucceeded
	if created == 0 {
		job.Status = ImportFailed
	}
	if h.Metrics != nil {
		h.Metrics.ImportFinished(job.Kind, job.Status, job.RejectedCount)
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Imports.UpdateImportJob(ctx, job); err != nil {
			return err
		}
		r := audit.ResultSuccess
		if job.Status == ImportFailed {
			r = audit.ResultFailure
		}
		return h.recordCurriculum(ctx, job.Owner, audit.Event{Action: ActionContentImported, TargetID: job.ID.String(), Result: r,
			Metadata: map[string]any{"kind": job.Kind, "dry_run": job.DryRun, "status": job.Status, "created": job.CreatedCount,
				"rejected": job.RejectedCount}})
	})
}

// readBytes reads the file of a job (≤ csv_import_max).
func (h Curriculum) readBytes(ctx context.Context, j dto.ImportJob) ([]byte, []dto.ImportRowError) {
	if j.AssetID == nil || h.Assets == nil {
		return nil, []dto.ImportRowError{{Row: 1, Field: "asset_id", Code: ImportErrAsset, Message: "the file is not available"}}
	}
	rc, size, err := h.Assets.Open(ctx, *j.AssetID)
	if err != nil {
		return nil, []dto.ImportRowError{{Row: 1, Field: "asset_id", Code: ImportErrAsset, Message: "the file is not available"}}
	}
	defer func() { _ = rc.Close() }()
	if size > CsvImportMax {
		return nil, []dto.ImportRowError{{Row: 1, Code: ImportErrFileTooLarge, Message: "the file is larger than csv_import_max (10 MB)"}}
	}
	b, err := io.ReadAll(io.LimitReader(rc, CsvImportMax+1))
	if err != nil || len(b) > CsvImportMax {
		return nil, []dto.ImportRowError{{Row: 1, Code: ImportErrFileTooLarge, Message: "the file is larger than csv_import_max (10 MB)"}}
	}
	return b, nil
}

func (h Curriculum) readImport(ctx context.Context, j dto.ImportJob) (dto.CurriculumFile, []dto.ImportRowError) {
	if j.AssetID == nil || h.Assets == nil {
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Field: "asset_id", Code: ImportErrAsset, Message: "the file is not available"}}
	}
	rc, size, err := h.Assets.Open(ctx, *j.AssetID)
	if err != nil {
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Field: "asset_id", Code: ImportErrAsset, Message: "the file is not available"}}
	}
	defer func() { _ = rc.Close() }()
	if size > CsvImportMax {
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Code: ImportErrFileTooLarge, Message: "the file is larger than csv_import_max (10 MB)"}}
	}
	return h.ParseImport(j.Kind, rc)
}

// ParseImport parses an import file of kind, at most csv_import_max bytes (shared with the seed).
func (h Curriculum) ParseImport(kind string, r io.Reader) (dto.CurriculumFile, []dto.ImportRowError) {
	b, err := io.ReadAll(io.LimitReader(r, CsvImportMax+1))
	if err != nil {
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Code: "INVALID_FORMAT", Message: "the file cannot be read"}}
	}
	if len(b) > CsvImportMax {
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Code: ImportErrFileTooLarge, Message: "the file is larger than csv_import_max (10 MB)"}}
	}
	return h.Codec.Parse(kind, strings.NewReader(string(b)))
}

// ImportCurriculumInput is one import of a curriculum file.
type ImportCurriculumInput struct {
	Owner  contentmodel.Owner
	Actor  uuid.UUID // the user who requested the import (author of the version); uuid.Nil for the seed
	File   dto.CurriculumFile
	DryRun bool
}

// ImportResult is the outcome of an import: ids created (none on dry run or error) and the rejected rows.
type ImportResult struct {
	CurriculumID uuid.UUID
	VersionID    uuid.UUID
	Created      int
	Errors       []dto.ImportRowError
}

var errRollback = errors.New("command: import rolled back")

// ImportCurriculum imports a parsed curriculum file in the scope of ctx with the validators of the CMS (F021-AC-22,
// AC-23): a new curriculum with version 1, or a new draft version of the curriculum with the same code (its
// supersedes_version_id = the current release). Every row is checked in its own savepoint so the report lists all
// rejected rows; the whole import is rolled back on any error or on dry run.
func (h Curriculum) ImportCurriculum(ctx context.Context, in ImportCurriculumInput) (out ImportResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ImportCurriculum")
	defer func() { done(err) }()
	f := in.File
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var errs []dto.ImportRowError
		reject := func(row int, field string, e error) {
			errs = append(errs, rowError(row, field, e))
		}
		v, err := h.importVersion(ctx, in, &out)
		if err != nil {
			reject(f.Version.Row, "version", err)
			out.Errors = errs
			return errRollback
		}
		var codes []string
		for _, sg := range f.SubjectGrades {
			codes = append(codes, sg.SubjectCode)
		}
		for _, c := range f.Competencies {
			if c.SubjectCode != "" {
				codes = append(codes, c.SubjectCode)
			}
		}
		subjects, err := h.Catalog.SubjectsByCode(ctx, codes)
		if err != nil {
			return err
		}
		compIDs := map[string]uuid.UUID{}
		for _, c := range f.Competencies {
			in := CompetencyInput{VersionID: v.ID, Code: c.Code, Name: c.Name, Kind: c.Kind}
			if c.SubjectCode != "" {
				s, ok := subjects[c.SubjectCode]
				if !ok {
					reject(c.Row, "subject_code", problem.Invalid("subject_code", ImportErrUnknownCode, "is not a subject of the catalog"))
					continue
				}
				in.SubjectID = &s.ID
			}
			if err := h.UoW.Savepoint(ctx, func(ctx context.Context) error {
				cc, err := h.addCompetency(ctx, v, in)
				if err == nil {
					compIDs[c.Code] = cc.ID
				}
				return err
			}); err != nil {
				reject(c.Row, "code", err)
				continue
			}
			out.Created++
		}
		type pending struct {
			id   uuid.UUID
			obj  dto.FileObjective
			code string
		}
		var los []pending
		loIDs := map[string]uuid.UUID{}
		for _, sg := range f.SubjectGrades {
			s, ok := subjects[sg.SubjectCode]
			if !ok {
				reject(sg.Row, "subject_code", problem.Invalid("subject_code", ImportErrUnknownCode, "is not a subject of the catalog"))
				continue
			}
			if err := h.UoW.Savepoint(ctx, func(ctx context.Context) error {
				return h.Store.InsertSubjectGrade(ctx, v, dto.SubjectGrade{ID: ids.New(), SubjectID: s.ID, GradeLevel: sg.GradeLevel,
					PeriodsPerYear: sg.PeriodsPerYear})
			}); err != nil {
				reject(sg.Row, "grade_level", err)
				continue
			}
			topicIDs := map[string]uuid.UUID{}
			for _, t := range topicsParentsFirst(sg.Topics) {
				tin := TopicInput{VersionID: v.ID, Code: t.Code, Name: t.Name, SubjectID: s.ID, GradeLevel: sg.GradeLevel, SortOrder: t.SortOrder}
				if t.ParentCode != "" {
					pid, ok := topicIDs[t.ParentCode]
					if !ok {
						reject(t.Row, "parent_code", problem.Invalid("parent_code", ImportErrUnknownCode, "is not a topic of the same subject × grade"))
						continue
					}
					tin.ParentID = &pid
				}
				var topic dto.Topic
				if err := h.UoW.Savepoint(ctx, func(ctx context.Context) error {
					if err := curriculum.CheckTopicCode(t.Code); err != nil {
						return invalid(err)
					}
					if err := curriculum.CheckName("name", t.Name); err != nil {
						return invalid(err)
					}
					var err error
					topic, err = h.addTopic(ctx, v, tin)
					return err
				}); err != nil {
					reject(t.Row, "code", err)
					continue
				}
				topicIDs[t.Code] = topic.ID
				out.Created++
				for _, o := range t.Objectives {
					oin := ObjectiveInput{TopicID: topic.ID, Code: o.Code, Description: o.Description, SortOrder: o.SortOrder}
					if o.BloomLevel != "" {
						oin.BloomLevel = ptrClone(&o.BloomLevel)
					}
					if o.YccdRef != "" {
						oin.YccdRef = ptrClone(&o.YccdRef)
					}
					for _, cc := range o.CompetencyCodes {
						id, ok := compIDs[cc]
						if !ok {
							reject(o.Row, "competency_codes", problem.Invalid("competency_codes", ImportErrUnknownCode, cc+" is not a competency of the file"))
							continue
						}
						oin.CompetencyIDs = append(oin.CompetencyIDs, id)
					}
					var lo dto.Objective
					if err := h.UoW.Savepoint(ctx, func(ctx context.Context) error {
						var err error
						lo, err = h.addObjective(ctx, topic, oin)
						return err
					}); err != nil {
						reject(o.Row, "code", err)
						continue
					}
					loIDs[o.Code] = lo.ID
					los = append(los, pending{id: lo.ID, obj: o, code: o.Code})
					out.Created++
				}
			}
		}
		// Prerequisites once every LO exists (they may point forward in the file); the DAG is checked LO by LO.
		for _, p := range los {
			if len(p.obj.PrerequisiteCodes) == 0 {
				continue
			}
			var pre []uuid.UUID
			ok := true
			for _, code := range p.obj.PrerequisiteCodes {
				id, found := loIDs[code]
				if !found {
					reject(p.obj.Row, "prerequisite_codes", problem.Invalid("prerequisite_codes", ImportErrUnknownCode, code+" is not an objective of the file"))
					ok = false
					break
				}
				pre = append(pre, id)
			}
			if !ok {
				continue
			}
			if err := h.UoW.Savepoint(ctx, func(ctx context.Context) error {
				o, err := h.Store.Objective(ctx, p.id)
				if err != nil {
					return err
				}
				t, err := h.Store.Topic(ctx, o.TopicID)
				if err != nil {
					return err
				}
				o.PrerequisiteIDs = pre
				if err := h.checkObjective(ctx, o, t); err != nil {
					return err
				}
				_, err = h.Store.UpdateObjective(ctx, o, o.RowVersion)
				return err
			}); err != nil {
				reject(p.obj.Row, "prerequisite_codes", err)
			}
		}
		if len(errs) > 0 {
			slices.SortStableFunc(errs, func(a, b dto.ImportRowError) int { return a.Row - b.Row })
			if len(errs) > 500 {
				errs = errs[:500]
			}
			out.Errors = errs
			return errRollback
		}
		if in.DryRun {
			return errRollback
		}
		return h.recordCurriculum(ctx, in.Owner, audit.Event{Action: ActionContentVersionCreated, TargetID: out.CurriculumID.String(),
			Metadata: map[string]any{"curriculum_version_id": out.VersionID.String(), "source": "import", "requested_by": in.Actor.String()}})
	})
	if errors.Is(err, errRollback) {
		err = nil
		if len(out.Errors) > 0 || in.DryRun {
			if len(out.Errors) > 0 {
				out.Created = 0
			}
			out.CurriculumID, out.VersionID = uuid.Nil, uuid.Nil
		}
	}
	return out, err
}

// importVersion creates the curriculum (version 1) or a new draft version of the curriculum with the same code.
func (h Curriculum) importVersion(ctx context.Context, in ImportCurriculumInput, out *ImportResult) (dto.CurriculumVersion, error) {
	f := in.File
	vin := VersionInput{Label: f.Version.Label, EffectiveFrom: f.Version.EffectiveFrom, EffectiveTo: f.Version.EffectiveTo,
		ChangeNote: f.Version.ChangeNote, ContentLanguage: f.Version.ContentLanguage}
	if vin.ContentLanguage == "" {
		vin.ContentLanguage = string(contentmodel.LangVI)
	}
	if err := vin.validate(); err != nil {
		return dto.CurriculumVersion{}, err
	}
	c, err := h.Store.CurriculumByCode(ctx, in.Owner, f.Curriculum.Code)
	if err != nil {
		return dto.CurriculumVersion{}, err
	}
	if c == nil {
		if err := curriculum.CheckCurriculumCode("code", f.Curriculum.Code); err != nil {
			return dto.CurriculumVersion{}, invalid(err)
		}
		if err := curriculum.CheckLearningName("name", f.Curriculum.Name); err != nil {
			return dto.CurriculumVersion{}, invalid(err)
		}
		cc, v, err := h.createCurriculum(ctx, in.Actor, in.Owner, CreateCurriculumInput{Code: f.Curriculum.Code, Name: f.Curriculum.Name,
			Framework: f.Curriculum.Framework, Version: vin})
		if err != nil {
			return v, err
		}
		out.CurriculumID, out.VersionID = cc.ID, v.ID
		return v, nil
	}
	vs, err := h.Store.Versions(ctx, c.ID)
	if err != nil {
		return dto.CurriculumVersion{}, err
	}
	maxNo := c.LatestVersionNo
	for _, x := range vs {
		if x.Label == vin.Label {
			return dto.CurriculumVersion{}, problem.Invalid("version_label", ImportErrVersionExists, "the curriculum already has version "+vin.Label)
		}
		if curriculum.Open(x.Status) {
			return dto.CurriculumVersion{}, curriculumErr(curriculum.ErrOpenDraft)
		}
		maxNo = max(maxNo, x.VersionNo)
	}
	v := dto.CurriculumVersion{ID: ids.New(), CurriculumID: c.ID, VersionNo: maxNo + 1, Label: vin.Label, Status: contentmodel.StatusDraft,
		EffectiveFrom: vin.EffectiveFrom, EffectiveTo: vin.EffectiveTo, SupersedesID: ptrClone(c.CurrentVersionID), ChangeNote: vin.ChangeNote,
		ContentLanguage: vin.ContentLanguage, AuthorID: in.Actor, Owner: c.Owner}
	if err := h.Store.InsertVersion(ctx, v); err != nil {
		return v, err
	}
	cc := *c
	cc.LatestVersionNo = v.VersionNo
	if _, err := h.Store.UpdateCurriculum(ctx, cc, c.RowVersion); err != nil {
		return v, err
	}
	out.CurriculumID, out.VersionID = c.ID, v.ID
	return v, nil
}

// topicsParentsFirst orders the topics of a file so parents come first (unknown parents stay last and are rejected).
func topicsParentsFirst(ts []dto.FileTopic) []dto.FileTopic {
	byCode := map[string]dto.FileTopic{}
	for _, t := range ts {
		byCode[t.Code] = t
	}
	depth := func(t dto.FileTopic) int {
		d := 0
		for cur := t.ParentCode; cur != "" && d <= len(ts); d++ {
			p, ok := byCode[cur]
			if !ok {
				return len(ts) + 1
			}
			cur = p.ParentCode
		}
		return d
	}
	out := slices.Clone(ts)
	slices.SortStableFunc(out, func(a, b dto.FileTopic) int { return depth(a) - depth(b) })
	return out
}

// rowError turns the error of a row into a report entry (field and code of a 422, else the problem code).
func rowError(row int, field string, err error) dto.ImportRowError {
	e := dto.ImportRowError{Row: max(row, 1), Field: field, Code: string(problem.ValidationFailed)}
	var pe *problem.Error
	if errors.As(err, &pe) {
		e.Code, e.Message = string(pe.Code), pe.Detail
		if len(pe.Fields) > 0 {
			e.Field, e.Code, e.Message = pe.Fields[0].Field, pe.Fields[0].Code, pe.Fields[0].Message
		}
	} else {
		e.Message = "the row is invalid"
	}
	if len([]rune(e.Message)) > 500 {
		e.Message = string([]rune(e.Message)[:500])
	}
	return e
}

// SeedResult is the outcome of SeedCurriculum.
type SeedResult struct {
	CurriculumID uuid.UUID
	VersionID    uuid.UUID
	Created      int
	Skipped      bool
	Errors       []dto.ImportRowError
}

// SeedCurriculum loads a platform curriculum file through the import use case and publishes it (F021-AC-23,
// FR-CUR-004): run again, it finds the version label already there and creates nothing. Platform job scope,
// never SQL by hand.
func (h Curriculum) SeedCurriculum(ctx context.Context, kind string, r io.Reader, at time.Time) (out SeedResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.SeedCurriculum")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope("curriculum-seed")
	if err != nil {
		return out, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	file, rowErrs := h.ParseImport(kind, r)
	if len(rowErrs) > 0 {
		out.Errors = rowErrs
		return out, nil
	}
	platform := contentmodel.Owner{Scope: contentmodel.OwnerPlatform}
	var existing *dto.Curriculum
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		existing, err = h.Store.CurriculumByCode(ctx, platform, file.Curriculum.Code)
		if err != nil || existing == nil {
			return err
		}
		vs, err := h.Store.Versions(ctx, existing.ID)
		if err != nil {
			return err
		}
		for _, v := range vs {
			if v.Label == file.Version.Label {
				out = SeedResult{CurriculumID: existing.ID, VersionID: v.ID, Skipped: v.Status == contentmodel.StatusPublished}
			}
		}
		return nil
	}); err != nil {
		return out, err
	}
	if out.VersionID == uuid.Nil {
		res, err := h.ImportCurriculum(ctx, ImportCurriculumInput{Owner: platform, File: file})
		if err != nil || len(res.Errors) > 0 {
			out.Errors = res.Errors
			return out, err
		}
		out.CurriculumID, out.VersionID, out.Created = res.CurriculumID, res.VersionID, res.Created
	}
	if out.Skipped {
		return out, nil
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Store.Curriculum(ctx, out.CurriculumID)
		if err != nil {
			return err
		}
		v, err := h.Store.VersionByID(ctx, out.VersionID)
		if err != nil {
			return err
		}
		for _, a := range []contentmodel.Action{contentmodel.ActionSubmitReview, contentmodel.ActionApprove} {
			if v.Status == contentmodel.StatusApproved {
				break
			}
			from := v.Status
			to, err := contentmodel.Transition(v.Status, a, contentmodel.TransitionInput{Actor: ids.New(), Author: v.AuthorID, Note: "seed"})
			if err != nil {
				return curriculumErr(err)
			}
			v.Status = to
			if v, err = h.Store.UpdateVersion(ctx, v, v.RowVersion); err != nil {
				return err
			}
			if err := h.Store.RecordTransition(ctx, v, from, to, uuid.Nil, "seed "+file.Curriculum.Code); err != nil {
				return err
			}
		}
		_, err = h.publish(ctx, uuid.Nil, c, v, "seed-"+v.ID.String())
		return err
	})
	return out, err
}

// SeedSubjects adds the subjects of the seed catalog that do not exist yet (platform job scope); it returns how many
// it created. Run before SeedCurriculum: curriculum files reference subjects by code.
func (h Curriculum) SeedSubjects(ctx context.Context, subjects []SubjectInput) (created int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.SeedSubjects")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope("curriculum-seed")
	if err != nil {
		return 0, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		codes := make([]string, len(subjects))
		for i, s := range subjects {
			codes[i] = s.Code
		}
		have, err := h.Catalog.SubjectsByCode(ctx, codes)
		if err != nil {
			return err
		}
		for _, s := range subjects {
			if _, ok := have[s.Code]; ok {
				continue
			}
			if err := curriculum.CheckSubjectCode(s.Code); err != nil {
				return invalid(err)
			}
			if err := curriculum.CheckLearningName("name", s.Name); err != nil {
				return invalid(err)
			}
			if err := checkGrades(s.GradeLevels); err != nil {
				return err
			}
			if err := h.Catalog.InsertSubject(ctx, dto.Subject{ID: ids.New(), Code: s.Code, Name: s.Name, GradeLevels: s.GradeLevels,
				IconKey: s.IconKey}); err != nil {
				return err
			}
			created++
		}
		return nil
	})
	return created, err
}
