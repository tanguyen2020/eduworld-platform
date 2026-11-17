package main

import (
	"strings"
	"testing"
)

const mp = "github.com/x/eduworld"

var allowed = map[string]map[string]bool{
	"chat":     {"identity": true},
	"identity": {"learning": true, "game": true, "competition": true, "chat": true},
	"learning": {"game": true, "competition": true, "identity": true},
}

func v(pkg string, imports ...string) []string {
	return Violations(mp, []Package{{ImportPath: mp + "/" + pkg, Imports: imports}}, allowed)
}

func TestModuleBoundaries(t *testing.T) {
	cases := []struct {
		name, pkg, imp string
		wantFail       bool
	}{
		{"allowed caller imports api", "modules/identity/internal/infrastructure", mp + "/modules/chat/api", false},
		{"caller not in x-allowed-callers", "modules/learning/internal/infrastructure", mp + "/modules/chat/api", true},
		{"game → learning api allowed", "modules/game/internal/infrastructure", mp + "/modules/learning/api", false},
		{"chat → learning api not allowed", "modules/chat/internal/infrastructure", mp + "/modules/learning/api", true},
		{"other module root package", "modules/game/internal/infrastructure", mp + "/modules/learning", true},
		{"pkg imports modules", "pkg/health", mp + "/modules/identity", true},
		{"cmd imports module root", "cmd/core-api", mp + "/modules/identity", false},
		{"cmd imports api", "cmd/core-api", mp + "/modules/identity/api", true},
		{"domain imports infrastructure", "modules/game/internal/domain", mp + "/modules/game/internal/infrastructure", true},
		{"application imports presentation", "modules/game/internal/application/command", mp + "/modules/game/internal/presentation", true},
		{"query imports port/write", "modules/game/internal/application/query", mp + "/modules/game/internal/application/port/write", true},
		{"command imports domain", "modules/game/internal/application/command", mp + "/modules/game/internal/domain", false},
		{"stdlib ignored", "modules/game/internal/domain", "strings", false},
	}
	for _, c := range cases {
		got := v(c.pkg, c.imp)
		if (len(got) > 0) != c.wantFail {
			t.Errorf("%s: violations=%v, wantFail=%v", c.name, got, c.wantFail)
		}
	}
}

func TestRenderDepguardDeniesOnlyDisallowed(t *testing.T) {
	out := string(renderDepguard(mp, []string{"chat", "identity", "learning"}, allowed))
	if !strings.Contains(out, `module-learning:`) || !strings.Contains(out, mp+`/modules/chat/api`) {
		t.Fatalf("learning must be denied chat/api:\n%s", out)
	}
	if strings.Contains(out, "module-identity:") {
		t.Fatalf("identity may call every module here, no rule expected:\n%s", out)
	}
}
