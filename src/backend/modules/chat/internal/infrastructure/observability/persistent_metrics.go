package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// F120 metrics (plan.md §Quan sát): no user, no content label (R14).
var (
	retentionAnonymized = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_retention_anonymized_total",
		Help: "Persistent chat messages anonymised by persistent-chat-retention (F120-AC-24)."}, []string{"tenant"})
	policyRejected = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_policy_rejected_total",
		Help: "Chat policy writes refused (F120-AC-07..09: relax, always-on, validation)."}, []string{"code"})
	persistentOnce sync.Once
)

// RegisterPersistent registers the F120 metrics once.
func RegisterPersistent(reg prometheus.Registerer) {
	persistentOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{retentionAnonymized, policyRejected} {
			_ = reg.Register(c)
		}
	})
}

// Persistent implements the F120 metric ports (command.RetentionMetrics, policy rejections).
type Persistent struct{}

// Anonymized counts the anonymised messages of a tenant.
func (Persistent) Anonymized(tenant string, n int) {
	if n > 0 {
		retentionAnonymized.WithLabelValues(tenant).Add(float64(n))
	}
}

// PolicyRejected counts a refused policy write by problem code.
func (Persistent) PolicyRejected(code string) { policyRejected.WithLabelValues(code).Inc() }
