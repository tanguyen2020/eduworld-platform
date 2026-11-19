package contractv1client_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contractv1client"
)

// OQ F017-3: secrets are read from EW_SIS_SECRETS_DIR/<secret_ref>, cached at most 5 minutes, never outside the dir.
func TestFileSecrets(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sis"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sis", "th-demo-a"), []byte("k1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
	s := &contractv1client.FileSecrets{Dir: dir, Now: func() time.Time { return now }}
	ctx := context.Background()
	if v, err := s.Get(ctx, "sis/th-demo-a"); err != nil || string(v) != "k1" {
		t.Fatalf("get = %q %v", v, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sis", "th-demo-a"), []byte("k2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Get(ctx, "sis/th-demo-a"); string(v) != "k1" {
		t.Fatalf("cached value expected, got %q", v)
	}
	now = now.Add(contractv1client.SecretCacheTTL + time.Second)
	if v, _ := s.Get(ctx, "sis/th-demo-a"); string(v) != "k2" {
		t.Fatalf("rotated value expected after the TTL, got %q", v)
	}
	for _, ref := range []string{"", "../etc/passwd", "/etc/passwd", "sis/../../x", "sis/a b", `sis\x`} {
		if _, err := s.Get(ctx, ref); !errors.Is(err, contractv1client.ErrSecretRef) {
			t.Fatalf("ref %q: %v, want ErrSecretRef", ref, err)
		}
	}
	if _, err := s.Get(ctx, "sis/missing"); err == nil {
		t.Fatal("missing secret accepted")
	}
}
