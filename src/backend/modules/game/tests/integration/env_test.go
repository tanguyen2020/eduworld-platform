//go:build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/security"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/ent/runtime" // mixin hooks, interceptors
)

// One migrated PostgreSQL for the package; each test works on its own copy of the databases.
func TestMain(m *testing.M) { os.Exit(testkit.RunWithSharedCluster(m)) }

// MinVersion is min_client_version of the tests.
const MinVersion = "1.2.0"

// clock is a settable test clock (TEST DOUBLE — R17).
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// pushes records realtime messages (TEST DOUBLE of port.RealtimePublisher — R17; the Valkey adapter has its own
// integration test).
type pushes struct {
	mu   sync.Mutex
	msgs []string // "<type>:<student>:<session>:<reason>"
}

func (p *pushes) SessionReplaced(_ context.Context, student, session uuid.UUID, _ string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, "replaced:"+student.String()+":"+session.String()+":")
	return nil
}

func (p *pushes) SessionEnded(_ context.Context, student, session uuid.UUID, reason string, _ time.Time, reopen *time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := "ended:" + student.String() + ":" + session.String() + ":" + reason
	if reopen != nil {
		m += ":reopen"
	}
	p.msgs = append(p.msgs, m)
	return nil
}

func (p *pushes) SessionEndingSoon(_ context.Context, student, session uuid.UUID, reason string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, "ending_soon:"+student.String()+":"+session.String()+":"+reason)
	return nil
}

func (p *pushes) all() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.msgs...)
}

// memTickets is an in-memory TEST DOUBLE of port.TicketStore (R17) for the tests that are not about tickets; the
// Valkey adapter is covered by realtime_ticket_test.go.
type memTickets struct {
	mu    sync.Mutex
	n     map[uuid.UUID]int
	saved map[string]rt.TicketClaims
}

func (m *memTickets) Allow(_ context.Context, user uuid.UUID, limit int, _ time.Duration) (bool, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.n == nil {
		m.n = map[uuid.UUID]int{}
	}
	m.n[user]++
	return m.n[user] <= limit, time.Minute, nil
}

func (m *memTickets) Save(_ context.Context, hash string, c rt.TicketClaims, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saved[hash] = c
	return nil
}

// env is the game integration environment of one test: pools, UnitOfWork, ReadSession, outbox writer, audit
// recorder, the use cases, the real pkg/httpx chain with a fake OIDC issuer, and tenants A, B with schools.
type env struct {
	cl                             *testkit.Cluster
	app, platform, migrator, relay *sql.DB
	uow                            *entx.UnitOfWork
	read                           *entx.ReadSession
	writer                         *outbox.Writer
	rec                            *outbox.AuditRecorder
	clock                          *clock
	pushes                         *pushes
	sessions                       command.Sessions
	cfg                            command.SessionConfig
	idp                            *httpxtest.IDP
	res                            *httpxtest.Resolver
	srv                            *httptest.Server
	tickets                        port.TicketStore
	realtimeURL                    string
	profiles                       command.Profiles
	profileCfg                     command.ProfileConfig
	options                        *memOptions
	tenantA, tenantB               uuid.UUID
	schoolA, schoolA2, schoolB     uuid.UUID
	// F071 world doubles (R17): presence, discovery rate limiter, world.area.unlocked pushes, hub positions (nil = none).
	presence  *memPresence
	rate      *memRate
	worldPush *memWorldPush
	hub       port.HubPositions
	// F072 quest doubles (R17): the learning facade (generated Fake) and the quest.progress.updated pushes.
	learning  *fakeLearning
	questPush *questPush
}

func open(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type envOpts struct {
	maintenance bool
	tickets     port.TicketStore // nil: in-memory TEST DOUBLE (tests not about the ticket store)
	realtimeURL string           // EW_REALTIME_PUBLIC_URL; "" = ws://localhost:9080/realtime/v1
	curfew      string           // platform curfew; "" = none (tests not about the curfew)
	emoteWheel  int              // emote_wheel_slots; 0 = 6 (CONFIG_DEFAULTS)
}

func setup(ctx context.Context, t *testing.T, o envOpts) *env {
	t.Helper()
	cl := testkit.Isolated(ctx, t)
	e := &env{cl: cl, app: open(t, cl.ModuleDSN("game", "app")), platform: open(t, cl.ModuleDSN("game", "platform")),
		migrator: open(t, cl.ModuleDSN("game", "migrator")), relay: open(t, cl.ModuleDSN("game", "relay")),
		tenantA: uuid.New(), tenantB: uuid.New(), schoolA: uuid.New(), schoolA2: uuid.New(), schoolB: uuid.New(),
		clock: &clock{now: time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC)}, pushes: &pushes{},
		presence: &memPresence{m: map[uuid.UUID]port.Presence{}}, rate: &memRate{n: map[string]int{}}, worldPush: &memWorldPush{}, learning: newFakeLearning(), questPush: &questPush{}}
	var err error
	if e.writer, err = outbox.NewWriter("game", "core", nil); err != nil {
		t.Fatal(err)
	}
	e.rec = outbox.NewAuditRecorder(e.writer, e.app)
	tenancy.Configure(tenancy.Hooks{Recorder: e.rec})
	authz.Configure(authz.Hooks{Recorder: e.rec})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	dbs := entx.DBs{App: e.app, Platform: e.platform}
	if e.uow, err = entx.NewUnitOfWork(entx.Options{Module: "game", DBs: dbs, Events: e.writer, Audit: e.rec}); err != nil {
		t.Fatal(err)
	}
	if e.read, err = entx.NewReadSession("game", dbs); err != nil {
		t.Fatal(err)
	}
	e.seedTenant(ctx, t, e.tenantA, "t-a", e.schoolA, e.schoolA2)
	e.seedTenant(ctx, t, e.tenantB, "t-b", e.schoolB)
	var curfew gamesession.Window
	if o.curfew != "" {
		if curfew, err = gamesession.ParseWindow(o.curfew, ict); err != nil {
			t.Fatal(err)
		}
	}
	e.cfg = command.SessionConfig{MinClientVersion: MinVersion, HeartbeatTimeout: 45 * time.Second, PlatformCurfew: curfew,
		CurfewLocation: ict, WarningBefore: 10 * time.Minute, Maintenance: command.Maintenance{Active: o.maintenance, Message: "Bảo trì định kỳ"}}
	e.sessions = command.Sessions{UoW: e.uow, AfterCommit: entx.AfterCommit, Sessions: write.SessionRepo{}, Logins: write.LoginEventRepo{}, Profiles: write.ProfileRepo{},
		Directory: write.Directory{}, Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec}, Realtime: e.pushes,
		Metrics: observability.SessionMetrics{}, Presence: e.presence, Config: e.cfg, Now: e.clock.Now}
	wheel := o.emoteWheel
	if wheel == 0 {
		wheel = 6
	}
	e.profileCfg = command.ProfileConfig{EmoteWheelSlots: wheel, SavedOutfitsMax: 3, SuggestionTTL: 10 * time.Minute,
		Nickname: profile.NicknamePolicy{MonthlyLimit: 1, Location: ict}, SupportedLocales: []string{"vi-VN", "en-US"}}
	e.options = &memOptions{now: e.clock.Now, sets: map[uuid.UUID]map[uuid.UUID]memOption{}}
	e.profiles = command.Profiles{UoW: e.uow, AfterCommit: entx.AfterCommit, Repo: write.ProfileRepo{}, Catalog: write.CatalogRepo{},
		Owned: write.OwnershipRepo{}, Presets: write.PresetRepo{}, Directory: write.Directory{}, Schools: write.Directory{},
		Options: e.options, Events: write.Publisher{W: e.writer}, Metrics: observability.ProfileMetrics{}, Config: e.profileCfg, Now: e.clock.Now}
	e.publishCatalog(ctx, t)
	e.tickets, e.realtimeURL = o.tickets, o.realtimeURL
	if e.tickets == nil {
		e.tickets = &memTickets{saved: map[string]rt.TicketClaims{}}
	}
	if e.realtimeURL == "" {
		e.realtimeURL = "ws://localhost:9080/realtime/v1"
	}
	e.server(ctx, t)
	return e
}

var ict = time.FixedZone("ICT", 7*3600)

func (e *env) server(ctx context.Context, t *testing.T) {
	t.Helper()
	spec, err := httpx.LoadSpec(ctx, testkit.RepoRoot(t)+"/specs/openapi/eduworld-api.v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	e.idp = httpxtest.NewIDP(t)
	e.res = &httpxtest.Resolver{}
	s, err := httpx.NewServer(httpx.Deps{Auth: e.idp.AuthConfig(), Keys: e.idp.JWKS(t), Resolver: e.res, Spec: spec,
		Idempotency: idempotency.NewStore(e.app, 0, nil)})
	if err != nil {
		t.Fatal(err)
	}
	codec, err := page.NewCodec([]byte(strings.Repeat("c", 32)))
	if err != nil {
		t.Fatal(err)
	}
	maint := e.cfg.Maintenance
	h := gamehttp.Handlers{Sessions: e.sessions,
		ClientConfig: query.ClientConfigSource{MinClientVersion: MinVersion, LatestClientVersion: "1.4.0",
			LoginURL: "https://api.dev.eduworld.example/api/v1/auth/game/login", RememberUsernameDefault: true,
			SupportedLocales: []string{"vi-VN", "en-US"},
			Maintenance: func() query.Maintenance {
				return query.Maintenance{Active: maint.Active, Message: maint.Message}
			}},
		Manifest:    query.GetContentManifest{Read: e.read, Manifests: read.Manifests{}, MinClientVersion: MinVersion},
		LoginEvents: query.LoginEvents{Read: e.read, Events: read.LoginEvents{}, Students: read.Students{}, Cursors: codec},
		IPHasher:    security.NewIPHasher([]byte(strings.Repeat("h", 32))),
		Tickets: command.RealtimeTickets{Sessions: e.sessions, Store: e.tickets,
			Config: command.TicketConfig{URL: e.realtimeURL, TTL: time.Minute, Rate: 10, RateWindow: time.Minute}},
		Profiles: e.profiles, ProfileQueries: query.Profile{Read: e.read, Profiles: read.Profiles{}, Catalog: read.Catalogs{},
			Options: e.options, TTL: e.profileCfg.SuggestionTTL},
		World: worldProxy{e}, WorldQueries: e.worldQueries(), Quests: questProxy{e}, QuestQueries: questQueryProxy{e},
		ProgressionQueries: progressionQueryProxy{e}, EconomyQueries: economyQueryProxy{e}, EconomyCommands: economyCommandProxy{e}}
	if err := gamehttp.Register(s, h); err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(s)
	t.Cleanup(e.srv.Close)
}

// seedTenant writes a tenant and its schools into the projections (as the consumer would).
func (e *env) seedTenant(ctx context.Context, t *testing.T, tenant uuid.UUID, code string, schools ...uuid.UUID) {
	t.Helper()
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO ref.tenants (tenant_id, code, tenant_type, status, source_version)
		VALUES ($1, $2, 'school', 'active', 1)`, tenant, code); err != nil {
		t.Fatal(err)
	}
	for _, s := range schools {
		if _, err := e.migrator.ExecContext(ctx, `INSERT INTO ref.schools (school_id, tenant_id, source_version) VALUES ($1, $2, 1)`,
			s, tenant); err != nil {
			t.Fatal(err)
		}
	}
}

// student is a projected student with its game token subject.
type student struct {
	id     uuid.UUID
	tenant uuid.UUID
	school uuid.UUID
	sub    string
}

// newStudent projects an active student into ref.students and registers its principal.
func (e *env) newStudent(ctx context.Context, t *testing.T, tenant, school uuid.UUID, status string) student {
	t.Helper()
	s := student{id: uuid.New(), tenant: tenant, school: school, sub: "kc-" + uuid.NewString()}
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, status, source_version)
		VALUES ($1, $2, $3, 3, $4, 1)`, s.id, tenant, school, status); err != nil {
		t.Fatal(err)
	}
	tt := tenant
	e.res.Set(s.sub, authz.PrincipalData{UserID: s.id, UserType: authz.UserTypeStudent, TenantID: &tt, TenantType: authz.TenantTypeSchool})
	return s
}

// staff registers a portal staff principal (tenant role) and returns its token subject.
func (e *env) staff(tenant uuid.UUID, role string, opts ...authztest.Option) string {
	sub := "kc-staff-" + uuid.NewString()
	e.res.Set(sub, authztest.Staff(tenant, role, opts...).PrincipalData)
	return sub
}

// platformStaff registers a platform staff principal and returns its token subject.
func (e *env) platformStaff(role string) string {
	sub := "kc-platform-" + uuid.NewString()
	e.res.Set(sub, authztest.Platform(role).PrincipalData)
	return sub
}

// gameToken is a student game token (azp eduworld-game-backend) with Keycloak session sid.
func (e *env) gameToken(s student, sid string) string {
	return e.idp.TokenSID(s.sub, httpxtest.GameClient, sid, time.Now())
}

type resp struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (r resp) code() string {
	c, _ := r.body["code"].(string)
	return c
}

func (e *env) do(t *testing.T, method, path, token string, headers map[string]string, body any) resp {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, e.srv.URL+"/api/v1"+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: buf.String()}
	_ = json.Unmarshal(buf.Bytes(), &out.body)
	return out
}

// device is one installation of the game (X-Device-Id) holding a game token.
type device struct {
	id    uuid.UUID
	token string
	sid   string
}

func (e *env) device(s student) device {
	sid := uuid.NewString()
	return device{id: uuid.New(), token: e.gameToken(s, sid), sid: sid}
}

func newKey() string { return uuid.Must(uuid.NewV7()).String() }

// start calls createGameSession from device d of student s.
func (e *env) start(t *testing.T, s student, d device, key string, body map[string]any) resp {
	t.Helper()
	if body == nil {
		body = map[string]any{"client_platform": "windows", "client_version": "1.2.3", "device_model": "Máy tính Windows"}
	}
	h := map[string]string{"X-Tenant-Id": s.tenant.String(), "X-Device-Id": d.id.String()}
	if key != "" {
		h["Idempotency-Key"] = key
	}
	return e.do(t, http.MethodPost, "/game/sessions", d.token, h, body)
}

func (e *env) takeover(t *testing.T, s student, d device) resp {
	t.Helper()
	return e.start(t, s, d, newKey(), map[string]any{"client_platform": "android", "client_version": "1.2.3",
		"device_model": "Điện thoại Android", "takeover": true})
}

func (e *env) end(t *testing.T, s student, d device, session string) resp {
	t.Helper()
	return e.do(t, http.MethodPost, "/game/sessions/"+session+":end", d.token, map[string]string{"X-Tenant-Id": s.tenant.String()}, nil)
}

func (e *env) heartbeat(t *testing.T, s student, d device, session string) resp {
	t.Helper()
	return e.do(t, http.MethodPost, "/game/sessions/"+session+":heartbeat", d.token,
		map[string]string{"X-Tenant-Id": s.tenant.String(), "X-Device-Id": d.id.String()}, nil)
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func count(ctx context.Context, t *testing.T, q querier, query string, args ...any) int {
	t.Helper()
	var n int
	if err := q.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// events counts outbox events of type (optionally about a session in their payload).
func (e *env) events(ctx context.Context, t *testing.T, typ string) int {
	t.Helper()
	return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1`, typ)
}

// audits counts audit.recorded events of action.
func (e *env) audits(ctx context.Context, t *testing.T, action string) int {
	t.Helper()
	return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = $1`, action)
}

// payloads returns the payloads of the outbox events of type, oldest first.
func (e *env) payloads(ctx context.Context, t *testing.T, typ string) []map[string]any {
	t.Helper()
	rows, err := e.relay.QueryContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = $1 ORDER BY created_at`, typ)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		out = append(out, m)
	}
	return out
}

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if !errors.As(err, &pe) {
		return ""
	}
	return pe.Code
}

// authztestStudent is the principal data of a student of tenant (resolver entry).
func authztestStudent(id, tenant uuid.UUID) authz.PrincipalData {
	return authz.PrincipalData{UserID: id, UserType: authz.UserTypeStudent, TenantID: &tenant, TenantType: authz.TenantTypeSchool}
}

// nowWall is the wall clock for tokens (the test clock drives the use cases, the issuer checks exp against now).
func nowWall() time.Time { return time.Now() }
