// Package outbox implements the transactional outbox (ADR-0042, EVENT_DESIGN §2–3, F003 US4): Writer
// (INSERT into infra.outbox_events in the UnitOfWork transaction), Relay (publish to JetStream after
// commit), Consumer (idempotent, DLQ) and AuditRecorder (audit.recorded through the outbox).
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Actor of the envelope.
type Actor struct {
	Type string  `json:"type"` // student | staff | system | service
	ID   *string `json:"id"`
}

// Subject of the envelope.
type Subject struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Envelope is the standard event envelope (specs/events/envelope.schema.json).
type Envelope struct {
	EventID        uuid.UUID       `json:"event_id"`
	EventType      string          `json:"event_type"`
	SchemaVersion  int             `json:"schema_version"`
	OccurredAt     time.Time       `json:"occurred_at"`
	RecordedAt     time.Time       `json:"recorded_at"`
	Producer       string          `json:"producer"`
	TenantID       *uuid.UUID      `json:"tenant_id"`
	Actor          Actor           `json:"actor"`
	Subject        Subject         `json:"subject"`
	SessionID      *uuid.UUID      `json:"session_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key"`
	TraceID        *string         `json:"trace_id"`
	Payload        json.RawMessage `json:"payload"`
}

// Message is what a command publishes.
type Message struct {
	Type           string // <entity>.<past_verb>
	SchemaVersion  int    // default 1
	Subject        Subject
	PartitionKey   string // ordering key (student_id, party_id…); default Subject.ID
	IdempotencyKey string // deterministic business key; default <type>:<subject id>:<event id>
	TenantID       *uuid.UUID
	Platform       bool // tenant_id NULL (platform event); otherwise the session tenant of the scope
	OccurredAt     time.Time
	Payload        any // JSON object: IDs and needed values only, no PII (R14)
}

var (
	eventTypeRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	componentRe = regexp.MustCompile(`^[a-z0-9_]+$`)
	moduleRe    = regexp.MustCompile(`^(identity|learning|game|competition|chat|realtime)$`)

	// ErrInvalidEvent: the message cannot produce a valid envelope.
	ErrInvalidEvent = errors.New("outbox: invalid event")
)

// Writer writes events into infra.outbox_events of the module database, in the transaction of ctx.
type Writer struct {
	module    string
	component string
	clock     clock.Clock
}

// NewWriter builds a writer for producer <module>/<component>.
func NewWriter(module, component string, clk clock.Clock) (*Writer, error) {
	if !moduleRe.MatchString(module) || !componentRe.MatchString(component) {
		return nil, fmt.Errorf("%w: producer %s/%s", ErrInvalidEvent, module, component)
	}
	if clk == nil {
		clk = clock.System()
	}
	return &Writer{module: module, component: component, clock: clk}, nil
}

// Subject returns the NATS subject ew.evt.<module>.<event_type>.
func (w *Writer) Subject(eventType string) string { return "ew.evt." + w.module + "." + eventType }

// Envelope builds the envelope of m for ctx (tenant from the scope, actor from the principal, trace id).
func (w *Writer) Envelope(ctx context.Context, m Message) (Envelope, error) {
	if !eventTypeRe.MatchString(m.Type) || len(m.Type) > 64 {
		return Envelope{}, fmt.Errorf("%w: event_type %q", ErrInvalidEvent, m.Type)
	}
	if m.Subject.Type == "" || m.Subject.ID == "" || len(m.Subject.Type) > 64 || len(m.Subject.ID) > 64 {
		return Envelope{}, fmt.Errorf("%w: subject", ErrInvalidEvent)
	}
	payload, err := json.Marshal(m.Payload)
	if err != nil || len(payload) == 0 || payload[0] != '{' {
		return Envelope{}, fmt.Errorf("%w: payload must be a JSON object", ErrInvalidEvent)
	}
	now := w.clock.Now().UTC()
	e := Envelope{
		EventID: ids.New(), EventType: m.Type, SchemaVersion: m.SchemaVersion, OccurredAt: m.OccurredAt.UTC(),
		RecordedAt: now, Producer: w.module + "/" + w.component, Subject: m.Subject, IdempotencyKey: m.IdempotencyKey,
		Payload: payload,
	}
	if e.SchemaVersion == 0 {
		e.SchemaVersion = 1
	}
	if m.OccurredAt.IsZero() {
		e.OccurredAt = now
	}
	switch {
	case m.Platform:
	case m.TenantID != nil:
		t := *m.TenantID
		e.TenantID = &t
	default:
		s, ok := tenancy.ScopeFrom(ctx)
		if !ok {
			return Envelope{}, fmt.Errorf("%w: no tenant (set Message.TenantID or Platform)", ErrInvalidEvent)
		}
		id, single := s.SessionTenantID()
		if !single {
			return Envelope{}, fmt.Errorf("%w: scope without a single tenant (set Message.TenantID)", ErrInvalidEvent)
		}
		e.TenantID = &id
	}
	e.Actor = actorOf(ctx)
	if p, ok := authz.PrincipalFrom(ctx); ok && p.SessionID != "" {
		if sid, err := uuid.Parse(p.SessionID); err == nil {
			e.SessionID = &sid
		}
	}
	if e.IdempotencyKey == "" {
		e.IdempotencyKey = m.Type + ":" + m.Subject.ID + ":" + e.EventID.String()
	}
	if len(e.IdempotencyKey) > 128 {
		return Envelope{}, fmt.Errorf("%w: idempotency_key longer than 128", ErrInvalidEvent)
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		t := sc.TraceID().String()
		e.TraceID = &t
	}
	return e, nil
}

func actorOf(ctx context.Context) Actor {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return Actor{Type: "system"}
	}
	id := p.UserID.String()
	if p.UserType == authz.UserTypeStudent {
		return Actor{Type: "student", ID: &id}
	}
	return Actor{Type: "staff", ID: &id}
}

// Publish writes m into the outbox in the transaction of ctx (F003-AC-17). The relay is woken by
// pg_notify('ew_outbox') at commit.
func (w *Writer) Publish(ctx context.Context, m Message) (Envelope, error) {
	tx, ok := entx.Tx(ctx)
	if !ok || !entx.InWriteTx(ctx) {
		return Envelope{}, entx.ErrNoTransaction
	}
	e, err := w.Envelope(ctx, m)
	if err != nil {
		return Envelope{}, err
	}
	return e, insert(ctx, tx, w.Subject(e.EventType), m.PartitionKey, e)
}

// Write implements entx.EventWriter (SoftDelete): subject type is the event's entity, id the key.
func (w *Writer) Write(ctx context.Context, eventType, key string, payload any) error {
	entity := eventType
	for i := range eventType {
		if eventType[i] == '.' {
			entity = eventType[:i]
			break
		}
	}
	_, err := w.Publish(ctx, Message{Type: eventType, Subject: Subject{Type: entity, ID: key}, PartitionKey: key,
		IdempotencyKey: eventType + ":" + key, Payload: payload})
	return err
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insert(ctx context.Context, tx execer, subject, partitionKey string, e Envelope) error {
	if partitionKey == "" {
		partitionKey = e.Subject.ID
	}
	if len(partitionKey) > 128 {
		return fmt.Errorf("%w: partition_key longer than 128", ErrInvalidEvent)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(raw) > 65536 {
		return fmt.Errorf("%w: envelope larger than 64 KiB", ErrInvalidEvent)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO infra.outbox_events
		(event_id, event_type, subject, tenant_id, partition_key, envelope, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, e.EventID, e.EventType, subject, e.TenantID, partitionKey, raw, e.RecordedAt); err != nil {
		return fmt.Errorf("outbox: insert: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_notify('ew_outbox', '')`); err != nil {
		return fmt.Errorf("outbox: notify: %w", err)
	}
	return nil
}
