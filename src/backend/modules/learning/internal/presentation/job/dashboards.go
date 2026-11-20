package job

import "context"

// DashboardRunner runs the dashboard rollup jobs (command.Dashboards, F043 US1).
type DashboardRunner interface {
	CloseDays(ctx context.Context) (int, error)
	Refresh(ctx context.Context) (int, error)
}

// DashboardDayClose is the job dashboard-day-close of the worker profile jobs (F043-AC-03, F043-AC-04, DASHBOARD_DATA §4–§5):
// each run closes, per tenant, the days up to the last closable one (after dashboard_day_close_time) and recloses the days
// marked by late events.
type DashboardDayClose struct{ Runner DashboardRunner }

// RunOnce closes the pending days and returns how many.
func (j DashboardDayClose) RunOnce(ctx context.Context) (int, error) { return j.Runner.CloseDays(ctx) }

// ReadModelRefresh is the job read-model-refresh (F043-AC-06, every school_dashboard_refresh): provisional rollups of today,
// state read models, "needs attention" list and weekly mastery snapshot.
type ReadModelRefresh struct{ Runner DashboardRunner }

// RunOnce refreshes every tenant and returns how many.
func (j ReadModelRefresh) RunOnce(ctx context.Context) (int, error) { return j.Runner.Refresh(ctx) }
