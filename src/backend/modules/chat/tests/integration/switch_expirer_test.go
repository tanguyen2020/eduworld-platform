//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
)

// T028 / F116-AC-25 (TC-CHAT-016): the job moderation-switch-expirer switches a level back on at its expires_at (history
// source auto_expire, audit, event); eduworld_chat_policy_disabled{tenant,step} is 1 for the whole disabled period and 0
// once back on; a lapsed value already counts as on before the job runs; a second replica in the same cycle does
// nothing (lease ew:lock:job:moderation-switch-expirer).
func TestSwitchExpirer(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	ctx := context.Background()
	sa, sa2 := c.platformStaff("system_admin"), c.platformStaff("system_admin")
	tA := c.tenantA
	gauge := func(level, step string) float64 { return observability.PolicyDisabledValue(level, step) }
	all := []string{"sensitive_terms", "spam", "bullying", "async_analysis"}

	c.disable(sa, sa2, &tA, true, []string{"spam"}, 5*time.Minute)
	c.disable(sa, sa2, nil, false, nil, time.Hour)
	if gauge(tA.String(), "spam") != 1 || gauge(tA.String(), "bullying") != 0 {
		t.Fatalf("gauge tenant A: spam %v bullying %v", gauge(tA.String(), "spam"), gauge(tA.String(), "bullying"))
	}
	for _, s := range all {
		if gauge("platform", s) != 1 {
			t.Fatalf("gauge platform %s: %v", s, gauge("platform", s))
		}
	}
	// Before expires_at: nothing to do, the gauge stays 1.
	c.clock.Add(4 * time.Minute)
	if n, err := c.expiry.Run(ctx); err != nil || n != 0 {
		t.Fatalf("early run: %d %v", n, err)
	}
	if gauge(tA.String(), "spam") != 1 {
		t.Fatal("gauge dropped before expires_at")
	}
	// Past expires_at the pipeline already counts the value as on (never off by default) …
	c.clock.Add(time.Minute)
	c.reload()
	e, _ := c.pipe.Switches.Effective(ctx, tA)
	if e.Tenant == nil || !e.Tenant.On("spam", e.At) { // (the platform level is still off: enabled_all)
		t.Fatalf("lapsed tenant value still off: %+v", e.Tenant)
	}
	// … and the job switches the level back on: auto_expire history, audit, event, gauge 0.
	if n, err := c.expiry.Run(ctx); err != nil || n != 1 {
		t.Fatalf("expiry run: %d %v", n, err)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switches WHERE tenant_id = $1 AND enabled_all AND steps->>'spam' = 'true'
		AND expires_at IS NULL AND deleted_at IS NULL`, tA); n != 1 {
		t.Fatalf("tenant A switch not back on: %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_changes WHERE tenant_id = $1 AND source = 'auto_expire'
		AND actor_id IS NULL AND before->>'spam' = 'false' AND after->>'spam' = 'true'`, tA); n != 1 {
		t.Fatalf("auto_expire history rows %d", n)
	}
	if !slices.Contains(c.audits(), "chat.moderation_switch.auto_expired") {
		t.Fatalf("auto_expire not audited: %v", c.audits())
	}
	sources := map[string]int{}
	for _, p := range c.events("chat_policy.moderation_switch_changed") {
		sources[p["source"].(string)]++
	}
	if sources["auto_expire"] != 1 || sources["approved_request"] != 2 {
		t.Fatalf("event sources: %v", sources)
	}
	if seen := c.checkEvents("chat_policy.moderation_switch_changed"); seen["chat_policy.moderation_switch_changed"] != 3 {
		t.Fatalf("event schema: %v", seen)
	}
	if gauge(tA.String(), "spam") != 0 || gauge("platform", "spam") != 1 {
		t.Fatalf("gauge after tenant expiry: tenant %v platform %v", gauge(tA.String(), "spam"), gauge("platform", "spam"))
	}
	// A second run of the same cycle changes nothing.
	if n, err := c.expiry.Run(ctx); err != nil || n != 0 {
		t.Fatalf("repeat run: %d %v", n, err)
	}
	// The platform level at its expiry; one replica per cycle (lease).
	c.clock.Add(time.Hour)
	leased := command.SwitchExpiry{Admin: c.switches, Lock: cache.JobLock{C: c.vk}}
	if ok, err := (cache.JobLock{C: c.vk}).Acquire(ctx, command.ModerationSwitchExpirerJob, 5*time.Second); err != nil || !ok {
		t.Fatalf("lease: %v %v", ok, err)
	}
	if n, err := leased.Run(ctx); err != nil || n != 0 {
		t.Fatalf("run while another replica holds the lease: %d %v", n, err)
	}
	if n, err := c.expiry.Run(ctx); err != nil || n != 1 {
		t.Fatalf("platform expiry: %d %v", n, err)
	}
	for _, s := range all {
		if gauge("platform", s) != 0 {
			t.Fatalf("gauge platform %s after expiry: %v", s, gauge("platform", s))
		}
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_switch_changes WHERE tenant_id IS NULL AND source = 'auto_expire'
		AND enabled_all_before = false AND enabled_all_after = true`); n != 1 {
		t.Fatalf("platform auto_expire rows %d", n)
	}
}
