//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// T033 — F118-AC-26 (TC-TEN-001): every new endpoint of F118 refuses a staff token (403 GAME_ACCESS_STUDENT_ONLY), answers
// 404 to a student of another tenant (never telling whether the message/channel exists) and 403 TENANT_MISMATCH to an
// X-Tenant-Id that is not the token's; the WS commands answer NOT_FOUND to a non-member.
// F118-AC-27 (TC-CHAT-036): a feature switched off in the platform features answers 403 CHAT_FEATURE_DISABLED; the
// "always" features of match/team/party stay on whatever the document says.
func TestFeaturesAuthz(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	outsider := f.student(f.tenantA, f.schoolA)
	foreign := f.student(f.tenantB, f.schoolB)
	ch := f.party(an, binh)
	id := f.sendText(an, ch, "xin chao", nil)
	f.persisted(id)
	type call struct {
		op, method, path string
		key              bool
		body             any
	}
	m, c := id.String(), ch.String()
	calls := []call{
		{"editMessage", http.MethodPatch, "/chat/messages/" + m, false, map[string]any{"body": "x"}},
		{"recallMessage", http.MethodPost, "/chat/messages/" + m + ":recall", false, nil},
		{"forwardMessage", http.MethodPost, "/chat/messages/" + m + ":forward", true, map[string]any{"target_channel_ids": []string{c}}},
		{"toggleMessageReaction", http.MethodPost, "/chat/messages/" + m + "/reactions", true, map[string]any{"reaction": "heart"}},
		{"pinMessage", http.MethodPut, "/chat/channels/" + c + "/pins/" + m, false, nil},
		{"unpinMessage", http.MethodDelete, "/chat/channels/" + c + "/pins/" + m, false, nil},
		{"listChannelPins", http.MethodGet, "/chat/channels/" + c + "/pins", false, nil},
		{"getMessageReceipts", http.MethodGet, "/chat/channels/" + c + "/messages/" + m + "/receipts", false, nil},
		{"listStickerPacks", http.MethodGet, "/chat/stickers", false, nil},
		{"listGifs", http.MethodGet, "/chat/gifs", false, nil},
	}
	headers := func(tenant uuid.UUID, key bool) map[string]string {
		h := map[string]string{"X-Tenant-Id": tenant.String(), "If-Match": `"1"`}
		if key {
			h["Idempotency-Key"] = newKey()
		}
		return h
	}
	staff := f.staff(f.tenantA, "tenant_admin")
	for _, x := range calls {
		if r := f.do(x.method, x.path, staff, headers(f.tenantA, x.key), x.body); r.status != http.StatusForbidden || r.code() != "GAME_ACCESS_STUDENT_ONLY" {
			t.Errorf("%s staff: %d %s", x.op, r.status, r.raw)
		}
		if r := f.do(x.method, x.path, an.token, headers(f.tenantB, x.key), x.body); r.status != http.StatusForbidden || r.code() != "TENANT_MISMATCH" {
			t.Errorf("%s header mismatch: %d %s", x.op, r.status, r.raw)
		}
		if x.op == "listStickerPacks" || x.op == "listGifs" {
			continue // no id: the tenant of the token only
		}
		for _, s := range []student{foreign, outsider} {
			if r := f.do(x.method, x.path, s.token, headers(s.tenant, x.key), x.body); r.status != http.StatusNotFound {
				t.Errorf("%s non-member/other tenant: %d %s", x.op, r.status, r.raw)
			}
		}
	}
	authztest.Cover(t, "editMessage", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "recallMessage", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "forwardMessage", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "toggleMessageReaction", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "pinMessage", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "unpinMessage", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listChannelPins", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getMessageReceipts", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listStickerPacks", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listGifs", authztest.CrossTenant, authztest.RoleDenied)
	for _, typ := range []string{"chat.message.edit", "chat.message.recall", "chat.reaction.toggle", "chat.pin.add", "chat.typing.start",
		"chat.receipt.delivered"} {
		p := map[string]any{"message_id": id, "channel_id": ch, "body": "x", "revision_expected": 1, "reaction": "heart", "seq": 1}
		if code := replyCode(f.ws(foreign, "cf", typ, p)); code != "NOT_FOUND" {
			t.Errorf("%s other tenant: %s", typ, code)
		}
	}

	// F118-AC-27: pin switched off for direct channels at the platform level → CHAT_FEATURE_DISABLED.
	dm := f.open(channel.Direct, an, binh)
	dmMsg := f.sendText(an, dm, "rieng", nil)
	f.exec(`UPDATE chat.chat_policies SET rules = jsonb_set(rules, '{rules}', (SELECT jsonb_agg(CASE WHEN r->>'channel_type' IN ('direct','party')
		THEN jsonb_set(jsonb_set(r, '{features,pin}', 'false'), '{features,edit}', 'false') ELSE r END) FROM jsonb_array_elements(rules->'rules') r))
		WHERE scope_type = 'platform' AND deleted_at IS NULL`)
	for _, ty := range []string{"direct", "party"} {
		_ = f.facts.Cache.Invalidate(context.Background(), "policy:"+f.tenantA.String()+":"+f.schoolA.String()+":"+ty)
	}
	if r := f.call(an, http.MethodPut, "/chat/channels/"+dm.String()+"/pins/"+dmMsg.String(), "", nil); r.status != http.StatusForbidden ||
		r.code() != "CHAT_FEATURE_DISABLED" {
		t.Fatalf("pin switched off: %d %s", r.status, r.raw)
	}
	if r := f.edit(an, dmMsg, 1, "sua"); r.status != http.StatusForbidden || r.code() != "CHAT_FEATURE_DISABLED" {
		t.Fatalf("edit switched off in direct: %d %s", r.status, r.raw)
	}
	// Edit is "always" in party: the document cannot switch it off (ADR-0020 §6).
	if r := f.edit(an, id, 1, "van sua duoc"); r.status != http.StatusOK {
		t.Fatalf("always-on edit in party: %d %s", r.status, r.raw)
	}
}
