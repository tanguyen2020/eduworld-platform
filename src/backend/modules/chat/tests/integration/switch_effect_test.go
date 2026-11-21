//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// failingRead is a TEST DOUBLE (R17): PostgreSQL unavailable.
type failingRead struct{}

func (failingRead) InTenantRead(context.Context, tenancy.Scope, func(ctx context.Context) error) error {
	return errors.New("postgres down")
}

// stepsSkipped returns steps_skipped of the decision of a stored message (written by chat-persister).
func (c *chatEnv) stepsSkipped(result map[string]any) []string {
	c.t.Helper()
	id, _ := result["message_id"].(string)
	if c.waitCount(1, 10*time.Second, `SELECT count(*) FROM chat.moderation_decisions WHERE message_id = $1`, id) != 1 {
		c.t.Fatalf("no decision for message %s", id)
	}
	var raw string
	if err := c.su.QueryRowContext(context.Background(), `SELECT array_to_string(steps_skipped, ',') FROM chat.moderation_decisions
		WHERE message_id = $1`, id).Scan(&raw); err != nil {
		c.t.Fatal(err)
	}
	if raw == "" {
		return nil
	}
	out := strings.Split(raw, ",")
	slices.Sort(out)
	return out
}

// T027 / F116-AC-22, F116-AC-23, F116-AC-24 (TC-CHAT-013, TC-CHAT-014): effective(step) = platform ∧ tenant; a step off
// for a tenant skips that step only (steps_skipped), the other tenant keeps it; the cached switches lost → PostgreSQL,
// never off by default; with enabled_all off the length limit, rate limit, channel rights, sanctions and user blocks
// still apply and the message is unchecked.
func TestSwitchEffect(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	c.publishPlatform(defaultTerms()...)
	sa, sa2 := c.platformStaff("system_admin"), c.platformStaff("system_admin")
	tA, tB := c.tenantA, c.tenantB
	an, binh := c.student(tA, c.schoolA), c.student(tA, c.schoolA)
	dung, em, giang := c.student(tB, c.schoolB), c.student(tB, c.schoolB), c.student(tB, c.schoolB)
	chA, chB := c.open(channel.Party, an, binh), c.open(channel.Party, dung, em)

	// Platform: bullying off; tenant A: spam off (each approved by a second System Admin).
	c.disable(sa, sa2, nil, true, []string{"bullying"}, 2*time.Hour)
	c.disable(sa, sa2, &tA, true, []string{"spam"}, 2*time.Hour)
	c.reload() // this test runs no notification subscriber: drop the cached switches of the tenants

	// AND: tenant A skips spam and bullying, tenant B only bullying (platform), F116-AC-22/23.
	var lastA, lastB map[string]any
	for i := 1; i <= 3; i++ {
		c.clock.Add(11 * time.Second)
		r := c.sendWS(an, "conn-an", chA, "Cùng một tin nè")
		if !r.OK {
			t.Fatalf("tenant A repeat %d (spam off): %+v", i, r.Problem)
		}
		lastA = resultOf(t, r.Result)
		rb := c.sendWS(dung, "conn-dung", chB, "Cùng một tin nè")
		if i < 3 && !rb.OK {
			t.Fatalf("tenant B repeat %d: %+v", i, rb.Problem)
		}
		if i == 3 && (replyCode(rb) != "CHAT_CONTENT_BLOCKED" || rb.Problem.Details["category_code"] != "spam") {
			t.Fatalf("tenant B third repeat (spam on): %+v", rb.Problem)
		}
		if i == 2 {
			lastB = resultOf(t, rb.Result)
		}
	}
	if got := c.stepsSkipped(lastA); !slices.Equal(got, []string{"bullying", "spam"}) || lastA["moderation_state"] != "allowed" {
		t.Fatalf("tenant A steps_skipped %v state %v", got, lastA["moderation_state"])
	}
	if got := c.stepsSkipped(lastB); !slices.Equal(got, []string{"bullying"}) {
		t.Fatalf("tenant B steps_skipped %v", got)
	}
	// Only the switched-off steps are skipped: the catalogue and PII still run in tenant A.
	c.clock.Add(11 * time.Second)
	if r := c.sendWS(an, "conn-an", chA, "đồ zqơrốp"); replyCode(r) != "CHAT_CONTENT_BLOCKED" || r.Problem.Details["category_code"] != "insult" {
		t.Fatalf("catalogue with spam off: %+v", r)
	}
	c.clock.Add(11 * time.Second)
	if r := c.sendWS(binh, "conn-binh", chA, "gọi mình 0912345678"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("PII with spam off: %+v", r)
	}

	// F116-AC-22: the cached switches lost → PostgreSQL (still off as configured), never "every step off" by default.
	ctx := context.Background()
	if err := c.facts.Cache.Invalidate(ctx, service.SwitchKey(tA)); err != nil {
		t.Fatal(err)
	}
	down, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{c.vkAddr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	down.Close() // Valkey unreachable
	noCache := service.Switches{Read: c.read, Reader: persistence.ModerationReader{}, Cache: cache.Facts{C: down, StaleTTL: time.Hour},
		Now: c.clock.Now}
	if e, _ := noCache.Effective(ctx, tA); !slices.Equal(e.Disabled(), []string{"spam", "bullying"}) {
		t.Fatalf("cache lost: %v", e.Disabled())
	}
	nothing := service.Switches{Read: failingRead{}, Reader: persistence.ModerationReader{}, Cache: cache.Facts{C: down, StaleTTL: time.Hour},
		Now: c.clock.Now}
	if e, _ := nothing.Effective(ctx, tA); len(e.Disabled()) != 0 || !e.AllEnabled() {
		t.Fatalf("cache and PostgreSQL lost must mean every step on: %v", e.Disabled())
	}

	// F116-AC-24: enabled_all off for tenant B → the content steps are skipped (unchecked) but the technical limits stay.
	c.disable(sa, sa2, &tB, false, nil, time.Hour)
	c.reload()
	c.clock.Add(time.Minute)
	r := c.sendWS(dung, "conn-dung", chB, "đồ zqơrốp")
	if !r.OK || resultOf(t, r.Result)["moderation_state"] != "unchecked" {
		t.Fatalf("enabled_all off: %+v %s", r.Problem, r.Result)
	}
	// tenant A is not affected by tenant B.
	c.clock.Add(11 * time.Second)
	if r := c.sendWS(binh, "conn-binh", chA, "đồ zqơrốp"); replyCode(r) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("tenant A with tenant B off: %+v", r)
	}
	// length limit
	c.clock.Add(11 * time.Second)
	if r := c.sendWS(em, "conn-em", chB, strings.Repeat("dài ", 600)); replyCode(r) != "VALIDATION_FAILED" {
		t.Fatalf("too long with enabled_all off: %+v", r)
	}
	// channel rights: not a member → NOT_FOUND
	if r := c.sendWS(giang, "conn-giang", chB, "cho mình vào với"); replyCode(r) != "NOT_FOUND" {
		t.Fatalf("non member with enabled_all off: %+v", r)
	}
	// rate limit: burst of 3 per 10 s
	c.clock.Add(time.Minute)
	pace := c.pace
	c.pace = 0
	for i := 1; i <= 3; i++ {
		if r := c.sendWS(em, "conn-em", chB, "nhanh "+string(rune('0'+i))); !r.OK {
			t.Fatalf("burst %d: %+v", i, r.Problem)
		}
	}
	if r := c.sendWS(em, "conn-em", chB, "nhanh 4"); replyCode(r) != "CHAT_RATE_LIMITED" {
		t.Fatalf("rate limit with enabled_all off: %+v", r)
	}
	c.pace = pace
	// sanction: an active mute → CHAT_MUTED
	now := c.clock.Now()
	c.exec(`INSERT INTO chat.sanctions (sanction_id, tenant_id, user_id, type, scope, reason, starts_at, ends_at)
		VALUES ($1, $2, $3, 'mute', 'all', 'integration test mute', $4, $5)`, uuid.New(), tB, giang.id, now.Add(-time.Minute), now.Add(time.Hour))
	chG := c.open(channel.Party, giang, dung)
	if r := c.sendWS(giang, "conn-giang", chG, "xin chào"); replyCode(r) != "CHAT_MUTED" {
		t.Fatalf("mute with enabled_all off: %+v", r)
	}
	// user block: a member who blocked the sender is not a recipient.
	if r := c.call(em, http.MethodPut, "/social/blocks/"+dung.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("block: %d %s", r.status, r.raw)
	}
	hoa := c.student(tB, c.schoolB)
	chBlock := c.open(channel.Party, dung, em, hoa)
	fan := channelInbox(t, c.vk, chBlock)
	c.clock.Add(time.Minute)
	if r := c.sendWS(dung, "conn-dung", chBlock, "chào hai bạn"); !r.OK {
		t.Fatalf("send with a blocker: %+v", r.Problem)
	}
	env := next(fan, "chat.message.created", 3*time.Second)
	if env == nil || env.Target == nil || containsID(env.Target.UserIDs, em.id) || !containsID(env.Target.UserIDs, hoa.id) {
		t.Fatalf("recipients with a blocker: %+v", env)
	}
}
