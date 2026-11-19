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
