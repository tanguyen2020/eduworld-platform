package observability

import "github.com/prometheus/client_golang/prometheus"

var (
	translationJobs = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_game_translation_jobs_total",
		Help: "Translation jobs of game content by kind (export, import) and status (queued, succeeded, failed) — F082."}, []string{"kind", "status"})
	translationRows = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_game_translation_rows_total",
		Help: "Lines of translation files by result (updated, skipped) — F082."}, []string{"result"})
)

// RegisterTranslations registers the translation metrics (F082 T034).
func RegisterTranslations(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{translationJobs, translationRows} {
		_ = r.Register(c)
	}
}

// TranslationMetrics implements command.TranslationMetrics.
type TranslationMetrics struct{}

// Job counts a job reaching status.
func (TranslationMetrics) Job(kind, status string) {
	translationJobs.WithLabelValues(kind, status).Inc()
}

// Rows counts n lines of result.
func (TranslationMetrics) Rows(result string, n int) {
	if n > 0 {
		translationRows.WithLabelValues(result).Add(float64(n))
	}
}
