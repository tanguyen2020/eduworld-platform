package observability

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
)

// Metrics of Game Analytics (F080, GAME_ANALYTICS §10, MONITORING). Labels never carry a user id, a tenant or content
// (F080-AC-18, R14).
var (
	telemetryEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_telemetry_events_total", Help: "Client telemetry events by type and acceptance (F080).",
	}, []string{"type", "accepted", "reason"})
	analyticsDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_analytics_events_dropped_total", Help: "Domain events not written to analytics by reason (qa_tenant, unknown_student…).",
	}, []string{"reason"})
	rollupLate = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_rollup_late_events_total", Help: "Analytics events later than analytics_late_event_window (rollups unchanged).",
	})
	rawRows = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_analytics_raw_rows", Help: "Rows written to analytics.events_raw (monotonic; size alert AnalyticsRawRowsHigh).",
	})
	rollupDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_analytics_rollup_duration_seconds", Help: "Duration of the analytics rollup of one tenant day.",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 300},
	}, []string{"step"})
	queryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_analytics_query_duration_seconds", Help: "Duration of the Game Analytics dashboard queries.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	}, []string{"query", "cache"})
	dashRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_dashboard_rejected_total", Help: "Game Analytics queries rejected by reason (range, busy, timeout).",
	}, []string{"reason"})
	matchDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_match_duration_seconds", Help: "Duration of completed matches by mode (FR-OPS-003).",
		Buckets: []float64{30, 60, 120, 180, 300, 600, 900, 1800},
	}, []string{"mode"})
)

// RegisterAnalytics registers the F080 metrics.
func RegisterAnalytics(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{telemetryEvents, analyticsDropped, rollupLate, rawRows, rollupDuration, queryDuration, dashRejected,
		matchDuration, clientCrashes, chatSeqGaps, contentDownload} {
		_ = r.Register(c)
	}
}

// AnalyticsMetrics implements port.AnalyticsMetrics.
type AnalyticsMetrics struct{}

var _ port.AnalyticsMetrics = AnalyticsMetrics{}

func boolLabel(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Telemetry counts one telemetry event.
func (AnalyticsMetrics) Telemetry(typ string, accepted bool, reason string) {
	telemetryEvents.WithLabelValues(typ, boolLabel(accepted), reason).Inc()
}

// Dropped counts one domain event not written.
func (AnalyticsMetrics) Dropped(reason string) { analyticsDropped.WithLabelValues(reason).Inc() }

// LateEvent counts one event later than the late window.
func (AnalyticsMetrics) LateEvent() { rollupLate.Inc() }

// RawInserted counts the rows written.
func (AnalyticsMetrics) RawInserted(n int) { rawRows.Add(float64(max(n, 0))) }

// Rollup observes one rollup step.
func (AnalyticsMetrics) Rollup(step string, seconds float64) {
	rollupDuration.WithLabelValues(step).Observe(seconds)
}

// Query observes one dashboard query.
func (AnalyticsMetrics) Query(id string, seconds float64, cache string) {
	queryDuration.WithLabelValues(id, cache).Observe(seconds)
}

// Rejected counts one rejected dashboard query.
func (AnalyticsMetrics) Rejected(reason string) { dashRejected.WithLabelValues(reason).Inc() }

// MatchDuration observes one completed match.
func (AnalyticsMetrics) MatchDuration(mode string, seconds float64) {
	if mode == "" {
		mode = "unknown"
	}
	matchDuration.WithLabelValues(mode).Observe(seconds)
}

// Client metrics of MONITORING §3 fed by client telemetry (F080) and by createGameSession (sessions).
var (
	clientCrashes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_client_crashes_total", Help: "Client crash telemetry events by platform and client version (ClientCrashRateHigh).",
	}, []string{"platform", "client_version"})
	chatSeqGaps = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_chat_seq_gaps_total", Help: "Chat messages the client found missing (seq gaps), by channel type (client telemetry).",
	}, []string{"channel_type"})
	contentDownload = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_client_content_download_bytes_total", Help: "Content bytes downloaded by game clients, by source (preinstalled, lan_cache, cdn).",
	}, []string{"source"})
)

// ClientCrashes exposes eduworld_client_crashes_total (tests; registered by RegisterAnalytics).
func ClientCrashes() *prometheus.CounterVec { return clientCrashes }

// ChatSeqGaps exposes eduworld_chat_seq_gaps_total (tests).
func ChatSeqGaps() *prometheus.CounterVec { return chatSeqGaps }

// ContentDownloadBytes exposes eduworld_client_content_download_bytes_total (tests).
func ContentDownloadBytes() *prometheus.CounterVec { return contentDownload }

// ClientSessions exposes eduworld_client_sessions_total (tests).
func ClientSessions() *prometheus.CounterVec { return clientSessions }

// Client implements port.AnalyticsMetrics: the label values come from closed lists validated by the domain
// (channel_type, source) or are bounded by ClientLabels (platform, client_version).
func (AnalyticsMetrics) Client(m analytics.ClientMetric, platform, clientVersion string) {
	switch m.Kind {
	case "crash":
		clientCrashes.WithLabelValues(ClientLabels(platform, clientVersion)).Add(m.Value)
	case "chat_seq_gap":
		chatSeqGaps.WithLabelValues(m.Label).Add(m.Value)
	case "content_download":
		contentDownload.WithLabelValues(m.Label).Add(m.Value)
	}
}

// ClientLabels bounds (platform, client_version): a platform outside the contract list and a version that is not
// major.minor.patch read "unknown"; a pre-release or build suffix is dropped so a client can never create free series.
func ClientLabels(platform, clientVersion string) (string, string) {
	if !gamesession.ValidPlatform(platform) {
		platform = analytics.Unknown
	}
	v, err := gamesession.ParseVersion(clientVersion)
	if err != nil {
		return platform, analytics.Unknown
	}
	return platform, strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}

// AnalyticsDroppedCounter exposes eduworld_analytics_events_dropped_total (tests of F080/F134 read it with testutil).
func AnalyticsDroppedCounter() *prometheus.CounterVec { return analyticsDropped }
