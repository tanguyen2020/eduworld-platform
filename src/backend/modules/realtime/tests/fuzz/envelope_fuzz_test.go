// Package fuzz holds the WebSocket fuzz tests of the gateway (F090 T038, SECURITY_ARCHITECTURE §16): random and
// oversized client frames and random Pub/Sub envelopes never panic, and nothing invalid is accepted.
package fuzz

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/protocol"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/revocation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/router"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// FuzzClientFrame: Parse never panics; an accepted frame always has a routed type, a usable id and v = 1.
func FuzzClientFrame(f *testing.F) {
	v, err := protocol.NewValidator()
	if err != nil {
		f.Fatal(err)
	}
	id := "01JCX4Z7Q2Z7N3M5V9W1A2B3C4"
	for _, s := range []string{
		`{"v":1,"type":"system.heartbeat.ping","id":"` + id + `","payload":{"client_time":1}}`,
		`{"v":1,"type":"hub.player.move","id":"` + id + `","payload":{"room_id":"r","client_seq":1,"p":[1,2,3],"r":0}}`,
		`{"v":1,"type":"chat.message.send","id":"` + id + `","payload":{"channel_id":"` + uuid.NewString() + `","kind":"text","body":"` + strings.Repeat("x", 400) + `"}}`,
		`{"v":1,"type":"system.session.resume","id":"` + id + `","payload":{"resume_token":"t","last_seq":-1}}`,
		`[]`, `null`, `{"v":1}`, "\x00\xff", `{"v":1,"type":"a.b.c","id":"` + id + `","payload":{"__proto__":{}}}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		env, rej := v.Parse(frame)
		if rej != nil {
			if rej.Code != protocol.CodeValidationFailed && rej.Code != protocol.CodeUnknownType {
				t.Fatalf("code %s", rej.Code)
			}
			return
		}
		if _, ok := router.Lookup(env.Type); !ok || env.V != 1 || len(env.ID) < 10 || len(env.ID) > 40 {
			t.Fatalf("accepted invalid envelope %+v", env)
		}
	})
}

// FuzzPubSubEnvelope: a random Pub/Sub message never panics the decision; anything that is not a valid envelope
// is dropped and a control is honored only with a known close code.
func FuzzPubSubEnvelope(f *testing.F) {
	c := revocation.Conn{UserID: uuid.New(), GameSessionID: uuid.New(), ConnID: "c-1"}
	for _, s := range []string{
		`{"v":1,"kind":"control","control":{"action":"close","code":4403,"reason":"admin"}}`,
		`{"v":1,"kind":"message","message":{"type":"session.session.ended","payload":{"game_session_id":"` + c.GameSessionID.String() + `","reason":"curfew"}}}`,
		`{"v":1,"kind":"control","control":{"action":"close","code":1000,"reason":"x"}}`, `{}`, `nope`,
	} {
		f.Add([]byte(s), true)
	}
	f.Fuzz(func(t *testing.T, data []byte, user bool) {
		var e rt.Envelope
		if json.Unmarshal(data, &e) != nil || e.Validate() != nil {
			return
		}
		a := revocation.Decide(e, c, user)
		switch a.Close {
		case 0, rt.CloseRevoked, rt.CloseReplaced, rt.CloseSessionDone:
		default:
			t.Fatalf("close code %d", a.Close)
		}
		if !user && (a.Close != 0 || a.EndedReason != "") {
			t.Fatalf("control honored outside the user channel: %+v", a)
		}
	})
}
