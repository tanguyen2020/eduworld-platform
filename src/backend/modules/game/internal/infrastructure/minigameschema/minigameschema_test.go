package minigameschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const root = "../../../../../../.."

func norm(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

// The embedded copies equal specs/schemas (single source, R33).
func TestEmbeddedCopiesMatchSpecs(t *testing.T) {
	for file, copy := range map[string][]byte{"minigame-config.schema.json": SchemaJSON, filepath.Join("common", "enums.schema.json"): EnumsJSON} {
		spec, err := os.ReadFile(filepath.Join(root, "specs", "schemas", file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(norm(spec), norm(copy)) {
			t.Fatalf("%s copy differs from specs/schemas: copy it again", file)
		}
	}
}

// F073-AC-01: the example config is valid; an unknown minigame_type, a question embedded in the config (AC-03: no question
// field exists) and a timed_challenge without time limit are refused with their pointers.
func TestValidate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(root, "specs", "examples", "minigame.math-g3-matching.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := (Validator{}).Validate(raw); err != nil {
		t.Fatal(err)
	}
	mutate := func(f func(m map[string]any)) []byte {
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		f(m)
		b, _ := json.Marshal(m)
		return b
	}
	for name, b := range map[string][]byte{
		"type":      mutate(func(m map[string]any) { m["minigame_type"] = "racing" }),
		"questions": mutate(func(m map[string]any) { m["questions"] = []any{map[string]any{"stem": "2+2", "answer": "4"}} }),
		"timed": mutate(func(m map[string]any) {
			m["minigame_type"] = "timed_challenge"
			delete(m, "time_limit_seconds")
		}),
		"count": mutate(func(m map[string]any) { m["question_count"] = 51 }),
		"name": mutate(func(m map[string]any) {
			m["name"] = map[string]any{"vi": string(bytes.Repeat([]byte("a"), 2001)), "en": "x"}
		}),
	} {
		if err := (Validator{}).Validate(b); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
