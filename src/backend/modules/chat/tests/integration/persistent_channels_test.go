//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/subscriber"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

func channelOf(r resp, typ string) map[string]any {
	for _, c := range r.data() {
		if c["channel_type"] == typ {
			return c
		}
	}
	return nil
}

func (p *pEnv) wsText(s student, ch uuid.UUID, body string) rt.Reply {
	return p.ws(s, "c-"+s.id.String()[:6], "chat.message.send", map[string]any{"channel_id": ch, "client_message_id": uuid.New(),
		"kind": "text", "body": body})
}

// T029 — F120-AC-12 (TC-CHAT-002): the World channel of a student is the one of its school (listMyChannels lists it with a
// deterministic id, created at the first join/send); a student of the school receives the message, a student of another
// school or tenant can neither join nor send (NOT_FOUND); World messages land in chat.ephemeral_messages, never in
// channel_members; in a tenant unit whose tenant level sets world_scope tenant, the World channel spans its schools.
func TestWorldChannel(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.pace = 0
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	chi, dung := c.student(c.tenantA, c.schoolA2), c.student(c.tenantB, c.schoolB)
	world := channel.WorldID(c.tenantA, &c.schoolA)
	lst := c.call(an, http.MethodGet, "/chat/channels", "", nil)
	w := channelOf(lst, "world")
	if lst.status != http.StatusOK || w == nil || w["channel_id"] != world.String() || w["capabilities"].(map[string]any)["voice"] != false {
		t.Fatalf("listMyChannels world: %d %s", lst.status, lst.raw)
	}
	if r := c.ws(binh, "cb", "chat.channel.join", map[string]any{"channel_id": world}); !r.OK {
		t.Fatalf("join own world: %+v", r.Problem)
	}
	inbox := channelInbox(t, c.vk, world)
	if r := c.wsText(an, world, "chao ca truong"); !r.OK {
		t.Fatalf("send world: %+v", r.Problem)
	}
	if env := next(inbox, "chat.message.created", 3*time.Second); env == nil || payloadOf(env)["body"] != "chao ca truong" {
		t.Fatal("world message not broadcast to the school")
	}
	for _, s := range []student{chi, dung} {
		if r := c.ws(s, "cx", "chat.channel.join", map[string]any{"channel_id": world}); replyCode(r) != "NOT_FOUND" {
			t.Fatalf("join another school's world: %+v", r)
		}
		if r := c.wsText(s, world, "xin chao"); replyCode(r) != "NOT_FOUND" {
			t.Fatalf("send to another school's world: %+v", r)
		}
	}
	if n := c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.ephemeral_messages WHERE channel_id = $1`, world); n != 1 {
		t.Fatalf("ephemeral rows: %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1`, world); n != 0 {
		t.Fatalf("world channel members: %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.messages WHERE channel_id = $1`, world); n != 0 {
		t.Fatalf("world message in chat.messages: %d", n)
	}
	// Tenant unit with world_scope tenant (ADR-0054): one World for its schools.
	u1, u2 := c.student(c.tenantU, c.schoolU1), c.student(c.tenantU, c.schoolU2)
	unitAdmin := c.staff(c.tenantU, "tenant_admin")
	body := policyBody("Mo world toan don vi", map[string]any{"channel_type": "world", "enabled": true, "allowed_kinds": []string{"text", "emoji"},
		"max_text_length": 120, "rate": map[string]any{"messages_per_minute": 6, "burst": 2}, "slow_mode_seconds": 5, "validation": validation})
	body["world_scope"] = "tenant"
	if r := c.portal(unitAdmin, c.tenantU, http.MethodPut, "/tenant/chat-policies/tenant/"+c.tenantU.String(), map[string]string{"If-Match": "0"},
		body); r.status != http.StatusOK {
		t.Fatalf("world_scope tenant: %d %s", r.status, r.raw)
	}
	wide := channel.WorldID(c.tenantU, nil)
	if w := channelOf(c.call(u2, http.MethodGet, "/chat/channels", "", nil), "world"); w == nil || w["channel_id"] != wide.String() {
		t.Fatalf("unit world: %v", w)
	}
	if r := c.ws(u2, "cu2", "chat.channel.join", map[string]any{"channel_id": wide}); !r.OK {
		t.Fatalf("join unit world: %+v", r.Problem)
	}
	if r := c.wsText(u1, wide, "chao don vi"); !r.OK {
		t.Fatalf("send unit world: %+v", r.Problem)
	}
	// A school tenant cannot widen World (tenant level of a non-unit tenant → RELAX).
	ta := c.staff(c.tenantA, "tenant_admin")
	if r := c.portal(ta, c.tenantA, http.MethodPut, "/tenant/chat-policies/tenant/"+c.tenantA.String(), map[string]string{"If-Match": "0"},
		body); r.status != 422 || r.code() != "CHAT_POLICY_RELAX_NOT_ALLOWED" {
		t.Fatalf("school tenant widens world: %d %s", r.status, r.raw)
	}
}

// T030 — F120-AC-13 (TC-CHAT-008): the Area channel is the one of the room the gateway reported (presence signal room_id);
// a student of another room is refused (NOT_FOUND) even with the channel id; leaving the room (offline) closes the access.
func TestAreaChannel(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.pace = 0
	an, binh, chi := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	// The presence signal of the gateway carries the room (subscriber → RoomTracker).
	raw, _ := json.Marshal(rt.PresenceChanged{V: 1, UserID: an.id, TenantID: c.tenantA, Status: rt.PresenceOnline, RoomID: "hub:main_hub:01",
		Ts: time.Now().UnixMilli()})
	sig, ok := subscriber.Signal(raw)
	if !ok || sig.RoomID != "hub:main_hub:01" {
		t.Fatalf("signal room: %+v", sig)
	}
	ctx := context.Background()
	for s, room := range map[student]string{an: "hub:main_hub:01", binh: "hub:main_hub:01", chi: "hub:main_hub:02"} {
		if err := c.rooms.SetRoom(ctx, c.tenantA, s.id, room); err != nil {
			t.Fatal(err)
		}
	}
	area := channel.AreaID(c.tenantA, "hub:main_hub:01")
	if a := channelOf(c.call(an, http.MethodGet, "/chat/channels", "", nil), "area"); a == nil || a["channel_id"] != area.String() {
		t.Fatalf("listMyChannels area: %v", a)
	}
	if r := c.ws(binh, "cb", "chat.channel.join", map[string]any{"channel_id": area}); !r.OK {
		t.Fatalf("join area: %+v", r.Problem)
	}
	if r := c.wsText(an, area, "o day vui qua"); !r.OK {
		t.Fatalf("send area: %+v", r.Problem)
	}
	if r := c.wsText(chi, area, "minh o phong khac"); replyCode(r) != "NOT_FOUND" {
		t.Fatalf("other room: %+v", r)
	}
	if err := c.rooms.SetRoom(ctx, c.tenantA, an.id, ""); err != nil {
		t.Fatal(err)
	}
	_ = c.facts.Cache.Invalidate(ctx, "student:"+c.tenantA.String()+":"+an.id.String())
	if r := c.wsText(an, area, "con o day khong"); replyCode(r) != "NOT_FOUND" {
		t.Fatalf("after leaving the room: %+v", r)
	}
	if n := c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.ephemeral_messages WHERE channel_id = $1`, area); n != 1 {
		t.Fatalf("area ephemeral rows: %d", n)
	}
}

// T031 — F120-AC-14 (TC-CHAT-005), F120-AC-15 (TC-SOC-006), F120-AC-04 (TC-SVC-003): the direct channel opens only between
// friends without a block (CHAT_NOT_ALLOWED otherwise), once per normalised pair (201 then 200, same id from both sides);
// user.blocked freezes it (a redelivered event changes nothing more: one audit), sending → CHAT_NOT_ALLOWED; friends again
// → it reopens; a student of another school only sees the nickname (no school, no real name); staff never joins.
func TestDirectChannel(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.pace = 0
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA2)
	dung := c.student(c.tenantB, c.schoolB)
	open := func(s student, other uuid.UUID) resp {
		return c.call(s, http.MethodPost, "/chat/direct-channels", "", map[string]any{"user_id": other})
	}
	if r := open(an, binh.id); r.status != 403 || r.code() != "CHAT_NOT_ALLOWED" {
		t.Fatalf("not friends: %d %s", r.status, r.raw)
	}
	if r := open(an, dung.id); r.status != 404 {
		t.Fatalf("another tenant: %d %s", r.status, r.raw)
	}
	if r := open(an, an.id); r.status != 422 {
		t.Fatalf("self: %d %s", r.status, r.raw)
	}
	c.tenant(c.tenantA, "unit", map[string]any{"cross_school_social_enabled": true}) // binh is in another school of the tenant
	c.befriend(an, binh)
	first := open(an, binh.id)
	if first.status != http.StatusCreated || first.body["channel_type"] != "direct" || first.body["capabilities"].(map[string]any)["voice"] != false {
		t.Fatalf("open: %d %s", first.status, first.raw)
	}
	dm := uuid.MustParse(str(first.body, "channel_id"))
	if dm != channel.DirectID(c.tenantA, an.id, binh.id) {
		t.Fatal("not the deterministic pair id")
	}
	if r := open(binh, an.id); r.status != http.StatusOK || str(r.body, "channel_id") != dm.String() {
		t.Fatalf("reopen from the other side: %d %s", r.status, r.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND left_at IS NULL`, dm); n != 2 {
		t.Fatalf("members: %d", n)
	}
	if r := c.wsText(an, dm, "chao ban"); !r.OK {
		t.Fatalf("send dm: %+v", r.Problem)
	}
	c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, dm)
	h := c.call(binh, http.MethodGet, "/chat/channels/"+dm.String()+"/messages", "", nil)
	if h.status != http.StatusOK || len(h.data()) != 1 {
		t.Fatalf("history: %d %s", h.status, h.raw)
	}
	pii(t, h) // nickname and avatar only for a student of another school (R16)
	// binh blocks an: user.blocked (outbox) → chat.membership freezes the channel, twice delivered, once applied.
	if r := c.call(binh, http.MethodPut, "/social/blocks/"+an.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("block: %d %s", r.status, r.raw)
	}
	ev := c.events("user.blocked")
	if len(ev) != 1 {
		t.Fatalf("user.blocked events: %d", len(ev))
	}
	m := consumer.Membership{H: c.persistent}
	id := uuid.New()
	for range 2 {
		if err := c.event(m.Handle, id, "user.blocked", c.tenantA, ev[0]); err != nil {
			t.Fatal(err)
		}
	}
	if n := countOf(c.audits(), "chat.channel.frozen"); n != 1 {
		t.Fatalf("frozen audits: %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_id = $1 AND status = 'frozen'`, dm); n != 1 {
		t.Fatal("not frozen")
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND source_event_id = $2`, dm, id); n != 2 {
		t.Fatalf("source_event_id: %d", n)
	}
	if r := c.wsText(an, dm, "sao vay"); replyCode(r) != "CHAT_NOT_ALLOWED" {
		t.Fatalf("send frozen: %+v", r)
	}
	if ch := channelOf(c.call(an, http.MethodGet, "/chat/channels", "", nil), "direct"); ch == nil || ch["status"] != "frozen" {
		t.Fatalf("listMyChannels frozen: %v", ch)
	}
	// Unblock: user.unblocked reopens only friends; the block ended the friendship — friends again, the channel reopens.
	if r := c.call(binh, http.MethodDelete, "/social/blocks/"+an.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("unblock: %d %s", r.status, r.raw)
	}
	if err := c.event(m.Handle, uuid.New(), "user.unblocked", c.tenantA, c.events("user.unblocked")[0]); err != nil {
		t.Fatal(err)
	}
	friends := c.count(`SELECT count(*) FROM social.friendships WHERE tenant_id = $1 AND deleted_at IS NULL`, c.tenantA)
	if friends == 0 {
		c.befriend(an, binh)
		if r := open(an, binh.id); r.status != http.StatusOK || r.body["status"] != "open" {
			t.Fatalf("reopen after friends again: %d %s", r.status, r.raw)
		}
	}
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_id = $1 AND status = 'open'`, dm); n != 1 {
		t.Fatal("not reopened")
	}
	// T016: the payloads the consumer read are the user.blocked / user.unblocked schemas of F095.
	if seen := c.checkEvents("user.blocked", "user.unblocked"); seen["user.blocked"] != 1 || seen["user.unblocked"] != 1 {
		t.Fatalf("relation events: %v", seen)
	}
	authztest.Cover(t, "openDirectChannel", authztest.CrossTenant, authztest.RoleDenied)
	staff := c.staff(c.tenantA, "tenant_admin")
	if r := c.do(http.MethodPost, "/chat/direct-channels", staff, map[string]string{"X-Tenant-Id": c.tenantA.String()},
		map[string]any{"user_id": an.id}); r.status != 403 || r.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff opens a dm: %d %s", r.status, r.raw)
	}
}

// T058 — F120-AC-28 (TC-CHAT-047): in a tenant common not re-enabled by System Admin, direct and World are refused with
// CHAT_NOT_ALLOWED and Area takes quick + emoji only (CHAT_KIND_NOT_ALLOWED for text); System Admin re-enables them for that
// tenant with a reason (audit, effective at once); the override of a tenant that is not common is 404.
func TestCommonTenantDefaults(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.pace = 0
	common, school := uuid.New(), uuid.New()
	c.tenant(common, "common", nil)
	x, y := c.student(common, school), c.student(common, school)
	c.exec(`INSERT INTO social.friendships (friendship_id, tenant_id, created_at, updated_at, user_low_id, user_high_id)
		VALUES ($1, $2, now(), now(), least($3::uuid, $4::uuid), greatest($3::uuid, $4::uuid))`, uuid.New(), common, x.id, y.id)
	ctx := context.Background()
	for _, s := range []student{x, y} {
		if err := c.rooms.SetRoom(ctx, common, s.id, "hub:common:01"); err != nil {
			t.Fatal(err)
		}
	}
	if r := c.call(x, http.MethodPost, "/chat/direct-channels", "", map[string]any{"user_id": y.id}); r.status != 403 || r.code() != "CHAT_NOT_ALLOWED" {
		t.Fatalf("common dm: %d %s", r.status, r.raw)
	}
	world := channel.WorldID(common, &school)
	if channelOf(c.call(x, http.MethodGet, "/chat/channels", "", nil), "world") != nil {
		t.Fatal("world listed in a common tenant")
	}
	if r := c.wsText(x, world, "xin chao"); replyCode(r) != "CHAT_NOT_ALLOWED" {
		t.Fatalf("common world: %+v", r)
	}
	area := channel.AreaID(common, "hub:common:01")
	if r := c.wsText(x, area, "xin chao"); replyCode(r) != "CHAT_KIND_NOT_ALLOWED" {
		t.Fatalf("common area text: %+v", r)
	}
	if r := c.ws(x, "cx", "chat.message.send", map[string]any{"channel_id": area, "client_message_id": uuid.New(), "kind": "emoji",
		"emoji": "smile"}); !r.OK {
		t.Fatalf("common area emoji: %+v", r.Problem)
	}
	sa := c.systemAdmin()
	if r := c.portal(sa, c.tenantA, http.MethodGet, "/admin/chat/common-tenant-social", nil, nil); r.status != 404 {
		t.Fatalf("override of a school tenant: %d %s", r.status, r.raw)
	}
	g := c.portal(sa, common, http.MethodGet, "/admin/chat/common-tenant-social", nil, nil)
	if g.status != http.StatusOK || g.body["direct_enabled"] != false {
		t.Fatalf("defaults: %d %s", g.status, g.raw)
	}
	if r := c.portal(sa, common, http.MethodPut, "/admin/chat/common-tenant-social", map[string]string{"Idempotency-Key": newKey()},
		map[string]any{"direct_enabled": true, "world_enabled": true, "area_free_text_enabled": true}); r.status != 422 {
		t.Fatalf("no reason: %d %s", r.status, r.raw)
	}
	p := c.portal(sa, common, http.MethodPut, "/admin/chat/common-tenant-social", map[string]string{"Idempotency-Key": newKey()},
		map[string]any{"direct_enabled": true, "world_enabled": true, "area_free_text_enabled": true, "reason": "Truong da xac nhan giam sat"})
	if p.status != http.StatusOK || p.body["direct_enabled"] != true || p.body["version"] != float64(1) {
		t.Fatalf("re-enable: %d %s", p.status, p.raw)
	}
	if countOf(c.audits(), "chat_policy.common_tenant_social_updated") != 1 {
		t.Fatalf("audit: %v", c.audits())
	}
	if r := c.call(x, http.MethodPost, "/chat/direct-channels", "", map[string]any{"user_id": y.id}); r.status != http.StatusCreated {
		t.Fatalf("dm after re-enable: %d %s", r.status, r.raw)
	}
	if r := c.wsText(x, world, "chao ca truong"); !r.OK {
		t.Fatalf("world after re-enable: %+v", r.Problem)
	}
	if r := c.wsText(x, area, "chao moi nguoi"); !r.OK {
		t.Fatalf("area text after re-enable: %+v", r.Problem)
	}
}

// delivered reports whether the gateway would deliver env to a connection of s (the same target filter as
// revocation.Decide of the realtime gateway).
func delivered(env *rt.Envelope, s student) bool {
	return env != nil && env.Target.Accepts(s.id, uuid.Nil, "conn-"+s.id.String()[:6])
}

// historyFrom pushes chat.history.fetch for s on ch and returns the senders of the messages of chat.history.loaded.
func (p *pEnv) historyFrom(s student, ch uuid.UUID, box <-chan rt.Envelope) []string {
	p.t.Helper()
	conn := "conn-" + s.id.String()[:6]
	if r := p.ws(s, conn, "chat.history.fetch", map[string]any{"channel_id": ch}); !r.OK {
		p.t.Fatalf("history fetch: %+v", r.Problem)
	}
	env := next(box, "chat.history.loaded", 3*time.Second)
	if env == nil {
		p.t.Fatal("no chat.history.loaded")
	}
	var out []string
	for _, m := range payloadOf(env)["messages"].([]any) {
		out = append(out, m.(map[string]any)["sender"].(map[string]any)["user_id"].(string))
	}
	return out
}

func countID(ids []string, id uuid.UUID) int {
	n := 0
	for _, x := range ids {
		if x == id.String() {
			n++
		}
	}
	return n
}

// T061 — Clarifications 2026-11-23 (orchestrator, CHILD_SAFETY: the server decides): a World/Area message is never
// delivered to a connection whose student blocked the sender, was blocked by the sender or muted the sender, neither over
// realtime (target of the fan-out envelope, applied by the gateway) nor in the history served from the recent buffer; an
// unrelated student still receives it; a block/mute made after the relations were cached applies to the next message,
// an unblock lets the messages through again. The fan-out still follows the JetStream ack (R25).
func TestWorldAreaBlockMuteFiltered(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	an := c.student(c.tenantA, c.schoolA)
	binh, chi, dung, em := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA),
		c.student(c.tenantA, c.schoolA)
	put := func(s student, path string, other student) {
		t.Helper()
		if r := c.call(s, http.MethodPut, path+other.id.String(), "", nil); r.status != http.StatusNoContent {
			t.Fatalf("PUT %s: %d %s", path, r.status, r.raw)
		}
	}
	put(binh, "/social/blocks/", an) // binh blocked the sender
	put(an, "/social/blocks/", chi)  // the sender blocked chi
	put(dung, "/social/mutes/", an)  // dung muted the sender
	world := channel.WorldID(c.tenantA, &c.schoolA)
	for _, s := range []student{an, binh, chi, dung, em} {
		if r := c.ws(s, "conn-"+s.id.String()[:6], "chat.channel.join", map[string]any{"channel_id": world}); !r.OK {
			t.Fatalf("join world: %+v", r.Problem)
		}
	}
	box := channelInbox(t, c.vk, world)
	if r := c.wsText(an, world, "chao ca truong"); !r.OK {
		t.Fatalf("send world: %+v", r.Problem)
	}
	env := next(box, "chat.message.created", 3*time.Second)
	if env == nil {
		t.Fatal("world message not broadcast")
	}
	if !delivered(env, em) || !delivered(env, an) {
		t.Fatalf("unrelated student or sender must receive: target %+v", env.Target)
	}
	for name, s := range map[string]student{"blocker": binh, "blocked": chi, "muter": dung} {
		if delivered(env, s) {
			t.Fatalf("%s receives the world message: target %+v", name, env.Target)
		}
	}
	// A message of an unrelated sender reaches everybody (the filter is per sender).
	if r := c.wsText(em, world, "xin chao"); !r.OK {
		t.Fatalf("send world em: %+v", r.Problem)
	}
	if env := next(box, "chat.message.created", 3*time.Second); env == nil || !delivered(env, binh) || !delivered(env, chi) ||
		!delivered(env, dung) {
		t.Fatalf("unrelated sender filtered: %+v", env)
	}
	// History from the recent buffer: nothing of the sender for binh, chi, dung; em sees both.
	for name, s := range map[string]student{"blocker": binh, "blocked": chi, "muter": dung} {
		h := c.historyFrom(s, world, inbox(t, c.vk, s.id))
		if countID(h, an.id) != 0 || countID(h, em.id) != 1 {
			t.Fatalf("%s history: %v", name, h)
		}
	}
	if h := c.historyFrom(em, world, inbox(t, c.vk, em.id)); countID(h, an.id) != 1 || countID(h, em.id) != 1 {
		t.Fatalf("unrelated history: %v", h)
	}
	// Relations changed after they were cached: em mutes the sender, binh unblocks it.
	put(em, "/social/mutes/", an)
	if r := c.call(binh, http.MethodDelete, "/social/blocks/"+an.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("unblock: %d %s", r.status, r.raw)
	}
	if r := c.wsText(an, world, "lan hai"); !r.OK {
		t.Fatalf("send world again: %+v", r.Problem)
	}
	env = next(box, "chat.message.created", 3*time.Second)
	if env == nil || delivered(env, em) || !delivered(env, binh) || delivered(env, chi) || delivered(env, dung) {
		t.Fatalf("after mute/unblock: %+v", env)
	}
	// Area: the same filter (room of the gateway).
	ctx := context.Background()
	for _, s := range []student{an, chi, binh} {
		if err := c.rooms.SetRoom(ctx, c.tenantA, s.id, "hub:main_hub:07"); err != nil {
			t.Fatal(err)
		}
	}
	area := channel.AreaID(c.tenantA, "hub:main_hub:07")
	for _, s := range []student{chi, binh} {
		if r := c.ws(s, "conn-"+s.id.String()[:6], "chat.channel.join", map[string]any{"channel_id": area}); !r.OK {
			t.Fatalf("join area: %+v", r.Problem)
		}
	}
	abox := channelInbox(t, c.vk, area)
	if r := c.wsText(an, area, "o day vui qua"); !r.OK {
		t.Fatalf("send area: %+v", r.Problem)
	}
	if env := next(abox, "chat.message.created", 3*time.Second); env == nil || delivered(env, chi) || !delivered(env, binh) {
		t.Fatalf("area fan-out: %+v", env)
	}
	if h := c.historyFrom(chi, area, inbox(t, c.vk, chi.id)); countID(h, an.id) != 0 {
		t.Fatalf("area history of the blocked student: %v", h)
	}
	if h := c.historyFrom(binh, area, inbox(t, c.vk, binh.id)); countID(h, an.id) != 1 {
		t.Fatalf("area history of binh: %v", h)
	}
}
