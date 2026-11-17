package outbox_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.opentelemetry.io/otel/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

func compile(t *testing.T, path string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	s, err := c.Compile("../../../../specs/events/" + path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validate(t *testing.T, s *jsonschema.Schema, v any) error {
	t.Helper()
	raw, _ := json.Marshal(v)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return s.Validate(doc)
}

// F003-AC-17: envelopes built by the writer satisfy specs/events/envelope.schema.json and carry trace_id.
func TestEnvelopeMatchesSchema(t *testing.T) {
	schema := compile(t, "envelope.schema.json")
	w, err := outbox.NewWriter("learning", "activity", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := authztest.Ctx(context.Background(), authztest.Student(authztest.TenantA))
	tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a1c5c6a3e5a2f3e1")
	sid, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid}))
	e, err := w.Envelope(ctx, outbox.Message{Type: "activity.submitted", Subject: outbox.Subject{Type: "attempt", ID: uuid.NewString()},
		Payload: map[string]any{"score": 7}})
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(t, schema, e); err != nil {
		t.Fatalf("envelope invalid: %v", err)
	}
	if e.TraceID == nil || *e.TraceID != tid.String() || *e.TenantID != authztest.TenantA || e.Actor.Type != "student" ||
		e.Producer != "learning/activity" || w.Subject(e.EventType) != "ew.evt.learning.activity.submitted" {
		t.Fatalf("envelope fields: %+v", e)
	}
	plat, err := w.Envelope(context.Background(), outbox.Message{Type: "config.published", Subject: outbox.Subject{Type: "config", ID: "x"},
		Platform: true, Payload: map[string]any{}})
	if err != nil || plat.TenantID != nil || plat.Actor.Type != "system" {
		t.Fatalf("platform event: %+v %v", plat, err)
	}
	if err := validate(t, schema, plat); err != nil {
		t.Fatalf("platform envelope invalid: %v", err)
	}
	for name, m := range map[string]outbox.Message{
		"bad type":      {Type: "Submitted", Subject: outbox.Subject{Type: "a", ID: "b"}, Payload: map[string]any{}},
		"array payload": {Type: "a.b", Subject: outbox.Subject{Type: "a", ID: "b"}, Payload: []int{1}},
		"no tenant":     {Type: "a.b", Subject: outbox.Subject{Type: "a", ID: "b"}, Payload: map[string]any{}},
	} {
		if _, err := w.Envelope(context.Background(), m); !errors.Is(err, outbox.ErrInvalidEvent) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	if _, err := w.Publish(ctx, outbox.Message{Type: "a.b", Subject: outbox.Subject{Type: "a", ID: "b"}, Payload: map[string]any{}}); !errors.Is(err, entx.ErrNoTransaction) {
		t.Fatalf("publish outside a transaction: %v", err)
	}
}

// F003-AC-22: audit.recorded payload matches its schema and never carries PII keys or secrets.
func TestAuditPayload(t *testing.T) {
	schema := compile(t, "payloads/common/audit.recorded.v1.schema.json")
	ctx := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin"))
	p := outbox.Payload(ctx, audit.Event{Action: "role.assigned", TargetType: "user", TargetID: uuid.NewString(), Result: audit.ResultSuccess,
		Reason: "contact hoc.sinh@truong.edu.vn", Metadata: map[string]any{"role": "school_admin", "email": "a@b.vn", "password": "x",
			"Bad-Key": "y", "note": "Bearer eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxIn0.c2ln"}})
	if err := validate(t, schema, p); err != nil {
		t.Fatalf("payload invalid: %v", err)
	}
	raw, _ := json.Marshal(p)
	for _, leak := range []string{"a@b.vn", "hoc.sinh@truong.edu.vn", `"password"`, "Bad-Key", "eyJhbGci"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("payload leaks %q: %s", leak, raw)
		}
	}
	if p.ActorType != "user" || p.ActorID == nil || p.Metadata["role"] != "school_admin" {
		t.Fatalf("payload: %+v", p)
	}
	failure := outbox.Payload(context.Background(), audit.Event{Action: "job.failed", Result: audit.ResultFailure})
	if err := validate(t, schema, failure); err != nil || failure.ActorType != "system" || failure.Result != "error" {
		t.Fatalf("system failure payload: %+v %v", failure, err)
	}
}
