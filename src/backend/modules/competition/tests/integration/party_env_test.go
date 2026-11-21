//go:build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// pclock is a settable test clock (TEST DOUBLE — R17).
type pclock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *pclock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *pclock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// pushRec is a TEST DOUBLE of port.PartyRealtime (R17): it records the realtime messages per recipient.
type pushRec struct {
	mu   sync.Mutex
	msgs []pushMsg
}

type pushMsg struct {
	to      uuid.UUID
	typ     string
	payload map[string]any
}

func (r *pushRec) Send(_ context.Context, to []uuid.UUID, typ string, payload any) error {
	b, _ := json.Marshal(payload)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range to {
		r.msgs = append(r.msgs, pushMsg{to: id, typ: typ, payload: m})
	}
	return nil
}

// of returns the messages of type typ sent to user.
func (r *pushRec) of(user uuid.UUID, typ string) []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []map[string]any
	for _, m := range r.msgs {
		if m.to == user && m.typ == typ {
			out = append(out, m.payload)
		}
	}
	return out
}

// onlineSet is a TEST DOUBLE of port.PartyPresence (R17): who is online is set by the test.
type onlineSet struct {
	mu sync.Mutex
	on map[uuid.UUID]bool
}

func (o *onlineSet) set(id uuid.UUID, on bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.on == nil {
		o.on = map[uuid.UUID]bool{}
	}
	o.on[id] = on
}

func (o *onlineSet) Online(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		out[id] = o.on[id]
	}
	return out, nil
}

// blockedName is the placeholder the fake moderator blocks (R15: no real sensitive word in tests).
const blockedName = "PLACEHOLDER_BLOCKED"

// fakeModerator is a TEST DOUBLE of port.NameModerator (MOCK of moderateDisplayText until F116 — R17): blocks names
// containing blockedName; down = module chat not answering.
type fakeModerator struct {
	mu    sync.Mutex
	down  bool
	calls int
}

func (m *fakeModerator) Check(_ context.Context, _, _ uuid.UUID, text string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.down {
		return false, port.ErrModerationUnavailable
	}
	return !strings.Contains(text, blockedName), nil
}

// partyConfig is the CONFIG_DEFAULTS of the party (party_max_members 4, invite TTL 5 min, 10/min, 50/day, repeat 5 min
// and 3/day, 20 pending, 10 creations and 10 kicks per hour, kick block 10 min).
var partyConfig = command.PartyConfig{DefaultMaxMembers: 4, InviteTTL: 5 * time.Minute, InvitePerMinute: 10, InvitePerDay: 50,
	RepeatWait: 5 * time.Minute, RejectsPerDay: 3, PendingMax: 20, CreatePerHour: 10, KickPerHour: 10, KickReinviteBlock: 10 * time.Minute}

// penv is the party integration environment of one test: migrated eduworld_competition, the F100 use cases behind the
// real pkg/httpx chain with a fake OIDC issuer, tenants A (school), B (school) and U (unit, cross-school on).
type penv struct {
	t                           *testing.T
	app, platform, relay, su    *sql.DB
	uow                         *entx.UnitOfWork
	read                        *entx.ReadSession
	rec                         *outbox.AuditRecorder
	clock                       *pclock
	push                        *pushRec
	online                      *onlineSet
	mod                         *fakeModerator
	parties                     command.Parties
	queries                     query.Parties
	idp                         *httpxtest.IDP
	res                         *httpxtest.Resolver
	srv                         *httptest.Server
	tenantA, tenantB, tenantU   uuid.UUID
	schoolA, schoolA2           uuid.UUID
	schoolB, schoolU1, schoolU2 uuid.UUID
	classA, classA2             uuid.UUID
}

type popts struct {
	config   *command.PartyConfig
	realtime port.PartyRealtime   // nil: the recording double
	cache    port.PartyStateCache // nil: none
	presence port.PartyPresence   // nil: the settable double
	// F101
	lobby    port.LobbyStore      // nil: no lobby state
	game     port.GameActivities  // nil: module game offers no party activities
	lobbyCfg *command.LobbyConfig // nil: lobbyConfig
	statuses port.PresenceStatuses
	// F104: extra handlers mounted on the same server (matchmaking).
	extra func(e *penv, h *competitionhttp.Handlers)
}

func setupParty(t *testing.T, o popts) *penv {
	t.Helper()
	ctx := context.Background()
	cl := testkit.Isolated(ctx, t)
	e := &penv{t: t, app: open(t, cl.ModuleDSN("competition", "app")), platform: open(t, cl.ModuleDSN("competition", "platform")),
		relay: open(t, cl.ModuleDSN("competition", "relay")), clock: &pclock{now: time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)},
		push: &pushRec{}, online: &onlineSet{}, mod: &fakeModerator{}, tenantA: uuid.New(), tenantB: uuid.New(), tenantU: uuid.New(),
		schoolA: uuid.New(), schoolA2: uuid.New(), schoolB: uuid.New(), schoolU1: uuid.New(), schoolU2: uuid.New(), classA: uuid.New(),
		classA2: uuid.New()}
	migrator := open(t, cl.ModuleDSN("competition", "migrator"))
	var dbName string
	if err := migrator.QueryRowContext(ctx, `SELECT current_database()`).Scan(&dbName); err != nil {
		t.Fatal(err)
	}
	e.su = open(t, cl.DSN("postgres", dbName))
	w, err := outbox.NewWriter("competition", "core", nil)
	if err != nil {
		t.Fatal(err)
	}
	e.rec = outbox.NewAuditRecorder(w, e.app)
	tenancy.Configure(tenancy.Hooks{Recorder: e.rec})
	authz.Configure(authz.Hooks{Recorder: e.rec})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	dbs := entx.DBs{App: e.app, Platform: e.platform}
	if e.uow, err = entx.NewUnitOfWork(entx.Options{Module: "competition", DBs: dbs, Events: w, Audit: e.rec}); err != nil {
		t.Fatal(err)
	}
	if e.read, err = entx.NewReadSession("competition", dbs); err != nil {
		t.Fatal(err)
	}
	e.ptenant(e.tenantA, "school", nil)
	e.ptenant(e.tenantB, "school", nil)
	e.ptenant(e.tenantU, "unit", map[string]any{"cross_school_social_enabled": true})
	cfg := partyConfig
	if o.config != nil {
		cfg = *o.config
	}
	var rtp port.PartyRealtime = e.push
	if o.realtime != nil {
		rtp = o.realtime
	}
	var pres port.PartyPresence = e.online
	if o.presence != nil {
		pres = o.presence
	}
	e.parties = command.Parties{UoW: e.uow, AfterCommit: entx.AfterCommit, Repo: persistence.Parties{}, Directory: persistence.PartyDirectory{},
		Events: persistence.Publisher{W: w}, Audit: persistence.Auditor{R: e.rec}, Moderator: e.mod, Presence: pres, Realtime: rtp,
		Cache: o.cache, Metrics: observability.Party{}, Config: cfg, Now: e.clock.Now,
		Lobby: o.lobby, Activities: persistence.Activities{}, Areas: persistence.Areas{}, Game: o.game, LobbyMetrics: observability.Lobby{},
		LobbyConfig: lobbyConfig, Sweeps: persistence.Sweeps{}}
	if o.lobbyCfg != nil {
		e.parties.LobbyConfig = *o.lobbyCfg
	}
	e.parties.Statuses = e.online
	if o.statuses != nil {
		e.parties.Statuses = o.statuses
	}
	e.queries = query.Parties{Read: e.read, Reader: persistence.Parties{}, Presence: pres, Now: e.clock.Now, Lobby: o.lobby,
		LobbyReader: persistence.LobbyRead{}, Game: o.game, PrivateCapacity: lobbyConfig.PrivateCapacity}
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
	handlers := competitionhttp.Handlers{Commands: e.parties, Queries: e.queries}
	if o.extra != nil {
		o.extra(e, &handlers)
	}
	if err := competitionhttp.Register(s, handlers); err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(s)
	t.Cleanup(e.srv.Close)
	return e
}

func (e *penv) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.su.ExecContext(context.Background(), q, args...); err != nil {
		e.t.Fatalf("%s: %v", strings.Fields(q)[0], err)
	}
}

func (e *penv) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.su.QueryRowContext(context.Background(), q, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *penv) ptenant(id uuid.UUID, typ string, settings map[string]any) {
	e.t.Helper()
	b := []byte("{}")
	if settings != nil {
		b, _ = json.Marshal(settings)
	}
	e.exec(`INSERT INTO ref.tenants (tenant_id, tenant_type, status, settings_subset, source_version) VALUES ($1, $2, 'active', $3, 1)
		ON CONFLICT (tenant_id) DO UPDATE SET tenant_type = $2, settings_subset = $3`, id, typ, string(b))
}

// kid is a projected student with a public profile, a class and its game token.
type kid struct {
	id, tenant, school uuid.UUID
	class              uuid.UUID
	grade              int
	nickname           string
	token              string
}

// kid projects an active student (ref.students, ref.player_profiles, ref.enrollments) and registers its principal.
func (e *penv) kid(tenant, school, class uuid.UUID, grade int) kid {
	e.t.Helper()
	k := kid{id: uuid.New(), tenant: tenant, school: school, class: class, grade: grade, nickname: "Meo" + uuid.NewString()[:6]}
	e.exec(`INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, status, source_version) VALUES ($1, $2, $3, $4, 'active', 1)`,
		k.id, tenant, school, grade)
	e.exec(`INSERT INTO ref.player_profiles (student_id, tenant_id, nickname, level, avatar_config, source_version)
		VALUES ($1, $2, $3, 4, '{"body":"body_01","hair":"hair_02"}', 1)`, k.id, tenant, k.nickname)
	if class != uuid.Nil {
		e.exec(`INSERT INTO ref.enrollments (student_id, class_id, tenant_id, school_id, status, start_date, source_version)
			VALUES ($1, $2, $3, $4, 'active', '2026-09-05', 1)`, k.id, class, tenant, school)
	}
	tt := tenant
	sub := "kc-" + uuid.NewString()
	e.res.Set(sub, authz.PrincipalData{UserID: k.id, UserType: authz.UserTypeStudent, TenantID: &tt, TenantType: authz.TenantTypeSchool})
	k.token = e.idp.TokenSID(sub, httpxtest.GameClient, uuid.NewString(), time.Now())
	return k
}

// ghost is a student with a game token whose profile is not projected yet (identity event late).
func (e *penv) ghost(tenant uuid.UUID) kid {
	e.t.Helper()
	k := kid{id: uuid.New(), tenant: tenant, nickname: "ghost"}
	tt := tenant
	sub := "kc-" + uuid.NewString()
	e.res.Set(sub, authz.PrincipalData{UserID: k.id, UserType: authz.UserTypeStudent, TenantID: &tt, TenantType: authz.TenantTypeSchool})
	k.token = e.idp.TokenSID(sub, httpxtest.GameClient, uuid.NewString(), time.Now())
	return k
}

// kidA is a grade 6 student of school A, class A (no same-class rule at grade 6).
func (e *penv) kidA() kid { return e.kid(e.tenantA, e.schoolA, e.classA, 6) }

func (e *penv) friends(a, b kid) {
	e.t.Helper()
	lo, hi := a.id, b.id
	if lo.String() > hi.String() {
		lo, hi = hi, lo
	}
	e.exec(`INSERT INTO ref.friendships (tenant_id, user_low_id, user_high_id, active, source_version) VALUES ($1, $2, $3, true, 1)
		ON CONFLICT (user_low_id, user_high_id) DO UPDATE SET active = true`, a.tenant, lo, hi)
}

func (e *penv) block(blocker, blocked kid) {
	e.t.Helper()
	e.exec(`INSERT INTO ref.blocks (tenant_id, blocker_id, blocked_id, active, source_version) VALUES ($1, $2, $3, true, 1)
		ON CONFLICT (blocker_id, blocked_id) DO UPDATE SET active = true`, blocker.tenant, blocker.id, blocked.id)
}

func (e *penv) staffToken(tenant uuid.UUID, role string) string {
	sub := "kc-staff-" + uuid.NewString()
	e.res.Set(sub, authztest.Staff(tenant, role).PrincipalData)
	return e.idp.Token(sub, httpxtest.PortalClient, true, time.Now())
}

func (e *penv) platformToken(role string) string {
	sub := "kc-platform-" + uuid.NewString()
	e.res.Set(sub, authztest.Platform(role).PrincipalData)
	return e.idp.Token(sub, httpxtest.PortalClient, true, time.Now())
}

type presp struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (r presp) code() string {
	c, _ := r.body["code"].(string)
	return c
}

func (r presp) str(k string) string {
	s, _ := r.body[k].(string)
	return s
}

func (r presp) members() []map[string]any {
	items, _ := r.body["members"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (r presp) data() []map[string]any {
	items, _ := r.body["data"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func key() string { return uuid.Must(uuid.NewV7()).String() }

// call sends one request as k (token, X-Tenant-Id; Idempotency-Key when idem is not empty).
func (e *penv) call(k kid, method, path, idem string, body any) presp {
	e.t.Helper()
	h := map[string]string{"X-Tenant-Id": k.tenant.String()}
	if idem != "" {
		h["Idempotency-Key"] = idem
	}
	ct := ""
	if method == http.MethodPatch {
		ct = "application/merge-patch+json"
	}
	return e.do(method, path, k.token, h, body, ct)
}

func (e *penv) do(method, path, token string, headers map[string]string, body any, contentType string) presp {
	e.t.Helper()
	rd := bytes.NewReader(nil)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.srv.URL+"/api/v1"+path, rd)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
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
	out := presp{status: res.StatusCode, header: res.Header, raw: buf.String()}
	_ = json.Unmarshal(buf.Bytes(), &out.body)
	return out
}

// create is createParty by k; it fails the test unless the status is want.
func (e *penv) create(k kid, want int, body map[string]any) presp {
	e.t.Helper()
	if body == nil {
		body = map[string]any{"name": "Nhóm Sao Băng", "party_type": "friend"}
	}
	r := e.call(k, http.MethodPost, "/multiplayer/parties", key(), body)
	if r.status != want {
		e.t.Fatalf("create by %s: %d %s", k.nickname, r.status, r.raw)
	}
	return r
}

// invite is inviteToParty from a to b; it fails the test unless the status is want.
func (e *penv) invite(a kid, party string, b kid, want int) presp {
	e.t.Helper()
	r := e.call(a, http.MethodPost, "/multiplayer/parties/"+party+"/invitations", key(), map[string]any{"invitee_id": b.id})
	if r.status != want {
		e.t.Fatalf("invite %s → %s: %d %s", a.nickname, b.nickname, r.status, r.raw)
	}
	return r
}

// accept is acceptPartyInvitation by k.
func (e *penv) accept(k kid, invitation string, want int) presp {
	e.t.Helper()
	r := e.call(k, http.MethodPost, "/multiplayer/invitations/"+invitation+":accept", key(), nil)
	if r.status != want {
		e.t.Fatalf("accept by %s: %d %s", k.nickname, r.status, r.raw)
	}
	return r
}

// party returns a party of leader a with members b... (friends of a, invited and accepted).
func (e *penv) party(a kid, others ...kid) string {
	e.t.Helper()
	id := e.create(a, http.StatusCreated, map[string]any{"name": "Nhóm", "party_type": "friend", "max_members": 8}).str("party_id")
	for _, b := range others {
		e.friends(a, b)
		inv := e.invite(a, id, b, http.StatusCreated).str("invitation_id")
		e.accept(b, inv, http.StatusOK)
	}
	return id
}

// events returns the payloads of the outbox events of type typ, oldest first.
func (e *penv) events(typ string) []map[string]any {
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

// audits returns the audit.recorded events (action → reasons).
func (e *penv) audits(action string) []map[string]any {
	var out []map[string]any
	for _, p := range e.events("audit.recorded") {
		if a, _ := p["action"].(string); a == action {
			out = append(out, p)
		}
	}
	return out
}

// partyEventTypes are the events of module competition owned by F100 (specs/events/payloads/competition).
var partyEventTypes = []string{"party.created", "party.settings_updated", "party.invitation_sent", "party.invitation_accepted",
	"party.invitation_rejected", "party.invitation_expired", "party.invitation_cancelled", "party.member_joined", "party.member_left",
	"party.member_kicked", "party.leader_transferred", "party.disbanded"}

// checkEventSchemas validates every F100 event of the outbox against the envelope and payload schemas (contract first:
// module chat F117 and game F103 rely on them). Returns the number checked per type.
func (e *penv) checkEventSchemas() map[string]int { return e.checkSchemasOf(partyEventTypes) }

// checkSchemasOf validates the outbox events of the given types against the envelope and their payload schema.
func (e *penv) checkSchemasOf(types []string) map[string]int {
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
		payload, err := c.Compile(filepath.Join(root, "payloads", "competition", typ+".v1.schema.json"))
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

// history returns the event types of the party history of a party, oldest first.
func (e *penv) history(party string) []string {
	e.t.Helper()
	rows, err := e.su.QueryContext(context.Background(), `SELECT event_type FROM multiplayer.party_histories WHERE party_id = $1
		ORDER BY occurred_at, history_id`, party)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// noPII fails the test when a response leaks a real name, school, class or tenant field (R16).
func noPII(t *testing.T, r presp) {
	t.Helper()
	for _, k := range []string{"school_id", "school", "class", "class_id", "full_name", "display_name", "username", "grade_level",
		"tenant_id", "grade_band"} {
		if strings.Contains(r.raw, `"`+k+`"`) {
			t.Fatalf("response leaks %s: %s", k, r.raw)
		}
	}
}

func sortedIDs(ms []map[string]any, k string) []string {
	var out []string
	for _, m := range ms {
		if s, ok := m[k].(string); ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
