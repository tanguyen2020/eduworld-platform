package secret

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// F035-AC-33: the key comes from the secret file only; missing or empty → ErrMissing.
func TestPseudonymKey(t *testing.T) {
	if _, err := (PseudonymKey{}).Key(context.Background()); !errors.Is(err, ErrMissing) {
		t.Fatalf("no path: %v", err)
	}
	dir := t.TempDir()
	if _, err := (PseudonymKey{Path: filepath.Join(dir, "none")}).Key(context.Background()); !errors.Is(err, ErrMissing) {
		t.Fatalf("no file: %v", err)
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (PseudonymKey{Path: empty}).Key(context.Background()); !errors.Is(err, ErrMissing) {
		t.Fatalf("empty: %v", err)
	}
	f := filepath.Join(dir, "key")
	if err := os.WriteFile(f, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	k, err := (PseudonymKey{Path: f}).Key(context.Background())
	if err != nil || string(k) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("key %q %v", k, err)
	}
}
