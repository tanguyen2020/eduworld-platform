package static

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// T019 / F117-AC-10 (TC-CHAT-018): no flag, feature flag, tenant/school/grade setting or environment value can turn the
// gameplay chat (match/team/party) off: no such key exists in the code of module chat or the deployment configuration,
// the resolver always applies policy.ApplyAlwaysOn to these types, and the send path decides always-on from the channel
// type only.
func TestAlwaysOnHasNoSwitch(t *testing.T) {
	root := moduleDir(t)
	flag := regexp.MustCompile(`(?i)(EW_[A-Z_]*(GAMEPLAY|MATCH|PARTY|TEAM)[A-Z_]*(CHAT)?[A-Z_]*(ENABLED|DISABLED|OFF|ON)\b)|` +
		`((gameplay|match|party|team)_chat_(enabled|disabled|off))|(always_?on_?(enabled|disabled|off))|(disable_?(gameplay|match|party|team)_?chat)`)
	n := 0
	for p, src := range goFiles(t, root) {
		n++
		if m := flag.FindString(src); m != "" {
			t.Errorf("%s declares a switch of the gameplay chat: %q", p, m)
		}
	}
	repo := filepath.Join(root, "..", "..", "..", "..")
	for _, f := range []string{"deploy/compose/docker-compose.app.yml", "deploy/compose/docker-compose.yml"} {
		b, err := os.ReadFile(filepath.Join(repo, f))
		if err != nil {
			continue
		}
		if m := flag.FindString(string(b)); m != "" {
			t.Errorf("%s declares a switch of the gameplay chat: %q", f, m)
		}
	}
	if n < 50 {
		t.Fatalf("scanned only %d files", n)
	}
	pol, err := os.ReadFile(filepath.Join(root, "internal", "domain", "policy", "policy.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`if alwaysOn \{\s*e = ApplyAlwaysOn\(e\)`).Match(pol) {
		t.Fatal("policy.Intersect no longer applies ApplyAlwaysOn to always-on channels")
	}
	ch, err := os.ReadFile(filepath.Join(root, "internal", "domain", "channel", "channel.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ch), "func (t Type) AlwaysOn() bool { return t == Match || t == Party || t == Team }") {
		t.Fatal("channel.Type.AlwaysOn must be decided by the channel type only")
	}
	facts, err := os.ReadFile(filepath.Join(root, "internal", "application", "service", "facts.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(facts), "policy.Intersect(string(ch.Type), ch.Type.AlwaysOn(), levels)") {
		t.Fatal("the resolver must pass the always-on flag of the channel type")
	}
}

// T031 / F117-AC-20, AC-23 (R21, ADR-0033): a physical delete (DELETE FROM, TRUNCATE, DROP TABLE, DETACH PARTITION)
// appears only in internal/infrastructure/purge; in the migrations only inside the SECURITY DEFINER partition functions
// of the purge, and never a DELETE.
func TestHardDeleteOnlyInPurge(t *testing.T) {
	root := moduleDir(t)
	hard := regexp.MustCompile(`(?i)\b(DELETE\s+FROM|TRUNCATE\s|DROP\s+TABLE|DETACH\s+PARTITION)`)
	purge := filepath.Join(root, "internal", "infrastructure", "purge") + string(os.PathSeparator)
	inPurge := 0
	for p, src := range goFiles(t, root) {
		if !hard.MatchString(src) {
			continue
		}
		if strings.HasPrefix(p, purge) {
			inPurge++
			continue
		}
		t.Errorf("%s holds a physical delete outside internal/infrastructure/purge (R21)", p)
	}
	if inPurge == 0 {
		t.Fatal("the purge package holds no delete: the scan is broken")
	}
	entries, err := os.ReadDir(filepath.Join(root, "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	fn := regexp.MustCompile(`(?s)CREATE FUNCTION chat\.(drop_gameplay_partition|drop_ephemeral_partition)\(.*?END \$\$;`)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, "migrations", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		src := regexp.MustCompile(`--[^\n]*`).ReplaceAllString(string(b), "")
		if regexp.MustCompile(`(?i)\bDELETE\s+FROM\b`).MatchString(src) {
			t.Errorf("%s: DELETE in a migration (R12, R21)", e.Name())
		}
		rest := fn.ReplaceAllString(src, "")
		for _, m := range regexp.MustCompile(`(?i)\b(DROP\s+TABLE|DETACH\s+PARTITION)\b`).FindAllString(rest, -1) {
			if e.Name() >= "20261231000004" {
				t.Errorf("%s: %s outside the purge partition functions", e.Name(), m)
			}
		}
	}
}
