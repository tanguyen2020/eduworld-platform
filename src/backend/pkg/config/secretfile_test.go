package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// F130-AC-11 (TC-AUTH-060): a required secret file that is missing or empty stops the binary with an error that
// names the variable only — never the file content or the path of another secret.
func TestRequireFailsOnMissingOrEmptySecretFile(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "game_db_dsn")
	if err := os.WriteFile(empty, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EW_DB_GAME_URL_FILE", empty)
	t.Setenv("EW_KC_GAME_CLIENT_SECRET_FILE", filepath.Join(dir, "absent"))
	err := config.Load().Require("EW_DB_GAME_URL", "EW_KC_GAME_CLIENT_SECRET")
	if err == nil {
		t.Fatal("missing/empty secret files were accepted")
	}
	msg := err.Error()
	for _, want := range []string{"EW_DB_GAME_URL_FILE: secret file is empty", "EW_KC_GAME_CLIENT_SECRET_FILE: cannot read secret file"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
	if strings.Contains(msg, dir) {
		t.Fatalf("error leaks the secret path: %s", msg)
	}
}

func TestReadSecretFileNeverReturnsValueInError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s")
	if err := os.WriteFile(f, []byte("p@ss-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := config.ReadSecretFile("EW_X_FILE", f)
	if err != nil || v != "p@ss-value" {
		t.Fatalf("ReadSecretFile = %q, %v", v, err)
	}
	if _, err := config.ReadSecretFile("EW_X_FILE", f+".missing"); err == nil || strings.Contains(err.Error(), "p@ss") {
		t.Fatalf("missing file error = %v", err)
	}
}
