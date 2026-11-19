package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Media pipeline and formula metrics (plan F027 "Quan sát", MONITORING).
var (
	mediaPipeline = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_media_pipeline_duration_seconds",
		Help:    "Duration of the media worker step (scan, magic bytes, variants) by kind and result (ready, rejected).",
		Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 30, 60, 120, 300, 900},
	}, []string{"kind", "result"})
	mediaRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_media_rejected_total",
		Help: "Media assets rejected by reason (virus, type_mismatch, too_large, transcode_failed).",
	}, []string{"reason"})
	mathFailed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_math_render_failed_total",
		Help: "Formulas refused with MATH_RENDER_FAILED by reason (latex, svg, svg_missing).",
	}, []string{"reason"})
)

// RegisterMedia adds the media metrics to r (composition root); already registered collectors are ignored.
func RegisterMedia(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{mediaPipeline, mediaRejected, mathFailed} {
		_ = r.Register(c)
	}
}

// MediaPipeline is eduworld_media_pipeline_duration_seconds{kind,result}.
func MediaPipeline() *prometheus.HistogramVec { return mediaPipeline }

// MediaRejected is eduworld_media_rejected_total{reason}.
func MediaRejected() *prometheus.CounterVec { return mediaRejected }

// MathFailed is eduworld_math_render_failed_total{reason}.
func MathFailed() *prometheus.CounterVec { return mathFailed }

// MediaCounter adapts the media metrics to command.MediaMetrics and command.MathMetrics.
type MediaCounter struct{}

// Pipeline observes one worker step.
func (MediaCounter) Pipeline(kind, result string, d time.Duration) {
	mediaPipeline.WithLabelValues(kind, result).Observe(d.Seconds())
}

// Rejected counts a rejected asset.
func (MediaCounter) Rejected(reason string) { mediaRejected.WithLabelValues(reason).Inc() }

// MathFailed counts a refused formula.
func (MediaCounter) MathFailed(reason string) { mathFailed.WithLabelValues(reason).Inc() }
