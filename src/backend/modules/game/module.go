// Package game is the composition entry of module game (ADR-0074 §1, BACKEND_DESIGN §3.3).
// cmd/<binary> may only call Register, Facade and Wire; everything else lives in internal/.
package game

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	gameconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/security"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/ent/runtime" // ent mixin hooks and interceptors (R27)
)

// Name is the module name used for its database (eduworld_game), env prefix and checks.
const Name = "game"

// Facade is the part of the game facade (api.StrictServerInterface, specs/openapi/modules/game.internal.v1.yaml)
// implemented in this release: the student lifecycle operations of the identity sagas (F016). The platform-ops
// operations (assignments, trash, clone, mergeUserData) are not implemented by module game yet (OQ F045-3): the
// identity gateway answers 503 UNAVAILABLE for them, never a silent success.
type Facade interface {
	CleanupStudentData(ctx context.Context, req api.CleanupStudentDataRequestObject) (api.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req api.RestoreStudentDataRequestObject) (api.RestoreStudentDataResponseObject, error)
	AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error)
}

// Imports are the facades module game calls: none (projections and events only, ADR-0037).
type Imports struct{}

// Module is the registered module.
type Module struct {
	facade Facade
}

// Facade returns the in-process facade of module game (nil in a binary that does not load its use cases).
func (m *Module) Facade() Facade { return m.facade }

// Wire connects the facades of other modules (none for module game).
func (m *Module) Wire(Imports) {}

// Register wires the module into the running binary (OQ F014-7, F010-2): health checks everywhere; the F014 routes in
// core-api; the outbox relay and the consumers game.projection_identity and game.session_end_on_revoke in worker
// events; the jobs session-timeout and curfew-session-closer in worker jobs.
func Register(r *module.Registry) (*Module, error) {
	pool, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("game: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(pool))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(pool, migrations.Latest()))
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() {
		return &Module{}, nil
	}
	core, err := r.Core(Name)
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	cfg, err := gameconfig.SessionFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	cursors, err := r.Cursors()
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	vk, err := r.Valkey()
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	observability.Register(r.Metrics())
	observability.RegisterManifest(r.Metrics())
	sessions := command.Sessions{UoW: core.UoW, AfterCommit: entx.AfterCommit, Sessions: write.SessionRepo{}, Logins: write.LoginEventRepo{},
		Directory: write.Directory{}, Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder},
		Realtime: realtime.ValkeyPublisher{C: vk}, Metrics: observability.SessionMetrics{}, Config: cfg.UseCase}
	m := &Module{facade: facade.Facade{Lifecycle: command.StudentLifecycle{UoW: core.UoW, Sessions: write.SessionRepo{}}}}

	h := gamehttp.Handlers{Sessions: sessions, ClientConfig: cfg.ClientConfig,
		Manifest:    query.GetContentManifest{Read: core.ReadSession, Manifests: read.Manifests{}, MinClientVersion: cfg.UseCase.MinClientVersion},
		LoginEvents: query.LoginEvents{Read: core.ReadSession, Events: read.LoginEvents{}, Students: read.Students{}, Cursors: cursors}}
	if len(cfg.IPHashKey) > 0 {
		h.IPHasher = security.NewIPHasher(cfg.IPHashKey)
	}
	r.HTTP(Name, func(s *httpx.Server) error { return gamehttp.Register(s, h) })

	if err := r.Relay(Name); err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	r.Consumer("game.projection_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.IdentityProjectionOptions(consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}},
			core.UoW, core.App, js)
	})
	r.Consumer("game.session_end_on_revoke", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.SessionEndOnRevokeOptions(consumer.SessionEndOnRevoke{Sessions: sessions}, core.UoW, core.App, js)
	})
	r.Consumer("game.manifest_on_learning_content", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ManifestOnLearningContentOptions(consumer.ManifestOnLearningContent{Manifests: command.Manifests{UoW: core.UoW,
			Store: write.ManifestRepo{}, Events: write.Publisher{W: core.Writer}, Metrics: observability.ManifestMetrics{}}}, core.UoW, core.App, js)
	})
	r.Job(Name, job.SessionTimeout, cfg.TimeoutScan, module.JobFunc(job.SessionTimeoutJob(sessions)))
	r.Job(Name, job.CurfewSessionCloser, job.CurfewCloserInterval, module.JobFunc(job.CurfewSessionCloserJob(sessions)))
	return m, nil
}
