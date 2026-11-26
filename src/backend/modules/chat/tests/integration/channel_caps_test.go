//go:build integration

package integration

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// listedChannel returns the Channel of id in a listMyChannels response (nil when absent).
func listedChannel(r resp, id uuid.UUID) map[string]any {
	for _, ch := range r.data() {
		if ch["channel_id"] == id.String() {
			return ch
		}
	}
	return nil
}

func strs(v any) []string {
	out := []string{}
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// Delta F119 (CHAT_SYSTEM §3, §7.1): listMyChannels returns per channel max_text_length of the effective policy (party:
// 200), the features the caller can use now and sendable_kinds narrowed by the caller's active sanctions — mute → none,
// restrict_text → quick only — while allowed_kinds stays the policy; the school preset quick_message_emote_only narrows
// kinds and features (no sticker, GIF, edit, mention) for every student of the school, whatever the grade (R15).
// Tenant isolation: a student of tenant B lists none of these channels; X-Tenant-Id of B → TENANT_MISMATCH.
func TestListMyChannelsCapabilities(t *testing.T) {
	c := setupChat(t, chatOpts{})
	an, binh, chi := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	party := c.open(channel.Party, an, binh, chi)
	now := c.clock.Now()
	c.exec(`INSERT INTO chat.sanctions (sanction_id, tenant_id, user_id, type, scope, reason, starts_at, ends_at)
		VALUES ($1, $2, $3, 'mute', 'all', 'integration test mute', $4, $5)`, uuid.New(), c.tenantA, binh.id, now.Add(-time.Minute), now.Add(time.Hour))
	c.exec(`INSERT INTO chat.sanctions (sanction_id, tenant_id, user_id, type, scope, reason, starts_at, ends_at)
		VALUES ($1, $2, $3, 'restrict_text', 'all', 'integration test restrict', $4, $5)`, uuid.New(), c.tenantA, chi.id, now.Add(-time.Minute), now.Add(time.Hour))
	list := func(s student) map[string]any {
		r := c.call(s, http.MethodGet, "/chat/channels", "", nil)
		if r.status != http.StatusOK {
			t.Fatalf("listMyChannels: %d %s", r.status, r.raw)
		}
		ch := listedChannel(r, party)
		if ch == nil {
			t.Fatalf("party channel missing: %s", r.raw)
		}
		return ch
	}
	all := []string{"text", "emoji", "quick", "sticker", "gif"}
	free := list(an)
	f := free["features"].(map[string]any)
	if free["max_text_length"] != float64(200) || !slices.Equal(strs(free["sendable_kinds"]), all) || !slices.Equal(strs(free["allowed_kinds"]), all) ||
		f["sticker"] != true || f["edit"] != true || f["mention"] != true || f["receipts"] != "per_member" ||
		f["edit_window_minutes"] != float64(15) || f["recall_window_hours"] != float64(24) {
		t.Fatalf("free member: %v", free)
	}
	muted := list(binh)
	mf := muted["features"].(map[string]any)
	if len(strs(muted["sendable_kinds"])) != 0 || !slices.Equal(strs(muted["allowed_kinds"]), all) || mf["sticker"] != false ||
		mf["edit"] != false || mf["reply"] != false || mf["recall"] != true || muted["max_text_length"] != float64(200) {
		t.Fatalf("muted member: %v", muted)
	}
	restricted := list(chi)
	rf := restricted["features"].(map[string]any)
	if !slices.Equal(strs(restricted["sendable_kinds"]), []string{"quick"}) || rf["sticker"] != false || rf["edit"] != false || rf["reply"] != true {
		t.Fatalf("restricted member: %v", restricted)
	}
	// School preset quick_message_emote_only (schoolA2): same narrowing for grade 1 and grade 9 (R15).
	young, older := c.student(c.tenantA, c.schoolA2), c.student(c.tenantA, c.schoolA2)
	c.exec(`UPDATE ref.students SET grade_level = 1 WHERE student_id = $1`, young.id)
	c.exec(`UPDATE ref.students SET grade_level = 9 WHERE student_id = $1`, older.id)
	c.exec(`INSERT INTO chat.chat_policies (policy_id, tenant_id, scope_type, scope_id, school_id, rules)
		VALUES ($1, $2, 'school', $3, $3, '{"schema_version":5,"scope_type":"school","scope_id":"`+c.schoolA2.String()+`",
		"chat_policy_preset":"quick_message_emote_only","rules":[]}')`, uuid.New(), c.tenantA, c.schoolA2)
	presetParty := c.open(channel.Party, young, older)
	for _, s := range []student{young, older} {
		r := c.call(s, http.MethodGet, "/chat/channels", "", nil)
		ch := listedChannel(r, presetParty)
		if ch == nil {
			t.Fatalf("preset party missing: %s", r.raw)
		}
		pf := ch["features"].(map[string]any)
		if !slices.Equal(strs(ch["allowed_kinds"]), []string{"emoji", "quick"}) || !slices.Equal(strs(ch["sendable_kinds"]), []string{"emoji", "quick"}) ||
			pf["sticker"] != false || pf["gif"] != false || pf["edit"] != false || pf["mention"] != false || pf["reaction"] != true {
			t.Fatalf("preset (grade %v): %v", s.id, ch)
		}
	}
	// Tenant isolation.
	dung := c.student(c.tenantB, c.schoolB)
	if r := c.call(dung, http.MethodGet, "/chat/channels", "", nil); r.status != http.StatusOK || listedChannel(r, party) != nil || listedChannel(r, presetParty) != nil {
		t.Fatalf("tenant B lists a channel of A: %d %s", r.status, r.raw)
	}
	if r := c.do(http.MethodGet, "/chat/channels", an.token, map[string]string{"X-Tenant-Id": c.tenantB.String()}, nil); r.status != http.StatusForbidden ||
		r.code() != "TENANT_MISMATCH" {
		t.Fatalf("X-Tenant-Id of B: %d %s", r.status, r.raw)
	}
}
