//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// checkMessageCreatedSchema validates every chat.message_created of the outbox against the envelope and payload
// schemas (contract first) and the stream envelope against chat-envelope.v1.
func checkMessageCreatedSchema(t *testing.T, c *chatEnv) {
	t.Helper()
	root := filepath.Join(testkit.RepoRoot(t), "specs", "events")
	comp := jsonschema.NewCompiler()
	comp.AssertFormat()
	payload, err := comp.Compile(filepath.Join(root, "payloads", "chat", "chat.message_created.v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range c.events("chat.message_created") {
		b, _ := json.Marshal(p)
		v, _ := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
		if err := payload.Validate(v); err != nil {
			t.Fatalf("chat.message_created payload: %v", err)
		}
	}
	envSchema, err := comp.Compile(filepath.Join(root, "chat-envelope.v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	cons, err := c.js.OrderedConsumer(context.Background(), "CHAT_MESSAGES", jetstreamOrdered())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := cons.FetchNoWait(50)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for m := range batch.Messages() {
		v, err := jsonschema.UnmarshalJSON(strings.NewReader(string(m.Data())))
		if err != nil {
			t.Fatal(err)
		}
		if err := envSchema.Validate(v); err != nil {
			t.Fatalf("stream envelope: %v", err)
		}
		if m.Headers().Get("Nats-Msg-Id") == "" {
			t.Fatal("envelope without Nats-Msg-Id")
		}
		n++
	}
	if n == 0 {
		t.Fatal("no envelope validated")
	}
}

// T022 — F115-AC-10, AC-11, AC-12, AC-13 (TC-CHAT-008, TC-CHAT-009, TC-TEN-011, TC-AUTH-014): a student who is not a
// member (same tenant, guessed id, other tenant) gets 404 / NOT_FOUND on every route and command, never learning whether
// the channel exists; X-Tenant-Id of another tenant is 403 TENANT_MISMATCH; staff tokens get GAME_ACCESS_STUDENT_ONLY;
// RLS of tenant B returns 0 rows of the channel of A.
func TestChatAccess(t *testing.T) {
	c := setupChat(t, chatOpts{})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	chi := c.student(c.tenantA, c.schoolA2) // same tenant, not a member
	dung := c.student(c.tenantB, c.schoolB) // other tenant
	ch := c.open(channel.Class, an, binh)
	if r := c.sendWS(an, "c1", ch, "xin chao lop"); !r.OK {
		t.Fatalf("member send: %+v", r.Problem)
	}
	guessed := uuid.New()
	for _, s := range []student{chi, dung} {
		for _, id := range []uuid.UUID{ch, guessed} {
			path := "/chat/channels/" + id.String() + "/messages"
			if r := c.call(s, http.MethodGet, path, "", nil); r.status != http.StatusNotFound || r.code() != "NOT_FOUND" {
				t.Fatalf("list by non member: %d %s", r.status, r.raw)
			}
			r := c.call(s, http.MethodPost, path, newKey(), map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "x"})
			if r.status != http.StatusNotFound || r.code() != "NOT_FOUND" {
				t.Fatalf("send by non member: %d %s", r.status, r.raw)
			}
			for typ, p := range map[string]any{"chat.channel.join": map[string]any{"channel_id": id},
				"chat.history.fetch": map[string]any{"channel_id": id},
				"chat.message.send":  map[string]any{"channel_id": id, "client_message_id": uuid.New(), "kind": "text", "body": "x"}} {
				if r := c.ws(s, "cx", typ, p); r.OK || replyCode(r) != "NOT_FOUND" || len(r.Subscribe) != 0 {
					t.Fatalf("%s by non member: %+v", typ, r)
				}
			}
		}
	}
	if !containsAudit(c.audits(), "chat.channel.access_denied") {
		t.Fatalf("no access denial audit: %v", c.audits())
	}
	// listMyChannels: only the caller's channels.
	if r := c.call(chi, http.MethodGet, "/chat/channels", "", nil); r.status != http.StatusOK || len(r.data()) != 0 {
		t.Fatalf("listMyChannels non member: %d %s", r.status, r.raw)
	}
	mine := c.call(an, http.MethodGet, "/chat/channels", "", nil)
	if len(mine.data()) != 1 || mine.data()[0]["channel_id"] != ch.String() || len(mine.data()[0]["allowed_kinds"].([]any)) != 5 {
		t.Fatalf("listMyChannels: %s", mine.raw)
	}
	// X-Tenant-Id of tenant B with a token of A → TENANT_MISMATCH (F115-AC-12).
	for _, op := range []struct{ method, path string }{{http.MethodGet, "/chat/channels"},
		{http.MethodGet, "/chat/channels/" + ch.String() + "/messages"}, {http.MethodPost, "/chat/channels/" + ch.String() + "/messages"}} {
		r := c.do(op.method, op.path, an.token, map[string]string{"X-Tenant-Id": c.tenantB.String(), "Idempotency-Key": newKey()},
			map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "x"})
		if r.status != http.StatusForbidden || r.code() != "TENANT_MISMATCH" {
			t.Fatalf("%s %s with tenant B: %d %s", op.method, op.path, r.status, r.raw)
		}
	}
	// Staff (5 roles) → GAME_ACCESS_STUDENT_ONLY on every chat route (F115-AC-13).
	tokens := map[string]string{"school_admin": c.staff(c.tenantA, "school_admin"), "academic_manager": c.staff(c.tenantA, "academic_manager"),
		"tenant_admin": c.staff(c.tenantA, "tenant_admin"), "safety_moderator": c.platformStaff("safety_moderator"),
		"system_admin": c.platformStaff("system_admin")}
	for _, id := range chathttp.ChatOperations {
		o := chathttp.Operations[id]
		path := strings.ReplaceAll(o.Pattern, "{channel_id}", ch.String())
		for role, tok := range tokens {
			r := c.do(o.Method, path, tok, map[string]string{"X-Tenant-Id": c.tenantA.String(), "Idempotency-Key": newKey()},
				map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "x"})
			want := "GAME_ACCESS_STUDENT_ONLY"
			if role == "safety_moderator" && r.code() == "TENANT_MISMATCH" {
				want = r.code()
			}
			if r.status != http.StatusForbidden || r.code() != want {
				t.Fatalf("%s by %s: %d %s", id, role, r.status, r.raw)
			}
		}
	}
	// RLS: the session of tenant B sees no row of the channel of A even without the tenant condition (F115-AC-11).
	if n := c.waitCount(1, 10*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch); n != 1 {
		t.Fatalf("persisted %d", n)
	}
	rlsCount := func(tenant uuid.UUID, q string) int {
		tx, err := c.app.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.Exec(`SELECT set_config('app.tenant_id', $1, true)`, tenant.String()); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := tx.QueryRow(q, ch).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, q := range []string{`SELECT count(*) FROM chat.messages WHERE channel_id = $1`, `SELECT count(*) FROM chat.channels WHERE channel_id = $1`,
		`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1`} {
		if rlsCount(c.tenantB, q) != 0 || rlsCount(c.tenantA, q) == 0 {
			t.Fatalf("RLS of %s", q)
		}
	}
	authztest.Cover(t, "listMyChannels", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listChannelMessages", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "sendChannelMessage", authztest.CrossTenant, authztest.RoleDenied)
}

func containsAudit(actions []string, a string) bool {
	for _, x := range actions {
		if x == a {
			return true
		}
	}
	return false
}

// T039 — F115-AC-22 (TC-CHAT-046): during the curfew (clock 21:31 ICT) every send — party, class, direct; REST and WS —
// is CURFEW_ACTIVE with reopen_at, before any seq or publish; at 06:31 sending works again; the channel stays.
func TestSendCurfew(t *testing.T) {
	c := setupChat(t, chatOpts{})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	chans := []uuid.UUID{c.open(channel.Party, an, binh), c.open(channel.Class, an, binh), c.open(channel.Direct, an, binh)}
	c.clock.Set(time.Date(2026, 11, 21, 21, 31, 0, 0, ict))
	before := c.validator.count()
	for _, ch := range chans {
		r := c.sendWS(an, "c1", ch, "khuya roi")
		if r.OK || replyCode(r) != "CURFEW_ACTIVE" {
			t.Fatalf("WS during curfew: %+v", r)
		}
		rest := c.sendREST(an, ch, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "khuya roi"})
		if rest.status != http.StatusForbidden || rest.code() != "CURFEW_ACTIVE" || rest.body["reopen_at"] == nil {
			t.Fatalf("REST during curfew: %d %s", rest.status, rest.raw)
		}
	}
	if c.validator.count() != before {
		t.Fatal("a curfew send reached the pipeline")
	}
	if info, _ := c.js.Stream(context.Background(), "CHAT_MESSAGES"); info.CachedInfo().State.Msgs != 0 {
		t.Fatal("a curfew send was published")
	}
	if n, _ := c.vk.Do(context.Background(), c.vk.B().Exists().Key("ew:chat:seq:"+chans[0].String()).Build()).AsInt64(); n != 0 {
		t.Fatal("a seq was allocated during the curfew")
	}
	// A school window extending the curfew (21:00) applies too; the platform curfew can be projected from identity.
	c.exec(`INSERT INTO ref.schools (school_id, tenant_id, status, settings_subset, source_version)
		VALUES ($1, $2, 'active', '{"curfew_window":{"start":"21:00","end":"07:00"}}', 1)`, c.schoolA, c.tenantA)
	c.clock.Set(time.Date(2026, 11, 22, 6, 45, 0, 0, ict))
	_ = c.facts.Cache.Invalidate(context.Background(), "curfew:"+c.tenantA.String()+":"+c.schoolA.String())
	if r := c.sendWS(an, "c1", chans[0], "som qua"); replyCode(r) != "CURFEW_ACTIVE" {
		t.Fatalf("school window 06:45: %+v", r)
	}
	c.clock.Set(time.Date(2026, 11, 22, 7, 1, 0, 0, ict))
	for _, ch := range chans {
		if r := c.sendWS(an, "c1", ch, "chao buoi sang"); !r.OK {
			t.Fatalf("after curfew: %+v", r.Problem)
		}
	}
}

// T040 — F115-AC-23, F115-AC-09 (TC-CHAT-048, TC-CHAT-010): a school with chat_policy_preset quick_message_emote_only
// refuses text, sticker and gif (CHAT_KIND_NOT_ALLOWED) on party too; quick and emoji pass; another school is not
// affected; students of grade 1 and grade 9 get the same decisions; the platform default without a party rule denies.
func TestSendPresetAndSameRulesForEveryGrade(t *testing.T) {
	c := setupChat(t, chatOpts{})
	young, older := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	c.exec(`UPDATE ref.students SET grade_level = 1 WHERE student_id = $1`, young.id)
	c.exec(`UPDATE ref.students SET grade_level = 9 WHERE student_id = $1`, older.id)
	other := c.student(c.tenantA, c.schoolA2)
	party := c.open(channel.Party, young, older)
	otherParty := c.open(channel.Party, other, c.student(c.tenantA, c.schoolA2))
	// Same decisions for grade 1 and grade 9 without a preset.
	for _, s := range []student{young, older} {
		if r := c.sendWS(s, "c", party, "chao ban"); !r.OK {
			t.Fatalf("grade send: %+v", r.Problem)
		}
	}
	c.exec(`INSERT INTO chat.chat_policies (policy_id, tenant_id, scope_type, scope_id, school_id, rules)
		VALUES ($1, $2, 'school', $3, $3, '{"schema_version":5,"scope_type":"school","scope_id":"`+c.schoolA.String()+`",
		"chat_policy_preset":"quick_message_emote_only","rules":[]}')`, uuid.New(), c.tenantA, c.schoolA)
	_ = c.facts.Cache.Invalidate(context.Background(), "policy:"+c.tenantA.String()+":"+c.schoolA.String()+":-:party")
	sticker := uuid.New()
	refused := []map[string]any{{"kind": "text", "body": "chao"}, {"kind": "sticker", "sticker_id": sticker}, {"kind": "gif", "gif_id": sticker}}
	allowed := []map[string]any{{"kind": "quick", "quick_message_id": "qm.good_luck"}, {"kind": "emoji", "emoji": "smile"}}
	for _, s := range []student{young, older} {
		for _, p := range refused {
			p["channel_id"], p["client_message_id"] = party, uuid.New()
			if r := c.ws(s, "c", "chat.message.send", p); r.OK || replyCode(r) != "CHAT_KIND_NOT_ALLOWED" {
				t.Fatalf("preset %v: %+v", p["kind"], r)
			}
		}
		for _, p := range allowed {
			p["channel_id"], p["client_message_id"] = party, uuid.New()
			if r := c.ws(s, "c", "chat.message.send", p); !r.OK {
				t.Fatalf("preset %v refused: %+v", p["kind"], r.Problem)
			}
		}
	}
	// REST: same refusal (403 CHAT_KIND_NOT_ALLOWED).
	if r := c.sendREST(young, party, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "chao"}); r.status != http.StatusForbidden ||
		r.code() != "CHAT_KIND_NOT_ALLOWED" {
		t.Fatalf("REST preset: %d %s", r.status, r.raw)
	}
	// Another school of the tenant is not affected.
	if r := c.sendWS(other, "c", otherParty, "truong khac"); !r.OK {
		t.Fatalf("other school: %+v", r.Problem)
	}
	// F115-AC-08: remove the platform party rule (fixture) → CHAT_NOT_ALLOWED for everyone.
	c.exec(`UPDATE chat.chat_policies SET rules = jsonb_set(rules, '{rules}', (SELECT jsonb_agg(r) FROM jsonb_array_elements(rules->'rules') r
		WHERE r->>'channel_type' <> 'party')) WHERE scope_type = 'platform' AND deleted_at IS NULL`)
	_ = c.facts.Cache.Invalidate(context.Background(), "policy:"+c.tenantA.String()+":"+c.schoolA2.String()+":-:party")
	if r := c.sendWS(other, "c", otherParty, "khong duoc"); r.OK || replyCode(r) != "CHAT_NOT_ALLOWED" {
		t.Fatalf("no platform party rule: %+v", r)
	}
	c.validator.set(message.Allow, "")
}
