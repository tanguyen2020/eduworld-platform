package revocation_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/revocation"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

func msg(t *testing.T, typ string, payload map[string]any) rt.Envelope {
	t.Helper()
	e, err := rt.NewMessage(typ, payload, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return e
}

type tcase struct {
	name string
	e    rt.Envelope
	user bool
	want revocation.Action
}

// T029/T030/T042 decision table (F090-AC-24, AC-25, AC-26, AC-32).
func TestDecide(t *testing.T) {
	c := revocation.Conn{UserID: uuid.New(), GameSessionID: uuid.New(), ConnID: "c1"}
	other := uuid.New()
	mine := c.GameSessionID.String()
	cases := []tcase{
		{"notification", msg(t, "notification.item.created", map[string]any{"x": 1}), true, revocation.Action{Deliver: true}},
		{"replaced mine", msg(t, "session.session.replaced", map[string]any{"game_session_id": mine}).ForSession(c.GameSessionID), true,
			revocation.Action{Deliver: true, Close: 4409, ClearResume: true}},
		{"replaced other", msg(t, "session.session.replaced", map[string]any{"game_session_id": other.String()}).ForSession(other), true,
			revocation.Action{}},
		{"replaced other payload only", msg(t, "session.session.replaced", map[string]any{"game_session_id": other.String()}), true,
			revocation.Action{}},
		{"curfew mine", msg(t, "session.session.ended", map[string]any{"game_session_id": mine, "reason": "curfew"}), true,
			revocation.Action{Deliver: true, Close: 4410, ClearResume: true}},
		{"daily limit mine", msg(t, "session.session.ended", map[string]any{"game_session_id": mine, "reason": "daily_limit"}), true,
			revocation.Action{Deliver: true, Close: 4410, ClearResume: true}},
		{"curfew other", msg(t, "session.session.ended", map[string]any{"game_session_id": other.String(), "reason": "curfew"}).ForSession(other), true,
			revocation.Action{}},
		{"locked mine", msg(t, "session.session.ended", map[string]any{"game_session_id": mine, "reason": "account_locked"}), true,
			revocation.Action{Deliver: true, Close: 4403, ClearResume: true}},
		{"ended on a room channel", msg(t, "session.session.ended", map[string]any{"game_session_id": mine, "reason": "curfew"}), false,
			revocation.Action{}},
		{"other conn", rt.Envelope{V: 1, Kind: rt.KindControl, Target: &rt.Target{ConnID: "c2"},
			Control: &rt.Control{Action: "close", Code: 4409, Reason: "replaced"}}, true, revocation.Action{}},
		{"node replace", rt.Envelope{V: 1, Kind: rt.KindControl, Target: &rt.Target{ConnID: "c1"},
			Control: &rt.Control{Action: "close", Code: 4409, Reason: "replaced"}}, true, revocation.Action{Close: 4409}},
		{"control on room channel", rt.Envelope{V: 1, Kind: rt.KindControl, Control: &rt.Control{Action: "close", Code: 4403,
			Reason: "access_revoked"}}, false, revocation.Action{}},
		{"targeted users", rt.Envelope{V: 1, Kind: rt.KindMessage, Target: &rt.Target{UserIDs: []uuid.UUID{uuid.New()}},
			Message: &rt.Message{Type: "hub.snapshot.updated", Payload: []byte(`{}`)}}, false, revocation.Action{}},
	}
	for _, reason := range []string{"account_locked", "access_revoked", "cleanup", "admin", "password_changed"} {
		e, err := rt.Revoke(reason)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, tcase{"revoke " + reason, e, true, revocation.Action{EndedReason: reason, Close: 4403, ClearResume: true}})
	}
	tr, _ := rt.Revoke("transferred")
	cases = append(cases, tcase{"revoke transferred", tr, true, revocation.Action{EndedReason: "access_revoked", Close: 4403, ClearResume: true}})
	for _, tc := range cases {
		if got := revocation.Decide(tc.e, c, tc.user); got != tc.want {
			t.Fatalf("%s: got %+v want %+v", tc.name, got, tc.want)
		}
	}
}
