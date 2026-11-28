//go:build integration

package testkit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// SystemEventProducer is the producer of the events that identity migrations write for the rows they create without a
// use case (migration 20270203000001_f010_system_rows_events, F010 Clarifications 2026-11-28).
const SystemEventProducer = "identity/migration"

// IdentitySystemEvents returns the envelopes of the identity outbox written by its migrations (tenant common, its virtual
// school, the province catalog) in the order the relay publishes them (event_id), so a consumer test can feed them to
// its projection exactly as the relay would.
func (c *Cluster) IdentitySystemEvents(ctx context.Context, t testing.TB) []outbox.Envelope {
	t.Helper()
	db := open(t, c.ModuleDSN("identity", "migrator"))
	rows, err := db.QueryContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE envelope->>'producer' = $1 ORDER BY event_id`,
		SystemEventProducer)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []outbox.Envelope
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var e outbox.Envelope
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatalf("identity system event: %v (%s)", err, raw)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("identity migrations wrote no system event")
	}
	return out
}
