package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

var consumed = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "eduworld_events_consumed_total", Help: "Events handled by consumers by result (ok, duplicate, retry, dlq).",
}, []string{"module", "consumer", "event_type", "result"})

// ConsumedCounter exposes eduworld_events_consumed_total.
func ConsumedCounter() *prometheus.CounterVec { return consumed }

// ErrPermanent marks a handler error that retrying cannot fix (→ DLQ at once), e.g. a payload that
// references another tenant's resource (F003-AC-20).
var ErrPermanent = errors.New("outbox: permanent failure")

// Permanent wraps err as ErrPermanent.
func Permanent(err error) error { return fmt.Errorf("%w: %w", ErrPermanent, err) }

// Handler processes one event inside the consumer's UnitOfWork transaction (scope from the envelope).
type Handler func(ctx context.Context, e Envelope) error

// ConsumerOptions configure a durable consumer <module>.<handler> (EVENT_DESIGN §3.1, §7).
type ConsumerOptions struct {
	Module      string // receiving module
	Name        string // <module>.<handler>
	Stream      string // EVT_<PRODUCER>
	Subjects    []string
	Platform    bool // handles tenant_id = null events (platform scope); tenant events are refused
	MaxSchema   int  // highest supported schema_version (default 1)
	MaxDeliver  int  // default 5
	BackOff     []time.Duration
	AckWait     time.Duration
	MaxInFlight int // messages buffered per pull (default: library default)
	UoW         *entx.UnitOfWork
	DLQDB       *sql.DB // <module>_app (INSERT infra.dead_letters)
	JS          jetstream.JetStream
	Handler     Handler
	Logger      *slog.Logger
	Registerer  prometheus.Registerer
}

// Consumer is an idempotent JetStream consumer: processed_events in the same transaction as the
// handler (F003-AC-19, AC-38), DLQ after MaxDeliver or on permanent errors (F003-AC-21).
type Consumer struct {
	o    ConsumerOptions
	cons jetstream.Consumer
}

// NewConsumer creates or updates the durable consumer.
func NewConsumer(ctx context.Context, o ConsumerOptions) (*Consumer, error) {
	if o.Module == "" || !strings.HasPrefix(o.Name, o.Module+".") || o.Stream == "" || len(o.Subjects) == 0 ||
		o.UoW == nil || o.DLQDB == nil || o.JS == nil || o.Handler == nil {
		return nil, errors.New("outbox: consumer needs module, name <module>.<handler>, stream, subjects, UoW, DLQ pool, JetStream and handler")
	}
	if o.MaxSchema == 0 {
		o.MaxSchema = 1
	}
	if o.MaxDeliver == 0 {
		o.MaxDeliver = 5
	}
	if o.BackOff == nil {
		o.BackOff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}
	}
	if o.AckWait == 0 {
		o.AckWait = 60 * time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Registerer != nil {
		_ = o.Registerer.Register(consumed)
	}
	durable := strings.ReplaceAll(o.Name, ".", "_")
	cfg := jetstream.ConsumerConfig{
		Durable: durable, FilterSubjects: o.Subjects, AckPolicy: jetstream.AckExplicitPolicy,
		AckWait: o.AckWait, MaxDeliver: -1, // DLQ decision is ours (delivery count), not the server's
	}
	// A durable reset to a point in time by scripts/ops/reset-consumers.sh after a restore (BACKUP_RECOVERY §3.5,
	// F130-AC-17) keeps its start position: the deliver policy of an existing consumer cannot be updated, so the
	// binary reuses it instead of failing at start-up or replaying from the beginning.
	cfg.DeliverPolicy, cfg.OptStartTime, cfg.OptStartSeq = existingStart(ctx, o.JS, o.Stream, durable)
	cons, err := o.JS.CreateOrUpdateConsumer(ctx, o.Stream, cfg)
	if err != nil {
		return nil, fmt.Errorf("outbox: consumer %s: %w", o.Name, err)
	}
	return &Consumer{o: o, cons: cons}, nil
}

// Run consumes until ctx ends, then drains: in-flight handlers finish and ack (F003-AC-30).
func (c *Consumer) Run(ctx context.Context) error {
	// Messages already delivered when shutdown starts are still handled to completion (drain), so their
	// handlers must not see the cancellation of ctx.
	hctx := context.WithoutCancel(ctx)
	var opts []jetstream.PullConsumeOpt
	if c.o.MaxInFlight > 0 {
		opts = append(opts, jetstream.PullMaxMessages(c.o.MaxInFlight))
	}
	cc, err := c.cons.Consume(func(m jetstream.Msg) { c.Handle(hctx, m) }, opts...)
	if err != nil {
		return err
	}
	<-ctx.Done()
	cc.Drain()
	<-cc.Closed()
	return nil
}

// Handle processes one message (exported for tests and the shutdown drain).
func (c *Consumer) Handle(ctx context.Context, m jetstream.Msg) {
	var e Envelope
	if err := json.Unmarshal(m.Data(), &e); err != nil || e.EventID == uuid.Nil || e.EventType == "" || e.OccurredAt.IsZero() {
		c.deadLetter(ctx, m, e, "invalid envelope")
		return
	}
	if e.SchemaVersion > c.o.MaxSchema {
		c.deadLetter(ctx, m, e, fmt.Sprintf("unsupported schema_version %d", e.SchemaVersion))
		return
	}
	scope, err := eventScope(c.o, e.TenantID)
	if err != nil {
		c.deadLetter(ctx, m, e, err.Error())
		return
	}
	ctx = tenancy.WithModule(ctx, c.o.Module)
	ctx, done := observability.UseCase(ctx, c.o.Module+".consumer."+c.o.Name[len(c.o.Module)+1:])
	duplicate := false
	err = c.inTx(ctx, scope, func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		res, err := tx.ExecContext(ctx, `INSERT INTO infra.processed_events (consumer, event_id, event_occurred_at)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, c.o.Name, e.EventID, e.OccurredAt.UTC())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			duplicate = true
			return nil
		}
		return c.o.Handler(ctx, e)
	})
	done(err)
	switch {
	case err == nil && duplicate:
		consumed.WithLabelValues(c.o.Module, c.o.Name, e.EventType, "duplicate").Inc()
		_ = m.Ack()
	case err == nil:
		consumed.WithLabelValues(c.o.Module, c.o.Name, e.EventType, "ok").Inc()
		_ = m.Ack()
	case errors.Is(err, ErrPermanent):
		c.deadLetter(ctx, m, e, err.Error())
	default:
		n := delivered(m)
		if n >= uint64(c.o.MaxDeliver) {
			c.deadLetter(ctx, m, e, err.Error())
			return
		}
		consumed.WithLabelValues(c.o.Module, c.o.Name, e.EventType, "retry").Inc()
		delay := c.o.BackOff[min(int(n)-1, len(c.o.BackOff)-1)]
		_ = m.NakWithDelay(delay)
	}
}

func delivered(m jetstream.Msg) uint64 {
	md, err := m.Metadata()
	if err != nil || md.NumDelivered == 0 {
		return 1
	}
	return md.NumDelivered
}

// inTx runs fn in the consumer transaction. A panic of the handler (re-raised by the UnitOfWork after its
// rollback) is recovered, counted and reported (eduworld_panics_recovered_total, GlitchTip — F131-AC-17) and
// becomes an ordinary handler error: the message is retried with back-off and dead-lettered after MaxDeliver
// instead of crashing the worker and every consumer in it.
func (c *Consumer) inTx(ctx context.Context, scope tenancy.Scope, fn func(ctx context.Context) error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			observability.RecoveredPanic(ctx, c.o.Module, observability.EntrypointConsumer, p)
			err = fmt.Errorf("outbox: consumer %s panicked: %v", c.o.Name, p)
		}
	}()
	return c.o.UoW.InTenantTx(ctx, scope, fn)
}

// deadLetter publishes ew.dlq.<module>.<consumer>, records infra.dead_letters and terminates the
// original message so later messages are not blocked (F003-AC-21).
func (c *Consumer) deadLetter(ctx context.Context, m jetstream.Msg, e Envelope, reason string) {
	consumed.WithLabelValues(c.o.Module, c.o.Name, orUnknown(e.EventType), "dlq").Inc()
	reason = bound(observability.RedactString(reason), 1000)
	short := strings.TrimPrefix(c.o.Name, c.o.Module+".")
	msg := nats.NewMsg(natsx.DLQSubject(c.o.Module, short))
	msg.Data = m.Data()
	msg.Header.Set(natsx.HeaderOriginalSubject, m.Subject())
	msg.Header.Set(natsx.HeaderDeadLetterConsumer, c.o.Name)
	dl := ids.New()
	if _, err := c.o.JS.PublishMsg(ctx, msg, jetstream.WithMsgID(dl.String())); err != nil {
		c.o.Logger.ErrorContext(ctx, "dlq publish failed", "consumer", c.o.Name, "error", err)
		_ = m.NakWithDelay(time.Minute)
		return
	}
	payload := m.Data()
	if !json.Valid(payload) {
		payload, _ = json.Marshal(map[string]string{"raw": "invalid json"})
	}
	if len(payload) > 65536 {
		payload, _ = json.Marshal(map[string]string{"raw": "envelope larger than 64 KiB"})
	}
	eventID := e.EventID
	if eventID == uuid.Nil {
		eventID = dl
	}
	if _, err := c.o.DLQDB.ExecContext(ctx, `INSERT INTO infra.dead_letters
		(dead_letter_id, stream, consumer, event_id, event_type, payload, error, delivery_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, dl, c.o.Stream, c.o.Name, eventID, bound(orUnknown(e.EventType), 64),
		payload, reason, int(delivered(m))); err != nil {
		c.o.Logger.ErrorContext(ctx, "dead letter insert failed", "consumer", c.o.Name, "error", err)
	}
	c.o.Logger.WarnContext(ctx, "event dead-lettered", "consumer", c.o.Name, "event_type", e.EventType, "reason", reason)
	_ = m.Term()
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// existingStart returns the start position of an existing durable (deliver all for a new one).
func existingStart(ctx context.Context, js jetstream.JetStream, stream, durable string) (jetstream.DeliverPolicy, *time.Time, uint64) {
	c, err := js.Consumer(ctx, stream, durable)
	if err != nil {
		return jetstream.DeliverAllPolicy, nil, 0
	}
	cur := c.CachedInfo().Config
	return cur.DeliverPolicy, cur.OptStartTime, cur.OptStartSeq
}
