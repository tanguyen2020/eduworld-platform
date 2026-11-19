package observability

import "github.com/prometheus/client_golang/prometheus"

// Release metrics (plan F026 "Quan sát").
var (
	contentReleases = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_content_releases_total",
		Help: "Release pointer changes of learning content by action (publish, rollback, unpublish).",
	}, []string{"action"})
	snapshotCacheMiss = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_snapshot_cache_miss_total",
		Help: "Snapshot reads served from PostgreSQL because Valkey had no entry or failed.",
	})
)

// RegisterRelease adds the release metrics to r; already registered collectors are ignored.
func RegisterRelease(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{contentReleases, snapshotCacheMiss} {
		_ = r.Register(c)
	}
}

// SnapshotCacheMiss is eduworld_snapshot_cache_miss_total.
func SnapshotCacheMiss() prometheus.Counter { return snapshotCacheMiss }

// ReleaseCounter adapts eduworld_content_releases_total to command.ReleaseMetrics.
type ReleaseCounter struct{}

// Released counts one release of action.
func (ReleaseCounter) Released(action string) { contentReleases.WithLabelValues(action).Inc() }
