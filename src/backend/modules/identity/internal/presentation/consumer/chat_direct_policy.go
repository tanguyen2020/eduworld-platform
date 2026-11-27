package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// ChatDirectPolicyName is the durable consumer that projects the direct-message setting of the chat policy levels of
// module chat for PlayerBootstrap.policies.chat.direct_message_enabled (delta F112-7).
const ChatDirectPolicyName = "identity.chat_direct_policy"

// ChatDirectPolicySubjects are the events it handles (stream EVT_CHAT, owner F120).
var ChatDirectPolicySubjects = []string{"ew.evt.chat.chat_policy.updated", "ew.evt.chat.chat_policy.common_tenant_social_updated"}

// ChatDirectPolicyRecorder is command.RecordChatDirectPolicy.
type ChatDirectPolicyRecorder interface {
	Handle(ctx context.Context, e command.ChatDirectPolicyEvent) error
}

// ChatDirectPolicy dispatches the two chat policy events to command.RecordChatDirectPolicy in the consumer transaction
// (platform scope for the platform level, TenantScope of the envelope otherwise). A chat_policy.updated published before
// the delta carries no direct_message_enabled and is skipped; a payload whose tenant differs from the envelope, or a
// malformed one, is dead-lettered.
type ChatDirectPolicy struct {
	Record ChatDirectPolicyRecorder
}

type chatPolicyUpdated struct {
	TenantID             *uuid.UUID `json:"tenant_id"`
	ScopeType            string     `json:"scope_type"`
	ScopeID              *uuid.UUID `json:"scope_id"`
	PolicyVersion        int64      `json:"policy_version"`
	DirectMessageEnabled *bool      `json:"direct_message_enabled"`
}

type commonSocialUpdated struct {
	TenantID      uuid.UUID `json:"tenant_id"`
	DirectEnabled *bool     `json:"direct_enabled"`
	Version       int64     `json:"version"`
}

// Handle implements outbox.Handler.
func (c ChatDirectPolicy) Handle(ctx context.Context, e outbox.Envelope) error {
	switch e.EventType {
	case "chat_policy.updated":
		return c.policyUpdated(ctx, e)
	case "chat_policy.common_tenant_social_updated":
		var p commonSocialUpdated
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.TenantID == uuid.Nil || p.DirectEnabled == nil || p.Version < 1 {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
		}
		if e.TenantID == nil || *e.TenantID != p.TenantID {
			return outbox.Permanent(fmt.Errorf("%s: payload tenant differs from the envelope", e.EventType))
		}
		t := p.TenantID
		return c.Record.Handle(ctx, command.ChatDirectPolicyEvent{Level: settings.ChatLevelCommon, Tenant: &t, ScopeID: t,
			Enabled: *p.DirectEnabled, Version: p.Version})
	}
	return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
}

func (c ChatDirectPolicy) policyUpdated(ctx context.Context, e outbox.Envelope) error {
	var p chatPolicyUpdated
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.PolicyVersion < 1 {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	if p.DirectMessageEnabled == nil {
		return nil // published before delta F112-7: nothing to project
	}
	ev := command.ChatDirectPolicyEvent{Level: p.ScopeType, Enabled: *p.DirectMessageEnabled, Version: p.PolicyVersion}
	switch p.ScopeType {
	case settings.ChatLevelPlatform:
		if e.TenantID != nil {
			return outbox.Permanent(fmt.Errorf("%s: platform level in a tenant event", e.EventType))
		}
	case settings.ChatLevelTenant, settings.ChatLevelSchool:
		if e.TenantID == nil || (p.TenantID != nil && *p.TenantID != *e.TenantID) {
			return outbox.Permanent(fmt.Errorf("%s: payload tenant differs from the envelope", e.EventType))
		}
		t := *e.TenantID
		ev.Tenant, ev.ScopeID = &t, t
		if p.ScopeType == settings.ChatLevelSchool {
			if p.ScopeID == nil || *p.ScopeID == uuid.Nil {
				return outbox.Permanent(fmt.Errorf("%s: school level without scope_id", e.EventType))
			}
			ev.ScopeID = *p.ScopeID
		}
	default:
		return outbox.Permanent(fmt.Errorf("%s: unknown scope_type %q", e.EventType, p.ScopeType))
	}
	return c.Record.Handle(ctx, ev)
}
