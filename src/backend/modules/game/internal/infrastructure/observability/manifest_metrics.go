package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var publishToVisible = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "eduworld_content_publish_to_visible_seconds",
	Help:    "Delay from a learning release (released_at) to the tenant manifest that contains it (publish_to_visible_max, F026).",
	Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 20, 30, 60, 120},
}, []string{"item_type"})

// RegisterManifest adds the manifest metrics to r; already registered collectors are ignored.
func RegisterManifest(r prometheus.Registerer) { _ = r.Register(publishToVisible) }

// ManifestMetrics adapts eduworld_content_publish_to_visible_seconds to command.ManifestMetrics.
type ManifestMetrics struct{}

// PublishToVisible observes one release → manifest delay.
func (ManifestMetrics) PublishToVisible(itemType string, d time.Duration) {
	publishToVisible.WithLabelValues(itemType).Observe(d.Seconds())
}
