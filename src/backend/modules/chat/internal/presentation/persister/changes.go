package persister

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/natsstream"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

// ChangesName is the consumer name of the message changes in infra.dead_letters (ew.dlq.chat.persister-evt).
const ChangesName = "chat.persister_evt"

// ChangeApplier is the use case (command.ApplyChange).
type ChangeApplier interface {
	Apply(ctx context.Context, c message.Change) (write.ChangeOutcome, error)
}

// ChangesWorker is the durable consumer "persister-evt" of worker-chat (F118): it applies the message changes of
// ew.chat.*.evt.* (edit, recall, reactions, pins) by revision; a change whose message is not persisted yet is retried
// with a delay (research R1); a change failing more than max_deliver times goes to DLQ_CHAT without content (R14).
type ChangesWorker struct {
	o     Options
	apply ChangeApplier
	cons  jetstream.Consumer
}

// NewChanges creates (or updates) the durable consumer of the changes.
func NewChanges(ctx context.Context, o Options, apply ChangeApplier) (*ChangesWorker, error) {
	if o.JS == nil || apply == nil || o.DB == nil {
		return nil, errors.New("persister: JetStream, use case and database are required")
	}
	if o.Batch <= 0 {
		o.Batch = 500
	}
	if o.Wait <= 0 {
		o.Wait = 50 * time.Millisecond
	}
	if o.MaxDeliver <= 0 {
		o.MaxDeliver = 5
	}
	if o.AckWait <= 0 {
		o.AckWait = 30 * time.Second
	}
	if o.Backoff <= 0 {
		o.Backoff = time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	w := &ChangesWorker{o: o, apply: apply}
	cons, err := o.JS.CreateOrUpdateConsumer(ctx, natsstream.StreamName, jetstream.ConsumerConfig{Durable: natsstream.ChangesConsumer,
		FilterSubject: natsstream.ChangesFilter, AckPolicy: jetstream.AckExplicitPolicy, AckWait: o.AckWait,
		MaxDeliver: o.MaxDeliver + 1, MaxAckPending: 20 * o.Batch, DeliverPolicy: jetstream.DeliverAllPolicy})
	if err != nil {
		return nil, fmt.Errorf("persister: changes consumer: %w", err)
	}
	w.cons = cons
	return w, nil
}

// Run applies changes until ctx ends.
func (w *ChangesWorker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		if _, err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			w.o.Logger.WarnContext(ctx, "chat persister: change batch failed", "error", err)
			select {
			case <-ctx.Done():
			case <-time.After(w.o.Backoff):
			}
		}
	}
	return nil
}

// RunOnce fetches one batch and handles it; it returns the number of changes acked.
func (w *ChangesWorker) RunOnce(ctx context.Context) (int, error) {
	batch, err := w.cons.Fetch(w.o.Batch, jetstream.FetchMaxWait(max(w.o.Wait, 10*time.Millisecond)))
	if err != nil {
		return 0, err
	}
	acked := 0
	for m := range batch.Messages() {
		if w.handle(ctx, m) {
			acked++
		}
	}
	if err := batch.Error(); err != nil && !errors.Is(err, nats.ErrTimeout) && acked == 0 {
		return 0, err
	}
	return acked, nil
}

func (w *ChangesWorker) dbReady(ctx context.Context) bool {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if w.o.Health != nil {
		return w.o.Health(cctx) == nil
	}
	return w.o.DB.PingContext(cctx) == nil
}

func (w *ChangesWorker) handle(ctx context.Context, m jetstream.Msg) bool {
	var c message.Change
	if err := json.Unmarshal(m.Data(), &c); err != nil || c.Check() != nil {
		w.deadLetter(ctx, m, nil, "invalid change")
		return false
	}
	for {
		out, err := w.apply.Apply(ctx, c)
		if err == nil && out != write.ChangeMissing {
			observability.Persisted("change", 1)
			return m.Ack() == nil
		}
		if err != nil && !w.dbReady(ctx) {
			// PostgreSQL down: hold the change without spending deliveries, then try again.
			_ = m.InProgress()
			select {
			case <-ctx.Done():
				return false
			case <-time.After(w.o.Backoff):
			}
			continue
		}
		reason := "message not persisted yet"
		if err != nil {
			reason = err.Error()
		}
		if delivered(m) >= uint64(w.o.MaxDeliver) {
			w.deadLetter(ctx, m, &c, reason)
			return false
		}
		_ = m.NakWithDelay(min(time.Duration(delivered(m))*w.o.Backoff, 30*time.Second))
		return false
	}
}

// deadLetter publishes the change to ew.dlq.chat.persister-evt and records infra.dead_letters with ids only (R14).
func (w *ChangesWorker) deadLetter(ctx context.Context, m jetstream.Msg, c *message.Change, reason string) {
	observability.Persisted("dead_letter", 1)
	dl := ids.New()
	dlq := nats.NewMsg(natsx.DLQSubject("chat", natsstream.ChangesConsumer))
	dlq.Data = m.Data()
	dlq.Header.Set(natsx.HeaderOriginalSubject, m.Subject())
	dlq.Header.Set(natsx.HeaderDeadLetterConsumer, ChangesName)
	if _, err := w.o.JS.PublishMsg(ctx, dlq, jetstream.WithMsgID(dl.String())); err != nil {
		w.o.Logger.ErrorContext(ctx, "chat persister: dlq publish failed", "error", err)
		_ = m.NakWithDelay(time.Minute)
		return
	}
	eventID := dl
	payload := []byte(`{"raw":"invalid change"}`)
	if c != nil {
		eventID = c.ChangeID
		payload, _ = json.Marshal(map[string]any{"change_id": c.ChangeID, "op": c.Op, "tenant_id": c.TenantID, "channel_id": c.ChannelID,
			"message_id": c.MessageID, "revision": c.Revision})
	}
	if len(reason) > 1000 {
		reason = reason[:1000]
	}
	// infra.* is exempt from the tenant rules (ADR-0089 §4): ids only, never the content.
	if _, err := w.o.DB.ExecContext(ctx, `INSERT INTO infra.dead_letters (dead_letter_id, stream, consumer, event_id, event_type,
		payload, error, delivery_count) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, dl, natsstream.StreamName, ChangesName, eventID,
		"chat.change", payload, reason, int(delivered(m))); err != nil {
		w.o.Logger.ErrorContext(ctx, "chat persister: dead letter insert failed", "error", err)
	}
	w.o.Logger.WarnContext(ctx, "chat change dead-lettered", "consumer", ChangesName, "reason", reason)
	_ = m.Term()
}
