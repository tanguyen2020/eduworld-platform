package observability

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// Moderation pipeline metrics (F116 T035, plan §Quan sát): no user, channel id or content label (R14). The tenant label
// identifies a school organisation (dashboard "tenants with moderation disabled", MODERATION §12.5).
var (
	moderationOnce sync.Once

	filterLatency = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_chat_filter_latency_seconds",
		Help:    "Synchronous moderation pipeline latency per message (moderation_pipeline_latency: p95 < 20 ms, ceiling 50 ms).",
		Buckets: []float64{0.0005, 0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1}})
	sensitiveMatches = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_sensitive_matches_total",
		Help: "Catalogue matches by category code and action (never the term)."}, []string{"category", "action"})
	uncheckedMessages = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_unchecked_messages_total",
		Help: "Messages delivered with decision unchecked (every content step switched off)."}, []string{"tenant"})
	policyDisabled = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_chat_policy_disabled",
		Help: "1 while a moderation step is switched off for a tenant (tenant=platform for the platform level; alert ChatModerationDisabled)."},
		[]string{"tenant", "step"})
	catalogVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_chat_sensitive_catalog_version",
		Help: "Newest catalogue version published (scope platform or tenant)."}, []string{"scope"})
	re2Timeouts = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_chat_re2_timeouts_total",
		Help: "RE2 patterns skipped after exceeding the 5 ms budget (sensitive_terms_engine_limits)."})
	decisionsDropped = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_chat_decision_log_dropped_total",
		Help: "Decision log records of blocked messages not written (buffer full or PostgreSQL down)."})
	invalidPatterns = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_chat_sensitive_invalid_patterns_total",
		Help: "Stored pattern terms skipped at compilation because they are not allowed RE2."})
)

// RegisterModeration registers the moderation metrics once.
func RegisterModeration(reg prometheus.Registerer) {
	moderationOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{filterLatency, sensitiveMatches, uncheckedMessages, policyDisabled, catalogVersion,
			re2Timeouts, decisionsDropped, invalidPatterns} {
			_ = reg.Register(c)
		}
	})
}

// Moderation implements port.ModerationMetrics.
type Moderation struct{}

var _ port.ModerationMetrics = Moderation{}

// FilterLatency implements port.ModerationMetrics.
func (Moderation) FilterLatency(d time.Duration) { filterLatency.Observe(d.Seconds()) }

// Match implements port.ModerationMetrics.
func (Moderation) Match(category, action string) {
	sensitiveMatches.WithLabelValues(category, action).Inc()
}

// Unchecked implements port.ModerationMetrics.
func (Moderation) Unchecked(tenant uuid.UUID) {
	uncheckedMessages.WithLabelValues(tenant.String()).Inc()
}

// RE2Timeouts implements port.ModerationMetrics.
func (Moderation) RE2Timeouts(n int) { re2Timeouts.Add(float64(n)) }

// DecisionDropped implements port.ModerationMetrics.
func (Moderation) DecisionDropped() { decisionsDropped.Inc() }

// InvalidPatterns counts invalid stored patterns skipped at compilation.
func InvalidPatterns(n int) { invalidPatterns.Add(float64(n)) }

// PolicyDisabled sets eduworld_chat_policy_disabled{tenant,step} (1 while off, 0 otherwise).
func PolicyDisabled(tenant, step string, off bool) {
	v := 0.0
	if off {
		v = 1
	}
	policyDisabled.WithLabelValues(tenant, step).Set(v)
}

// CatalogVersion sets eduworld_chat_sensitive_catalog_version{scope}.
func CatalogVersion(scope string, v int64) { catalogVersion.WithLabelValues(scope).Set(float64(v)) }
