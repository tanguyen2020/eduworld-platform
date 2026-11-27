package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/docximport"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// KindLessonsDocx is the import kind of Word files (F028).
const KindLessonsDocx = "lessons_docx"

// DocxImportMaxQuestions is docx_import_max_questions (CONFIG_DEFAULTS, owner CONTENT_PRODUCTION_PLAN §8, scope code).
const DocxImportMaxQuestions = 200

// Sub-codes of 409 CONFLICT of createDraftFromImportItem (F028-AC-11).
const (
	SubImportItemNotImportable = "IMPORT_ITEM_NOT_IMPORTABLE"
	SubImportItemCreated       = "IMPORT_ITEM_ALREADY_CREATED"
)

// DocxImport is the Word import lessons_docx (F028, research R5): Analyse converts the file of a job and stores one
// report item per question (no question is created); CreateDraft makes a version 1 draft of one analysed question
// with the SVG of its formulas rendered by the portal (ADR-0055 §4).
type DocxImport struct {
	Items     write.ImportItemStore
	Jobs      write.ImportJobStore
	Converter port.DocxConverter
	Assets    port.AssetReader
	Content   *ContentImport
	Media     *Media // pictures of the questions (nil = a question with a picture cannot be created)
	Clock     func() time.Time
	// MaxQuestions is docx_import_max_questions (0 = DocxImportMaxQuestions).
	MaxQuestions int
	Limits       docximport.Limits // zero = docximport.DefaultLimits
}

func (h DocxImport) limits() docximport.Limits {
	if h.Limits == (docximport.Limits{}) {
		return docximport.DefaultLimits
	}
	return h.Limits
}

func fileError(code, msg string) []dto.ImportRowError {
	return []dto.ImportRowError{{Row: 1, Field: "file", Code: code, Message: msg}}
}

// Analysis is the outcome of Analyse: the items to store, or the file-level errors.
type Analysis struct {
	Items      []dto.ImportItem
	FileErrors []dto.ImportRowError
}

// Convert runs the converter on data (the .docx of job) outside any transaction (F028-AC-01, AC-06): a file-level
// failure comes back as row errors.
func (h DocxImport) Convert(ctx context.Context, job dto.ImportJob, data []byte) (docximport.Document, []dto.ImportRowError) {
	if h.Converter == nil {
		return docximport.Document{}, fileError(docximport.FileConverterUnavailable, port.ErrDocxNoConverter.Message)
	}
	start := time.Now()
	doc, err := h.Converter.Convert(ctx, data)
	var de port.DocxError
	switch {
	case errors.As(err, &de):
		slog.WarnContext(ctx, "learning: docx conversion refused", "import_id", job.ID, "code", de.Code, "duration_ms", time.Since(start).Milliseconds())
		return docximport.Document{}, fileError(de.Code, de.Message)
	case err != nil:
		slog.ErrorContext(ctx, "learning: docx conversion failed", "import_id", job.ID, "error", err)
		return docximport.Document{}, fileError(docximport.FileConverterUnavailable, port.ErrDocxNoConverter.Message)
	}
	slog.InfoContext(ctx, "learning: docx converted", "import_id", job.ID, "blocks", len(doc.Blocks), "duration_ms", time.Since(start).Milliseconds())
	return doc, nil
}

// Analyse splits a converted document into questions (F028-AC-01…07): every question is an item with its status and
// issues. Curriculum codes are resolved in the scope of ctx (the owner of the job) inside the transaction of ctx.
func (h DocxImport) Analyse(ctx context.Context, job dto.ImportJob, doc docximport.Document) (out Analysis, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RunDocxImport")
	defer func() { done(err) }()
	hdr, raws := docximport.Split(doc)
	maxQ := h.MaxQuestions
	if maxQ <= 0 {
		maxQ = DocxImportMaxQuestions
	}
	switch {
	case len(raws) == 0:
		out.FileErrors = fileError(docximport.FileNoQuestions, "no question found: every question starts with \"Câu n.\"")
		return out, nil
	case len(raws) > maxQ:
		out.FileErrors = fileError(docximport.FileTooManyQuestions, fmt.Sprintf("the file has %d questions, at most %d (docx_import_max_questions)", len(raws), maxQ))
		return out, nil
	}
	repeated := docximport.Repeated(raws)
	lim := h.limits()
	resolved := map[string]error{}
	at := now(h.Clock)
	for _, r := range raws {
		it := docximport.Build(docximport.Parse(r), hdr, lim, func() string { return ids.New().String() })
		if w, ok := repeated[r.No]; ok {
			it.Issues = append(it.Issues, w)
		}
		item := dto.ImportItem{ID: ids.New(), ImportID: job.ID, Owner: job.Owner, No: it.No, Label: it.Label, StemPreview: it.StemPreview,
			Formulas: it.Formulas, Images: it.Images, UpdatedAt: at}
		if it.Type != "" {
			t := it.Type
			item.QuestionType = &t
			item.Payload = dto.ImportPayload{Fields: map[string]json.RawMessage{}, Codes: it.Codes}
			for k, v := range it.Fields {
				b, err := json.Marshal(v)
				if err != nil {
					return out, err
				}
				item.Payload.Fields[k] = b
			}
			if !hasMetadataError(it.Issues) && h.Content != nil {
				if is, err := h.resolve(ctx, job.Owner, it.Codes, resolved); err != nil {
					return out, err
				} else if is != nil {
					it.Issues = append(it.Issues, *is)
				}
			}
		}
		if len(it.Issues) > docximport.MaxIssues {
			it.Issues = it.Issues[:docximport.MaxIssues]
		}
		item.Issues, item.Status = it.Issues, docximport.StatusOf(it.Issues)
		out.Items = append(out.Items, item)
	}
	slog.InfoContext(ctx, "learning: docx analysed", "import_id", job.ID, "items", len(out.Items))
	return out, nil
}

func hasMetadataError(is []docximport.Issue) bool {
	for _, i := range is {
		if i.Code == docximport.CodeMetadataMissing || i.Code == docximport.CodeDifficultyInvalid {
			return true
		}
	}
	return false
}

// resolve checks the curriculum codes of a question in the scope of the job (UNKNOWN_CODE); results are cached per
// set of codes for the job.
func (h DocxImport) resolve(ctx context.Context, o contentmodel.Owner, c docximport.Codes, cache map[string]error) (*docximport.Issue, error) {
	key, _ := json.Marshal(c)
	err, ok := cache[string(key)]
	if !ok {
		_, err = h.Content.fields(ctx, o, dto.QuestionRow{Codes: codesOf(c)})
		var pe *problem.Error
		if err != nil && !errors.As(err, &pe) {
			return nil, err
		}
		cache[string(key)] = err
	}
	var pe *problem.Error
	if !errors.As(err, &pe) {
		return nil, nil
	}
	field, msg := "", truncate(pe.Detail, 500)
	if len(pe.Fields) > 0 {
		field, msg = truncate(pe.Fields[0].Field, 128), truncate(pe.Fields[0].Message, 500)
	}
	is := docximport.Err(docximport.CodeUnknownCode, field, msg)
	return &is, nil
}

func codesOf(c docximport.Codes) *dto.QuestionCodes {
	return &dto.QuestionCodes{Subject: c.Subject, Grades: c.Grades, Curriculum: c.Curriculum, CurriculumVersion: c.CurriculumVersion,
		Topic: c.Topic, Objectives: c.Objectives, Difficulty: c.Difficulty, Language: c.Language}
}

// runDocxImport is the job of kind lessons_docx (F028-AC-01, AC-06, AC-07): the items are stored with the report of
// the job in one transaction; the job succeeds when at least one question can be imported. dry_run has no effect:
// no question is ever created by the job.
func (h Curriculum) runDocxImport(ctx context.Context, job dto.ImportJob) error {
	data, rowErrs := h.readBytes(ctx, job)
	var doc docximport.Document
	switch {
	case len(rowErrs) > 0:
	case h.Docx == nil:
		rowErrs = fileError(docximport.FileConverterUnavailable, port.ErrDocxNoConverter.Message)
	default:
		doc, rowErrs = h.Docx.Convert(ctx, job, data)
	}
	var an Analysis
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		an = Analysis{FileErrors: rowErrs}
		if len(rowErrs) == 0 {
			var err error
			if an, err = h.Docx.Analyse(ctx, job, doc); err != nil {
				return err
			}
		}
		importable := 0
		var errs []dto.ImportRowError
		for _, it := range an.Items {
			if docximport.Importable(it.Status) {
				importable++
			}
			for _, is := range it.Issues {
				if is.Severity == docximport.SevError {
					errs = append(errs, dto.ImportRowError{Row: it.No, Field: is.Field, Code: is.Code, Message: truncate(it.Label+": "+is.Message, 500)})
				}
			}
		}
		if len(an.Items) > 0 {
			if err := h.Docx.Items.InsertImportItems(ctx, an.Items); err != nil {
				return err
			}
		}
		at := h.now()
		job.FinishedAt, job.CreatedCount = &at, importable
		job.Errors = append(an.FileErrors, errs...)
		if len(job.Errors) > 500 {
			job.Errors = job.Errors[:500]
		}
		job.RejectedCount = len(an.Items) - importable
		if len(an.FileErrors) > 0 {
			job.RejectedCount = 0
		}
		job.Result = map[string]any{"items_total": len(an.Items), "items_importable": importable}
		job.Status = ImportSucceeded
		if importable == 0 {
			job.Status = ImportFailed
		}
		if err := h.Imports.UpdateImportJob(ctx, job); err != nil {
			return err
		}
		return h.recordImported(ctx, job)
	})
	if err == nil && h.Metrics != nil {
		h.Metrics.ImportFinished(job.Kind, job.Status, job.RejectedCount)
	}
	return err
}

// CreateDraftInput is the request of createDraftFromImportItem.
type CreateDraftInput struct {
	ImportID   uuid.UUID
	ItemNo     int
	MathAssets []DraftMathAsset
}

// DraftMathAsset is one rendered formula sent by the portal.
type DraftMathAsset struct {
	Latex      string `json:"latex"`
	SVG        string `json:"svg"`
	SpeechText string `json:"speech_text"`
}

// CreateDraft creates the version 1 draft of an analysed question (F028-AC-09…13): the content is the one analysed
// on the server, the request brings only the SVG of exactly its formulas; pictures go to the media pipeline; the
// validators of createQuestion run; the item becomes created (or draft_failed with the refusal).
func (h DocxImport) CreateDraft(ctx context.Context, in CreateDraftInput) (out dto.ImportItem, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateDraftFromImportItem")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentImport)
	if err != nil {
		return out, err
	}
	var refused error
	err = h.Content.Questions.UoW.Do(ctx, func(ctx context.Context) error {
		job, err := h.Jobs.ImportJob(ctx, in.ImportID)
		if err != nil {
			return err
		}
		if job.Kind != KindLessonsDocx || job.Status != ImportSucceeded {
			return itemConflict(SubImportItemNotImportable, "the import is not an analysed Word import")
		}
		it, err := h.Items.ImportItem(ctx, in.ImportID, in.ItemNo)
		if err != nil {
			return err
		}
		switch {
		case it.Status == docximport.StatusCreated:
			return itemConflict(SubImportItemCreated, it.Label+" was already created")
		case !docximport.Importable(it.Status):
			return itemConflict(SubImportItemNotImportable, it.Label+" has errors and cannot be imported")
		}
		mathRaw, err := matchFormulas(it.Formulas, in.MathAssets)
		if err != nil {
			return err
		}
		fields, err := h.withImages(ctx, job, it, p.UserID)
		if err != nil {
			return err
		}
		fields["math_assets"] = mathRaw
		f, err := h.Content.fields(ctx, job.Owner, dto.QuestionRow{Row: it.No, Fields: fields, Codes: codesOf(it.Payload.Codes)})
		var created QuestionResult
		if err == nil {
			err = h.Content.Questions.UoW.Savepoint(ctx, func(ctx context.Context) error {
				created, err = h.Content.Questions.createDraftWith(ctx, job.Owner, p.UserID, f,
					map[string]any{"import_id": job.ID.String(), "item_no": it.No})
				return err
			})
		}
		var pe *problem.Error
		if err != nil {
			if !errors.As(err, &pe) || pe.Code != problem.ValidationFailed {
				return err
			}
			// F028-AC-12: the refusal is kept on the item; the 422 is returned after the commit.
			it.Status, it.Issues, it.UpdatedAt = docximport.StatusDraftFailed, refusalIssues(it.Issues, pe), now(h.Clock)
			if err := h.Items.UpdateImportItem(ctx, it); err != nil {
				return err
			}
			refused = pe
			out = it
			return nil
		}
		qid, vid := created.Question.ID, created.Version.ID
		it.Status, it.QuestionID, it.QuestionVersionID, it.UpdatedAt = docximport.StatusCreated, &qid, &vid, now(h.Clock)
		it.Issues = withoutRefusal(it.Issues)
		if err := h.Items.UpdateImportItem(ctx, it); err != nil {
			return err
		}
		out = it
		return idempotency.SetResult(ctx, dto.ImportItemJSON(it))
	})
	if err == nil && refused != nil {
		return out, refused
	}
	return out, err
}

func itemConflict(sub, msg string) error {
	pe := problem.New(problem.Conflict, msg)
	pe.AddField("item_no", sub, msg)
	return pe
}

// matchFormulas checks that the request renders exactly the formulas of the item (F028-AC-10) and returns the
// math_assets of the draft.
func matchFormulas(want []docximport.Formula, got []DraftMathAsset) (json.RawMessage, error) {
	have := map[string]bool{}
	for _, f := range want {
		have[f.Latex] = true
	}
	sent := map[string]bool{}
	assets := make([]map[string]any, 0, len(got))
	for i, a := range got {
		if !have[a.Latex] || sent[a.Latex] {
			pe := problem.New(problem.MathRenderFailed, "a formula does not belong to the question")
			pe.AddField("/math_assets/"+strconv.Itoa(i)+"/latex", problem.SubInvalid, "not a formula of the question, or sent twice")
			return nil, pe
		}
		sent[a.Latex] = true
		assets = append(assets, map[string]any{"latex": a.Latex, "svg": a.SVG, "speech_text": a.SpeechText})
	}
	for i, f := range want {
		if !sent[f.Latex] {
			pe := problem.New(problem.MathRenderFailed, "a formula of the question has no rendered SVG")
			pe.AddField("/formulas/"+strconv.Itoa(i), problem.SubRequired, "send the rendered SVG of every formula of the question")
			return nil, pe
		}
	}
	if len(assets) == 0 {
		return nil, nil
	}
	return json.Marshal(assets)
}

// withImages returns the authoring fields of the item with the placeholder media_ref of every picture replaced by
// the media asset it becomes (F028-AC-13).
func (h DocxImport) withImages(ctx context.Context, job dto.ImportJob, it dto.ImportItem, actor uuid.UUID) (map[string]json.RawMessage, error) {
	fields := make(map[string]json.RawMessage, len(it.Payload.Fields)+1)
	for k, v := range it.Payload.Fields {
		fields[k] = v
	}
	if len(it.Images) == 0 {
		return fields, nil
	}
	if h.Media == nil || h.Assets == nil || job.AssetID == nil || h.Converter == nil {
		return nil, problem.New(problem.Unavailable, "the media pipeline is not available").Retry()
	}
	data, err := h.readFile(ctx, *job.AssetID)
	if err != nil {
		return nil, err
	}
	license := ""
	if a, err := h.Media.Store.Asset(ctx, *job.AssetID); err == nil {
		license = a.License
	}
	repl := make([]string, 0, 2*len(it.Images))
	for _, img := range it.Images {
		b, err := h.Converter.Media(data, img.Path)
		if err != nil {
			return nil, problem.Invalid("/media_refs", docximport.CodeImageNotFound, "a picture of the question cannot be read from the file")
		}
		id, err := h.Media.IngestFile(ctx, job.Owner, actor, IngestInput{Kind: media.KindImage, MIME: img.MIME, Filename: path.Base(img.Path),
			AltText: img.Alt, License: license, Data: b})
		if err != nil {
			return nil, err
		}
		repl = append(repl, `"`+img.Ref+`"`, `"`+id.String()+`"`)
	}
	r := strings.NewReplacer(repl...)
	for k, v := range fields {
		fields[k] = json.RawMessage(r.Replace(string(v)))
	}
	return fields, nil
}

func (h DocxImport) readFile(ctx context.Context, id uuid.UUID) ([]byte, error) {
	rc, size, err := h.Assets.Open(ctx, id)
	if err != nil {
		return nil, problem.New(problem.Unavailable, "the Word file is not available").Retry().Wrap(err)
	}
	defer func() { _ = rc.Close() }()
	if size > CsvImportMax {
		return nil, problem.Invalid("asset_id", problem.SubInvalid, "the file is larger than csv_import_max")
	}
	return io.ReadAll(io.LimitReader(rc, CsvImportMax+1))
}

// refusalIssues replaces the previous refusal of the item by the fields of pe (severity error, source validator).
func refusalIssues(prev []docximport.Issue, pe *problem.Error) []docximport.Issue {
	out := withoutRefusal(prev)
	for _, f := range rowErrors(0, pe) {
		if len(out) >= 50 {
			break
		}
		out = append(out, docximport.Issue{Code: f.Code, Field: f.Field, Message: f.Message, Severity: docximport.SevError, Source: docximport.SourceValidator})
	}
	return out
}

func withoutRefusal(is []docximport.Issue) []docximport.Issue {
	out := make([]docximport.Issue, 0, len(is))
	for _, i := range is {
		if i.Source != docximport.SourceValidator {
			out = append(out, i)
		}
	}
	return out
}
