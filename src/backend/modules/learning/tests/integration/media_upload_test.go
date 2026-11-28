//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media/mediatest"
)

// F027-AC-01, AC-03, AC-05, AC-06 (TC-CNT-008), SC-001: a phone photo goes quarantined → processing → ready; the
// variants are content-addressed, immutable, without EXIF/GPS, private for a tenant and served by signed URLs; the
// same file again is a duplicate; completing twice publishes media.uploaded once.
func TestMediaUploadPipeline(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	photo := mediatest.JPEGWithEXIF(t, 640, 480, 1)
	id, done := c.upload(t, edA, "image", "image/jpeg", photo, nil)
	must(t, done, http.StatusOK, "complete")
	if done.str("status") != "processing" || done.str("bucket") != pkgmedia.BucketPrivate {
		t.Fatalf("complete: %s", done.raw)
	}
	again := must(t, c.do(t, http.MethodPost, "/cms/media/"+id+":complete-upload", edA, idem(), nil), http.StatusOK, "complete again")
	if again.str("status") != "processing" || c.mediaEvents(ctx, t, "media.uploaded", id) != 1 {
		t.Fatalf("second complete: %s, events %d", again.raw, c.mediaEvents(ctx, t, "media.uploaded", id))
	}
	if n := c.audits(ctx, t, command.ActionMediaUploaded, &c.tenantA); n != 1 {
		t.Fatalf("audit media.uploaded = %d", n)
	}
	if err := c.process(ctx, t, id); err != nil {
		t.Fatal(err)
	}
	got := must(t, c.do(t, http.MethodGet, "/cms/media/"+id, edA, nil, nil), http.StatusOK, "get")
	if got.str("status") != "ready" || len(got.items("variants")) != 6 || got.str("width") != "640" || c.mediaEvents(ctx, t, "media.processed", id) != 1 {
		t.Fatalf("processed: %s", got.raw)
	}
	key := variantKey(got, "webp_1024")
	if key != "media/"+sha(photo)+"/webp_1024.webp" {
		t.Fatalf("key %s", key)
	}
	for _, v := range []string{"webp_256", "webp_1024", "webp_2048", "png_256", "png_1024", "png_2048"} {
		b := c.object(ctx, t, pkgmedia.BucketPrivate, variantKey(got, v))
		if len(b) == 0 || bytes.Contains(b, []byte("Exif")) || bytes.Contains(b, []byte("GPS")) {
			t.Errorf("%s keeps EXIF/GPS or is empty (%d bytes)", v, len(b))
		}
	}
	h, err := c.media.s3.Stat(ctx, pkgmedia.BucketPrivate, key)
	if err != nil || h.Get("Cache-Control") != pkgmedia.ImmutableCacheControl || h.Get("Content-Type") != "image/webp" {
		t.Fatalf("headers %v %v", h, err)
	}
	// F027-AC-06: a private URL is signed and refused by the CDN rule once expired.
	u, err := url.Parse(variantURL(got, "webp_1024"))
	if err != nil || u.Query().Get("md5") == "" {
		t.Fatalf("url %s", variantURL(got, "webp_1024"))
	}
	now := time.Now()
	if !c.media.signer.Verify(u.Path, u.Query().Get("md5"), u.Query().Get("expires"), now) ||
		c.media.signer.Verify(u.Path, u.Query().Get("md5"), u.Query().Get("expires"), now.Add(command.MediaSignedURLTTL+time.Second)) {
		t.Fatal("signed URL rule")
	}
	// F027-AC-01: the same file in the same scope is the existing asset, without URL; another tenant gets its own.
	dup := must(t, c.presign(t, edA, "image", "image/jpeg", photo, nil), http.StatusCreated, "duplicate")
	if dup.str("duplicate_of") != id || dup.str("asset_id") != id || dup.str("upload_url") != "" {
		t.Fatalf("duplicate: %s", dup.raw)
	}
	other := must(t, c.presign(t, edB, "image", "image/jpeg", photo, nil), http.StatusCreated, "tenant B")
	if other.str("duplicate_of") != "" || other.str("upload_url") == "" || !strings.Contains(other.str("upload_url"), "ew-media-quarantine") {
		t.Fatalf("tenant B: %s", other.raw)
	}
	// Platform media of System Admin: public bucket, plain CDN URL.
	png := mediatest.JPEGWithEXIF(t, 64, 64, 1)
	pid, pdone := c.upload(t, saAdmin, "image", "image/jpeg", png, nil)
	must(t, pdone, http.StatusOK, "platform complete")
	if err := c.process(ctx, t, pid); err != nil {
		t.Fatal(err)
	}
	pg := must(t, c.do(t, http.MethodGet, "/cms/media/"+pid, saAdmin, nil, nil), http.StatusOK, "platform get")
	if pg.str("bucket") != pkgmedia.BucketPublic || pg.str("owner_scope") != "platform" || strings.Contains(variantURL(pg, "png_256"), "md5=") {
		t.Fatalf("platform: %s", pg.raw)
	}
	_ = c.object(ctx, t, pkgmedia.BucketPublic, variantKey(pg, "png_256"))
}

// F027-AC-03, AC-04, AC-05: mismatching uploads, infected files, wrong magic bytes and undecodable images are rejected
// with media.rejected; a scanner outage keeps the asset processing (never ready unscanned, SC-002).
func TestMediaRejections(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	reason := func(id string) string {
		return must(t, c.do(t, http.MethodGet, "/cms/media/"+id, edA, nil, nil), http.StatusOK, "get").str("reject_reason")
	}
	// Not uploaded yet → 422, still quarantined.
	p := must(t, c.presign(t, edA, "image", "image/png", []byte("never uploaded png"), nil), http.StatusCreated, "presign")
	must(t, c.do(t, http.MethodPost, "/cms/media/"+p.str("asset_id")+":complete-upload", edA, idem(), nil), http.StatusUnprocessableEntity, "no object")
	// sha256 different from the declared one (same size) → rejected type_mismatch at completion.
	declared := bytes.Repeat([]byte{1}, 128)
	p = must(t, c.presign(t, edA, "image", "image/png", declared, nil), http.StatusCreated, "presign sha")
	putUpload(t, p, bytes.Repeat([]byte{2}, 128))
	r := must(t, c.do(t, http.MethodPost, "/cms/media/"+p.str("asset_id")+":complete-upload", edA, idem(), nil), http.StatusOK, "complete sha")
	if r.str("status") != "rejected" || r.str("reject_reason") != "type_mismatch" || c.mediaEvents(ctx, t, "media.rejected", p.str("asset_id")) != 1 ||
		c.mediaEvents(ctx, t, "media.uploaded", p.str("asset_id")) != 0 {
		t.Fatalf("sha mismatch: %s", r.raw)
	}
	cases := []struct {
		name, mime, reason string
		data               []byte
	}{
		{"eicar", "image/png", "virus", []byte(`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`)},
		{"pdf as png", "image/png", "type_mismatch", []byte("%PDF-1.7\n1 0 obj << >> endobj\n")},
		{"corrupt png", "image/png", "transcode_failed", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 200)...)},
	}
	for _, tc := range cases {
		id, done := c.upload(t, edA, "image", tc.mime, tc.data, nil)
		must(t, done, http.StatusOK, tc.name)
		if err := c.process(ctx, t, id); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := reason(id); got != tc.reason || c.mediaEvents(ctx, t, "media.rejected", id) != 1 {
			t.Errorf("%s: reason %q", tc.name, got)
		}
	}
	// Scanner down: the worker step fails (retried by the consumer) and the asset stays processing.
	c.media.clamd.unavailable.Store(true)
	id, done := c.upload(t, edA, "image", "image/jpeg", mediatest.JPEGWithEXIF(t, 32, 32, 1), nil)
	must(t, done, http.StatusOK, "complete")
	if err := c.process(ctx, t, id); !errors.Is(err, command.ErrScannerUnavailable) {
		t.Fatalf("scanner down: %v", err)
	}
	if s := must(t, c.do(t, http.MethodGet, "/cms/media/"+id, edA, nil, nil), http.StatusOK, "get").str("status"); s != "processing" {
		t.Fatalf("status %s", s)
	}
	c.media.clamd.unavailable.Store(false)
	if err := c.process(ctx, t, id); err != nil {
		t.Fatal(err)
	}
}

// F027 follow-up (F065 OQ, T040): a WebVTT subtitle is MediaKind subtitle accepted as text/vtt, checked by its WEBVTT header
// and stored once under media/<sha256>/vtt.vtt (variant "vtt", immutable); an SRT declared as text/vtt is rejected
// type_mismatch; a VTT over the document limit is refused at presign.
func TestMediaWebVTT(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	vtt := []byte("WEBVTT\n\n00:00.000 --> 00:04.000\nXin chào các em!\n")
	id, done := c.upload(t, edA, "subtitle", "text/vtt", vtt, nil)
	must(t, done, http.StatusOK, "complete vtt")
	if err := c.process(ctx, t, id); err != nil {
		t.Fatal(err)
	}
	got := must(t, c.do(t, http.MethodGet, "/cms/media/"+id, edA, nil, nil), http.StatusOK, "get vtt")
	if got.str("status") != "ready" || variantKey(got, "vtt") != "media/"+sha(vtt)+"/vtt.vtt" {
		t.Fatalf("vtt: %s", got.raw)
	}
	if b := c.object(ctx, t, pkgmedia.BucketPrivate, variantKey(got, "vtt")); !bytes.Equal(b, vtt) {
		t.Fatalf("stored vtt %q", b)
	}
	h, err := c.media.s3.Stat(ctx, pkgmedia.BucketPrivate, variantKey(got, "vtt"))
	if err != nil || h.Get("Cache-Control") != pkgmedia.ImmutableCacheControl {
		t.Fatalf("headers %v %v", h, err)
	}
	srt, done := c.upload(t, edA, "subtitle", "text/vtt", []byte("1\n00:00:00,000 --> 00:00:04,000\nXin chào\n"), nil)
	must(t, done, http.StatusOK, "complete srt")
	if err := c.process(ctx, t, srt); err != nil {
		t.Fatal(err)
	}
	if r := must(t, c.do(t, http.MethodGet, "/cms/media/"+srt, edA, nil, nil), http.StatusOK, "get srt"); r.str("reject_reason") != "type_mismatch" {
		t.Fatalf("srt: %s", r.raw)
	}
	if r := c.presign(t, edA, "subtitle", "text/vtt", vtt, map[string]any{"size_bytes": 10*1024*1024 + 1}); r.status != 413 ||
		r.str("code") != "MEDIA_TOO_LARGE" {
		t.Fatalf("too large: %d %s", r.status, r.raw)
	}
	if r := c.presign(t, edA, "subtitle", "text/plain", vtt, nil); r.status != 415 || r.str("code") != "MEDIA_TYPE_NOT_ALLOWED" {
		t.Fatalf("subtitle as text/plain: %d %s", r.status, r.raw)
	}
}

// F027-AC-02: type, size and alt text rules of a presign.
func TestMediaPresignRules(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	data := []byte("x")
	must(t, c.presign(t, edA, "image", "application/x-msdownload", data, nil), http.StatusUnsupportedMediaType, "exe")
	r := must(t, c.presign(t, edA, "image", "image/png", data, map[string]any{"size_bytes": 11 << 20}), http.StatusRequestEntityTooLarge, "11 MB")
	if r.str("code") != "MEDIA_TOO_LARGE" {
		t.Fatalf("code %s", r.raw)
	}
	r = must(t, c.presign(t, edA, "image", "image/png", data, map[string]any{"alt_text": nil}), http.StatusUnprocessableEntity, "no alt")
	if r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("code %s", r.raw)
	}
	must(t, c.presign(t, edA, "audio", "audio/mpeg", data, nil), http.StatusCreated, "audio without alt")
	must(t, c.presign(t, edA, "model3d", "model/gltf-binary", data, nil), http.StatusForbidden, "tenant 3D model")
}

// F027-AC-07 (TC-TEN-001): tenant users cannot create platform media nor see another tenant's assets; game tokens and
// users without media.upload are refused; System Admin writes a tenant asset with a target tenant.
func TestMediaAuthorizationAndTenants(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	photo := mediatest.JPEGWithEXIF(t, 48, 48, 1)
	id, done := c.upload(t, edA, "image", "image/jpeg", photo, nil)
	must(t, done, http.StatusOK, "complete")
	authztest.Cover(t, "presignMediaUpload", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "completeMediaUpload", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getMediaAsset", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listMediaAssets", authztest.CrossTenant, authztest.RoleDenied)
	must(t, c.presign(t, edA, "image", "image/png", []byte("p"), map[string]any{"owner_scope": "platform"}), http.StatusForbidden, "tenant platform media")
	must(t, c.do(t, http.MethodGet, "/cms/media/"+id, edB, nil, nil), http.StatusNotFound, "tenant B get")
	must(t, c.do(t, http.MethodPost, "/cms/media/"+id+":complete-upload", edB, idem(), nil), http.StatusNotFound, "tenant B complete")
	if l := must(t, c.do(t, http.MethodGet, "/cms/media", edB, nil, nil), http.StatusOK, "tenant B list"); len(l.items("data")) != 0 {
		t.Fatalf("tenant B sees %s", l.raw)
	}
	must(t, c.presign(t, hsAn, "image", "image/png", []byte("p"), nil), http.StatusForbidden, "game token")
	must(t, c.do(t, http.MethodGet, "/cms/media/"+id, hsAn, nil, nil), http.StatusForbidden, "game token read")
	must(t, c.presign(t, noRoleA, "image", "image/png", []byte("p"), nil), http.StatusForbidden, "no role")
	// OQ F027-2 (F002 T059): the roles writing content upload media (content_admin, academic_manager); a reviewer does not.
	must(t, c.presign(t, cmDuyet, "image", "image/png", []byte("p"), nil), http.StatusForbidden, "content_reviewer without media.upload")
	if r := must(t, c.presign(t, cmNoidung, "image", "image/png", []byte("cm"), nil), http.StatusCreated, "content_admin"); r.str("status") != "quarantined" {
		t.Fatalf("content_admin presign: %s", r.raw)
	}
	must(t, c.presign(t, qlcmA, "image", "image/png", []byte("am"), nil), http.StatusCreated, "academic_manager")
	must(t, c.do(t, http.MethodGet, "/cms/media/"+id, noRoleA, nil, nil), http.StatusForbidden, "no role read")
	// System Admin: a tenant asset needs the target tenant (tenant_id); the asset belongs to it.
	sa := must(t, c.presign(t, saAdmin, "image", "image/png", []byte("sa"), map[string]any{"tenant_id": c.tenantB.String()}), http.StatusCreated, "SA tenant")
	got := must(t, c.do(t, http.MethodGet, "/cms/media/"+sa.str("asset_id"), saAdmin, hdr{"X-Tenant-Id": c.tenantB.String()}, nil), http.StatusOK, "SA read")
	if got.str("tenant_id") != c.tenantB.String() || got.str("bucket") != pkgmedia.BucketPrivate {
		t.Fatalf("SA asset: %s", got.raw)
	}
	must(t, c.do(t, http.MethodGet, "/cms/media/"+sa.str("asset_id"), edA, nil, nil), http.StatusNotFound, "tenant A reads B")
}
