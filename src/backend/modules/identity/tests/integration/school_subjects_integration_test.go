//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F015 T054 (F071 Câu hỏi mở F071-1): updateSchoolSettings writes enabled_subject_codes of the school (null = every
// subject on, the default) and school.updated carries it in settings_subset only while set, valid against the
// payload schema, so ref.schools.settings_subset of module game hides the turned-off subjects (game side:
// TestSubjectDisabledFromSchoolUpdated).
func TestSchoolEnabledSubjects(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-subj-a")
	portalUser(ctx, t, e, idp.Issuer(), "ad.truong.subj", "school_admin", &a.tenant, "school", &a.school)
	tok := idp.Token("sub-ad.truong.subj", httpxtest.PortalClient, time.Now())
	path := "/schools/" + a.school + "/settings"
	hdr := func(etag string) map[string]string {
		return map[string]string{"X-Tenant-Id": a.tenant, "Content-Type": "application/merge-patch+json", "If-Match": etag}
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	schema, err := c.Compile(filepath.Join(testkit.RepoRoot(t), "specs", "events", "payloads", "identity", "school.updated.v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	// lastSubset returns settings_subset of the newest school.updated of the school after checking the payload.
	lastSubset := func() map[string]any {
		t.Helper()
		var raw []byte
		if err := e.relay.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'school.updated'
			AND tenant_id = $1 ORDER BY (envelope->'payload'->>'source_version')::bigint DESC LIMIT 1`, a.tenant).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Fatalf("school.updated payload: %v (%s)", err, raw)
		}
		p := doc.(map[string]any)
		if !strings.Contains(fmt.Sprint(p["changed_fields"]), "settings.enabled_subject_codes") {
			t.Fatalf("changed_fields: %v", p["changed_fields"])
		}
		s, ok := p["settings_subset"].(map[string]any)
		if !ok {
			t.Fatalf("settings_subset missing: %s", raw)
		}
		return s
	}

	// Default: every subject on (present, null).
	r := h.do(t, "GET", path, tok, map[string]string{"X-Tenant-Id": a.tenant}, nil)
	if v, present := r.body["enabled_subject_codes"]; r.status != 200 || !present || v != nil {
		t.Fatalf("default: %d %v", r.status, r.body)
	}
	etag := r.header.Get("ETag")
	// Invalid lists → 422 (empty, blank, duplicate, too long, not an array).
	for _, bad := range []any{[]string{}, []string{""}, []string{"math", "math"}, []string{strings.Repeat("m", 65)}, "math", []int{1}} {
		if r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"enabled_subject_codes": bad}); r.status != 422 {
			t.Fatalf("enabled_subject_codes %v: %d", bad, r.status)
		}
	}
	// Set (no reason needed): stored, audited, carried in school.updated.
	r = h.do(t, "PATCH", path, tok, hdr(etag), map[string]any{"enabled_subject_codes": []string{"math", "vietnamese"}})
	if r.status != 200 || fmt.Sprint(r.body["enabled_subject_codes"]) != "[math vietnamese]" {
		t.Fatalf("set: %d %v", r.status, r.body)
	}
	if s := lastSubset(); fmt.Sprint(s["enabled_subject_codes"]) != "[math vietnamese]" {
		t.Fatalf("subset on: %v", s)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%school.settings_updated%' AND envelope::text LIKE '%settings.enabled_subject_codes%'`); n != 1 {
		t.Fatalf("audit = %d", n)
	}
	// Another key keeps the list in the subset (consumers replace the whole object).
	r = h.do(t, "PATCH", path, tok, hdr(r.header.Get("ETag")), map[string]any{"student_club_creation": true})
	if r.status != 200 || fmt.Sprint(r.body["enabled_subject_codes"]) != "[math vietnamese]" {
		t.Fatalf("other key: %d %v", r.status, r.body)
	}
	// null turns every subject back on: the key leaves the subset (absent = every subject on).
	r = h.do(t, "PATCH", path, tok, hdr(r.header.Get("ETag")), map[string]any{"enabled_subject_codes": nil})
	if v, present := r.body["enabled_subject_codes"]; r.status != 200 || !present || v != nil {
		t.Fatalf("reset: %d %v", r.status, r.body)
	}
	if s := lastSubset(); s["enabled_subject_codes"] != nil {
		t.Fatalf("subset after reset: %v", s)
	} else if _, present := s["enabled_subject_codes"]; present {
		t.Fatalf("enabled_subject_codes must be absent after reset: %v", s)
	}
}
