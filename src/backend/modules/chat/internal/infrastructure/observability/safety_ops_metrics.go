package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics of the Safety Team operations (F124 plan §Quan sát, MONITORING §3): levels, roles and statuses only — no user,
// case or tenant label (R14).
var (
	safetyOpsOnce sync.Once

	oncallPages = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_moderation_oncall_pages_total",
		Help: "On-call pages of unclaimed p1 cases, by level (primary, secondary, lead, system_admin)."}, []string{"level"})
	oncallCoverage = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_moderation_oncall_coverage",
		Help: "1 when a live on-call shift of the role covers now, 0 otherwise (alert ModerationOncallGap; refreshed by oncall-escalator)."},
		[]string{"oncall_role"})
	breakGlass = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_break_glass_grants_total",
		Help: "Break-glass grant transitions by status (pending, approved, rejected, expired, revoked)."}, []string{"status"})
)

// RegisterSafetyOps registers the F124 metrics once.
func RegisterSafetyOps(reg prometheus.Registerer) {
	safetyOpsOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{oncallPages, oncallCoverage, breakGlass} {
			_ = reg.Register(c)
		}
	})
}

// SafetyOps implements command.OncallMetrics and command.BreakGlassMetrics.
type SafetyOps struct{}

// Paged counts one page of level.
func (SafetyOps) Paged(level string) { oncallPages.WithLabelValues(level).Inc() }

// Coverage sets eduworld_moderation_oncall_coverage{oncall_role}.
func (SafetyOps) Coverage(role string, covered bool) {
	v := 0.0
	if covered {
		v = 1
	}
	oncallCoverage.WithLabelValues(role).Set(v)
}

// Grant counts one transition of a break-glass grant to status.
func (SafetyOps) Grant(status string) { breakGlass.WithLabelValues(status).Inc() }

// OncallCoverage returns the gauge (tests).
func OncallCoverage() *prometheus.GaugeVec { return oncallCoverage }

// OncallPages returns the counter (tests).
func OncallPages() *prometheus.CounterVec { return oncallPages }
