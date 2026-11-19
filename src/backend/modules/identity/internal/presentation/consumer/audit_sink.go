package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// AuditSinkName is the durable consumer of the central audit log (F015 T012); one consumer per stream EVT_*.
const AuditSinkName = "identity.audit_sink"

// AuditSinkSubject is the subject of audit.recorded of a module (ew.evt.<module>.audit.recorded).
func AuditSinkSubject(module string) string { return "ew.evt." + module + ".audit.recorded" }

// AuditSinkModules are the producing modules (one stream EVT_<MODULE> each, ADR-0042).
var AuditSinkModules = []string{"identity", "learning", "game", "competition", "chat", "realtime"}

// AuditSink maps audit.recorded envelopes to RecordAuditEntry. Idempotency per event id is handled by
// pkg/outbox.Consumer (processed_events) and by the key of audit.audit_logs.
type AuditSink struct {
	Sink command.AuditSink
}

type auditPayload struct {
	ActorID    *uuid.UUID     `json:"actor_id"`
	ActorType  string         `json:"actor_type"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Result     string         `json:"result"`
	Reason     *string        `json:"reason"`
	SchoolID   *uuid.UUID     `json:"school_id"`
	Metadata   map[string]any `json:"metadata"`
}

// Handle implements outbox.Handler.
func (c AuditSink) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "audit.recorded" {
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	var p auditPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return outbox.Permanent(errors.New("audit.recorded: invalid payload"))
	}
	entry := audit.Entry{ID: e.EventID, OccurredAt: e.OccurredAt, ActorID: p.ActorID, ActorType: p.ActorType, TenantID: e.TenantID,
		SchoolID: p.SchoolID, Action: p.Action, TargetType: p.TargetType, TargetID: p.TargetID, Result: p.Result,
		Metadata: p.Metadata, Producer: e.Producer}
	if p.Reason != nil {
		entry.Reason = *p.Reason
	}
	if e.TraceID != nil {
		entry.TraceID = *e.TraceID
	}
	err := c.Sink.Record(ctx, entry)
	if errors.Is(err, audit.ErrInvalid) || errors.Is(err, command.ErrAuditTenantMismatch) {
		return outbox.Permanent(err) // dead letters (DLQ_IDENTITY); the tenant mismatch is also logged as a security warning
	}
	return err
}
