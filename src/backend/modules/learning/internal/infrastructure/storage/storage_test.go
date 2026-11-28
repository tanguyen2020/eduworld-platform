package storage

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// F027-AC-06: public objects get plain CDN URLs; private ones a secure_link URL refused once expired.
func TestCdnSigner(t *testing.T) {
	s := CdnSigner{BaseURL: "http://cdn.test/", Key: "test-only-key", TTL: 10 * time.Minute}
	now := time.Unix(1_800_000_000, 0)
	pub, exp := s.URL("ew-media-public", "media/abc/webp_256.webp", now)
	if pub != "http://cdn.test/ew-media-public/media/abc/webp_256.webp" || exp != nil {
		t.Fatalf("public %s %v", pub, exp)
	}
	priv, exp := s.URL("ew-media-private", "media/abc/webp_256.webp", now)
	if exp == nil || !exp.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("expiry %v", exp)
	}
	u, err := url.Parse(priv)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if !s.Verify(u.Path, q.Get("md5"), q.Get("expires"), now) {
		t.Fatal("fresh URL refused")
	}
	if s.Verify(u.Path, q.Get("md5"), q.Get("expires"), now.Add(10*time.Minute)) {
		t.Fatal("expired URL accepted")
	}
	if s.Verify(strings.Replace(u.Path, "abc", "abd", 1), q.Get("md5"), q.Get("expires"), now) {
		t.Fatal("other object accepted")
	}
	if (CdnSigner{BaseURL: "http://cdn.test", Key: "other", TTL: time.Minute}).Verify(u.Path, q.Get("md5"), q.Get("expires"), now) {
		t.Fatal("other key accepted")
	}
}

func TestPresignPutSignsSizeAndType(t *testing.T) {
	s, err := New(Config{Endpoint: "http://floci:4566", AccessKey: "ak", SecretKey: "sk"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.clock = func() time.Time { return time.Unix(1_800_000_000, 0) }
	raw, hdr, err := s.PresignPut(t.Context(), "ew-media-quarantine", "quarantine/x", 42, "image/png", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	if u.Path != "/ew-media-quarantine/quarantine/x" || u.Query().Get("X-Amz-SignedHeaders") != "content-length;content-type;host" ||
		u.Query().Get("X-Amz-Expires") != "900" || hdr["Content-Type"] != "image/png" {
		t.Fatalf("presign %s %v", raw, hdr)
	}
}

// HLS of private media (ADR-0044 §2, CONTENT_MODEL §7, owner "chốt hết" 2026-11-28): the playlist URL carries the
// secure_link token in its path, scoped to the variant directory, so the relative segment URIs of the playlist
// (seg_000.ts) resolve to signed URLs with the same expiry; another variant, another asset, a tampered token or an
// expired one is refused, and the other private objects keep the query-string signature.
func TestCdnSignerHLSDirectoryToken(t *testing.T) {
	s := CdnSigner{BaseURL: "http://cdn.test", Key: "test-only-key", TTL: 10 * time.Minute}
	now := time.Unix(1_800_000_000, 0)
	sha := strings.Repeat("ab", 32)
	raw, exp := s.URL("ew-media-private", "media/"+sha+"/hls_720/index.m3u8", now)
	if exp == nil || !exp.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("expiry %v", exp)
	}
	playlist, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if playlist.RawQuery != "" || !strings.HasPrefix(playlist.Path, "/ew-media-private/t/") ||
		!strings.HasSuffix(playlist.Path, "/media/"+sha+"/hls_720/index.m3u8") {
		t.Fatalf("playlist URL %s", raw)
	}
	seg := playlist.ResolveReference(&url.URL{Path: "seg_000.ts"})
	if !s.VerifyPath(playlist.Path, now) || !s.VerifyPath(seg.Path, now) {
		t.Fatalf("playlist or segment refused: %s %s", playlist.Path, seg.Path)
	}
	if s.VerifyPath(seg.Path, now.Add(10*time.Minute)) {
		t.Fatal("expired segment accepted")
	}
	for _, bad := range []string{
		strings.Replace(seg.Path, "/hls_720/", "/hls_360/", 1),                          // another variant
		strings.Replace(seg.Path, "/media/"+sha, "/media/"+strings.Repeat("cd", 32), 1), // another asset
		strings.Replace(seg.Path, "/t/", "/t/A", 1),                                     // tampered token
		strings.Replace(seg.Path, "seg_000.ts", "../ogv_720.ogv", 1),                    // outside the directory
	} {
		if s.VerifyPath(bad, now) {
			t.Fatalf("accepted %s", bad)
		}
	}
	if (CdnSigner{BaseURL: "http://cdn.test", Key: "other", TTL: time.Minute}).VerifyPath(seg.Path, now) {
		t.Fatal("other key accepted")
	}
	// the MP4/Theora/WebVTT of the same asset keep the per-object query signature
	if mp4, _ := s.URL("ew-media-private", "media/"+sha+"/mp4_720.mp4", now); !strings.Contains(mp4, "?md5=") || strings.Contains(mp4, "/t/") {
		t.Fatalf("mp4 URL %s", mp4)
	}
	// a public playlist needs no token (public-read bucket)
	if pub, e := s.URL("ew-media-public", "media/"+sha+"/hls_720/index.m3u8", now); e != nil || strings.Contains(pub, "/t/") {
		t.Fatalf("public playlist %s", pub)
	}
}
