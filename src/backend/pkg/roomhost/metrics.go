package roomhost

import "github.com/prometheus/client_golang/prometheus"

// Metrics of the room host (F076 plan "Quan sát"): takeovers of a room by another node, timer lag behind due + grace,
// leases lost, rooms hosted and failed timers, by room type.
var (
	takeovers = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_roomhost_lease_takeover_total", Help: "Rooms taken over from another node after their lease expired (failover).",
	}, []string{"room_type"})
	timerLag = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eduworld_roomhost_timer_lag_seconds", Help: "Delay between due time + grace and the timer firing (server clock).",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 15, 60},
	}, []string{"room_type"})
	leasesLost = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_roomhost_lease_lost_total", Help: "Rooms dropped by this node because its lease was lost.",
	}, []string{"room_type"})
	roomsHosted = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_roomhost_rooms", Help: "Rooms hosted by this node.",
	}, []string{"room_type"})
	timerFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_roomhost_timer_failed_total", Help: "Timer handler failures (retried at a later tick).",
	}, []string{"room_type", "reason"})
)

// RegisterMetrics registers the room host metrics.
func RegisterMetrics(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{takeovers, timerLag, leasesLost, roomsHosted, timerFailures} {
		_ = r.Register(c)
	}
}

// PromMetrics implements Metrics with the Prometheus collectors above.
type PromMetrics struct{}

var _ Metrics = PromMetrics{}

// Takeover implements Metrics.
func (PromMetrics) Takeover(t string) { takeovers.WithLabelValues(t).Inc() }

// TimerLag implements Metrics.
func (PromMetrics) TimerLag(t string, s float64) { timerLag.WithLabelValues(t).Observe(max(0, s)) }

// LeaseLost implements Metrics.
func (PromMetrics) LeaseLost(t string) { leasesLost.WithLabelValues(t).Inc() }

// Rooms implements Metrics.
func (PromMetrics) Rooms(t string, n int) { roomsHosted.WithLabelValues(t).Set(float64(n)) }

// TimerFailed implements Metrics.
func (PromMetrics) TimerFailed(t, reason string) { timerFailures.WithLabelValues(t, reason).Inc() }
