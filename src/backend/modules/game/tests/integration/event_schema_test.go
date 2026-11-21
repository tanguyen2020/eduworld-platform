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

// gameEventTypes are the events of module game owned by F014 (specs/events/payloads/game).
var sessionEventTypes = []string{"game_session.started", "game_session.ended", "game_session.replaced", "student.game_logged_in",
	"student.game_logged_out"}

// gameEventTypes are the events checked against their schema: F014 plus the profile events of F070.
var gameEventTypes = append(append([]string{}, sessionEventTypes...), "avatar.created", "avatar.updated", "game_profile.updated",
	"content.published", "content.manifest_updated", "world.area_entered", "world.area_unlocked", "world.discovery_found", "world.area_completed",
	"story.chapter_completed")

// checkEventSchemas validates every F014 event in the outbox: envelope (specs/events/envelope.schema.json) and payload
// (specs/events/payloads/game/<type>.v1.schema.json, contract first) — the consumers of identity, learning and the
// audit pipeline rely on them. Returns the number of events checked per type.
func (e *env) checkEventSchemas(ctx context.Context, t *testing.T) map[string]int {
	t.Helper()
	root := filepath.Join(testkit.RepoRoot(t), "specs", "events")
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	envelope, err := c.Compile(filepath.Join(root, "envelope.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, typ := range gameEventTypes {
		payload, err := c.Compile(filepath.Join(root, "payloads", "game", typ+".v1.schema.json"))
		if err != nil {
			t.Fatal(err)
		}
		rows, err := e.relay.QueryContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type = $1`, typ)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				t.Fatal(err)
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if err := envelope.Validate(doc); err != nil {
				t.Errorf("%s envelope: %v", typ, err)
			}
			var env struct {
				Payload json.RawMessage `json:"payload"`
			}
			_ = json.Unmarshal(raw, &env)
			pdoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(env.Payload))
			if err != nil {
				t.Fatal(err)
			}
			if err := payload.Validate(pdoc); err != nil {
				t.Errorf("%s payload: %v", typ, err)
			}
			seen[typ]++
		}
		_ = rows.Close()
	}
	return seen
}
