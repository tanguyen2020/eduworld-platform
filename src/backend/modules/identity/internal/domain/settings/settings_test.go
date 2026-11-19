package settings

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCurfewRules(t *testing.T) {
	platform := Curfew{"21:30", "06:30"}
	if err := platform.Validate("platform_curfew"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []Curfew{{"21:30", "21:30"}, {"25:00", "06:00"}, {"9:30", "06:30"}} {
		if bad.Validate("x") == nil {
			t.Fatalf("%v accepted", bad)
		}
	}
	wider := Curfew{"21:00", "06:30"}
	if !wider.Contains(platform) || platform.Contains(wider) {
		t.Fatal("contains")
	}
	if (Curfew{"22:00", "06:30"}).Contains(platform) || (Curfew{"21:30", "06:00"}).Contains(platform) {
		t.Fatal("a narrower school window must not contain the platform curfew")
	}
	if u := Union(platform, wider); u != wider {
		t.Fatalf("union = %v", u)
	}
	if u := Union(platform, Curfew{}); u != platform {
		t.Fatalf("union with none = %v", u)
	}
}

func TestAllowedHours(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	h := AllowedHours{Windows: []Window{{Days: []string{"mon", "tue", "wed", "thu", "fri"}, From: "07:00", To: "17:00"}}}
	if err := h.Validate("allowed_hours"); err != nil {
		t.Fatal(err)
	}
	thu := time.Date(2026, 11, 19, 10, 0, 0, 0, ict) // Thursday
	if !h.Contains(thu, ict) || h.Contains(thu.Add(10*time.Hour), ict) || h.Contains(thu.AddDate(0, 0, 2), ict) {
		t.Fatal("contains")
	}
	tenant := AllowedHours{Windows: []Window{{Days: []string{"mon", "tue", "wed", "thu", "fri", "sat"}, From: "06:00", To: "21:00"}}}
	if !h.Within(&tenant) || tenant.Within(&h) || !h.Within(nil) {
		t.Fatal("within")
	}
	for _, bad := range []AllowedHours{{}, {Windows: []Window{{Days: []string{"xyz"}, From: "07:00", To: "08:00"}}},
		{Windows: []Window{{Days: []string{"mon"}, From: "09:00", To: "08:00"}}}, {TimeZone: "Mars/Base", Windows: h.Windows}} {
		if bad.Validate("h") == nil {
			t.Fatalf("%+v accepted", bad)
		}
	}
}

func TestValuesAndEffective(t *testing.T) {
	d180, d90, zero := 180, 90, 0
	p := StudentCleanup{DefaultDays: &d180, NoticeDays: 14, AllowTenantOverride: true, ApprovalThresholdPercent: 10}
	if p.Validate() != nil || (StudentCleanup{NoticeDays: 100, ApprovalThresholdPercent: 10}).Validate() == nil {
		t.Fatal("cleanup validate")
	}
	if *EffectiveCleanupDays(p, &d90, true) != 90 || EffectiveCleanupDays(p, &zero, true) != nil || *EffectiveCleanupDays(p, nil, false) != 180 {
		t.Fatal("effective with override")
	}
	p.AllowTenantOverride = false
	if *EffectiveCleanupDays(p, &d90, true) != 180 {
		t.Fatal("override not allowed")
	}
	c := ChatRetention{GameplayChatRetentionDays: 2, EphemeralRetentionHours: 24}
	if !errors.Is(c.Validate(3), ErrRetentionBelowMinimum) {
		t.Fatal("minimum")
	}
}

func TestToggles(t *testing.T) {
	ta := uuid.New()
	items := []CatalogItem{{Key: "school_lms", Kind: "workspace"}, {Key: "menu.s.a", Kind: "menu", Parent: "school_lms"},
		{Key: "action.s.a.view", Kind: "action", Parent: "menu.s.a"}, {Key: "action.s.a.edit", Kind: "action", Parent: "menu.s.a"},
		{Key: "menu.s.b", Kind: "menu", Parent: "school_lms"}}
	// No row = enabled.
	if m, a := Disabled(items, nil, &ta); len(m)+len(a) != 0 {
		t.Fatal("no rows must enable everything")
	}
	flags := map[string]Flag{
		"action.s.a.edit": {Key: "action.s.a.edit", Default: false},
		"menu.s.b":        {Key: "menu.s.b", Default: true, Overrides: map[string]bool{ta.String(): false}},
	}
	m, a := Disabled(items, flags, &ta)
	if len(m) != 1 || m[0] != "menu.s.b" || len(a) != 1 || a[0] != "action.s.a.edit" {
		t.Fatalf("tenant: %v %v", m, a)
	}
	if m, _ := Disabled(items, flags, nil); len(m) != 0 {
		t.Fatal("platform scope uses the platform default")
	}
	// Menu off → its actions off (AND), even an action enabled on its own.
	flags["menu.s.a"] = Flag{Key: "menu.s.a", Default: false}
	flags["action.s.a.view"] = Flag{Key: "action.s.a.view", Default: true}
	_, a = Disabled(items, flags, nil)
	if len(a) != 2 {
		t.Fatalf("actions of a disabled menu: %v", a)
	}
	if KindOfKey("menu.x.y") != "menu" || KindOfKey("action.x.y.z") != "action" || KindOfKey("preview_game_frame") != "feature" {
		t.Fatal("kind of key")
	}
	if o := flags["menu.s.b"].Override(&ta); o == nil || *o {
		t.Fatal("override")
	}
}
