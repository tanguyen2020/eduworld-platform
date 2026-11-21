//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// syncedPayloads returns the outbox payloads of an identity event type of a tenant, each checked against its payload
// schema (specs/events/payloads/identity/<event>.v1.schema.json) — F011 T040.
func syncedPayloads(ctx context.Context, t *testing.T, e *env, event, tenant string) []map[string]any {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	schema, err := c.Compile(filepath.Join(testkit.RepoRoot(t), "specs", "events", "payloads", "identity", event+".v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := e.relay.QueryContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = $1 AND tenant_id = $2`, event, tenant)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Fatalf("%s payload: %v (%s)", event, err, raw)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
