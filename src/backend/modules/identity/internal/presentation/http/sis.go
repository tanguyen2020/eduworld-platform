package http

import (
	"net/http"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
)

// F011 operations (internal gateway, x-internal).
var sisOperations = map[string]func(Handlers) http.HandlerFunc{
	"listSisReconciliationIssues":   func(h Handlers) http.HandlerFunc { return h.listSisIssues },
	"resolveSisReconciliationIssue": func(h Handlers) http.HandlerFunc { return h.resolveSisIssue },
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
