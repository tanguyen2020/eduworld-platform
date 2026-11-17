// Package partitionmaint is the job infra-partition-maintenance (F003 T070, ADR-0089 §5–6): it keeps
// daily partitions of the four infra tables at least 7 days ahead and drops expired ones through the
// SECURITY DEFINER functions of the infra migration, so the role <module>_partition_admin has nothing
// but EXECUTE. Business tables are never touched (ADR-0070).
package partitionmaint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
)

// Tables are the partitioned infra tables.
var Tables = []string{"outbox_events", "processed_events", "dead_letters", "idempotency_records"}

// MinDaysAhead below which the job warns (P2 alert).
const MinDaysAhead = 7

var (
	daysAhead = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_infra_partition_days_ahead", Help: "Consecutive daily infra partitions from today (alert P2 when < 7).",
	}, []string{"module", "table"})
	dropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_infra_partition_dropped_total", Help: "Expired infra partitions dropped.",
	}, []string{"module", "table"})
	refused = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_infra_partition_refused_total", Help: "Expired infra partitions kept because of unpublished outbox rows or unresolved dead letters.",
	}, []string{"module", "table"})
)

// Options of the job (retentions from CONFIG_DEFAULTS / EVENT_DESIGN §8).
type Options struct {
	Module              string
	DB                  *sql.DB // <module>_partition_admin
	Clock               clock.Clock
	DaysAhead           int           // partitions created from today (default 9 → ≥ 8 ahead)
	OutboxRetention     time.Duration // outbox_retention: outbox_events, processed_events (default 30 days)
	IdempotencyTTL      time.Duration // idempotency_record_ttl (default 24 h)
	DeadLetterRetention time.Duration // EVENT_DESIGN §8 (default 90 days)
	Registerer          prometheus.Registerer
	Logger              *slog.Logger
}

// Report summarizes one run.
type Report struct {
	Created   int
	DaysAhead map[string]int
	Dropped   []string
	Refused   []string
}

// Job runs the maintenance.
type Job struct{ o Options }

// New validates options.
func New(o Options) (*Job, error) {
	if o.Module == "" || o.DB == nil {
		return nil, errors.New("partitionmaint: needs a module and the partition_admin pool")
	}
	if o.Clock == nil {
		o.Clock = clock.System()
	}
	if o.DaysAhead <= MinDaysAhead {
		o.DaysAhead = MinDaysAhead + 2
	}
	if o.OutboxRetention <= 0 {
		o.OutboxRetention = 30 * 24 * time.Hour
	}
	if o.IdempotencyTTL <= 0 {
		o.IdempotencyTTL = 24 * time.Hour
	}
	if o.DeadLetterRetention <= 0 {
		o.DeadLetterRetention = 90 * 24 * time.Hour
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Registerer != nil {
		for _, c := range []prometheus.Collector{daysAhead, dropped, refused} {
			_ = o.Registerer.Register(c)
		}
	}
	return &Job{o: o}, nil
}

func day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (j *Job) retention(table string) time.Duration {
	switch table {
	case "idempotency_records":
		return j.o.IdempotencyTTL + 24*time.Hour // keep the day of the last valid key
	case "dead_letters":
		return j.o.DeadLetterRetention
	default:
		return j.o.OutboxRetention
	}
}

// RunOnce creates upcoming partitions, reports coverage and drops expired partitions.
func (j *Job) RunOnce(ctx context.Context) (Report, error) {
	today := day(j.o.Clock.Now())
	rep := Report{DaysAhead: map[string]int{}}
	if err := j.o.DB.QueryRowContext(ctx, `SELECT infra.ensure_partitions($1::date, $2)`, today, j.o.DaysAhead).Scan(&rep.Created); err != nil {
		return rep, fmt.Errorf("partitionmaint: ensure: %w", err)
	}
	for _, t := range Tables {
		var n int
		if err := j.o.DB.QueryRowContext(ctx, `SELECT infra.partition_days_ahead($1, $2::date)`, t, today).Scan(&n); err != nil {
			return rep, fmt.Errorf("partitionmaint: days ahead %s: %w", t, err)
		}
		rep.DaysAhead[t] = n
		daysAhead.WithLabelValues(j.o.Module, t).Set(float64(n))
		if n < MinDaysAhead {
			j.o.Logger.WarnContext(ctx, "infra partitions missing", "module", j.o.Module, "table", t, "days_ahead", n, "alert", "P2")
		}
	}
	for _, t := range Tables {
		before := day(today.Add(-j.retention(t)))
		rows, err := j.o.DB.QueryContext(ctx, `SELECT partition_name, action FROM infra.drop_expired_partitions($1, $2::date)`, t, before)
		if err != nil {
			return rep, fmt.Errorf("partitionmaint: drop %s: %w", t, err)
		}
		if err := j.collect(ctx, rows, t, &rep); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

func (j *Job) collect(ctx context.Context, rows *sql.Rows, table string, rep *Report) error {
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, action string
		if err := rows.Scan(&name, &action); err != nil {
			return err
		}
		if action == "refused" {
			rep.Refused = append(rep.Refused, name)
			refused.WithLabelValues(j.o.Module, table).Inc()
			j.o.Logger.WarnContext(ctx, "expired infra partition kept: unpublished outbox rows or unresolved dead letters",
				"module", j.o.Module, "partition", name)
			continue
		}
		rep.Dropped = append(rep.Dropped, name)
		dropped.WithLabelValues(j.o.Module, table).Inc()
	}
	return rows.Err()
}
