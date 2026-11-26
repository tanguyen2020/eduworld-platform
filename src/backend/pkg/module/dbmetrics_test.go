package module_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// MONITORING §3 (alert ModuleDatabaseUnavailable): every pool the registry opens is listed once in
// eduworld_db_pool_connections with module, database and role; a lazy pool without a connection reads total 0. The read
// role falling back to the app pool is not listed twice; the pools are named for the acquire histogram of pkg/entx.
func TestPoolConnectionsMetric(t *testing.T) {
	hr := health.NewRegistry(buildinfo.Get("core-api", ""))
	env := map[string]string{"EW_DB_GAME_DSN": "postgres://a@127.0.0.1:1/eduworld_game",
		"EW_DB_GAME_PLATFORM_DSN": "postgres://b@127.0.0.1:1/eduworld_game", "EW_DB_CHAT_DSN": "postgres://c@127.0.0.1:1/eduworld_chat"}
	r := module.NewRegistry("core-api", "", hr, func(k string) string { return env[k] })
	t.Cleanup(r.Close)
	app, err := r.SQL("game", module.RoleApp)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{module.RolePlatform, module.RoleRead} {
		if _, err := r.SQL("game", role); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Database("chat"); err != nil {
		t.Fatal(err)
	}
	if p, ok := observability.PoolOf(app); !ok || p.Module != "game" || p.Pool != module.RoleApp {
		t.Fatalf("pool name %+v %v", p, ok)
	}
	families, err := hr.Metrics().Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, f := range families {
		if f.GetName() != "eduworld_db_pool_connections" {
			continue
		}
		for _, m := range f.GetMetric() {
			got[labels(m)] = m.GetGauge().GetValue()
		}
	}
	for _, want := range []string{
		"database=eduworld_game,module=game,pool=app,state=total",
		"database=eduworld_game,module=game,pool=platform,state=total",
		"database=eduworld_chat,module=chat,pool=health,state=total",
		"database=eduworld_game,module=game,pool=app,state=max",
	} {
		v, ok := got[want]
		if !ok {
			t.Errorf("missing series %s in %v", want, got)
			continue
		}
		if want[len(want)-5:] == "total" && v != 0 {
			t.Errorf("%s = %v, want 0 (lazy pool, database unreachable)", want, v)
		}
	}
	if len(got) != 3*4 { // game app + platform (read = app), chat health; 4 states each
		t.Errorf("series %d, want 12: %v", len(got), got)
	}
	// The acquire and query histograms are registered on the admin registry as well.
	for _, name := range []string{"eduworld_db_pool_acquire_duration_seconds", "eduworld_db_query_duration_seconds"} {
		if n, err := testutil.GatherAndCount(hr.Metrics(), name); err != nil || n < 0 {
			t.Errorf("%s not gatherable: %v", name, err)
		}
	}
	if err := hr.Metrics().Register(observability.DBPoolAcquire()); err == nil {
		t.Error("acquire histogram was not registered by NewRegistry")
	}
}

func labels(m *dto.Metric) string {
	s := ""
	for i, l := range m.GetLabel() {
		if i > 0 {
			s += ","
		}
		s += l.GetName() + "=" + l.GetValue()
	}
	return s
}
