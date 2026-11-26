package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// In-game notification metrics (F123 T030, plan §Quan sát): by kind only, never a user or notification id (R14).
// eduworld_consumer_pending{stream,consumer} of the chat.notify_* consumers comes from pkg/outbox.
var (
	notificationOnce sync.Once

	notificationsCreated = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_notifications_created_total",
		Help: "In-game notifications stored, by kind (consumers chat.notify_*)."}, []string{"kind"})
	notificationsRead = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_notifications_read_total",
		Help: "In-game notifications marked read (REST or WS)."})
	notificationLatency = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_notification_delivery_latency_seconds",
		Help:    "Time from the source event to the push of notification.item.created (SC-001: p95 ≤ 2 s).",
		Buckets: []float64{.05, .1, .25, .5, 1, 2, 5, 10, 30, 60}})
)

// RegisterNotifications registers the notification metrics once.
func RegisterNotifications(reg prometheus.Registerer) {
	notificationOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{notificationsCreated, notificationsRead, notificationLatency} {
			_ = reg.Register(c)
		}
	})
}

// Notifications implements port.NotificationMetrics.
type Notifications struct{}

var _ port.NotificationMetrics = Notifications{}

// Created counts n notifications of kind.
func (Notifications) Created(kind string, n int) {
	notificationsCreated.WithLabelValues(kind).Add(float64(n))
}

// Read counts n notifications marked read.
func (Notifications) Read(n int) { notificationsRead.Add(float64(n)) }

// Delivered observes the latency from the source event to the push.
func (Notifications) Delivered(d time.Duration) { notificationLatency.Observe(max(d, 0).Seconds()) }
