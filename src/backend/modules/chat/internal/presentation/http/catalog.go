package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CatalogCommands are the write use cases of the sensitive-term catalogue (command.CatalogAdmin, F116 US2).
type CatalogCommands interface {
	CreateCategory(ctx context.Context, in command.CategoryInput) (catalog.StoredCategory, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, expected int64, in command.CategoryInput) (catalog.StoredCategory, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
	CreateTerm(ctx context.Context, in command.TermInput) (catalog.StoredTerm, error)
	UpdateTerm(ctx context.Context, id uuid.UUID, expected int64, in command.TermInput) (catalog.StoredTerm, error)
	DeleteTerm(ctx context.Context, id uuid.UUID) error
	CreateAllow(ctx context.Context, in command.AllowInput) (catalog.StoredAllow, error)
	DeleteAllow(ctx context.Context, id uuid.UUID) error
	PublishCatalog(ctx context.Context, summary string) (catalog.StoredVersion, error)
	CreateTenantTerm(ctx context.Context, in command.TermInput) (catalog.StoredTerm, error)
	UpdateTenantTerm(ctx context.Context, id uuid.UUID, expected int64, in command.TermInput) (catalog.StoredTerm, error)
	DeleteTenantTerm(ctx context.Context, id uuid.UUID) error
	PublishTenantCatalog(ctx context.Context, summary string) (catalog.StoredVersion, error)
	ToggleTenantCategory(ctx context.Context, code string, enabled bool, reason string) (command.TenantCategory, error)
}

// CatalogQueries are the read use cases of the catalogue (query.Catalog).
type CatalogQueries interface {
	ListCategories(ctx context.Context) ([]read.CategoryRow, error)
	ListTerms(ctx context.Context, category *uuid.UUID, text string, limit int, cursor string) (query.TermPage, error)
	ListTenantTerms(ctx context.Context, category *uuid.UUID, limit int, cursor string) (query.TermPage, error)
	ListAllowlist(ctx context.Context, limit int, cursor string) (query.AllowPage, error)
	ListVersions(ctx context.Context, limit int, cursor string) (query.VersionPage, error)
	TestTerms(ctx context.Context, in query.TestInput) (query.TestResult, error)
	ListTenantCategories(ctx context.Context) (query.TenantCategories, error)
}

// CatalogHandlers serve the catalogue operations (core-api only). The /admin/ routes are internal-gateway routes:
// httpx adds GuardPortalStaff + GuardPlatformRole (platform role; no MFA, ADR-0094); /tenant/chat/categories gets GuardPortalStaff.
// Authorization of the permission is in each use case (authz.Require, R7).
type CatalogHandlers struct {
	Commands CatalogCommands
	Queries  CatalogQueries
}

// CatalogOperations are the F116 US2 operations (method, path, X-Tenant-Id mode, Idempotency-Key and success status of
// specs/openapi/eduworld-api.v1.yaml; checked by the contract test).
var CatalogOperations = []string{"listSensitiveCategories", "createSensitiveCategory", "updateSensitiveCategory",
	"deleteSensitiveCategory", "listSensitiveTerms", "createSensitiveTerm", "updateSensitiveTerm", "deleteSensitiveTerm",
	"testSensitiveTerms", "listSensitiveAllowlist", "createSensitiveAllowlistEntry", "deleteSensitiveAllowlistEntry",
	"publishSensitiveCatalog", "listSensitiveCatalogVersions", "listTenantSensitiveTerms", "createTenantSensitiveTerm",
	"updateTenantSensitiveTerm", "deleteTenantSensitiveTerm", "publishTenantSensitiveCatalog", "listTenantSensitiveCategories",
	"toggleTenantSensitiveCategory"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "listSensitiveCategories", Module: "chat", Method: http.MethodGet, Pattern: "/admin/chat/sensitive-categories",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
		{ID: "createSensitiveCategory", Module: "chat", Method: http.MethodPost, Pattern: "/admin/chat/sensitive-categories",
			Tenant: httpx.TenantOptional, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "updateSensitiveCategory", Module: "chat", Method: http.MethodPatch, Pattern: "/admin/chat/sensitive-categories/{category_id}",
			Tenant: httpx.TenantOptional, SuccessStatus: http.StatusOK},
		{ID: "deleteSensitiveCategory", Module: "chat", Method: http.MethodDelete, Pattern: "/admin/chat/sensitive-categories/{category_id}",
			Tenant: httpx.TenantOptional, SuccessStatus: http.StatusNoContent},
		{ID: "listSensitiveTerms", Module: "chat", Method: http.MethodGet, Pattern: "/admin/chat/sensitive-terms",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
		{ID: "createSensitiveTerm", Module: "chat", Method: http.MethodPost, Pattern: "/admin/chat/sensitive-terms",
			Tenant: httpx.TenantOptional, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "updateSensitiveTerm", Module: "chat", Method: http.MethodPatch, Pattern: "/admin/chat/sensitive-terms/{term_id}",
			Tenant: httpx.TenantOptional, SuccessStatus: http.StatusOK},
		{ID: "deleteSensitiveTerm", Module: "chat", Method: http.MethodDelete, Pattern: "/admin/chat/sensitive-terms/{term_id}",
			Tenant: httpx.TenantOptional, SuccessStatus: http.StatusNoContent},
		{ID: "testSensitiveTerms", Module: "chat", Method: http.MethodPost, Pattern: "/admin/chat/sensitive-terms:test",
			Tenant: httpx.TenantOptional, SuccessStatus: http.StatusOK},
		{ID: "listSensitiveAllowlist", Module: "chat", Method: http.MethodGet, Pattern: "/admin/chat/sensitive-allowlist",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
		{ID: "createSensitiveAllowlistEntry", Module: "chat", Method: http.MethodPost, Pattern: "/admin/chat/sensitive-allowlist",
			Tenant: httpx.TenantOptional, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "deleteSensitiveAllowlistEntry", Module: "chat", Method: http.MethodDelete, Pattern: "/admin/chat/sensitive-allowlist/{entry_id}",
			Tenant: httpx.TenantOptional, SuccessStatus: http.StatusNoContent},
		{ID: "publishSensitiveCatalog", Module: "chat", Method: http.MethodPost, Pattern: "/admin/chat/sensitive-catalog:publish",
			Tenant: httpx.TenantOptional, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "listSensitiveCatalogVersions", Module: "chat", Method: http.MethodGet, Pattern: "/admin/chat/sensitive-catalog/versions",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
		{ID: "listTenantSensitiveTerms", Module: "chat", Method: http.MethodGet, Pattern: "/admin/chat/tenant-terms",
			Tenant: httpx.TenantRequired, SuccessStatus: http.StatusOK},
		{ID: "createTenantSensitiveTerm", Module: "chat", Method: http.MethodPost, Pattern: "/admin/chat/tenant-terms",
			Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "updateTenantSensitiveTerm", Module: "chat", Method: http.MethodPatch, Pattern: "/admin/chat/tenant-terms/{term_id}",
			Tenant: httpx.TenantRequired, SuccessStatus: http.StatusOK},
		{ID: "deleteTenantSensitiveTerm", Module: "chat", Method: http.MethodDelete, Pattern: "/admin/chat/tenant-terms/{term_id}",
			Tenant: httpx.TenantRequired, SuccessStatus: http.StatusNoContent},
		{ID: "publishTenantSensitiveCatalog", Module: "chat", Method: http.MethodPost, Pattern: "/admin/chat/tenant-terms:publish",
			Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "listTenantSensitiveCategories", Module: "chat", Method: http.MethodGet, Pattern: "/tenant/chat/categories",
			Tenant: httpx.TenantRequired, SuccessStatus: http.StatusOK},
		{ID: "toggleTenantSensitiveCategory", Module: "chat", Method: http.MethodPost, Pattern: "/admin/chat/tenant-categories/{code}:toggle",
			Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
	} {
		Operations[o.ID] = o
	}
}

// registerCatalog mounts the F116 US2 operations.
func registerCatalog(s *httpx.Server, h *CatalogHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the catalogue handlers are required")
	}
	routes := map[string]http.HandlerFunc{
		"listSensitiveCategories":       h.listCategories,
		"createSensitiveCategory":       h.createCategory,
		"updateSensitiveCategory":       h.updateCategory,
		"deleteSensitiveCategory":       h.deleteByID("category_id", h.Commands.DeleteCategory),
		"listSensitiveTerms":            h.listTerms,
		"createSensitiveTerm":           h.createTerm(h.Commands.CreateTerm),
		"updateSensitiveTerm":           h.updateTerm(h.Commands.UpdateTerm),
		"deleteSensitiveTerm":           h.deleteByID("term_id", h.Commands.DeleteTerm),
		"testSensitiveTerms":            h.testTerms,
		"listSensitiveAllowlist":        h.listAllowlist,
		"createSensitiveAllowlistEntry": h.createAllow,
		"deleteSensitiveAllowlistEntry": h.deleteByID("entry_id", h.Commands.DeleteAllow),
		"publishSensitiveCatalog":       h.publish(h.Commands.PublishCatalog),
		"listSensitiveCatalogVersions":  h.listVersions,
		"listTenantSensitiveTerms":      h.listTenantTerms,
		"createTenantSensitiveTerm":     h.createTerm(h.Commands.CreateTenantTerm),
		"updateTenantSensitiveTerm":     h.updateTerm(h.Commands.UpdateTenantTerm),
		"deleteTenantSensitiveTerm":     h.deleteByID("term_id", h.Commands.DeleteTenantTerm),
		"publishTenantSensitiveCatalog": h.publish(h.Commands.PublishTenantCatalog),
		"listTenantSensitiveCategories": h.listTenantCategories,
		"toggleTenantSensitiveCategory": h.toggle,
	}
	for _, id := range CatalogOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return problem.New(problem.BadRequest, "invalid JSON body")
	}
	return nil
}

func etag(w http.ResponseWriter, version int64) {
	w.Header().Set("ETag", `W/"`+strconv.FormatInt(version, 10)+`"`)
}

// ifMatch parses If-Match (W/"n", "n" or n): missing → 428 PRECONDITION_REQUIRED, malformed → 412 VERSION_MISMATCH.
func ifMatch(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return 0, problem.New(problem.PreconditionRequired, "")
	}
	n, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(raw, "W/"), `"`), 10, 64)
	if err != nil || n < 1 {
		return 0, problem.New(problem.VersionMismatch, "")
	}
	return n, nil
}

func optUUID(r *http.Request, name string) (*uuid.UUID, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil, problem.Invalid(name, problem.SubInvalid, "must be a UUID")
	}
	return &id, nil
}

func limitOf(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return n
}

// ---------- rendering (SensitiveTermCategory, SensitiveTerm, SensitiveAllowlistEntry, SensitiveCatalogVersion)

func categoryBody(c catalog.StoredCategory, termCount *int) map[string]any {
	m := map[string]any{"category_id": c.ID, "code": c.Code, "name": c.Name, "severity": c.Severity, "default_action": c.DefaultAction,
		"enabled": c.Enabled, "allow_tenant_toggle": c.AllowTenantToggle, "updated_at": c.UpdatedAt.UTC()}
	if c.Description != nil {
		m["description"] = *c.Description
	}
	if termCount != nil {
		m["term_count"] = *termCount
	}
	return m
}

func termBody(t catalog.StoredTerm) map[string]any {
	m := map[string]any{"term_id": t.ID, "scope": t.Scope(), "tenant_id": t.TenantID, "category_id": t.CategoryID, "term": t.Term,
		"match_type": t.MatchType, "locale": t.Locale, "enabled": t.Enabled, "status": t.Status, "updated_at": t.UpdatedAt.UTC()}
	if len(t.Variants) > 0 {
		m["variants"] = t.Variants
	}
	if t.SeverityOverride != nil {
		m["severity_override"] = *t.SeverityOverride
	}
	if t.ActionOverride != nil {
		m["action_override"] = *t.ActionOverride
	}
	if t.Note != nil {
		m["note"] = *t.Note
	}
	return m
}

func allowBody(a catalog.StoredAllow) map[string]any {
	m := map[string]any{"entry_id": a.ID, "tenant_id": a.TenantID, "phrase": a.Phrase, "locale": a.Locale, "reason": ""}
	if a.Reason != nil {
		m["reason"] = *a.Reason
	}
	return m
}

func versionBody(v catalog.StoredVersion) map[string]any {
	by := uuid.Nil // the seeded version 1 has no publisher (system)
	if v.PublishedBy != nil {
		by = *v.PublishedBy
	}
	m := map[string]any{"catalog_version": v.Version, "scope": v.Scope, "tenant_id": v.TenantID, "published_at": v.PublishedAt.UTC(),
		"published_by": by, "change_summary": v.ChangeSummary, "term_count": v.TermCount}
	if v.Recall != nil || v.FPRate != nil {
		reg := map[string]any{}
		if v.Recall != nil {
			reg["recall"] = *v.Recall
		}
		if v.FPRate != nil {
			reg["false_positive_rate"] = *v.FPRate
		}
		m["regression"] = reg
	}
	return m
}

// ---------- handlers

func (h CatalogHandlers) listCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Queries.ListCategories(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(rows))
	for i := range rows {
		data[i] = categoryBody(rows[i].StoredCategory, &rows[i].TermCount)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

type categoryIn struct {
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	Severity          string  `json:"severity"`
	DefaultAction     string  `json:"default_action"`
	Enabled           bool    `json:"enabled"`
	AllowTenantToggle bool    `json:"allow_tenant_toggle"`
}

func (b categoryIn) input() command.CategoryInput {
	return command.CategoryInput{Code: b.Code, Name: b.Name, Description: b.Description, Severity: b.Severity,
		DefaultAction: b.DefaultAction, Enabled: b.Enabled, AllowTenantToggle: b.AllowTenantToggle}
}

func (h CatalogHandlers) createCategory(w http.ResponseWriter, r *http.Request) {
	var b categoryIn
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	c, err := h.Commands.CreateCategory(r.Context(), b.input())
	if err != nil {
		fail(w, r, err)
		return
	}
	zero := 0
	etag(w, c.Version)
	writeJSON(w, http.StatusCreated, categoryBody(c, &zero))
}

func (h CatalogHandlers) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "category_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	expected, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b categoryIn
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	c, err := h.Commands.UpdateCategory(r.Context(), id, expected, b.input())
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, c.Version)
	writeJSON(w, http.StatusOK, categoryBody(c, nil))
}

func (h CatalogHandlers) deleteByID(param string, fn func(ctx context.Context, id uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, param)
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := fn(r.Context(), id); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type termIn struct {
	CategoryID       uuid.UUID `json:"category_id"`
	Term             string    `json:"term"`
	MatchType        string    `json:"match_type"`
	Locale           string    `json:"locale"`
	Variants         []string  `json:"variants"`
	SeverityOverride *string   `json:"severity_override"`
	ActionOverride   *string   `json:"action_override"`
	Enabled          bool      `json:"enabled"`
	Note             *string   `json:"note"`
}

func (b termIn) input() command.TermInput {
	return command.TermInput{CategoryID: b.CategoryID, Term: b.Term, MatchType: b.MatchType, Locale: b.Locale, Variants: b.Variants,
		SeverityOverride: b.SeverityOverride, ActionOverride: b.ActionOverride, Enabled: b.Enabled, Note: b.Note}
}

func (h CatalogHandlers) createTerm(fn func(ctx context.Context, in command.TermInput) (catalog.StoredTerm, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b termIn
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
		t, err := fn(r.Context(), b.input())
		if err != nil {
			fail(w, r, err)
			return
		}
		etag(w, t.Version)
		writeJSON(w, http.StatusCreated, termBody(t))
	}
}

func (h CatalogHandlers) updateTerm(fn func(ctx context.Context, id uuid.UUID, expected int64, in command.TermInput) (catalog.StoredTerm, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "term_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		expected, err := ifMatch(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var b termIn
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
		t, err := fn(r.Context(), id, expected, b.input())
		if err != nil {
			fail(w, r, err)
			return
		}
		etag(w, t.Version)
		writeJSON(w, http.StatusOK, termBody(t))
	}
}

func termPage(w http.ResponseWriter, p query.TermPage) {
	data := make([]map[string]any, len(p.Data))
	for i, t := range p.Data {
		data[i] = termBody(t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h CatalogHandlers) listTerms(w http.ResponseWriter, r *http.Request) {
	cat, err := optUUID(r, "category_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListTerms(r.Context(), cat, r.URL.Query().Get("q"), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	termPage(w, p)
}

func (h CatalogHandlers) listTenantTerms(w http.ResponseWriter, r *http.Request) {
	cat, err := optUUID(r, "category_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListTenantTerms(r.Context(), cat, limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	termPage(w, p)
}

func (h CatalogHandlers) testTerms(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Text          string     `json:"text"`
		TenantID      *uuid.UUID `json:"tenant_id"`
		IncludeDrafts *bool      `json:"include_drafts"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	drafts := b.IncludeDrafts == nil || *b.IncludeDrafts
	res, err := h.Queries.TestTerms(r.Context(), query.TestInput{Text: b.Text, TenantID: b.TenantID, IncludeDrafts: drafts})
	if err != nil {
		fail(w, r, err)
		return
	}
	matches := make([]map[string]any, len(res.Matches))
	for i, m := range res.Matches {
		matches[i] = map[string]any{"category_code": m.CategoryCode, "term_id": m.TermID, "severity": m.Severity, "action": m.Action,
			"start": m.Start, "end": m.End}
	}
	body := map[string]any{"decision": res.Decision, "matches": matches}
	if res.MaskedText != nil {
		body["masked_text"] = *res.MaskedText
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, body)
}

func (h CatalogHandlers) listAllowlist(w http.ResponseWriter, r *http.Request) {
	p, err := h.Queries.ListAllowlist(r.Context(), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, a := range p.Data {
		data[i] = allowBody(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h CatalogHandlers) createAllow(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Phrase string  `json:"phrase"`
		Locale string  `json:"locale"`
		Reason *string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	a, err := h.Commands.CreateAllow(r.Context(), command.AllowInput{Phrase: b.Phrase, Locale: b.Locale, Reason: b.Reason})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, allowBody(a))
}

func (h CatalogHandlers) publish(fn func(ctx context.Context, summary string) (catalog.StoredVersion, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			ChangeSummary string `json:"change_summary"`
		}
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
		v, err := fn(r.Context(), b.ChangeSummary)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, versionBody(v))
	}
}

func (h CatalogHandlers) listVersions(w http.ResponseWriter, r *http.Request) {
	p, err := h.Queries.ListVersions(r.Context(), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, v := range p.Data {
		data[i] = versionBody(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func tenantCategoryBody(v query.TenantCategoryView) map[string]any {
	m := map[string]any{"code": v.Code, "name": v.Name, "severity": v.Severity, "enabled_platform": v.EnabledPlatform,
		"enabled_in_tenant": v.EnabledInTenant, "allow_tenant_toggle": v.AllowTenantToggle, "tenant_term_count": v.TenantTermCount}
	if v.Description != nil {
		m["description"] = *v.Description
	}
	return m
}

// switchState renders ModerationSwitchState of the tenant (pii and grooming have no switch, ADR-0078).
func switchState(s query.SwitchState) map[string]any {
	t := s.TenantID
	return service.StateBody(&t, s.Configured, s.Effective, nil)
}

func (h CatalogHandlers) listTenantCategories(w http.ResponseWriter, r *http.Request) {
	res, err := h.Queries.ListTenantCategories(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(res.Data))
	for i, v := range res.Data {
		data[i] = tenantCategoryBody(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "moderation_switches": switchState(res.Switches)})
}

func (h CatalogHandlers) toggle(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Enabled *bool  `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := decode(r, &b); err != nil || b.Enabled == nil {
		fail(w, r, problem.Invalid("enabled", problem.SubInvalid, "required"))
		return
	}
	res, err := h.Commands.ToggleTenantCategory(r.Context(), chi.URLParam(r, "code"), *b.Enabled, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tenantCategoryBody(query.TenantCategoryView{Code: res.Category.Code, Name: res.Category.Name,
		Description: res.Category.Description, Severity: res.Category.Severity, EnabledPlatform: res.Category.Enabled,
		EnabledInTenant: res.EnabledInTenant, AllowTenantToggle: res.Category.AllowTenantToggle, TenantTermCount: res.TenantTermCount}))
}
