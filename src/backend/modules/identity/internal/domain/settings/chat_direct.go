package settings

// Levels of the chat direct-message projection (delta F112-7; the levels of the chat policy of module chat, CHAT_SYSTEM
// §3: platform → common built-in level of a tenant common → tenant → school).
const (
	ChatLevelPlatform = "platform"
	ChatLevelCommon   = "common"
	ChatLevelTenant   = "tenant"
	ChatLevelSchool   = "school"
)

// ChatDirectLevels are the projected direct_message_enabled values of the levels of one student (nil: nothing projected
// for that level yet).
type ChatDirectLevels struct {
	Platform *bool
	Common   *bool
	Tenant   *bool
	School   *bool
}

// DirectMessageEnabled resolves PlayerBootstrap.policies.chat.direct_message_enabled the way module chat resolves the
// enabled flag of a channel type (module identity keeps its own copy of the rule — modules never share internals,
// ADR-0074; same as the curfew rule kept by module chat): an AND across levels, so any level that disables direct wins.
// nil (field omitted, the client shows the button and the server decides) while the platform level is not projected.
// A tenant common without a projected override keeps the built-in default of module chat: direct on
// (common_tenant_social_defaults, owner decision 2026-11-28, CHILD_SAFETY §3.6); only a projected System Admin override
// that switched direct off turns it off.
func DirectMessageEnabled(l ChatDirectLevels, commonTenant bool) *bool {
	if l.Platform == nil {
		return nil
	}
	off := false
	if !*l.Platform {
		return &off
	}
	if commonTenant && l.Common != nil && !*l.Common {
		return &off
	}
	for _, v := range []*bool{l.Tenant, l.School} {
		if v != nil && !*v {
			return &off
		}
	}
	on := true
	return &on
}
