package protocol_test

import (
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/protocol"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/router"
)

// T025 (F090-AC-20): every routed type has a schema; a valid command passes; unknown type, broken envelope,
// a string over maxLength of the AsyncAPI and a bad enum are rejected with the right code and ref.
func TestValidator(t *testing.T) {
	v, err := protocol.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	for typ := range router.Routes {
		if !v.Known(typ) {
			t.Fatalf("route %s has no schema", typ)
		}
	}
	id := "01JCX4Z7Q2Z7N3M5V9W1A2B3C4"
	ok := `{"v":1,"type":"hub.instance.join","id":"` + id + `","payload":{"area_id":"main_hub","tenant_id":"x"}}`
	if env, rej := v.Parse([]byte(ok)); rej != nil || env.Type != "hub.instance.join" || env.ID != id {
		t.Fatalf("valid: %+v %+v", env, rej)
	}
	cases := map[string]struct{ frame, code, ref string }{
		"not json":     {`{"v":1,`, protocol.CodeValidationFailed, ""},
		"array":        {`[1,2]`, protocol.CodeValidationFailed, ""},
		"unknown type": {`{"v":1,"type":"foo.bar.baz","id":"` + id + `","payload":{}}`, protocol.CodeUnknownType, id},
		"no type":      {`{"v":1,"id":"` + id + `","payload":{}}`, protocol.CodeUnknownType, id},
		"max length": {`{"v":1,"type":"hub.instance.join","id":"` + id + `","payload":{"area_id":"` + strings.Repeat("a", 65) + `"}}`,
			protocol.CodeValidationFailed, id},
		"missing required": {`{"v":1,"type":"hub.instance.join","id":"` + id + `","payload":{}}`, protocol.CodeValidationFailed, id},
		"bad version":      {`{"v":2,"type":"hub.instance.join","id":"` + id + `","payload":{"area_id":"a"}}`, protocol.CodeValidationFailed, id},
		"extra envelope field": {`{"v":1,"type":"hub.instance.join","id":"` + id + `","tenant_id":"t","payload":{"area_id":"a"}}`,
			protocol.CodeValidationFailed, id},
		"short id": {`{"v":1,"type":"hub.instance.join","id":"x","payload":{"area_id":"a"}}`, protocol.CodeValidationFailed, ""},
		"bad enum": {`{"v":1,"type":"presence.status.set","id":"` + id + `","payload":{"status":"dancing"}}`, protocol.CodeValidationFailed, id},
		"long resume token": {`{"v":1,"type":"system.session.resume","id":"` + id + `","payload":{"resume_token":"` + strings.Repeat("r", 513) + `","last_seq":1}}`,
			protocol.CodeValidationFailed, id},
	}
	for name, c := range cases {
		_, rej := v.Parse([]byte(c.frame))
		if rej == nil || rej.Code != c.code || rej.Ref != c.ref {
			t.Fatalf("%s: %+v", name, rej)
		}
	}
}

// F115 T043 (OQ F090-1): chat.sync became chat.channel.sync, a 3-segment type that passes the ClientEnvelope pattern and
// is routed to module chat; the old name is unknown.
func TestChatChannelSyncRouted(t *testing.T) {
	v, err := protocol.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	id := "01JCX4Z7Q2Z7N3M5V9W1A2B3C4"
	frame := `{"v":1,"type":"chat.channel.sync","id":"` + id + `","payload":{"channels":{"0193a1aa-0000-7000-8000-000000000020":12}}}`
	if env, rej := v.Parse([]byte(frame)); rej != nil || env.Type != "chat.channel.sync" {
		t.Fatalf("chat.channel.sync rejected: %+v", rej)
	}
	if r, ok := router.Lookup("chat.channel.sync"); !ok || r.Module != "chat" || r.Kind != router.KindRequest {
		t.Fatalf("route: %+v %v", r, ok)
	}
	if _, ok := router.Lookup("chat.sync"); ok {
		t.Fatal("chat.sync still routed")
	}
	old := `{"v":1,"type":"chat.sync","id":"` + id + `","payload":{"channels":{}}}`
	if _, rej := v.Parse([]byte(old)); rej == nil || rej.Code != protocol.CodeUnknownType {
		t.Fatalf("chat.sync: %+v", rej)
	}
	send := `{"v":1,"type":"chat.message.send","id":"` + id + `","payload":{"channel_id":"0193a1aa-0000-7000-8000-000000000020","kind":"text","body":"x"}}`
	if _, rej := v.Parse([]byte(send)); rej == nil || rej.Code != protocol.CodeValidationFailed {
		t.Fatalf("chat.message.send without client_message_id: %+v", rej)
	}
}
