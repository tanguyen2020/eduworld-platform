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
		// F002-AC-26: domain never imports infrastructure libraries or the module ent package.
		{"domain imports net/http", "modules/game/internal/domain", "net/http", true},
		{"domain imports httptest", "modules/game/internal/domain/quest", "net/http/httptest", true},
		{"domain imports ent", "modules/game/internal/domain", "entgo.io/ent", true},
		{"domain imports module ent", "modules/game/internal/domain", mp + "/modules/game/ent", true},
		{"domain imports pgx", "modules/game/internal/domain", "github.com/jackc/pgx/v5", true},
		{"domain imports nats", "modules/game/internal/domain", "github.com/nats-io/nats.go", true},
		{"domain imports valkey", "modules/game/internal/domain", "github.com/valkey-io/valkey-go", true},
		{"domain imports database/sql", "modules/game/internal/domain", "database/sql", true},
		{"domain imports uuid", "modules/game/internal/domain", "github.com/google/uuid", false},
		{"infrastructure imports pgx", "modules/game/internal/infrastructure", "github.com/jackc/pgx/v5", false},
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

// F002-AC-27: pkg/authz, pkg/problem and pkg/tenancy never depend on ent or pgx, even transitively.
func TestCorePkgsWithoutDataLayer(t *testing.T) {
	for _, c := range []struct {
		pkg      string
		deps     []string
		wantFail bool
	}{
		{"pkg/tenancy", []string{"github.com/jackc/pgx/v5/pgconn"}, true},
		{"pkg/authz", []string{"entgo.io/ent/dialect/sql"}, true},
		{"pkg/problem", []string{"encoding/json"}, false},
		{"pkg/authz/authztest", []string{"github.com/jackc/pgx/v5"}, true},
		{"pkg/testkit", []string{"github.com/jackc/pgx/v5"}, false},
	} {
		got := Violations(mp, []Package{{ImportPath: mp + "/" + c.pkg, Deps: c.deps}}, allowed)
		if (len(got) > 0) != c.wantFail {
			t.Errorf("%s: violations=%v, wantFail=%v", c.pkg, got, c.wantFail)
		}
	}
}
