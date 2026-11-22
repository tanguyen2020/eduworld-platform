package questschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The embedded copy equals specs/schemas/quest.schema.json (single source, R33).
func TestEmbeddedCopyMatchesSpec(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join(root, "specs", "schemas", "quest.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(spec, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(SchemaJSON, []byte("\r\n"), []byte("\n"))) {
		t.Fatal("quest.schema.json copy differs from specs/schemas/quest.schema.json: copy it again")
	}
}

// The example quest of the spec is valid; a wrong step type is refused with its pointer.
func TestValidate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(root, "specs", "examples", "quest.math-g3-multiplication.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := (Validator{}).Validate(raw); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["quest_type"] = "educational"
	b, _ := json.Marshal(m)
	if err := (Validator{}).Validate(b); !errors.Is(err, ErrInvalid) {
		t.Fatalf("v1 quest type: %v", err)
	}
}

// root is the repository root (this package is seven levels below it).
const root = "../../../../../../.."
