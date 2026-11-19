// Package observability holds the metrics of module learning (plan F020 "Quan sát", MONITORING): consumer
// rejections, projection lag and assignment operations. The bypass counter
// eduworld_tenant_filter_bypass_total{module,reason} belongs to pkg/tenancy.
package observability

import "github.com/prometheus/client_golang/prometheus"

var (
	consumerRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_consumer_rejected_total",
		Help: "Events refused by a consumer of module learning (sent to DLQ_LEARNING) by reason.",
	}, []string{"consumer", "reason"})
	projectionLag = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_projection_lag_seconds",
		Help:    "Delay between an identity event and its projection into ref.* of eduworld_learning.",
		Buckets: []float64{.05, .1, .25, .5, 1, 2, 5, 10, 30, 60, 300},
	}, []string{"table"})
	assignments = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_tenant_assignments_total",
		Help: "Tenant resource assignments created (apply) or revoked (revoke) by module learning.",
	}, []string{"op"})
)

// ConsumerRejected is eduworld_consumer_rejected_total{consumer,reason}.
func ConsumerRejected() *prometheus.CounterVec { return consumerRejected }

// ProjectionLag is eduworld_projection_lag_seconds{table}.
func ProjectionLag() *prometheus.HistogramVec { return projectionLag }

// Assignments is eduworld_tenant_assignments_total{op}.
func Assignments() *prometheus.CounterVec { return assignments }

// Register adds the metrics to r (composition root); already registered collectors are ignored.
func Register(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{consumerRejected, projectionLag, assignments} {
		_ = r.Register(c)
	}
}

// AssignmentCounter adapts eduworld_tenant_assignments_total to command.AssignmentMetrics.
type AssignmentCounter struct{}

// Add counts n assignments of op (apply, revoke).
func (AssignmentCounter) Add(op string, n int) {
	if n > 0 {
		assignments.WithLabelValues(op).Add(float64(n))
	}
}

// Curriculum metrics (plan F021 "Quan sát").
var (
	curriculumPublish = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_curriculum_publish_total",
		Help: "Curriculum version publishes by result (success, rejected, error).",
	}, []string{"result"})
	importJobs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_import_jobs_total",
		Help: "Content import jobs finished by kind and status.",
	}, []string{"kind", "status"})
	importRowsRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_import_rows_rejected_total",
		Help: "Rows rejected by content imports by kind.",
	}, []string{"kind"})
)

// RegisterCurriculum adds the curriculum metrics to r; already registered collectors are ignored.
func RegisterCurriculum(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{curriculumPublish, importJobs, importRowsRejected} {
		_ = r.Register(c)
	}
}

// CurriculumCounter adapts the curriculum metrics to command.CurriculumMetrics.
type CurriculumCounter struct{}

// Published counts a publish of result (success, rejected, error).
func (CurriculumCounter) Published(result string) { curriculumPublish.WithLabelValues(result).Inc() }

// ImportFinished counts a finished import job and its rejected rows.
func (CurriculumCounter) ImportFinished(kind, status string, rejected int) {
	importJobs.WithLabelValues(kind, status).Inc()
	if rejected > 0 {
		importRowsRejected.WithLabelValues(kind).Add(float64(rejected))
	}
}

// CurriculumPublishTotal is eduworld_curriculum_publish_total{result} (tests).
func CurriculumPublishTotal() *prometheus.CounterVec { return curriculumPublish }

// ImportJobsTotal is eduworld_import_jobs_total{kind,status} (tests).
func ImportJobsTotal() *prometheus.CounterVec { return importJobs }

// Question bank metrics (plan F022 "Quan sát").
var (
	qbGrade = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_qb_grade_total",
		Help: "Server-side grades of questions by type and result (correct, incorrect, invalid_response).",
	}, []string{"type", "result"})
	qbPoolInsufficient = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_qb_select_pool_insufficient_total",
		Help: "Question selections refused with QUESTION_POOL_INSUFFICIENT.",
	})
	qbPoolCache = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_qb_pool_cache_requests_total",
		Help: "Question pool cache lookups by outcome (hit, miss); the hit ratio is hit / total.",
	}, []string{"outcome"})
)

// RegisterQuestionBank adds the question bank metrics to r; already registered collectors are ignored.
func RegisterQuestionBank(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{qbGrade, qbPoolInsufficient, qbPoolCache} {
		_ = r.Register(c)
	}
}

// QuestionCounter adapts the question bank metrics to query.DeliveryMetrics.
type QuestionCounter struct{}

// Graded counts a grade of questionType with result.
func (QuestionCounter) Graded(questionType, result string) {
	qbGrade.WithLabelValues(questionType, result).Inc()
}

// PoolInsufficient counts a refused selection.
func (QuestionCounter) PoolInsufficient() { qbPoolInsufficient.Inc() }

// PoolCache counts a pool cache lookup.
func (QuestionCounter) PoolCache(hit bool) {
	outcome := "miss"
	if hit {
		outcome = "hit"
	}
	qbPoolCache.WithLabelValues(outcome).Inc()
}

// QBGradeTotal is eduworld_qb_grade_total{type,result} (tests).
func QBGradeTotal() *prometheus.CounterVec { return qbGrade }
