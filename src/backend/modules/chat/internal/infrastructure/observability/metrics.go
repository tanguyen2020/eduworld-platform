// Package observability holds the metrics of module chat (F095 T026). Labels never carry ids or PII. Projection lag
// carries the module in its name (one registry per binary, OQ F014-6).
package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

var (
	once sync.Once

	friendRequests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_social_friend_requests_total",
		Help: "Friend request outcomes (sent, accepted, rejected, scope_violation, limit_reached, rate_limited)."}, []string{"result"})
	blocks = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_social_blocks_total",
		Help: "Blocks created by students."})
	presenceFanout = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_social_presence_fanout_total",
		Help: "presence.friend.updated messages sent to online friends."})
	projectionLag = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_chat_projection_lag_seconds",
		Help: "Delay between an event and its projection in module chat.", Buckets: prometheus.ExponentialBuckets(0.01, 4, 8)},
		[]string{"table"})
)

// Register registers the metrics once (nil: nothing registered).
func Register(reg prometheus.Registerer) {
	once.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{friendRequests, blocks, presenceFanout, projectionLag} {
			_ = reg.Register(c)
		}
	})
}

// Social implements port.Metrics.
type Social struct{}

var _ port.Metrics = Social{}

// FriendRequest implements port.Metrics.
func (Social) FriendRequest(result string) { friendRequests.WithLabelValues(result).Inc() }

// Block implements port.Metrics.
func (Social) Block() { blocks.Inc() }

// PresenceFanout implements port.Metrics.
func (Social) PresenceFanout(n int) { presenceFanout.Add(float64(n)) }

// ProjectionLag observes the lag of a projection.
func ProjectionLag(table string, d time.Duration) {
	projectionLag.WithLabelValues(table).Observe(d.Seconds())
}
