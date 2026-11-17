package api

import "net/http"

type ServerInterface interface{}
type StrictServerInterface interface{}

func Handler(si ServerInterface) http.Handler                              { return nil }
func HandlerFromMux(si ServerInterface, m *http.ServeMux) http.Handler     { return nil }
func NewStrictHandler(ssi StrictServerInterface, mw []any) ServerInterface { return nil }
func AllowedCallersOf(op string) []string                                  { return nil }
