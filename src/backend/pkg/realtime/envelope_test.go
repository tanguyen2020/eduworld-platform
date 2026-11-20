package realtime_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/santhosh-tekuri/jsonschema/v6"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

func schema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "specs", "schemas", "realtime", "rt-pubsub-envelope.schema.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("mem://env.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("mem://env.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Contract (T003/T006): every envelope built by pkg/realtime is valid against rt-pubsub-envelope.schema.json, and
// Validate refuses what the schema refuses.
func TestEnvelopeMatchesSchema(t *testing.T) {
	s := schema(t)
	session := uuid.New()
	m, err := rt.NewMessage("session.session.replaced", map[string]any{"game_session_id": session.String(), "replaced_at": 1}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	revoke, err := rt.Revoke("password_changed")
	if err != nil {
		t.Fatal(err)
	}
	node := rt.Envelope{V: 1, Kind: rt.KindControl, Target: &rt.Target{ConnID: "c-1"},
		Control: &rt.Control{Action: "close", Code: rt.CloseReplaced, Reason: "replaced"}}
	// F091: presence.update control and the per-node snapshot bundle.
	presence, err := rt.PresenceUpdate("main_hub", "room-1")
	if err != nil {
		t.Fatal(err)
	}
	left, err := rt.PresenceUpdate("", "")
	if err != nil {
		t.Fatal(err)
	}
	bundle := rt.Envelope{V: 1, Kind: rt.KindBundle, Droppable: true, Bundle: &rt.Bundle{Type: "hub.snapshot.updated", Ts: 1,
		Items: []rt.BundleItem{{UserID: session, Payload: []byte(`{"room_id":"room-1","tick":1,"entities":[]}`)}}}}
	if bundle.Bundle.Item(session) == nil || bundle.Bundle.Item(uuid.New()) != nil {
		t.Fatal("bundle item lookup")
	}
	for i, e := range []rt.Envelope{m.ForSession(session), revoke, node, presence, left, bundle} {
		if err := e.Validate(); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		b, _ := json.Marshal(e)
		inst, _ := jsonschema.UnmarshalJSON(bytesReader(b))
		if err := s.Validate(inst); err != nil {
			t.Fatalf("%d schema: %v", i, err)
		}
	}
	bad := []rt.Envelope{
		{V: 2, Kind: rt.KindMessage, Message: &rt.Message{Type: "a.b.c", Payload: []byte(`{}`)}},
		{V: 1, Kind: rt.KindMessage},
		{V: 1, Kind: rt.KindControl, Control: &rt.Control{Action: "close", Code: 4403, Reason: "replaced"}},
		{V: 1, Kind: rt.KindMessage, Message: &rt.Message{Type: "Bad", Payload: []byte(`{}`)}},
		{V: 1, Kind: rt.KindControl, Control: &rt.Control{Action: rt.ActionPresenceUpdate}},
		{V: 1, Kind: rt.KindControl, Control: &rt.Control{Action: rt.ActionPresenceUpdate, Code: 4403, Presence: &rt.Presence{}}},
		{V: 1, Kind: rt.KindBundle, Bundle: &rt.Bundle{Type: "hub.snapshot.updated"}},
		{V: 1, Kind: rt.KindBundle, Bundle: &rt.Bundle{Type: "hub.snapshot.updated", Items: []rt.BundleItem{{Payload: []byte(`{}`)}}}},
	}
	for i, e := range bad {
		if e.Validate() == nil {
			t.Fatalf("bad %d accepted", i)
		}
	}
	if _, err := rt.Revoke("curfew"); err == nil {
		t.Fatal("curfew is not a 4403 reason")
	}
}

// Topics a module may grant, ids safe in channel names and subjects.
func TestParseTopic(t *testing.T) {
	u := uuid.New()
	cases := map[string]rt.TopicKind{
		rt.UserChannel(u): rt.TopicUser, rt.PartyChannel("p1"): rt.TopicParty, rt.RoomOutChannel("r-1"): rt.TopicRoom,
		rt.ChatFanoutChannel("ch_1"): rt.TopicChat, rt.BroadcastChannel: rt.TopicBroadcast, rt.NodeChannel("gw-1"): rt.TopicInvalid,
		"ew:rt:room:a.b:out": rt.TopicInvalid, "ew:rt:user:not-a-uuid": rt.TopicInvalid, "ew:chat:fanout:": rt.TopicInvalid,
		"ew:rt:room:x": rt.TopicInvalid, "other": rt.TopicInvalid,
	}
	for topic, want := range cases {
		if got, _ := rt.ParseTopic(topic); got != want {
			t.Fatalf("%s: %v want %v", topic, got, want)
		}
	}
}

// Headers round trip; a missing header is refused (handlers never trust partial identity).
func TestCallerHeaders(t *testing.T) {
	c := rt.Caller{TenantID: uuid.New(), UserID: uuid.New(), GameSessionID: uuid.New(), DeviceID: uuid.New(), ConnID: "c-1", NodeID: "gw-1"}
	got, err := rt.CallerFrom(c.Header(t.Context()))
	if err != nil || got.TenantID != c.TenantID || got.UserID != c.UserID || got.ConnID != "c-1" || got.NodeID != "gw-1" {
		t.Fatalf("%+v %v", got, err)
	}
	h := c.Header(t.Context())
	h.Del(rt.HeaderTenantID)
	if _, err := rt.CallerFrom(h); err == nil {
		t.Fatal("missing tenant accepted")
	}
	if _, err := rt.CallerFrom(nats.Header{}); err == nil {
		t.Fatal("empty header accepted")
	}
	if rt.CommandSubject("chat", "chat.message.send") != "ew.cmd.chat.chat.message.send" || rt.RoomInputSubject("r1") != "ew.in.room.r1" {
		t.Fatal("subjects")
	}
}
