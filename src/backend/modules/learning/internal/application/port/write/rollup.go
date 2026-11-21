package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
)

// RollupStore keeps the time-series rollups of the dashboards incrementally (F043 US1, DASHBOARD_DATA §4) in the
// TenantScope of ctx, inside the transaction of the progress_daily consumer (one processed_events row for both).
type RollupStore interface {
	// DayFinal reports whether the day (date part) is closed for the session tenant.
	DayFinal(ctx context.Context, day time.Time) (bool, error)
	// AddRollups adds the delta of one event to the class (class snapshot at the time), school and tenant rows of its day.
	AddRollups(ctx context.Context, d ProgressDelta) error
	// MarkReclose flags a closed day that received a late event within dashboard_late_window_hours.
	MarkReclose(ctx context.Context, day time.Time) error
}

// Period is one bucket of a rollup level to recompute: Suffix "daily", "weekly" or "monthly", Bucket the bucket_date,
// [From, To] the calendar days it covers, Final its is_final.
type Period struct {
	Suffix   string
	Bucket   time.Time
	From, To time.Time
	Final    bool
}

// RollupJobs are the statements of the jobs dashboard-day-close and read-model-refresh (F043 US1) in the TenantScope of
// ctx. The worker reads the sources on the primary (progress_daily, enrolments, assignment submissions, masteries, learning
// events); the dashboards never do (F043-AC-18).
type RollupJobs interface {
	// Tenants lists the active tenants (platform job bypass).
	Tenants(ctx context.Context) ([]uuid.UUID, error)
	// DaysToClose lists, ascending, up to limit days ≤ last that have unclosed rollup rows or are marked for reclosing.
	DaysToClose(ctx context.Context, last time.Time, limit int) ([]time.Time, error)
	// Recompute rebuilds the class, school and tenant rows of one bucket from progress_daily (distinct counts included) and
	// the assignment submissions, and sets their is_final.
	Recompute(ctx context.Context, p Period) error
	// MarkClosed records the day as closed (rollup_days: is_final, needs_reclose = false, closed_at).
	MarkClosed(ctx context.Context, day, at time.Time) error
	// ObjectiveWeek rebuilds rollup_class_objective_weekly of the ISO week [start, start+7d) from the scored first answers
	// (window in UTC instants [from, to)); withLevels also copies the current level distribution of rm_class_objective_stats.
	ObjectiveWeek(ctx context.Context, start, from, to time.Time, final, withLevels bool) error
	// RefreshReadModels rebuilds rm_class_objective_stats, rm_class_subject_summary, rm_school_grade_summary at now.
	RefreshReadModels(ctx context.Context, now time.Time, today time.Time, th analytics.Thresholds) error
	// AttentionSignals returns the facts of the "needs attention" rule of every student of the tenant.
	AttentionSignals(ctx context.Context, now time.Time, today time.Time, th analytics.Thresholds) ([]AttentionRow, error)
	// SaveAttention writes rm_student_attention.
	SaveAttention(ctx context.Context, rows []AttentionRow) error
	// SnapshotMastery upserts mastery_snapshots_weekly of the ISO week starting at start from the current masteries.
	SnapshotMastery(ctx context.Context, start time.Time) error
}

// AttentionRow is one student of rm_student_attention.
type AttentionRow struct {
	StudentID, SchoolID uuid.UUID
	Signals             analytics.Signals
	Reasons             []analytics.Reason
}
