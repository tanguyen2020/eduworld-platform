package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// Safety Team metrics (F121 T041, plan §Quan sát, MODERATION §6.3): no user, case id or content label (R14).
var (
	safetyOnce sync.Once

	casesOpen = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_moderation_cases_open",
		Help: "Moderation cases not resolved, by severity and assignment (refreshed by moderation-sla-escalator)."},
		[]string{"severity", "assigned"})
	unclaimedAge = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_moderation_case_unclaimed_age_seconds",
		Help: "Age of the oldest open case without assignee, by severity (alert ModerationP1Unclaimed after p1_unclaimed_alert_minutes)."},
		[]string{"severity"})
	firstResponse = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_moderation_case_first_response_seconds",
		Help:    "Time from case creation to the first response (assignment) by severity (report_sla).",
		Buckets: []float64{60, 300, 900, 1800, 3600, 4 * 3600, 24 * 3600, 3 * 24 * 3600}}, []string{"severity"})
	reportsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_reports_total",
		Help: "Reports created by students, by reason (MODERATION §5)."}, []string{"reason"})
	notificationsOverdue = prometheus.NewGauge(prometheus.GaugeOpts{Name: "eduworld_moderation_school_notifications_overdue",
		Help: "School safety notifications past ack_due_at without acknowledgement (school_notification_ack_sla_hours)."})
)

// RegisterSafety registers the Safety Team metrics once.
func RegisterSafety(reg prometheus.Registerer) {
	safetyOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{casesOpen, unclaimedAge, firstResponse, reportsTotal, notificationsOverdue} {
			_ = reg.Register(c)
		}
	})
}

// Safety implements command.SafetyMetrics and command.SafetyGauges.
type Safety struct{}

// Report counts one report.
func (Safety) Report(reason string) { reportsTotal.WithLabelValues(reason).Inc() }

// FirstResponse observes the first response of a case.
func (Safety) FirstResponse(severity string, d time.Duration) {
	firstResponse.WithLabelValues(severity).Observe(d.Seconds())
}

// Cases sets eduworld_moderation_cases_open{severity,assigned} and the oldest unclaimed age per severity (every label
// reset each refresh: a bucket without cases reads 0).
func (Safety) Cases(counts []read.OpenCount, now time.Time) {
	for _, sev := range []string{"p1", "p2", "p3"} {
		casesOpen.WithLabelValues(sev, "true").Set(0)
		casesOpen.WithLabelValues(sev, "false").Set(0)
		unclaimedAge.WithLabelValues(sev).Set(0)
	}
	for _, c := range counts {
		assigned := "false"
		if c.Assigned {
			assigned = "true"
		}
		casesOpen.WithLabelValues(c.Severity, assigned).Set(float64(c.Count))
		if !c.OldestUnclaimed.IsZero() {
			unclaimedAge.WithLabelValues(c.Severity).Set(now.Sub(c.OldestUnclaimed).Seconds())
		}
	}
}

// OverdueNotifications sets eduworld_moderation_school_notifications_overdue.
func (Safety) OverdueNotifications(n int) { notificationsOverdue.Set(float64(n)) }

// CasesOpen returns the gauge (tests).
func CasesOpen() *prometheus.GaugeVec { return casesOpen }

// UnclaimedAge returns the gauge (tests).
func UnclaimedAge() *prometheus.GaugeVec { return unclaimedAge }
