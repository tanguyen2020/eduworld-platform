package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/analyticssql"
)

func backendRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("no go.mod")
	return ""
}

// goSources concatenates the non-test Go sources of the backend (metric registrations).
func goSources(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(backendRoot(t), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "testdata") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			b.Write(src)
			b.WriteByte('\n')
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

var metricRe = regexp.MustCompile(`eduworld_[a-z0-9_]+`)

// F080-AC-18 (T036): every KPI of FR-OPS-003 has a query of the Live dashboard catalog, every eduworld_* metric the
// catalog reads is registered by a backend binary, and the labels of the registered metrics never carry a user id,
// a student id, a nickname or content (R14).
func TestLiveOpsMetricsCatalog(t *testing.T) {
	src := goSources(t)
	byCode := map[string]query.KPIDef{}
	for _, d := range query.KPICatalog {
		byCode[d.Code] = d
		for _, m := range metricRe.FindAllString(d.Query, -1) {
			m = strings.TrimSuffix(m, "_bucket")
			if !strings.Contains(src, `"`+m+`"`) {
				t.Errorf("KPI %s reads %s, registered by no binary", d.Code, m)
			}
		}
		if strings.Contains(d.Query, "go_sql_") && !strings.Contains(src, "NewDBStatsCollector") {
			t.Errorf("KPI %s needs the DB stats collector", d.Code)
		}
	}
	// FR-OPS-003: online players, active sessions, matchmaking queue, match duration, API latency, error rate, chat rate,
	// learning completion, exam submission, DB performance.
	for _, code := range []string{"ccu", "game_sessions_active", "matchmaking_queue", "match_duration_p50_seconds", "api_latency_p95_seconds",
		"http_5xx_rate", "chat_messages_per_minute", "attempts_submitted_per_minute", "exam_attempts_per_minute", "db_wait_seconds_per_minute"} {
		if _, ok := byCode[code]; !ok {
			t.Errorf("FR-OPS-003 KPI %s missing from the catalog", code)
		}
	}
	labels := regexp.MustCompile(`(?s)Name:\s*"eduworld_[a-z0-9_]+".{0,400}?\}, \[\]string\{([^}]*)\}`)
	forbidden := regexp.MustCompile(`"(user_id|student_id|student|player_key|nickname|name|email|message|content)"`)
	for _, m := range labels.FindAllStringSubmatch(src, -1) {
		if forbidden.MatchString(m[1]) {
			t.Errorf("metric labels with an identity or content: %s", m[1])
		}
	}
}

// F080-AC-11, AC-31 (T026, T056; TC-DASH-001): the dashboard query catalog reads rollup tables and projections only —
// never analytics.events_raw nor analytics.player_days (ADR-0062, DASHBOARD_DATA §7).
func TestAnalyticsDashboardCatalogTables(t *testing.T) {
	for id, tables := range analyticssql.Catalog {
		for _, tb := range tables {
			if strings.Contains(tb, "events_raw") || strings.Contains(tb, "player_days") {
				t.Errorf("query %s reads %s", id, tb)
			}
		}
	}
	src, err := os.ReadFile(filepath.Join(backendRoot(t), "modules", "game", "internal", "infrastructure", "persistence", "analyticssql", "dashboard.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "analytics.events_raw") || strings.Contains(string(src), "analytics.player_days") {
		t.Fatal("the dashboard catalog references a raw table")
	}
}

// F080-AC-06 (T019; AC-ANL-103): the migrations of schema analytics declare no identifying column.
func TestAnalyticsSchemaNoIdentity(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(backendRoot(t), "modules", "game", "migrations", "20270102000080_f080_analytics.sql"))
	if err != nil {
		t.Fatal(err)
	}
	col := regexp.MustCompile(`(?m)^\s+(student_id|user_id|name|nickname|full_name|display_name|email|phone|ip|ip_address|message|chat_content)\s`)
	if m := col.FindString(string(b)); m != "" {
		t.Fatalf("identifying column in schema analytics: %q", m)
	}
}
