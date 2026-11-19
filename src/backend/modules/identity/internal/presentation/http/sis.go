package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F011 operations (internal gateway, x-internal).
var sisOperations = map[string]func(Handlers) http.HandlerFunc{
	"listSisReconciliationIssues":   func(h Handlers) http.HandlerFunc { return h.listSisIssues },
	"resolveSisReconciliationIssue": func(h Handlers) http.HandlerFunc { return h.resolveSisIssue },
	"createSisImport":               func(h Handlers) http.HandlerFunc { return h.createSisImport },
	"getSisImport":                  func(h Handlers) http.HandlerFunc { return h.getSisImport },
	"createPlatformSisImport":       func(h Handlers) http.HandlerFunc { return h.createPlatformSisImport },
	// F052-1: the school-scoped public view of the issues (public gateway, School Admin / Tenant Admin)
	"listSchoolReconciliationIssues":   func(h Handlers) http.HandlerFunc { return h.listSchoolSisIssues },
	"resolveSchoolReconciliationIssue": func(h Handlers) http.HandlerFunc { return h.resolveSchoolSisIssue },
}

func (h Handlers) listSchoolSisIssues(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	imp, err := optUUID(r, "import_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	out, err := h.Sis.ListSchoolSisIssues(r.Context(), query.SchoolSisIssuesInput{ListInput: list(r), SchoolID: school, ImportID: imp,
		Type: q.Get("issue_type"), Status: q.Get("status")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) resolveSchoolSisIssue(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "issue_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var body struct {
		Resolution string `json:"resolution"`
		Reason     string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.ResolveSisIssue.Handle(r.Context(), command.ResolveSisIssueInput{IssueID: id, Resolution: body.Resolution, Note: body.Reason,
		SchoolID: &school})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listSisIssues(w http.ResponseWriter, r *http.Request) {
	school, err := optUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	conn, err := optUUID(r, "sis_connection_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	out, err := h.Sis.ListSisIssues(r.Context(), query.SisIssuesInput{ListInput: list(r), SchoolID: school, ConnectionID: conn,
		Type: q.Get("issue_type"), Status: q.Get("status")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) resolveSisIssue(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "issue_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var body struct {
		Resolution string `json:"resolution"`
		Note       string `json:"resolution_note"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.ResolveSisIssue.Handle(r.Context(), command.ResolveSisIssueInput{IssueID: id, Resolution: body.Resolution, Note: body.Note})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type sisImportBody struct {
	SchoolID    *uuid.UUID `json:"school_id"`
	FileName    string     `json:"file_name"`
	Size        int64      `json:"size_bytes"`
	SHA256      string     `json:"sha256"`
	EntityKinds []string   `json:"entity_kinds"`
	DryRun      *bool      `json:"dry_run"`
}

func (h Handlers) createSisImport(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	h.sisImport(w, r, &school, false)
}

func (h Handlers) createPlatformSisImport(w http.ResponseWriter, r *http.Request) {
	h.sisImport(w, r, nil, true)
}

func (h Handlers) sisImport(w http.ResponseWriter, r *http.Request, school *uuid.UUID, platform bool) {
	var b sisImportBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	if school == nil {
		if b.SchoolID == nil {
			fail(w, r, problem.Invalid("school_id", problem.SubRequired, "required"))
			return
		}
		school = b.SchoolID
	}
	out, err := h.CreateSisImport.Handle(r.Context(), command.CreateSisImportInput{SchoolID: *school, FileName: b.FileName, Size: b.Size,
		SHA256: b.SHA256, EntityKinds: b.EntityKinds, DryRun: b.DryRun, Platform: platform})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h Handlers) getSisImport(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "import_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.SisImports.Get(r.Context(), school, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
