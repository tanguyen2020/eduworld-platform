package settingscfg

import (
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

func TestDefaultsFrom(t *testing.T) {
	d, err := DefaultsFrom(config.Load())
	if err != nil || *d.StudentCleanup.DefaultDays != 180 || d.PlatformCurfew.Start != "21:30" || d.ChatMinimumDays != 3 {
		t.Fatalf("defaults = %+v %v", d, err)
	}
	t.Setenv("EW_PLATFORM_CURFEW", "21:00")
	if _, err := DefaultsFrom(config.Load()); err == nil {
		t.Fatal("bad curfew accepted")
	}
}
