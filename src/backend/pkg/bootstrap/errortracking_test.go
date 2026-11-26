package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
)

func TestNewErrorTracker(t *testing.T) {
	info := buildinfo.Get("worker", "jobs")
	env := func(m map[string]string) Options { return Options{Getenv: func(k string) string { return m[k] }} }

	if tr, err := newErrorTracker(env(nil), info); tr != nil || err != nil {
		t.Fatalf("unset: %v, %v (want disabled)", tr, err)
	}

	missing := filepath.Join(t.TempDir(), "none")
	if _, err := newErrorTracker(env(map[string]string{"EW_ERROR_TRACKING_DSN_FILE": missing}), info); err == nil ||
		!strings.Contains(err.Error(), "EW_ERROR_TRACKING_DSN_FILE") || strings.Contains(err.Error(), missing) {
		t.Fatalf("missing file: %v", err)
	}

	f := filepath.Join(t.TempDir(), "dsn")
	if err := os.WriteFile(f, []byte("https://public-key@glitchtip.invalid/3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tr, err := newErrorTracker(env(map[string]string{"EW_ERROR_TRACKING_DSN_FILE": f, "EW_ENV": "staging"}), info)
	if err != nil || tr == nil {
		t.Fatalf("enabled: %v, %v", tr, err)
	}
	tr.Flush(0)
}
