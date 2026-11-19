//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/auditmaint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

func auditEnvelope(t *testing.T, producer string, tenant *uuid.UUID, payload map[string]any) outbox.Envelope {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	trace := "4bf92f3577b34da6a1c5c6a3e5a2f3e1"
	return outbox.Envelope{EventID: uuid.Must(uuid.NewV7()), EventType: "audit.recorded", SchemaVersion: 1, OccurredAt: time.Now().UTC(),
		RecordedAt: time.Now().UTC(), Producer: producer, TenantID: tenant, Actor: outbox.Actor{Type: "staff"},
		Subject: outbox.Subject{Type: "audit", ID: "x"}, IdempotencyKey: uuid.NewString(), TraceID: &trace, Payload: raw}
}

// F015 US1: F015-AC-01 (one row per event, from identity and game, redelivery), AC-04 (forbidden keys,
// metric), AC-05 (school of another tenant → permanent/DLQ), AC-03 and AC-06 through the jobs.
func TestAuditSink(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-audit-a")
	b := seedTenant(ctx, t, e.platform, "pgd-audit-b")
	ta := uuid.MustParse(a.tenant)
	m, _ := auditmaint.NewMetrics(nil)
	sink := consumer.AuditSink{Sink: command.AuditSink{UoW: e.uow, Repo: write.AuditRepo{}, Metrics: m}}
	actor := uuid.New()
	school := uuid.MustParse(a.school)
	payload := map[string]any{"actor_id": actor, "actor_type": "user", "action": "user_role.granted", "target_type": "user",
		"target_id": uuid.NewString(), "result": "success", "reason": nil, "school_id": school,
		"metadata": map[string]any{"role_code": "school_admin", "password": "secret-1", "email": "x@y.vn"}}

	// AC-01: identity and game events, each delivered twice → one row each.
	idEv := auditEnvelope(t, "identity/core", &ta, payload)
	gameEv := auditEnvelope(t, "game/session", &ta, map[string]any{"actor_id": nil, "actor_type": "system", "action": "game_session.replaced",
		"target_type": "game_session", "target_id": uuid.NewString(), "result": "success", "metadata": map[string]any{}})
	for _, ev := range []outbox.Envelope{idEv, idEv, gameEv, gameEv} {
		if err := sink.Handle(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM audit.audit_logs WHERE audit_id IN ($1, $2)`, idEv.EventID, gameEv.EventID); n != 2 {
		t.Fatalf("rows = %d", n)
	}
	var meta, producer, trace string
	var actorCol uuid.UUID
	if err := e.platform.QueryRowContext(ctx, `SELECT metadata::text, producer, trace_id, actor_id FROM audit.audit_logs WHERE audit_id = $1`, idEv.EventID).
		Scan(&meta, &producer, &trace, &actorCol); err != nil || meta != `{"role_code": "school_admin"}` || producer != "identity/core" ||
		trace == "" || actorCol != actor {
		t.Fatalf("row = %s %s %s %v %v", meta, producer, trace, actorCol, err)
	}
	// AC-04: the forbidden keys were dropped (twice: one per delivery reaching the sink) and counted.
	if got := testutil.ToFloat64(m.SanitizedCount()); got != 4 {
		t.Fatalf("sanitized fields = %v", got)
	}
	if testutil.ToFloat64(m.RecordedCount("identity", command.AuditRecorded)) != 1 || testutil.ToFloat64(m.RecordedCount("identity", command.AuditDuplicate)) != 1 ||
		testutil.ToFloat64(m.RecordedCount("game", command.AuditRecorded)) != 1 {
		t.Fatal("records metric")
	}

	// AC-05: a school of tenant A in an envelope of tenant B → permanent (dead letters), nothing stored.
	tb := uuid.MustParse(b.tenant)
	bad := auditEnvelope(t, "game/session", &tb, payload)
	if err := sink.Handle(ctx, bad); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("tenant mismatch: %v", err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM audit.audit_logs WHERE audit_id = $1`, bad.EventID); n != 0 {
		t.Fatal("mismatched entry stored")
	}
	// Invalid payload (unknown result) → permanent.
	invalid := auditEnvelope(t, "identity/core", &ta, map[string]any{"actor_type": "user", "action": "x.y", "target_type": "t", "target_id": "1", "result": "maybe"})
	if err := sink.Handle(ctx, invalid); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("invalid: %v", err)
	}

	// AC-03, AC-06: the jobs with the partition admin pool; the clock runs 3 years ahead for the retention.
	maint := command.AuditMaintenance{Maint: auditmaint.Maint{DB: open(t, e.cl.ModuleDSN("identity", "partition_admin"))}, Metrics: m}
	if _, err := maint.EnsurePartitions(ctx); err != nil {
		t.Fatal(err)
	}
	if testutil.ToFloat64(m.MissingCount("audit.audit_logs")) != 0 {
		t.Fatal("partitions reported missing right after the job")
	}
	far := command.AuditMaintenance{Maint: maint.Maint, Metrics: m, Clock: func() time.Time { return time.Now().AddDate(3, 0, 0) }}
	if _, err := far.EnsurePartitions(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := far.Retain(ctx)
	if err != nil || n != 1 { // the game entry has no actor nor identifier to remove
		t.Fatalf("anonymized = %d %v", n, err)
	}
	if c := count(ctx, t, e.platform, `SELECT count(*) FROM audit.audit_logs WHERE audit_id IN ($1, $2) AND actor_id IS NULL`, idEv.EventID, gameEv.EventID); c != 2 { // anonymized, never deleted
		t.Fatal("rows not anonymized (or deleted)")
	}
	if n, _ := far.Retain(ctx); n != 0 {
		t.Fatalf("second retention run = %d", n)
	}
}
