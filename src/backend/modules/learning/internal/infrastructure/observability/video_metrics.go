package observability

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// Video lesson metrics (plan F065 "Quan sát"). Labels never carry ids or answers (R14).
var (
	videoProgressTrimmed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_video_progress_trimmed_total",
		Help: "Heartbeats whose watched ranges grew faster than real time × the fastest playback rate (trimmed).",
	})
	videoSeekRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_video_seek_rejected_total",
		Help: "Heartbeats refused with VIDEO_SEEK_NOT_ALLOWED.",
	})
	videoCompletion = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_video_completion_total",
		Help: "Video attempts submitted, by completion of the rule.",
	}, []string{"completed"})
)

// RegisterVideo adds the video metrics to r; already registered collectors are ignored.
func RegisterVideo(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{videoProgressTrimmed, videoSeekRejected, videoCompletion} {
		_ = r.Register(c)
	}
}

// VideoCounter adapts the video metrics to command.VideoMetrics.
type VideoCounter struct{}

// ProgressTrimmed counts one trimmed heartbeat.
func (VideoCounter) ProgressTrimmed() { videoProgressTrimmed.Inc() }

// SeekRejected counts one refused heartbeat.
func (VideoCounter) SeekRejected() { videoSeekRejected.Inc() }

// Completion counts one submitted video attempt.
func (VideoCounter) Completion(completed bool) {
	videoCompletion.WithLabelValues(strconv.FormatBool(completed)).Inc()
}
