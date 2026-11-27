package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

var cmsCurriculumRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listCmsCurricula":               Handlers.listCmsCurricula,
	"createCurriculum":               Handlers.createCurriculum,
	"getCmsCurriculum":               Handlers.getCmsCurriculum,
	"updateCurriculum":               Handlers.updateCurriculum,
	"deleteCurriculum":               Handlers.deleteCurriculum,
	"listCurriculumVersions":         Handlers.listCurriculumVersions,
	"createCurriculumVersion":        Handlers.createCurriculumVersion,
	"getCurriculumVersion":           Handlers.getCurriculumVersion,
	"updateCurriculumVersionDraft":   Handlers.updateCurriculumVersionDraft,
	"transitionCurriculumVersion":    Handlers.transitionCurriculumVersion,
	"publishCurriculumVersion":       Handlers.publishCurriculumVersion,
	"exportCurriculumVersion":        Handlers.exportCurriculumVersion,
	"getCurriculumVersionLoMappings": Handlers.getCurriculumVersionLoMappings,
	"putCurriculumVersionLoMappings": Handlers.putCurriculumVersionLoMappings,
	"createContentImport":            Handlers.createContentImport,
	"getContentImport":               Handlers.getContentImport,
	"listContentImportItems":         Handlers.listContentImportItems,
	"createDraftFromImportItem":      Handlers.createDraftFromImportItem,
}

func date(s *string, field string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	d, err := time.Parse(time.DateOnly, *s)
	if err != nil {
		return nil, problem.Invalid(field, problem.SubInvalid, "must be a date YYYY-MM-DD")
	}
	return &d, nil
}

type versionBody struct {
	VersionLabel    string  `json:"version_label"`
	EffectiveFrom   *string `json:"effective_from"`
	EffectiveTo     *string `json:"effective_to"`
	ChangeNote      *string `json:"change_note"`
	ContentLanguage string  `json:"content_language"`
}

func (b versionBody) input() (command.VersionInput, error) {
	from, err := date(b.EffectiveFrom, "effective_from")
	if err != nil {
		return command.VersionInput{}, err
	}
	to, err := date(b.EffectiveTo, "effective_to")
	if err != nil {
		return command.VersionInput{}, err
	}
	return command.VersionInput{Label: b.VersionLabel, EffectiveFrom: from, EffectiveTo: to, ChangeNote: b.ChangeNote,
		ContentLanguage: b.ContentLanguage}, nil
}

func (h Handlers) listCmsCurricula(w http.ResponseWriter, r *http.Request) {
	p, err := h.Queries.ListCurricula(r.Context(), r.URL.Query().Get("owner_scope"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.CurriculumJSON)
}

func (h Handlers) createCurriculum(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Code           string            `json:"code"`
		Name           map[string]string `json:"name"`
		Framework      *string           `json:"framework"`
		TenantID       *uuid.UUID        `json:"tenant_id"`
		InitialVersion *versionBody      `json:"initial_version"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := command.CreateCurriculumInput{Code: b.Code, Name: b.Name, Framework: b.Framework, TenantID: b.TenantID}
	if b.InitialVersion != nil {
		v, err := b.InitialVersion.input()
		if err != nil {
			fail(w, r, err)
			return
		}
		in.Version = v
	}
	c, err := h.Commands.CreateCurriculum(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, c.RowVersion)
	writeJSON(w, http.StatusCreated, dto.CurriculumJSON(c))
}

func (h Handlers) getCmsCurriculum(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "curriculum_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	c, err := h.Queries.GetCurriculum(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, c.RowVersion)
	writeJSON(w, http.StatusOK, dto.CurriculumJSON(c))
}

func (h Handlers) updateCurriculum(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "curriculum_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.UpdateCurriculumInput{ID: id, IfMatch: v}
	if _, err = p.into("name", &in.Name); err == nil {
		in.Framework, err = nullable[string](p, "framework")
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	c, err := h.Commands.UpdateCurriculum(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, c.RowVersion)
	writeJSON(w, http.StatusOK, dto.CurriculumJSON(c))
}

type reasonBody struct {
	Reason string `json:"reason"`
}

func readReason(r *http.Request) (string, error) {
	var b reasonBody
	if r.ContentLength == 0 {
		return "", nil
	}
	if err := decode(r, &b); err != nil {
		return "", err
	}
	return b.Reason, nil
}

func (h Handlers) deleteCurriculum(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "curriculum_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	reason, err := readReason(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Commands.DeleteCurriculum(r.Context(), command.DeleteCurriculumInput{ID: id, IfMatch: v, Reason: reason}); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) listCurriculumVersions(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "curriculum_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListVersions(r.Context(), id, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.VersionJSON)
}

func (h Handlers) createCurriculumVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "curriculum_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		FromVersion int `json:"from_version"`
		versionBody     // version_label, effective_*, change_note
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	vin, err := b.input()
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.CreateVersion(r.Context(), command.NewVersionInput{CurriculumID: id, FromVersion: b.FromVersion, Label: vin.Label,
		EffectiveFrom: vin.EffectiveFrom, EffectiveTo: vin.EffectiveTo, ChangeNote: vin.ChangeNote})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.RowVersion)
	writeJSON(w, http.StatusCreated, dto.VersionJSON(v))
}

func versionPath(r *http.Request) (uuid.UUID, int, error) {
	id, err := pathUUID(r, "curriculum_id")
	if err != nil {
		return id, 0, err
	}
	n, err := pathVersion(r)
	return id, n, err
}

func (h Handlers) getCurriculumVersion(w http.ResponseWriter, r *http.Request) {
	id, n, err := versionPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Queries.GetVersion(r.Context(), id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.RowVersion)
	writeJSON(w, http.StatusOK, dto.VersionJSON(v))
}

// subjectGradeBody is SubjectGrade of a request.
type subjectGradeBody struct {
	SubjectID      uuid.UUID `json:"subject_id"`
	GradeLevel     int       `json:"grade_level"`
	PeriodsPerYear *int      `json:"periods_per_year"`
}

func (h Handlers) updateCurriculumVersionDraft(w http.ResponseWriter, r *http.Request) {
	id, n, err := versionPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.UpdateVersionDraftInput{CurriculumID: id, VersionNo: n, IfMatch: rv}
	var sgs *[]subjectGradeBody
	steps := []func() error{
		func() (e error) { in.Label, e = optionalOf[string](p, "version_label"); return },
		func() (e error) { in.ChangeNote, e = optionalOf[string](p, "change_note"); return },
		func() error {
			from, e := nullable[string](p, "effective_from")
			if e == nil {
				in.EffectiveFrom, e = datePatch(from, "effective_from")
			}
			return e
		},
		func() error {
			to, e := nullable[string](p, "effective_to")
			if e == nil {
				in.EffectiveTo, e = datePatch(to, "effective_to")
			}
			return e
		},
		func() (e error) { sgs, e = optionalOf[[]subjectGradeBody](p, "subject_grades"); return },
	}
	for _, s := range steps {
		if err := s(); err != nil {
			fail(w, r, err)
			return
		}
	}
	if sgs != nil {
		list := make([]dto.SubjectGrade, len(*sgs))
		for i, x := range *sgs {
			list[i] = dto.SubjectGrade{SubjectID: x.SubjectID, GradeLevel: x.GradeLevel, PeriodsPerYear: x.PeriodsPerYear}
		}
		in.SubjectGrades = &list
	}
	v, err := h.Commands.UpdateVersionDraft(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.RowVersion)
	writeJSON(w, http.StatusOK, dto.VersionJSON(v))
}

func datePatch(p command.Patch[string], field string) (command.Patch[time.Time], error) {
	if !p.Set || p.Value == nil {
		return command.Patch[time.Time]{Set: p.Set}, nil
	}
	d, err := date(p.Value, field)
	if err != nil {
		return command.Patch[time.Time]{}, err
	}
	return command.Patch[time.Time]{Set: true, Value: d}, nil
}

func (h Handlers) transitionCurriculumVersion(w http.ResponseWriter, r *http.Request) {
	id, n, err := versionPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Transition(r.Context(), command.TransitionInput{CurriculumID: id, VersionNo: n, IfMatch: rv, Action: b.Action, Note: b.Note})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.RowVersion)
	writeJSON(w, http.StatusOK, dto.VersionJSON(v))
}

func (h Handlers) publishCurriculumVersion(w http.ResponseWriter, r *http.Request) {
	id, n, err := versionPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Publish(r.Context(), command.PublishInput{CurriculumID: id, VersionNo: n, IfMatch: rv})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.RowVersion)
	writeJSON(w, http.StatusOK, dto.VersionJSON(v))
}

func (h Handlers) exportCurriculumVersion(w http.ResponseWriter, r *http.Request) {
	id, n, err := versionPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "yaml"
	}
	b, err := h.Queries.Export(r.Context(), id, n, format)
	if err != nil {
		fail(w, r, err)
		return
	}
	ct := "application/yaml"
	if format == "csv" {
		ct = "text/csv; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (h Handlers) getCurriculumVersionLoMappings(w http.ResponseWriter, r *http.Request) {
	id, n, err := versionPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	m, rv, err := h.Queries.LoMappings(r.Context(), id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, rv)
	writeJSON(w, http.StatusOK, m)
}

func (h Handlers) putCurriculumVersionLoMappings(w http.ResponseWriter, r *http.Request) {
	id, n, err := versionPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Items curriculum.MappingSet `json:"items"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	m, err := h.Commands.PutLoMappings(r.Context(), command.LoMappingsInput{CurriculumID: id, VersionNo: n, IfMatch: rv, Items: b.Items})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, m.Version.RowVersion)
	writeJSON(w, http.StatusOK, m.View())
}

func (h Handlers) createContentImport(w http.ResponseWriter, r *http.Request) {
	var b struct {
		AssetID uuid.UUID `json:"asset_id"`
		Kind    string    `json:"kind"`
		DryRun  bool      `json:"dry_run"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	j, err := h.Commands.CreateImport(r.Context(), command.CreateImportInput{Kind: b.Kind, AssetID: b.AssetID, DryRun: b.DryRun})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, dto.ImportJobJSON(j))
}

func (h Handlers) getContentImport(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "import_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	j, err := h.Queries.GetImport(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.ImportJobJSON(j))
}
