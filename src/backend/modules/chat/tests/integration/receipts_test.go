//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
)

// T030 — F118-AC-22, AC-23, AC-24 (TC-CHAT-038): delivered/read cursors are coalesced and never move back, with no row
// per message × member; receipts are listed member by member in a party, as counts in a club channel of more than 8
// members, refused in world/area; two students in a block relation never see each other's status.
func TestReceipts(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	f.receipts.Schedule = func(time.Duration, func()) {} // trailing flushes are not needed here
	startFeatures(t, f)
	an, binh, chi, dung := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA),
		f.student(f.tenantA, f.schoolA)
	ch := f.party(an, binh, chi, dung)
	f.exec(`INSERT INTO social.blocks (block_id, tenant_id, blocker_id, blocked_id) VALUES ($1, $2, $3, $4)`, uuid.New(), f.tenantA, dung.id, an.id)
	_ = f.facts.InvalidateChannel(context.Background(), f.tenantA, ch)
	inbox := channelInbox(t, f.vk, ch)
	id := f.sendText(an, ch, "moi nguoi oi", nil)
	f.persisted(id)
	if rep := f.ws(binh, "cb", "chat.receipt.delivered", map[string]any{"channel_id": ch, "seq": 1}); rep.Problem != nil {
		t.Fatalf("delivered: %+v", rep.Problem)
	}
	if rep := f.ws(chi, "cc", "chat.receipt.read", map[string]any{"channel_id": ch, "seq": 1}); rep.Problem != nil {
		t.Fatalf("read: %+v", rep.Problem)
	}
	if rep := f.ws(dung, "cd", "chat.receipt.read", map[string]any{"channel_id": ch, "seq": 1}); rep.Problem != nil {
		t.Fatalf("read (dung): %+v", rep.Problem)
	}
	env := next(inbox, "chat.receipt.updated", 3*time.Second)
	if env == nil {
		t.Fatal("chat.receipt.updated not fanned out")
	}
	// The receipt of dung (who blocked an) never reaches an.
	for e := env; e != nil; e = next(inbox, "chat.receipt.updated", 300*time.Millisecond) {
		member, _ := payloadOf(e)["member"].(map[string]any)
		if member["user_id"] == dung.id.String() && e.Target != nil {
			for _, u := range e.Target.UserIDs {
				if u == an.id {
					t.Fatal("receipt pushed between students in a block relation")
				}
			}
		}
	}
	// A cursor never moves back (a late lower seq) and there is no table per message × member.
	f.clock.Add(3 * time.Second)
	if rep := f.ws(binh, "cb", "chat.receipt.delivered", map[string]any{"channel_id": ch, "seq": 0}); rep.Problem != nil {
		t.Fatalf("late delivered: %+v", rep.Problem)
	}
	if n := f.count(`SELECT last_delivered_seq FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, binh.id); n != 1 {
		t.Fatalf("delivered cursor moved back: %d", n)
	}
	r := f.call(an, http.MethodGet, "/chat/channels/"+ch.String()+"/messages/"+id.String()+"/receipts", "", nil)
	if r.status != http.StatusOK || r.body["mode"] != "per_member" {
		t.Fatalf("receipts: %d %s", r.status, r.raw)
	}
	status := map[string]string{}
	for _, m := range r.body["members"].([]any) {
		mm := m.(map[string]any)
		status[mm["member"].(map[string]any)["user_id"].(string)] = mm["status"].(string)
	}
	if status[binh.id.String()] != "delivered" || status[chi.id.String()] != "read" || len(status) != 2 {
		t.Fatalf("per member status (dung hidden, an is the sender): %v", status)
	}
	if r.body["read_count"] != float64(1) || r.body["delivered_count"] != float64(2) {
		t.Fatalf("counts: %s", r.raw)
	}
	// Club channel (team without match, > 8 members): counts only.
	members := []student{an}
	for i := 0; i < 9; i++ {
		members = append(members, f.student(f.tenantA, f.schoolA))
	}
	club := uuid.New()
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.id
	}
	if _, err := f.channels.OpenChannel(context.Background(), command.OpenChannelInput{ID: club, TenantID: f.tenantA, Type: "team",
		ScopeRef: map[string]string{"club_id": uuid.NewString()}, Members: ids}); err != nil {
		t.Fatal(err)
	}
	cm := f.sendText(an, club, "chao cau lac bo", nil)
	f.persisted(cm)
	r = f.call(an, http.MethodGet, "/chat/channels/"+club.String()+"/messages/"+cm.String()+"/receipts", "", nil)
	if r.status != http.StatusOK || r.body["mode"] != "count" || r.body["members"] != nil {
		t.Fatalf("club receipts: %d %s", r.status, r.raw)
	}
}

// T031 — F118-AC-25 (TC-CHAT-039): reading on one device updates unread_count and unread_mentions (reset when reading past
// the message with the mention) and pushes chat.unread.updated to every device of the reader.
func TestUnreadMentions(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, chi := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.party(an, chi)
	f.sendText(an, ch, "tin mot", nil)
	m2 := f.sendText(an, ch, "@Chi xem nay", map[string]any{"mentions": []uuid.UUID{chi.id}})
	f.persisted(m2)
	if n := f.waitCount(1, 5*time.Second, `SELECT unread_mentions FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, chi.id); n != 1 {
		t.Fatalf("unread_mentions before reading: %d", n)
	}
	devices := userInbox(t, f.vk, chi.id)
	// Device 2 reads up to seq 1: still one mention unread.
	if rep := f.ws(chi, "device-2", "chat.receipt.read", map[string]any{"channel_id": ch, "seq": 1}); rep.Problem != nil {
		t.Fatalf("read 1: %+v", rep.Problem)
	}
	env := next(devices, "chat.unread.updated", 3*time.Second)
	if env == nil {
		t.Fatal("chat.unread.updated not pushed")
	}
	p := payloadOf(env)
	counts, _ := p["counts"].(map[string]any)
	mentions, _ := p["mentions"].(map[string]any)
	if counts[ch.String()] != float64(1) || mentions[ch.String()] != float64(1) {
		t.Fatalf("after reading seq 1: %v", p)
	}
	// Reading past the mention resets it (after the coalescing window).
	f.clock.Add(3 * time.Second)
	time.Sleep(2100 * time.Millisecond)
	if rep := f.ws(chi, "device-2", "chat.receipt.read", map[string]any{"channel_id": ch, "seq": 2}); rep.Problem != nil {
		t.Fatalf("read 2: %+v", rep.Problem)
	}
	env = next(devices, "chat.unread.updated", 3*time.Second)
	if env == nil {
		t.Fatal("second chat.unread.updated missing")
	}
	p = payloadOf(env)
	counts, _ = p["counts"].(map[string]any)
	mentions, _ = p["mentions"].(map[string]any)
	if counts[ch.String()] != float64(0) || mentions[ch.String()] != float64(0) {
		t.Fatalf("after reading past the mention: %v", p)
	}
	if n := f.count(`SELECT unread_mentions FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, chi.id); n != 0 {
		t.Fatalf("unread_mentions not reset: %d", n)
	}
}
