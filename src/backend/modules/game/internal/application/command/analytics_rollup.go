package command

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Job names of Game Analytics (SystemPlatformScope actor, logs, metrics).
const (
	JobAnalyticsRollup    = "analytics-rollup-hourly"
	JobAnalyticsDayClose  = "dashboard-day-close-game"
	JobAnalyticsRetention = "analytics-retention"
	JobAnalyticsExport    = "analytics-export"
)

// RetentionDays are the day offsets of analytics.cohort_retention (retention_d1/d7/d30 plus d3/d14 for the heatmap).
var RetentionDays = []int{1, 3, 7, 14, 30}

// AnalyticsRollup are the rollup jobs (F080-AC-08, AC-09, AC-10, AC-35, DASHBOARD_DATA §4–§5): the hourly job recomputes
// the open days (today, days with raw events not final) as provisional rows; the day close job finalises every day up to
// LastClosable (dashboard_day_close_time) and the days marked by late events within analytics_late_event_window, then
// bumps the data version of the tenant (cache invalidation). Each tenant day is recomputed from its sources (idempotent
// upserts): player_days from events_raw, daily metrics, cohorts, funnels, content and performance, then the week and the
// month containing the day from the daily rows and player_days. Runs on the platform pool (game_platform, explicit tenant
// in every statement).
type AnalyticsRollup struct {
	UoW      *entx.UnitOfWork
	Store    port.AnalyticsStore
	Versions port.DashboardCache
	Metrics  port.AnalyticsMetrics
	Config   analytics.Config
	Clock    func() time.Time
}

func (h AnalyticsRollup) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

// RunHourly is one run of analytics-rollup-hourly: provisional rows of the open days of the last late window.
func (h AnalyticsRollup) RunHourly(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.AnalyticsRollupHourly")
	defer func() { done(err) }()
	now := h.now()
	today := analytics.DayOf(now)
	from := analytics.DayOf(now.Add(-h.Config.LateWindow))
	return h.run(ctx, JobAnalyticsRollup, from, today, func(d time.Time) bool { return false })
}

// CloseDays is one run of the day close job: every pending day up to LastClosable becomes final (is_final = true).
func (h AnalyticsRollup) CloseDays(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.AnalyticsCloseDays")
	defer func() { done(err) }()
	now := h.now()
	last := h.Config.LastClosable(now)
	from := analytics.DayOf(now.Add(-h.Config.LateWindow)).AddDate(0, 0, -1)
	return h.run(ctx, JobAnalyticsDayClose, from, last, func(d time.Time) bool { return !d.After(last) })
}

func (h AnalyticsRollup) run(ctx context.Context, job string, from, to time.Time, final func(time.Time) bool) (int, error) {
	ps, err := authz.SystemPlatformScope(job)
	if err != nil {
		return 0, err
	}
	var days []port.TenantDay
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		days, err = h.Store.PendingDays(ctx, from, to)
		return err
	}); err != nil {
		return 0, err
	}
	sort.Slice(days, func(i, j int) bool {
		if !days[i].Day.Equal(days[j].Day) {
			return days[i].Day.Before(days[j].Day)
		}
		return days[i].TenantID.String() < days[j].TenantID.String()
	})
	var errs []error
	n := 0
	bumped := map[uuid.UUID]bool{}
	for _, d := range days {
		if d.Day.After(to) {
			continue
		}
		fin := final(d.Day)
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error { return h.Day(ctx, d, fin) }); err != nil {
			errs = append(errs, fmt.Errorf("rollup %s %s: %w", d.TenantID, d.Day.Format(time.DateOnly), err))
			continue
		}
		n++
		if fin && h.Versions != nil && !bumped[d.TenantID] {
			bumped[d.TenantID] = true
			_ = h.Versions.Bump(ctx, d.TenantID) // cache invalidation; a Valkey error only delays it (ADR-0004)
		}
	}
	return n, errors.Join(errs...)
}

// Day recomputes one tenant day and the week and month containing it (transaction of ctx).
func (h AnalyticsRollup) Day(ctx context.Context, d port.TenantDay, final bool) error {
	start := time.Now()
	step := func(name string, t0 time.Time) {
		if h.Metrics != nil {
			h.Metrics.Rollup(name, time.Since(t0).Seconds())
		}
	}
	if err := h.Store.RebuildPlayerDays(ctx, d, h.Config.ActiveSessionMin); err != nil {
		return err
	}
	rows, err := h.Store.DailyRows(ctx, d)
	if err != nil {
		return err
	}
	funnelRows, err := h.Store.Funnels(ctx, d, final)
	if err != nil {
		return err
	}
	rows = append(rows, funnelRows...)
	for i := range rows {
		rows[i].Final = final
		if m, ok := analytics.Lookup(rows[i].Code); ok {
			rows[i].Version = m.Version
			rows[i].Value = m.Value(rows[i].Cell)
		}
	}
	if err := h.Store.UpsertRollups(ctx, "rollup_daily", d.TenantID, rows); err != nil {
		return err
	}
	if err := h.Store.Cohorts(ctx, d, RetentionDays, final); err != nil {
		return err
	}
	week := analytics.BucketStart(d.Day, analytics.Week)
	if err := h.Store.ContentWeek(ctx, d.TenantID, week, d.Day, final && !analytics.BucketEnd(week, analytics.Week).After(d.Day)); err != nil {
		return err
	}
	if err := h.Store.PerfDay(ctx, d, final); err != nil {
		return err
	}
	for _, g := range []analytics.Grain{analytics.Week, analytics.Month} {
		if err := h.period(ctx, d, g, final); err != nil {
			return err
		}
	}
	if err := h.Store.CloseDay(ctx, d, final, h.now()); err != nil {
		return err
	}
	step("day", start)
	return nil
}

// period rebuilds the weekly or monthly rows of the bucket containing the day: merged daily rows (counts averaged over
// the days, ratios and sums added, histograms merged, rolling counts from the last day) with the distinct players of the
// period, plus the period-only metrics from player_days.
func (h AnalyticsRollup) period(ctx context.Context, d port.TenantDay, g analytics.Grain, final bool) error {
	from := analytics.BucketStart(d.Day, g)
	to := analytics.BucketEnd(from, g)
	upTo := to
	if d.Day.Before(upTo) {
		upTo = d.Day
	}
	daily, err := h.Store.ReadDaily(ctx, d.TenantID, from, upTo)
	if err != nil {
		return err
	}
	extra, players, err := h.Store.PeriodRows(ctx, d.TenantID, from, upTo)
	if err != nil {
		return err
	}
	out := MergePeriod(daily, players, from)
	periodFinal := final && !to.After(d.Day)
	for _, r := range extra {
		r.Date = from
		out = append(out, r)
	}
	for i := range out {
		out[i].Final = periodFinal
		if m, ok := analytics.Lookup(out[i].Code); ok {
			out[i].Version = m.Version
			out[i].Value = m.Value(out[i].Cell)
		}
	}
	table := "rollup_weekly"
	if g == analytics.Month {
		table = "rollup_monthly"
	}
	return h.Store.UpsertRollups(ctx, table, d.TenantID, out)
}

// MergePeriod merges the daily rows of one period per (metric, dims): KindCount sums the days and keeps their number as
// the denominator (average), KindLast keeps the last day, others add numerator/denominator and histograms. The players of
// each row are the distinct active players of the period for its dims (suppression), when known.
func MergePeriod(daily []port.RollupRow, players map[string]int64, bucket time.Time) []port.RollupRow {
	type acc struct {
		row  port.RollupRow
		last time.Time
		days int64
	}
	byKey := map[string]*acc{}
	var keys []string
	for _, r := range daily {
		m, ok := analytics.Lookup(r.Code)
		if !ok || m.Kind == analytics.KindWeekly {
			continue
		}
		k := r.Code + "\x00" + r.Dims.Key()
		a, ok := byKey[k]
		if !ok {
			a = &acc{row: port.RollupRow{Date: bucket, Code: r.Code, Dims: r.Dims}}
			byKey[k] = a
			keys = append(keys, k)
		}
		switch m.Kind {
		case analytics.KindLast:
			if r.Date.After(a.last) || a.days == 0 {
				a.row.Cell, a.last = r.Cell, r.Date
			}
		case analytics.KindCount:
			a.row.Cell.Num += r.Cell.Num
			a.row.Cell.Players = max(a.row.Cell.Players, r.Cell.Players)
		default:
			a.row.Cell = analytics.Metric{Kind: m.Kind}.Merge(a.row.Cell, r.Cell)
		}
		a.days++
	}
	sort.Strings(keys)
	out := make([]port.RollupRow, 0, len(keys))
	for _, k := range keys {
		a := byKey[k]
		if m, _ := analytics.Lookup(a.row.Code); m.Kind == analytics.KindCount {
			a.row.Cell.Den = a.days
		}
		if p, ok := players[a.row.Dims.Key()]; ok {
			a.row.Cell.Players = p
		}
		out = append(out, a.row)
	}
	return out
}

// RunRetention is one run of analytics-retention (GAME_ANALYTICS §9, ADR-0070): raw rows older than
// analytics_raw_retention_days and player_days older than analytics_player_days_retention_days lose their keys; nothing
// is deleted and the aggregates stay.
func (h AnalyticsRollup) RunRetention(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.AnalyticsRetention")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope(JobAnalyticsRetention)
	if err != nil {
		return 0, err
	}
	today := analytics.DayOf(h.now())
	err = h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		raw, days, err := h.Store.AnonymizeBefore(ctx, today.Add(-h.Config.RawRetention), today.Add(-h.Config.PlayerDaysRetention), batch)
		n = int(raw + days)
		return err
	})
	return n, err
}
