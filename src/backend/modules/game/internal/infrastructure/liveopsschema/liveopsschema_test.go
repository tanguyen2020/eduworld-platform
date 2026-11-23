package liveopsschema

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
	for file, copy := range map[string][]byte{"event.schema.json": EventJSON, "event-template.schema.json": TemplateJSON,
		"season-track.schema.json": TrackJSON} {
		spec, err := os.ReadFile(filepath.Join(root, "specs", "schemas", file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(norm(spec), norm(copy)) {
			t.Fatalf("%s copy differs from specs/schemas: copy it again", file)
		}
	}
}

// F077-AC-20, AC-27: the example payloads are valid; a paid season track and an unknown field (a lock on a learning
// area) are refused with their pointer.
func TestValidate(t *testing.T) {
	read := func(file string) map[string]any {
		raw, err := os.ReadFile(filepath.Join(root, "specs", "examples", file))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	for file, v := range map[string]interface{ Validate([]byte) error }{"event.mid-autumn-festival.json": Event{},
		"event-template.math-week.json": Template{}, "season-track.season-1.json": Track{}} {
		b, _ := json.Marshal(read(file))
		if err := v.Validate(b); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	track := read("season-track.season-1.json")
	track["paid_track"] = true
	b, _ := json.Marshal(track)
	var se *Error
	if err := (Track{}).Validate(b); !errors.As(err, &se) || se.Pointers[0] != "/paid_track" {
		t.Fatalf("paid track: %v", err)
	}
	ev := read("event.mid-autumn-festival.json")
	ev["locked_area_ids"] = []string{"learning_zone.math"}
	b, _ = json.Marshal(ev)
	if err := (Event{}).Validate(b); !errors.Is(err, ErrInvalid) {
		t.Fatalf("lock field: %v", err)
	}
}
