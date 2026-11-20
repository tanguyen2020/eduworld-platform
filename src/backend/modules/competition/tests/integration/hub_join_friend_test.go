//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

// T013 (F091-AC-05, AC-MP-010): "Tham gia bạn" seats the student in the friend's instance when they are friends, no
// block either way, same partition and area, a seat is free; otherwise ROOM_FULL or PERMISSION_DENIED, and the answer
// never carries the friend's room.
func TestHubJoinFriend(t *testing.T) {
	e := setup(t, envOpts{})
	e.node("core-1")
	e.area("learning_zone.math", 2, []string{"g1_3"}, false, "active")
	an := e.student(e.tenantA, e.schoolA, 3)
	binh := e.student(e.tenantA, e.schoolA, 3)
	chi := e.student(e.tenantA, e.schoolA, 3)
	stranger := e.student(e.tenantA, e.schoolA, 3)
	otherSchool := e.student(e.tenantA, e.schoolA2, 3)
	e.friends(an, binh)
	e.friends(an, chi)
	e.friends(an, otherSchool)

	ra := e.mustJoin(an, "main_hub")
	follow := func(s student, friend student, area string) (*assigned, string, string) {
		t.Helper()
		a, r := e.join(s, "gw-1", map[string]any{"area_id": area, "follow_user_id": friend.id.String()})
		if a != nil {
			return a, "", ""
		}
		b, _ := json.Marshal(r)
		return nil, r.Problem.Code, string(b)
	}
	if a, code, _ := follow(binh, an, "main_hub"); a == nil || a.RoomID != ra.RoomID {
		t.Fatalf("friend follow: %s", code)
	}
	for name, s := range map[string]student{"not friends": stranger, "other school": otherSchool} {
		a, code, raw := follow(s, an, "main_hub")
		if a != nil || code != "PERMISSION_DENIED" || strings.Contains(raw, ra.RoomID) {
			t.Errorf("%s: %s %s", name, code, raw)
		}
	}
	// blocked (either way) → denied
	e.block(an, chi)
	if a, code, _ := follow(chi, an, "main_hub"); a != nil || code != "PERMISSION_DENIED" {
		t.Fatalf("blocked follow: %s", code)
	}
	// friend in another area → denied (no location disclosed)
	if a, code, raw := follow(binh, an, "learning_zone.math"); a != nil || code != "PERMISSION_DENIED" || strings.Contains(raw, ra.RoomID) {
		t.Fatalf("other area: %s %s", code, raw)
	}
	// friend's instance full → ROOM_FULL
	dung := e.student(e.tenantA, e.schoolA, 3)
	em := e.student(e.tenantA, e.schoolA, 3)
	e.friends(dung, em)
	rd := e.mustJoin(dung, "learning_zone.math")
	filler := e.student(e.tenantA, e.schoolA, 3)
	if rf := e.mustJoin(filler, "learning_zone.math"); rf.RoomID != rd.RoomID {
		t.Fatal("filler not in dung's room")
	}
	if a, code, raw := follow(em, dung, "learning_zone.math"); a != nil || code != "ROOM_FULL" || strings.Contains(raw, rd.RoomID) {
		t.Fatalf("full: %s %s", code, raw)
	}
	// friend offline (no hub seat) → denied
	ghostFriend := e.student(e.tenantA, e.schoolA, 3)
	e.friends(ghostFriend, binh)
	if a, code, _ := follow(binh, ghostFriend, "main_hub"); a != nil || code != "PERMISSION_DENIED" {
		t.Fatalf("offline friend: %s", code)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.rooms`); n != 2 {
		t.Fatalf("rooms %d", n)
	}
}
