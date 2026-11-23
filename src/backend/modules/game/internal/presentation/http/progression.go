package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ProgressionQueries are the progression read use cases of a student (query.Progression, F074).
type ProgressionQueries interface {
	GetProgression(ctx context.Context) (query.ProgressionStatus, error)
	GetXPStatus(ctx context.Context) (query.XPStatus, error)
	ListXPHistory(ctx context.Context, limit int, cursor, lang string) (query.XPHistoryPage, error)
	ListLevels(ctx context.Context) (query.LevelsView, error)
}

// ProgressionOperations are the player routes of F074 (F074-AC-19: staff tokens get 403 GAME_ACCESS_STUDENT_ONLY).
var ProgressionOperations = []string{"getProgression", "listXpHistory", "listProgressionLevels", "getXpStatus"}

func (h Handlers) progressionRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"getProgression":        h.getProgression,
		"listXpHistory":         h.listXpHistory,
		"listProgressionLevels": h.listProgressionLevels,
		"getXpStatus":           h.getXpStatus,
	}
}

// lang is the language of the friendly labels (Accept-Language: en… → en, otherwise vi).
func lang(r *http.Request) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Accept-Language"))), "en") {
		return "en"
	}
	return "vi"
}

func contentLanguage(w http.ResponseWriter, l string) {
	if l == "en" {
		w.Header().Set("Content-Language", "en-US")
		return
	}
	w.Header().Set("Content-Language", "vi-VN")
}

func (h Handlers) getProgression(w http.ResponseWriter, r *http.Request) {
	out, err := h.ProgressionQueries.GetProgression(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getXpStatus(w http.ResponseWriter, r *http.Request) {
	out, err := h.ProgressionQueries.GetXPStatus(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listXpHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 0
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			fail(w, r, problem.Invalid("limit", problem.SubInvalid, "an integer"))
			return
		}
		limit = n
	}
	l := lang(r)
	out, err := h.ProgressionQueries.ListXPHistory(r.Context(), limit, q.Get("cursor"), l)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, l)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listProgressionLevels(w http.ResponseWriter, r *http.Request) {
	out, err := h.ProgressionQueries.ListLevels(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}
