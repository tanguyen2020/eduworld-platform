package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// Chat core metrics (F115 T035, plan §Quan sát): no user, channel id or content label (R14).
var (
	chatOnce sync.Once

	messages = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_messages_total",
		Help: "Chat messages by channel type, kind and moderation decision (block included, never stored)."},
		[]string{"channel_type", "kind", "decision"})
	rejected = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_rejected_total",
		Help: "Chat sends refused before the stream by error code."}, []string{"code"})
	sendLatency = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_chat_send_latency_seconds",
		Help: "Send command to JetStream ack (Sent).", Buckets: prometheus.ExponentialBuckets(0.005, 2, 10)})
	publishErrors = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_chat_jetstream_publish_errors_total",
		Help: "JetStream publishes of chat messages without ack (CHAT_TEMPORARILY_UNAVAILABLE)."})
	persistLag = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_chat_persist_lag_seconds",
		Help: "Message created_at to PostgreSQL commit by chat-persister.", Buckets: prometheus.ExponentialBuckets(0.01, 2, 12)})
	persisted = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_persisted_total",
		Help: "chat-persister outcomes per message (inserted, duplicate, orphan, dead_letter, retry)."}, []string{"result"})
	staleFacts = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_chat_stale_facts_total",
		Help: "Cached send-path facts served stale because PostgreSQL could not be read."})
)

// RegisterChat registers the chat core metrics once.
func RegisterChat(reg prometheus.Registerer) {
	chatOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{messages, rejected, sendLatency, publishErrors, persistLag, persisted, staleFacts} {
			_ = reg.Register(c)
		}
	})
}

// Chat implements port.ChatMetrics.
type Chat struct{}

var _ port.ChatMetrics = Chat{}

// Sent implements port.ChatMetrics.
func (Chat) Sent(channelType, kind, decision string) {
	messages.WithLabelValues(channelType, kind, decision).Inc()
}

// Rejected implements port.ChatMetrics.
func (Chat) Rejected(code string) { rejected.WithLabelValues(code).Inc() }

// SendLatency implements port.ChatMetrics.
func (Chat) SendLatency(d time.Duration) { sendLatency.Observe(d.Seconds()) }

// PublishError implements port.ChatMetrics.
func (Chat) PublishError() { publishErrors.Inc() }

// PersistLag observes the persist lag of one message.
func PersistLag(d time.Duration) { persistLag.Observe(d.Seconds()) }

// Persisted counts persister outcomes.
func Persisted(result string, n int) { persisted.WithLabelValues(result).Add(float64(n)) }

// StaleFacts counts a stale cached fact served.
func StaleFacts() { staleFacts.Inc() }
