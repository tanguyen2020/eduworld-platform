package http

import (
	"net/http"
)

// F012 US2: the session user's own reads (portal session bootstrap after portalLogin, ADR-0091).
var meOperations = map[string]func(Handlers) http.HandlerFunc{
	"listMyTenants":    func(h Handlers) http.HandlerFunc { return h.listMyTenants },
	"getMyPermissions": func(h Handlers) http.HandlerFunc { return h.getMyPermissions },
	"getMe":            func(h Handlers) http.HandlerFunc { return h.getMe },
}

func (h Handlers) listMyTenants(w http.ResponseWriter, r *http.Request) {
	out, err := h.Me.ListMyTenants(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getMyPermissions(w http.ResponseWriter, r *http.Request) {
	out, err := h.Me.GetMyPermissions(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getMe(w http.ResponseWriter, r *http.Request) {
	out, err := h.Me.GetMe(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
