package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

var (
	unpublished = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_outbox_unpublished_count", Help: "Outbox rows not yet published (last relay batch).",
	}, []string{"module"})
	lag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_outbox_lag_seconds", Help: "Age of the oldest unpublished outbox row.",
	}, []string{"module"})
)

// RelayOptions configure the relay.
type RelayOptions struct {
	Module     string
	DB         *sql.DB // <module>_relay: SELECT, UPDATE (published_at, attempts) on infra.outbox_events
	JS         jetstream.JetStream
	Batch      int           // default 500
	Poll       time.Duration // default 200 ms
	Registerer prometheus.Registerer
	Logger     *slog.Logger
	// AfterPublish is a test hook called after each JetStream ack, before the row is marked.
	AfterPublish func(eventID uuid.UUID) error
}

// Relay publishes committed outbox rows to JetStream (ADR-0042, F003-AC-18): Nats-Msg-Id = event_id so
// a re-publish after a crash is deduplicated; published_at is set only after the ack.
type Relay struct{ o RelayOptions }

// NewRelay validates options.
func NewRelay(o RelayOptions) (*Relay, error) {
	if o.Module == "" || o.DB == nil || o.JS == nil {
		return nil, fmt.Errorf("outbox: relay needs module, relay pool and JetStream")
	}
	if o.Batch <= 0 {
		o.Batch = 500
	}
	if o.Poll <= 0 {
		o.Poll = 200 * time.Millisecond
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Registerer != nil {
		_ = o.Registerer.Register(unpublished)
		_ = o.Registerer.Register(lag)
	}
	return &Relay{o: o}, nil
}

// Run relays until ctx ends (poll; pg_notify('ew_outbox') only shortens the wait when wired).
func (r *Relay) Run(ctx context.Context) error {
	t := time.NewTicker(r.o.Poll)
	defer t.Stop()
	for {
		n, err := r.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			r.o.Logger.WarnContext(ctx, "outbox relay batch failed", "module", r.o.Module, "error", err)
		}
		if n == r.o.Batch {
			continue // more work waiting
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

type row struct {
	id        uuid.UUID
	createdAt time.Time
	subject   string
	tenant    *uuid.UUID
	envelope  []byte
}

// RunOnce publishes one batch in event_id (time) order and returns the number published. A failed
// publish increments attempts and stops the batch so ordering per partition key is kept.
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	tx, err := r.o.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	batch, err := r.selectBatch(ctx, tx)
	if err != nil {
		return 0, err
	}
	unpublished.WithLabelValues(r.o.Module).Set(float64(len(batch)))
	if len(batch) > 0 {
		lag.WithLabelValues(r.o.Module).Set(time.Since(batch[0].createdAt).Seconds())
	} else {
		lag.WithLabelValues(r.o.Module).Set(0)
	}
	published := 0
	var pubErr error
	for _, x := range batch {
		if err := r.publish(ctx, x); err != nil {
			pubErr = err
			if _, uerr := tx.ExecContext(ctx, `UPDATE infra.outbox_events SET attempts = attempts + 1
				WHERE event_id = $1 AND created_at = $2`, x.id, x.createdAt); uerr != nil {
				return published, uerr
			}
			break
		}
		if r.o.AfterPublish != nil {
			if err := r.o.AfterPublish(x.id); err != nil {
				return published, err // simulated crash: nothing marked, rows republished next run
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE infra.outbox_events SET published_at = now()
			WHERE event_id = $1 AND created_at = $2`, x.id, x.createdAt); err != nil {
			return published, err
		}
		published++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return published, pubErr
}

func (r *Relay) selectBatch(ctx context.Context, tx *sql.Tx) ([]row, error) {
	rows, err := tx.QueryContext(ctx, `SELECT event_id, created_at, subject, tenant_id, envelope FROM infra.outbox_events
		WHERE published_at IS NULL ORDER BY event_id LIMIT $1 FOR UPDATE SKIP LOCKED`, r.o.Batch)
	if err != nil {
		return nil, fmt.Errorf("outbox: select: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var batch []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.createdAt, &x.subject, &x.tenant, &x.envelope); err != nil {
			return nil, err
		}
		batch = append(batch, x)
	}
	return batch, rows.Err()
}

func (r *Relay) publish(ctx context.Context, x row) error {
	msg := nats.NewMsg(x.subject)
	msg.Data = x.envelope
	var env struct {
		TraceID *string `json:"trace_id"`
	}
	if json.Unmarshal(x.envelope, &env) == nil && env.TraceID != nil && len(*env.TraceID) == 32 {
		span := ids.New().String()
		msg.Header.Set(natsx.HeaderTraceparent, "00-"+*env.TraceID+"-"+span[len(span)-16:]+"-01")
	}
	if x.tenant != nil {
		msg.Header.Set(natsx.HeaderTenantID, x.tenant.String())
	}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.o.JS.PublishMsg(pctx, msg, jetstream.WithMsgID(x.id.String()))
	return err
}
