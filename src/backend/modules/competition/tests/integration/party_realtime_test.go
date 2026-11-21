//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/subscriber"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T012, T021 (SC-002, research R1): with the real Valkey adapters, the invitee receives party.invitation.received on
// ew:rt:user:<invitee> within 1 s after the invitation; presence signals of the gateway (ew.rt.presence.*) make a
// member online for transferPartyLeader; the realtime party state ew:party:* follows the party and is dropped when it
// is disbanded.
func TestPartyRealtimeValkey(t *testing.T) {
	ctx := context.Background()
	_, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	_, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	presence := cache.PartyPresence{C: vk, TTL: 90 * time.Second}
	sctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go func() { _ = subscriber.PartyPresence(sctx, nc, presence) }()
	time.Sleep(200 * time.Millisecond)

	e := setupParty(t, popts{realtime: realtime.PartyPublisher{P: rt.Publisher{C: vk}}, cache: cache.PartyState{C: vk}, presence: presence})
	a, b := e.kidA(), e.kidA()
	e.friends(a, b)
	id := e.create(a, http.StatusCreated, nil).str("party_id")

	// Subscribe ew:rt:user:<b> like the gateway, then invite and time the delivery.
	got := make(chan rt.Envelope, 4)
	dedicated, release := vk.Dedicate()
	defer release()
	wait := dedicated.SetPubSubHooks(valkey.PubSubHooks{OnMessage: func(m valkey.PubSubMessage) {
		var env rt.Envelope
		if json.Unmarshal([]byte(m.Message), &env) == nil {
			got <- env
		}
	}})
	if err := dedicated.Do(ctx, dedicated.B().Subscribe().Channel(rt.UserChannel(b.id)).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	_ = wait
	start := time.Now()
	inv := e.invite(a, id, b, http.StatusCreated).str("invitation_id")
	select {
	case env := <-got:
		if env.Message == nil || env.Message.Type != "party.invitation.received" || !jsonHas(env.Message.Payload, inv) {
			t.Fatalf("message: %+v", env)
		}
		if d := time.Since(start); d > time.Second {
			t.Fatalf("delivered after %v (SC-002 ≤ 1 s)", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("party.invitation.received not delivered on ew:rt:user")
	}
	e.accept(b, inv, http.StatusOK)

	// Valkey party state (rebuildable): party hash, members, user keys.
	if v, err := vk.Do(ctx, vk.B().Get().Key(cache.PartyUserKey(b.id)).Build()).ToString(); err != nil || v != id {
		t.Fatalf("ew:party:user of b: %q %v", v, err)
	}
	if m, err := vk.Do(ctx, vk.B().Hgetall().Key(cache.PartyMembersKey(uuid.MustParse(id))).Build()).AsStrMap(); err != nil || len(m) != 2 ||
		m[a.id.String()] != "leader" {
		t.Fatalf("members hash: %v %v", m, err)
	}

	// Presence: b offline → transfer refused; a presence signal online → transfer done.
	path := "/multiplayer/parties/" + id + ":transfer-leader"
	if r := e.call(a, http.MethodPost, path, key(), map[string]any{"new_leader_id": b.id}); r.code() != "PARTY_MEMBER_NOT_ONLINE" {
		t.Fatalf("offline: %s", r.raw)
	}
	sig, _ := json.Marshal(rt.PresenceChanged{V: 1, UserID: b.id, TenantID: e.tenantA, Status: rt.PresenceOnline, Reason: rt.PresenceConnected,
		Ts: time.Now().UnixMilli()})
	if err := nc.Publish(rt.PresenceSubject(e.tenantA), sig); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		on, err := presence.Online(ctx, []uuid.UUID{b.id})
		if err != nil {
			t.Fatal(err)
		}
		if on[b.id] {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("presence signal not stored")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if r := e.call(a, http.MethodPost, path, key(), map[string]any{"new_leader_id": b.id}); r.status != http.StatusOK {
		t.Fatalf("online: %d %s", r.status, r.raw)
	}
	// An older signal never overrides a newer one; a lost connection is offline.
	old, _ := json.Marshal(rt.PresenceChanged{V: 1, UserID: b.id, TenantID: e.tenantA, Status: rt.PresenceOffline, Ts: 1})
	_ = nc.Publish(rt.PresenceSubject(e.tenantA), old)
	time.Sleep(200 * time.Millisecond)
	if on, _ := presence.Online(ctx, []uuid.UUID{b.id}); !on[b.id] {
		t.Fatal("stale signal overrode presence")
	}

	// Disband drops the party state.
	if r := e.call(b, http.MethodPost, "/multiplayer/parties/"+id+":disband", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	if n, err := vk.Do(ctx, vk.B().Exists().Key(cache.PartyKey(uuid.MustParse(id)), cache.PartyUserKey(a.id), cache.PartyUserKey(b.id)).Build()).AsInt64(); err != nil || n != 0 {
		t.Fatalf("party state after disband: %d %v", n, err)
	}
}

func jsonHas(raw json.RawMessage, s string) bool {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	for _, v := range m {
		if v == s {
			return true
		}
	}
	return false
}
