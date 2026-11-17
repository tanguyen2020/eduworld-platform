package app

import (
	"net/http"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
)

func mount(mux *http.ServeMux, s api.StrictServerInterface) {
	_ = api.AllowedCallersOf("resolvePrincipal")
	h := api.NewStrictHandler(s, nil)        // want `api.NewStrictHandler would serve a module facade over HTTP`
	mux.Handle("/internal/", api.Handler(h)) // want `api.Handler would serve a module facade over HTTP`
	_ = api.HandlerFromMux(h, mux)           // want `api.HandlerFromMux would serve a module facade over HTTP`
}
