package challengeschema

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
	for file, copy := range map[string][]byte{"challenge.schema.json": SchemaJSON, filepath.Join("common", "enums.schema.json"): EnumsJSON} {
		spec, err := os.ReadFile(filepath.Join(root, "specs", "schemas", file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(norm(spec), norm(copy)) {
			t.Fatalf("%s copy differs from specs/schemas: copy it again", file)
		}
	}
}

// The example challenge of the specs is valid; a wrong type or a missing time limit is refused with its pointer.
func TestValidate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(root, "specs", "examples", "challenge.math-g3-time-attack.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := (Validator{}).Validate(raw); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["challenge_type"] = "speedrun"
	b, _ := json.Marshal(m)
	if err := (Validator{}).Validate(b); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown type: %v", err)
	}
	m["challenge_type"] = "time"
	delete(m, "time_limit_seconds")
	b, _ = json.Marshal(m)
	if err := (Validator{}).Validate(b); !errors.Is(err, ErrInvalid) {
		t.Fatalf("time without limit: %v", err)
	}
}
