//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// userInbox subscribes ew:rt:user:<id> (every connection of a user) and returns the envelopes pushed to it.
func userInbox(t *testing.T, vk valkey.Client, user uuid.UUID) <-chan rt.Envelope {
	t.Helper()
	out := make(chan rt.Envelope, 256)
	dc, release := vk.Dedicate()
	wait := dc.SetPubSubHooks(valkey.PubSubHooks{OnMessage: func(m valkey.PubSubMessage) {
		var env rt.Envelope
		if json.Unmarshal([]byte(m.Message), &env) == nil {
			out <- env
		}
	}})
	if err := dc.Do(context.Background(), dc.B().Subscribe().Channel(rt.UserChannel(user)).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { release(); <-wait })
	return out
}

// T024 — F118-AC-18, AC-19 (TC-CHAT-035): more than chat_mentions_max mentions → 422 CHAT_MENTION_LIMIT; a mention of a
// non-member or of someone in a block relation is dropped (the message is still sent); a valid mention raises
// unread_mentions and pushes chat.mention.created with the game nickname and a moderated preview, never a login name.
func TestMentions(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh, chi, dung := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA),
		f.student(f.tenantA, f.schoolA)
	outsider := f.student(f.tenantA, f.schoolA)
	ch := f.party(an, binh, chi, dung)
	f.exec(`INSERT INTO social.blocks (block_id, tenant_id, blocker_id, blocked_id, created_at) VALUES ($1, $2, $3, $4, now())`,
		uuid.New(), f.tenantA, dung.id, an.id)
	_ = f.facts.InvalidateChannel(context.Background(), f.tenantA, ch)
	six := []uuid.UUID{binh.id, chi.id, dung.id, outsider.id, uuid.New(), uuid.New()}
	if r := f.sendREST(an, ch, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "@ca nha", "mentions": six}); r.status != http.StatusUnprocessableEntity || r.code() != "CHAT_MENTION_LIMIT" {
		t.Fatalf("6 mentions: %d %s", r.status, r.raw)
	}
	binhBox := userInbox(t, f.vk, binh.id)
	outBox := userInbox(t, f.vk, outsider.id)
	dungBox := userInbox(t, f.vk, dung.id)
	id := f.sendText(an, ch, "@Binh ra cong truong nhe", map[string]any{"mentions": []uuid.UUID{binh.id, outsider.id, dung.id}})
	env := next(binhBox, "chat.mention.created", 3*time.Second)
	if env == nil {
		t.Fatal("chat.mention.created not pushed")
	}
	p := payloadOf(env)
	by, _ := p["by"].(map[string]any)
	if by["nickname"] != an.nickname || p["message_id"] != id.String() || p["preview"] != "@Binh ra cong truong nhe" {
		t.Fatalf("mention payload: %v", p)
	}
	if strings.Contains(string(env.Message.Payload), "username") || strings.Contains(string(env.Message.Payload), an.sub) {
		t.Fatalf("mention leaks a login name: %s", env.Message.Payload)
	}
	if next(outBox, "chat.mention.created", 500*time.Millisecond) != nil || next(dungBox, "chat.mention.created", 300*time.Millisecond) != nil {
		t.Fatal("a non-member or a blocking member got a mention")
	}
	f.persisted(id)
	var mentions string
	if err := f.su.QueryRow(`SELECT array_to_string(mentions, ',') FROM chat.gameplay_messages WHERE message_id = $1`, id).Scan(&mentions); err != nil ||
		mentions != binh.id.String() {
		t.Fatalf("stored mentions: %q %v", mentions, err)
	}
	if n := f.count(`SELECT unread_mentions FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, binh.id); n != 1 {
		t.Fatalf("unread_mentions: %d", n)
	}
}

// T025 — F118-AC-20 (TC-CHAT-036): pin rights by channel type (leader in a party, any player in a match, both in a direct
// channel), at most chat_pins_max live pins, audit, chat.pin.updated; unpin is a soft delete.
func TestPins(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh, chi := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	party := f.party(an, binh, chi)
	inbox := channelInbox(t, f.vk, party)
	var ids []uuid.UUID
	for i := 0; i < 4; i++ {
		ids = append(ids, f.sendText(binh, party, "tin "+string(rune('a'+i)), nil))
	}
	pin := func(s student, ch, m uuid.UUID) resp {
		return f.call(s, http.MethodPut, "/chat/channels/"+ch.String()+"/pins/"+m.String(), "", nil)
	}
	if r := pin(chi, party, ids[0]); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("non-leader pin in a party: %d %s", r.status, r.raw)
	}
	for i := 0; i < 3; i++ {
		if r := pin(an, party, ids[i]); r.status != http.StatusNoContent {
			t.Fatalf("leader pin %d: %d %s", i, r.status, r.raw)
		}
	}
	env := next(inbox, "chat.pin.updated", 3*time.Second)
	if env == nil {
		t.Fatal("chat.pin.updated missing")
	}
	if r := pin(an, party, ids[3]); r.status != http.StatusConflict || r.code() != "CHAT_PIN_LIMIT_REACHED" {
		t.Fatalf("4th pin: %d %s", r.status, r.raw)
	}
	if n := f.waitCount(3, 5*time.Second, `SELECT count(*) FROM chat.pinned_messages WHERE channel_id = $1 AND deleted_at IS NULL`, party); n != 3 {
		t.Fatalf("live pins: %d", n)
	}
	l := f.call(binh, http.MethodGet, "/chat/channels/"+party.String()+"/pins", "", nil)
	if l.status != http.StatusOK || len(l.data()) != 3 {
		t.Fatalf("listChannelPins: %d %s", l.status, l.raw)
	}
	if r := f.call(an, http.MethodDelete, "/chat/channels/"+party.String()+"/pins/"+ids[0].String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("unpin: %d %s", r.status, r.raw)
	}
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.pinned_messages WHERE message_id = $1 AND deleted_at IS NOT NULL`, ids[0]); n != 1 {
		t.Fatal("unpin must be a soft delete")
	}
	if a := f.audits(); !containsAudit(a, "chat.message.pinned") || !containsAudit(a, "chat.message.unpinned") {
		t.Fatalf("pin audit: %v", a)
	}
	f.checkEvents("chat.message_pinned", "chat.message_unpinned")
	if len(f.events("chat.message_pinned")) != 3 || len(f.events("chat.message_unpinned")) != 1 {
		t.Fatal("pin events")
	}
	// Match: any player; direct: both members.
	match := f.open(channel.Match, an, chi)
	mm := f.sendText(an, match, "tran dau", nil)
	if r := pin(chi, match, mm); r.status != http.StatusNoContent {
		t.Fatalf("player pin in a match: %d %s", r.status, r.raw)
	}
	dm := f.open(channel.Direct, an, binh)
	dmm := f.sendText(an, dm, "rieng", nil)
	if r := pin(binh, dm, dmm); r.status != http.StatusNoContent {
		t.Fatalf("pin in a direct channel: %d %s", r.status, r.raw)
	}
	// WS chat.pin.remove takes the same path.
	if rep := f.ws(binh, "c1", "chat.pin.remove", map[string]any{"channel_id": dm, "message_id": dmm}); rep.Problem != nil {
		t.Fatalf("ws unpin: %+v", rep.Problem)
	}
}

// T026 — F118-AC-21 (TC-CHAT-037): typing is throttled to once per 3 s, shown at most for 3 nicknames, pushed through
// Valkey Pub/Sub only (no row anywhere), never to a member in a block relation.
func TestTyping(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh, chi := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.party(an, binh, chi)
	f.exec(`INSERT INTO social.blocks (block_id, tenant_id, blocker_id, blocked_id, created_at) VALUES ($1, $2, $3, $4, now())`,
		uuid.New(), f.tenantA, chi.id, binh.id)
	_ = f.facts.InvalidateChannel(context.Background(), f.tenantA, ch)
	inbox := channelInbox(t, f.vk, ch)
	tablesBefore := f.count(`SELECT count(*) FROM chat.message_revisions`) + f.count(`SELECT count(*) FROM infra.outbox_events`)
	broadcasts := 0
	for i := 0; i < 10; i++ { // 10 s of key presses, one per second
		if rep := f.ws(binh, "c1", "chat.typing.start", map[string]any{"channel_id": ch}); rep.Problem != nil {
			t.Fatalf("typing: %+v", rep.Problem)
		}
		for env := next(inbox, "chat.typing.updated", 150*time.Millisecond); env != nil; env = next(inbox, "chat.typing.updated", 100*time.Millisecond) {
			broadcasts++
			if env.Target != nil {
				for _, u := range env.Target.UserIDs {
					if u == chi.id {
						t.Fatal("typing pushed to a member who blocked the typist")
					}
				}
			}
		}
		f.clock.Add(time.Second)
	}
	if broadcasts < 3 || broadcasts > 4 {
		t.Fatalf("typing broadcasts over 10 s: %d (want one per 3 s)", broadcasts)
	}
	if after := f.count(`SELECT count(*) FROM chat.message_revisions`) + f.count(`SELECT count(*) FROM infra.outbox_events`); after != tablesBefore {
		t.Fatal("typing wrote to PostgreSQL")
	}
	if rep := f.ws(binh, "c1", "chat.typing.stop", map[string]any{"channel_id": ch}); rep.Problem != nil {
		t.Fatalf("typing stop: %+v", rep.Problem)
	}
}

// T028 — F118-AC-20: the leader role of a party channel follows party.created (leader_id) and party.leader_transferred of
// module competition (a late transfer older than the applied state is ignored); only the leader pins.
func TestPartyLeaderRole(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.openParty(an, binh)
	role := func(s student) string {
		var r string
		if err := f.su.QueryRow(`SELECT role FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, s.id).Scan(&r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	if role(an) != "leader" || role(binh) != "member" {
		t.Fatalf("roles after party.created: %s %s", role(an), role(binh))
	}
	msg := f.sendText(binh, ch, "ghim giup minh", nil)
	if r := f.call(binh, http.MethodPut, "/chat/channels/"+ch.String()+"/pins/"+msg.String(), "", nil); r.status != http.StatusForbidden {
		t.Fatalf("member pin: %d", r.status)
	}
	var party uuid.UUID
	if err := f.su.QueryRow(`SELECT (scope_ref->>'party_id')::uuid FROM chat.channels WHERE channel_id = $1`, ch).Scan(&party); err != nil {
		t.Fatal(err)
	}
	f.mustCompetition("party.leader_transferred", f.tenantA, map[string]any{"party_id": party, "tenant_id": f.tenantA, "from": an.id,
		"to": binh.id, "reason": "manual", "state_version": 2})
	if role(an) != "member" || role(binh) != "leader" {
		t.Fatalf("roles after the transfer: %s %s", role(an), role(binh))
	}
	f.mustCompetition("party.leader_transferred", f.tenantA, map[string]any{"party_id": party, "tenant_id": f.tenantA, "from": binh.id,
		"to": an.id, "reason": "manual", "state_version": 0}) // late, older than the applied state
	if role(binh) != "leader" {
		t.Fatal("a late transfer moved the leader back")
	}
	if r := f.call(binh, http.MethodPut, "/chat/channels/"+ch.String()+"/pins/"+msg.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("new leader pin: %d %s", r.status, r.raw)
	}
}
