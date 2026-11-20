package command

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// DashboardVersions bumps the data version of a tenant's dashboards after its rollups changed (Valkey
// ew:learning:dash:ver:<tenant_id>, DASHBOARD_DATA §9): cached results of the old version are never read again. A failure
// is logged only: the cache keys of today expire after dashboard_cache_ttl_today.
type DashboardVersions interface {
	Bump(ctx context.Context, tenantID uuid.UUID) error
}

// RollupMetrics observes the jobs (eduworld_rollup_day_close_duration_seconds).
type RollupMetrics interface {
	DayCloseDuration(seconds float64)
}

// Dashboards runs the jobs of the dashboard rollups (F043 US1, DASHBOARD_DATA §4–§5, LEARNING_ANALYTICS §5): the day close
// (dashboard-day-close) and the read-model refresh (read-model-refresh, every school_dashboard_refresh).
type Dashboards struct {
	UoW      *entx.UnitOfWork
	Jobs     write.RollupJobs
	Versions DashboardVersions
	Config   analytics.Config
	Metrics  RollupMetrics
	Clock    func() time.Time
}

// maxCloseDaysPerRun bounds the days one tenant closes in one run (backlog after an outage is caught up run by run).
const maxCloseDaysPerRun = 62

func (h Dashboards) tenants(ctx context.Context) ([]uuid.UUID, error) {
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return nil, err
	}
	var out []uuid.UUID
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		out, err = h.Jobs.Tenants(ctx)
		return err
	})
	return out, err
}

func (h Dashboards) inTenant(ctx context.Context, tenant uuid.UUID, fn func(ctx context.Context) error) error {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

// periods are the week and the month containing day, final when they ended on or before last.
func periods(day, last time.Time) []write.Period {
	out := make([]write.Period, 0, 2)
	for _, g := range []struct {
		suffix string
		grain  analytics.Grain
	}{{"weekly", analytics.Week}, {"monthly", analytics.Month}} {
		start := analytics.BucketStart(day, g.grain)
		end := analytics.BucketEnd(start, g.grain)
		out = append(out, write.Period{Suffix: g.suffix, Bucket: start, From: start, To: end, Final: !end.After(last)})
	}
	return out
}

// utcWindow is [start, end+1 day) of calendar days of the rollup zone, as UTC instants.
func (h Dashboards) utcWindow(start, end time.Time) (time.Time, time.Time) {
	z := h.Config.Zone
	if z == nil {
		z = analytics.Zone
	}
	from := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, z)
	to := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, z).AddDate(0, 0, 1)
	return from.UTC(), to.UTC()
}

// CloseDays is the job dashboard-day-close (F043-AC-03, F043-AC-04): per tenant, every day up to the last closable one
// (yesterday after dashboard_day_close_time) that still has provisional rows or was marked by a late event is rebuilt from
// progress_daily (exact distinct counts), set is_final, and its week and month are rebuilt from the day table; then the
// data version of the tenant is bumped. Returns the number of days closed.
func (h Dashboards) CloseDays(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.DashboardDayClose")
	defer func() { done(err) }()
	start := time.Now()
	at := now(h.Clock)
	last := h.Config.LastClosable(at)
	tenants, err := h.tenants(ctx)
	if err != nil {
		return 0, err
	}
	for _, t := range tenants {
		closed := 0
		err := h.inTenant(ctx, t, func(ctx context.Context) error {
			days, err := h.Jobs.DaysToClose(ctx, last, maxCloseDaysPerRun)
			if err != nil {
				return err
			}
			for _, d := range days {
				if err := h.closeDay(ctx, d, last, at); err != nil {
					return err
				}
				closed++
			}
			return nil
		})
		if err != nil {
			return n, err
		}
		n += closed
		if closed > 0 && h.Versions != nil {
			if err := h.Versions.Bump(ctx, t); err != nil {
				slog.WarnContext(ctx, "dashboard version bump failed: cached results expire with their ttl", "error", err)
			}
		}
	}
	if h.Metrics != nil {
		h.Metrics.DayCloseDuration(time.Since(start).Seconds())
	}
	return n, nil
}

func (h Dashboards) closeDay(ctx context.Context, day, last, at time.Time) error {
	if err := h.Jobs.Recompute(ctx, write.Period{Suffix: "daily", Bucket: day, From: day, To: day, Final: true}); err != nil {
		return err
	}
	for _, p := range periods(day, last) {
		if err := h.Jobs.Recompute(ctx, p); err != nil {
			return err
		}
	}
	week := analytics.BucketStart(day, analytics.Week)
	weekEnd := analytics.BucketEnd(week, analytics.Week)
	from, to := h.utcWindow(week, weekEnd)
	if err := h.Jobs.ObjectiveWeek(ctx, week, from, to, !weekEnd.After(last), false); err != nil {
		return err
	}
	return h.Jobs.MarkClosed(ctx, day, at)
}

// Refresh is the job read-model-refresh (F043-AC-06, every school_dashboard_refresh): per tenant, the provisional rows of
// today (exact distinct counts and assignment progress), the current week and month, the class × objective week, the state
// read models, the "needs attention" list and the weekly mastery snapshot. Returns the number of tenants refreshed.
func (h Dashboards) Refresh(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.ReadModelRefresh")
	defer func() { done(err) }()
	at := now(h.Clock)
	today := h.Config.Day(at)
	last := h.Config.LastClosable(at)
	tenants, err := h.tenants(ctx)
	if err != nil {
		return 0, err
	}
	for _, t := range tenants {
		err := h.inTenant(ctx, t, func(ctx context.Context) error { return h.refreshTenant(ctx, at, today, last) })
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (h Dashboards) refreshTenant(ctx context.Context, at, today, last time.Time) error {
	for d := last.AddDate(0, 0, 1); !d.After(today); d = d.AddDate(0, 0, 1) {
		if err := h.Jobs.Recompute(ctx, write.Period{Suffix: "daily", Bucket: d, From: d, To: d}); err != nil {
			return err
		}
	}
	for _, p := range periods(today, last) {
		if err := h.Jobs.Recompute(ctx, p); err != nil {
			return err
		}
	}
	if err := h.Jobs.RefreshReadModels(ctx, at, today, h.Config.Attention); err != nil {
		return err
	}
	week := analytics.BucketStart(today, analytics.Week)
	from, to := h.utcWindow(week, analytics.BucketEnd(week, analytics.Week))
	if err := h.Jobs.ObjectiveWeek(ctx, week, from, to, false, true); err != nil {
		return err
	}
	rows, err := h.Jobs.AttentionSignals(ctx, at, today, h.Config.Attention)
	if err != nil {
		return err
	}
	for i := range rows {
		rows[i].Reasons = h.Config.Attention.Evaluate(rows[i].Signals)
	}
	if err := h.Jobs.SaveAttention(ctx, rows); err != nil {
		return err
	}
	return h.Jobs.SnapshotMastery(ctx, week)
}
