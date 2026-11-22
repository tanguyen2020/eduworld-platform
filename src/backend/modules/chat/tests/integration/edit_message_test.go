//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// T008 — F118-AC-01, AC-02, AC-03 (TC-CHAT-029): an edit runs the whole pipeline again, keeps the previous version as a
// revision (evidence) and is told to the members only after the stream ack; outside the window, by someone else or with a
// stale revision it is refused; a blocked edit keeps the previous version; a held edit is never shown to the others.
func TestEditMessage(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.party(an, binh)
	inbox := channelInbox(t, f.vk, ch)
	id := f.sendText(an, ch, "chao ca nha", nil)
	f.persisted(id)
	calls := f.validator.count()

	f.clock.Add(time.Minute)
	r := f.edit(an, id, 1, "chao ca lop minh")
	if r.status != http.StatusOK || r.body["revision"] != float64(2) || r.body["body"] != "chao ca lop minh" || r.body["edited_at"] == nil {
		t.Fatalf("edit: %d %s", r.status, r.raw)
	}
	if f.validator.count() != calls+1 {
		t.Fatalf("the edit did not go through the pipeline again (calls %d → %d)", calls, f.validator.count())
	}
	env := next(inbox, "chat.message.edited", 3*time.Second)
	if env == nil {
		t.Fatal("chat.message.edited not fanned out")
	}
	if p := payloadOf(env); p["revision"] != float64(2) || p["body"] != "chao ca lop minh" {
		t.Fatalf("edited payload: %v", p)
	}
	// chat-persister: revision 1 kept as evidence, the row moved to revision 2.
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.message_revisions WHERE message_id = $1 AND revision = 1
		AND state = 'previous' AND body = 'chao ca nha'`, id); n != 1 {
		t.Fatal("previous version not kept in chat.message_revisions")
	}
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE message_id = $1 AND revision = 2
		AND edit_count = 1 AND body_display = 'chao ca lop minh' AND edited_at IS NOT NULL`, id); n != 1 {
		t.Fatal("message row not edited")
	}
	f.checkEvents("chat.message_edited")
	if n := len(f.events("chat.message_edited")); n != 1 {
		t.Fatalf("chat.message_edited events: %d", n)
	}
	// F118-AC-02: stale revision → 412; another student → 409; past the window → 409.
	if r := f.edit(an, id, 1, "lan nua"); r.status != http.StatusPreconditionFailed || r.code() != "VERSION_MISMATCH" {
		t.Fatalf("stale revision: %d %s", r.status, r.raw)
	}
	if r := f.edit(binh, id, 2, "sua ho"); r.status != http.StatusConflict || r.code() != "CHAT_EDIT_WINDOW_EXPIRED" {
		t.Fatalf("not the sender: %d %s", r.status, r.raw)
	}
	// F118-AC-03: a blocked edit keeps revision 2.
	f.validator.set(message.Block, "")
	if r := f.edit(an, id, 2, "noi dung bi chan"); r.code() != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("blocked edit: %d %s", r.status, r.raw)
	}
	// A held edit: the sender sees its proposal held, the others keep revision 2; evidence row state held.
	f.validator.set(message.HoldForReview, "")
	r = f.edit(an, id, 2, "noi dung cho duyet")
	if r.status != http.StatusOK || r.body["moderation_state"] != "held" {
		t.Fatalf("held edit: %d %s", r.status, r.raw)
	}
	f.validator.set(message.Allow, "")
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.message_revisions WHERE message_id = $1 AND state = 'held'`, id); n != 1 {
		t.Fatal("held edit not kept as evidence")
	}
	h := f.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil)
	if d := h.data(); h.status != http.StatusOK || len(d) != 1 || d[0]["body"] != "chao ca lop minh" || d[0]["revision"] != float64(2) {
		t.Fatalf("others must keep revision 2: %s", h.raw)
	}
	if len(f.events("chat.message_held")) != 1 {
		t.Fatal("held edit must open a case (chat.message_held)")
	}
	// Past chat_edit_window (15 minutes from created_at).
	f.clock.Add(15 * time.Minute)
	if r := f.edit(an, id, 2, "qua han"); r.status != http.StatusConflict || r.code() != "CHAT_EDIT_WINDOW_EXPIRED" {
		t.Fatalf("past the window: %d %s", r.status, r.raw)
	}
	// WS chat.message.edit takes the same path (revision_expected).
	id2 := f.sendText(an, ch, "tin thu hai", nil)
	rep := f.ws(an, "c1", "chat.message.edit", map[string]any{"message_id": id2, "body": "tin thu hai da sua", "revision_expected": 1})
	if rep.Problem != nil {
		t.Fatalf("ws edit: %+v", rep.Problem)
	}
	if code := replyCode(f.ws(an, "c1", "chat.message.edit", map[string]any{"message_id": id2, "body": "x", "revision_expected": 1})); code != "VERSION_MISMATCH" {
		t.Fatalf("ws stale edit: %s", code)
	}
}

// T010 — research R1 (EVENT_DESIGN §4.10): chat-persister applies changes by revision; a late older edit changes nothing
// but its replaced version is still kept as evidence; a change of a message not persisted yet waits.
func TestPersisterChangeOrder(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.party(an, binh)
	id := f.sendText(an, ch, "ban dau", nil)
	f.persisted(id)
	ctx := context.Background()
	mod := &message.Moderation{Decision: message.Allow, State: message.StateAllowed}
	b2, b3, b1 := "ban hai", "ban ba", "ban dau"
	mk := func(rev int, body, prevBody string) message.Change {
		return message.Change{V: 1, Kind: message.ChangeKind, Op: message.OpEdit, ChangeID: uuid.New(), TenantID: f.tenantA, ChannelID: ch,
			ChannelType: "party", IsGameplay: true, MessageID: id, ActorID: an.id, Revision: rev, BodyDisplay: &body, Moderation: mod,
			Previous: &message.Previous{Revision: rev - 1, BodyDisplay: &prevBody, Decision: message.Allow}, At: f.clock.Now()}
	}
	if out, err := f.apply.Apply(ctx, mk(3, b3, b2)); err != nil || out != write.ChangeApplied {
		t.Fatalf("revision 3 first: %v %v", out, err)
	}
	if out, err := f.apply.Apply(ctx, mk(2, b2, b1)); err != nil || out != write.ChangeStale {
		t.Fatalf("late revision 2: %v %v", out, err)
	}
	if n := f.count(`SELECT count(*) FROM chat.gameplay_messages WHERE message_id = $1 AND revision = 3 AND body = 'ban ba'`, id); n != 1 {
		t.Fatal("the late edit overwrote the newer revision")
	}
	if n := f.count(`SELECT count(*) FROM chat.message_revisions WHERE message_id = $1 AND state = 'previous'
		AND ((revision = 1 AND body = 'ban dau') OR (revision = 2 AND body = 'ban hai'))`, id); n != 2 {
		t.Fatalf("evidence rows: %d (want revisions 1 and 2)", n)
	}
	missing := mk(2, b2, b1)
	missing.MessageID = uuid.New()
	if out, err := f.apply.Apply(ctx, missing); err != nil || out != write.ChangeMissing {
		t.Fatalf("unknown message: %v %v", out, err)
	}
}
