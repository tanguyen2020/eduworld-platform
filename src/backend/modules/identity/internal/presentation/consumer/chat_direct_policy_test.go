package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// fakeChatDirect is a TEST DOUBLE of command.RecordChatDirectPolicy (R17): it records the projected levels.
type fakeChatDirect struct {
	got []command.ChatDirectPolicyEvent
}

func (f *fakeChatDirect) Handle(_ context.Context, e command.ChatDirectPolicyEvent) error {
	f.got = append(f.got, e)
	return nil
}

// Delta F112-7: chat_policy.updated and chat_policy.common_tenant_social_updated map to one projected level each; an event
// published before the delta is skipped; a tenant mismatch, a platform level in a tenant event, a school level without
// scope_id or a malformed payload is permanent (DLQ).
func TestChatDirectPolicyConsumer(t *testing.T) {
	tenant, school := uuid.New(), uuid.New()
	env := func(typ string, tid *uuid.UUID, p map[string]any) outbox.Envelope {
		raw, _ := json.Marshal(p)
		return outbox.Envelope{EventID: uuid.New(), EventType: typ, TenantID: tid, Payload: raw}
	}
	f := &fakeChatDirect{}
	c := ChatDirectPolicy{Record: f}
	ctx := context.Background()
	ok := []outbox.Envelope{
		env("chat_policy.updated", nil, map[string]any{"scope_type": "platform", "policy_version": 3, "direct_message_enabled": true}),
		env("chat_policy.updated", &tenant, map[string]any{"scope_type": "tenant", "tenant_id": tenant, "scope_id": tenant, "policy_version": 2,
			"direct_message_enabled": false}),
		env("chat_policy.updated", &tenant, map[string]any{"scope_type": "school", "tenant_id": tenant, "scope_id": school, "policy_version": 4,
			"direct_message_enabled": true}),
		env("chat_policy.common_tenant_social_updated", &tenant, map[string]any{"tenant_id": tenant, "direct_enabled": true, "version": 5}),
		env("chat_policy.updated", nil, map[string]any{"scope_type": "platform", "policy_version": 6}), // before the delta: skipped
	}
	for i, e := range ok {
		if err := c.Handle(ctx, e); err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}
	want := []command.ChatDirectPolicyEvent{
		{Level: "platform", Enabled: true, Version: 3},
		{Level: "tenant", Tenant: &tenant, ScopeID: tenant, Enabled: false, Version: 2},
		{Level: "school", Tenant: &tenant, ScopeID: school, Enabled: true, Version: 4},
		{Level: "common", Tenant: &tenant, ScopeID: tenant, Enabled: true, Version: 5},
	}
	if len(f.got) != len(want) {
		t.Fatalf("projected %d events: %+v", len(f.got), f.got)
	}
	for i, w := range want {
		g := f.got[i]
		if g.Level != w.Level || g.Enabled != w.Enabled || g.Version != w.Version || g.ScopeID != w.ScopeID ||
			(g.Tenant == nil) != (w.Tenant == nil) || (g.Tenant != nil && *g.Tenant != *w.Tenant) {
			t.Errorf("event %d: %+v, want %+v", i, g, w)
		}
	}
	other := uuid.New()
	for name, e := range map[string]outbox.Envelope{
		"tenant mismatch": env("chat_policy.updated", &tenant, map[string]any{"scope_type": "tenant", "tenant_id": other, "policy_version": 1,
			"direct_message_enabled": false}),
		"platform in a tenant event": env("chat_policy.updated", &tenant, map[string]any{"scope_type": "platform", "policy_version": 1,
			"direct_message_enabled": false}),
		"tenant level without tenant": env("chat_policy.updated", nil, map[string]any{"scope_type": "tenant", "policy_version": 1,
			"direct_message_enabled": false}),
		"school without scope": env("chat_policy.updated", &tenant, map[string]any{"scope_type": "school", "policy_version": 1,
			"direct_message_enabled": false}),
		"unknown scope": env("chat_policy.updated", &tenant, map[string]any{"scope_type": "class", "policy_version": 1,
			"direct_message_enabled": false}),
		"no version": env("chat_policy.updated", nil, map[string]any{"scope_type": "platform", "direct_message_enabled": true}),
		"common of another tenant": env("chat_policy.common_tenant_social_updated", &tenant, map[string]any{"tenant_id": other,
			"direct_enabled": true, "version": 1}),
		"common without flag": env("chat_policy.common_tenant_social_updated", &tenant, map[string]any{"tenant_id": tenant, "version": 1}),
		"unexpected type":     env("chat_policy.moderation_switch_changed", nil, map[string]any{}),
	} {
		if err := c.Handle(ctx, e); !errors.Is(err, outbox.ErrPermanent) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
