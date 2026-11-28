package settings

import "testing"

// Delta F112-7: an AND across the projected levels; omitted while the platform level is unknown. Owner 2026-11-28: a tenant
// common has direct on by default (common_tenant_social_defaults); only a projected System Admin override turns it off.
func TestDirectMessageEnabled(t *testing.T) {
	on, off := true, false
	for _, c := range []struct {
		name   string
		l      ChatDirectLevels
		common bool
		want   *bool
	}{
		{"nothing projected", ChatDirectLevels{}, false, nil},
		{"school known, platform unknown", ChatDirectLevels{School: &off}, false, nil},
		{"platform on", ChatDirectLevels{Platform: &on}, false, &on},
		{"platform off", ChatDirectLevels{Platform: &off, Tenant: &on, School: &on}, false, &off},
		{"tenant off", ChatDirectLevels{Platform: &on, Tenant: &off, School: &on}, false, &off},
		{"school off", ChatDirectLevels{Platform: &on, Tenant: &on, School: &off}, false, &off},
		{"all on", ChatDirectLevels{Platform: &on, Tenant: &on, School: &on}, false, &on},
		{"common default (on, owner 2026-11-28)", ChatDirectLevels{Platform: &on}, true, &on},
		{"common default, platform off", ChatDirectLevels{Platform: &off}, true, &off},
		{"common override off", ChatDirectLevels{Platform: &on, Common: &off}, true, &off},
		{"common override on", ChatDirectLevels{Platform: &on, Common: &on}, true, &on},
		{"common on, tenant off", ChatDirectLevels{Platform: &on, Common: &on, Tenant: &off}, true, &off},
		{"common row ignored for another tenant", ChatDirectLevels{Platform: &on, Common: &off}, false, &on},
	} {
		got := DirectMessageEnabled(c.l, c.common)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}
