//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/gameauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// deliverChat hands one chat policy event of module chat to identity.chat_direct_policy in the scope pkg/outbox uses: the
// envelope tenant, or the platform scope of the consumer for a platform event.
func deliverChat(ctx context.Context, t *testing.T, e *env, c consumer.ChatDirectPolicy, typ string, tenant *uuid.UUID, payload map[string]any) error {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ev := outbox.Envelope{EventID: ids.New(), EventType: typ, SchemaVersion: 1, OccurredAt: time.Now().UTC(), RecordedAt: time.Now().UTC(),
		Producer: "chat/core", TenantID: tenant, Payload: raw}
	if tenant == nil {
		ps, err := authz.SystemPlatformScope("consumer-identity-chat-direct-policy")
		if err != nil {
			t.Fatal(err)
		}
		return e.uow.InTenantTx(ctx, ps, func(ctx context.Context) error { return c.Handle(ctx, ev) })
	}
	sc, err := authz.EventScope(*tenant)
	if err != nil {
		t.Fatal(err)
	}
	return e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return c.Handle(ctx, ev) })
}

// scopeCtx is the TenantScope of tenant for a direct read of the projection.
func scopeCtx(ctx context.Context, t *testing.T, tenant string) context.Context {
	t.Helper()
	sc, err := authz.EventScope(uuid.MustParse(tenant))
	if err != nil {
		t.Fatal(err)
	}
	return tenancy.WithScope(ctx, sc)
}

func directOf(t *testing.T, r gameResp) any {
	t.Helper()
	b, ok := r.body["bootstrap"].(map[string]any)
	if !ok {
		t.Fatalf("no bootstrap: %d %v", r.code, r.body)
	}
	pol, _ := b["policies"].(map[string]any)
	chat, ok := pol["chat"].(map[string]any)
	if !ok {
		return nil
	}
	return chat["direct_message_enabled"]
}

// Delta F112-7 (F013): PlayerBootstrap.policies.chat.direct_message_enabled comes from the projection of the chat policy
// events of module chat — absent until the platform level is projected; any level that turns direct off wins (platform,
// tenant, school of the student; a tenant common stays off until System Admin re-enables direct); an older policy version
// never overwrites a newer one; the rows of a tenant are invisible to another tenant (RLS); a chat_policy.updated
// published before the delta changes nothing; a payload whose tenant differs from the envelope is refused (DLQ).
func TestChatDirectPolicyBootstrap(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	g.auth.Chat = gameauth.ChatDirectPolicy{Reader: read.ChatDirectModel{Session: g.read}}
	c := consumer.ChatDirectPolicy{Record: command.RecordChatDirectPolicy{Policies: write.ChatDirectPolicyRepo{}}}
	a := seedTenant(ctx, t, g.platform, "pgd-dm-a")
	b := seedTenant(ctx, t, g.platform, "pgd-dm-b")
	g.student(ctx, t, a, "hs.nhan.tin")
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	school := uuid.MustParse(a.school)
	dev := uuid.New()
	updated := func(scope string, tenant *uuid.UUID, scopeID *uuid.UUID, version int, enabled *bool) error {
		p := map[string]any{"policy_id": uuid.NewString(), "scope_type": scope, "policy_version": version, "changed_fields": []string{"rules"},
			"actor_id": uuid.NewString(), "occurred_at": time.Now().UTC()}
		if tenant != nil {
			p["tenant_id"] = tenant.String()
		}
		if scopeID != nil {
			p["scope_id"] = scopeID.String()
		}
		if enabled != nil {
			p["direct_message_enabled"] = *enabled
		}
		return deliverChat(ctx, t, g.env, c, "chat_policy.updated", tenant, p)
	}
	on, off := true, false
	login := func(want any) {
		t.Helper()
		r := g.login(t, "hs.nhan.tin", dev)
		if r.code != 200 || directOf(t, r) != want {
			t.Fatalf("direct_message_enabled: %d %v (want %v)", r.code, directOf(t, r), want)
		}
	}
	// Nothing projected: the field is omitted (the client shows the button, module chat decides); an event published before
	// the delta projects nothing.
	login(nil)
	if err := updated("platform", nil, nil, 1, nil); err != nil {
		t.Fatal(err)
	}
	login(nil)
	if err := updated("platform", nil, nil, 2, &on); err != nil {
		t.Fatal(err)
	}
	login(true)
	if err := updated("school", &ta, &school, 1, &off); err != nil {
		t.Fatal(err)
	}
	login(false)
	if err := updated("school", &ta, &school, 1, &on); err != nil { // same version again: skipped
		t.Fatal(err)
	}
	login(false)
	if err := updated("school", &ta, &school, 2, &on); err != nil {
		t.Fatal(err)
	}
	login(true)
	if err := updated("tenant", &ta, &ta, 3, &off); err != nil {
		t.Fatal(err)
	}
	login(false)
	if err := updated("tenant", &ta, &ta, 2, &on); err != nil { // an older version arriving late never wins
		t.Fatal(err)
	}
	login(false)
	if err := updated("tenant", &ta, &ta, 4, &on); err != nil {
		t.Fatal(err)
	}
	if err := updated("platform", nil, nil, 3, &off); err != nil {
		t.Fatal(err)
	}
	login(false)
	if err := updated("platform", nil, nil, 4, &on); err != nil {
		t.Fatal(err)
	}
	login(true)
	// Tenant B turns direct off for itself only; its rows are invisible from tenant A's scope (RLS).
	if err := updated("tenant", &tb, &tb, 1, &off); err != nil {
		t.Fatal(err)
	}
	login(true)
	m := read.ChatDirectModel{Session: g.read}
	levels, err := m.ChatDirectLevels(scopeCtx(ctx, t, a.tenant), tb, nil)
	if err != nil || levels.Tenant != nil {
		t.Fatalf("tenant B rows from tenant A's scope: %+v %v", levels, err)
	}
	// A payload whose tenant differs from the envelope, or a platform level inside a tenant event, is refused.
	if err := deliverChat(ctx, t, g.env, c, "chat_policy.updated", &ta, map[string]any{"policy_id": uuid.NewString(), "scope_type": "tenant",
		"tenant_id": tb.String(), "policy_version": 9, "direct_message_enabled": false}); err == nil {
		t.Fatal("tenant mismatch accepted")
	}
	if err := deliverChat(ctx, t, g.env, c, "chat_policy.updated", &ta, map[string]any{"policy_id": uuid.NewString(), "scope_type": "platform",
		"policy_version": 9, "direct_message_enabled": false}); err == nil {
		t.Fatal("platform level in a tenant event accepted")
	}
	login(true)
	// Tenant common: off by default (no override projected), on once System Admin re-enabled direct.
	common := uuid.MustParse(commonTenant)
	levels, err = m.ChatDirectLevels(scopeCtx(ctx, t, commonTenant), common, nil)
	if err != nil || levels.Common != nil {
		t.Fatalf("common before: %+v %v", levels, err)
	}
	override := func(version int, direct bool) error {
		return deliverChat(ctx, t, g.env, c, "chat_policy.common_tenant_social_updated", &common, map[string]any{"tenant_id": common.String(),
			"direct_enabled": direct, "world_enabled": false, "area_free_text_enabled": false, "version": version,
			"actor_id": uuid.NewString(), "occurred_at": time.Now().UTC()})
	}
	src := gameauth.ChatDirectPolicy{Reader: m}
	if v, err := src.DirectMessageEnabled(ctx, common, nil, true); err != nil || v == nil || *v {
		t.Fatalf("common default: %v %v", v, err)
	}
	if err := override(1, true); err != nil {
		t.Fatal(err)
	}
	if v, err := src.DirectMessageEnabled(ctx, common, nil, true); err != nil || v == nil || !*v {
		t.Fatalf("common re-enabled: %v %v", v, err)
	}
	if err := deliverChat(ctx, t, g.env, c, "chat_policy.common_tenant_social_updated", &ta, map[string]any{"tenant_id": common.String(),
		"direct_enabled": true, "version": 2}); err == nil {
		t.Fatal("common override of another tenant accepted")
	}
}
