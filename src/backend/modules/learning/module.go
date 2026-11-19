// Package learning is the composition entry of module learning (ADR-0074 §1, BACKEND_DESIGN §3.3).
// cmd/<binary> may only call Register, Facade and Wire; everything else lives in internal/.
package learning

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/readiness"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/importer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/mediaproc"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/qbschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/translation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	learninghttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/migrations"
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
)

// Facade is the part of the learning facade (api.StrictServerInterface, specs/openapi/modules/learning.internal.v1.yaml)
// implemented in this release: the platform-ops operations of F020 and the student lifecycle steps of F016. The
// attempt and question-plan operations (createAttempt, getAttemptResult, createQuestionPlan, scorePlanAnswer) and
// mergeUserData belong to later features (OQ F045-3): no binary calls them yet.
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

// Register wires the module into the running binary (OQ F020-3, F010-2): health checks everywhere; the CMS routes
// (F021–F027) in core-api; the outbox relay and the consumers projection_identity, qb_pool_invalidation and
// media_worker in worker events; the import and translation jobs in worker jobs.
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
	r.Job(Name, "import-jobs", ImportJobInterval, job.ImportWorker{Runner: w.curriculum}.RunOnce)
	r.Job(Name, "translation-jobs", TranslationJobInterval, job.TranslationWorker{Runner: w.translations}.RunOnce)
	r.Job(Name, "content-scheduled-publish", ScheduledPublishInterval, job.ScheduleWorker{Runner: w.releases}.RunOnce)
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
	registry := question.Full() // F023 Done: the 11 question types (OQ F023-2 [Proposed])

	cur := command.Curriculum{UoW: core.UoW, Store: write.CurriculumRepo{}, Catalog: write.CatalogRepo{}, Imports: write.ImportJobRepo{},
		Resources: resources, Events: events, Audit: auditor, Assets: assets, Codec: importer.Codec{}, Metrics: observability.CurriculumCounter{}}
	qcmd := command.Questions{UoW: core.UoW, Store: write.QuestionRepo{}, Curriculum: write.CurriculumRepo{}, Catalog: write.CatalogRepo{},
		Schema: schema, Registry: registry, Resources: resources, Events: events, Audit: auditor, Math: math, Media: write.MediaRepo{},
		Docs: write.DocRepo{}}
	dcmd := command.Docs{UoW: core.UoW, Store: write.DocRepo{}, Questions: write.QuestionRepo{}, Curriculum: write.CurriculumRepo{},
		Catalog: write.CatalogRepo{}, Schema: docSchema, Media: write.MediaRepo{}, Read: core.ReadSession, Resources: resources,
		Events: events, Audit: auditor, Math: math}
	wf := command.Workflow{UoW: core.UoW, Status: write.WorkflowRepo{}, Reviews: write.WorkflowRepo{}, Questions: write.QuestionRepo{},
		Docs: write.DocRepo{}, Schema: schema, Media: write.MediaRepo{}, Guard: readiness.Default(), Events: events, Audit: auditor,
		Metrics: observability.WorkflowCounter{}}
	tr := command.Translations{UoW: core.UoW, Jobs: write.TranslationJobRepo{}, Workflow: wf, Questions: qcmd, Docs: dcmd,
		Codec: translation.Codec{}, Files: files, Assets: assets, Audit: auditor}
	cur.Content = &command.ContentImport{Questions: qcmd, Codec: importer.QuestionCodec{}}
	snapshots := cache.SnapshotCache{C: vk}
	rel := command.Releases{UoW: core.UoW, Workflow: wf, Store: write.ReleaseRepo{}, Schedules: write.ReleaseRepo{}, Events: events,
		Audit: auditor, Metrics: observability.ReleaseCounter{}, Cache: snapshots, CDN: s3, Signer: signer}
	reader := query.ContentReader{Questions: read.QuestionModel{}, Docs: read.DocModel{}, Cache: snapshots}

	h := learninghttp.Handlers{Commands: cur,
		Queries:          query.Curriculum{Session: core.ReadSession, Model: read.CurriculumModel{}, Settings: read.SchoolSettings{}, Codec: importer.Codec{}, Cursors: cursors},
		QuestionCommands: qcmd, QuestionQueries: query.Questions{Session: core.ReadSession, Model: read.QuestionModel{}, Cursors: cursors},
		Media: media, MediaQueries: query.MediaLibrary{Session: core.ReadSession, Model: read.MediaModel{}, Signer: signer, Cursors: cursors},
		Preview: command.Preview{UoW: core.UoW, Session: core.ReadSession, Questions: read.QuestionModel{}, Registry: registry, Schema: schema,
			Store: cache.PreviewStore{C: vk}},
		Docs: dcmd, DocQueries: query.Docs{Session: core.ReadSession, Model: read.DocModel{}, Pool: read.Delivery{}, Cursors: cursors},
		Widgets:  query.QuestionWidgets{Widgets: registry.Widgets()},
		Workflow: wf, Reviews: query.ReviewTasks{Session: core.ReadSession, Model: read.ReviewTaskModel{}, Cursors: cursors},
		Translations: tr, TranslationJobs: query.TranslationJobs{Session: core.ReadSession, Model: read.TranslationJobModel{}, Files: files},
		Releases: rel, ReleaseQueries: query.Releases{Session: core.ReadSession, Model: read.ReleaseModel{}, Reader: reader, Cursors: cursors}}

	deps := command.AssignmentDeps{UoW: core.UoW, Assignments: write.AssignmentRepo{}, Tenants: read.TenantDirectory{}, Resources: resources,
		Audit: auditor, Cache: cache.Invalidator{C: vk}, Metrics: observability.AssignmentCounter{}}
	trash := command.TrashDeps{UoW: core.UoW, Resources: resources, Events: events, Audit: auditor}
	f := facade.Facade{Apply: command.ApplyTenantAssignments{AssignmentDeps: deps}, Revoke: command.RevokeTenantAssignments{AssignmentDeps: deps},
		Assignments: query.ListResourceAssignments{Session: core.ReadSession, Assignments: read.AssignmentReadModel{}, Cursors: cursors},
		Trash:       query.ListTrash{Session: core.ReadSession, Resources: resources, Cursors: cursors},
		Restore:     command.RestoreResource{TrashDeps: trash},
		Clone: command.ExecuteCloneStep{CloneDeps: command.CloneDeps{UoW: core.UoW, Resources: resources, Tenants: read.TenantDirectory{},
			Audit: auditor}},
		Idempotency: core.Idempotency}
	return &wiring{core: core, valkey: vk, http: h, facade: f, curriculum: cur, translations: tr, media: media, releases: rel}, nil
}
