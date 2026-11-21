//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// Placeholder terms of the catalogue tests (R15: placeholders only).
const (
	phAdmin   = "zqadmin"
	phDraft   = "zqdraftonly"
	phPattern = "zq[0-9]+x"
)

// admin sends one portal request with token; POSTs get a new Idempotency-Key; extra headers (If-Match, X-Tenant-Id).
func (c *chatEnv) admin(token, method, path string, body any, headers ...string) resp {
	c.t.Helper()
	h := map[string]string{}
	if method == http.MethodPost {
		h["Idempotency-Key"] = newKey()
	}
	for i := 0; i+1 < len(headers); i += 2 {
		h[headers[i]] = headers[i+1]
	}
	return c.do(method, path, token, h, body)
}

// inTenant registers a platform staff member of role resolved into tenant (the X-Tenant-Id target of the Safety Team
// routes, ADR-0072) and returns its portal token.
func (c *chatEnv) inTenant(role string, tenant uuid.UUID) string {
	d := authztest.Platform(role).PrincipalData
	t := tenant
	d.TenantID, d.TenantType = &t, authz.TenantTypeSchool
	sub := "kc-platform-" + uuid.NewString()
	c.res.Set(sub, d)
	return c.idp.Token(sub, httpxtest.PortalClient, true, time.Now())
}

func termInput(cat uuid.UUID, text, match string) map[string]any {
	return map[string]any{"category_id": cat, "term": text, "match_type": match, "locale": "vi", "enabled": true}
}

func idOf(r resp, key string) string {
	s, _ := r.body[key].(string)
	return s
}

// outboxLeaks fails the test when an outbox envelope (events, audit) carries a placeholder term (R14, F116-AC-19).
func (c *chatEnv) outboxLeaks(terms ...string) {
	c.t.Helper()
	rows, err := c.relay.QueryContext(context.Background(), `SELECT event_type, envelope::text FROM infra.outbox_events`)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		var typ, raw string
		if err := rows.Scan(&typ, &raw); err != nil {
			c.t.Fatal(err)
		}
		n++
		for _, term := range terms {
			if strings.Contains(strings.ToLower(raw), strings.ToLower(term)) {
				c.t.Errorf("%s carries catalogue content", typ)
			}
		}
	}
	if n == 0 {
		c.t.Fatal("no outbox row checked")
	}
}

// T019 / F116-AC-12, F116-AC-13 (TC-CHAT-016, TC-CHAT-011): System Admin creates, updates (If-Match) and soft-deletes
// categories, terms and allowlist entries as drafts; an invalid RE2 pattern is SENSITIVE_TERM_INVALID; every change is
// audited and term changes raise sensitive_term.* without content; testSensitiveTerms answers category/severity/action/
// span/decision without writing any message.
func TestCatalogAdmin(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	sa := c.platformStaff("system_admin")
	versions := func() int { return c.count(`SELECT count(*) FROM chat.sensitive_catalog_versions`) }
	v0 := versions()

	// Category: create, update with If-Match, stale If-Match → 412, code immutable, grooming protected.
	r := c.admin(sa, http.MethodPost, "/admin/chat/sensitive-categories", map[string]any{"code": "zqcat", "name": "Nhóm thử",
		"severity": "medium", "default_action": "block", "enabled": true, "allow_tenant_toggle": false})
	if r.status != http.StatusCreated || r.header.Get("ETag") != `W/"1"` || r.body["term_count"] != float64(0) {
		t.Fatalf("create category: %d %s", r.status, r.raw)
	}
	catID := idOf(r, "category_id")
	upd := map[string]any{"code": "zqcat", "name": "Nhóm thử 2", "severity": "high", "default_action": "hold_for_review", "enabled": true,
		"allow_tenant_toggle": true}
	if r = c.admin(sa, http.MethodPatch, "/admin/chat/sensitive-categories/"+catID, upd, "If-Match", `W/"1"`); r.status != http.StatusOK ||
		r.header.Get("ETag") != `W/"2"` || r.body["severity"] != "high" {
		t.Fatalf("update category: %d %s", r.status, r.raw)
	}
	if r = c.admin(sa, http.MethodPatch, "/admin/chat/sensitive-categories/"+catID, upd, "If-Match", `W/"1"`); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	upd["code"] = "zqother"
	if r = c.admin(sa, http.MethodPatch, "/admin/chat/sensitive-categories/"+catID, upd, "If-Match", `W/"2"`); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("code change: %d %s", r.status, r.raw)
	}
	grooming := "01936a00-0000-7000-8000-000000000107"
	if r = c.admin(sa, http.MethodDelete, "/admin/chat/sensitive-categories/"+grooming, nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("delete grooming category: %d %s", r.status, r.raw)
	}
	if r = c.admin(sa, http.MethodPatch, "/admin/chat/sensitive-categories/"+grooming, map[string]any{"code": "grooming_scam",
		"name": "Dụ dỗ", "severity": "critical", "default_action": "block_and_alert", "enabled": false, "allow_tenant_toggle": false},
		"If-Match", `W/"1"`); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("switch grooming off: %d %s", r.status, r.raw)
	}

	// Terms: draft, event without content; RE2 rules (F116-AC-12).
	r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, phAdmin, "word"))
	if r.status != http.StatusCreated || r.body["status"] != "draft" || r.body["scope"] != "platform" || r.body["tenant_id"] != nil {
		t.Fatalf("create term: %d %s", r.status, r.raw)
	}
	termID := idOf(r, "term_id")
	if r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, phAdmin, "word")); r.status != http.StatusConflict {
		t.Fatalf("duplicate term: %d %s", r.status, r.raw)
	}
	for _, bad := range []string{`zq(?=x)`, `zq(a)\1`, `zq(?!x)`, `zq(`, "zq" + strings.Repeat("a", 199), `zq|`} {
		r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, bad, "pattern"))
		if r.status != http.StatusUnprocessableEntity || r.code() != "SENSITIVE_TERM_INVALID" && r.code() != "VALIDATION_FAILED" {
			t.Fatalf("pattern %q: %d %s", bad, r.status, r.raw)
		}
		if strings.Contains(r.raw, bad) {
			t.Fatalf("problem echoes the pattern: %s", r.raw)
		}
	}
	if r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, `zq(?=x)`, "pattern")); r.code() != "SENSITIVE_TERM_INVALID" {
		t.Fatalf("lookahead: %s", r.raw)
	}
	if r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, phPattern, "pattern")); r.status != http.StatusCreated {
		t.Fatalf("valid pattern: %d %s", r.status, r.raw)
	}
	if r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(uuid.New(), "zqnocat", "word")); r.code() != "SENSITIVE_TERM_INVALID" {
		t.Fatalf("unknown category: %d %s", r.status, r.raw)
	}
	in := termInput(catInsult, phAdmin, "word")
	in["variants"] = []string{"zqadm1n"}
	in["note"] = "ghi chú thử"
	if r = c.admin(sa, http.MethodPatch, "/admin/chat/sensitive-terms/"+termID, in, "If-Match", `W/"1"`); r.status != http.StatusOK ||
		r.header.Get("ETag") != `W/"2"` || r.body["status"] != "draft" {
		t.Fatalf("update term: %d %s", r.status, r.raw)
	}
	if r = c.admin(sa, http.MethodPatch, "/admin/chat/sensitive-terms/"+termID, in); r.status != http.StatusPreconditionRequired {
		t.Fatalf("update without If-Match: %d %s", r.status, r.raw)
	}
	if r = c.admin(sa, http.MethodPatch, "/admin/chat/sensitive-terms/"+termID, in, "If-Match", `W/"1"`); r.code() != "VERSION_MISMATCH" {
		t.Fatalf("stale term update: %d %s", r.status, r.raw)
	}

	// testSensitiveTerms: drafts included by default; published only → no match (nothing published yet); never writes.
	r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, phDraft, "word"))
	if r.status != http.StatusCreated {
		t.Fatalf("draft term: %s", r.raw)
	}
	msgs := c.count(`SELECT count(*) FROM chat.messages`) + c.count(`SELECT count(*) FROM chat.moderation_decisions`)
	r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms:test", map[string]any{"text": "nói " + phDraft + " nhé"})
	if r.status != http.StatusOK || r.body["decision"] != "block" {
		t.Fatalf("test drafts: %d %s", r.status, r.raw)
	}
	matches, _ := r.body["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("matches: %s", r.raw)
	}
	m := matches[0].(map[string]any)
	if m["category_code"] != "insult" || m["severity"] != "medium" || m["action"] != "block" || m["start"] != float64(4) ||
		m["end"] != float64(4+len(phDraft)) {
		t.Fatalf("match: %v", m)
	}
	if r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms:test", map[string]any{"text": "nói " + phDraft + " nhé",
		"include_drafts": false}); r.body["decision"] != "allow" {
		t.Fatalf("published only: %s", r.raw)
	}
	if after := c.count(`SELECT count(*) FROM chat.messages`) + c.count(`SELECT count(*) FROM chat.moderation_decisions`); after != msgs {
		t.Fatalf("testSensitiveTerms wrote %d rows", after-msgs)
	}
	if !slices.Contains(c.audits(), "chat.sensitive_terms.tested") {
		t.Fatal("testSensitiveTerms not audited")
	}

	// Lists: categories with counts, terms with q and cursor paging, allowlist.
	if r = c.admin(sa, http.MethodGet, "/admin/chat/sensitive-categories", nil); r.status != http.StatusOK || len(r.data()) != 11 {
		t.Fatalf("categories: %d %s", r.status, r.raw)
	}
	r = c.admin(sa, http.MethodGet, "/admin/chat/sensitive-terms?limit=1", nil)
	next, _ := r.body["page"].(map[string]any)["next_cursor"].(string)
	if r.status != http.StatusOK || len(r.data()) != 1 || next == "" {
		t.Fatalf("terms page 1: %d %s", r.status, r.raw)
	}
	r2 := c.admin(sa, http.MethodGet, "/admin/chat/sensitive-terms?limit=1&cursor="+next, nil)
	if len(r2.data()) != 1 || r2.data()[0]["term_id"] == r.data()[0]["term_id"] {
		t.Fatalf("terms page 2: %s", r2.raw)
	}
	if r = c.admin(sa, http.MethodGet, "/admin/chat/sensitive-terms?q=DRAFTONLY", nil); len(r.data()) != 1 || r.data()[0]["term"] != phDraft {
		t.Fatalf("terms q: %s", r.raw)
	}

	// Allowlist: create, list, soft-delete.
	r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-allowlist", map[string]any{"phrase": "zqadmin hợp lệ", "locale": "vi",
		"reason": "cụm hợp lệ thử"})
	if r.status != http.StatusCreated {
		t.Fatalf("allowlist: %d %s", r.status, r.raw)
	}
	entry := idOf(r, "entry_id")
	if r = c.admin(sa, http.MethodGet, "/admin/chat/sensitive-allowlist", nil); len(r.data()) != 1 {
		t.Fatalf("allowlist list: %s", r.raw)
	}
	if r = c.admin(sa, http.MethodDelete, "/admin/chat/sensitive-allowlist/"+entry, nil); r.status != http.StatusNoContent {
		t.Fatalf("allowlist delete: %d %s", r.status, r.raw)
	}
	if c.count(`SELECT count(*) FROM chat.sensitive_allowlist WHERE entry_id = $1 AND deleted_at IS NOT NULL`, entry) != 1 {
		t.Fatal("allowlist entry not soft-deleted")
	}

	// Soft-delete of a term (R21): row kept with deleted_at, second delete 404, sensitive_term.deleted.
	if r = c.admin(sa, http.MethodDelete, "/admin/chat/sensitive-terms/"+termID, nil); r.status != http.StatusNoContent {
		t.Fatalf("delete term: %d %s", r.status, r.raw)
	}
	if r = c.admin(sa, http.MethodDelete, "/admin/chat/sensitive-terms/"+termID, nil); r.status != http.StatusNotFound {
		t.Fatalf("delete twice: %d %s", r.status, r.raw)
	}
	if c.count(`SELECT count(*) FROM chat.sensitive_terms WHERE term_id = $1 AND deleted_at IS NOT NULL AND deleted_by IS NOT NULL`, termID) != 1 {
		t.Fatal("term not soft-deleted")
	}
	// A category with terms is not deleted; an empty one is soft-deleted.
	if r = c.admin(sa, http.MethodDelete, "/admin/chat/sensitive-categories/"+catInsult.String(), nil); r.status != http.StatusConflict {
		t.Fatalf("delete used category: %d %s", r.status, r.raw)
	}
	if r = c.admin(sa, http.MethodDelete, "/admin/chat/sensitive-categories/"+catID, nil); r.status != http.StatusNoContent {
		t.Fatalf("delete category: %d %s", r.status, r.raw)
	}

	// Drafts only: no version was created, every term is a draft.
	if versions() != v0 || c.count(`SELECT count(*) FROM chat.sensitive_terms WHERE status <> 'draft'`) != 0 {
		t.Fatal("a write published the catalogue")
	}
	// Events and audit: schema-valid, without any term content (F116-AC-12, F116-AC-19).
	if n := len(c.events("sensitive_term.created")); n != 3 {
		t.Fatalf("sensitive_term.created: %d", n)
	}
	if len(c.events("sensitive_term.updated")) != 1 || len(c.events("sensitive_term.deleted")) != 1 {
		t.Fatal("sensitive_term.updated/deleted missing")
	}
	seen := c.checkEvents("sensitive_term.created", "sensitive_term.updated", "sensitive_term.deleted")
	if seen["sensitive_term.created"] != 3 {
		t.Fatalf("schema-checked events: %v", seen)
	}
	for _, a := range []string{"chat.sensitive_category.created", "chat.sensitive_category.updated", "chat.sensitive_category.deleted",
		"chat.sensitive_term.created", "chat.sensitive_term.updated", "chat.sensitive_term.deleted", "chat.sensitive_allowlist.created",
		"chat.sensitive_allowlist.deleted"} {
		if !slices.Contains(c.audits(), a) {
			t.Errorf("audit %s missing", a)
		}
	}
	c.outboxLeaks(phAdmin, phDraft, "zqadm1n", "zq[0-9]", "ghi chú thử", "hợp lệ")
}
