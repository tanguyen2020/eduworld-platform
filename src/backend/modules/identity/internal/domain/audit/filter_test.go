package audit

import (
	"errors"
	"strings"
	"testing"
)

// F059-3: the action_prefix filter accepts dotted lower-case prefixes only, bounded in count and length.
func TestCheckActionPrefixes(t *testing.T) {
	ok := [][]string{nil, {"role", "user_role"}, {"cross_tenant.accessed"}, {"ab"}, make([]string, 0, 10)}
	for _, in := range ok {
		if err := CheckActionPrefixes(in); err != nil {
			t.Errorf("%v: %v", in, err)
		}
	}
	ten := make([]string, MaxActionPrefixes+1)
	for i := range ten {
		ten[i] = "role"
	}
	bad := map[string][]string{
		"too many":     ten,
		"too short":    {"r"},
		"too long":     {strings.Repeat("a", MaxActionPrefixLen+1)},
		"wildcard":     {"role.*"},
		"percent":      {"role%"},
		"upper case":   {"Role"},
		"trailing dot": {"role."},
		"leading dot":  {".role"},
		"space":        {"role created"},
		"quote":        {"role'"},
		"digit first":  {"1role"},
		"empty":        {""},
	}
	for name, in := range bad {
		if err := CheckActionPrefixes(in); !errors.Is(err, ErrActionPrefix) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestMatchesPrefix(t *testing.T) {
	cases := []struct {
		action, prefix string
		want           bool
	}{
		{"role.created", "role", true},
		{"role", "role", true},
		{"user_role.granted", "user_role", true},
		{"user_role.granted", "role", false},
		{"role_x.created", "role", false},
		{"cross_tenant.accessed", "cross_tenant.accessed", true},
		{"cross_tenant.accessed", "cross_tenant.access", false},
	}
	for _, c := range cases {
		if got := MatchesPrefix(c.action, c.prefix); got != c.want {
			t.Errorf("%s ~ %s = %v", c.action, c.prefix, got)
		}
	}
}
