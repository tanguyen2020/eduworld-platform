package module_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func newRegistry(t *testing.T, binary, profile string, env map[string]string) *module.Registry {
	t.Helper()
	r := module.NewRegistry(binary, profile, health.NewRegistry(buildinfo.Get(binary, profile)), func(k string) string { return env[k] })
	t.Cleanup(r.Close)
	return r
}

// Each role pool comes from its own variable; the read pool falls back to the primary; a missing role is reported
// by variable name.
func TestSQLRoles(t *testing.T) {
	r := newRegistry(t, "worker", "events", map[string]string{"EW_DB_GAME_DSN": "postgres://a@127.0.0.1:1/eduworld_game",
		"EW_DB_GAME_PLATFORM_DSN": "postgres://b@127.0.0.1:1/eduworld_game"})
	app, err := r.SQL("game", module.RoleApp)
	if err != nil {
		t.Fatal(err)
	}
	if rd, err := r.SQL("game", module.RoleRead); err != nil || rd != app {
		t.Fatalf("read pool = %v %v, want the app pool", rd, err)
	}
	if again, _ := r.SQL("game", module.RoleApp); again != app {
		t.Fatal("pool not reused")
	}
	_, err = r.SQL("game", module.RoleRelay)
	if !errors.Is(err, module.ErrDatabaseNotConfigured) || !strings.Contains(err.Error(), "EW_DB_GAME_RELAY_DSN") {
		t.Fatalf("missing relay role: %v", err)
	}
	if _, err := r.Core("game"); err != nil {
		t.Fatalf("core: %v", err)
	}
}

// Entry points run only where their role is on: HTTP in core-api, relays/consumers in events, jobs in jobs.
func TestDeclarationsFollowTheRole(t *testing.T) {
	cases := []struct {
		binary, profile  string
		http, relay, job bool
	}{
		{"core-api", "", true, false, false},
		{"worker", "events", false, true, false},
		{"worker", "jobs", false, false, true},
		{"worker", "all", false, true, true},
		{"worker", "chat", false, false, false},
	}
	for _, c := range cases {
		r := newRegistry(t, c.binary, c.profile, map[string]string{"EW_DB_GAME_RELAY_DSN": "postgres://c@127.0.0.1:1/eduworld_game"})
		r.HTTP("game", func(*httpx.Server) error { return nil })
		if err := r.Relay("game"); err != nil {
			t.Fatal(err)
		}
		r.Job("game", "tick", time.Minute, func(context.Context) (int, error) { return 0, nil })
		got := map[string]bool{}
		for _, d := range r.Declarations() {
			got[d.Kind] = d.Active
		}
		if got[module.KindHTTP] != c.http || got[module.KindRelay] != c.relay || got[module.KindJob] != c.job {
			t.Errorf("%s/%s: %+v", c.binary, c.profile, got)
		}
		active := 0
		for _, on := range got {
			if on {
				active++
			}
		}
		if n := len(r.Workers()); n != active-btoi(c.http) {
			t.Errorf("%s/%s: %d workers for %d active background declarations", c.binary, c.profile, n, active-btoi(c.http))
		}
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// The audit hook writes into the outbox of the module of ctx and refuses an unknown module (never another database).
func TestAuditRecorderNeedsTheModuleOfContext(t *testing.T) {
	r := newRegistry(t, "core-api", "", nil)
	err := r.AuditRecorder().Record(tenancy.WithModule(context.Background(), "learning"), audit.Event{Action: "x"})
	if err == nil || !strings.Contains(err.Error(), "learning") {
		t.Fatalf("unknown module accepted: %v", err)
	}
}

// The cursor key is required and at least 32 bytes.
func TestCursorsKey(t *testing.T) {
	if _, err := newRegistry(t, "core-api", "", nil).Cursors(); err == nil {
		t.Fatal("missing EW_PAGE_CURSOR_KEY accepted")
	}
	if _, err := newRegistry(t, "core-api", "", map[string]string{"EW_PAGE_CURSOR_KEY": "short"}).Cursors(); err == nil {
		t.Fatal("short key accepted")
	}
	if _, err := newRegistry(t, "core-api", "", map[string]string{"EW_PAGE_CURSOR_KEY": strings.Repeat("k", 32)}).Cursors(); err != nil {
		t.Fatal(err)
	}
}
