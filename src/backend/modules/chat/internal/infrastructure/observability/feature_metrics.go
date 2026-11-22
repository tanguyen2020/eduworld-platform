package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Message feature metrics (F118 T034, plan §Quan sát): no user, channel id or content label (R14).
var (
	featuresOnce sync.Once

	featureEvents = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_events_total",
		Help: "Message changes published to the durable stream by event (edit, edit_held, recall, reaction_set/unset, pin/unpin, forward)."},
		[]string{"event"})
	typingEvents = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_chat_typing_events_total",
		Help: "chat.typing.updated broadcasts (after the 3 s throttle)."})
	assetRejected = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_asset_rejected_total",
		Help: "Sticker/GIF uploads refused by the file checks of Game Studio, by reason."}, []string{"reason"})
)

// RegisterFeatures registers the message feature metrics once.
func RegisterFeatures(reg prometheus.Registerer) {
	featuresOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{featureEvents, typingEvents, assetRejected} {
			_ = reg.Register(c)
		}
	})
}

// Features implements command.FeatureMetrics.
type Features struct{}

// Event counts one published change.
func (Features) Event(event string) { featureEvents.WithLabelValues(event).Inc() }

// Typing counts one typing broadcast.
func (Features) Typing() { typingEvents.Inc() }

// AssetRejected counts one refused upload (reason: format, size, dimensions, preview, decode).
func (Features) AssetRejected(reason string) { assetRejected.WithLabelValues(reason).Inc() }
