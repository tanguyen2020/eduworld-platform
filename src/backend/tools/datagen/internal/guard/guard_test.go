package guard

import (
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
)

// F132-AC-07 (T007): datagen refuses production, an unnamed environment and a non-perf tenant prefix.
func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		env, prefix string
		ok          bool
	}{
		{"staging", "perf", true},
		{"dev", "perf-lab", true},
		{"local", "perf", true},
		{"production", "perf", false},
		{"", "perf", false},
		{"staging", "th-demo", false},
		{"staging", "", false},
	} {
		err := Check(tc.env, tc.prefix)
		if (err == nil) != tc.ok {
			t.Fatalf("Check(%q, %q) = %v, want ok=%v", tc.env, tc.prefix, err, tc.ok)
		}
		if err != nil && !errors.Is(err, envguard.ErrRefused) {
			t.Fatalf("Check(%q, %q) = %v, want ErrRefused", tc.env, tc.prefix, err)
		}
	}
}
