package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// F002-AC-22: missing required variables are reported by name only, never by value.
func TestRequireListsMissingNamesOnly(t *testing.T) {
	t.Setenv("EW_OIDC_AUDIENCE", "eduworld-api")
	t.Setenv("EW_SECRET_THING", "super-secret-value")
	c := config.Load()
	err := c.Require("EW_OIDC_ISSUERS", "EW_OIDC_AUDIENCE", "EW_VALKEY_ADDR", "EW_SECRET_THING")
	if err == nil {
		t.Fatal("missing variables not reported")
	}
	msg := err.Error()
	if !strings.Contains(msg, "EW_OIDC_ISSUERS") || !strings.Contains(msg, "EW_VALKEY_ADDR") {
		t.Fatalf("missing names not listed: %s", msg)
	}
	if strings.Contains(msg, "EW_OIDC_AUDIENCE") || strings.Contains(msg, "eduworld-api") || strings.Contains(msg, "super-secret-value") {
		t.Fatalf("error leaks present names or values: %s", msg)
	}
}

// F002-AC-22: X_FILE is read from the secret file (staging/production Docker secrets).
func TestFileVariant(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "secret")
	if err := os.WriteFile(f, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EW_KC_ADMIN_CLIENT_SECRET_FILE", f)
	c := config.Load()
	if got := c.String("EW_KC_ADMIN_CLIENT_SECRET"); got != "from-file" {
		t.Fatalf("String = %q, want from-file", got)
	}
	if err := c.Require("EW_KC_ADMIN_CLIENT_SECRET"); err != nil {
		t.Fatalf("file variant not accepted as present: %v", err)
	}
}

func TestDirectValueWinsOverFile(t *testing.T) {
	t.Setenv("EW_X", "direct")
	t.Setenv("EW_X_FILE", filepath.Join(t.TempDir(), "absent"))
	if got := config.Load().String("EW_X"); got != "direct" {
		t.Fatalf("got %q", got)
	}
}

func TestUnreadableFileIsAnErrorWithoutValue(t *testing.T) {
	t.Setenv("EW_Y_FILE", filepath.Join(t.TempDir(), "missing"))
	err := config.Load().Require("EW_Y")
	if err == nil || !strings.Contains(err.Error(), "EW_Y_FILE") {
		t.Fatalf("unreadable secret file not reported: %v", err)
	}
}

func TestTypedGetters(t *testing.T) {
	t.Setenv("EW_OIDC_ISSUERS", "http://a/realms/x, http://b/realms/y")
	t.Setenv("EW_OIDC_ALLOWED_CLOCK_SKEW", "45s")
	t.Setenv("EW_FLAG", "true")
	c := config.Load()
	if got := c.List("EW_OIDC_ISSUERS"); len(got) != 2 || got[1] != "http://b/realms/y" {
		t.Fatalf("List = %v", got)
	}
	if d := c.Duration("EW_OIDC_ALLOWED_CLOCK_SKEW", time.Second); d != 45*time.Second {
		t.Fatalf("Duration = %v", d)
	}
	if d := c.Duration("EW_ABSENT", 3*time.Second); d != 3*time.Second {
		t.Fatalf("default Duration = %v", d)
	}
	if !c.Bool("EW_FLAG", false) || c.Bool("EW_ABSENT", false) {
		t.Fatal("Bool")
	}
	if c.StringOr("EW_ABSENT", "dflt") != "dflt" {
		t.Fatal("StringOr")
	}
}
