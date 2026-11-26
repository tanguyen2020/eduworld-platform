package port

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
)

// RawEvent is one row of analytics.events_raw (F080, DATABASE_DESIGN §2.8b): pseudonymous keys, standard dimensions and
// catalog props only (never a student id, a name, a nickname or chat content — F080-AC-06).
type RawEvent struct {
	EventID       uuid.UUID
	OccurredAt    time.Time
	TenantID      uuid.UUID
	SchoolID      *uuid.UUID
	GradeLevel    *int
	Platform      string
	Locale        string
	DeviceTier    string
	ClientVersion string
	PlayerKey     string
	SessionKey    string
	KeyVersion    int
	EventName     string
	Source        string // server | client
	Props         map[string]any
}

// TenantFlags are the analytics flags of a tenant projected from module identity (ref.tenants): the QA tenant (never
// counted, GAME_ANALYTICS A-8) and analytics_product_telemetry_enabled (settings_subset, default on).
type TenantFlags struct {
	Known            bool
	QA               bool
	ProductTelemetry bool
}

// StudentDims are the dimensions of a student read from the projections (ref.students) and its game session.
type StudentDims struct {
	Known         bool
	SchoolID      *uuid.UUID
	GradeLevel    *int
	SessionID     *uuid.UUID // active (or last) game session
	Platform      string
	ClientVersion string
}

// TenantDay is one tenant × analytics day to roll up.
type TenantDay struct {
	TenantID uuid.UUID
	Day      time.Time
}

// RollupRow is one row of analytics.rollup_daily/weekly/monthly.
type RollupRow struct {
	Date    time.Time
	Code    string
	Version int
	Dims    analytics.Dims
	Cell    analytics.Cell
	Value   *float64
	Final   bool
}

// AnalyticsStore is the write side of Game Analytics on the transaction of ctx: raw ingestion, projections lookups,
// anonymisation (no physical delete — ADR-0070) and the rollup steps of the jobs (explicit SQL, tenant named in every
// statement).
type AnalyticsStore interface {
	TenantFlags(ctx context.Context, tenant uuid.UUID) (TenantFlags, error)
	StudentDims(ctx context.Context, tenant, student uuid.UUID) (StudentDims, error)
	// MatchStart is the occurred_at of the match_start row of a match (within two days before end).
	MatchStart(ctx context.Context, tenant uuid.UUID, matchID string, end time.Time) (time.Time, bool, error)
	DayFinal(ctx context.Context, tenant uuid.UUID, day time.Time) (bool, error)
	MarkReclose(ctx context.Context, tenant uuid.UUID, day time.Time) error
	// InsertRaw stores the events once per (occurred_on, event_id) and returns how many were new.
	InsertRaw(ctx context.Context, rows []RawEvent) (int, error)
	// Anonymize sets player_key/session_key to NULL for the keys in raw and player_days.
	Anonymize(ctx context.Context, playerKeys []string) (raw, days int64, err error)
	// AnonymizeBefore anonymises raw rows older than rawBefore and player_days older than daysBefore (retention job).
	AnonymizeBefore(ctx context.Context, rawBefore, daysBefore time.Time, batch int) (raw, days int64, err error)

	// PendingDays lists the tenant days with raw events in [from, to] that are not final, plus the days marked for reclose.
	PendingDays(ctx context.Context, from, to time.Time) ([]TenantDay, error)
	RebuildPlayerDays(ctx context.Context, d TenantDay, activeMin time.Duration) error
	DailyRows(ctx context.Context, d TenantDay) ([]RollupRow, error)
	UpsertRollups(ctx context.Context, table string, tenant uuid.UUID, rows []RollupRow) error
	ReadDaily(ctx context.Context, tenant uuid.UUID, from, to time.Time) ([]RollupRow, error)
	// PeriodRows computes the period-only metrics (active_days_per_week, social_play_rate, match_participation_rate,
	// shop_conversion) and the distinct active players of each dims value of [from, to] from player_days.
	PeriodRows(ctx context.Context, tenant uuid.UUID, from, to time.Time) (rows []RollupRow, players map[string]int64, err error)
	Cohorts(ctx context.Context, d TenantDay, dayN []int, final bool) error
	Funnels(ctx context.Context, d TenantDay, final bool) ([]RollupRow, error)
	ContentWeek(ctx context.Context, tenant uuid.UUID, weekStart, upTo time.Time, final bool) error
	PerfDay(ctx context.Context, d TenantDay, final bool) error
	CloseDay(ctx context.Context, d TenantDay, final bool, at time.Time) error
}

// DashboardRow is one rollup row read by a dashboard query (tenant level, merged by the query).
type DashboardRow struct {
	Date     time.Time
	TenantID uuid.UUID
	Code     string
	Dims     analytics.Dims
	Cell     analytics.Cell
	Final    bool
	Computed time.Time
}

// RetentionRow is one row of analytics.cohort_retention.
type RetentionRow struct {
	Cohort   time.Time
	DayN     int
	Size     int64
	Retained int64
	Final    bool
	Computed time.Time
}

// FunnelRow is one step of analytics.funnel_daily.
type FunnelRow struct {
	Step     int
	Count    int64
	Final    bool
	Computed time.Time
}

// ContentRow is one aggregated row of analytics.content_stats_weekly.
type ContentRow struct {
	Kind        string
	ContentID   uuid.UUID
	Started     int64
	Completed   int64
	Abandoned   int64
	Succeeded   int64
	Ended       int64
	Players     int64
	DurationP50 *float64
	Final       bool
	Computed    time.Time
	SortValue   float64
}

// PerfRow is one aggregated row of analytics.perf_daily.
type PerfRow struct {
	Platform, DeviceTier, Preset, Renderer, ClientVersion string
	Samples, Sessions, Players                            int64
	FPSP50, FPSP10, BudgetPass, LoadP50, CrashFree        *float64
	Final                                                 bool
	Computed                                              time.Time
}

// DashboardQuery is the scope and range of a catalog query (tenants nil = every non-QA tenant of a platform read).
type DashboardQuery struct {
	Tenants []uuid.UUID
	From    time.Time
	To      time.Time
	Grain   analytics.Grain
}

// AnalyticsDashboard is the query catalog of Game Analytics (DASHBOARD_DATA §8 rule 6): rollup tables only, through the
// dashboard read session (game_dashboard on the replica; game_platform for cross-tenant platform reads), never events_raw
// nor player_days. Every statement names the tenants and a bounded range.
type AnalyticsDashboard interface {
	Prepare(ctx context.Context, timeout time.Duration) error
	Rollups(ctx context.Context, q DashboardQuery, codes []string, dimSet string) ([]DashboardRow, error)
	Retention(ctx context.Context, q DashboardQuery, dimSet string, dimValues []string) ([]RetentionRow, error)
	Funnel(ctx context.Context, q DashboardQuery, code, dimSet string, dimValues []string) ([]FunnelRow, error)
	Content(ctx context.Context, q DashboardQuery, kind, sort string, limit int, after *ContentCursor) ([]ContentRow, error)
	Performance(ctx context.Context, q DashboardQuery, filter PerfFilter) ([]PerfRow, error)
	AvailableFrom(ctx context.Context, tenants []uuid.UUID) (*time.Time, error)
	// SchoolsInTenant returns the given schools that belong to the tenant (ref.schools).
	SchoolsInTenant(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID) ([]uuid.UUID, error)
	// NonQATenants filters the given tenants (nil = all) to the tenants that are not QA (GAME_ANALYTICS A-8).
	NonQATenants(ctx context.Context, tenants []uuid.UUID) ([]uuid.UUID, error)
}

// PerfFilter narrows the performance report.
type PerfFilter struct {
	Platforms, DeviceTiers, ClientVersions []string
}

// ContentCursor is the keyset position of a content stats page (sort value, content id).
type ContentCursor struct {
	Value float64   `json:"v"`
	ID    uuid.UUID `json:"id"`
}

// ExportJob is one analytics.export_jobs row.
type ExportJob struct {
	ID          uuid.UUID
	TenantID    *uuid.UUID
	RequestedBy uuid.UUID
	Report      string
	Params      json.RawMessage
	Status      string
	RowCount    int
	ObjectKey   *string
	ErrorCode   *string
	CreatedAt   time.Time
	FinishedAt  *time.Time
	ExpiresAt   *time.Time
}

// ExportStore keeps the export jobs (platform pool, explicit SQL).
type ExportStore interface {
	ActiveExports(ctx context.Context, user uuid.UUID) (int, error)
	InsertExport(ctx context.Context, j ExportJob) error
	Export(ctx context.Context, id uuid.UUID) (*ExportJob, error)
	ClaimExport(ctx context.Context, now time.Time) (*ExportJob, error)
	FinishExport(ctx context.Context, j ExportJob) error
}

// ExportFiles writes CSV files to the export bucket and signs download links (object storage ew-exports, ADR-0044).
type ExportFiles interface {
	Put(ctx context.Context, key string, body io.Reader, size int64) error
	SignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// DashboardCache caches serialized answers per tenant data version (Valkey, DASHBOARD_DATA §9); every error is a miss.
type DashboardCache interface {
	Version(ctx context.Context, tenant uuid.UUID) (int64, error)
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Bump(ctx context.Context, tenant uuid.UUID) error
}

// DashboardLimiter bounds concurrent dashboard queries per user and tenant (DASHBOARD_DATA §7).
type DashboardLimiter interface {
	Acquire(ctx context.Context, user, tenant uuid.UUID, perUser, perTenant int) (release func(), ok bool)
}

// AnalyticsMetrics observes Game Analytics (GAME_ANALYTICS §10, MONITORING).
type AnalyticsMetrics interface {
	Telemetry(typ string, accepted bool, reason string)
	Dropped(reason string)
	LateEvent()
	RawInserted(n int)
	Rollup(step string, seconds float64)
	Query(queryID string, seconds float64, cache string)
	Rejected(reason string)
	MatchDuration(mode string, seconds float64)
}

// MetricsSource is the Prometheus HTTP API (instant queries of a fixed catalog only — F080-AC-17).
type MetricsSource interface {
	// Query runs one PromQL instant query and returns its samples (label set → value); ErrMetricsUnavailable when
	// Prometheus does not answer.
	Query(ctx context.Context, promql string) ([]Sample, error)
}

// ErrExportTooLarge: an export report has more rows than dashboard_export_max_rows.
var ErrExportTooLarge = errors.New("export: more rows than dashboard_export_max_rows")

// ErrMetricsUnavailable is returned by a MetricsSource when Prometheus does not answer.
var ErrMetricsUnavailable = errors.New("prometheus unavailable")

// Sample is one instant vector sample.
type Sample struct {
	Labels map[string]string
	Value  float64
}
