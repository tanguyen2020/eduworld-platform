//go:build integration

// Package integration holds the integration tests of module chat (F095): migrated PostgreSQL (eduworld_chat), the real
// pkg/httpx chain with a fake OIDC issuer, the production use cases and repositories; Valkey and NATS (testcontainers)
// for the presence fan-out. Projections are seeded in ref.* as the consumers write them (the consumers have their own
// test).
package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/security"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/ent/runtime" // mixin hooks, interceptors
)

// One migrated PostgreSQL for the package; each test works on its own copy of the databases.
func TestMain(m *testing.M) { os.Exit(testkit.RunWithSharedCluster(m)) }

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

func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var ict = time.FixedZone("ICT", 7*3600)

// limits are friend_limits of CONFIG_DEFAULTS (10 pending, 20 per day, 100 friends).
var limits = social.Limits{MaxPending: 10, MaxPerDay: 20, MaxFriends: 100, DayLocation: ict}

// env is the chat integration environment of one test: pools, UnitOfWork, ReadSession, outbox writer, audit recorder,
// the F095 use cases behind the real pkg/httpx chain, and tenants A (school), B (school) and U (unit) with schools.
type env struct {
	t                          *testing.T
	app, platform, relay       *sql.DB
	su                         *sql.DB // superuser of the test database: seeds and assertions across tenants (RLS FORCE)
	uow                        *entx.UnitOfWork
	read                       *entx.ReadSession
	writer                     *outbox.Writer
	rec                        *outbox.AuditRecorder
	clock                      *clock
	social                     command.Social
	queries                    query.Social
	idp                        *httpxtest.IDP
	res                        *httpxtest.Resolver
	srv                        *httptest.Server
	tenantA, tenantB, tenantU  uuid.UUID
	schoolA, schoolA2, schoolB uuid.UUID
	schoolU1, schoolU2         uuid.UUID
	chatHandlers               *chathttp.ChatHandlers    // F115 routes (setupChat)
	catalogHandlers            *chathttp.CatalogHandlers // F116 US2 routes (setupChat with the real pipeline)
	switchHandlers             *chathttp.SwitchHandlers  // F116 US3 routes (setupChat with the real pipeline)
	safetyHandlers             *chathttp.SafetyHandlers  // F121 routes (setupSafety)
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
	limits   *social.Limits
	presence port.PresenceStore // nil: everyone offline in listFriends
	friends  port.FriendCache
}

func setup(t *testing.T, o envOpts) *env {
	t.Helper()
	ctx := context.Background()
	cl := testkit.Isolated(ctx, t)
	e := &env{t: t, app: open(t, cl.ModuleDSN("chat", "app")), platform: open(t, cl.ModuleDSN("chat", "platform")),
		relay: open(t, cl.ModuleDSN("chat", "relay")), clock: &clock{now: time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)},
		tenantA: uuid.New(), tenantB: uuid.New(), tenantU: uuid.New(), schoolA: uuid.New(), schoolA2: uuid.New(), schoolB: uuid.New(),
		schoolU1: uuid.New(), schoolU2: uuid.New()}
	migrator := open(t, cl.ModuleDSN("chat", "migrator"))
	var dbName string
	if err := migrator.QueryRowContext(ctx, `SELECT current_database()`).Scan(&dbName); err != nil {
		t.Fatal(err)
	}
	e.su = open(t, cl.DSN("postgres", dbName))
	var err error
	if e.writer, err = outbox.NewWriter("chat", "core", nil); err != nil {
		t.Fatal(err)
	}
	e.rec = outbox.NewAuditRecorder(e.writer, e.app)
	tenancy.Configure(tenancy.Hooks{Recorder: e.rec})
	authz.Configure(authz.Hooks{Recorder: e.rec})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	dbs := entx.DBs{App: e.app, Platform: e.platform}
	if e.uow, err = entx.NewUnitOfWork(entx.Options{Module: "chat", DBs: dbs, Events: e.writer, Audit: e.rec}); err != nil {
		t.Fatal(err)
	}
	if e.read, err = entx.NewReadSession("chat", dbs); err != nil {
		t.Fatal(err)
	}
	e.tenant(e.tenantA, "school", nil)
	e.tenant(e.tenantB, "school", nil)
	e.tenant(e.tenantU, "unit", nil)
	l := limits
	if o.limits != nil {
		l = *o.limits
	}
	e.social = command.Social{UoW: e.uow, AfterCommit: entx.AfterCommit, Repo: persistence.Social{}, Directory: persistence.Directory{},
		Events: persistence.Publisher{W: e.writer}, Audit: persistence.Auditor{R: e.rec}, Codes: security.Codes{}, Friends: o.friends,
		Metrics: observability.Social{}, Config: command.SocialConfig{Limits: l, RequestTTL: 7 * 24 * time.Hour, CodeTTL: 10 * time.Minute},
		Now: e.clock.Now}
	codec, err := page.NewCodec([]byte(strings.Repeat("c", 32)))
	if err != nil {
		t.Fatal(err)
	}
	e.queries = query.Social{Read: e.read, Relations: persistence.Reader{}, Presence: o.presence, Cursors: codec, Now: e.clock.Now}
	e.server(ctx)
	return e
}

func (e *env) server(ctx context.Context) {
	e.t.Helper()
	spec, err := httpx.LoadSpec(ctx, testkit.RepoRoot(e.t)+"/specs/openapi/eduworld-api.v1.yaml")
	if err != nil {
		e.t.Fatal(err)
	}
	e.idp = httpxtest.NewIDP(e.t)
	e.res = &httpxtest.Resolver{}
	s, err := httpx.NewServer(httpx.Deps{Auth: e.idp.AuthConfig(), Keys: e.idp.JWKS(e.t), Resolver: e.res, Spec: spec,
		Idempotency: idempotency.NewStore(e.app, 0, nil)})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := chathttp.Register(s, chathttp.Handlers{Commands: e.social, Queries: e.queries, Chat: e.chatHandlers,
		Catalog: e.catalogHandlers, Switches: e.switchHandlers, Safety: e.safetyHandlers}); err != nil {
		e.t.Fatal(err)
	}
	e.srv = httptest.NewServer(s)
	e.t.Cleanup(e.srv.Close)
}

func (e *env) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.su.ExecContext(context.Background(), q, args...); err != nil {
		e.t.Fatalf("%s: %v", strings.Fields(q)[0], err)
	}
}

func (e *env) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.su.QueryRowContext(context.Background(), q, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// tenant projects a tenant (settings_subset as the identity consumer writes it).
func (e *env) tenant(id uuid.UUID, typ string, settings map[string]any) {
	e.t.Helper()
	b := []byte("{}")
	if settings != nil {
		b, _ = json.Marshal(settings)
	}
	e.exec(`INSERT INTO ref.tenants (tenant_id, tenant_type, status, settings_subset, source_version) VALUES ($1, $2, 'active', $3, 1)
		ON CONFLICT (tenant_id) DO UPDATE SET tenant_type = $2, settings_subset = $3`, id, typ, string(b))
}

// student is a projected student with a public profile and its game token subject.
type student struct {
	id, tenant, school uuid.UUID
	nickname           string
	sub                string
	token              string
}

// student projects an active student of tenant/school (ref.students, ref.player_profiles) and registers its principal.
func (e *env) student(tenant, school uuid.UUID) student {
	e.t.Helper()
	s := student{id: uuid.New(), tenant: tenant, school: school, nickname: "Meo" + uuid.NewString()[:6], sub: "kc-" + uuid.NewString()}
	e.exec(`INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, status, source_version) VALUES ($1, $2, $3, 3, 'active', 1)`,
		s.id, tenant, school)
	e.exec(`INSERT INTO ref.player_profiles (student_id, tenant_id, nickname, level, avatar_config, source_version)
		VALUES ($1, $2, $3, 4, '{"body":"body_01","hair":"hair_02"}', 1)`, s.id, tenant, s.nickname)
	tt := tenant
	typ := authz.TenantTypeSchool
	e.res.Set(s.sub, authz.PrincipalData{UserID: s.id, UserType: authz.UserTypeStudent, TenantID: &tt, TenantType: typ})
	s.token = e.idp.TokenSID(s.sub, httpxtest.GameClient, uuid.NewString(), time.Now())
	return s
}

// staff registers a portal staff principal (tenant role) and returns its portal token.
func (e *env) staff(tenant uuid.UUID, role string) string {
	sub := "kc-staff-" + uuid.NewString()
	e.res.Set(sub, authztest.Staff(tenant, role).PrincipalData)
	return e.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

// platformStaff registers a platform staff principal and returns its portal token.
func (e *env) platformStaff(role string) string {
	sub := "kc-platform-" + uuid.NewString()
	e.res.Set(sub, authztest.Platform(role).PrincipalData)
	return e.idp.Token(sub, httpxtest.PortalClient, time.Now())
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

func (r resp) data() []map[string]any {
	items, _ := r.body["data"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func newKey() string { return uuid.Must(uuid.NewV7()).String() }

// call sends one request with the token and X-Tenant-Id of s (and Idempotency-Key when key is not empty).
func (e *env) call(s student, method, path, key string, body any) resp {
	e.t.Helper()
	h := map[string]string{"X-Tenant-Id": s.tenant.String()}
	if key != "" {
		h["Idempotency-Key"] = key
	}
	return e.do(method, path, s.token, h, body)
}

func (e *env) do(method, path, token string, headers map[string]string, body any) resp {
	e.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, e.srv.URL+"/api/v1"+path, rd)
	if err != nil {
		e.t.Fatal(err)
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
		e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: buf.String()}
	_ = json.Unmarshal(buf.Bytes(), &out.body)
	return out
}

// send is sendFriendRequest from a to b with a new key; it fails the test unless the status is want.
func (e *env) send(a, b student, want int) resp {
	e.t.Helper()
	r := e.call(a, http.MethodPost, "/social/friend-requests", newKey(), map[string]any{"addressee_id": b.id})
	if r.status != want {
		e.t.Fatalf("send %s → %s: %d %s", a.nickname, b.nickname, r.status, r.raw)
	}
	return r
}

func requestID(r resp) string {
	id, _ := r.body["request_id"].(string)
	return id
}

// befriend makes a and b friends through the API.
func (e *env) befriend(a, b student) {
	e.t.Helper()
	id := requestID(e.send(a, b, http.StatusCreated))
	if r := e.call(b, http.MethodPost, "/social/friend-requests/"+id+":accept", newKey(), nil); r.status != http.StatusNoContent {
		e.t.Fatalf("accept: %d %s", r.status, r.raw)
	}
}

// events returns the payloads of the outbox events of type typ, oldest first.
func (e *env) events(typ string) []map[string]any {
	e.t.Helper()
	rows, err := e.relay.QueryContext(context.Background(), `SELECT envelope FROM infra.outbox_events WHERE event_type = $1
		ORDER BY created_at, event_id`, typ)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			e.t.Fatal(err)
		}
		var env struct {
			Payload map[string]any `json:"payload"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, env.Payload)
	}
	return out
}

// audits returns the actions of the audit.recorded events of the outbox.
func (e *env) audits() []string {
	var out []string
	for _, p := range e.events("audit.recorded") {
		if a, ok := p["action"].(string); ok {
			out = append(out, a)
		}
	}
	return out
}

// chatEventTypes are the events of module chat owned by F095 (specs/events/payloads/chat).
var chatEventTypes = []string{"friend_request.sent", "friend.added", "friend.removed", "user.blocked", "user.unblocked", "user.muted",
	"user.unmuted"}

// checkEventSchemas validates every F095 event of the outbox: envelope and payload schemas (contract first; the
// consumers of competition rely on them, T021). Returns the number checked per type.
func (e *env) checkEventSchemas() map[string]int {
	e.t.Helper()
	return e.checkEvents(chatEventTypes...)
}

// checkEvents validates the outbox events of the given types against the envelope and their payload schema
// (specs/events/payloads/chat/<type>.v1.schema.json); returns the number checked per type.
func (e *env) checkEvents(types ...string) map[string]int {
	e.t.Helper()
	root := filepath.Join(testkit.RepoRoot(e.t), "specs", "events")
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	envelope, err := c.Compile(filepath.Join(root, "envelope.schema.json"))
	if err != nil {
		e.t.Fatal(err)
	}
	seen := map[string]int{}
	for _, typ := range types {
		payload, err := c.Compile(filepath.Join(root, "payloads", "chat", typ+".v1.schema.json"))
		if err != nil {
			e.t.Fatal(err)
		}
		rows, err := e.relay.QueryContext(context.Background(), `SELECT envelope FROM infra.outbox_events WHERE event_type = $1`, typ)
		if err != nil {
			e.t.Fatal(err)
		}
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				e.t.Fatal(err)
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				e.t.Fatal(err)
			}
			if err := envelope.Validate(doc); err != nil {
				e.t.Errorf("%s envelope: %v", typ, err)
			}
			var env struct {
				Payload json.RawMessage `json:"payload"`
			}
			_ = json.Unmarshal(raw, &env)
			pd, err := jsonschema.UnmarshalJSON(bytes.NewReader(env.Payload))
			if err != nil {
				e.t.Fatal(err)
			}
			if err := payload.Validate(pd); err != nil {
				e.t.Errorf("%s payload: %v", typ, err)
			}
			seen[typ]++
		}
		_ = rows.Close()
	}
	return seen
}

// pii fails the test when a response leaks a real name, school, class or score field (R16).
func pii(t *testing.T, r resp) {
	t.Helper()
	for _, k := range []string{"school_id", "school", "class", "class_id", "full_name", "display_name", "username", "grade_level",
		"mastery", "score", "tenant_id"} {
		if strings.Contains(r.raw, `"`+k+`"`) {
			t.Fatalf("response leaks %s: %s", k, r.raw)
		}
	}
}
