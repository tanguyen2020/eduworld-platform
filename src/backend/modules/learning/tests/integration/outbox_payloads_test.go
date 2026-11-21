//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// outboxPayloads returns the payloads of the outbox events of a type about a subject id, checked against the payload
// schema of the type.
func (c *curEnv) outboxPayloads(ctx context.Context, t *testing.T, typ, field, id string) []map[string]any {
	t.Helper()
	path := filepath.Join(testkit.RepoRoot(t), "specs", "events", "payloads", "learning", typ+".v1.schema.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	comp := jsonschema.NewCompiler()
	if err := comp.AddResource(path, doc); err != nil {
		t.Fatal(err)
	}
	sch, err := comp.Compile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := c.relay.QueryContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = $1
		AND envelope->'payload'->>`+"'"+field+"'"+` = $2 ORDER BY created_at`, typ, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		_ = rows.Scan(&raw)
		var v any
		_ = json.Unmarshal(raw, &v)
		if err := sch.Validate(v); err != nil {
			t.Fatalf("%s payload: %v (%s)", typ, err, raw)
		}
		out = append(out, v.(map[string]any))
	}
	return out
}
