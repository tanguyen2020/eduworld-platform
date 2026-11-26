package job

import "context"

// Names of the Game Analytics jobs (F080 T002, T025, T031).
const (
	AnalyticsRollup    = "analytics-rollup-hourly"
	AnalyticsDayClose  = "dashboard-day-close-game"
	AnalyticsRetention = "analytics-retention"
	AnalyticsExport    = "analytics-export"
)

// AnalyticsRunner is the use case behind the rollup jobs (command.AnalyticsRollup).
type AnalyticsRunner interface {
	RunHourly(ctx context.Context) (int, error)
	CloseDays(ctx context.Context) (int, error)
	RunRetention(ctx context.Context, batch int) (int, error)
}

// ExportRunner is the export worker (command.AnalyticsExports).
type ExportRunner interface {
	RunOnce(ctx context.Context) (int, error)
}

// AnalyticsRollupJob is one run of analytics-rollup-hourly (F080-AC-08: provisional rows of the open days).
func AnalyticsRollupJob(r AnalyticsRunner) RunOnce { return r.RunHourly }

// AnalyticsDayCloseJob is one run of the day close (F080-AC-08, AC-09: is_final, late days reclosed, cache version bumped).
func AnalyticsDayCloseJob(r AnalyticsRunner) RunOnce { return r.CloseDays }

// AnalyticsRetentionJob is one run of analytics-retention (anonymisation past retention, ADR-0070).
func AnalyticsRetentionJob(r AnalyticsRunner, batch int) RunOnce {
	return func(ctx context.Context) (int, error) { return r.RunRetention(ctx, batch) }
}

// AnalyticsExportJob is one tick of the export worker (F080-AC-13).
func AnalyticsExportJob(r ExportRunner) RunOnce { return r.RunOnce }
