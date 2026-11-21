//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// T032 / F116-AC-27, F116-AC-28 (TC-I18N-008): listQuickMessages returns the approved messages in vi and en, the same
// for every grade; kind = quick is checked (unknown code, parameters), delivered as code + params (never a translated
// string) and its parameters go through the pipeline like a text.
func TestQuickMessages(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	c.publishPlatform(defaultTerms()...)
	an := c.student(c.tenantA, c.schoolA)
	binh := c.student(c.tenantA, c.schoolA2)
	ch := c.open(channel.Party, an, binh)
	fan := channelInbox(t, c.vk, ch)
	time.Sleep(200 * time.Millisecond)

	list := c.call(an, http.MethodGet, "/chat/quick-messages?channel_type=party", "", nil)
	if list.status != http.StatusOK || len(list.data()) < 10 {
		t.Fatalf("list: %d %s", list.status, list.raw)
	}
	for _, q := range list.data() {
		text, _ := q["text"].(map[string]any)
		if q["quick_message_id"] == "" || text["vi"] == "" || text["en"] == "" || text["vi"] == nil || text["en"] == nil {
			t.Fatalf("not bilingual: %v", q)
		}
	}
	// The same list for another student (another school, any grade: R15).
	if other := c.call(binh, http.MethodGet, "/chat/quick-messages", "", nil); other.raw != list.raw {
		t.Fatalf("lists differ between students:\n%s\n%s", other.raw, list.raw)
	}
	if bad := c.call(an, http.MethodGet, "/chat/quick-messages?channel_type=nope", "", nil); bad.code() != "VALIDATION_FAILED" {
		t.Fatalf("bad channel type: %d %s", bad.status, bad.raw)
	}
	// Only students (game_player) and only the tenant of the session.
	if r := c.do(http.MethodGet, "/chat/quick-messages", c.staff(c.tenantA, "school_admin"), map[string]string{"X-Tenant-Id": c.tenantA.String()}, nil); r.status != http.StatusForbidden {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	if r := c.do(http.MethodGet, "/chat/quick-messages", an.token, map[string]string{"X-Tenant-Id": c.tenantB.String()}, nil); r.status != http.StatusForbidden &&
		r.status != http.StatusNotFound {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "listQuickMessages", authztest.CrossTenant, authztest.RoleDenied)

	quick := func(id string, params map[string]string) map[string]any {
		p := map[string]any{"channel_id": ch, "client_message_id": uuid.New(), "kind": "quick", "quick_message_id": id}
		if params != nil {
			p["params"] = params
		}
		return p
	}
	// Unknown or retired code → VALIDATION_FAILED.
	if r := c.ws(an, "conn-an", "chat.message.send", quick("qm_unknown", nil)); replyCode(r) != "VALIDATION_FAILED" {
		t.Fatalf("unknown quick: %+v", r)
	}
	// A parameter the message does not use, or a missing one → VALIDATION_FAILED.
	if r := c.ws(an, "conn-an", "chat.message.send", quick("qm_hello", map[string]string{"n": "3"})); replyCode(r) != "VALIDATION_FAILED" {
		t.Fatalf("extra param: %+v", r)
	}
	if r := c.ws(an, "conn-an", "chat.message.send", quick("qm_done_question", nil)); replyCode(r) != "VALIDATION_FAILED" {
		t.Fatalf("missing param: %+v", r)
	}
	// Delivered as code + params: the receiver renders its own locale.
	if r := c.ws(an, "conn-an", "chat.message.send", quick("qm_done_question", map[string]string{"n": "3"})); !r.OK {
		t.Fatalf("quick: %+v", r.Problem)
	}
	env := next(fan, "chat.message.created", 3*time.Second)
	p := payloadOf(env)
	params, _ := p["params"].(map[string]any)
	if env == nil || p["quick_message_id"] != "qm_done_question" || params["n"] != "3" || p["body"] != nil {
		t.Fatalf("created: %v", p)
	}
	// Parameters go through the pipeline: a phone number is blocked (pii), a term too.
	if r := c.ws(an, "conn-an", "chat.message.send", quick("qm_done_question", map[string]string{"n": "0912345678"})); replyCode(r) != "CHAT_CONTENT_BLOCKED" ||
		r.Problem.Details["category_code"] != "pii" {
		t.Fatalf("pii param: %+v", r.Problem)
	}
	if r := c.ws(an, "conn-an", "chat.message.send", quick("qm_done_question", map[string]string{"n": "zqlow"})); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("masked param must block: %+v", r.Problem)
	}
	// REST too, with the stored row keeping code + params.
	rr := c.sendREST(binh, ch, map[string]any{"client_message_id": uuid.New(), "kind": "quick", "quick_message_id": "qm_thanks"})
	if rr.status != http.StatusCreated || rr.body["quick_message_id"] != "qm_thanks" {
		t.Fatalf("REST quick: %d %s", rr.status, rr.raw)
	}
	if n := c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1 AND kind = 'quick'
		AND quick_message_id = 'qm_done_question' AND filter_verdict->'params'->>'n' = '3'`, ch); n != 1 {
		t.Fatalf("stored quick rows %d", n)
	}
}
