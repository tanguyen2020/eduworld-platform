package policy

// DirectChannel is the channel type of a direct message conversation.
const DirectChannel = "direct"

// DirectMessageEnabled reports whether the document of one level leaves the direct channel enabled (delta F112-7: the
// direct_message_enabled of chat_policy.updated that module identity projects into PlayerBootstrap): false when the direct
// rule of the level is disabled or lists allowed_kinds without one valid kind; at the platform level also false without
// a direct rule (Intersect denies a channel type without a platform rule). A lower level without a direct rule tightens
// nothing (true). The intersection of allowed_kinds across levels, the allowed hours and the sanctions are not part of it:
// the value is for display, the send path checks every message (CHAT_SYSTEM §3).
func DirectMessageEnabled(d Document, platform bool) bool {
	r, ok := d.rule(DirectChannel)
	if !ok {
		return !platform
	}
	if r.Enabled != nil && !*r.Enabled {
		return false
	}
	if r.AllowedKinds != nil && len(validKinds(r.AllowedKinds)) == 0 {
		return false
	}
	if platform && len(validKinds(r.AllowedKinds)) == 0 {
		return false
	}
	return true
}
