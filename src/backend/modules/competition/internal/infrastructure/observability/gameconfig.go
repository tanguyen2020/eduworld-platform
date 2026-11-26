package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics of the competition configurations (F108 plan §Quan sát): labelled by config_type only.
var (
	cfgOnce = sync.Once{}

	cfgPublished = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_competition_config_publish_total",
		Help: "Competition configuration versions published, by config type."}, []string{"config_type"})
	cfgInvalid = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_competition_config_validation_failed_total",
		Help: "Competition configuration payloads refused by their schema or a competition rule, by config type."}, []string{"config_type"})
)

// RegisterGameConfig registers the configuration metrics once.
func RegisterGameConfig(reg prometheus.Registerer) {
	cfgOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{cfgPublished, cfgInvalid} {
			_ = reg.Register(c)
		}
	})
}

// GameConfig implements command.GameConfigMetrics.
type GameConfig struct{}

// Published counts a published version.
func (GameConfig) Published(configType string) { cfgPublished.WithLabelValues(configType).Inc() }

// ValidationFailed counts a refused payload.
func (GameConfig) ValidationFailed(configType string) { cfgInvalid.WithLabelValues(configType).Inc() }
