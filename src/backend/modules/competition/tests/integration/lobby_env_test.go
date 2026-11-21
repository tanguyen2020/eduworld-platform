//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// lobbyConfig is the CONFIG_DEFAULTS of the lobby (party_lobby_cmd_rate 5/s, private_instance_capacity 8,
// party_leader_grace 60 s, party_dormant_timeout 30 min, disconnect_grace private_instance 120 s,
// party_activity_min_members_timeout 120 s).
var lobbyConfig = command.LobbyConfig{CmdPerSecond: 5, PrivateCapacity: 8, LeaderGrace: time.Minute, DormantTimeout: 30 * time.Minute,
	MemberGrace: 2 * time.Minute, MinMembersTimeout: 2 * time.Minute, SweepBatch: 100}

// Statuses makes the settable presence double a port.PresenceStatuses (TEST DOUBLE — R17): online or no signal.
func (o *onlineSet) Statuses(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if o.on[id] {
			out[id] = "online"
		}
	}
	return out, nil
}

// valkeyLobby starts a Valkey for one test and returns the real lobby store and the client.
func valkeyLobby(t *testing.T) (cache.LobbyStore, valkey.Client) {
	t.Helper()
	_, addr := testkit.StartValkey(context.Background(), t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	return cache.LobbyStore{C: vk}, vk
}

// fakeGame is a MOCK of the game facade for party activities (startPartyActivity, checkPartyActivityEligibility,
// listPartyActivityCandidates — owner F103, R17): quest "q-bridge" (2–4 members, private_instance in area "forest"),
// boss "b-golem" (grades 4 only: everybody else grade_not_allowed), boss "b-slime" (everybody, 2–4 members). It records the calls and the scope they carried.
type fakeGame struct {
	mu       sync.Mutex
	down     bool
	starts   map[uuid.UUID]uuid.UUID // party_activity_id → external_ref
	startN   int
	checkN   int
	scopes   []uuid.UUID // session tenant of each call (F101-AC-20)
	activity []uuid.UUID // party_activity_id of each start
	onStart  func()      // runs after a successful start (simulates a change of the party before the commit)
}

func (g *fakeGame) record(ctx context.Context) {
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if id, ok := s.SessionTenantID(); ok {
			g.scopes = append(g.scopes, id)
		}
	}
}

func (g *fakeGame) checks(typ, ref string, ids []uuid.UUID) party.Eligibility {
	e := party.Eligibility{MinMembers: 2, MaxMembers: 4, AllowContinue: true, AreaID: "forest"}
	for _, id := range ids {
		c := party.Check{StudentID: id, Eligible: true}
		if typ == party.ActivityBoss && ref == "b-golem" {
			c = party.Check{StudentID: id, Eligible: false, Reason: party.ReasonGradeNotAllowed, Requirement: "grade_levels:4"}
		}
		e.Members = append(e.Members, c)
	}
	return e
}

func (g *fakeGame) Candidates(ctx context.Context, _ uuid.UUID, ids []uuid.UUID) ([]port.GameCandidate, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.record(ctx)
	if g.down {
		return nil, port.ErrGameUnavailable
	}
	return []port.GameCandidate{
		{Type: party.ActivityQuest, Ref: "q-bridge", Title: map[string]string{"vi": "Sửa cầu"}, Eligibility: g.checks(party.ActivityQuest, "q-bridge", ids)},
		{Type: party.ActivityBoss, Ref: "b-golem", Title: map[string]string{"vi": "Người đá"}, Eligibility: g.checks(party.ActivityBoss, "b-golem", ids)},
	}, nil
}

func (g *fakeGame) Check(ctx context.Context, _ uuid.UUID, typ, ref string, ids []uuid.UUID) (party.Eligibility, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.record(ctx)
	g.checkN++
	if g.down {
		return party.Eligibility{}, port.ErrGameUnavailable
	}
	if ref != "q-bridge" && ref != "b-golem" && ref != "b-slime" {
		return party.Eligibility{}, port.ErrGameRefused
	}
	return g.checks(typ, ref, ids), nil
}

func (g *fakeGame) Start(ctx context.Context, s port.GameStart) (port.GameStarted, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.record(ctx)
	g.startN++
	g.activity = append(g.activity, s.ActivityID)
	if g.down {
		return port.GameStarted{}, port.ErrGameUnavailable
	}
	if g.starts == nil {
		g.starts = map[uuid.UUID]uuid.UUID{}
	}
	ext, ok := g.starts[s.ActivityID]
	if !ok {
		ext = uuid.New()
		g.starts[s.ActivityID] = ext
	}
	if g.onStart != nil {
		g.onStart()
	}
	if s.Type == party.ActivityBoss {
		return port.GameStarted{ExternalRef: ext, RoomType: party.RoomBoss, SceneRef: "boss/golem"}, nil
	}
	return port.GameStarted{ExternalRef: ext, RoomType: party.RoomPrivateInstance, AreaID: "forest", SceneRef: "quest/bridge"}, nil
}

// area projects a world area (ref.world_areas of module game).
func (e *penv) area(id string, bands []string, unlock bool) {
	e.t.Helper()
	bb, _ := json.Marshal(bands)
	e.exec(`INSERT INTO ref.world_areas (area_id, capacity, max_speed_mps, bounds, spawn_points, allowed_grade_bands, requires_unlock,
		status, source_version) VALUES ($1, 40, 6, '{"min":[-200,-10,-200],"max":[200,50,200]}', '{"default":[0,0,0]}',
		ARRAY(SELECT jsonb_array_elements_text($2::jsonb))::varchar(64)[], $3, 'active', 1)
		ON CONFLICT (area_id) DO UPDATE SET allowed_grade_bands = ARRAY(SELECT jsonb_array_elements_text($2::jsonb))::varchar(64)[],
		requires_unlock = $3, status = 'active'`, id, string(bb), unlock)
}

// unlock projects an unlocked area of a student (ref.area_unlocks).
func (e *penv) unlock(k kid, area string) {
	e.t.Helper()
	e.exec(`INSERT INTO ref.area_unlocks (tenant_id, student_id, area_id, unlocked_at, source_version) VALUES ($1, $2, $3, now(), 1)`,
		k.tenant, k.id, area)
}

// version returns the state_version of a party (getParty by k).
func (e *penv) version(k kid, partyID string) int64 {
	e.t.Helper()
	r := e.call(k, http.MethodGet, "/multiplayer/parties/"+partyID, "", nil)
	if r.status != http.StatusOK {
		e.t.Fatalf("getParty: %d %s", r.status, r.raw)
	}
	v, _ := r.body["state_version"].(float64)
	return int64(v)
}

// ready is setPartyReady by k.
func (e *penv) ready(k kid, partyID string, ready bool) presp {
	e.t.Helper()
	return e.call(k, http.MethodPut, "/multiplayer/parties/"+partyID+"/members/me/ready", "", map[string]any{"ready": ready})
}

// pick is selectPartyActivity by k with the current state_version (expected < 0) or the given one.
func (e *penv) pick(k kid, partyID, typ, ref string, expected int64) presp {
	e.t.Helper()
	if expected < 0 {
		expected = e.version(k, partyID)
	}
	return e.call(k, http.MethodPut, "/multiplayer/parties/"+partyID+"/activity", "", map[string]any{"activity_type": typ,
		"activity_ref": ref, "expected_version": expected})
}

// startSession is startPartySession by k with the current state_version.
func (e *penv) startSession(k kid, partyID, idem string) presp {
	e.t.Helper()
	return e.call(k, http.MethodPost, "/multiplayer/parties/"+partyID+":start", idem, map[string]any{"expected_version": e.version(k, partyID)})
}

// lobbyReady selects an activity and makes every given member online and Ready.
func (e *penv) lobbyReady(partyID, typ, ref string, leader kid, others ...kid) {
	e.t.Helper()
	for _, k := range append([]kid{leader}, others...) {
		e.online.set(k.id, true)
	}
	if r := e.pick(leader, partyID, typ, ref, -1); r.status != http.StatusOK {
		e.t.Fatalf("select %s/%s: %d %s", typ, ref, r.status, r.raw)
	}
	for _, k := range append([]kid{leader}, others...) {
		if r := e.ready(k, partyID, true); r.status != http.StatusNoContent {
			e.t.Fatalf("ready %s: %d %s", k.nickname, r.status, r.raw)
		}
	}
}

func (e *penv) one(q string, args ...any) string {
	e.t.Helper()
	var s string
	if err := e.su.QueryRowContext(context.Background(), q, args...).Scan(&s); err != nil {
		e.t.Fatalf("%s: %v", q, err)
	}
	return s
}

func str(v any) string { return fmt.Sprint(v) }

// lobbyEventTypes are the events of module competition owned by F101 (specs/events/payloads/competition).
var lobbyEventTypes = []string{"party.activity_selected", "party.activity_started", "party.activity_ended", "party.activity_completed"}

// checkLobbyEventSchemas validates every F101 event of the outbox (and party.leader_transferred / party.disbanded that F101
// also emits) against the envelope and payload schemas; returns the number checked per type.
func checkLobbyEventSchemas(t *testing.T, e *penv) map[string]int {
	t.Helper()
	root := filepath.Join(testkit.RepoRoot(t), "specs")
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	raw, err := os.ReadFile(filepath.Join(root, "schemas", "common", "enums.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	enums, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("https://schemas.eduworld.vn/common/enums.schema.json", enums); err != nil {
		t.Fatal(err)
	}
	envelope, err := c.Compile(filepath.Join(root, "events", "envelope.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, typ := range append(append([]string{}, lobbyEventTypes...), "party.leader_transferred", "party.disbanded") {
		payload, err := c.Compile(filepath.Join(root, "events", "payloads", "competition", typ+".v1.schema.json"))
		if err != nil {
			t.Fatal(err)
		}
		rows, err := e.relay.QueryContext(context.Background(), `SELECT envelope FROM infra.outbox_events WHERE event_type = $1`, typ)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var b []byte
			if err := rows.Scan(&b); err != nil {
				t.Fatal(err)
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			if err := envelope.Validate(doc); err != nil {
				t.Errorf("%s envelope: %v", typ, err)
			}
			var env struct {
				Payload json.RawMessage `json:"payload"`
			}
			_ = json.Unmarshal(b, &env)
			pd, err := jsonschema.UnmarshalJSON(bytes.NewReader(env.Payload))
			if err != nil {
				t.Fatal(err)
			}
			if err := payload.Validate(pd); err != nil {
				t.Errorf("%s payload: %v", typ, err)
			}
			seen[typ]++
		}
		_ = rows.Close()
	}
	return seen
}

// partyRef is the sweeper reference of a party.
func partyRef(e *penv, id string) port.PartyRef {
	e.t.Helper()
	return port.PartyRef{TenantID: uuid.MustParse(e.one(`SELECT tenant_id::text FROM multiplayer.parties WHERE party_id = $1`, id)),
		PartyID: uuid.MustParse(id)}
}
