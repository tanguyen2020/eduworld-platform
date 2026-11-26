//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/analyticssql"
	promsource "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/prometheus"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// anaKey is the analytics key of the tests (TEST DOUBLE of the secret store, R17).
var anaKey = analytics.Key{Version: 1, Secret: []byte(strings.Repeat("k", 32))}

// memExportFiles is an in-memory TEST DOUBLE of port.ExportFiles (R17): the object storage adapter has its own tests.
type memExportFiles struct {
	mu    sync.Mutex
	files map[string][]byte
}

func (m *memExportFiles) Put(_ context.Context, key string, body io.Reader, _ int64) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key] = b
	return nil
}

func (m *memExportFiles) SignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("https://exports.test/%s?expires=%d", key, int(ttl.Seconds())), nil
}

// memVersions is a TEST DOUBLE of port.DashboardCache (R17): no cached answer, counts the version bumps.
type memVersions struct {
	mu    sync.Mutex
	bumps map[uuid.UUID]int
}

func (m *memVersions) Version(context.Context, uuid.UUID) (int64, error)        { return 0, nil }
func (m *memVersions) Get(context.Context, string) ([]byte, bool, error)        { return nil, false, nil }
func (m *memVersions) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (m *memVersions) bumped(t uuid.UUID) int                                   { m.mu.Lock(); defer m.mu.Unlock(); return m.bumps[t] }
func (m *memVersions) Bump(_ context.Context, t uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bumps[t]++
	return nil
}

// busyLimiter is a TEST DOUBLE of port.DashboardLimiter (R17) that rejects every query (429 DASHBOARD_BUSY).
type busyLimiter struct{}

func (busyLimiter) Acquire(context.Context, uuid.UUID, uuid.UUID, int, int) (func(), bool) {
	return func() {}, false
}

// slowModel wraps the query catalog with a statement that outlives the statement timeout (F080-AC-12: 504).
type slowModel struct{ analyticssql.Dashboard }

func (s slowModel) NonQATenants(ctx context.Context, tenants []uuid.UUID) ([]uuid.UUID, error) {
	t, _ := entx.Tx(ctx)
	if _, err := t.ExecContext(ctx, "SELECT pg_sleep(1)"); err != nil {
		return nil, analyticssql.MapError("tenants", err)
	}
	return s.Dashboard.NonQATenants(ctx, tenants)
}

// promAnswers is a fake Prometheus HTTP API (TEST DOUBLE, R17): canned instant vectors per query substring.
type promAnswers struct {
	mu   sync.Mutex
	down bool
	vals map[string][]map[string]any
}

func (p *promAnswers) handler(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.down {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	q := r.URL.Query().Get("query")
	res := []map[string]any{}
	for k, v := range p.vals {
		if strings.Contains(q, k) {
			res = v
			break
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "vector", "result": res}})
}

func promSample(labels map[string]string, v string) map[string]any {
	if labels == nil {
		labels = map[string]string{}
	}
	return map[string]any{"metric": labels, "value": []any{1700000000.0, v}}
}

// anaKit is the Game Analytics wiring of the tests (same use cases and adapters as the composition root).
type anaKit struct {
	cfg      analytics.Config
	ingest   command.AnalyticsIngest
	queries  query.GameAnalytics
	exports  command.AnalyticsExports
	rollup   command.AnalyticsRollup
	overview query.LiveOpsOverview
	files    *memExportFiles
	versions *memVersions
	prom     *promAnswers
	logs     *bytes.Buffer
	flag     query.ProductTelemetryFlag
}

// setupAnalytics wires the kit on e: dashboard session as game_dashboard (tenant reads) and game_platform (platform reads).
func (e *env) setupAnalytics(t *testing.T, edit func(c *analytics.Config)) *anaKit {
	t.Helper()
	cfg := analytics.DefaultConfig()
	cfg.TelemetryRatePerMin = 30
	if edit != nil {
		edit(&cfg)
	}
	keys, err := analytics.NewKeyRing(anaKey)
	if err != nil {
		t.Fatal(err)
	}
	dash, err := entx.NewReadSession("game", entx.DBs{App: open(t, e.cl.ModuleDSN("game", "dashboard")), Platform: e.platform})
	if err != nil {
		t.Fatal(err)
	}
	codec, err := page.NewCodec([]byte(strings.Repeat("c", 32)))
	if err != nil {
		t.Fatal(err)
	}
	k := &anaKit{cfg: cfg, files: &memExportFiles{files: map[string][]byte{}}, versions: &memVersions{bumps: map[uuid.UUID]int{}},
		prom: &promAnswers{vals: map[string][]map[string]any{}}, logs: &bytes.Buffer{}}
	k.ingest = command.AnalyticsIngest{UoW: e.uow, Store: analyticssql.Store{}, Keys: keys, Rate: &memRate{n: map[string]int{}}, Config: cfg,
		Clock: e.clock.Now, Log: slog.New(slog.NewJSONHandler(k.logs, nil)), Metrics: observability.AnalyticsMetrics{}}
	k.queries = query.GameAnalytics{Session: dash, Model: analyticssql.Dashboard{}, Limiter: cache.NewDashLimiter(nil), Config: cfg, Cursors: codec,
		Clock: e.clock.Now}
	k.exports = command.AnalyticsExports{UoW: e.uow, Store: analyticssql.Exports{}, Files: k.files, Renderer: k.queries, Audit: e.rec, Config: cfg,
		Clock: e.clock.Now}
	k.rollup = command.AnalyticsRollup{UoW: e.uow, Store: analyticssql.Store{}, Versions: k.versions, Config: cfg, Clock: e.clock.Now}
	srv := httptest.NewServer(http.HandlerFunc(k.prom.handler))
	t.Cleanup(srv.Close)
	k.overview = query.LiveOpsOverview{Source: promsource.Source{BaseURL: srv.URL}, Thresholds: query.DefaultKPIThresholds(), Clock: e.clock.Now}
	k.flag = query.ProductTelemetryFlag{Read: e.read, Flags: analyticssql.Store{}, Platform: true}
	e.ana = k
	return k
}

func (e *env) kit() (*anaKit, error) {
	if e.ana == nil {
		return nil, problem.New(problem.Unavailable, "analytics not wired in this test")
	}
	return e.ana, nil
}

type anaIngestProxy struct{ e *env }

func (p anaIngestProxy) PostTelemetry(ctx context.Context, b command.TelemetryBatch) (int, error) {
	k, err := p.e.kit()
	if err != nil {
		return 0, err
	}
	return k.ingest.PostTelemetry(ctx, b)
}

type anaQueryProxy struct{ e *env }

func (p anaQueryProxy) GetMetrics(ctx context.Context, c []string, f query.AnalyticsFilter) (query.MetricsView, error) {
	k, err := p.e.kit()
	if err != nil {
		return query.MetricsView{}, err
	}
	return k.queries.GetMetrics(ctx, c, f)
}
func (p anaQueryProxy) GetRetention(ctx context.Context, f query.AnalyticsFilter) (query.RetentionView, error) {
	k, err := p.e.kit()
	if err != nil {
		return query.RetentionView{}, err
	}
	return k.queries.GetRetention(ctx, f)
}
func (p anaQueryProxy) GetFunnel(ctx context.Context, c string, f query.AnalyticsFilter) (query.FunnelView, error) {
	k, err := p.e.kit()
	if err != nil {
		return query.FunnelView{}, err
	}
	return k.queries.GetFunnel(ctx, c, f)
}
func (p anaQueryProxy) GetContentStats(ctx context.Context, kind, s string, l int, c string, f query.AnalyticsFilter) (query.ContentPage, error) {
	k, err := p.e.kit()
	if err != nil {
		return query.ContentPage{}, err
	}
	return k.queries.GetContentStats(ctx, kind, s, l, c, f)
}
func (p anaQueryProxy) GetPerformance(ctx context.Context, f query.AnalyticsFilter) (query.PerformanceView, error) {
	k, err := p.e.kit()
	if err != nil {
		return query.PerformanceView{}, err
	}
	return k.queries.GetPerformance(ctx, f)
}
func (p anaQueryProxy) GetTenantSummary(ctx context.Context, in query.TenantSummaryInput) (query.TenantSummaryView, error) {
	k, err := p.e.kit()
	if err != nil {
		return query.TenantSummaryView{}, err
	}
	return k.queries.GetTenantSummary(ctx, in)
}

type anaExportProxy struct{ e *env }

func (p anaExportProxy) Create(ctx context.Context, r string, params json.RawMessage) (command.ExportView, error) {
	k, err := p.e.kit()
	if err != nil {
		return command.ExportView{}, err
	}
	return k.exports.Create(ctx, r, params)
}
func (p anaExportProxy) Get(ctx context.Context, id uuid.UUID) (command.ExportView, error) {
	k, err := p.e.kit()
	if err != nil {
		return command.ExportView{}, err
	}
	return k.exports.Get(ctx, id)
}

type anaOverviewProxy struct{ e *env }

func (p anaOverviewProxy) Get(ctx context.Context, ids []uuid.UUID) (query.OverviewView, error) {
	k, err := p.e.kit()
	if err != nil {
		return query.OverviewView{}, err
	}
	return k.overview.Get(ctx, ids)
}

type anaFlagProxy struct{ e *env }

func (p anaFlagProxy) ProductTelemetry(ctx context.Context, tenant *uuid.UUID) bool {
	if p.e.ana == nil {
		return true
	}
	return p.e.ana.flag.ProductTelemetry(ctx, tenant)
}

// --- raw events of the golden dataset ---

// ictAt is a wall time of an analytics day (ICT).
func ictAt(day string, hhmm string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", day+" "+hhmm, analytics.Zone)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

// rawEvent builds one events_raw row of a student (pseudonymous key of the test key).
func rawEvent(tenant uuid.UUID, school *uuid.UUID, grade int, student uuid.UUID, session string, name string, at time.Time,
	props map[string]any) port.RawEvent {
	g := grade
	r := port.RawEvent{EventID: uuid.New(), OccurredAt: at, TenantID: tenant, SchoolID: school, GradeLevel: &g, Platform: "web", Locale: "vi-VN",
		DeviceTier: "mid", ClientVersion: "1.2.3", PlayerKey: analytics.Pseudonym(anaKey, student), KeyVersion: 1, EventName: name,
		Source: "server", Props: props}
	if session != "" {
		r.SessionKey = analytics.Pseudonym(anaKey, uuid.NewSHA1(uuid.NameSpaceOID, []byte(session)))
	}
	if props == nil {
		r.Props = map[string]any{}
	}
	return r
}

// insertRaw writes rows in the tenant scope (as the consumer would).
func (e *env) insertRaw(ctx context.Context, t *testing.T, tenant uuid.UUID, rows ...port.RawEvent) {
	t.Helper()
	if err := e.uow.InTenantTx(ctx, mustEventScope(t, tenant), func(ctx context.Context) error {
		_, err := analyticssql.Store{}.InsertRaw(ctx, rows)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func mustEventScope(t *testing.T, tenant uuid.UUID) tenancy.Scope {
	t.Helper()
	s, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// session returns the raw events of one game session (start, end with active seconds, optional late night).
func session(tenant uuid.UUID, school *uuid.UUID, grade int, st uuid.UUID, key string, startAt time.Time, seconds int, late bool) []port.RawEvent {
	return []port.RawEvent{
		rawEvent(tenant, school, grade, st, key, "session_start", startAt, nil),
		rawEvent(tenant, school, grade, st, key, "session_end", startAt.Add(time.Duration(seconds)*time.Second),
			map[string]any{"active_seconds": float64(seconds), "end_reason": "logout", "late_night": late}),
	}
}

// rollupValue reads value, numerator, denominator and is_final of a whole-tenant daily rollup row.
func (e *env) rollupRow(ctx context.Context, t *testing.T, table string, tenant uuid.UUID, code string, day string) (float64, int64, int64, bool) {
	t.Helper()
	var v *float64
	var num, den *int64
	var final bool
	if err := e.platform.QueryRowContext(ctx, `SELECT value, numerator, denominator, is_final FROM analytics.`+table+`
		WHERE tenant_id = $1 AND metric_code = $2 AND metric_date = $3 AND dim_set = ''`, tenant, code, day).Scan(&v, &num, &den, &final); err != nil {
		t.Fatalf("rollup %s %s %s: %v", table, code, day, err)
	}
	out := func(p *int64) int64 {
		if p == nil {
			return 0
		}
		return *p
	}
	vv := 0.0
	if v != nil {
		vv = *v
	}
	return vv, out(num), out(den), final
}
