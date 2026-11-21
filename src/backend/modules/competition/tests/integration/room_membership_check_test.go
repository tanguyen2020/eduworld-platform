//go:build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// partySession seeds a running explore session of a party (members in order, the first is the leader) in area, like a
// committed startPartySession, and returns the room id of its private_instance.
func (e *env) partySession(area string, members ...student) uuid.UUID {
	e.t.Helper()
	pid, act, room := uuid.New(), uuid.New(), uuid.New()
	l := members[0]
	e.exec(`INSERT INTO multiplayer.parties (party_id, tenant_id, created_at, updated_at, school_id, name, leader_id, party_type, max_members,
		grade_band, status, state_version, current_activity_id) VALUES ($1, $2, now(), now(), $3, 'Nhóm', $4, 'friend', 4, 'g6_7', 'in_activity', 3, $5)`,
		pid, l.tenant, l.school, l.id, act)
	for i, m := range members {
		role := "member"
		if i == 0 {
			role = "leader"
		}
		e.exec(`INSERT INTO multiplayer.party_members (membership_id, tenant_id, created_at, updated_at, party_id, student_id, school_id, role,
			joined_at) VALUES ($1, $2, now(), now(), $3, $4, $5, $6, now() + $7::interval)`, uuid.New(), m.tenant, pid, m.id,
			m.school, role, fmt.Sprintf("%d seconds", i))
	}
	e.exec(`INSERT INTO multiplayer.rooms (room_id, tenant_id, created_at, updated_at, room_type, area_id, school_id, grade_band, capacity, status)
		VALUES ($1, $2, now(), now(), 'private_instance', $3, $4, 'g6_7', 8, 'open')`, room, l.tenant, area, l.school)
	e.exec(`INSERT INTO multiplayer.party_activities (party_activity_id, tenant_id, created_at, updated_at, school_id, party_id, activity_type,
		activity_ref, room_id, room_type, status, min_members, max_members, allow_continue_on_disconnect, started_at)
		VALUES ($1, $2, now(), now(), $3, $4, 'explore', $5, $6, 'private_instance', 'running', 1, 8, true, now())`, act, l.tenant, l.school, pid,
		area, room)
	for _, m := range members {
		e.exec(`INSERT INTO multiplayer.session_memberships (membership_id, tenant_id, created_at, updated_at, room_id, room_type, school_id,
			student_id, party_id, party_activity_id, joined_at) VALUES ($1, $2, now(), now(), $3, 'private_instance', $4, $5, $6, $7, now())`,
			uuid.New(), m.tenant, room, m.school, m.id, pid, act)
	}
	return room
}

// T022 — F101-AC-11, AC-17 (TC-PTY-011, TC-PTY-008, AC-MP-008, AC-MP-010): the participants of a party session joining the area
// of the session are seated in its private_instance (same room_id); a student outside the session membership is never placed
// there, cannot follow a friend into it, and every command it sends into the room is refused with ROOM_NOT_MEMBER, the room
// untouched; a participant back within disconnect_grace — after the hub seat expired, or from another device — returns to
// the same room_id with its new game session recorded.
func TestPartyRoomMembership(t *testing.T) {
	e := setup(t, envOpts{grace: time.Second})
	e.node("core-1")
	e.area("meadow", 40, []string{"g6_7"}, false, "active")
	a, b := e.student(e.tenantA, e.schoolA, 6), e.student(e.tenantA, e.schoolA, 6)
	x := e.student(e.tenantA, e.schoolA, 6)
	e.friends(x, a)
	room := e.partySession("meadow", a, b)

	ra := e.mustJoin(a, "meadow")
	if ra.RoomID != room.String() {
		t.Fatalf("A not in the party room: %s", ra.RoomID)
	}
	rb := e.mustJoin(b, "meadow")
	if rb.RoomID != room.String() || len(rb.Players) != 1 {
		t.Fatalf("B: %+v", rb)
	}
	// The outsider: another (public) instance, no following into the party room.
	if rx := e.mustJoin(x, "meadow"); rx.RoomID == room.String() {
		t.Fatal("outsider placed in the private_instance")
	}
	if _, r := e.join(x, "gw-1", map[string]any{"area_id": "meadow", "follow_user_id": a.id.String()}); r.OK || r.Problem.Code != "PERMISSION_DENIED" {
		t.Fatalf("follow into the party room: %+v", r)
	}
	out := e.channel(rt.RoomOutChannel(room.String()))
	xch := e.channel(rt.UserChannel(x.id))
	cx := x.caller("gw-1")
	if r := e.request("hub.player.emote", &cx, map[string]any{"room_id": room.String(), "emote_id": "emote_wave"}); r.OK || r.Problem.Code != "ROOM_NOT_MEMBER" {
		t.Fatalf("outsider emote: %+v", r)
	}
	e.move(x, room.String(), 1, [3]float64{1, 0, 1})
	if p := next(t, xch, "system.command.failed", nil, 5*time.Second); p["code"] != "ROOM_NOT_MEMBER" {
		t.Fatalf("outsider move: %v", p)
	}
	none(t, out, "hub.player.joined", 300*time.Millisecond)
	if n := e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND left_at IS NULL`, room); n != 2 {
		t.Fatalf("room memberships changed: %d", n)
	}

	// B loses the connection: the hub seat expires after 1 s, the session membership stays open (disconnect_grace of the
	// party session, F101-AC-17); B comes back from another device to the same room.
	e.presence(b, rt.PresenceOffline, rt.PresenceDisconnected)
	next(t, out, "hub.player.left", nil, 5*time.Second)
	if n := e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND student_id = $2 AND left_at IS NULL`, room, b.id); n != 1 {
		t.Fatal("party membership closed by the room host")
	}
	b.session, b.device = uuid.New(), uuid.New()
	if back := e.mustJoin(b, "meadow"); back.RoomID != room.String() {
		t.Fatalf("B not back in the party room: %s", back.RoomID)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE room_id = $1 AND student_id = $2 AND game_session_id = $3`,
		room, b.id, b.session); n != 1 {
		t.Fatal("game session of the new device not recorded")
	}
	// The session ends: the area leads back to a public instance.
	e.exec(`UPDATE multiplayer.session_memberships SET left_at = now(), leave_reason = 'room_closed' WHERE room_id = $1`, room)
	if r := e.mustJoin(a, "meadow"); r.RoomID == room.String() {
		t.Fatal("still routed to the closed party room")
	}
}
