// Package learning is the composition entry of module learning (ADR-0074 §1, BACKEND_DESIGN §3.3).
// cmd/<binary> may only call Register, Facade and Wire; everything else lives in internal/.
package learning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/simulation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/examquality"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/readiness"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/recommendation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/reporting"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/export"
	learninghealth "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/i18n"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/importer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/mediaproc"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/qbschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/secret"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/translation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	learninghttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/runtime" // ent mixin hooks and interceptors (R27)
)

// Name is the module name used for its database (eduworld_learning), env prefix and checks.
const Name = "learning"

// Intervals of the learning jobs [Proposed] (OQ F020-3): queued imports and translation jobs are picked up within
// seconds of their creation.
const (
	ImportJobInterval      = 30 * time.Second
	TranslationJobInterval = 30 * time.Second
	// ScheduledPublishInterval: F026 T036 applies due scheduled publications every minute.
	ScheduledPublishInterval = time.Minute
	// AttemptLifecycleInterval: F030 T003/T035 pauses idle attempts, abandons and expires due attempts every minute.
	AttemptLifecycleInterval = time.Minute
	// LearningPartitionsInterval: F031 T016 keeps the monthly partitions of the learning tables ahead once a day.
	LearningPartitionsInterval = 24 * time.Hour
	// RegradeRunnerInterval: F031 T038 picks the queued re-evaluations.
	RegradeRunnerInterval = 30 * time.Second
	// RecommendationNightlyInterval: F032 T051 recomputes the recommendations once a day (decay R4, warm-up R7).
	RecommendationNightlyInterval = 24 * time.Hour
	// AssignmentSchedulerInterval: F041 T028 opens and closes the school assignments every minute.
	AssignmentSchedulerInterval = time.Minute
	// ExamWindowInterval: F042 T027 opens and closes the exam windows every 15 seconds; ExamAutoSubmitInterval is
	// exam_autosubmit_interval (15 s [Confirmed], EXAM_ENGINE §7.2).
	ExamWindowInterval     = 15 * time.Second
	ExamAutoSubmitInterval = 15 * time.Second
	// DashboardDayCloseInterval: F043 T015 checks every 15 minutes for days to close (a day closes once
	// dashboard_day_close_time 02:30 ICT has passed) and for days marked by late events [Proposed].
	DashboardDayCloseInterval = 15 * time.Minute
	// F044: the export queue is polled every 10 seconds; item analysis and reference levels run nightly, the calibration
	// proposals weekly (QUESTION_BANK §9, §9.1; ASSESSMENT_REGULATION_PROFILE §4) [Proposed intervals].
	ReportJobsInterval      = 10 * time.Second
	ItemAnalysisInterval    = 24 * time.Hour
	CalibrationInterval     = 7 * 24 * time.Hour
	ReferenceLevelsInterval = 24 * time.Hour
	// F035: the efficacy waves open and close every minute (plan: job efficacy-wave-monitor).
	EfficacyWaveInterval = time.Minute
	// F035: the queued efficacy reports are computed within 30 seconds [Proposed].
	EfficacyReportsInterval = 30 * time.Second
)

// DashboardPoolSize is dashboard_db_pool_per_api_replica (CONFIG_DEFAULTS, DASHBOARD_DATA §7).
const DashboardPoolSize = 10

// Facade is the part of the learning facade (api.StrictServerInterface, specs/openapi/modules/learning.internal.v1.yaml)
// implemented in this release: the platform-ops operations of F020, the student lifecycle steps of F016 and the
// attempt operations of F030 US5 (createAttempt, getAttemptResult; caller module game, wired by F072) and the merge
// step mergeUserData (F030 T056). The question-plan operations (createQuestionPlan, scorePlanAnswer) belong to later
// features.
type Facade interface {
	ApplyTenantAssignments(ctx context.Context, req api.ApplyTenantAssignmentsRequestObject) (api.ApplyTenantAssignmentsResponseObject, error)
	RevokeTenantAssignments(ctx context.Context, req api.RevokeTenantAssignmentsRequestObject) (api.RevokeTenantAssignmentsResponseObject, error)
	ListResourceAssignments(ctx context.Context, req api.ListResourceAssignmentsRequestObject) (api.ListResourceAssignmentsResponseObject, error)
	ExecuteTenantCloneStep(ctx context.Context, req api.ExecuteTenantCloneStepRequestObject) (api.ExecuteTenantCloneStepResponseObject, error)
	ListTrash(ctx context.Context, req api.ListTrashRequestObject) (api.ListTrashResponseObject, error)
	RestoreTrashItem(ctx context.Context, req api.RestoreTrashItemRequestObject) (api.RestoreTrashItemResponseObject, error)
	CleanupStudentData(ctx context.Context, req api.CleanupStudentDataRequestObject) (api.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req api.RestoreStudentDataRequestObject) (api.RestoreStudentDataResponseObject, error)
	AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error)
	CreateAttempt(ctx context.Context, req api.CreateAttemptRequestObject) (api.CreateAttemptResponseObject, error)
	GetAttemptResult(ctx context.Context, req api.GetAttemptResultRequestObject) (api.GetAttemptResultResponseObject, error)
	MergeUserData(ctx context.Context, req api.MergeUserDataRequestObject) (api.MergeUserDataResponseObject, error)
}

var _ Facade = facade.Facade{}

// Imports are the facades module learning calls: none (identity data arrives as ref.* projections, ADR-0037).
type Imports struct{}

// Module is the registered module.
type Module struct {
	facade Facade
}

// Facade returns the in-process facade of module learning (nil in a binary that does not load its use cases).
func (m *Module) Facade() Facade { return m.facade }

// Wire connects the facades of other modules (none for module learning).
func (m *Module) Wire(Imports) {}

// Register wires the module into the running binary (OQ F020-3, F010-2): health checks everywhere (plus the Activity
// Registry readiness of F030 where the use cases load); the CMS and attempt routes (F021–F027, F030) in core-api; the
// outbox relay and the consumers projection_identity, qb_pool_invalidation, media_worker and release_pointer_cache in
// worker events; the import, translation, scheduled-publish and attempt-lifecycle jobs in worker jobs.
func Register(r *module.Registry) (*Module, error) {
	pool, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("learning: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(pool))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(pool, migrations.Latest()))
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() {
		return &Module{}, nil
	}
	w, err := build(r)
	if err != nil {
		return nil, fmt.Errorf("learning: %w", err)
	}
	// F030-AC-23: not ready while published content has an activity type without plugin (platform role: every tenant).
	r.Health().Register("activity_registry:"+Name, health.Readiness,
		learninghealth.RegistryCheck(w.activities, learninghealth.SQLPublishedTypes(learninghealth.FromSQL(w.core.Platform))))
	// F066 research R2: not ready while a template declares a checker that is not registered.
	r.Health().Register("simulation_checkers:"+Name, health.Readiness, func(context.Context) error { return simulation.Ready() })
	r.HTTP(Name, func(s *httpx.Server) error { return learninghttp.Register(s, w.http) })
	if err := r.Relay(Name); err != nil {
		return nil, fmt.Errorf("learning: %w", err)
	}
	core := w.core
	r.Consumer("learning.projection_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.IdentityProjectionOptions(consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}},
			core.UoW, core.App, js)
	})
	r.Consumer("learning.qb_pool_invalidation", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.QuestionPoolOptions(consumer.QuestionPoolInvalidation{Cache: cache.Invalidator{C: w.valkey}}, core.UoW, core.App, js)
	})
	r.Consumer("learning.media_worker", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MediaWorkerOptions(consumer.MediaWorker{Process: w.media}, core.UoW, core.App, js)
	})
	r.Consumer("learning.release_pointer_cache", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ReleasePointerOptions(consumer.ReleasePointerCache{Cache: cache.SnapshotCache{C: w.valkey}}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.MasteryName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MasteryOptions(consumer.MasteryOnGraded{Apply: w.mastery}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.LessonCompletionName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.LessonCompletionOptions(consumer.LessonCompletion{Lessons: w.lessons}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.LMSCoursesName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.LMSCoursesOptions(consumer.LMSCourses{Courses: w.lms}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.AssignmentSubmissionsName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.AssignmentSubmissionsOptions(consumer.AssignmentSubmissions{Tracker: w.assignments}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.AssignmentEnrollmentName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.AssignmentEnrollmentOptions(consumer.AssignmentEnrollment{Tracker: w.assignments}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.ExamGradeName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ExamGradeOptions(consumer.ExamGrade{Grader: w.exams}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.LMSScheduleName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.LMSScheduleOptions(consumer.LMSSchedule{Schedule: w.lms}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.ProgressDailyName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ProgressDailyOptions(consumer.ProgressDaily{Record: w.progress}, core.UoW, core.App, js)
	})
	r.Job(Name, "import-jobs", ImportJobInterval, job.ImportWorker{Runner: w.curriculum}.RunOnce)
	r.Job(Name, "translation-jobs", TranslationJobInterval, job.TranslationWorker{Runner: w.translations}.RunOnce)
	r.Job(Name, "content-scheduled-publish", ScheduledPublishInterval, job.ScheduleWorker{Runner: w.releases}.RunOnce)
	r.Job(Name, "attempt-lifecycle", AttemptLifecycleInterval, job.AttemptLifecycle{Runner: w.attempts}.Count)
	// The partition job runs as learning_partition_admin (EXECUTE on the partition functions only, F031 migration); the
	// pool is opened where the job runs.
	partitionDB := core.Platform
	if r.RunsJobs() {
		if partitionDB, err = r.SQL(Name, "partition_admin"); err != nil {
			return nil, fmt.Errorf("learning: %w", err)
		}
	}
	parts := command.LearningPartitions{Maint: write.PartitionMaint{DB: partitionDB}, Metrics: observability.MasteryCounter{}}
	r.Job(Name, "regrade-runner", RegradeRunnerInterval, w.regrades.RunOnce)
	r.Job(Name, "learning-events-partitions", LearningPartitionsInterval, job.LearningEventsPartitions{Runner: parts}.RunOnce)
	r.Job(Name, "recommendation-nightly", RecommendationNightlyInterval, job.RecommendationNightly{Runner: w.recommendations}.RunOnce)
	r.Job(Name, "assignment-scheduler", AssignmentSchedulerInterval, job.AssignmentScheduler{Runner: w.assignments}.RunOnce)
	r.Job(Name, "exam-window", ExamWindowInterval, job.ExamWindow{Runner: w.exams}.RunOnce)
	r.Job(Name, "exam-autosubmit", ExamAutoSubmitInterval, job.ExamAutoSubmit{Runner: w.exams}.RunOnce)
	// F043: rollup day close and read-model refresh (school_dashboard_refresh).
	r.Job(Name, "dashboard-day-close", DashboardDayCloseInterval, job.DashboardDayClose{Runner: w.dashboards}.RunOnce)
	r.Job(Name, "read-model-refresh", w.dashboards.Config.Refresh, job.ReadModelRefresh{Runner: w.dashboards}.RunOnce)
	// F044: export queue, item analysis, calibration proposals, reference levels; exam quality on exam.closed.
	r.Job(Name, "report-jobs", ReportJobsInterval, job.ReportJobs{Runner: w.reports}.RunOnce)
	r.Job(Name, "item-analysis-nightly", ItemAnalysisInterval, job.ItemAnalysisNightly{Runner: w.quality}.RunOnce)
	r.Job(Name, "calibration-weekly", CalibrationInterval, job.CalibrationWeekly{Runner: w.quality}.RunOnce)
	r.Job(Name, "reference-levels-nightly", ReferenceLevelsInterval, job.ReferenceLevelsNightly{Runner: w.quality}.RunOnce)
	// F035: efficacy waves (participant and mastery snapshots).
	r.Job(Name, "efficacy-wave-monitor", EfficacyWaveInterval, job.EfficacyWaveMonitor{Runner: w.efficacy}.RunOnce)
	r.Job(Name, "efficacy-reports", EfficacyReportsInterval, job.EfficacyReports{Runner: w.efficacy}.RunOnce)
	r.Consumer(messaging.ExamQualityName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ExamQualityOptions(consumer.ExamQuality{Runner: w.quality}, core.UoW, core.App, js)
	})
	return &Module{facade: w.facade}, nil
}

type wiring struct {
	core         *module.Core
	valkey       valkey.Client
	http         learninghttp.Handlers
	facade       facade.Facade
	curriculum   command.Curriculum
	translations command.Translations
	media        command.Media
	releases     command.Releases
	attempts     command.Attempts
	activities   *activity.Registry
	mastery      command.Mastery
	progress     command.Progress
	regrades     command.Regrades
	lessons      command.Lessons
	// F032: the rule engine of the recommendations (R1/R5/R6 are registered here by F041/F042/F040).
	recommendations command.Recommendations
	// F040: the school LMS (courses, paths, schedule, study groups).
	lms         command.LMS
	assignments command.Assignments
	exams       command.Exams
	// F043: the dashboard rollup jobs.
	dashboards command.Dashboards
	// F044: exports and quality jobs.
	reports command.Reports
	quality command.Quality
	// F035: the efficacy study.
	efficacy command.Efficacy
}

// build constructs the use cases of module learning on the real adapters: PostgreSQL (UnitOfWork/ReadSession),
// Valkey (pool and preview caches), S3-compatible object storage, the CDN signer and clamd (EW_MEDIA_SCAN).
func build(r *module.Registry) (*wiring, error) {
	core, err := r.Core(Name)
	if err != nil {
		return nil, err
	}
	cfg := r.Config()
	cursors, err := r.Cursors()
	if err != nil {
		return nil, err
	}
	vk, err := r.Valkey()
	if err != nil {
		return nil, err
	}
	if err := cfg.Require("EW_S3_ENDPOINT", "EW_S3_ACCESS_KEY", "EW_S3_SECRET_KEY", "EW_CDN_BASE_URL", "EW_CDN_SIGNING_KEY", "EW_MEDIA_SCAN"); err != nil {
		return nil, err
	}
	s3, err := storage.New(storage.Config{Endpoint: cfg.String("EW_S3_ENDPOINT"), Region: cfg.String("EW_S3_REGION"),
		AccessKey: cfg.String("EW_S3_ACCESS_KEY"), SecretKey: cfg.String("EW_S3_SECRET_KEY"),
		Timeout: cfg.Duration("EW_S3_TIMEOUT", 0)}, nil)
	if err != nil {
		return nil, err
	}
	scan := cfg.String("EW_MEDIA_SCAN")
	scanDisabled := scan == "disabled"
	if !scanDisabled && !strings.HasPrefix(scan, "clamd://") {
		return nil, fmt.Errorf("EW_MEDIA_SCAN must be clamd://host:port or disabled")
	}
	reg := r.Metrics()
	observability.Register(reg)
	observability.RegisterCurriculum(reg)
	observability.RegisterQuestionBank(reg)
	observability.RegisterAdvanced(reg)
	observability.RegisterMedia(reg)
	observability.RegisterContent(reg)
	observability.RegisterWorkflow(reg)
	observability.RegisterRelease(reg)
	observability.RegisterAttempts(reg)
	observability.RegisterVideo(reg)
	observability.RegisterSimulation(reg)
	observability.RegisterMastery(reg)
	observability.RegisterRegrade(reg)
	observability.RegisterLessons(reg)
	observability.RegisterSelfStudy(reg)
	observability.RegisterLMS(reg)
	observability.RegisterAssignments(reg)
	observability.RegisterExams(reg)
	observability.RegisterDashboards(reg)
	observability.RegisterReporting(reg)
	observability.RegisterEfficacy(reg)

	resources, err := port.NewRegistry(append([]port.ResourceHandler{
		command.CurriculumResource{Store: write.CurriculumRepo{}, Trash: write.CurriculumTrash{}},
		command.QuestionResource{Store: write.QuestionRepo{}}}, command.DocResources(write.DocRepo{}, write.QuestionRepo{})...)...)
	if err != nil {
		return nil, err
	}
	schema, err := qbschema.New()
	if err != nil {
		return nil, err
	}
	docSchema, err := qbschema.NewDocs()
	if err != nil {
		return nil, err
	}
	events, auditor := write.Publisher{W: core.Writer}, write.Auditor{R: core.Recorder}
	signer := storage.CdnSigner{BaseURL: cfg.String("EW_CDN_BASE_URL"), Key: cfg.String("EW_CDN_SIGNING_KEY"), TTL: command.MediaSignedURLTTL}
	assets := storage.AssetReader{Assets: write.MediaRepo{}, Storage: s3}
	files := storage.ExportBucket{S3: s3}
	media := command.Media{UoW: core.UoW, Store: write.MediaRepo{}, Storage: s3, Signer: signer,
		Processor: mediaproc.New(strings.TrimPrefix(scan, "clamd://")), Events: events, Audit: auditor, ScanDisabled: scanDisabled,
		WorkDir: cfg.StringOr("EW_MEDIA_WORK_DIR", os.TempDir()), Metrics: observability.MediaCounter{}}
	math := &command.MathAssets{Store: write.MathRepo{}, Storage: s3, Metrics: observability.MediaCounter{}}
	registry := question.Full()      // F023 Done: the 11 question types (OQ F023-2 [Proposed])
	activities := plugins.Registry() // F030: quiz, interactive_exercise; F065: video_lesson
	activitySchema, err := qbschema.NewActivities()
	if err != nil {
		return nil, err
	}

	cur := command.Curriculum{UoW: core.UoW, Store: write.CurriculumRepo{}, Catalog: write.CatalogRepo{}, Imports: write.ImportJobRepo{},
		Resources: resources, Events: events, Audit: auditor, Assets: assets, Codec: importer.Codec{}, Metrics: observability.CurriculumCounter{}}
	qcmd := command.Questions{UoW: core.UoW, Store: write.QuestionRepo{}, Curriculum: write.CurriculumRepo{}, Catalog: write.CatalogRepo{},
		Schema: schema, Registry: registry, Resources: resources, Events: events, Audit: auditor, Math: math, Media: write.MediaRepo{},
		Docs: write.DocRepo{}}
	dcmd := command.Docs{UoW: core.UoW, Store: write.DocRepo{}, Questions: write.QuestionRepo{}, Curriculum: write.CurriculumRepo{},
		Catalog: write.CatalogRepo{}, Schema: docSchema, Media: write.MediaRepo{}, Read: core.ReadSession, Resources: resources,
		Events: events, Audit: auditor, Math: math, Activities: activities, ActivitySchema: activitySchema, ActivityEnabled: true}
	wf := command.Workflow{UoW: core.UoW, Status: write.WorkflowRepo{}, Reviews: write.WorkflowRepo{}, Questions: write.QuestionRepo{},
		Docs: write.DocRepo{}, Schema: schema, Media: write.MediaRepo{}, Guard: readiness.Default(), Events: events, Audit: auditor,
		Metrics: observability.WorkflowCounter{}}
	tr := command.Translations{UoW: core.UoW, Jobs: write.TranslationJobRepo{}, Workflow: wf, Questions: qcmd, Docs: dcmd,
		Codec: translation.Codec{}, Files: files, Assets: assets, Audit: auditor}
	cur.Content = &command.ContentImport{Questions: qcmd, Codec: importer.QuestionCodec{}}
	snapshots := cache.SnapshotCache{C: vk}
	rel := command.Releases{UoW: core.UoW, Workflow: wf, Store: write.ReleaseRepo{}, Schedules: write.ReleaseRepo{}, Events: events,
		Audit: auditor, Metrics: observability.ReleaseCounter{}, Cache: snapshots, CDN: s3, Signer: signer,
		Activities: activities, ActivitySchema: activitySchema}
	reader := query.ContentReader{Questions: read.QuestionModel{}, Docs: read.DocModel{}, Cache: snapshots}
	docQueries := query.Docs{Session: core.ReadSession, Model: read.DocModel{}, Pool: read.Delivery{}, Cursors: cursors}
	delivery := query.Delivery{Session: core.ReadSession, Reader: read.Delivery{}, Cache: cache.Pool{C: vk}, Registry: registry, Schema: schema,
		Metrics: observability.QuestionCounter{}}
	att := command.Attempts{UoW: core.UoW, Store: write.AttemptRepo{}, Learning: write.LearningEventWriter{}, Events: events,
		Docs: write.DocRepo{}, Questions: write.QuestionRepo{}, Delivery: delivery, Students: read.SchoolSettings{}, Registry: activities,
		Metrics: observability.AttemptCounter{}, Rate: cache.Rate{C: vk, Metrics: observability.RateCounter{}},
		Progress: write.MediaProgressRepo{}, Media: write.MediaRepo{}, Signer: signer, MediaMetrics: observability.VideoCounter{},
		Runs: write.SimulationRunRepo{}, SimMetrics: observability.SimulationCounter{}}
	lessons := command.Lessons{Store: write.LessonRepo{}, Docs: write.DocRepo{}, Learning: write.LearningEventWriter{}, Events: events,
		UoW: core.UoW, Metrics: observability.LessonCounter{}}
	att.Lessons = lessons
	// F032: review list, rapid_guess, adaptive selection, diagnostics and recommendations.
	selfRepo, practice := write.SelfStudyRepo{}, command.DefaultPracticeConfig()
	att.Self = &command.SelfStudy{Wrong: selfRepo, Recommendations: selfRepo, Reader: selfRepo, Masteries: write.MasteryRepo{}, Config: practice,
		Metrics: observability.SelfStudyCounter{}}
	recs := command.Recommendations{UoW: core.UoW, Store: selfRepo, Wrong: selfRepo, Reader: selfRepo, Masteries: write.MasteryRepo{},
		Delivery: delivery, Students: read.SchoolSettings{}, Events: events, Engine: recommendation.Default(), Config: practice,
		Metrics: observability.SelfStudyCounter{}, Sources: []command.SnapshotSource{command.LMSUpcoming{Store: write.LMSRepo{}}}}
	// F040: R6 (next lesson of the LMS schedule) registers into the engine of F032.
	recs.Engine.Register(recommendation.R6{})
	// F041: school assignments (ExamSessionCreator "unavailable" until F042), R1 and the gate of the assignment attempts.
	asg := command.Assignments{UoW: core.UoW, Store: write.SchoolAssignmentRepo{}, Read: core.ReadSession,
		Events: events, Audit: auditor, Metrics: observability.SchoolAssignmentCounter{}}
	// F042: the exam engine; its adapter replaces the "unavailable" ExamSessionCreator of F041 (F042-AC-31).
	exams := command.Exams{UoW: core.UoW, Store: write.ExamRepo{}, Events: events, Audit: auditor, Metrics: observability.ExamCounter{},
		Instruments: write.EfficacyRepo{},
		Balancer:    examquality.Balancer{}, PValues: write.QualityRepo{}}
	att.ExamRelease = exams.ReleaseOf
	att.EfficacyWave = exams.WaveOf // F035: no event, no mastery evidence for an efficacy exam
	asg.Exams = &command.ExamSessionCreator{Exams: &exams}
	att.Gate = asg
	exams.Asg, exams.Att = asg, att
	recs.Sources = append(recs.Sources, command.AssignmentsDue{Store: write.SchoolAssignmentRepo{}})
	recs.Engine.Register(recommendation.R1{})
	// F042: R5 (objectives below 60 % in a released exam).
	recs.Sources = append(recs.Sources, command.ExamFollowups{Store: write.ExamRepo{}})
	recs.Engine.Register(recommendation.R5{})
	asgq := query.SchoolAssignments{Session: core.ReadSession, Model: read.SchoolAssignmentModel{}, Cursors: cursors}
	lmsCmd := command.LMS{UoW: core.UoW, Store: write.LMSRepo{}, Read: core.ReadSession, Events: events, Audit: auditor,
		Metrics: observability.LMSCounter{}}
	attq := query.Attempts{Session: core.ReadSession, Model: read.AttemptModel{}, Docs: read.DocModel{}, Delivery: delivery,
		Registry: activities, Cursors: cursors, Media: read.MediaModel{}, Progress: read.MediaProgressModel{}, Signer: signer, Assignments: asgq, Runs: read.SimulationRunModel{},
		Recommendations: read.SelfStudyModel{}}

	h := learninghttp.Handlers{Commands: cur,
		Queries:          query.Curriculum{Session: core.ReadSession, Model: read.CurriculumModel{}, Settings: read.SchoolSettings{}, Codec: importer.Codec{}, Cursors: cursors},
		QuestionCommands: qcmd, QuestionQueries: query.Questions{Session: core.ReadSession, Model: read.QuestionModel{}, Cursors: cursors},
		Media: media, MediaQueries: query.MediaLibrary{Session: core.ReadSession, Model: read.MediaModel{}, Signer: signer, Cursors: cursors},
		Preview: command.Preview{UoW: core.UoW, Session: core.ReadSession, Questions: read.QuestionModel{}, Registry: registry, Schema: schema,
			Store: cache.PreviewStore{C: vk}},
		Docs: dcmd, DocQueries: docQueries,
		Widgets:  query.QuestionWidgets{Widgets: registry.Widgets()},
		Workflow: wf, Reviews: query.ReviewTasks{Session: core.ReadSession, Model: read.ReviewTaskModel{}, Cursors: cursors},
		Translations: tr, TranslationJobs: query.TranslationJobs{Session: core.ReadSession, Model: read.TranslationJobModel{}, Files: files},
		Releases: rel, ReleaseQueries: query.Releases{Session: core.ReadSession, Model: read.ReleaseModel{}, Reader: reader, Cursors: cursors},
		CmsActivities: query.CmsActivities{Docs: docQueries}, ActivityTypes: query.ActivityTypes{Registry: activities},
		Attempts: att, AttemptQueries: attq,
		Progress:     query.Progress{Session: core.ReadSession, Model: read.ProgressModel{}, Details: read.SelfStudyModel{}},
		ProfileAudit: command.ProfileViews{UoW: core.UoW, Audit: auditor}}
	h.Hints, h.Mastery = att, query.Progress{Session: core.ReadSession, Model: read.ProgressModel{}, Details: read.SelfStudyModel{}}
	h.LMSCommands = lmsCmd
	h.AssignmentCommands, h.Assignments = asg, asgq
	h.ExamCommands, h.ExamQueries = exams, query.Exams{Session: core.ReadSession, Model: read.ExamModel{}, Attempts: attq, Cursors: cursors}
	h.LMS = query.LMS{Session: core.ReadSession, Model: read.LMSModel{}, Progress: read.ProgressModel{}, Cursors: cursors}
	h.SelfStudy = query.SelfStudy{Session: core.ReadSession, Model: read.SelfStudyModel{}, Reasons: i18n.MustReasons(), Cursors: cursors}
	learningRecord := write.MasteryRepo{}
	mst := command.Mastery{Store: learningRecord, Evidence: learningRecord, Learning: write.LearningEventWriter{}, Events: events,
		Metrics: observability.MasteryCounter{}, Diagnostic: &command.DiagnosticPriors{Reader: selfRepo, Prior: practice.Diagnostic},
		Recommend: recs}
	// F043: the progress consumer also keeps the class/school/tenant daily rollups (late events: dashboard_late_window_hours).
	dashCfg := analytics.DefaultConfig()
	prg := command.Progress{Store: learningRecord, Evidence: learningRecord, Lessons: write.LessonRepo{}, Rollups: write.RollupRepo{},
		LateWindow: dashCfg.LateWindow, Late: observability.DashboardCounter{}}
	dashboards := command.Dashboards{UoW: core.UoW, Jobs: write.RollupRepo{}, Versions: cache.Dashboard{C: vk}, Config: dashCfg,
		Metrics: observability.DashboardCounter{}}
	dashSession, err := dashboardSession(r, r.ServesHTTP() || r.RunsJobs())
	if err != nil {
		return nil, err
	}
	dq := query.Dashboards{Model: read.DashboardModel{}, Cache: cache.Dashboard{C: vk}, Limiter: cache.NewLimiter(vk),
		Metrics: observability.DashboardCounter{}, Config: dashCfg, Cursors: cursors}
	if dashSession != nil {
		dq.Session = dashSession
	}
	h.Dashboards = dq
	h.Lessons, h.LessonQueries = lessons, query.Lessons{Session: core.ReadSession, Model: read.LessonModel{}}
	rgd := command.Regrades{UoW: core.UoW, Store: write.RegradeRepo{}, Attempts: att, Audit: auditor, Metrics: observability.RegradeCounter{}, Exams: exams}
	h.Regrades, h.RegradeQueries = rgd, query.Regrades{Session: core.ReadSession, Model: read.RegradeModel{}}

	deps := command.AssignmentDeps{UoW: core.UoW, Assignments: write.AssignmentRepo{}, Tenants: read.TenantDirectory{}, Resources: resources,
		Audit: auditor, Cache: cache.Invalidator{C: vk}, Metrics: observability.AssignmentCounter{}}
	trash := command.TrashDeps{UoW: core.UoW, Resources: resources, Events: events, Audit: auditor}
	f := facade.Facade{Apply: command.ApplyTenantAssignments{AssignmentDeps: deps}, Revoke: command.RevokeTenantAssignments{AssignmentDeps: deps},
		Assignments: query.ListResourceAssignments{Session: core.ReadSession, Assignments: read.AssignmentReadModel{}, Cursors: cursors},
		Trash:       query.ListTrash{Session: core.ReadSession, Resources: resources, Cursors: cursors},
		Restore:     command.RestoreResource{TrashDeps: trash},
		Clone: command.ExecuteCloneStep{CloneDeps: command.CloneDeps{UoW: core.UoW, Resources: resources, Tenants: read.TenantDirectory{},
			Audit: auditor}},
		AttemptOpener: att, AttemptResults: attq,
		Merge:       command.MergeUserData{UoW: core.UoW, Store: write.MergeRepo{}, Audit: auditor},
		Idempotency: core.Idempotency}
	// F044: exports on the dashboard session (replica) in core-api and in the worker, quality jobs and reads.
	reportCfg := reporting.DefaultConfig()
	reports := command.Reports{UoW: core.UoW, Jobs: write.ReportRepo{}, Queue: write.ReportRepo{}, Read: core.ReadSession, Scope: read.QualityModel{},
		Data: read.ReportModel{}, References: read.QualityModel{}, Files: files, Writers: export.Writers{}, Audit: auditor, Events: events,
		Config: reportCfg, Metrics: observability.ReportingCounter{}}
	if dashSession != nil {
		reports.DataSession = dashSession
	}
	quality := command.Quality{UoW: core.UoW, Items: write.QualityRepo{}, Calibrations: write.QualityRepo{}, Exams: write.QualityRepo{},
		References: write.QualityRepo{}, Tenants: write.RollupRepo{}, Questions: qcmd, Audit: auditor, Config: command.DefaultQualityConfig(),
		Metrics: observability.ReportingCounter{}}
	h.Reports = learninghttp.ReportHandlers{Commands: reports, Calibration: quality, Queries: query.Reports{Session: core.ReadSession,
		Jobs: read.ReportModel{}, Files: files, Courses: read.QualityModel{}, References: read.QualityModel{}, Quality: read.QualityModel{},
		Exams: read.QualityModel{}, Config: reportCfg, Cursors: cursors}}
	// F035: the efficacy study of System Admin (bypass efficacy_study across tenants) and its wave monitor.
	eff := command.Efficacy{UoW: core.UoW, Store: write.EfficacyRepo{}, Content: write.EfficacyRepo{}, Exams: &exams, Exclusions: write.ExamRepo{},
		Read: core.ReadSession, Data: read.EfficacyModel{}, Keys: secret.PseudonymKey{Path: cfg.String("EW_EFFICACY_PSEUDONYM_KEY_FILE")}, Audit: auditor, Config: efficacy.DefaultConfig(), Metrics: observability.EfficacyCounter{}}
	reports.Efficacy = eff // F035 US7: report_type efficacy_dataset of createPlatformReport
	f.Efficacy = eff       // F035-AC-19: anonymizeUserData excludes the student from the studies
	h.Reports.Commands = reports
	h.Efficacy = learninghttp.EfficacyHandlers{Commands: eff, Queries: query.Efficacy{Session: core.ReadSession, Model: read.EfficacyModel{},
		Cursors: cursors}}
	return &wiring{core: core, valkey: vk, http: h, facade: f, curriculum: cur, translations: tr, media: media, releases: rel,
		attempts: att, activities: activities, mastery: mst, progress: prg, regrades: rgd, lessons: lessons, recommendations: recs,
		lms: lmsCmd, assignments: asg, exams: exams, dashboards: dashboards, reports: reports, quality: quality, efficacy: eff}, nil
}

// dashboardSession opens the read session of the dashboards (F043-AC-18, F043-AC-21, DASHBOARD_DATA §7): the role
// learning_dashboard (EW_DB_LEARNING_DASHBOARD_DSN) and, for the cross-tenant summary of System Admin, learning_platform
// (EW_DB_LEARNING_DASHBOARD_PLATFORM_DSN), both on the read replica in staging/production (local: the primary). Without them
// the dashboard routes answer 503; they never fall back to the business pools.
func dashboardSession(r *module.Registry, needed bool) (*entx.ReadSession, error) {
	if !needed {
		return nil, nil
	}
	dash, err := r.SQL(Name, "dashboard")
	if errors.Is(err, module.ErrDatabaseNotConfigured) {
		slog.Warn("learning dashboards disabled: EW_DB_LEARNING_DASHBOARD_DSN is not set (503)")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plat, err := r.SQL(Name, "dashboard_platform")
	if errors.Is(err, module.ErrDatabaseNotConfigured) {
		slog.Warn("learning dashboards disabled: EW_DB_LEARNING_DASHBOARD_PLATFORM_DSN is not set (503)")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dash.SetMaxOpenConns(DashboardPoolSize)
	plat.SetMaxOpenConns(DashboardPoolSize)
	return entx.NewReadSession(Name, entx.DBs{App: dash, Platform: plat})
}
