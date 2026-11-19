package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the translation jobs (F025 US5).
const (
	ActionTranslationQueued = "content.translation_job_queued"
	ActionTranslationDone   = "content.translation_job_finished"
)

// Translation job statuses and limits (TranslationExportRequest: ≤ 500 content ids, report.errors ≤ 500).
const (
	translationQueued    = "queued"
	translationRunning   = "running"
	translationSucceeded = "succeeded"
	translationFailed    = "failed"
	maxTranslationItems  = 500
	maxReportErrors      = 500
	translationFileMax   = 20 << 20 // request_body_max of an import file [Proposed]
)

// Translations runs the export and import of translation files (F025 US5, ADMIN_PORTAL §1.5).
type Translations struct {
	UoW       write.UnitOfWork
	Jobs      write.TranslationJobStore
	Workflow  Workflow // loaders of every content type
	Questions Questions
	Docs      Docs
	Codec     port.TranslationCodec
	Files     port.ExportFiles
	Assets    port.AssetReader
	Audit     write.Auditor
	Clock     func() time.Time
}

// TranslationRequest is the stored request of a job (TranslationExportRequest / TranslationImportRequest).
type TranslationRequest struct {
	ContentTypes []string    `json:"content_types,omitempty"`
	ContentIDs   []uuid.UUID `json:"content_ids,omitempty"`
	OnlyMissing  bool        `json:"only_missing"`
	DryRun       bool        `json:"dry_run"`
}

// TranslationJobInput is the request of exportLearningTranslations or importLearningTranslations.
type TranslationJobInput struct {
	Kind         string // export | import
	Format       string
	SourceLang   string
	TargetLang   string
	ContentTypes []string
	ContentIDs   []uuid.UUID
	OnlyMissing  bool
	AssetID      *uuid.UUID
	DryRun       bool
}

func (h Translations) now() time.Time { return now(h.Clock) }

func (h Translations) record(ctx context.Context, o contentmodel.Owner, e audit.Event) error {
	e.TargetType, e.Result = "translation_job", audit.ResultSuccess
	return TrashDeps{Audit: h.Audit}.recordFor(ctx, o.TenantID, e)
}

// TranslationJobViewOf is schema TranslationJob of j.
func TranslationJobViewOf(j write.TranslationJob) dto.TranslationJobView {
	v := dto.TranslationJobView{JobID: j.ID, Kind: j.Kind, Status: j.Status, ResultKey: j.ResultKey, TenantID: j.Owner.TenantID}
	if len(j.Report) > 0 && string(j.Report) != "null" {
		var r dto.TranslationReport
		if json.Unmarshal(j.Report, &r) == nil {
			v.Report = &r
		}
	}
	return v
}

// Create queues an export or import job (F025-AC-22, AC-23): learning_content.translate, owner = the session tenant
// or the platform; answered 202 with the queued job.
func (h Translations) Create(ctx context.Context, in TranslationJobInput) (out dto.TranslationJobView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateTranslationJob")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentTranslate)
	if err != nil {
		return out, err
	}
	if in.Format != "xliff" && in.Format != "csv" {
		return out, problem.Invalid("format", problem.SubInvalid, "must be xliff or csv")
	}
	if in.TargetLang != i18ntext.VI && in.TargetLang != i18ntext.EN {
		return out, problem.Invalid("target_language", problem.SubInvalid, "must be vi or en")
	}
	src := in.SourceLang
	if src == "" {
		src = i18ntext.Other(in.TargetLang)
	}
	if src == in.TargetLang || (src != i18ntext.VI && src != i18ntext.EN) {
		return out, problem.Invalid("source_language", problem.SubInvalid, "must be the other content language")
	}
	req := TranslationRequest{ContentTypes: in.ContentTypes, ContentIDs: in.ContentIDs, OnlyMissing: in.OnlyMissing, DryRun: in.DryRun}
	for i, t := range in.ContentTypes {
		ct, err := contentmodel.ParseContentType(t)
		if err != nil || !slices.Contains(WorkflowTypes, ct) {
			return out, problem.Invalid("content_types/"+strconv.Itoa(i), problem.SubInvalid, "is not a learning content type with versions")
		}
	}
	switch in.Kind {
	case "export":
		if len(in.ContentIDs) == 0 || len(in.ContentIDs) > maxTranslationItems {
			return out, problem.Invalid("content_ids", problem.SubRequired, "1–500 content ids are exported per job")
		}
	case "import":
		if in.AssetID == nil {
			return out, problem.Invalid("asset_id", problem.SubRequired, "the uploaded file is required")
		}
	default:
		return out, problem.New(problem.Internal, "unknown translation job kind")
	}
	o := contentmodel.Owner{Scope: contentmodel.OwnerPlatform}
	if p.TenantID != nil {
		o = contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: ptrClone(p.TenantID)}
	} else if !p.IsPlatform() {
		return out, denied(ctx, "platform_translation")
	}
	b, _ := json.Marshal(req)
	j := write.TranslationJob{ID: ids.New(), Owner: o, Kind: in.Kind, Status: translationQueued, Format: in.Format, SourceLang: src,
		TargetLang: in.TargetLang, Request: b, InputAssetID: in.AssetID, RequestedBy: p.UserID}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Jobs.InsertTranslationJob(ctx, j); err != nil {
			return err
		}
		if err := h.record(ctx, o, audit.Event{Action: ActionTranslationQueued, TargetID: j.ID.String(),
			Metadata: map[string]any{"kind": in.Kind, "format": in.Format, "target_language": in.TargetLang, "items": len(in.ContentIDs)}}); err != nil {
			return err
		}
		out = TranslationJobViewOf(j)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// RunPendingTranslations runs up to limit queued jobs of every owner (worker profile jobs).
func (h Translations) RunPendingTranslations(ctx context.Context, limit int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.RunPendingTranslations")
	defer func() { done(err) }()
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return 0, err
	}
	var jobs []write.TranslationJob
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		jobs, err = h.Jobs.QueuedTranslationJobs(ctx, limit)
		return err
	}); err != nil {
		return 0, err
	}
	for _, j := range jobs {
		if err := h.RunTranslationJob(ctx, j.ID, j.Owner); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// report collects the lines of a job report.
type report struct{ dto.TranslationReport }

func (r *report) fail(ref, msg string) {
	if len(r.Errors) < maxReportErrors {
		r.Errors = append(r.Errors, dto.TranslationReportError{Ref: truncate(ref, 255), Message: truncate(msg, 500)})
	}
}

// RunTranslationJob runs one queued job in the scope of its owner: another tenant's item does not exist for it and is
// reported like an unknown item (F025-AC-24).
func (h Translations) RunTranslationJob(ctx context.Context, id uuid.UUID, o contentmodel.Owner) error {
	ctx, err := jobScope(ctx, o)
	if err != nil {
		return err
	}
	var job write.TranslationJob
	claimed := false
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		j, err := h.Jobs.TranslationJob(ctx, id)
		if err != nil || j.Status != translationQueued {
			return err
		}
		j.Status = translationRunning
		job, claimed = j, true
		return h.Jobs.UpdateTranslationJob(ctx, j)
	}); err != nil || !claimed {
		return err
	}
	var req TranslationRequest
	_ = json.Unmarshal(job.Request, &req)
	var rep report
	rep.Errors = []dto.TranslationReportError{}
	if job.Kind == "export" {
		err = h.export(ctx, &job, req, &rep)
	} else {
		err = h.importFile(ctx, job, req, &rep)
	}
	at := h.now()
	job.FinishedAt, job.Status = &at, translationSucceeded
	if err != nil {
		var pe *problem.Error
		if !errors.As(err, &pe) {
			return err
		}
		job.Status = translationFailed
		rep.fail(job.ID.String(), pe.Detail)
	}
	job.Report, _ = json.Marshal(rep.TranslationReport)
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Jobs.UpdateTranslationJob(ctx, job); err != nil {
			return err
		}
		return h.record(ctx, job.Owner, audit.Event{Action: ActionTranslationDone, TargetID: job.ID.String(),
			Metadata: map[string]any{"kind": job.Kind, "status": job.Status, "total": rep.Total, "updated": rep.Updated, "skipped": rep.Skipped}})
	})
}

// find loads the version of an item among types (the current release, else the newest version).
func (h Translations) find(ctx context.Context, types []string, id uuid.UUID, versionNo int) (Subject, error) {
	if len(types) == 0 {
		for _, t := range WorkflowTypes {
			types = append(types, string(t))
		}
	}
	for _, t := range types {
		ct := contentmodel.ContentType(t)
		no := versionNo
		if no == 0 {
			latest, err := h.latest(ctx, ct, id)
			if err != nil {
				continue
			}
			no = latest
		}
		s, err := h.Workflow.Load(ctx, ct, id, no)
		if err == nil {
			return s, nil
		}
	}
	return Subject{}, problem.New(problem.NotFound, "")
}

func (h Translations) latest(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (int, error) {
	if t == contentmodel.TypeQuestion {
		q, err := h.Workflow.Questions.Question(ctx, id)
		if err != nil {
			return 0, err
		}
		return releaseNo(q.CurrentVersionID, q.LatestVersionNo, func() ([]int, []uuid.UUID, error) {
			vs, err := h.Workflow.Questions.QuestionVersions(ctx, id)
			nos, idl := make([]int, len(vs)), make([]uuid.UUID, len(vs))
			for i, v := range vs {
				nos[i], idl[i] = v.VersionNo, v.ID
			}
			return nos, idl, err
		})
	}
	d, err := h.Workflow.Docs.Doc(ctx, t, id)
	if err != nil {
		return 0, err
	}
	return releaseNo(d.CurrentVersionID, d.LatestVersionNo, func() ([]int, []uuid.UUID, error) {
		vs, err := h.Workflow.Docs.DocVersions(ctx, t, id)
		nos, idl := make([]int, len(vs)), make([]uuid.UUID, len(vs))
		for i, v := range vs {
			nos[i], idl[i] = v.VersionNo, v.ID
		}
		return nos, idl, err
	})
}

// releaseNo is the version number of the release pointer, else latest.
func releaseNo(current *uuid.UUID, latest int, versions func() ([]int, []uuid.UUID, error)) (int, error) {
	if current == nil {
		return latest, nil
	}
	nos, idl, err := versions()
	if err != nil {
		return 0, err
	}
	for i, id := range idl {
		if id == *current {
			return nos[i], nil
		}
	}
	return latest, nil
}

func unitKey(s Subject, pointer string) string {
	return string(s.Type) + "/" + s.ItemID.String() + "/" + strconv.Itoa(s.VersionNo) + pointer
}

// export writes the texts of the requested items to ew-exports/translations/<job_id>.<ext> (F025-AC-22).
func (h Translations) export(ctx context.Context, job *write.TranslationJob, req TranslationRequest, rep *report) error {
	var units []port.TranslationUnit
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		for _, id := range req.ContentIDs {
			rep.Total++
			s, err := h.find(ctx, req.ContentTypes, id, 0)
			if err != nil {
				rep.Skipped++
				rep.fail(id.String(), "not found")
				continue
			}
			if s.ContentLanguage != job.SourceLang {
				rep.Skipped++
				rep.fail(id.String(), "content_language is "+s.ContentLanguage)
				continue
			}
			for _, t := range i18ntext.Texts(s.Doc, s.ContentLanguage) {
				if req.OnlyMissing && t.Values[job.TargetLang] != "" {
					continue
				}
				units = append(units, port.TranslationUnit{Key: unitKey(s, t.Pointer), Source: t.Values[job.SourceLang], Target: t.Values[job.TargetLang]})
			}
			rep.Updated++
		}
		return nil
	})
	if err != nil {
		return err
	}
	data, mime, err := h.Codec.Encode(job.Format, job.SourceLang, job.TargetLang, units)
	if err != nil {
		return problem.New(problem.ValidationFailed, "cannot encode the file").Wrap(err)
	}
	ext := map[string]string{"xliff": "xlf", "csv": "csv"}[job.Format]
	key := "translations/" + job.ID.String() + "." + ext
	if err := h.Files.Put(ctx, key, data, mime); err != nil {
		return err
	}
	job.ResultKey = &key
	return nil
}

// parsedKey is a decoded unit key.
type parsedKey struct {
	t       contentmodel.ContentType
	id      uuid.UUID
	version int
	pointer string
}

func parseUnitKey(k string) (parsedKey, bool) {
	parts := strings.SplitN(k, "/", 4)
	if len(parts) < 4 {
		return parsedKey{}, false
	}
	id, err := uuid.Parse(parts[1])
	no, err2 := strconv.Atoi(parts[2])
	if err != nil || err2 != nil || no < 1 {
		return parsedKey{}, false
	}
	return parsedKey{t: contentmodel.ContentType(parts[0]), id: id, version: no, pointer: "/" + parts[3]}, true
}

// importFile reads the uploaded file and creates one new draft version per item with its translations (F025-AC-23):
// published versions never change; a key of an unknown or invisible item, or of an item with an open draft, is
// reported; nothing is written on dry run.
func (h Translations) importFile(ctx context.Context, job write.TranslationJob, req TranslationRequest, rep *report) error {
	rc, size, err := h.Assets.Open(ctx, *job.InputAssetID)
	if err != nil {
		return problem.New(problem.ValidationFailed, "the uploaded file cannot be read")
	}
	defer func() { _ = rc.Close() }()
	if size > translationFileMax {
		return problem.New(problem.ValidationFailed, "the file is too large")
	}
	data, err := io.ReadAll(io.LimitReader(rc, translationFileMax+1))
	if err != nil {
		return err
	}
	units, err := h.Codec.Decode(job.Format, data)
	if err != nil {
		return problem.New(problem.ValidationFailed, "the file is not a valid "+job.Format+" translation file")
	}
	type group struct {
		key   parsedKey
		units []port.TranslationUnit
	}
	var groups []*group
	byItem := map[string]*group{}
	for _, u := range units {
		k, ok := parseUnitKey(u.Key)
		if !ok || !slices.Contains(WorkflowTypes, k.t) {
			rep.fail(u.Key, "unknown key")
			continue
		}
		gk := fmt.Sprintf("%s/%s/%d", k.t, k.id, k.version)
		g, ok := byItem[gk]
		if !ok {
			g = &group{key: k}
			byItem[gk] = g
			groups = append(groups, g)
		}
		g.units = append(g.units, u)
	}
	actor := job.RequestedBy
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		for _, g := range groups {
			rep.Total++
			ref := fmt.Sprintf("%s/%s/%d", g.key.t, g.key.id, g.key.version)
			err := h.UoW.Savepoint(ctx, func(ctx context.Context) error {
				s, err := h.Workflow.Load(ctx, g.key.t, g.key.id, g.key.version)
				if err != nil {
					return problem.New(problem.NotFound, "not found")
				}
				doc, applied := s.Doc, 0
				for _, u := range g.units {
					if strings.TrimSpace(u.Target) == "" {
						continue
					}
					k, _ := parseUnitKey(u.Key)
					next, err := i18ntext.SetTranslation(doc, k.pointer, s.ContentLanguage, job.TargetLang, strings.TrimSpace(u.Target))
					if err != nil {
						rep.fail(u.Key, "the pointer is not a learning text")
						continue
					}
					doc, applied = next, applied+1
				}
				if applied == 0 {
					return problem.New(problem.Conflict, "no translation to apply")
				}
				return h.newVersion(ctx, s, actor, doc, job.ID)
			})
			if err != nil {
				var pe *problem.Error
				if !errors.As(err, &pe) {
					return err
				}
				rep.Skipped++
				msg := pe.Detail
				if msg == "" {
					msg = string(pe.Code)
				}
				rep.fail(ref, msg)
				continue
			}
			rep.Updated++
		}
		if req.DryRun {
			return errRollback
		}
		return nil
	})
	if errors.Is(err, errRollback) {
		return nil
	}
	return err
}

// newVersion creates the next draft version of s with the texts of doc (based_on = s), validated like a CMS save.
func (h Translations) newVersion(ctx context.Context, s Subject, actor uuid.UUID, doc json.RawMessage, jobID uuid.UUID) error {
	if s.Type == contentmodel.TypeQuestion {
		return h.Questions.translatedVersion(ctx, s, actor, doc, jobID)
	}
	return h.Docs.translatedVersion(ctx, s, actor, doc, jobID)
}

// translatedVersion copies version s of a question into a new draft with the texts of doc (translation import).
func (h Questions) translatedVersion(ctx context.Context, s Subject, actor uuid.UUID, doc json.RawMessage, jobID uuid.UUID) error {
	q, err := h.Store.Question(ctx, s.ItemID)
	if err != nil {
		return err
	}
	vs, err := h.Store.QuestionVersions(ctx, q.ID)
	if err != nil {
		return err
	}
	var src *dto.QuestionVersion
	maxNo := q.LatestVersionNo
	for i, v := range vs {
		if v.Status == contentmodel.StatusDraft || v.Status == contentmodel.StatusInReview {
			return problem.New(problem.Conflict, "the item already has an open draft or in-review version")
		}
		if v.ID == s.VersionID {
			src = &vs[i]
		}
		maxNo = max(maxNo, v.VersionNo)
	}
	if src == nil {
		return problem.New(problem.NotFound, "")
	}
	var parts map[string]json.RawMessage
	if err := json.Unmarshal(doc, &parts); err != nil {
		return err
	}
	v := *src
	srcID := src.ID
	v.ID, v.VersionNo, v.Status, v.AuthorID, v.ReviewerID, v.PublishedAt, v.BasedOnID = ids.New(), maxNo+1, contentmodel.StatusDraft, actor, nil, nil, &srcID
	v.AudioExemptApprovedBy, v.Owner = nil, q.Owner
	v.Stem, v.Explanation, v.Hints = parts["stem"], parts["explanation"], parts["hints"]
	if err := h.prepare(ctx, &v, parts["render_payload"]); err != nil {
		return err
	}
	prev, _ := questionDoc(*src)
	next, _ := questionDoc(v)
	v.TranslationStatus, v.TranslationMeta = translation(prev, next, v.ContentLanguage, src.TranslationMeta, now(h.Clock))
	if err := h.Store.InsertQuestionVersion(ctx, v); err != nil {
		return err
	}
	if err := h.recordUsages(ctx, v); err != nil {
		return err
	}
	q.LatestVersionNo = v.VersionNo
	if _, err := h.Store.UpdateQuestion(ctx, q, q.RowVersion); err != nil {
		return err
	}
	return h.recordQuestion(ctx, q.Owner, audit.Event{Action: ActionContentVersionCreated, TargetID: q.ID.String(),
		Metadata: map[string]any{"question_version_id": v.ID.String(), "version": v.VersionNo, "based_on_version_id": srcID.String(),
			"translation_job_id": jobID.String()}})
}

// translatedVersion copies version s of a composed item into a new draft with the texts of doc (translation import).
func (h Docs) translatedVersion(ctx context.Context, s Subject, actor uuid.UUID, doc json.RawMessage, jobID uuid.UUID) error {
	d, err := h.Store.Doc(ctx, s.Type, s.ItemID)
	if err != nil {
		return err
	}
	vs, err := h.Store.DocVersions(ctx, s.Type, d.ID)
	if err != nil {
		return err
	}
	var src *dto.DocVersion
	maxNo := d.LatestVersionNo
	for i, v := range vs {
		if v.Status == contentmodel.StatusDraft || v.Status == contentmodel.StatusInReview {
			return problem.New(problem.Conflict, "the item already has an open draft or in-review version")
		}
		if v.ID == s.VersionID {
			src = &vs[i]
		}
		maxNo = max(maxNo, v.VersionNo)
	}
	if src == nil {
		return problem.New(problem.NotFound, "")
	}
	var parts map[string]json.RawMessage
	if err := json.Unmarshal(doc, &parts); err != nil {
		return err
	}
	v := *src
	srcID := src.ID
	v.ID, v.VersionNo, v.Status, v.AuthorID, v.ReviewerID, v.PublishedAt, v.BasedOnID = ids.New(), maxNo+1, contentmodel.StatusDraft, actor, nil, nil, &srcID
	v.AudioExemptApprovedBy, v.Owner = nil, d.Owner
	v.Title, v.Summary = parts["title"], parts["summary"]
	body := bodyOf(*src)
	for k := range body {
		if raw, ok := parts[k]; ok {
			body[k] = raw
		}
	}
	if v.Body, err = json.Marshal(body); err != nil {
		return err
	}
	if err := h.prepare(ctx, &v); err != nil {
		return err
	}
	v.TranslationStatus, v.TranslationMeta = translation(docDoc(*src), docDoc(v), v.ContentLanguage, src.TranslationMeta, now(h.Clock))
	if err := h.Store.InsertDocVersion(ctx, v); err != nil {
		return err
	}
	if err := h.recordUsages(ctx, v); err != nil {
		return err
	}
	d.LatestVersionNo = v.VersionNo
	if _, err := h.Store.UpdateDoc(ctx, d, d.RowVersion); err != nil {
		return err
	}
	return h.record(ctx, s.Type, d.Owner, audit.Event{Action: ActionContentVersionCreated, TargetID: d.ID.String(),
		Metadata: map[string]any{"content_version_id": v.ID.String(), "version": v.VersionNo, "based_on_version_id": srcID.String(),
			"translation_job_id": jobID.String()}})
}
