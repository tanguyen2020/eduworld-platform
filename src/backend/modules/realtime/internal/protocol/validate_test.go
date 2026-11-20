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
