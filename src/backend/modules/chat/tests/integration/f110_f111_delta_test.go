//go:build integration

package integration

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// Tests of the server deltas the Godot client specs F110 and F111 depend on (contracts §6, owner chốt 2026-11-28
// [Confirmed]): PlayerCard.title (F095), listFriendSuggestions source=recently_played (F095, ADR-0095: no classmates),
// listMyClubInvitations + ClubInvitation.club_name/inviter_nickname and the club events club.invitation_sent /
// club.join_requested (F120).

func (p *pEnv) coplay(t *testing.T, typ string, tenant uuid.UUID, payload map[string]any) {
	t.Helper()
	h := consumer.Coplayers{H: command.Coplayers{UoW: p.uow, Store: persistence.Coplay{}}}
	if err := p.handle(t, h.Handle, typ, tenant, payload); err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
}

func (p *pEnv) partyActivity(t *testing.T, tenant uuid.UUID, activity string, at time.Time, members ...student) {
	t.Helper()
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.id.String()
	}
	p.coplay(t, "party.activity_started", tenant, map[string]any{"party_activity_id": uuid.NewString(), "party_id": uuid.NewString(),
		"tenant_id": tenant, "activity_type": activity, "activity_ref": map[string]any{"kind": activity, "id": "x"},
		"room_id": uuid.NewString(), "room_type": "private_instance", "member_ids": ids, "game_session_ref": uuid.NewString(),
		"started_at": at.UTC().Format(time.RFC3339Nano), "state_version": 1})
}

func suggestionIDs(r resp) []string {
	var out []string
	for _, d := range r.data() {
		pl, _ := d["player"].(map[string]any)
		out = append(out, str(pl, "user_id"))
	}
	return out
}

// F095 delta (client F111 F111-AC-06/07): "Vừa chơi cùng" lists the students who played with the caller within
// friend_suggestion_recent_window, most recent first, with the context of the last play; never a friend, a pending
// request (either way), a block (either way), a muted student, an inactive student, another school (unless the unit
// tenant enabled cross-school social), another tenant, or anything in a common tenant without friends. Public card only:
// nickname, level, avatar, title — no presence, no real name, school or grade (R16). Only recently_played exists.
func TestF095FriendSuggestions(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	now := c.clock.Now()
	an, binh, chi := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	dung, em, hoa := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	khoa, old, gone := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	other := c.student(c.tenantA, c.schoolA2)
	// One boss of a party (coop), one explore activity (party), an old one outside the window.
	c.partyActivity(t, c.tenantA, "boss", now.Add(-time.Hour), an, binh, chi, dung)
	c.partyActivity(t, c.tenantA, "explore", now.Add(-2*time.Hour), an, em, hoa, gone, other)
	c.partyActivity(t, c.tenantA, "quest", now.Add(-8*24*time.Hour), an, old)
	// A match: khoa is the opponent (2 participants → all pairs).
	c.coplay(t, "match.started", c.tenantA, map[string]any{"match_id": uuid.NewString(), "room_id": uuid.NewString(), "mode": "duel",
		"matchmaking_type": "quick", "ranked": false, "is_bot": false, "subject_id": uuid.NewString(), "grade_level": 3,
		"question_count": 5, "started_at": now.Add(-30 * time.Minute).Format(time.RFC3339Nano), "participants": []any{
			map[string]any{"student_id": an.id, "tenant_id": c.tenantA, "school_id": c.schoolA, "team_no": 0},
			map[string]any{"student_id": khoa.id, "tenant_id": c.tenantA, "school_id": c.schoolA, "team_no": 1}}})
	// chi is a friend, dung blocked an, an has a pending request to em, an muted hoa, gone was cleaned up.
	c.befriend(an, chi)
	if r := c.call(dung, http.MethodPut, "/social/blocks/"+an.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("block: %d %s", r.status, r.raw)
	}
	if r := c.call(an, http.MethodPost, "/social/friend-requests", newKey(), map[string]any{"addressee_id": em.id}); r.status != http.StatusCreated {
		t.Fatalf("request: %d %s", r.status, r.raw)
	}
	if r := c.call(an, http.MethodPut, "/social/mutes/"+hoa.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("mute: %d %s", r.status, r.raw)
	}
	c.exec(`UPDATE ref.students SET deleted_at = now() WHERE student_id = $1`, gone.id)
	c.exec(`UPDATE ref.player_profiles SET title = '{"vi":"Tân binh","en":"Beginner"}' WHERE student_id = $1`, binh.id)

	list := func(s student, query string) resp {
		return c.call(s, http.MethodGet, "/social/friend-suggestions"+query, "", nil)
	}
	r := list(an, "?source=recently_played")
	if got := suggestionIDs(r); r.status != 200 || len(got) != 2 || got[0] != khoa.id.String() || got[1] != binh.id.String() {
		t.Fatalf("suggestions of an: %d %s", r.status, r.raw)
	}
	first, second := r.data()[0], r.data()[1]
	if first["source"] != "recently_played" || first["context_kind"] != "match" || second["context_kind"] != "coop" ||
		str(second, "last_played_at") == "" {
		t.Fatalf("contexts: %s", r.raw)
	}
	pl, _ := second["player"].(map[string]any)
	title, _ := pl["title"].(map[string]any)
	if pl["nickname"] != binh.nickname || title["vi"] != "Tân binh" || title["en"] != "Beginner" {
		t.Fatalf("card: %v", pl)
	}
	for _, k := range []string{"presence", "school_id", "grade_level", "display_name", "full_name"} {
		if _, ok := pl[k]; ok {
			t.Fatalf("card exposes %s: %v", k, pl)
		}
	}
	// Symmetric rows: binh sees an; dung (who blocked an) sees neither an nor the block.
	if got := suggestionIDs(list(binh, "?source=recently_played")); len(got) != 3 { // an, chi, dung
		t.Fatalf("binh: %v", got)
	}
	if got := suggestionIDs(list(dung, "?source=recently_played")); len(got) != 2 || slices.Contains(got, an.id.String()) {
		t.Fatalf("dung: %v", got)
	}
	// Keyset pages of one.
	p1 := list(an, "?source=recently_played&limit=1")
	page, _ := p1.body["page"].(map[string]any)
	if got := suggestionIDs(p1); len(got) != 1 || page["has_more"] != true {
		t.Fatalf("page 1: %s", p1.raw)
	}
	p2 := list(an, "?source=recently_played&limit=1&cursor="+str(page, "next_cursor"))
	if got := suggestionIDs(p2); len(got) != 1 || got[0] != binh.id.String() {
		t.Fatalf("page 2: %s", p2.raw)
	}
	// Only recently_played exists (classmates removed, ADR-0095).
	for _, q := range []string{"", "?source=classmates", "?source=nearby"} {
		if r := list(an, q); r.status != 422 {
			t.Errorf("source %q: %d %s", q, r.status, r.raw)
		}
	}
	// A unit tenant: another school only with cross_school_social_enabled.
	u1, u2 := c.student(c.tenantU, c.schoolU1), c.student(c.tenantU, c.schoolU2)
	c.partyActivity(t, c.tenantU, "explore", now.Add(-time.Hour), u1, u2)
	if got := suggestionIDs(list(u1, "?source=recently_played")); len(got) != 0 {
		t.Fatalf("other school without cross-school: %v", got)
	}
	c.tenant(c.tenantU, "unit", map[string]any{"cross_school_social_enabled": true})
	if got := suggestionIDs(list(u1, "?source=recently_played")); len(got) != 1 || got[0] != u2.id.String() {
		t.Fatalf("cross-school: %v", got)
	}
	// A large match records same-team pairs only; a participant of another tenant never pairs.
	big := []any{}
	team := map[int][]student{}
	for i := range 10 {
		s := c.student(c.tenantA, c.schoolA)
		team[i%2] = append(team[i%2], s)
		big = append(big, map[string]any{"student_id": s.id, "tenant_id": c.tenantA, "school_id": c.schoolA, "team_no": i % 2})
	}
	foreign := c.student(c.tenantB, c.schoolB)
	big = append(big, map[string]any{"student_id": foreign.id, "tenant_id": c.tenantB, "school_id": c.schoolB, "team_no": 0})
	c.coplay(t, "match.started", c.tenantA, map[string]any{"match_id": uuid.NewString(), "room_id": uuid.NewString(), "mode": "team",
		"matchmaking_type": "quick", "ranked": false, "is_bot": false, "subject_id": uuid.NewString(), "grade_level": 3,
		"question_count": 5, "started_at": now.Format(time.RFC3339Nano), "participants": big})
	if got := suggestionIDs(list(team[0][0], "?source=recently_played")); len(got) != 4 {
		t.Fatalf("team-only pairs: %v", got)
	}
	if n := c.count(`SELECT count(*) FROM social.recent_coplayers WHERE user_id = $1 OR other_id = $1`, foreign.id); n != 0 {
		t.Fatalf("cross-tenant rows: %d", n)
	}
	// Tenant isolation: the student of tenant B sees nothing of tenant A; a header of another tenant is refused.
	if got := suggestionIDs(list(foreign, "?source=recently_played")); len(got) != 0 {
		t.Fatalf("tenant B: %v", got)
	}
	if r := c.do(http.MethodGet, "/social/friend-suggestions?source=recently_played", an.token,
		map[string]string{"X-Tenant-Id": c.tenantB.String()}, nil); r.status != 403 || r.code() != "TENANT_MISMATCH" {
		t.Fatalf("header mismatch: %d %s", r.status, r.raw)
	}
	// A common tenant suggests like a school tenant (friends on by default, owner 2026-11-28); once System Admin switched
	// friends off for it (incident switch), it suggests nobody (F095-AC-23).
	common := uuid.New()
	c.tenant(common, "common", nil)
	schoolC := uuid.New()
	ca, cb := c.student(common, schoolC), c.student(common, schoolC)
	c.partyActivity(t, common, "explore", now.Add(-time.Hour), ca, cb)
	if r := list(ca, "?source=recently_played"); r.status != 200 || len(r.data()) != 1 {
		t.Fatalf("common tenant by default: %d %s", r.status, r.raw)
	}
	c.exec(`INSERT INTO chat.common_tenant_social_overrides (tenant_id, direct_enabled, world_enabled, area_free_text_enabled,
		friends_enabled, reason) VALUES ($1, true, true, false, false, 'Su co spam ket ban')`, common)
	if r := list(ca, "?source=recently_played"); r.status != 200 || len(r.data()) != 0 {
		t.Fatalf("common tenant friends switched off: %d %s", r.status, r.raw)
	}
	// Staff never use the player routes (R23).
	staff := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	if r := c.portal(staff, c.tenantA, http.MethodGet, "/social/friend-suggestions?source=recently_played", nil, nil); r.status != 403 ||
		r.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	// Replaying an event changes nothing (upsert keeps the newest time).
	before := c.count(`SELECT count(*) FROM social.recent_coplayers`)
	c.partyActivity(t, c.tenantA, "boss", now.Add(-3*time.Hour), an, binh)
	if after := c.count(`SELECT count(*) FROM social.recent_coplayers`); after != before {
		t.Fatalf("rows %d → %d", before, after)
	}
	if r := list(an, "?source=recently_played"); r.data()[1]["context_kind"] != "coop" {
		t.Fatalf("an older play overwrote the newer: %s", r.raw)
	}
	authztest.Cover(t, "listFriendSuggestions", authztest.CrossTenant, authztest.RoleDenied)
}

// F095 delta: level.reached (owner F074) raises the level and sets the level title of the public card; an older event
// never lowers it; the title shows on listFriends and listFriendRequests cards (PlayerCard.title).
func TestF095PlayerCardTitle(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	game := consumer.GameProjection{P: command.Projection{Refs: persistence.Refs{}, Social: persistence.Social{}, Now: c.clock.Now}}
	reach := func(level int, title any) {
		t.Helper()
		if err := c.handle(t, game.Handle, "level.reached", c.tenantA, map[string]any{"student_id": binh.id, "from_level": level - 1,
			"to_level": level, "curve_version": 1, "title_code": "explorer", "title": title, "reached_at": c.clock.Now()}); err != nil {
			t.Fatalf("level.reached %d: %v", level, err)
		}
	}
	reach(10, map[string]any{"vi": "Nhà thám hiểm", "en": "Explorer"})
	reach(9, map[string]any{"vi": "Tân binh", "en": "Beginner"}) // late, older: ignored
	reach(11, nil)                                               // no title of its own: keeps the current one
	var level int
	var title string
	if err := c.su.QueryRow(`SELECT level, title->>'vi' FROM ref.player_profiles WHERE student_id = $1`, binh.id).Scan(&level, &title); err != nil ||
		level != 11 || title != "Nhà thám hiểm" {
		t.Fatalf("profile: %d %q %v", level, title, err)
	}
	if err := c.handle(t, game.Handle, "level.reached", c.tenantA, map[string]any{"student_id": binh.id, "from_level": 11, "to_level": 12,
		"curve_version": 1, "title": map[string]any{"vi": ""}, "reached_at": c.clock.Now()}); err == nil {
		t.Fatal("an incomplete title was accepted")
	}
	c.befriend(an, binh)
	r := c.call(an, http.MethodGet, "/social/friends", "", nil)
	tl, _ := r.data()[0]["title"].(map[string]any)
	if r.status != 200 || tl["en"] != "Explorer" || r.data()[0]["level"] != float64(11) {
		t.Fatalf("friends: %d %s", r.status, r.raw)
	}
}

// F120 delta (client F111): listMyClubInvitations lists only the invitations of the caller with the club name and the
// inviter nickname (R16); pending = live and not expired; expired includes the pending ones past expires_at; a new
// invitation and a new join request publish club.invitation_sent / club.join_requested once (schemas checked) and the
// invitation is audited (R10); a repeated one publishes nothing.
func TestF120MyClubInvitations(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	owner, invitee, bystander := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	joiner := c.student(c.tenantA, c.schoolA)
	clubID, _ := c.mustClub(owner, "CLB Kham pha")
	id := clubID.String()
	invite := func() resp {
		return c.call(owner, http.MethodPost, "/social/clubs/"+id+"/invitations", newKey(), map[string]any{"invitee_ids": []uuid.UUID{invitee.id}})
	}
	if r := invite(); r.status != http.StatusCreated {
		t.Fatalf("invite: %d %s", r.status, r.raw)
	}
	if r := invite(); r.status != http.StatusCreated { // pending already: same invitation, no new event
		t.Fatalf("invite again: %d %s", r.status, r.raw)
	}
	mine := func(s student, query string) resp {
		return c.call(s, http.MethodGet, "/social/club-invitations"+query, "", nil)
	}
	r := mine(invitee, "")
	if r.status != 200 || len(r.data()) != 1 {
		t.Fatalf("pending: %d %s", r.status, r.raw)
	}
	inv := r.data()[0]
	if inv["club_id"] != id || inv["club_name"] != "CLB Kham pha" || inv["inviter_nickname"] != owner.nickname ||
		inv["status"] != "pending" || inv["invitee_id"] != invitee.id.String() {
		t.Fatalf("invitation: %v", inv)
	}
	if r := mine(bystander, ""); r.status != 200 || len(r.data()) != 0 {
		t.Fatalf("another student sees: %s", r.raw)
	}
	if r := mine(owner, "?status=pending"); len(r.data()) != 0 {
		t.Fatalf("the inviter sees the invitee's list: %s", r.raw)
	}
	if r := mine(invitee, "?status=maybe"); r.status != 422 {
		t.Fatalf("status: %d %s", r.status, r.raw)
	}
	// Past expires_at: no longer pending, listed as expired.
	c.exec(`UPDATE social.club_invitations SET expires_at = $2 WHERE invitee_id = $1`, invitee.id, c.clock.Now().Add(-time.Minute))
	if r := mine(invitee, ""); len(r.data()) != 0 {
		t.Fatalf("expired still pending: %s", r.raw)
	}
	if r := mine(invitee, "?status=expired"); len(r.data()) != 1 || r.data()[0]["status"] != "expired" {
		t.Fatalf("expired: %s", r.raw)
	}
	// Join request: one club.join_requested even when repeated.
	for range 2 {
		if r := c.call(joiner, http.MethodPost, "/social/clubs/"+id+"/join-requests", newKey(), nil); r.status != http.StatusCreated {
			t.Fatalf("join request: %d %s", r.status, r.raw)
		}
	}
	seen := c.checkEvents("club.invitation_sent", "club.join_requested")
	if seen["club.invitation_sent"] != 1 || seen["club.join_requested"] != 1 {
		t.Fatalf("events: %v", seen)
	}
	if n := countOf(c.audits(), "club.invitation_sent"); n != 1 {
		t.Fatalf("audit: %d %v", n, c.audits())
	}
	// Staff and other tenants.
	staff := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	if r := c.portal(staff, c.tenantA, http.MethodGet, "/social/club-invitations", nil, nil); r.status != 403 || r.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	foreign := c.student(c.tenantB, c.schoolB)
	if r := mine(foreign, "?status=expired"); r.status != 200 || len(r.data()) != 0 {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	if r := c.do(http.MethodGet, "/social/club-invitations", invitee.token, map[string]string{"X-Tenant-Id": c.tenantB.String()}, nil); r.status != 403 ||
		r.code() != "TENANT_MISMATCH" {
		t.Fatalf("header mismatch: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "listMyClubInvitations", authztest.CrossTenant, authztest.RoleDenied)
}
