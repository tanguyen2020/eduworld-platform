package observability

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// schoolKitFetch is eduworld_school_kit_index_fetch_total{result} (F134, MONITORING: alert SchoolKitIndexInvalid on
// result="invalid"): ok, invalid (not loadable, bad signature or schema), unconfigured.
var schoolKitFetch = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "eduworld_school_kit_index_fetch_total", Help: "Loads of the signed school-kit index by result (F134).",
}, []string{"result"})

// RegisterInstaller registers the F134 installer metric.
func RegisterInstaller(r prometheus.Registerer) { _ = r.Register(schoolKitFetch) }

// InstallerMetrics implements port.InstallerMetrics.
type InstallerMetrics struct{}

var _ port.InstallerMetrics = InstallerMetrics{}

// IndexFetch implements port.InstallerMetrics.
func (InstallerMetrics) IndexFetch(result string) { schoolKitFetch.WithLabelValues(result).Inc() }
