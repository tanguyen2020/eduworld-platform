//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F021-AC-30 (FR-CUR-008, moved from F040-AC-03 by ADR-0095): the curriculum version a school uses per academic year and
// grade — published versions of the tenant or of assigned platform curricula only (403 RESOURCE_NOT_ASSIGNED, 409
// CONTENT_NOT_PUBLISHED), If-Match (428, 412), school.settings.write only, audit school_curriculum.set; the default
// version of the school follows the setting (T044, OQ F021-5).
func TestSchoolCurriculumSettings(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.lmsSetup(ctx, t)
	authztest.Cover(t, "getSchoolCurriculumSettings", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "putSchoolCurriculumSettings", authztest.CrossTenant, authztest.RoleDenied)
	settings := "/lms/schools/" + k.school.String() + "/curriculum-settings"
	g := must(t, c.do(t, http.MethodGet, settings, adA, nil, nil), http.StatusOK, "settings")
	if g.header.Get("ETag") != `W/"1"` || len(g.items("items")) != 0 {
		t.Fatalf("settings: %s", g.raw)
	}
	year := uuid.New()
	cur2, ver2 := c.curriculum(t, cmNoidung, "QB_MATH_2")
	put := func(version string, h hdr) resp {
		return c.do(t, http.MethodPut, settings, adA, h, map[string]any{"items": []any{map[string]any{"academic_year_id": year, "grade_level": 3,
			"curriculum_version_id": version}}})
	}
	if r := put(ver2, ifm(`W/"1"`)); r.status != 403 || r.str("code") != "RESOURCE_NOT_ASSIGNED" {
		t.Fatalf("not assigned: %d %s", r.status, r.raw)
	}
	c.assign(ctx, t, cur2, c.tenantA)
	if r := put(ver2, ifm(`W/"1"`)); r.status != 409 || r.str("code") != "CONTENT_NOT_PUBLISHED" {
		t.Fatalf("not published: %d %s", r.status, r.raw)
	}
	c.subjectGrade(ctx, t, cur2, ver2, k.s.qc.subject, 3)
	c.publishCurriculum(ctx, t, ver2)
	if r := put(ver2, nil); r.status != 428 {
		t.Fatalf("no If-Match: %d", r.status)
	}
	if r := c.do(t, http.MethodPut, settings, qlcmA, ifm(`W/"1"`), map[string]any{"items": []any{}}); r.status != 403 {
		t.Fatalf("academic manager writes settings: %d", r.status)
	}
	if r := c.do(t, http.MethodGet, settings, adA2, nil, nil); r.status != 404 {
		t.Fatalf("school admin of another school: %d", r.status)
	}
	p := must(t, put(ver2, ifm(`W/"1"`)), http.StatusOK, "put settings")
	if items := p.items("items"); len(items) != 1 || items[0]["curriculum_version_id"] != ver2 || p.header.Get("ETag") != `W/"2"` {
		t.Fatalf("put: %s", p.raw)
	}
	if r := put(ver2, ifm(`W/"1"`)); r.status != 412 {
		t.Fatalf("stale settings: %d", r.status)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'school_curriculum.set'`); n != 1 {
		t.Fatal("audit school_curriculum.set")
	}
	// T044 (OQ F021-5): the default version of the school follows the academic year of its classes when the projection
	// still knows it (until WP-I1 stops class.synced, ADR-0095); otherwise the setting of any year applies.
	defaultVersion := func() *uuid.UUID {
		var out *uuid.UUID
		sctx := authztest.Ctx(ctx, authzPrincipal(c, adA))
		if err := c.read.Read(tenancy.WithScope(sctx, scopeOf(t, c.tenantA)), func(ctx context.Context) error {
			g := 3
			var err error
			out, err = read.SchoolSettings{}.DefaultVersion(ctx, k.school, &g)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if v := defaultVersion(); v == nil || v.String() != ver2 {
		t.Fatalf("default without academic year: %v", v)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE ref.classes SET academic_year_id = $2 WHERE class_id = $1`, k.class, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if v := defaultVersion(); v != nil {
		t.Fatalf("setting of another year applied: %v", v)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE ref.classes SET academic_year_id = $2 WHERE school_id = $1`, k.school, year); err != nil {
		t.Fatal(err)
	}
	if v := defaultVersion(); v == nil || v.String() != ver2 {
		t.Fatalf("default of the year: %v", v)
	}
}
