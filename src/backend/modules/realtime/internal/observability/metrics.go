// Package observability registers the metrics of the gateway (REALTIME_ARCHITECTURE §12, F090 T037). Labels are
// bounded: message types come from the AsyncAPI, codes from ErrorCode; never a user, tenant or payload.
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics of the gateway.
type Metrics struct {
	Connections      *prometheus.GaugeVec
	Messages         *prometheus.CounterVec
	QueueDropped     *prometheus.CounterVec
	CmdLatency       *prometheus.HistogramVec
	CmdFailed        *prometheus.CounterVec
	Resume           *prometheus.CounterVec
	TicketRejected   *prometheus.CounterVec
	DeliveryLatency  prometheus.Histogram
	Closed           *prometheus.CounterVec
	SubscribeRefused prometheus.Counter
	Pings            prometheus.Counter
}

// New creates the metrics and registers them on reg (nil: not registered, tests).
func New(reg prometheus.Registerer, node string) *Metrics {
	m := &Metrics{
		Connections: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_rt_connections",
			Help: "Open WebSocket connections of the gateway node."}, []string{"node"}),
		Messages: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_rt_messages_total",
			Help: "WebSocket messages by direction (in, out) and type."}, []string{"direction", "type"}),
		QueueDropped: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_rt_send_queue_dropped_total",
			Help: "Droppable messages dropped because the send queue was full."}, []string{"type"}),
		CmdLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_rt_cmd_latency_seconds",
			Help: "NATS request/reply latency of commands to core-api.", Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2, 5}},
			[]string{"module", "type"}),
		CmdFailed: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_rt_cmd_failed_total",
			Help: "Commands answered with system.command.failed by module and code."}, []string{"module", "code"}),
		Resume: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_rt_resume_total",
			Help: "Resume attempts by result (resumed, expired, unknown_token, seq_gap)."}, []string{"result"}),
		TicketRejected: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_rt_ticket_rejected_total",
			Help: "Handshakes refused because of the ticket, by reason."}, []string{"reason"}),
		DeliveryLatency: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_rt_message_delivery_latency_seconds",
			Help: "Server ts of a message to its write on the socket (NFR-PERF-003).", Buckets: prometheus.DefBuckets}),
		Closed: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_rt_connections_closed_total",
			Help: "Connections closed by the gateway by close code."}, []string{"code"}),
		SubscribeRefused: prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_rt_subscribe_refused_total",
			Help: "Topics of a module reply refused by the gateway (not a subscribable channel)."}),
		Pings: prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_rt_ws_pings_total",
			Help: "WebSocket ping control frames answered by clients (ws_ping)."}),
	}
	m.Connections.WithLabelValues(node).Set(0)
	if reg != nil {
		for _, c := range []prometheus.Collector{m.Connections, m.Messages, m.QueueDropped, m.CmdLatency, m.CmdFailed, m.Resume,
			m.TicketRejected, m.DeliveryLatency, m.Closed, m.SubscribeRefused, m.Pings} {
			_ = reg.Register(c)
		}
	}
	return m
}
