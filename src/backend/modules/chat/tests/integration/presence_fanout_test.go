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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/subscriber"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// inbox subscribes ew:rt:user:<id> (what the gateway delivers to the student) and returns the decoded envelopes.
func inbox(t *testing.T, vk valkey.Client, user uuid.UUID) <-chan rt.Envelope {
	t.Helper()
	out := make(chan rt.Envelope, 64)
	dc, release := vk.Dedicate()
	wait := dc.SetPubSubHooks(valkey.PubSubHooks{OnMessage: func(m valkey.PubSubMessage) {
		var env rt.Envelope
		if json.Unmarshal([]byte(m.Message), &env) == nil {
			out <- env
		}
	}})
	if err := dc.Do(context.Background(), dc.B().Subscribe().Channel(rt.UserChannel(user)).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { release(); <-wait })
	return out
}

// friendUpdate waits for presence.friend.updated and returns its payload.
func friendUpdate(t *testing.T, ch <-chan rt.Envelope, within time.Duration) map[string]any {
	t.Helper()
	select {
	case env := <-ch:
		if env.Kind != rt.KindMessage || env.Message.Type != "presence.friend.updated" {
			t.Fatalf("unexpected envelope %+v", env)
		}
		var p map[string]any
		if err := json.Unmarshal(env.Message.Payload, &p); err != nil {
			t.Fatal(err)
		}
		return p
	case <-time.After(within):
		t.Fatalf("no presence.friend.updated within %v", within)
	}
	return nil
}

func silent(t *testing.T, ch <-chan rt.Envelope, within time.Duration, who string) {
	t.Helper()
	select {
	case env := <-ch:
		t.Fatalf("%s received %+v", who, env.Message)
	case <-time.After(within):
	}
}

// T023 — F095-AC-20, AC-21, AC-22 (SC-004): a presence signal of the gateway reaches, as presence.friend.updated
// (user_id, status, activity, area_id), only the online friends of the student without a block, within 2 s; hidden →
// offline; during an exam the activity is generic; a refresh with no visible change sends nothing; listFriends shows
// the last presence received and offline without data (TTL presence_ttl).
func TestPresenceFanout(t *testing.T) {
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
	store := cache.Presence{C: vk, TTL: 90 * time.Second}
	friendsCache := cache.Friends{C: vk, TTL: time.Minute}
	e := setup(t, envOpts{presence: store, friends: friendsCache})
	fanout := command.PresenceFanout{Read: e.read, Relations: persistence.Reader{}, Store: store, Cache: friendsCache,
		Realtime: realtime.Publisher{P: rt.Publisher{C: vk}}, Metrics: observability.Social{}}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = subscriber.PresenceFanout(runCtx, nc, fanout) }()
	t.Cleanup(func() { stop(); <-done })

	an, binh, chi, dung := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA),
		e.student(e.tenantA, e.schoolA)
	blocked := e.student(e.tenantA, e.schoolA)
	e.befriend(an, binh)
	e.befriend(dung, binh) // dung is a friend of binh but stays offline
	// A leftover friendship with a live block (inconsistent on purpose) must never be used for the fan-out.
	e.befriend(blocked, binh)
	e.exec(`INSERT INTO social.blocks (block_id, tenant_id, blocker_id, blocked_id) VALUES ($1, $2, $3, $4)`, uuid.New(), e.tenantA, blocked.id, binh.id)
	time.Sleep(200 * time.Millisecond) // subscription registered

	ts := time.Now().UnixMilli()
	signal := func(s student, status, activity, area string, hidden bool, reason string) {
		ts++
		b, _ := json.Marshal(rt.PresenceChanged{V: 1, UserID: s.id, TenantID: s.tenant, Status: status, Activity: activity, AreaID: area,
			Hidden: hidden, Reason: reason, Ts: ts})
		if err := nc.Publish(rt.PresenceSubject(s.tenant), b); err != nil {
			t.Fatal(err)
		}
		_ = nc.Flush()
	}
	inAn, inChi, inDung, inBlocked := inbox(t, vk, an.id), inbox(t, vk, chi.id), inbox(t, vk, dung.id), inbox(t, vk, blocked.id)
	signal(an, "online", "idle", "main_hub", false, "connected")
	signal(chi, "online", "idle", "main_hub", false, "connected")
	signal(blocked, "online", "idle", "main_hub", false, "connected")
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	signal(binh, "online", "idle", "main_hub", false, "connected")
	p := friendUpdate(t, inAn, 2*time.Second)
	if lat := time.Since(start); lat > 2*time.Second {
		t.Fatalf("fan-out took %v", lat)
	}
	if p["user_id"] != binh.id.String() || p["status"] != "online" || p["activity"] != "idle" || p["area_id"] != "main_hub" {
		t.Fatalf("payload %v", p)
	}
	silent(t, inChi, 400*time.Millisecond, "a non-friend")
	silent(t, inDung, 10*time.Millisecond, "an offline friend")
	silent(t, inBlocked, 10*time.Millisecond, "a blocked friend")
	friends := e.call(an, http.MethodGet, "/social/friends", "", nil)
	if len(friends.data()) != 1 || friends.data()[0]["presence"] != "online" {
		t.Fatalf("listFriends presence: %s", friends.raw)
	}

	// A refresh with the same state → nothing (only the TTL is extended).
	signal(binh, "online", "idle", "main_hub", false, "refresh")
	silent(t, inAn, 400*time.Millisecond, "a friend on refresh")

	// Hidden → friends see offline (and listFriends too); a change while hidden sends nothing.
	signal(binh, "online", "idle", "main_hub", true, "status_set")
	if p := friendUpdate(t, inAn, 2*time.Second); p["status"] != "offline" || p["activity"] != nil || p["area_id"] != nil {
		t.Fatalf("hidden payload %v", p)
	}
	if r := e.call(an, http.MethodGet, "/social/friends", "", nil); r.data()[0]["presence"] != "offline" {
		t.Fatalf("listFriends of a hidden friend: %s", r.raw)
	}
	signal(binh, "in_match", "battle", "arena", true, "status_set")
	silent(t, inAn, 400*time.Millisecond, "a friend while hidden")

	// Exam: generic activity, no area (F095-AC-21).
	signal(binh, "in_activity", "exam", "exam_hall", false, "status_set")
	if p := friendUpdate(t, inAn, 2*time.Second); p["status"] != "in_activity" || p["activity"] != "learning" || p["area_id"] != nil {
		t.Fatalf("exam payload %v", p)
	}
	// An older signal than the stored one is ignored.
	ts -= 10
	signal(binh, "online", "idle", "main_hub", false, "connected")
	silent(t, inAn, 400*time.Millisecond, "a friend on an older signal")
	ts += 20

	// Offline at disconnection; listFriends offline.
	signal(binh, "offline", "", "", false, "disconnected")
	if p := friendUpdate(t, inAn, 2*time.Second); p["status"] != "offline" {
		t.Fatalf("offline payload %v", p)
	}
	// No data (TTL presence_ttl elapsed: the key expires) → offline in listFriends.
	signal(binh, "online", "idle", "main_hub", false, "connected")
	friendUpdate(t, inAn, 2*time.Second)
	if err := vk.Do(ctx, vk.B().Pexpire().Key("ew:chat:social:presence:"+binh.id.String()).Milliseconds(50).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if r := e.call(an, http.MethodGet, "/social/friends", "", nil); r.data()[0]["presence"] != "offline" {
		t.Fatalf("listFriends after the TTL: %s", r.raw)
	}
	// Removing the friend drops binh's cached friend list: the next change reaches nobody.
	if r := e.call(an, http.MethodDelete, "/social/friends/"+binh.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("remove: %d", r.status)
	}
	signal(binh, "in_match", "battle", "arena", false, "status_set")
	silent(t, inAn, 500*time.Millisecond, "a former friend")
}
