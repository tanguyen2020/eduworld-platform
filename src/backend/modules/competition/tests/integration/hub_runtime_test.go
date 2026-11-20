//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T022 (F091-AC-12…AC-15, AC-18, AC-19): moves within speed and bounds reach the other members in a snapshot bundle
// (droppable, only eid/p/r/a), a 50 m jump is corrected for the sender only; more than 20 violations per minute
// write one integrity signal and one audit event; hub.player.joined carries PublicPlayer only; an emote not on the
// wheel or faster than emote_rate is refused; game_profile.updated pushes hub.player.appearance_changed.
func TestHubRuntime(t *testing.T) {
	e := setup(t, envOpts{})
	e.node("core-1")
	an := e.student(e.tenantA, e.schoolA, 3)
	binh := e.student(e.tenantA, e.schoolA, 3)
	ra := e.mustJoin(an, "main_hub")
	out := e.channel(rt.RoomOutChannel(ra.RoomID))
	anUser := e.channel(rt.UserChannel(an.id))
	rb := e.mustJoin(binh, "main_hub")
	if rb.RoomID != ra.RoomID || len(rb.Players) != 1 || rb.Players[0].Player["nickname"] != an.nickname {
		t.Fatalf("binh assigned %+v", rb)
	}
	joined := next(t, out, "hub.player.joined", &an.id, 5*time.Second)
	b, _ := json.Marshal(joined)
	player := joined["player"].(map[string]any)
	for k := range player {
		if k != "user_id" && k != "nickname" && k != "avatar_config" && k != "level" {
			t.Fatalf("hub.player.joined.player has %s", k)
		}
	}
	for _, banned := range []string{"school", "class", "full_name", "username", e.schoolA.String()} {
		if strings.Contains(string(b), banned) {
			t.Fatalf("hub.player.joined contains %s", banned)
		}
	}

	// F091-AC-12, AC-14: a valid move reaches binh in a snapshot; an's own bundle item acknowledges client_seq.
	time.Sleep(200 * time.Millisecond)
	e.move(an, ra.RoomID, 1, [3]float64{1, 0, 1})
	snap := next(t, out, "hub.snapshot.updated", &binh.id, 5*time.Second)
	ents := snap["entities"].([]any)
	if len(ents) != 1 || ents[0].(map[string]any)["eid"] != float64(ra.SelfEID) {
		t.Fatalf("binh snapshot %v", snap)
	}
	for k := range ents[0].(map[string]any) {
		if k != "eid" && k != "p" && k != "r" && k != "a" {
			t.Fatalf("entity field %s", k)
		}
	}
	// F091-AC-12: 50 m in 100 ms → corrected to the last valid position, for the sender only.
	time.Sleep(100 * time.Millisecond)
	e.move(an, ra.RoomID, 2, [3]float64{51, 0, 1})
	c := next(t, anUser, "hub.player.corrected", nil, 5*time.Second)
	if c["reason"] != "speed" || c["client_seq"] != float64(2) || c["p"].([]any)[0] != float64(1) {
		t.Fatalf("corrected %v", c)
	}
	e.move(an, ra.RoomID, 3, [3]float64{1, 0, 250})
	if c := next(t, anUser, "hub.player.corrected", nil, 5*time.Second); c["reason"] != "out_of_bounds" {
		t.Fatalf("bounds %v", c)
	}
	// F091-AC-13: > 20 violations in a minute → one integrity signal + audit, no sanction (still a member).
	for i := int64(4); i < 30; i++ {
		e.move(an, ra.RoomID, i, [3]float64{100, 0, 100})
	}
	waitFor(t, 5*time.Second, "integrity signal", func() bool {
		return e.count(`SELECT count(*) FROM competition.integrity_signals WHERE student_id = $1 AND signal_type = 'hub_speed'
			AND tenant_id = $2 AND room_id = $3`, an.id, an.tenant, ra.RoomID) == 1
	})
	if n := e.count(`SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'hub.violation_threshold_exceeded'`); n != 1 {
		t.Fatalf("audit events %d", n)
	}
	// F091-AC-18: emote on the wheel → hub.player.emoted; again within emote_rate → RATE_LIMITED; not owned → denied.
	ca := an.caller("gw-1")
	if r := e.request("hub.player.emote", &ca, map[string]any{"room_id": ra.RoomID, "emote_id": "emote_wave"}); !r.OK {
		t.Fatalf("emote: %+v", r.Problem)
	}
	if em := next(t, out, "hub.player.emoted", nil, 5*time.Second); em["emote_id"] != "emote_wave" || em["eid"] != float64(ra.SelfEID) {
		t.Fatalf("emoted %v", em)
	}
	if r := e.request("hub.player.emote", &ca, map[string]any{"room_id": ra.RoomID, "emote_id": "emote_clap"}); r.OK ||
		r.Problem.Code != "RATE_LIMITED" || r.Problem.RetryAfterMs <= 0 {
		t.Fatalf("emote rate: %+v", r.Problem)
	}
	cb := binh.caller("gw-1")
	if r := e.request("hub.player.emote", &cb, map[string]any{"room_id": ra.RoomID, "emote_id": "emote_dance"}); r.OK ||
		r.Problem.Code != "PERMISSION_DENIED" {
		t.Fatalf("emote not owned: %+v", r.Problem)
	}

	// F091-AC-19: game_profile.updated (consumer competition.projection_game) → appearance_changed for the others.
	n := e.node("worker-like") // any instance reaches the owner over NATS
	gp := consumer.GameProjection{P: command.Projection{Refs: persistence.Refs{}}, Hub: n.hub}
	payload, _ := json.Marshal(map[string]any{"student_id": an.id, "tenant_id": an.tenant, "nickname": an.nickname, "level": 4,
		"avatar_config_hash": strings.Repeat("a", 64), "avatar_config": map[string]any{"body": "body_01", "hair": "hair_09",
			"emote_slots": []string{"emote_wave"}}, "source_version": 5})
	tenant := an.tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: "game_profile.updated", OccurredAt: time.Now(), TenantID: &tenant, Payload: payload}
	sc, _ := authz.EventScope(an.tenant)
	if err := e.uow.InTenantTx(context.Background(), sc, func(ctx context.Context) error { return gp.Handle(ctx, env) }); err != nil {
		t.Fatal(err)
	}
	ap := next(t, out, "hub.player.appearance_changed", &binh.id, 5*time.Second)
	if ap["eid"] != float64(ra.SelfEID) || ap["appearance"].(map[string]any)["hair"] != "hair_09" {
		t.Fatalf("appearance %v", ap)
	}
	if e.count(`SELECT count(*) FROM ref.player_profiles WHERE student_id = $1 AND level = 4 AND source_version = 5`, an.id) != 1 {
		t.Fatal("profile not projected")
	}
}

// T022 (F091-AC-17): no REST operation takes 3D movement — no path of the public specs mentions move/position/hub
// sync — and the simulation is not in Valkey (only registry, lease, members and last positions keys exist).
func TestNoRESTMovementAndNoValkeySimulation(t *testing.T) {
	root := testkit.RepoRoot(t)
	for _, f := range []string{"specs/openapi/eduworld-api.v1.yaml", "specs/openapi/modules/competition.v1.yaml", "specs/openapi/modules/game.v1.yaml"} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			l := strings.TrimSpace(line)
			if strings.HasPrefix(l, "/api/") && (strings.Contains(l, "move") || strings.Contains(l, "position") || strings.Contains(l, "snapshot")) {
				t.Errorf("%s: REST path for movement %s", f, l)
			}
		}
	}
	e := setup(t, envOpts{})
	e.node("core-1")
	an := e.student(e.tenantA, e.schoolA, 3)
	ra := e.mustJoin(an, "main_hub")
	e.move(an, ra.RoomID, 1, [3]float64{1, 0, 1})
	time.Sleep(1500 * time.Millisecond)
	keys, err := e.vk.Do(context.Background(), e.vk.B().Keys().Pattern("*").Build()).AsStrSlice()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if !strings.HasPrefix(k, "ew:competition:rt:") {
			t.Errorf("unexpected key %s", k)
		}
		for _, bad := range []string{"tick", "state", "snapshot", "sim"} {
			if strings.Contains(k, bad) {
				t.Errorf("simulation key %s", k)
			}
		}
	}
}

// T022 (F091-AC-20): when the instance holding the lease dies, another instance takes the room over at its next
// watch, restores the seats from the registry and sends hub.instance.migrated; inputs then flow to the new owner.
func TestHubFailover(t *testing.T) {
	e := setup(t, envOpts{})
	n1 := e.node("core-1")
	an := e.student(e.tenantA, e.schoolA, 3)
	binh := e.student(e.tenantA, e.schoolA, 3)
	ra := e.mustJoin(an, "main_hub")
	e.mustJoin(binh, "main_hub")
	out := e.channel(rt.RoomOutChannel(ra.RoomID))
	time.Sleep(1200 * time.Millisecond) // positions written (1 Hz)
	// Crash of core-1: no handover, the lease simply stops being renewed.
	n1.host.Crash()
	n1.stop()
	n2 := e.node("core-2")
	m := next(t, out, "hub.instance.migrated", nil, 10*time.Second)
	if m["room_id"] != ra.RoomID || m["old_room_id"] != ra.RoomID || m["reason"] != "failover" {
		t.Fatalf("migrated %v", m)
	}
	waitFor(t, 5*time.Second, "core-2 hosts the room", func() bool {
		for _, id := range n2.host.Hosted() {
			if id == ra.RoomID {
				return true
			}
		}
		return false
	})
	e.move(an, ra.RoomID, 1, [3]float64{1, 0, 1})
	snap := next(t, out, "hub.snapshot.updated", &binh.id, 5*time.Second)
	if len(snap["entities"].([]any)) == 0 {
		t.Fatalf("no state after failover: %v", snap)
	}
	if back := e.mustJoin(binh, "main_hub"); back.RoomID != ra.RoomID {
		t.Fatal("seat lost on failover")
	}
}
