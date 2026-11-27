package envguard

import (
	"errors"
	"testing"
)

// F132-AC-07: the tooling refuses production and anything that is not named as a test environment.
func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		env string
		ok  bool
	}{
		{"local", true}, {"dev", true}, {"staging", true}, {" Staging ", true},
		{"production", false}, {"prod", false}, {"PRODUCTION", false}, {"", false}, {"stage", false}, {"staging-eu", false},
	} {
		err := Check(tc.env)
		if (err == nil) != tc.ok {
			t.Fatalf("Check(%q) = %v, want ok=%v", tc.env, err, tc.ok)
		}
		if err != nil && !errors.Is(err, ErrRefused) {
			t.Fatalf("Check(%q) error %v is not ErrRefused", tc.env, err)
		}
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(Var, "production")
	if _, err := FromEnv(); err == nil {
		t.Fatal("production accepted")
	}
	t.Setenv(Var, "Staging")
	if v, err := FromEnv(); err != nil || v != "staging" {
		t.Fatalf("FromEnv = %q, %v", v, err)
	}
}
