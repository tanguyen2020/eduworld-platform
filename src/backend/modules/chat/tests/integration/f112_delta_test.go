//go:build integration

package integration

import (
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// Tests of the server deltas that close the open questions F112-1…F112-9 of the client spec F112
// (specs/112-godot-persistent-chat-club-tools, contracts §6, Clarifications 2026-11-28).

func firstErrorCode(r resp) string {
	errs, _ := r.body["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	e, _ := errs[0].(map[string]any)
	return str(e, "field") + "/" + str(e, "code")
}

// F112-1, F112-2, F112-3, F112-4 — each role of a club against each owner tool: the owner edits the club, names officers
// and creates goals and events; an officer only lists/accepts join requests and invites (SOCIAL_SYSTEM §6.2) — updateClub,
// createClubGoal, createClubEvent, updateClubMemberRole → 403; a member neither. No student removes a member or archives
// the club from the game (403, nothing written; a non-member gets 404); school staff do it in the portal.
// internal_tournament (or a tournament template) → 422 VALUE_NOT_AVAILABLE; a metric outside the accepted ones → 422
// VALUE_NOT_AVAILABLE; listClubGoals publishes accepted_metrics.
func TestF112ClubRoles(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	owner, officer, member := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	outsider, joiner, invitee := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	clubID, _ := c.mustClub(owner, "CLB Kham pha")
	c.joinClub(owner, officer, clubID)
	c.joinClub(owner, member, clubID)
	id := clubID.String()
	if r := c.call(owner, http.MethodPatch, "/social/clubs/"+id+"/members/"+officer.id.String(), "", map[string]any{"role": "officer"}); r.status != 200 ||
		r.body["role"] != "officer" {
		t.Fatalf("owner names an officer: %d %s", r.status, r.raw)
	}
	version := func() string {
		r := c.call(owner, http.MethodGet, "/social/clubs/"+id, "", nil)
		v, _ := r.body["version"].(float64)
		return strconv.Itoa(int(v))
	}
	update := func(s student) resp {
		return c.do(http.MethodPatch, "/social/clubs/"+id, s.token, map[string]string{"X-Tenant-Id": s.tenant.String(), "If-Match": version()},
			map[string]any{"description": "Mo ta moi"})
	}
	goal := func(s student, metric string) resp {
		return c.call(s, http.MethodPost, "/social/clubs/"+id+"/goals", newKey(), map[string]any{"metric": metric, "target": 5})
	}
	event := func(s student, body map[string]any) resp {
		b := map[string]any{"title": "Buoi luyen tap", "starts_at": "2026-12-01T08:00:00Z"}
		for k, v := range body {
			b[k] = v
		}
		return c.call(s, http.MethodPost, "/social/clubs/"+id+"/events", newKey(), b)
	}
	setRole := func(s student, target student, role string) resp {
		return c.call(s, http.MethodPatch, "/social/clubs/"+id+"/members/"+target.id.String(), "", map[string]any{"role": role})
	}
	// Officer and member: no owner tool (403 PERMISSION_DENIED, nothing written).
	for _, s := range []student{officer, member} {
		for name, r := range map[string]resp{
			"updateClub":           update(s),
			"createClubGoal":       goal(s, "quests_completed"),
			"createClubEvent":      event(s, map[string]any{"event_kind": "practice_session"}),
			"updateClubMemberRole": setRole(s, member, "officer"),
		} {
			if r.status != 403 || r.code() != "PERMISSION_DENIED" {
				t.Errorf("%s %s: %d %s", s.nickname, name, r.status, r.raw)
			}
		}
	}
	if n := c.count(`SELECT count(*) FROM social.club_goals WHERE club_id = $1`, clubID) + c.count(`SELECT count(*) FROM social.club_events
		WHERE club_id = $1`, clubID); n != 0 {
		t.Fatalf("a refused tool wrote %d rows", n)
	}
	// The officer still handles join requests and invitations.
	jr := c.call(joiner, http.MethodPost, "/social/clubs/"+id+"/join-requests", newKey(), nil)
	if jr.status != http.StatusCreated {
		t.Fatalf("join request: %d %s", jr.status, jr.raw)
	}
	if r := c.call(officer, http.MethodGet, "/social/clubs/"+id+"/join-requests", "", nil); r.status != 200 || len(r.data()) != 1 {
		t.Fatalf("officer lists requests: %d %s", r.status, r.raw)
	}
	if r := c.call(officer, http.MethodPost, "/social/club-join-requests/"+str(jr.body, "request_id")+":accept", newKey(), nil); r.status != 200 {
		t.Fatalf("officer accepts: %d %s", r.status, r.raw)
	}
	if r := c.call(officer, http.MethodPost, "/social/clubs/"+id+"/invitations", newKey(), map[string]any{"invitee_ids": []uuid.UUID{invitee.id}}); r.status != 201 {
		t.Fatalf("officer invites: %d %s", r.status, r.raw)
	}
	if r := c.call(member, http.MethodGet, "/social/clubs/"+id+"/join-requests", "", nil); r.status != 403 {
		t.Fatalf("member lists requests: %d %s", r.status, r.raw)
	}
	// The owner keeps every owner tool.
	if r := update(owner); r.status != 200 || r.body["description"] != "Mo ta moi" {
		t.Fatalf("owner updates: %d %s", r.status, r.raw)
	}
	if r := goal(owner, "discoveries_found"); r.status != 201 {
		t.Fatalf("owner goal: %d %s", r.status, r.raw)
	}
	for _, m := range []string{"minutes_learning", "boss_defeated"} {
		if r := goal(owner, m); r.status != 422 || firstErrorCode(r) != "metric/VALUE_NOT_AVAILABLE" {
			t.Errorf("metric %s: %d %s", m, r.status, r.raw)
		}
	}
	if r := goal(owner, "score_total"); r.status != 422 || firstErrorCode(r) != "metric/INVALID" {
		t.Errorf("unknown metric: %d %s", r.status, r.raw)
	}
	l := c.call(member, http.MethodGet, "/social/clubs/"+id+"/goals", "", nil)
	am, _ := l.body["accepted_metrics"].([]any)
	if l.status != 200 || !slices.Equal(am, []any{"activities_completed", "quests_completed", "discoveries_found"}) {
		t.Fatalf("accepted_metrics: %d %s", l.status, l.raw)
	}
	if r := event(owner, map[string]any{"event_kind": "boss_raid"}); r.status != 201 || r.body["event_kind"] != "boss_raid" {
		t.Fatalf("owner event: %d %s", r.status, r.raw)
	}
	if r := event(owner, nil); r.status != 201 || r.body["event_kind"] != "other" {
		t.Fatalf("event without kind: %d %s", r.status, r.raw)
	}
	for name, body := range map[string]map[string]any{
		"internal_tournament": {"event_kind": "internal_tournament", "tournament_template_id": uuid.NewString()},
		"template only":       {"event_kind": "practice_session", "tournament_template_id": uuid.NewString()},
	} {
		if r := event(owner, body); r.status != 422 || firstErrorCode(r) != "event_kind/VALUE_NOT_AVAILABLE" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if r := event(owner, map[string]any{"event_kind": "party"}); r.status != 422 || firstErrorCode(r) != "event_kind/INVALID" {
		t.Errorf("unknown kind: %d %s", r.status, r.raw)
	}
	// F112-2: no student removes a member or archives the club; a non-member does not learn the club exists.
	for _, s := range []student{owner, officer} {
		if r := c.call(s, http.MethodDelete, "/social/clubs/"+id+"/members/"+member.id.String(), "", nil); r.status != 403 || r.code() != "PERMISSION_DENIED" {
			t.Errorf("%s removes: %d %s", s.nickname, r.status, r.raw)
		}
		if r := c.call(s, http.MethodPost, "/social/clubs/"+id+":archive", newKey(), nil); r.status != 403 || r.code() != "PERMISSION_DENIED" {
			t.Errorf("%s archives: %d %s", s.nickname, r.status, r.raw)
		}
	}
	if r := c.call(outsider, http.MethodDelete, "/social/clubs/"+id+"/members/"+member.id.String(), "", nil); r.status != 404 {
		t.Errorf("non-member removes: %d %s", r.status, r.raw)
	}
	if n := c.count(`SELECT count(*) FROM social.club_members WHERE club_id = $1 AND left_at IS NULL`, clubID); n != 4 {
		t.Fatalf("active members after the refused removals: %d", n)
	}
	if n := c.count(`SELECT count(*) FROM social.clubs WHERE club_id = $1 AND status = 'active'`, clubID); n != 1 {
		t.Fatal("club archived by a student")
	}
	denied := 0
	for _, p := range c.events("audit.recorded") {
		if meta, _ := p["metadata"].(map[string]any); p["action"] == "authz.denied" && meta["operation"] == "removeClubMember" {
			denied++
		}
	}
	if denied < 2 {
		t.Fatalf("denied removals audited: %d (%v)", denied, c.audits())
	}
	staff := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	if r := c.portal(staff, c.tenantA, http.MethodPost, "/social/clubs/"+id+":archive", map[string]string{"Idempotency-Key": newKey()}, nil); r.status != 204 {
		t.Fatalf("staff archives: %d %s", r.status, r.raw)
	}
}

// F112-5, F112-9 — listMyChannels and openDirectChannel carry the public card of the other student of a direct channel
// (user_id, nickname, level, avatar — never a real name, school or grade); updateChannelMembership sets the notification
// level of the caller only (Idempotency-Key, audited once per change, a replay changes nothing), shows as
// notification_level/muted in listMyChannels, never stops a message (send still accepted, the moderation pipeline runs)
// and is refused for World (422), a non-member, another tenant (404), staff (403 GAME_ACCESS_STUDENT_ONLY) and an
// X-Tenant-Id of another tenant (403 TENANT_MISMATCH).
func TestF112ChannelMembershipAndPeer(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.pace = 0
	an, binh, chi := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	foreign := c.student(c.tenantB, c.schoolB)
	c.befriend(an, binh)
	open := c.call(an, http.MethodPost, "/chat/direct-channels", "", map[string]any{"user_id": binh.id})
	peer, _ := open.body["peer"].(map[string]any)
	if open.status != http.StatusCreated || peer["user_id"] != binh.id.String() || peer["nickname"] != binh.nickname {
		t.Fatalf("open direct: %d %s", open.status, open.raw)
	}
	pii(t, open)
	dm := uuid.MustParse(str(open.body, "channel_id"))
	list := c.call(binh, http.MethodGet, "/chat/channels", "", nil)
	d := channelOf(list, "direct")
	if d == nil || d["peer"].(map[string]any)["user_id"] != an.id.String() || d["notification_level"] != "all" || d["muted"] != false {
		t.Fatalf("listMyChannels direct: %s", list.raw)
	}
	pii(t, list)
	if w := channelOf(list, "world"); w != nil && (w["notification_level"] != nil || w["peer"] != nil) {
		t.Fatalf("world carries member fields: %v", w)
	}
	path := "/chat/channels/" + dm.String() + "/membership"
	key := newKey()
	for range 2 { // a replay with the same key gives the same answer and no second write
		r := c.call(an, http.MethodPatch, path, key, map[string]any{"notification_level": "none"})
		if r.status != 200 || r.body["notification_level"] != "none" || r.body["muted"] != true || r.body["channel_id"] != dm.String() {
			t.Fatalf("mute: %d %s", r.status, r.raw)
		}
	}
	if r := c.call(an, http.MethodPatch, path, newKey(), map[string]any{"notification_level": "none"}); r.status != 200 {
		t.Fatalf("same level again: %d %s", r.status, r.raw)
	}
	if n := countOf(c.audits(), "chat.channel_membership.updated"); n != 1 {
		t.Fatalf("audit rows: %d %v", n, c.audits())
	}
	mine := channelOf(c.call(an, http.MethodGet, "/chat/channels", "", nil), "direct")
	if mine["notification_level"] != "none" || mine["muted"] != true {
		t.Fatalf("muted not listed: %v", mine)
	}
	if other := channelOf(c.call(binh, http.MethodGet, "/chat/channels", "", nil), "direct"); other["muted"] != false {
		t.Fatalf("the other member got muted: %v", other)
	}
	// A notification setting only: binh's message still reaches the muted channel through the pipeline.
	if r := c.wsText(binh, dm, "chao ban"); !r.OK {
		t.Fatalf("send to a muted channel: %+v", r.Problem)
	}
	if r := c.call(an, http.MethodPatch, path, newKey(), map[string]any{"notification_level": "mentions"}); r.status != 200 || r.body["muted"] != false {
		t.Fatalf("mentions: %d %s", r.status, r.raw)
	}
	if r := c.call(an, http.MethodPatch, path, newKey(), map[string]any{"notification_level": "loud"}); r.status != 422 {
		t.Fatalf("unknown level: %d %s", r.status, r.raw)
	}
	if r := c.call(an, http.MethodPatch, path, "", map[string]any{"notification_level": "all"}); r.status != 428 {
		t.Fatalf("no Idempotency-Key: %d %s", r.status, r.raw)
	}
	world := channel.WorldID(c.tenantA, &c.schoolA)
	if r := c.wsText(an, world, "xin chao"); !r.OK { // the World row exists from the first send
		t.Fatalf("world send: %+v", r.Problem)
	}
	if r := c.call(an, http.MethodPatch, "/chat/channels/"+world.String()+"/membership", newKey(), map[string]any{"notification_level": "none"}); r.status != 422 {
		t.Fatalf("world: %d %s", r.status, r.raw)
	}
	for name, s := range map[string]student{"non-member": chi, "other tenant": foreign} {
		if r := c.call(s, http.MethodPatch, path, newKey(), map[string]any{"notification_level": "none"}); r.status != 404 {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if r := c.call(an, http.MethodPatch, "/chat/channels/"+uuid.NewString()+"/membership", newKey(), map[string]any{"notification_level": "none"}); r.status != 404 {
		t.Fatalf("unknown channel: %d %s", r.status, r.raw)
	}
	staff := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	if r := c.portal(staff, c.tenantA, http.MethodPatch, path, map[string]string{"Idempotency-Key": newKey()},
		map[string]any{"notification_level": "none"}); r.status != 403 || r.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	if r := c.do(http.MethodPatch, path, an.token, map[string]string{"X-Tenant-Id": c.tenantB.String(), "Idempotency-Key": newKey()},
		map[string]any{"notification_level": "none"}); r.status != 403 || r.code() != "TENANT_MISMATCH" {
		t.Fatalf("header mismatch: %d %s", r.status, r.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND notification_level <> 'all' AND user_id <> $2`, dm, an.id); n != 0 {
		t.Fatal("another member's setting changed")
	}
	authztest.Cover(t, "updateChannelMembership", authztest.CrossTenant, authztest.RoleDenied)
}
