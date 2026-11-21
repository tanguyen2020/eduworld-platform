//go:build integration

// Package integration holds the integration tests of module competition (F091): migrated PostgreSQL
// (eduworld_competition), real Valkey and NATS (testcontainers), one or more room host instances (the production
// roomhost.Host and command.Hub) and NATS requests carrying the Ew-* headers exactly like the realtime gateway sends
// them. Projections are seeded directly in ref.* as the consumers would write them (their payload owners F071/F095 are
// not done yet, F091 T005).
package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	"github.com/nats-io/nats.go"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/roomhost"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

func TestMain(m *testing.M) { os.Exit(testkit.RunWithSharedCluster(m)) }

type env struct {
	t                          *testing.T
	app, platform, migrator    *sql.DB
	su                         *sql.DB // superuser of the test database: seeds and assertions across tenants (RLS FORCE)
	uow                        *entx.UnitOfWork
	rec                        *outbox.AuditRecorder
	vk                         valkey.Client
	nc                         *nats.Conn
	tenantA, tenantB           uuid.UUID
	schoolA, schoolA2, schoolB uuid.UUID
	opts                       envOpts
}

type envOpts struct {
	grace, idle, silence time.Duration
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

func setup(t *testing.T, o envOpts) *env {
	t.Helper()
	ctx := context.Background()
	cl := testkit.Isolated(ctx, t)
	e := &env{t: t, app: open(t, cl.ModuleDSN("competition", "app")), platform: open(t, cl.ModuleDSN("competition", "platform")),
		migrator: open(t, cl.ModuleDSN("competition", "migrator")), tenantA: uuid.New(), tenantB: uuid.New(), schoolA: uuid.New(),
		schoolA2: uuid.New(), schoolB: uuid.New(), opts: o}
	var dbName string
	if err := e.migrator.QueryRowContext(ctx, `SELECT current_database()`).Scan(&dbName); err != nil {
		t.Fatal(err)
	}
	e.su = open(t, cl.DSN("postgres", dbName))
	if e.opts.grace == 0 {
		e.opts.grace = 30 * time.Second
	}
	if e.opts.idle == 0 {
		e.opts.idle = 5 * time.Minute
	}
	w, err := outbox.NewWriter("competition", "core", nil)
	if err != nil {
		t.Fatal(err)
	}
	e.rec = outbox.NewAuditRecorder(w, e.app)
	tenancy.Configure(tenancy.Hooks{Recorder: e.rec})
	authz.Configure(authz.Hooks{Recorder: e.rec})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}); authz.Configure(authz.Hooks{}) })
	if e.uow, err = entx.NewUnitOfWork(entx.Options{Module: "competition", DBs: entx.DBs{App: e.app, Platform: e.platform}, Events: w,
		Audit: e.rec}); err != nil {
		t.Fatal(err)
	}
	_, addr := testkit.StartValkey(ctx, t)
	if e.vk, err = valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.vk.Close)
	_, url := testkit.StartNATS(ctx, t)
	if e.nc, err = nats.Connect(url); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.nc.Close)
	e.tenant(e.tenantA, nil)
	e.tenant(e.tenantB, nil)
	e.area("main_hub", 40, []string{"g1_3", "g4_5", "g6_7", "g8_9"}, false, "active")
	return e
}

// node is one room host instance (core-api replica).
type node struct {
	id     string
	host   *roomhost.Host
	hub    command.Hub
	cancel context.CancelFunc
	done   chan struct{}
}

// node starts a room host instance serving the hub commands (queue group competition) like core-api.
func (e *env) node(id string) *node {
	e.t.Helper()
	reg := cache.HubRegistry{C: e.vk}
	pub := rt.Publisher{C: e.vk}
	h := roomhost.New(roomhost.Config{InstanceID: id, TickHz: 10, Grace: e.opts.grace, IdleClose: e.opts.idle, Silence: e.opts.silence,
		EmoteRate: 2 * time.Second, LeaseTTL: 1500 * time.Millisecond, LeaseEvery: 300 * time.Millisecond, WatchEvery: 300 * time.Millisecond},
		e.nc, reg, pub, nil, nil, nil)
	hub := command.Hub{UoW: e.uow, Directory: persistence.Directory{}, Store: persistence.Store{}, Registry: reg, Rooms: h,
		Presence: realtime.Presence{P: pub}, Audit: persistence.Auditor{R: e.rec}, Metrics: observability.Hub{},
		Config:     command.HubConfig{Capacity: 40, TickHz: 10, Grace: e.opts.grace, IdleClose: e.opts.idle, EmoteRate: 2 * time.Second},
		PartyRooms: persistence.Activities{}}
	h.SetApp(hub, hub.LoadArea)
	ctx, cancel := context.WithCancel(context.Background())
	n := &node{id: id, host: h, hub: hub, cancel: cancel, done: make(chan struct{})}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); _ = h.Run(ctx) }()
	go func() {
		defer wg.Done()
		_ = rt.ServeCommand(ctx, e.nc, natscmd.Module, natscmd.TypeInstanceJoin, natscmd.InstanceJoin(hub), 5*time.Second)
	}()
	go func() {
		defer wg.Done()
		_ = rt.ServeCommand(ctx, e.nc, natscmd.Module, natscmd.TypePlayerEmote, natscmd.PlayerEmote(hub), 5*time.Second)
	}()
	go func() { wg.Wait(); close(n.done) }()
	e.t.Cleanup(n.stop)
	time.Sleep(100 * time.Millisecond) // subscriptions registered
	return n
}

func (n *node) stop() {
	n.cancel()
	select {
	case <-n.done:
	case <-time.After(5 * time.Second):
	}
}

func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.su.ExecContext(context.Background(), query, args...); err != nil {
		e.t.Fatalf("%s: %v", strings.Fields(query)[0], err)
	}
}

func (e *env) tenant(id uuid.UUID, settings map[string]any) {
	e.t.Helper()
	b, _ := json.Marshal(settings)
	if settings == nil {
		b = []byte("{}")
	}
	e.exec(`INSERT INTO ref.tenants (tenant_id, tenant_type, status, settings_subset, source_version) VALUES ($1, 'school', 'active', $2, 1)
		ON CONFLICT (tenant_id) DO UPDATE SET settings_subset = $2`, id, string(b))
}

func (e *env) area(id string, capacity int, bands []string, unlock bool, status string) {
	e.t.Helper()
	bb, _ := json.Marshal(bands)
	e.exec(`INSERT INTO ref.world_areas (area_id, capacity, max_speed_mps, bounds, spawn_points, allowed_grade_bands, requires_unlock,
		status, source_version) VALUES ($1, $2, 6, '{"min":[-200,-10,-200],"max":[200,50,200]}', '{"default":[0,0,0],"gate":[10,0,10]}',
		ARRAY(SELECT jsonb_array_elements_text($3::jsonb))::varchar(64)[], $4, $5, 1)
		ON CONFLICT (area_id) DO UPDATE SET capacity = $2, allowed_grade_bands = ARRAY(SELECT jsonb_array_elements_text($3::jsonb))::varchar(64)[],
		requires_unlock = $4, status = $5`, id, capacity, string(bb), unlock, status)
}

// student is a projected student with a public profile and the identity the gateway puts in the Ew-* headers.
type student struct {
	id, tenant, school, session, device uuid.UUID
	nickname                            string
}

func (s student) caller(node string) rt.Caller {
	return rt.Caller{TenantID: s.tenant, UserID: s.id, SchoolID: s.school, GameSessionID: s.session, DeviceID: s.device, ConnID: "c-" + s.id.String()[:8],
		NodeID: node}
}

func (e *env) student(tenant, school uuid.UUID, grade int) student {
	e.t.Helper()
	s := student{id: uuid.New(), tenant: tenant, school: school, session: uuid.New(), device: uuid.New(), nickname: "Meo" + uuid.NewString()[:6]}
	e.exec(`INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, status, source_version) VALUES ($1, $2, $3, $4, 'active', 1)`,
		s.id, tenant, school, grade)
	e.exec(`INSERT INTO ref.player_profiles (student_id, tenant_id, nickname, level, avatar_config, source_version)
		VALUES ($1, $2, $3, 3, '{"body":"body_01","hair":"hair_02","emote_slots":["emote_wave","emote_clap"]}', 1)`, s.id, tenant, s.nickname)
	return s
}

func (e *env) friends(a, b student) {
	lo, hi := a.id, b.id
	if strings.Compare(lo.String(), hi.String()) > 0 {
		lo, hi = hi, lo
	}
	e.exec(`INSERT INTO ref.friendships (tenant_id, user_low_id, user_high_id, active, source_version) VALUES ($1, $2, $3, true, 1)`, a.tenant, lo, hi)
}

func (e *env) block(blocker, blocked student) {
	e.exec(`INSERT INTO ref.blocks (tenant_id, blocker_id, blocked_id, active, source_version) VALUES ($1, $2, $3, true, 1)`,
		blocker.tenant, blocker.id, blocked.id)
}

// request sends a realtime command like the gateway (Ew-* headers; header=false: a client of another NATS user that
// forges nothing but has no headers).
func (e *env) request(typ string, c *rt.Caller, payload any) rt.Reply {
	e.t.Helper()
	m := nats.NewMsg(rt.CommandSubject("competition", typ))
	m.Data, _ = json.Marshal(payload)
	if c != nil {
		m.Header = c.Header(context.Background())
	}
	res, err := e.nc.RequestMsg(m, 8*time.Second)
	if err != nil {
		e.t.Fatalf("%s: %v", typ, err)
	}
	var r rt.Reply
	if err := json.Unmarshal(res.Data, &r); err != nil {
		e.t.Fatal(err)
	}
	return r
}

// assigned is the result of hub.instance.join.
type assigned struct {
	RoomID     string     `json:"room_id"`
	AreaID     string     `json:"area_id"`
	SelfEID    int        `json:"self_eid"`
	Spawn      [3]float64 `json:"spawn"`
	TickRateHz int        `json:"tick_rate_hz"`
	MaxSpeed   float64    `json:"max_speed_mps"`
	Players    []struct {
		EID    int            `json:"eid"`
		Player map[string]any `json:"player"`
	} `json:"players"`
}

func (e *env) join(s student, node string, payload map[string]any) (*assigned, rt.Reply) {
	e.t.Helper()
	c := s.caller(node)
	r := e.request("hub.instance.join", &c, payload)
	if !r.OK {
		return nil, r
	}
	var a assigned
	if err := json.Unmarshal(r.Result, &a); err != nil {
		e.t.Fatal(err)
	}
	return &a, r
}

func (e *env) mustJoin(s student, area string) *assigned {
	e.t.Helper()
	a, r := e.join(s, "gw-1", map[string]any{"area_id": area})
	if a == nil {
		e.t.Fatalf("join %s: %+v", area, r.Problem)
	}
	return a
}

// move sends hub.player.move on ew.in.room.<room_id> like the gateway (fire-and-forget).
func (e *env) move(s student, room string, seq int64, p [3]float64) {
	e.t.Helper()
	m := nats.NewMsg(rt.RoomInputSubject(room))
	m.Header = s.caller("gw-1").Header(context.Background())
	m.Data, _ = json.Marshal(map[string]any{"room_id": room, "client_seq": seq, "p": p, "r": 0.5, "a": "walk"})
	if err := e.nc.PublishMsg(m); err != nil {
		e.t.Fatal(err)
	}
}

// channel subscribes a Valkey Pub/Sub channel and returns the decoded envelopes.
func (e *env) channel(ch string) <-chan rt.Envelope {
	e.t.Helper()
	out := make(chan rt.Envelope, 4096)
	dc, release := e.vk.Dedicate()
	wait := dc.SetPubSubHooks(valkey.PubSubHooks{OnMessage: func(m valkey.PubSubMessage) {
		var env rt.Envelope
		if json.Unmarshal([]byte(m.Message), &env) == nil {
			select {
			case out <- env:
			default:
			}
		}
	}})
	if err := dc.Do(context.Background(), dc.B().Subscribe().Channel(ch).Build()).Error(); err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { release(); <-wait })
	return out
}

// next returns the next message of type typ (bundles: the item of user when set).
func next(t *testing.T, ch <-chan rt.Envelope, typ string, user *uuid.UUID, within time.Duration) map[string]any {
	t.Helper()
	timeout := time.After(within)
	for {
		select {
		case env := <-ch:
			var raw json.RawMessage
			switch {
			case env.Kind == rt.KindMessage && env.Message.Type == typ:
				if user != nil && env.Target != nil && len(env.Target.UserIDs) > 0 && !containsID(env.Target.UserIDs, *user) {
					continue
				}
				raw = env.Message.Payload
			case env.Kind == rt.KindBundle && env.Bundle.Type == typ:
				if user == nil {
					raw = env.Bundle.Items[0].Payload
				} else if raw = env.Bundle.Item(*user); raw == nil {
					continue
				}
			case env.Kind == rt.KindControl && typ == "control:"+env.Control.Action:
				b, _ := json.Marshal(env.Control)
				raw = b
			default:
				continue
			}
			var p map[string]any
			_ = json.Unmarshal(raw, &p)
			return p
		case <-timeout:
			t.Fatalf("no %s within %v", typ, within)
		}
	}
}

func none(t *testing.T, ch <-chan rt.Envelope, typ string, d time.Duration) {
	t.Helper()
	timeout := time.After(d)
	for {
		select {
		case env := <-ch:
			if (env.Kind == rt.KindMessage && env.Message.Type == typ) || (env.Kind == rt.KindBundle && env.Bundle.Type == typ) {
				t.Fatalf("unexpected %s", typ)
			}
		case <-timeout:
			return
		}
	}
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// count runs a count query as the superuser (sees every tenant).
func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.su.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func waitFor(t *testing.T, within time.Duration, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
