package progressionschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// root is the repository root (this package is seven levels below it).
const root = "../../../../../../.."

func norm(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

// The embedded copies equal the specs (single source, R33).
func TestEmbeddedCopiesMatchSpecs(t *testing.T) {
	for file, copy := range map[string][]byte{"progression-curve.schema.json": CurveJSON, "reward-rule.schema.json": RuleJSON,
		filepath.Join("common", "enums.schema.json"): EnumsJSON} {
		spec, err := os.ReadFile(filepath.Join(root, "specs", "schemas", file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(norm(spec), norm(copy)) {
			t.Fatalf("%s copy differs from specs/schemas: copy it again", file)
		}
	}
}

// The example curve and rule of the specs are valid; a wrong value is refused with its pointer (F074-AC-23).
func TestValidate(t *testing.T) {
	for file, v := range map[string]interface{ Validate([]byte) error }{"progression-curve.default.json": Curve{},
		"reward-rule.activity-completed.json": Rule{}} {
		raw, err := os.ReadFile(filepath.Join(root, "specs", "examples", file))
		if err != nil {
			t.Fatal(err)
		}
		if err := v.Validate(raw); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(root, "specs", "examples", "reward-rule.activity-completed.json"))
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["trigger"] = map[string]any{"event_type": "game_session.started"}
	b, _ := json.Marshal(m)
	var se *Error
	if err := (Rule{}).Validate(b); !errors.As(err, &se) || !errors.Is(err, ErrInvalid) || se.Pointers[0] != "/trigger/event_type" {
		t.Fatalf("login-like trigger: %v", err)
	}
}
