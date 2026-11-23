package policy

// CommonOverride is what System Admin re-enabled for one tenant common (chat.common_tenant_social_overrides, F120-AC-28);
// the zero value is the defaults of common_tenant_social_defaults (ADR-0078, CHILD_SAFETY §3.6).
type CommonOverride struct {
	DirectEnabled       bool `json:"direct_enabled"`
	WorldEnabled        bool `json:"world_enabled"`
	AreaFreeTextEnabled bool `json:"area_free_text_enabled"`
}

// CommonDefaults is the built-in level of a tenant common: direct and world off, Area quick + emoji only, unless the
// override re-enables them. It sits between platform and tenant (Levels.Common) so that no tenant role can lift it: only
// System Admin writes the override. nil when everything is re-enabled.
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
