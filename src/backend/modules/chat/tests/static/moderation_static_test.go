package static

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// placeholderTerm is the only shape a sensitive term may have in this repository (R15): "__…__" or the nonsense prefix
// "zq" (no Vietnamese or English word starts with it); a phrase may join a placeholder with ordinary words.
var quoted = regexp.MustCompile(`"([^"]*)"`)

var placeholderTerm = regexp.MustCompile(`^(__[a-z0-9_]+__|(?:[\p{L}\p{M}]+ )*zq[\p{L}\p{M}\p{N}\[\]{}()|+*?.\\ -]*)$`)

func walk(t *testing.T, root string, fn func(p string, b []byte)) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fn(p, b)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// T002 / R15, F116-AC-19: no real sensitive word in the seed, the fixtures or the tests of module chat — every term or
// variant given to the catalogue is a placeholder; the migration seeds categories only (no term).
func TestNoRealTerms(t *testing.T) {
	root := moduleDir(t)
	termLit := regexp.MustCompile(`\bterm\(\w+,\s*"([^"]+)"|(?:Term|term|Phrase|phrase)\s*[:=,(]\s*"([^"]+)"|"(?:term|phrase)"\s*:\s*"([^"]+)"|Variants:\s*\[\]string\{([^}]*)\}`)
	checked := 0
	check := func(p string, v string) {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		v = strings.NewReplacer("%04d", "", "%03d", "", "%02d", "").Replace(v) // generated placeholders (fmt)
		if v == "" || strings.HasPrefix(v, "%") {
			return
		}
		checked++
		if !placeholderTerm.MatchString(v) {
			t.Errorf("%s: term %q is not a placeholder (R15)", p, v)
		}
	}
	for _, dir := range []string{filepath.Join(root, "tests"), filepath.Join(root, "seed")} {
		walk(t, dir, func(p string, b []byte) {
			if !strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, ".json") {
				return
			}
			if strings.HasSuffix(p, "moderation_static_test.go") {
				return
			}
			for _, m := range termLit.FindAllStringSubmatch(string(b), -1) {
				switch {
				case m[1] != "":
					check(p, m[1])
				case m[2] != "":
					check(p, m[2])
				case m[3] != "":
					check(p, m[3])
				case m[4] != "":
					for _, v := range quoted.FindAllStringSubmatch(m[4], -1) {
						check(p, v[1])
					}
				}
			}
		})
	}
	if checked < 5 {
		t.Fatalf("only %d terms checked: the scan lost the fixtures", checked)
	}
	mig, err := os.ReadFile(filepath.Join(root, "migrations", "20261230000001_f116_moderation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?i)INSERT INTO chat\.(sensitive_terms|sensitive_allowlist)\b`).Match(mig) {
		t.Fatal("the migration seeds terms (R15: terms come from the CMS)")
	}
}

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
			if !ok || strings.HasSuffix(p, "moderation_static_test.go") || strings.Contains(p, "node_modules") {
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

// T008: the seed of the migration equals modules/chat/seed/moderation/*.json.
func TestSeedModeration(t *testing.T) {
	root := moduleDir(t)
	mig, err := os.ReadFile(filepath.Join(root, "migrations", "20261230000001_f116_moderation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(mig)
	var cats struct {
		Categories []struct {
			ID, Code, Name, Severity, DefaultAction string
			Enabled, AllowTenantToggle              bool
		} `json:"categories"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "seed", "moderation", "categories.platform.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tmp struct {
		Categories []map[string]any `json:"categories"`
	}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		t.Fatal(err)
	}
	_ = cats
	q := func(v any) string { return "'" + strings.ReplaceAll(v.(string), "'", "''") + "'" }
	for _, c := range tmp.Categories {
		row := "(" + q(c["category_id"]) + ", NULL, now(), now(), " + q(c["code"]) + ", " + q(c["name"]) + ", " + q(c["severity"]) + ", " +
			q(c["default_action"]) + ", " + jsonBool(c["enabled"]) + ", " + jsonBool(c["allow_tenant_toggle"]) + ")"
		if !strings.Contains(sql, row) {
			t.Errorf("category %v differs from the migration", c["code"])
		}
	}
	var qm struct {
		Messages []map[string]any `json:"quick_messages"`
	}
	raw, err = os.ReadFile(filepath.Join(root, "seed", "moderation", "quick_messages.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &qm); err != nil {
		t.Fatal(err)
	}
	for _, m := range qm.Messages {
		if m["text_vi"] == "" || m["text_en"] == "" {
			t.Errorf("quick message %v is not bilingual (ADR-0056)", m["quick_message_id"])
		}
		row := "(" + q(m["quick_message_id"]) + ", now(), now(), " + q(m["category"]) + ", " + q(m["text_vi"]) + ", " + q(m["text_en"])
		if !strings.Contains(sql, row) {
			t.Errorf("quick message %v differs from the migration", m["quick_message_id"])
		}
	}
	if len(tmp.Categories) < 9 || len(qm.Messages) < 10 {
		t.Fatalf("seed too small: %d categories, %d quick messages", len(tmp.Categories), len(qm.Messages))
	}
}

func jsonBool(v any) string {
	if b, _ := v.(bool); b {
		return "true"
	}
	return "false"
}

// F116-AC-11: the moderation packages never reference a grade.
func TestModerationNoGrade(t *testing.T) {
	root := moduleDir(t)
	grade := regexp.MustCompile(`(?i)grade_?(level|band)|GradeLevel|GradeBand|lớp_?học|khối`)
	n := 0
	for _, d := range []string{"internal/domain/moderation", "internal/application/pipeline", "internal/infrastructure/moderationcfg"} {
		for p, src := range goFiles(t, filepath.Join(root, d)) {
			n++
			if grade.MatchString(src) {
				t.Errorf("%s references a grade", p)
			}
		}
	}
	if n < 5 {
		t.Fatalf("only %d files scanned", n)
	}
}
