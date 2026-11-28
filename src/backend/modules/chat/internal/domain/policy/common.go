package policy

// CommonOverride is the effective social switches of one tenant common (chat.common_tenant_social_overrides, F120-AC-28):
// each field is the effective state (true = on). Owner decision 2026-11-28 (ADR-0078 §3 partly superseded, ADR-0104):
// direct, world and friends are on by default and the System Admin override is an incident switch that turns each off
// again; Area stays quick + emoji only unless the override allows free text. A stored row keeps its meaning: its values were
// always the effective state, only the defaults of an absent row changed.
type CommonOverride struct {
	DirectEnabled       bool `json:"direct_enabled"`
	WorldEnabled        bool `json:"world_enabled"`
	AreaFreeTextEnabled bool `json:"area_free_text_enabled"`
	FriendsEnabled      bool `json:"friends_enabled"`
}

// DefaultCommonOverride is common_tenant_social_defaults (CONFIG_DEFAULTS, CHILD_SAFETY §3.6): what a tenant common gets
// while System Admin stored no override — direct, world and friends on, Area quick + emoji only.
func DefaultCommonOverride() CommonOverride {
	return CommonOverride{DirectEnabled: true, WorldEnabled: true, AreaFreeTextEnabled: false, FriendsEnabled: true}
}

// CommonDefaults is the built-in level of a tenant common: direct or world off when the override switched them off, Area
// quick + emoji only unless free text is allowed. It sits between platform and tenant (Levels.Common) so that no tenant
// role can lift it: only System Admin writes the override. nil when nothing is restricted. Friends are decided by the
// social scope rule (social.Tenant.CommonFriendsEnabled), not by a channel rule.
func CommonDefaults(o CommonOverride) *Document {
	off := false
	d := Document{ScopeType: "common"}
	if !o.DirectEnabled {
		d.Rules = append(d.Rules, Rule{ChannelType: "direct", Enabled: &off})
	}
	if !o.WorldEnabled {
		d.Rules = append(d.Rules, Rule{ChannelType: "world", Enabled: &off})
	}
	if !o.AreaFreeTextEnabled {
		d.Rules = append(d.Rules, Rule{ChannelType: "area", AllowedKinds: []Kind{KindQuick, KindEmoji}})
	}
	if len(d.Rules) == 0 {
		return nil
	}
	return &d
}
