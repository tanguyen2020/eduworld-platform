// Package observability registers the metrics of the gateway (REALTIME_ARCHITECTURE §12, F090 T037; names and labels of
// eduworld_ws_* and eduworld_players_online owned by MONITORING §3). Labels are bounded: message types come from the
// AsyncAPI, codes from ErrorCode, platform and grade band from closed lists; never a user, tenant or payload.
package observability

import (
	"slices"

	"github.com/prometheus/client_golang/prometheus"
)

// Results of eduworld_ws_connect_total (MONITORING §3): ok, ticket_invalid (missing, malformed, unknown, expired or
// already used — an expired ticket is gone from Valkey and cannot be told apart from an unknown one), rejected (any
// other refusal: draining, Valkey down, subprotocol, Origin, accept rate or capacity, registry unavailable).
const (
	ConnectOK            = "ok"
	ConnectTicketInvalid = "ticket_invalid"
	ConnectRejected      = "rejected"
	labelUnknown         = "unknown"
)

// Platforms and grade bands of eduworld_players_online (GameSessionCreate.client_platform, GradeBand of the contract);
// any other value is counted as "unknown" so a ticket can never create a new series.
var (
	platforms  = []string{"windows", "android", "ios", "web", "macos", "linux"}
	gradeBands = []string{"g1_3", "g4_5", "g6_7", "g8_9"}
)

// Metrics of the gateway.
type Metrics struct {
	Connections      *prometheus.GaugeVec
	Connect          *prometheus.CounterVec
	PlayersOnline    *prometheus.GaugeVec
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
		Connections: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_ws_connections",
			Help: "Open WebSocket connections of the gateway node."}, []string{"node"}),
		Connect: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_ws_connect_total",
			Help: "WebSocket handshakes by result (ok, ticket_invalid, rejected)."}, []string{"result"}),
		PlayersOnline: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_players_online",
			Help: "Students connected to this gateway node (registered and present), by client platform and grade band."},
			[]string{"platform", "grade_band"}),
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
	for _, r := range []string{ConnectOK, ConnectTicketInvalid, ConnectRejected} {
		m.Connect.WithLabelValues(r) // every result exposed from start: the RealtimeConnectFailures ratio needs them
	}
	if reg != nil {
		for _, c := range []prometheus.Collector{m.Connections, m.Connect, m.PlayersOnline, m.Messages, m.QueueDropped, m.CmdLatency,
			m.CmdFailed, m.Resume, m.TicketRejected, m.DeliveryLatency, m.Closed, m.SubscribeRefused, m.Pings} {
			_ = reg.Register(c)
		}
	}
	return m
}

// PlayerLabels returns the bounded (platform, grade_band) of a connection: values outside the closed lists are "unknown".
func PlayerLabels(platform, gradeBand string) (string, string) {
	if !slices.Contains(platforms, platform) {
		platform = labelUnknown
	}
	if !slices.Contains(gradeBands, gradeBand) {
		gradeBand = labelUnknown
	}
	return platform, gradeBand
}

// ConnectResult maps a ticket rejection reason (ticket.Reason*) to the result of eduworld_ws_connect_total: the gateway's
// own failure (Valkey unavailable) is "rejected", every refusal of the ticket itself is "ticket_invalid".
func ConnectResult(ticketReason string) string {
	if ticketReason == "unavailable" {
		return ConnectRejected
	}
	return ConnectTicketInvalid
}
