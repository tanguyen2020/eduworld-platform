package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// F013 T045: the event fixtures of the revocation consumer are valid envelopes (specs/events/envelope.schema.json)
// whose payload satisfies the payload schema of the producer (owner F011/F016 — EVENT_DESIGN §4).
func TestGameAuthEventFixturesMatchSchemas(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..", "..", "specs", "events")
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	envelope, err := c.Compile(filepath.Join(root, "envelope.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join("..", "fixtures", "events", "*.json"))
	if err != nil || len(files) < 5 {
		t.Fatalf("fixtures: %v %v", files, err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if err := envelope.Validate(doc); err != nil {
			t.Errorf("%s: envelope: %v", f, err)
		}
		var e struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		_ = json.Unmarshal(raw, &e)
		payload, err := c.Compile(filepath.Join(root, "payloads", "identity", e.EventType+".v1.schema.json"))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		pdoc, _ := jsonschema.UnmarshalJSON(bytes.NewReader(e.Payload))
		if err := payload.Validate(pdoc); err != nil {
			t.Errorf("%s: payload: %v", f, err)
		}
	}
}
