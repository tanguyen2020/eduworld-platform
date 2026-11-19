package command

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Question import kinds handled by ContentImport (F024 US4); lessons_docx is refused (IMPORT_KIND_NOT_SUPPORTED).
var QuestionImportKinds = []string{"questions_csv", "questions_xlsx", "questions_qti_lite", "content_json"}

// SubImportKindNotSupported is errors[].code of an import kind outside the release (lessons_docx).
const SubImportKindNotSupported = "IMPORT_KIND_NOT_SUPPORTED"

// QuestionImportCodec parses question import files (infrastructure/importer).
type QuestionImportCodec interface {
	Questions(kind string, data []byte) ([]dto.QuestionRow, []dto.ImportRowError)
}

// ContentImport imports question files into drafts (F024-AC-13…16): every row is validated by the validators of
// createQuestion in its own savepoint; valid rows become version 1 drafts of the owner of the job, rejected rows
// are reported by number; nothing is written on dry run.
type ContentImport struct {
	Questions Questions
	Codec     QuestionImportCodec
}

// Run imports the parsed file of job; it returns the number of drafts created (or that would be created).
func (h ContentImport) Run(ctx context.Context, job dto.ImportJob, data []byte) (int, []dto.ImportRowError, error) {
	rows, errs := h.Codec.Questions(job.Kind, data)
	if len(rows) == 0 {
		return 0, errs, nil
	}
	actor := uuid.Nil
	if job.CreatedBy != nil {
		actor = *job.CreatedBy
	}
	created := 0
	err := h.Questions.UoW.Do(ctx, func(ctx context.Context) error {
		for _, r := range rows {
			err := h.Questions.UoW.Savepoint(ctx, func(ctx context.Context) error {
				f, err := h.fields(ctx, job.Owner, r)
				if err != nil {
					return err
				}
				_, err = h.Questions.createDraft(ctx, job.Owner, actor, f)
				return err
			})
			if err != nil {
				var pe *problem.Error
				if !errors.As(err, &pe) {
					return err
				}
				errs = append(errs, rowErrors(r.Row, pe)...)
				continue
			}
			created++
		}
		if job.DryRun {
			return errRollback
		}
		return nil
	})
	if errors.Is(err, errRollback) {
		err = nil
	}
	slices.SortStableFunc(errs, func(a, b dto.ImportRowError) int { return a.Row - b.Row })
	if len(errs) > 500 {
		errs = errs[:500]
	}
	return created, errs, err
}

// rowErrors maps a problem of one row to its report; a formula without rendered SVG is MATH_RENDER_FAILED
// (F024-AC-14).
func rowErrors(row int, pe *problem.Error) []dto.ImportRowError {
	if len(pe.Fields) == 0 {
		return []dto.ImportRowError{{Row: row, Code: string(pe.Code), Message: truncate(pe.Detail, 500)}}
	}
	out := make([]dto.ImportRowError, 0, len(pe.Fields))
	for _, f := range pe.Fields {
		code := f.Code
		if pe.Code == problem.MathRenderFailed || f.Field == "/math_assets" {
			code = string(problem.MathRenderFailed)
		}
		out = append(out, dto.ImportRowError{Row: row, Field: truncate(f.Field, 128), Code: truncate(code, 64), Message: truncate(f.Message, 500)})
	}
	return out
}

// fields builds the authoring fields of a row: JSON columns as is, curriculum codes resolved in the scope of the
// job (the curriculum of the owner, or a platform one visible to it).
func (h ContentImport) fields(ctx context.Context, o contentmodel.Owner, r dto.QuestionRow) (QuestionFields, error) {
	f := QuestionFields{Present: map[string]bool{}}
	dec := func(k string, v any) error {
		raw, ok := r.Fields[k]
		if !ok {
			return nil
		}
		f.Present[k] = true
		if err := json.Unmarshal(raw, v); err != nil {
			return problem.Invalid("/"+k, problem.SubInvalid, "has the wrong type")
		}
		return nil
	}
	raw := func(k string, dst *json.RawMessage) {
		if v, ok := r.Fields[k]; ok {
			f.Present[k] = true
			*dst = v
		}
	}
	raw("stem", &f.Stem)
	raw("render_payload", &f.RenderPayload)
	raw("answer_key", &f.AnswerKey)
	raw("scoring_rule", &f.ScoringRule)
	raw("explanation", &f.Explanation)
	raw("hints", &f.Hints)
	raw("math_assets", &f.MathAssets)
	raw("worked_example_ref", &f.WorkedExampleRef)
	for _, x := range []struct {
		k string
		v any
	}{{"type", &f.Type}, {"max_score", &f.MaxScore}, {"tags", &f.Tags}, {"difficulty", &f.Difficulty}, {"grade_levels", &f.GradeLevels},
		{"subject_id", &f.SubjectID}, {"topic_id", &f.TopicID}, {"curriculum_version_id", &f.CurriculumVersionID}, {"objective_ids", &f.ObjectiveIDs},
		{"media_refs", &f.MediaRefs}, {"content_language", &f.ContentLanguage}, {"min_response_ms", &f.MinResponseMS}, {"exam_only", &f.ExamOnly}} {
		if err := dec(x.k, x.v); err != nil {
			return f, err
		}
	}
	if r.Codes == nil {
		return f, nil
	}
	c := r.Codes
	subjects, err := h.Questions.Catalog.SubjectsByCode(ctx, []string{c.Subject})
	s, ok := subjects[c.Subject]
	if err != nil || !ok {
		return f, problem.Invalid("subject_code", problem.SubInvalid, "unknown subject "+c.Subject)
	}
	cur, err := h.Questions.Curriculum.CurriculumByCode(ctx, o, c.Curriculum)
	if (err != nil || cur == nil) && !o.Platform() {
		cur, err = h.Questions.Curriculum.CurriculumByCode(ctx, contentmodel.Owner{Scope: contentmodel.OwnerPlatform}, c.Curriculum)
	}
	if err != nil || cur == nil {
		return f, problem.Invalid("curriculum_code", problem.SubInvalid, "unknown curriculum "+c.Curriculum)
	}
	var cv dto.CurriculumVersion
	switch {
	case c.CurriculumVersion > 0:
		cv, err = h.Questions.Curriculum.Version(ctx, cur.ID, c.CurriculumVersion)
	case cur.CurrentVersionID != nil:
		cv, err = h.Questions.Curriculum.VersionByID(ctx, *cur.CurrentVersionID)
	default:
		err = problem.New(problem.NotFound, "")
	}
	if err != nil {
		return f, problem.Invalid("curriculum_version", problem.SubInvalid, "no such version of "+c.Curriculum)
	}
	topics, err := h.Questions.Curriculum.Topics(ctx, cv.ID)
	if err != nil {
		return f, err
	}
	topic := uuid.Nil
	for _, t := range topics {
		if t.Code == c.Topic {
			topic = t.ID
		}
	}
	if topic == uuid.Nil {
		return f, problem.Invalid("topic_code", problem.SubInvalid, "unknown topic "+c.Topic)
	}
	los, err := h.Questions.Curriculum.Objectives(ctx, cv.ID)
	if err != nil {
		return f, err
	}
	var objectives []uuid.UUID
	for _, code := range c.Objectives {
		i := slices.IndexFunc(los, func(o dto.Objective) bool { return o.Code == code })
		if i < 0 {
			return f, problem.Invalid("objective_codes", problem.SubInvalid, "unknown objective "+code)
		}
		objectives = append(objectives, los[i].ID)
	}
	sid, cvid, diff := s.ID, cv.ID, c.Difficulty
	f.SubjectID, f.TopicID, f.CurriculumVersionID, f.ObjectiveIDs, f.GradeLevels, f.Difficulty, f.Tags = &sid, &topic, &cvid, objectives, c.Grades, &diff, c.Tags
	for _, k := range []string{"subject_id", "topic_id", "curriculum_version_id", "objective_ids", "grade_levels", "difficulty", "tags"} {
		f.Present[k] = true
	}
	if c.Language != "" {
		f.ContentLanguage, f.Present["content_language"] = c.Language, true
	}
	return f, nil
}

// createDraft creates a question with version 1 in draft for owner o (the core of CreateQuestion without the
// authorization of an HTTP principal: the import job was authorized when it was created).
func (h Questions) createDraft(ctx context.Context, o contentmodel.Owner, author uuid.UUID, f QuestionFields) (QuestionResult, error) {
	q := dto.Question{ID: ids.New(), Owner: o, LatestVersionNo: 1}
	v := dto.QuestionVersion{ID: ids.New(), QuestionID: q.ID, VersionNo: 1, Status: contentmodel.StatusDraft, MaxScore: 1, ContentLanguage: "vi",
		AuthorID: author, Owner: o}
	var authRender json.RawMessage
	f.apply(&v, &authRender)
	if err := h.prepare(ctx, &v, authRender); err != nil {
		return QuestionResult{}, err
	}
	next, _ := questionDoc(v)
	v.TranslationStatus, v.TranslationMeta = translation(nil, next, v.ContentLanguage, nil, now(h.Clock))
	if err := h.Store.InsertQuestion(ctx, q); err != nil {
		return QuestionResult{}, err
	}
	if err := h.Store.InsertQuestionVersion(ctx, v); err != nil {
		return QuestionResult{}, err
	}
	if err := h.recordUsages(ctx, v); err != nil {
		return QuestionResult{}, err
	}
	if err := h.recordQuestion(ctx, o, audit.Event{Action: ActionContentVersionCreated, TargetID: q.ID.String(),
		Metadata: map[string]any{"question_version_id": v.ID.String(), "version": 1, "type": v.Type, "source": "import"}}); err != nil {
		return QuestionResult{}, err
	}
	return h.load(ctx, q.ID, 1)
}
