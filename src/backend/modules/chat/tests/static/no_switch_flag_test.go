package static

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// T029 / F116-AC-26: no off-switch of moderation in code, environment or deployment configuration — the switches are
// read only from the effective configuration in eduworld_chat.
func TestNoModerationOffSwitch(t *testing.T) {
	root := moduleDir(t)
	repo := filepath.Join(root, "..", "..", "..", "..")
	flag := regexp.MustCompile(`(?i)(EW_[A-Z_]*(MODERATION|FILTER|PIPELINE|SENSITIVE|PROFANITY|PII)[A-Z_]*|` +
		`(disable|skip|bypass|no)_?(moderation|filter|pipeline|pii|sensitive_terms))`)
	scan := func(dir string, exts ...string) {
		walk(t, dir, func(p string, b []byte) {
			ok := false
			for _, e := range exts {
				ok = ok || strings.HasSuffix(p, e)
			}
			if !ok || strings.HasSuffix(p, "no_switch_flag_test.go") || strings.Contains(p, "node_modules") {
				return
			}
			if m := flag.Find(b); m != nil {
				t.Errorf("%s: moderation flag %q (R15: only the switches of System Admin turn a step off)", p, m)
			}
		})
	}
	scan(filepath.Join(root, "internal"), ".go")
	scan(filepath.Join(root, "module.go"), ".go")
	scan(filepath.Join(repo, "deploy"), ".yml", ".yaml", ".env.example", ".json")
	scan(filepath.Join(repo, "src", "backend", "cmd"), ".go")
	// The env config of module chat exposes no moderation key.
	cfg, err := os.ReadFile(filepath.Join(root, "internal", "infrastructure", "config", "chat.go"))
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?i)moderation|EW_[A-Z_]*SWITCH|filter`).Match(cfg) {
		t.Fatal("chat config reads a moderation setting from the environment")
	}
}
