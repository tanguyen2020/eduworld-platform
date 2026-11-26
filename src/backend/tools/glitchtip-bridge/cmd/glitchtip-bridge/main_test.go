package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func secret(t *testing.T, name, value string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad(t *testing.T) {
	hook := secret(t, "hook", "0123456789abcdefghijklmnop")
	gt := secret(t, "gt", "gt-secret-value")
	gh := secret(t, "gh", "gh-secret-value")
	base := map[string]string{
		"EW_GLITCHTIP_URL": "http://glitchtip:8000", "EW_GITHUB_REPO": "org/repo",
		"EW_GLITCHTIP_BRIDGE_TOKEN_FILE": hook, "EW_GLITCHTIP_API_TOKEN_FILE": gt, "EW_GITHUB_TOKEN_FILE": gh,
	}
	get := func(over map[string]string) func(string) string {
		return func(k string) string {
			if v, ok := over[k]; ok {
				return v
			}
			return base[k]
		}
	}

	s, err := load(get(nil))
	if err != nil {
		t.Fatal(err)
	}
	if s.hookToken != "0123456789abcdefghijklmnop" || s.githubToken != "gh-secret-value" || s.githubURL != "https://api.github.com" ||
		strings.Join(s.envs, ",") != "production" {
		t.Errorf("settings %+v", s)
	}

	cases := map[string]struct {
		over map[string]string
		msg  string
	}{
		"missing":     {map[string]string{"EW_GITHUB_REPO": "", "EW_GITHUB_TOKEN_FILE": ""}, "EW_GITHUB_REPO, EW_GITHUB_TOKEN_FILE"},
		"unreadable":  {map[string]string{"EW_GITHUB_TOKEN_FILE": filepath.Join(t.TempDir(), "none")}, "EW_GITHUB_TOKEN_FILE: cannot read"},
		"short token": {map[string]string{"EW_GLITCHTIP_BRIDGE_TOKEN_FILE": secret(t, "short", "abc")}, "at least 24"},
		"bad repo":    {map[string]string{"EW_GITHUB_REPO": "https://github.com/org/repo"}, "owner/name"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(get(c.over))
			if err == nil || !strings.Contains(err.Error(), c.msg) {
				t.Fatalf("err %v, want %q", err, c.msg)
			}
			for _, v := range []string{"gt-secret-value", "gh-secret-value", "0123456789abcdef"} {
				if strings.Contains(err.Error(), v) {
					t.Errorf("error leaks a secret: %v", err)
				}
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	var sb strings.Builder
	if got := run([]string{"nope"}, &sb, func(string) string { return "" }); got != 2 {
		t.Fatalf("exit %d", got)
	}
}
