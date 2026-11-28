//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

const systemRowsMigration = "20270203000001_f010_system_rows_events.sql"

// F010 Clarifications 2026-11-28 (F010-AC-01 for the migration-created rows): after the migrations, the identity
// outbox holds tenant.created of the common tenant, school.updated of its virtual school and province.updated of every
// catalog province — schema-valid envelopes and payloads, actor system, UUIDv7 ids ordered tenant → school → provinces
// (the relay publishes by event_id) — and applying the migration again inserts nothing.
func TestMigrationsAnnounceSystemRows(t *testing.T) {
	ctx := context.Background()
	cl := testkit.Isolated(ctx, t)
	migrator := open(t, cl.ModuleDSN("identity", "migrator"))
	evs := cl.IdentitySystemEvents(ctx, t)

	root := testkit.RepoRoot(t)
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	envelope, err := c.Compile(filepath.Join(root, "specs", "events", "envelope.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schemas := map[string]*jsonschema.Schema{}
	for _, typ := range []string{"tenant.created", "school.updated", "province.updated"} {
		if schemas[typ], err = c.Compile(filepath.Join(root, "specs", "events", "payloads", "identity", typ+".v1.schema.json")); err != nil {
			t.Fatal(err)
		}
	}
	validate := func(s *jsonschema.Schema, raw []byte, what string) {
		t.Helper()
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(doc); err != nil {
			t.Fatalf("%s: %v (%s)", what, err, raw)
		}
	}

	byType := map[string]int{}
	for i, e := range evs {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		validate(envelope, raw, "envelope "+e.EventType)
		s, ok := schemas[e.EventType]
		if !ok {
			t.Fatalf("unexpected system event %s", e.EventType)
		}
		validate(s, e.Payload, "payload "+e.EventType)
		if e.EventID.Version() != 7 || e.Actor.Type != "system" || e.Producer != testkit.SystemEventProducer || e.OccurredAt.IsZero() {
			t.Fatalf("envelope %d = %+v", i, e)
		}
		if i > 0 && e.EventID.String() <= evs[i-1].EventID.String() {
			t.Fatal("event ids not in relay order")
		}
		byType[e.EventType]++
	}
	if evs[0].EventType != "tenant.created" || evs[0].TenantID == nil || evs[0].TenantID.String() != commonTenant ||
		evs[1].EventType != "school.updated" || evs[1].TenantID == nil || evs[1].TenantID.String() != commonTenant {
		t.Fatalf("order: %s, %s", evs[0].EventType, evs[1].EventType)
	}
	var tenant struct {
		TenantType string `json:"tenant_type"`
		Code       string `json:"code"`
	}
	var school struct {
		SchoolID  string `json:"school_id"`
		IsVirtual bool   `json:"is_virtual"`
	}
	_ = json.Unmarshal(evs[0].Payload, &tenant)
	_ = json.Unmarshal(evs[1].Payload, &school)
	if tenant.TenantType != "common" || tenant.Code != "common" || school.SchoolID != "0193a000-0000-7000-8000-000000000002" || !school.IsVirtual {
		t.Fatalf("payloads: %+v %+v", tenant, school)
	}
	provinces := count(ctx, t, migrator, `SELECT count(*) FROM catalog.provinces WHERE deleted_at IS NULL`)
	if byType["tenant.created"] != 1 || byType["school.updated"] != 1 || byType["province.updated"] != provinces || provinces == 0 {
		t.Fatalf("events %v, provinces %d", byType, provinces)
	}
	for _, e := range evs[2:] {
		if e.TenantID != nil {
			t.Fatal("province.updated must be a platform event")
		}
	}

	// A second apply (same statements, new session) inserts nothing: every subject already has its event.
	sqlText, err := os.ReadFile(filepath.Join(root, "src", "backend", "modules", "identity", "migrations", systemRowsMigration))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.ExecContext(ctx, string(sqlText)); err != nil {
		t.Fatal(err)
	}
	if n := len(cl.IdentitySystemEvents(ctx, t)); n != len(evs) {
		t.Fatalf("second apply: %d events, want %d", n, len(evs))
	}
}
