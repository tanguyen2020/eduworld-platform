// Package observability holds the metrics of module competition (MULTIPLAYER_GAMEPLAY §12, REALTIME_ARCHITECTURE §8,
// §12, F091 T032). Labels never carry ids or PII. Projection lag carries the module in its name (one registry per
// binary, OQ F014-6).
package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
)

var (
	once sync.Once

	roomsActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_rooms_active",
		Help: "Rooms hosted by this instance."}, []string{"room_type"})
	roomPlayers = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_room_players",
		Help: "Seats taken in the rooms hosted by this instance."}, []string{"room_type"})
	assignDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_instance_assign_duration_seconds",
		Help: "Duration of a hub instance assignment (hub.instance.join).", Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1}})
	tickDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_room_tick_duration_seconds",
		Help:    "Duration of one room tick (budget ≤ 5 ms for a hub of 40, alert p99 > 20 ms).",
		Buckets: []float64{.0005, .001, .002, .005, .01, .02, .05}}, []string{"room_type"})
	joinFriend = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_join_friend_total",
		Help: "Join a friend (follow_user_id) outcomes."}, []string{"result"})
	violations = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_hub_violations_total",
		Help: "Hub movements refused by the room host."}, []string{"reason"})
	// speedViolations is the anti-cheat counter of ANTI_CHEAT §10 (F132-AC-24): hub moves above max_speed × 1.2, each
	// answered with a snap back (hub.player.corrected). No label: never a user, room or tenant.
	speedViolations = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_rt_speed_violations_total",
		Help: "Hub moves faster than the area max speed (× tolerance), corrected by the room host (speed hack signal)."})
	snapshotBytes = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_room_snapshot_bytes_total",
		Help: "Bytes of snapshot payloads published per room type (bandwidth budget hub_sync_bandwidth)."}, []string{"room_type"})
	projectionLag = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_competition_projection_lag_seconds",
		Help: "Delay between an event and its projection in module competition.", Buckets: prometheus.ExponentialBuckets(0.01, 4, 8)},
		[]string{"table"})
)

// Register registers the metrics once (nil: process default registry not used).
func Register(reg prometheus.Registerer) {
	once.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{roomsActive, roomPlayers, assignDuration, tickDuration, joinFriend, violations,
			speedViolations, snapshotBytes, projectionLag} {
			_ = reg.Register(c)
		}
	})
}

// Hub implements port.HubMetrics and the room host metrics.
type Hub struct{}

// AssignDuration implements port.HubMetrics.
func (Hub) AssignDuration(d time.Duration) { assignDuration.Observe(d.Seconds()) }

// JoinFriend implements port.HubMetrics.
func (Hub) JoinFriend(result string) { joinFriend.WithLabelValues(result).Inc() }

// Violation implements port.HubMetrics.
// A speed violation also counts in eduworld_rt_speed_violations_total (ANTI_CHEAT §10).
func (Hub) Violation(reason string) {
	violations.WithLabelValues(reason).Inc()
	if reason == hub.ReasonSpeed {
		speedViolations.Inc()
	}
}

// Tick observes one tick.
func (Hub) Tick(roomType string, d time.Duration) {
	tickDuration.WithLabelValues(roomType).Observe(d.Seconds())
}

// Rooms adds delta to the hosted rooms.
func (Hub) Rooms(roomType string, delta float64) { roomsActive.WithLabelValues(roomType).Add(delta) }

// Players adds delta to the seats of the hosted rooms.
func (Hub) Players(roomType string, delta float64) { roomPlayers.WithLabelValues(roomType).Add(delta) }

// SnapshotBytes counts published snapshot bytes.
func (Hub) SnapshotBytes(roomType string, n int) {
	snapshotBytes.WithLabelValues(roomType).Add(float64(n))
}

// ProjectionLag observes the lag of a projection.
func ProjectionLag(table string, d time.Duration) {
	projectionLag.WithLabelValues(table).Observe(d.Seconds())
}
