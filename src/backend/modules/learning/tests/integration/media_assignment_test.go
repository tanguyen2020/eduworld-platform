//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// platformMedia inserts a platform media asset (tenant_id NULL, ew-media-public) of kind/status with variants — TEST
// helper: the row the media pipeline of F027 writes for content_admin, without the object storage.
func (c *curEnv) platformMedia(ctx context.Context, t *testing.T, kind, status, variants string) uuid.UUID {
	t.Helper()
	id := ids.New()
	sha := strings.ReplaceAll(id.String()+id.String(), "-", "")[:64]
	mime := map[string]string{"video": "video/mp4", "document": "text/csv"}[kind]
	// tenancyguard:ok TEST helper under learning_platform: the row of the media pipeline
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO content.media_assets (asset_id, tenant_id, created_at, updated_at, owner_scope, kind,
		original_filename, mime_type, size_bytes, duration_ms, sha256, bucket, variants, license, status, quarantine_key, uploaded_by, reject_reason)
		VALUES ($1, NULL, now(), now(), 'platform', $2, 'bai-giang.mp4', $3, 1024, 100000, $4, 'ew-media-public', $5::jsonb, 'CC BY 4.0', $6::varchar, $7, $1,
		CASE WHEN $6::varchar = 'rejected' THEN 'type_mismatch' END)`,
		id, kind, mime, sha, strings.ReplaceAll(variants, "{sha}", sha), status, "q/"+id.String()); err != nil {
		t.Fatal(err)
	}
	return id
}

// Fix 2026-11-28 (F065 Clarifications, ADR-0029): resource_type media is assignable. A platform video_lesson assigned to
// tenant A (with its platform cue question) is 422 VIDEO_NOT_READY for a student of A while its media is not assigned;
// once System Admin assigns the video and the subtitle (applyTenantAssignments media, audit tenant_data.assigned,
// created = true, replay creates nothing) the student plays the public CDN variants (unsigned: ew-media-public);
// tenant B never sees the asset; a tenant asset, a rejected or unknown asset is not a live platform resource (422
// resource_id); only System Admin assigns (content_admin → 403); revoking the video makes a new start 422 again.
func TestPlatformMediaAssignment(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)

	// The real registry of module.go: full handlers + media assign-only.
	f := c.docFacade(t)
	reg := c.realRegistry(t)
	deps := command.AssignmentDeps{UoW: c.uow, Assignments: write.AssignmentRepo{}, Tenants: read.TenantDirectory{}, Resources: reg,
		Audit: write.Auditor{R: c.rec}, Metrics: observability.AssignmentCounter{}}
	f.Apply, f.Revoke = command.ApplyTenantAssignments{AssignmentDeps: deps}, command.RevokeTenantAssignments{AssignmentDeps: deps}
	sa, _ := systemAdmin(ctx)

	// Platform content: a cue question, the video, its subtitle and the video_lesson, published and assigned to A.
	pq, _ := c.question(t, cmNoidung, s.qc, map[string]any{"owner_scope": "platform"})
	pqv := c.publishQuestion(ctx, t, pq, 1)
	video := c.platformMedia(ctx, t, "video", "ready", theoraVariants)
	subtitle := c.platformMedia(ctx, t, "document", "ready", `{"vtt":{"key":"media/{sha}/sub.vtt","bytes":64,"mime":"text/vtt"}}`)
	body := activityBody(s.qc, "video_lesson", videoConfig(video.String(), subtitle.String(), pqv))
	body["owner_scope"] = "platform"
	act := c.createActivity(t, cmNoidung, body)
	c.publishDoc(ctx, t, "activities", "activity_versions", "activity_id", act, 1)
	c.assignResource(ctx, t, "activity", act, c.tenantA)
	c.assignResource(ctx, t, "question", pq, c.tenantA)

	if r := c.start(t, hsAn, act, nil, nil); r.status != http.StatusUnprocessableEntity || r.str("code") != "VIDEO_NOT_READY" {
		t.Fatalf("media not assigned: %d %s", r.status, r.raw)
	}

	key := newKey()
	for i, m := range []uuid.UUID{video, subtitle} {
		k := newKey()
		if i == 0 {
			k = key
		}
		res, err := apply(sa, f, k, "media", m, c.tenantA)
		if err != nil || len(res.Data) != 1 || res.Data[0].TenantId != c.tenantA || res.Data[0].ResourceType != "media" ||
			res.Data[0].Created == nil || !*res.Data[0].Created {
			t.Fatalf("assign media %s: %+v %v", m, res.Data, err)
		}
	}
	if again, err := apply(sa, f, key, "media", video, c.tenantA); err != nil || len(again.Data) != 1 {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if again, err := apply(sa, f, newKey(), "media", video, c.tenantA); err != nil || again.Data[0].Created == nil || *again.Data[0].Created {
		t.Fatalf("re-assign: %+v %v", again, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_type = 'media'
		AND resource_id IN ($1, $2) AND tenant_id = $3 AND deleted_at IS NULL`, video, subtitle, c.tenantA); n != 2 {
		t.Fatalf("active media assignments = %d", n)
	}
	if n := c.audits(ctx, t, "tenant_data.assigned", &c.tenantA); n != 2 {
		t.Fatalf("audit tenant_data.assigned = %d", n)
	}

	r := must(t, c.start(t, hsAn, act, nil, nil), http.StatusCreated, "start with assigned media")
	urls := map[string]string{}
	for _, m := range r.body["render"].(map[string]any)["media"].([]any) {
		mm := m.(map[string]any)
		urls[mm["variant"].(string)] = mm["url"].(string)
	}
	if len(urls) != 3 || urls["vtt"] == "" {
		t.Fatalf("media: %v", urls)
	}
	for v, u := range urls {
		if !strings.HasPrefix(u, testSigner.BaseURL+"/ew-media-public/media/") || strings.Contains(u, "expires=") {
			t.Fatalf("variant %s of platform media is not a public CDN URL: %s", v, u)
		}
	}

	// Tenant isolation: the assignment of A gives B nothing.
	visible := func(tenant uuid.UUID) int {
		var n int
		if err := c.read.Read(authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin")), func(ctx context.Context) error {
			got, err := write.MediaRepo{}.Assets(ctx, []uuid.UUID{video, subtitle})
			n = len(got)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if a, b := visible(c.tenantA), visible(c.tenantB); a != 2 || b != 0 {
		t.Fatalf("visible media: A %d, B %d", a, b)
	}

	// Not a live platform resource: a tenant asset of A, a rejected platform asset, an unknown id.
	own := uuid.MustParse(c.videoAsset(ctx, t, "video", "ready", theoraVariants))
	rejected := c.platformMedia(ctx, t, "video", "rejected", `{}`)
	for _, id := range []uuid.UUID{own, rejected, uuid.New()} {
		_, err := apply(sa, f, newKey(), "media", id, c.tenantB)
		var pe *problem.Error
		if !asProblem(err, &pe) || pe.Code != problem.ValidationFailed || len(pe.Fields) != 1 || pe.Fields[0].Field != "resource_id" {
			t.Fatalf("asset %s: %v", id, err)
		}
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM infra.tenant_resource_assignments WHERE tenant_id = $1 AND resource_type = 'media'`,
		c.tenantB); n != 0 {
		t.Fatalf("%d media assignments written for B by invalid requests", n)
	}
	// Only System Admin assigns (platform.tenant_data.assign).
	ca := authztest.Ctx(ctx, authztest.Platform("content_admin"))
	if _, err := apply(ca, f, newKey(), "media", video, c.tenantB); code(err) != problem.PermissionDenied {
		t.Fatalf("content_admin assigns media: %v", err)
	}

	// Revoke the video: a new start of another student of A is refused again.
	res, err := asIdentity(sa, "revokeTenantAssignments", newKey(), func(ctx context.Context) (learningapi.RevokeTenantAssignmentsResponseObject, error) {
		return f.RevokeTenantAssignments(ctx, learningapi.RevokeTenantAssignmentsRequestObject{Body: &learningapi.RevokeTenantAssignmentsJSONRequestBody{
			ResourceType: "media", ResourceId: video, TenantIds: []uuid.UUID{c.tenantA}, Reason: "hết giấy phép"}})
	})
	if err != nil || len(res.(learningapi.RevokeTenantAssignments200JSONResponse).Data) != 1 {
		t.Fatalf("revoke: %+v %v", res, err)
	}
	if r := c.start(t, hsBinh, act, nil, nil); r.status != http.StatusUnprocessableEntity || r.str("code") != "VIDEO_NOT_READY" {
		t.Fatalf("after revoke: %d %s", r.status, r.raw)
	}
}
