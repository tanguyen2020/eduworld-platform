// Package report holds the measurements of a loadbot run: Prometheus metrics eduworld_loadbot_* on the admin port
// (scraped or remote-written to the staging Prometheus so the F131 "Load test" panels and the perf report read them by
// run_id), and a JSON summary with p50/p95/p99 per operation. Labels are bounded: scenario (one per run), op (a fixed
// list in code), result/status class; never a user, tenant or device id (MONITORING §2, AGENTS R14).
package report

import (
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// reservoir bounds the samples kept per operation for the summary percentiles.
const reservoir = 20000

// Metrics of one run.
type Metrics struct {
	scenario string
	reg      *prometheus.Registry
	logins   *prometheus.CounterVec
	untilIn  prometheus.Observer
	http     *prometheus.CounterVec
	httpDur  *prometheus.HistogramVec
	ws       *prometheus.CounterVec
	wsRTT    *prometheus.HistogramVec
	conns    prometheus.Gauge
	errs     *prometheus.CounterVec

	mu      sync.Mutex
	samples map[string][]float64 // op → seconds
	seen    map[string]int
	counts  map[string]int
}

// New creates the metrics of a run of scenario.
func New(scenario string) *Metrics {
	l := prometheus.Labels{"scenario": scenario}
	buckets := []float64{.005, .01, .025, .05, .1, .2, .25, .3, .4, .5, .8, 1, 2, 3, 5, 10, 30, 60}
	m := &Metrics{scenario: scenario, reg: prometheus.NewRegistry(), samples: map[string][]float64{}, seen: map[string]int{}, counts: map[string]int{}}
	m.logins = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_loadbot_logins_total", Help: "Bot login outcomes (ok/queue_busy/rate_limited/server_error/failed).", ConstLabels: l}, []string{"result"})
	ui := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_loadbot_login_until_in_seconds", Help: "First gameLogin to realtime ticket (login_slo).", ConstLabels: l, Buckets: buckets})
	m.untilIn = ui
	m.http = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_loadbot_http_requests_total", Help: "REST calls of the bots.", ConstLabels: l}, []string{"op", "status_class"})
	m.httpDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_loadbot_http_duration_seconds", Help: "REST latency seen by the bots.", ConstLabels: l, Buckets: buckets}, []string{"op"})
	m.ws = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_loadbot_ws_commands_total", Help: "Realtime commands of the bots.", ConstLabels: l}, []string{"op", "result"})
	m.wsRTT = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_loadbot_ws_command_seconds", Help: "Command to ack round trip.", ConstLabels: l, Buckets: buckets}, []string{"op"})
	m.conns = prometheus.NewGauge(prometheus.GaugeOpts{Name: "eduworld_loadbot_ws_connected", Help: "Open bot WebSockets.", ConstLabels: l})
	m.errs = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_loadbot_errors_total", Help: "Bot errors by kind.", ConstLabels: l}, []string{"kind"})
	m.reg.MustRegister(m.logins, ui, m.http, m.httpDur, m.ws, m.wsRTT, m.conns, m.errs)
	return m
}

// Login records the outcome of one bot login.
func (m *Metrics) Login(result string, untilIn time.Duration) {
	m.logins.WithLabelValues(result).Inc()
	m.count("login_" + result)
	if result == "ok" {
		m.untilIn.Observe(untilIn.Seconds())
		m.sample("login_until_in", untilIn)
	}
}

// LoginRetries records the 429/5xx answers met before a login succeeded or gave up.
func (m *Metrics) LoginRetries(queueBusy, rateLimited, serverErrors int) {
	m.logins.WithLabelValues("queue_busy").Add(float64(queueBusy))
	m.logins.WithLabelValues("rate_limited").Add(float64(rateLimited))
	m.logins.WithLabelValues("server_error").Add(float64(serverErrors))
	m.countN("login_queue_busy", queueBusy)
	m.countN("login_rate_limited", rateLimited)
	m.countN("login_server_error", serverErrors)
}

// HTTP records one REST call.
func (m *Metrics) HTTP(op string, status int, d time.Duration) {
	m.http.WithLabelValues(op, StatusClass(status)).Inc()
	m.httpDur.WithLabelValues(op).Observe(d.Seconds())
	m.sample("http_"+op, d)
	m.count("http_" + op + "_" + StatusClass(status))
}

// WS records one realtime command (err nil = acked).
func (m *Metrics) WS(op string, d time.Duration, err error) {
	res := "acked"
	if err != nil {
		res = "failed"
	}
	m.ws.WithLabelValues(op, res).Inc()
	m.count("ws_" + op + "_" + res)
	if err == nil {
		m.wsRTT.WithLabelValues(op).Observe(d.Seconds())
		m.sample("ws_"+op, d)
	}
}

// Sent counts a fire-and-forget realtime message (hub moves are not acked).
func (m *Metrics) Sent(op string) {
	m.ws.WithLabelValues(op, "sent").Inc()
	m.count("ws_" + op + "_sent")
}

// Connected moves the open WebSocket gauge.
func (m *Metrics) Connected(delta float64) { m.conns.Add(delta) }

// Error counts an error of kind.
func (m *Metrics) Error(kind string) {
	m.errs.WithLabelValues(kind).Inc()
	m.count("error_" + kind)
}

// Handler serves /metrics.
func (m *Metrics) Handler() http.Handler { return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}) }

func (m *Metrics) count(k string) { m.countN(k, 1) }

func (m *Metrics) countN(k string, n int) {
	if n == 0 {
		return
	}
	m.mu.Lock()
	m.counts[k] += n
	m.mu.Unlock()
}

func (m *Metrics) sample(op string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen[op]++
	s := m.samples[op]
	if len(s) < reservoir {
		m.samples[op] = append(s, d.Seconds())
		return
	}
	if j := rand.IntN(m.seen[op]); j < reservoir { // #nosec G404 -- reservoir sampling
		s[j] = d.Seconds()
	}
}

// Quantiles of one operation, in milliseconds.
type Quantiles struct {
	Count int     `json:"count"`
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	P99   float64 `json:"p99_ms"`
}

// Summary of a run.
type Summary struct {
	Scenario  string               `json:"scenario"`
	Started   time.Time            `json:"started"`
	Ended     time.Time            `json:"ended"`
	Counts    map[string]int       `json:"counts"`
	Latencies map[string]Quantiles `json:"latencies"`
}

// Summary computes the summary now.
func (m *Metrics) Summary(started, ended time.Time) Summary {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Summary{Scenario: m.scenario, Started: started, Ended: ended, Counts: map[string]int{}, Latencies: map[string]Quantiles{}}
	for k, v := range m.counts {
		s.Counts[k] = v
	}
	for op, v := range m.samples {
		c := append([]float64(nil), v...)
		sort.Float64s(c)
		s.Latencies[op] = Quantiles{Count: m.seen[op], P50: q(c, .50), P95: q(c, .95), P99: q(c, .99)}
	}
	return s
}

// WriteJSON writes the summary.
func (s Summary) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

func q(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(p*float64(len(sorted)-1) + 0.5)
	return float64(int(sorted[i]*1e6)) / 1e3 // ms, µs precision
}

// StatusClass maps a status to 2xx/3xx/4xx/5xx (0 = transport error → "error").
func StatusClass(status int) string {
	if status < 100 {
		return "error"
	}
	return strconv.Itoa(status/100) + "xx"
}
