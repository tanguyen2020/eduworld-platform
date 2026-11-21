//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// memPresence is an in-memory TEST DOUBLE of port.PresenceStore (R17); the Valkey adapter has its own test
// (world_valkey_test.go). lost simulates a Valkey loss.
type memPresence struct {
	mu   sync.Mutex
	m    map[uuid.UUID]port.Presence
	lost bool
}

func (p *memPresence) Set(_ context.Context, student uuid.UUID, v port.Presence, _ time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[student] = v
	return nil
}

func (p *memPresence) Get(_ context.Context, student uuid.UUID) (*port.Presence, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.m[student]
	if !ok || p.lost {
		return nil, nil
	}
	return &v, nil
}

func (p *memPresence) Touch(context.Context, uuid.UUID, time.Duration) error { return nil }

func (p *memPresence) get(student uuid.UUID) (port.Presence, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.m[student]
	return v, ok
}

func (p *memPresence) lose(lost bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lost = lost
}

// memRate is an in-memory fixed-window TEST DOUBLE of port.RateLimiter (R17).
type memRate struct {
	mu sync.Mutex
	n  map[string]int
}

func (r *memRate) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.n[key]++
	return r.n[key] <= limit, window, nil
}

// memWorldPush records world.area.unlocked pushes (TEST DOUBLE of port.WorldRealtime, R17).
type memWorldPush struct {
	mu   sync.Mutex
	msgs []string // "<student>:<area>:<by>"
}

func (p *memWorldPush) AreaUnlocked(_ context.Context, student uuid.UUID, area string, _ world.Name, by string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, student.String()+":"+area+":"+by)
	return nil
}

func (p *memWorldPush) all() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.msgs...)
}

// memHub is a TEST DOUBLE of port.HubPositions (R17): the position the room host of F091 reports (nil = not in a hub).
type memHub struct {
	mu  sync.Mutex
	pos map[uuid.UUID]*port.HubPosition
}

func (h *memHub) Position(_ context.Context, _, student uuid.UUID) (*port.HubPosition, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pos[student], nil
}

// worldUC returns the world use cases on the test databases with the doubles of env.
func (e *env) worldUC() command.World {
	return command.World{UoW: e.uow, AfterCommit: entx.AfterCommit, Content: worldsql.Content{}, Store: writerepo.WorldRepo{},
		Sessions: writerepo.SessionRepo{}, Manifests: writerepo.ManifestRepo{}, Events: writerepo.Publisher{W: e.writer}, Presence: e.presence,
		Rate: e.rate, Realtime: e.worldPush, Hub: e.hub, Metrics: observability.WorldMetrics{}, Config: command.WorldConfig{PresenceTTL: 45 * time.Second,
			DiscoveryRate: 30, DiscoveryWindow: time.Minute, EnteredDedupe: time.Minute, InteractRadius: 3, InteractTolerance: 1.5}, Now: e.clock.Now}
}

// worldProxy calls the current world use cases of env (so a test may swap the hub double after setup).
type worldProxy struct{ e *env }

func (p worldProxy) EnterArea(ctx context.Context, in command.EnterInput) (command.EnterResult, error) {
	return p.e.worldUC().EnterArea(ctx, in)
}

func (p worldProxy) StartLevel(ctx context.Context, level, session uuid.UUID) (command.LevelSessionView, error) {
	return p.e.worldUC().StartLevel(ctx, level, session)
}

func (p worldProxy) EndLevel(ctx context.Context, id uuid.UUID, outcome string) (command.LevelSessionView, error) {
	return p.e.worldUC().EndLevel(ctx, id, outcome)
}

func (p worldProxy) ReportDiscovery(ctx context.Context, area, id string) (command.DiscoveryResult, error) {
	return p.e.worldUC().ReportDiscovery(ctx, area, id)
}

// worldQueries returns the world read use cases.
func (e *env) worldQueries() query.World {
	return query.World{Read: e.read, Content: worldsql.Content{}, Manifests: read.Manifests{}}
}

// session starts a game session for s and returns the device and the session id.
func (e *env) session(t *testing.T, s student) (device, string) {
	t.Helper()
	d := e.device(s)
	r := e.start(t, s, d, newKey(), nil)
	if r.status != http.StatusCreated {
		t.Fatalf("start session: %d %s", r.status, r.raw)
	}
	return d, r.body["game_session_id"].(string)
}

// enter calls enterWorldArea.
func (e *env) enter(t *testing.T, s student, d device, session, area, portal string) resp {
	t.Helper()
	body := map[string]any{"game_session_id": session}
	if portal != "" {
		body["portal_id"] = portal
	}
	return e.do(t, http.MethodPost, "/game/world/areas/"+area+":enter", d.token, th(s), body)
}

// discover calls reportDiscovery with key.
func (e *env) discover(t *testing.T, s student, d device, area, id, key string) resp {
	t.Helper()
	h := th(s)
	h["Idempotency-Key"] = key
	return e.do(t, http.MethodPost, "/game/world/areas/"+area+"/discoveries/"+id+":discover", d.token, h, nil)
}

// progress delivers one progress event of the game stream to the consumer game.unlock_on_events (TenantScope).
func (e *env) progress(ctx context.Context, t *testing.T, typ string, tenant uuid.UUID, payload map[string]any) {
	t.Helper()
	if err := e.progressEnv(ctx, t, e.progressEnvelope(typ, tenant, payload)); err != nil {
		t.Fatal(err)
	}
}

func (e *env) progressEnvelope(typ string, tenant uuid.UUID, payload map[string]any) outbox.Envelope {
	b, _ := json.Marshal(payload)
	tt := tenant
	return outbox.Envelope{EventID: uuid.New(), EventType: typ, SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "game", TenantID: &tt, Payload: b}
}

func (e *env) progressEnv(ctx context.Context, t *testing.T, env outbox.Envelope) error {
	t.Helper()
	sc, err := authz.EventScope(*env.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	c := consumer.UnlockOnEvents{World: e.worldUC()}
	return e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return c.Handle(ctx, env) })
}

// setGrade changes the projected grade of a student (as student.synced would).
func (e *env) setGrade(ctx context.Context, t *testing.T, s student, grade int) {
	t.Helper()
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.students SET grade_level = $2 WHERE student_id = $1`, s.id, grade); err != nil {
		t.Fatal(err)
	}
}

// enableOnly keeps only subjects on in the settings of the school (school.updated settings_subset.enabled_subject_codes).
func (e *env) enableOnly(ctx context.Context, t *testing.T, school uuid.UUID, subjects ...string) {
	t.Helper()
	b, _ := json.Marshal(subjects)
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.schools SET settings_subset = settings_subset || jsonb_build_object('enabled_subject_codes', $2::jsonb)
		WHERE school_id = $1`, school, string(b)); err != nil {
		t.Fatal(err)
	}
}

// scopeOf is the TenantScope of an event of tenant.
func scopeOf(t *testing.T, tenant uuid.UUID) tenancy.Scope {
	t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// txOf is the transaction of ctx.
func txOf(ctx context.Context) *sql.Tx {
	tx, _ := entx.Tx(ctx)
	return tx
}
