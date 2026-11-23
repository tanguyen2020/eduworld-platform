package policyschema

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// root is the repository root (this package is seven levels below it).
const root = "../../../../../../.."

// The embedded copy equals specs/schemas/chat-policy.schema.json (single source, R33).
func TestEmbeddedCopyMatchesSpec(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join(root, "specs", "schemas", "chat-policy.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(spec, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(SchemaJSON, []byte("\r\n"), []byte("\n"))) {
		t.Fatal("chat-policy.schema.json copy differs from specs/schemas/chat-policy.schema.json: copy it again")
	}
}

// The platform default seed is valid; a rule with grade_band is refused at its pointer (F120-AC-08, TC-CHAT-010).
func TestValidate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(root, "src", "backend", "modules", "chat", "seed", "chat_policies.platform.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bad, err := (Validator{}).Validate(raw); err != nil || len(bad) > 0 {
		t.Fatalf("platform seed: %v %v", bad, err)
	}
	doc := []byte(`{"schema_version":5,"scope_type":"school","scope_id":"0193a5f4-6c1e-7b2a-9d1f-000000000002","rules":[` +
		`{"channel_type":"direct","enabled":true,"allowed_kinds":["emoji"],"grade_band":"1-3","validation":{"strictness":"strict"}}]}`)
	if bad, err := (Validator{}).Validate(doc); err != nil || len(bad) == 0 {
		t.Fatalf("grade_band accepted: %v %v", bad, err)
	}
}
