//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	chatrealtime "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/chatid"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

func (c *chatEnv) openParty(leader student, others ...student) uuid.UUID {
	c.t.Helper()
	party := uuid.New()
	c.mustCompetition("party.created", leader.tenant, partyCreated(party, leader.tenant, leader.school, leader.id, 0, c.clock.Now()))
	for i, s := range others {
		c.mustCompetition("party.member_joined", leader.tenant, memberJoined(party, leader.tenant, s.id, i+1))
	}
	return chatid.Party(party)
}

// T020 / F117-AC-11 (TC-CHAT-019): a student muted by a sanction stays in the gameplay channel and still receives every
// message; only sending is refused (CHAT_MUTED); the others chat normally. Real moderation pipeline (F116).
func TestGameplayMutedStudent(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.openParty(an, binh)
	now := c.clock.Now()
	c.exec(`INSERT INTO chat.sanctions (sanction_id, tenant_id, user_id, type, scope, reason, starts_at, ends_at)
		VALUES ($1, $2, $3, 'mute', 'all', 'integration test mute', $4, $5)`, uuid.New(), c.tenantA, binh.id, now.Add(-time.Minute), now.Add(time.Hour))
	fan := channelInbox(t, c.vk, ch)
	time.Sleep(150 * time.Millisecond)
	if r := c.sendWS(binh, "conn-binh", ch, "xin chao"); replyCode(r) != "CHAT_MUTED" {
		t.Fatalf("muted send: %+v", r)
	}
	if r := c.sendWS(an, "conn-an", ch, "chao ban"); !r.OK {
		t.Fatalf("send of the other member: %+v", r.Problem)
	}
	env := next(fan, "chat.message.created", 3*time.Second)
	if env == nil || env.Target == nil || !containsID(env.Target.UserIDs, binh.id) {
		t.Fatalf("the muted student must still receive: %+v", env)
	}
	if c.activeMembers(ch) != 2 {
		t.Fatal("the muted student left the channel")
	}
}

// T021 / F117-AC-12 (TC-SOC-003, TC-CHAT-005): A blocks B → their messages are hidden both ways in the shared gameplay
// channel (fan-out and history); C mutes B → only B's messages are hidden from C; nobody leaves the party.
func TestGameplayBlockMuteVisibility(t *testing.T) {
	c := setupChat(t, chatOpts{})
	a, b, cc := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.openParty(a, b, cc)
	c.exec(`INSERT INTO social.blocks (block_id, tenant_id, blocker_id, blocked_id) VALUES ($1, $2, $3, $4)`, uuid.New(), c.tenantA, a.id, b.id)
	c.exec(`INSERT INTO social.mutes (mute_id, tenant_id, muter_id, muted_id) VALUES ($1, $2, $3, $4)`, uuid.New(), c.tenantA, cc.id, b.id)
	_ = c.facts.InvalidateChannel(context.Background(), c.tenantA, ch)
	fan := channelInbox(t, c.vk, ch)
	time.Sleep(150 * time.Millisecond)
	if r := c.sendWS(b, "conn-b", ch, "tu b"); !r.OK {
		t.Fatalf("b: %+v", r.Problem)
	}
	env := next(fan, "chat.message.created", 3*time.Second)
	if env == nil || env.Target == nil || containsID(env.Target.UserIDs, a.id) || containsID(env.Target.UserIDs, cc.id) ||
		!containsID(env.Target.UserIDs, b.id) {
		t.Fatalf("recipients of B: %+v (A blocked B, C muted B)", env)
	}
	if r := c.sendWS(a, "conn-a", ch, "tu a"); !r.OK {
		t.Fatalf("a: %+v", r.Problem)
	}
	env = next(fan, "chat.message.created", 3*time.Second)
	if env == nil || env.Target == nil || containsID(env.Target.UserIDs, b.id) || !containsID(env.Target.UserIDs, cc.id) {
		t.Fatalf("recipients of A: %+v (block is two-way, the mute of C is one-way)", env)
	}
	if r := c.sendWS(cc, "conn-c", ch, "tu c"); !r.OK {
		t.Fatalf("c: %+v", r.Problem)
	}
	env = next(fan, "chat.message.created", 3*time.Second)
	if env == nil || env.Target == nil || !containsID(env.Target.UserIDs, b.id) || !containsID(env.Target.UserIDs, a.id) {
		t.Fatalf("recipients of C: %+v (B did not mute C)", env)
	}
	c.waitCount(3, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch)
	bodies := func(s student) string {
		r := c.call(s, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil)
		if r.status != http.StatusOK {
			t.Fatalf("history: %d %s", r.status, r.raw)
		}
		var out []string
		for _, d := range r.data() {
			body, _ := d["body"].(string)
			out = append(out, body)
		}
		return strings.Join(out, ",")
	}
	if got := bodies(a); got != "tu a,tu c" {
		t.Fatalf("history of A: %q", got)
	}
	if got := bodies(b); got != "tu b,tu c" {
		t.Fatalf("history of B: %q", got)
	}
	if got := bodies(cc); got != "tu a,tu c" {
		t.Fatalf("history of C: %q", got)
	}
	if c.activeMembers(ch) != 3 {
		t.Fatal("block/mute must not remove anyone from the party")
	}
}

// T038 / F117-AC-09, AC-24 (TC-CHAT-048, TC-CHAT-018): a school with the preset quick_message_emote_only still gets its
// party channel created and joined automatically, with quick + emoji only; a school rule dropping text (no preset) or
// disabling party is refused by every write path: the gate CheckPolicyDocument (422 CHAT_POLICY_ALWAYS_ON) and the
// database CHECK chat_policies_always_on_chk.
func TestGameplayPresetAndAlwaysOnWrites(t *testing.T) {
	c := setupChat(t, chatOpts{})
	c.exec(`INSERT INTO chat.chat_policies (policy_id, tenant_id, scope_type, scope_id, school_id, rules)
		VALUES ($1, $2, 'school', $3, $3, '{"schema_version":5,"scope_type":"school","scope_id":"`+c.schoolA.String()+`",
		"chat_policy_preset":"quick_message_emote_only","rules":[]}')`, uuid.New(), c.tenantA, c.schoolA)
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	ch := c.openParty(an, binh)
	if c.activeMembers(ch) != 2 {
		t.Fatal("party channel not created/joined under the preset")
	}
	if r := c.sendWS(an, "conn-an", ch, "chao"); r.OK || replyCode(r) != "CHAT_KIND_NOT_ALLOWED" {
		t.Fatalf("text under the preset: %+v", r)
	}
	if r := c.ws(an, "conn-an", "chat.message.send", map[string]any{"channel_id": ch, "client_message_id": uuid.New(), "kind": "quick",
		"quick_message_id": "qm.good_luck"}); !r.OK {
		t.Fatalf("quick under the preset: %+v", r.Problem)
	}
	lr := c.call(an, http.MethodGet, "/chat/channels", "", nil)
	for _, d := range lr.data() {
		if d["channel_id"] == ch.String() {
			if kinds, _ := d["allowed_kinds"].([]any); len(kinds) != 2 {
				t.Fatalf("allowed_kinds under the preset: %v", kinds)
			}
		}
	}
	for _, rules := range []string{
		`{"schema_version":5,"scope_type":"school","rules":[{"channel_type":"party","enabled":false}]}`,
		`{"schema_version":5,"scope_type":"school","rules":[{"channel_type":"match","allowed_kinds":["quick","emoji"]}]}`,
		`{"schema_version":5,"scope_type":"tenant","rules":[{"channel_type":"team","slow_mode_seconds":30}]}`,
	} {
		if err := command.CheckPolicyDocument([]byte(rules)); err == nil || !strings.Contains(err.Error(), "CHAT_POLICY_ALWAYS_ON") {
			t.Fatalf("gate accepted %s: %v", rules, err)
		}
		_, err := c.su.Exec(`INSERT INTO chat.chat_policies (policy_id, tenant_id, scope_type, scope_id, school_id, rules)
			VALUES ($1, $2, 'school', $3, $3, $4::jsonb)`, uuid.New(), c.tenantA, c.schoolA2, rules)
		if err == nil || !strings.Contains(err.Error(), "chat_policies_always_on_chk") {
			t.Fatalf("database accepted %s: %v", rules, err)
		}
	}
	// The platform policy cannot drop the rule of an always-on channel either.
	if _, err := c.su.Exec(`UPDATE chat.chat_policies SET rules = rules - 'rules' || '{"rules":[]}'::jsonb WHERE scope_type = 'platform'`); err == nil ||
		!strings.Contains(err.Error(), "chat_policies_always_on_chk") {
		t.Fatalf("platform without always-on rules accepted: %v", err)
	}
}

// T024 / F117-AC-13, AC-14 (TC-CHAT-039, TC-CHAT-004): chat.receipt.read raises last_read_seq (never back), coalesces to
// at most one durable write per 2 s per member (the newest seq of the window is written by a trailing flush) and pushes
// chat.unread.updated to every device of the reader; listMyChannels returns unread_count = last_seq − last_read_seq; a
// non-member gets NOT_FOUND.
func TestGameplayUnread(t *testing.T) {
	c := setupChat(t, chatOpts{})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	stranger := c.student(c.tenantA, c.schoolA)
	ch := c.openParty(an, binh)
	for i := 0; i < 12; i++ {
		if i == 10 {
			c.clock.Add(61 * time.Second)
		}
		if r := c.sendWS(an, "conn-an", ch, "tin "+uuid.NewString()[:4]); !r.OK {
			t.Fatalf("send %d: %+v", i, r.Problem)
		}
	}
	c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.channels WHERE channel_id = $1 AND last_seq = 12`, ch)
	var flushes []func()
	receipts := command.Receipts{UoW: c.uow, Facts: c.facts, Repo: persistence.Gameplay{}, Buffer: cache.Cursor{C: c.vk}, Recent: c.send.Recent,
		Push:   chatrealtime.ChatFanout{P: rt.Publisher{C: c.vk}, Now: c.clock.Now},
		Window: 2 * time.Second, Schedule: func(_ time.Duration, f func()) { flushes = append(flushes, f) }, Now: c.clock.Now}
	device2 := inbox(t, c.vk, binh.id)
	time.Sleep(150 * time.Millisecond)
	ctx := context.Background()
	if err := receipts.Read(ctx, c.tenantA, binh.id, ch, 10); err != nil {
		t.Fatal(err)
	}
	env := next(device2, "chat.unread.updated", 3*time.Second)
	if env == nil {
		t.Fatal("chat.unread.updated not pushed to the devices of the reader")
	}
	if counts, _ := payloadOf(env)["counts"].(map[string]any); counts[ch.String()] != float64(2) {
		t.Fatalf("unread after seq 10: %v", payloadOf(env))
	}
	// Within the window: buffered, not written; a lower seq never moves the cursor back.
	if err := receipts.Read(ctx, c.tenantA, binh.id, ch, 12); err != nil {
		t.Fatal(err)
	}
	if err := receipts.Read(ctx, c.tenantA, binh.id, ch, 5); err != nil {
		t.Fatal(err)
	}
	if n := c.count(`SELECT last_read_seq FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, binh.id); n != 10 {
		t.Fatalf("cursor written inside the window: %d", n)
	}
	if len(flushes) != 1 {
		t.Fatalf("trailing flushes scheduled: %d (want 1)", len(flushes))
	}
	flushes[0]()
	if n := c.count(`SELECT last_read_seq FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, binh.id); n != 12 {
		t.Fatalf("trailing flush: last_read_seq %d, want 12", n)
	}
	time.Sleep(2100 * time.Millisecond) // the window expired
	if err := receipts.Read(ctx, c.tenantA, binh.id, ch, 3); err != nil {
		t.Fatal(err)
	}
	if n := c.count(`SELECT last_read_seq FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, binh.id); n != 12 {
		t.Fatalf("cursor moved back: %d", n)
	}
	lr := c.call(binh, http.MethodGet, "/chat/channels", "", nil)
	found := false
	for _, d := range lr.data() {
		if d["channel_id"] == ch.String() {
			found = d["unread_count"] == float64(0) && d["my_read_seq"] == float64(12) && d["status"] == "open"
		}
	}
	if !found {
		t.Fatalf("listMyChannels: %s", lr.raw)
	}
	if lr := c.call(an, http.MethodGet, "/chat/channels", "", nil); !strings.Contains(lr.raw, `"unread_count":12`) {
		t.Fatalf("unread of An (read nothing): %s", lr.raw)
	}
	if err := receipts.Read(ctx, c.tenantA, stranger.id, ch, 1); err == nil || !strings.Contains(err.Error(), "NOT_FOUND") {
		t.Fatalf("non-member receipt: %v", err)
	}
	// Over the gateway command ew.cmd.chat.receipt.read the same use case answers (validation first).
	sctx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = natscmd.ServeReceipts(sctx, c.nc, receipts) }()
	t.Cleanup(func() { stop(); <-done })
	time.Sleep(150 * time.Millisecond)
	if r := c.ws(binh, "conn-binh", "chat.receipt.read", map[string]any{"channel_id": ch}); r.OK || replyCode(r) != "VALIDATION_FAILED" {
		t.Fatalf("receipt without seq: %+v", r)
	}
	if r := c.ws(stranger, "conn-x", "chat.receipt.read", map[string]any{"channel_id": ch, "seq": 1}); r.OK || replyCode(r) != "NOT_FOUND" {
		t.Fatalf("receipt of a non-member over WS: %+v", r)
	}
}
