// Package persister is chat-persister (worker-chat, ADR-0035, F115 US3): the durable pull consumer "persister" of
// CHAT_MESSAGES fetches batches (chat_persister_batch), groups them by tenant and calls the Persist use case; a message is
// acked only after the commit. While PostgreSQL is down the batch is held (InProgress) and retried without spending
// deliveries, so nothing is lost or dead-lettered by an outage (F115-AC-16); a message failing alone more than
// max_deliver times goes to DLQ_CHAT + infra.dead_letters and never blocks the others (F115-AC-17).
package persister

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/natsstream"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

// Name is the consumer name recorded in infra.dead_letters and the DLQ subject (ew.dlq.chat.persister).
const Name = "chat.persister"

// Persister is the use case (command.Persist).
type Persister interface {
	Persist(ctx context.Context, tenant uuid.UUID, es []message.Envelope) (write.PersistResult, error)
}

// Options configure the worker (CONFIG_DEFAULTS chat_persister_batch; EVENT_DESIGN §4.10 ack_wait, max_deliver).
type Options struct {
	JS         jetstream.JetStream
	Persist    Persister
	DB         *sql.DB                         // <module>_app: health ping and infra.dead_letters
	Health     func(ctx context.Context) error // PostgreSQL reachable (default DB.PingContext)
	Batch      int
	Wait       time.Duration
	MaxDeliver int
	AckWait    time.Duration
	Backoff    time.Duration            // retry pause while PostgreSQL is down (default 1 s, doubling to 30 s)
	Stream     natsstream.StreamOptions // to recreate CHAT_MESSAGES when it disappeared
	Logger     *slog.Logger
	Now        func() time.Time
}

// Worker is chat-persister.
type Worker struct {
	o    Options
	cons jetstream.Consumer
}

// New creates (or updates) the durable consumer.
func New(ctx context.Context, o Options) (*Worker, error) {
	if o.JS == nil || o.Persist == nil || o.DB == nil {
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
	w := &Worker{o: o}
	if err := w.ensure(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

// ensure creates or updates the durable consumer (again after the stream was recreated).
func (w *Worker) ensure(ctx context.Context) error {
	cons, err := w.o.JS.CreateOrUpdateConsumer(ctx, natsstream.StreamName, jetstream.ConsumerConfig{Durable: natsstream.PersisterName,
		FilterSubject: natsstream.PersisterFilter, AckPolicy: jetstream.AckExplicitPolicy, AckWait: w.o.AckWait,
		MaxDeliver: w.o.MaxDeliver + 1, MaxAckPending: 20 * w.o.Batch, DeliverPolicy: jetstream.DeliverAllPolicy})
	if err != nil {
		return fmt.Errorf("persister: consumer: %w", err)
	}
	w.cons = cons
	return nil
}

func (w *Worker) now() time.Time {
	if w.o.Now != nil {
		return w.o.Now()
	}
	return time.Now()
}

// Run fetches and persists batches until ctx ends.
func (w *Worker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		if _, err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			w.o.Logger.WarnContext(ctx, "chat persister batch failed", "error", err)
			select {
			case <-ctx.Done():
			case <-time.After(w.o.Backoff):
			}
			if errors.Is(err, jetstream.ErrConsumerNotFound) || errors.Is(err, jetstream.ErrStreamNotFound) ||
				errors.Is(err, jetstream.ErrConsumerDeleted) || errors.Is(err, nats.ErrNoResponders) {
				if err := natsstream.EnsureStream(ctx, w.o.JS, w.o.Stream); err == nil {
					_ = w.ensure(ctx)
				}
			}
		}
	}
	return nil
}

type item struct {
	msg jetstream.Msg
	env message.Envelope
}

// RunOnce fetches one batch and handles it; it returns the number of messages acked (tests).
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	batch, err := w.cons.Fetch(w.o.Batch, jetstream.FetchMaxWait(max(w.o.Wait, 10*time.Millisecond)))
	if err != nil {
		return 0, err
	}
	var msgs []jetstream.Msg
	for m := range batch.Messages() {
		msgs = append(msgs, m)
	}
	if err := batch.Error(); err != nil && !errors.Is(err, nats.ErrTimeout) && len(msgs) == 0 {
		return 0, err
	}
	if len(msgs) == 0 {
		return 0, nil
	}
	return w.handle(ctx, msgs), nil
}

func (w *Worker) handle(ctx context.Context, msgs []jetstream.Msg) int {
	byTenant := map[uuid.UUID][]item{}
	var order []uuid.UUID
	for _, m := range msgs {
		var e message.Envelope
		if err := json.Unmarshal(m.Data(), &e); err != nil || e.Check() != nil {
			w.deadLetter(ctx, m, nil, "invalid envelope")
			continue
		}
		if _, ok := byTenant[e.TenantID]; !ok {
			order = append(order, e.TenantID)
		}
		byTenant[e.TenantID] = append(byTenant[e.TenantID], item{msg: m, env: e})
	}
	acked := 0
	for _, tenant := range order {
		acked += w.tenantBatch(ctx, tenant, byTenant[tenant])
	}
	return acked
}

// tenantBatch persists the items of one tenant in one transaction; on failure it waits for PostgreSQL (holding the
// messages) or isolates the failing message.
func (w *Worker) tenantBatch(ctx context.Context, tenant uuid.UUID, items []item) int {
	envs := make([]message.Envelope, len(items))
	for i, it := range items {
		envs[i] = it.env
	}
	res, err := w.o.Persist.Persist(ctx, tenant, envs)
	if err == nil {
		return w.settle(ctx, items, res)
	}
	w.o.Logger.WarnContext(ctx, "chat persister: batch not written", "messages", len(items), "error", err)
	if !w.dbReady(ctx) {
		if !w.waitDB(ctx, items) {
			return 0
		}
		res, err = w.o.Persist.Persist(ctx, tenant, envs)
		if err == nil {
			return w.settle(ctx, items, res)
		}
	}
	// PostgreSQL answers: isolate the failing message(s) so the others are not blocked.
	acked := 0
	for i := 0; i < len(items); i++ {
		it := items[i]
		res, err := w.o.Persist.Persist(ctx, tenant, []message.Envelope{it.env})
		if err == nil {
			acked += w.settle(ctx, []item{it}, res)
			continue
		}
		if !w.dbReady(ctx) { // the outage started now: hold the rest, then retry this one
			if !w.waitDB(ctx, items[i:]) {
				return acked
			}
			i--
			continue
		}
		w.retry(ctx, it.msg, err)
	}
	return acked
}

// settle acks inserted and duplicate messages and dead-letters the orphans (unknown channel in the envelope tenant,
// seq already taken).
func (w *Worker) settle(ctx context.Context, items []item, res write.PersistResult) int {
	orphan := map[uuid.UUID]bool{}
	for _, e := range res.Orphans {
		orphan[e.MessageID] = true
	}
	inserted := map[uuid.UUID]bool{}
	for _, e := range res.Inserted {
		inserted[e.MessageID] = true
		observability.PersistLag(w.now().Sub(e.CreatedAt))
	}
	acked := 0
	for _, it := range items {
		switch {
		case orphan[it.env.MessageID]:
			w.deadLetter(ctx, it.msg, &it.env, "channel unknown in the envelope tenant or seq already taken")
		case inserted[it.env.MessageID]:
			observability.Persisted("inserted", 1)
			if it.msg.Ack() == nil {
				acked++
			}
		default:
			observability.Persisted("duplicate", 1)
			if it.msg.Ack() == nil {
				acked++
			}
		}
	}
	return acked
}

func (w *Worker) dbReady(ctx context.Context) bool {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if w.o.Health != nil {
		return w.o.Health(cctx) == nil
	}
	return w.o.DB.PingContext(cctx) == nil
}

// waitDB keeps the messages in progress (no redelivery, no delivery spent) until PostgreSQL answers or ctx ends.
func (w *Worker) waitDB(ctx context.Context, items []item) bool {
	delay := w.o.Backoff
	for {
		for _, it := range items {
			_ = it.msg.InProgress()
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}
		if w.dbReady(ctx) {
			return true
		}
		observability.Persisted("waiting_db", len(items))
		delay = min(2*delay, 30*time.Second, w.o.AckWait/3)
	}
}

func delivered(m jetstream.Msg) uint64 {
	md, err := m.Metadata()
	if err != nil || md.NumDelivered == 0 {
		return 1
	}
	return md.NumDelivered
}

func (w *Worker) retry(ctx context.Context, m jetstream.Msg, err error) {
	var e message.Envelope
	_ = json.Unmarshal(m.Data(), &e)
	if delivered(m) >= uint64(w.o.MaxDeliver) {
		w.deadLetter(ctx, m, &e, err.Error())
		return
	}
	observability.Persisted("retry", 1)
	_ = m.NakWithDelay(min(time.Duration(delivered(m))*w.o.Backoff, 30*time.Second))
}

// deadLetter publishes the envelope to ew.dlq.chat.persister (DLQ_CHAT), records infra.dead_letters WITHOUT the message
// content (R14: ids and decision only) and terminates the message so later ones are not blocked.
func (w *Worker) deadLetter(ctx context.Context, m jetstream.Msg, e *message.Envelope, reason string) {
	observability.Persisted("dead_letter", 1)
	dl := ids.New()
	dlq := nats.NewMsg(natsx.DLQSubject("chat", natsstream.PersisterName))
	dlq.Data = m.Data()
	dlq.Header.Set(natsx.HeaderOriginalSubject, m.Subject())
	dlq.Header.Set(natsx.HeaderDeadLetterConsumer, Name)
	if _, err := w.o.JS.PublishMsg(ctx, dlq, jetstream.WithMsgID(dl.String())); err != nil {
		w.o.Logger.ErrorContext(ctx, "chat persister: dlq publish failed", "error", err)
		_ = m.NakWithDelay(time.Minute)
		return
	}
	eventID := dl
	payload := []byte(`{"raw":"invalid envelope"}`)
	if e != nil && e.MessageID != uuid.Nil {
		eventID = e.MessageID
		payload, _ = json.Marshal(map[string]any{"message_id": e.MessageID, "tenant_id": e.TenantID, "channel_id": e.ChannelID,
			"seq": e.Seq, "sender_id": e.SenderID, "msg_kind": e.MsgKind, "moderation_state": e.Moderation.State})
	}
	if len(reason) > 1000 {
		reason = reason[:1000]
	}
	// infra.* is exempt from the tenant rules (ADR-0089 §4): the dead letter keeps ids only, never the content.
	if _, err := w.o.DB.ExecContext(ctx, `INSERT INTO infra.dead_letters (dead_letter_id, stream, consumer, event_id, event_type,
		payload, error, delivery_count) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, dl, natsstream.StreamName, Name, eventID,
		"chat.message", payload, reason, int(delivered(m))); err != nil {
		w.o.Logger.ErrorContext(ctx, "chat persister: dead letter insert failed", "error", err)
	}
	w.o.Logger.WarnContext(ctx, "chat message dead-lettered", "consumer", Name, "reason", reason)
	_ = m.Term()
}
