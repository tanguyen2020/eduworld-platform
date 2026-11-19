package featuregate_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/featuregate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// catalogPath is the single menu/action catalog of ADR-0090 (repo root/specs/portal).
const catalogPath = "../../../../../specs/portal/menu-catalog.yaml"

const (
	ttl          = 60 * time.Second // test value standing in for feature_toggle_cache_ttl
	principalTTL = 60 * time.Second // test value standing in for principal_cache_ttl
)

// fakeSource is a TEST FAKE (R17) of featuregate.Source: an in-memory table of disabled menus/actions per
// scope ("platform" or a tenant id). The production Source is module identity's (F015).
type fakeSource struct {
	mu      sync.Mutex
	byScope map[string]featuregate.Set
	err     error
	calls   atomic.Int32
	block   chan struct{} // when set, DisabledFor waits on it (single-flight test)
}

func scopeKey(t *uuid.UUID) string {
	if t == nil {
		return "platform"
	}
	return t.String()
}

func (f *fakeSource) set(t *uuid.UUID, menus, actions []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.byScope == nil {
		f.byScope = map[string]featuregate.Set{}
	}
	f.byScope[scopeKey(t)] = featuregate.NewSet(menus, actions)
}

func (f *fakeSource) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeSource) DisabledFor(_ context.Context, t *uuid.UUID) (featuregate.Set, error) {
	f.calls.Add(1)
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return featuregate.Set{}, f.err
	}
	return f.byScope[scopeKey(t)], nil
}

func loadCatalog(t *testing.T) *featuregate.Catalog {
	t.Helper()
	c, err := featuregate.LoadCatalog(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newGate(t *testing.T, src featuregate.Source, clk clock.Clock) *featuregate.Gate {
	t.Helper()
	g, err := featuregate.New(featuregate.Options{Source: src, Catalog: loadCatalog(t), TTL: ttl, PrincipalTTL: principalTTL,
		Clock: clk, Registerer: prometheus.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func ptr(u uuid.UUID) *uuid.UUID { return &u }

func codeOf(err error) (problem.Code, string) {
	var pe *problem.Error
	if !errors.As(err, &pe) {
		return "", ""
	}
	k, _ := pe.Ext["feature_key"].(string)
	return pe.Code, k
}

func TestNewOptions(t *testing.T) {
	cat := loadCatalog(t)
	src := &fakeSource{}
	cases := []struct {
		name string
		o    featuregate.Options
		ok   bool
	}{
		{"valid", featuregate.Options{Source: src, Catalog: cat, TTL: ttl, PrincipalTTL: principalTTL}, true},
		{"ttl equal to principal ttl", featuregate.Options{Source: src, Catalog: cat, TTL: principalTTL, PrincipalTTL: principalTTL}, true},
		{"no source", featuregate.Options{Catalog: cat, TTL: ttl, PrincipalTTL: principalTTL}, false},
		{"no catalog", featuregate.Options{Source: src, TTL: ttl, PrincipalTTL: principalTTL}, false},
		{"no ttl", featuregate.Options{Source: src, Catalog: cat, PrincipalTTL: principalTTL}, false},
		{"no principal ttl", featuregate.Options{Source: src, Catalog: cat, TTL: ttl}, false},
		// F012-AC-41: feature_toggle_cache_ttl must not exceed principal_cache_ttl.
		{"ttl above principal ttl", featuregate.Options{Source: src, Catalog: cat, TTL: principalTTL + time.Second, PrincipalTTL: principalTTL}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := featuregate.New(c.o)
			if (err == nil) != c.ok {
				t.Fatalf("err = %v, want ok=%v", err, c.ok)
			}
		})
	}
}

func TestCatalog(t *testing.T) {
	cat := loadCatalog(t)
	cases := []struct {
		action     string
		menu       string
		canDisable bool
	}{
		{"action.school_lms.assignments.view", "menu.school_lms.assignments", true},
		{"action.school_lms.assignments.release_results", "menu.school_lms.assignments", true},
		{"action.school_lms.access_control.view_users", "menu.school_lms.access_control", false},
		{"action.platform_admin.feature_toggles.view", "menu.platform_admin.feature_toggles", false},
	}
	for _, c := range cases {
		a, ok := cat.Action(c.action)
		if !ok || a.Menu != c.menu || a.CanDisable != c.canDisable {
			t.Fatalf("%s = %+v (%v), want menu %s can_disable %v", c.action, a, ok, c.menu, c.canDisable)
		}
	}
	if _, ok := cat.Action("action.nope.nope.nope"); ok {
		t.Fatal("unknown action found")
	}
	bad := []string{
		"workspaces: [",
		"workspaces: []",
		"workspaces: [{key: w, menus: [{key: menu.w.m, can_disable: true, actions: [{key: action.w.m.a, can_disable: true}, {key: action.w.m.a, can_disable: true}]}]}]",
		"workspaces: [{key: w, menus: [{key: menu.w.m, can_disable: false, actions: [{key: action.w.m.a, can_disable: true}]}]}]",
		"workspaces: [{key: w, menus: [{key: menu.w.m, actions: [{key: action.w.m.a}]}]}]",
	}
	for _, b := range bad {
		if _, err := featuregate.ParseCatalog([]byte(b)); err == nil {
			t.Fatalf("ParseCatalog(%q) accepted", b)
		}
	}
}

// F012-AC-40: action or parent menu disabled for the session scope → 403 FEATURE_DISABLED with feature_key;
// other scopes and can_disable = false actions are untouched.
func TestCheck(t *testing.T) {
	ta, tb := uuid.New(), uuid.New()
	src := &fakeSource{}
	src.set(&ta, nil, []string{"action.school_lms.assignments.edit"})
	src.set(&tb, []string{"menu.school_lms.assignments"}, nil)
	src.set(nil, []string{"menu.content_studio.lessons"}, nil)
	// A Source must never return protected items (F015 refuses them with 422); the gate ignores them anyway.
	src.set(ptr(uuid.MustParse("00000000-0000-7000-8000-000000000001")), []string{"menu.school_lms.access_control"},
		[]string{"action.school_lms.access_control.view_users"})
	g := newGate(t, src, clock.NewFake(time.Now()))
	prot := ptr(uuid.MustParse("00000000-0000-7000-8000-000000000001"))
	cases := []struct {
		name   string
		tenant *uuid.UUID
		action string
		code   problem.Code
		key    string
	}{
		{"action disabled", &ta, "action.school_lms.assignments.edit", problem.FeatureDisabled, "action.school_lms.assignments.edit"},
		{"sibling action enabled", &ta, "action.school_lms.assignments.view", "", ""},
		{"parent menu disabled → view", &tb, "action.school_lms.assignments.view", problem.FeatureDisabled, "menu.school_lms.assignments"},
		{"parent menu disabled → release", &tb, "action.school_lms.assignments.release_results", problem.FeatureDisabled, "menu.school_lms.assignments"},
		{"other menu of the tenant", &tb, "action.school_lms.schools.view", "", ""},
		{"platform default (no tenant)", nil, "action.content_studio.lessons.view", problem.FeatureDisabled, "menu.content_studio.lessons"},
		{"platform default does not leak into tenant scope", &ta, "action.content_studio.lessons.view", "", ""},
		{"protected action listed by the source", prot, "action.school_lms.access_control.view_users", "", ""},
		{"unknown tenant", ptr(uuid.New()), "action.school_lms.assignments.edit", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := testutil.ToFloat64(featuregate.BlockedCounter().WithLabelValues(c.key))
			err := g.Check(context.Background(), c.tenant, c.action)
			code, key := codeOf(err)
			if c.code == "" {
				if err != nil {
					t.Fatalf("err = %v, want allowed", err)
				}
				return
			}
			if code != c.code || key != c.key {
				t.Fatalf("code %q key %q (err %v), want %q %q", code, key, err, c.code, c.key)
			}
			if problem.Status(code) != 403 {
				t.Fatalf("status %d", problem.Status(code))
			}
			if after := testutil.ToFloat64(featuregate.BlockedCounter().WithLabelValues(c.key)); after != before+1 {
				t.Fatalf("eduworld_featuregate_blocked_total{feature_key=%q} = %v, want %v", c.key, after, before+1)
			}
		})
	}
	if err := g.Check(context.Background(), &ta, "action.nope.nope.nope"); err == nil {
		t.Fatal("unknown action allowed")
	}
}

// F012-AC-41: Source error with an empty cache → 503 UNAVAILABLE (retryable, never fail-open); protected
// actions never call the Source.
func TestSourceErrorFailsClosed(t *testing.T) {
	ta := uuid.New()
	src := &fakeSource{}
	src.fail(errors.New("identity database down"))
	g := newGate(t, src, clock.NewFake(time.Now()))
	before := testutil.ToFloat64(featuregate.SourceErrorsCounter())
	err := g.Check(context.Background(), &ta, "action.school_lms.assignments.view")
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.Unavailable || !pe.Retryable {
		t.Fatalf("err = %v, want retryable UNAVAILABLE", err)
	}
	if strings.Contains(problem.From(err, "").Detail, "database") {
		t.Fatal("internal cause leaked into detail")
	}
	if after := testutil.ToFloat64(featuregate.SourceErrorsCounter()); after != before+1 {
		t.Fatalf("eduworld_featuregate_source_errors_total = %v, want %v", after, before+1)
	}
	calls := src.calls.Load()
	if err := g.Check(context.Background(), &ta, "action.school_lms.access_control.view_users"); err != nil {
		t.Fatalf("protected action: %v", err)
	}
	if src.calls.Load() != calls {
		t.Fatal("protected action consulted the source")
	}
}

// F012-AC-41: state is cached per scope for feature_toggle_cache_ttl; a change applies at the latest after
// the TTL; an expired entry is never served when the Source fails.
func TestCacheTTL(t *testing.T) {
	ta := uuid.New()
	clk := clock.NewFake(time.Now())
	src := &fakeSource{}
	g := newGate(t, src, clk)
	ctx := context.Background()
	const act = "action.school_lms.assignments.view"
	if err := g.Check(ctx, &ta, act); err != nil {
		t.Fatal(err)
	}
	src.set(&ta, []string{"menu.school_lms.assignments"}, nil)
	clk.Advance(ttl - time.Second)
	if err := g.Check(ctx, &ta, act); err != nil {
		t.Fatalf("within TTL: %v (cache not used)", err)
	}
	if n := src.calls.Load(); n != 1 {
		t.Fatalf("source calls = %d, want 1", n)
	}
	// Valid cache hides a Source outage.
	src.fail(errors.New("down"))
	if err := g.Check(ctx, &ta, act); err != nil {
		t.Fatalf("cached: %v", err)
	}
	src.fail(nil)
	clk.Advance(2 * time.Second)
	if code, _ := codeOf(g.Check(ctx, &ta, act)); code != problem.FeatureDisabled {
		t.Fatalf("after TTL: %q, want FEATURE_DISABLED", code)
	}
	if n := src.calls.Load(); n != 2 {
		t.Fatalf("source calls = %d, want 2", n)
	}
	clk.Advance(ttl + time.Second)
	src.fail(errors.New("down"))
	if code, _ := codeOf(g.Check(ctx, &ta, act)); code != problem.Unavailable {
		t.Fatalf("expired + source down: %q, want UNAVAILABLE (no stale read)", code)
	}
}

// F012-AC-41: platform_settings.changed scope feature_flag → Invalidate (tenant override → that tenant;
// platform default → every scope).
func TestInvalidate(t *testing.T) {
	ta, tb := uuid.New(), uuid.New()
	clk := clock.NewFake(time.Now())
	src := &fakeSource{}
	g := newGate(t, src, clk)
	ctx := context.Background()
	const act = "action.school_lms.assignments.view"
	for _, tn := range []*uuid.UUID{&ta, &tb, nil} {
		if err := g.Check(ctx, tn, act); err != nil {
			t.Fatal(err)
		}
	}
	src.set(&ta, nil, []string{act})
	src.set(&tb, nil, []string{act})
	src.set(nil, nil, []string{act})
	g.Invalidate(&ta)
	if code, _ := codeOf(g.Check(ctx, &ta, act)); code != problem.FeatureDisabled {
		t.Fatalf("tenant A after invalidation: %q", code)
	}
	if err := g.Check(ctx, &tb, act); err != nil {
		t.Fatalf("tenant B must still be cached: %v", err)
	}
	if err := g.Check(ctx, nil, act); err != nil {
		t.Fatalf("platform must still be cached: %v", err)
	}
	g.Invalidate(nil)
	for _, tn := range []*uuid.UUID{&tb, nil} {
		if code, _ := codeOf(g.Check(ctx, tn, act)); code != problem.FeatureDisabled {
			t.Fatalf("scope %v after platform invalidation: %q", tn, code)
		}
	}
}

// Concurrent misses of one scope share one Source call.
func TestSingleFlight(t *testing.T) {
	ta := uuid.New()
	src := &fakeSource{block: make(chan struct{})}
	g := newGate(t, src, clock.NewFake(time.Now()))
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { errs <- g.Check(context.Background(), &ta, "action.school_lms.assignments.view") })
	}
	for src.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(src.block)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := src.calls.Load(); n != 1 {
		t.Fatalf("source calls = %d, want 1", n)
	}
}

func TestRuleFromOperation(t *testing.T) {
	g := newGate(t, &fakeSource{}, clock.NewFake(time.Now()))
	mk := func(ext map[string]any) *openapi3.Operation {
		return &openapi3.Operation{OperationID: "op", Extensions: ext}
	}
	cases := []struct {
		name    string
		ext     map[string]any
		gated   bool
		wantErr bool
	}{
		{"no extension", nil, false, false},
		{"shell", map[string]any{"x-portal-shell": true}, false, false},
		{"single action", map[string]any{"x-portal-action": "action.school_lms.assignments.view"}, true, false},
		{"param map", map[string]any{"x-portal-action": map[string]any{"param": "content_type",
			"map": map[string]any{"lesson": "action.content_studio.lessons.view", "question": "action.content_studio.question_bank.view"}}}, true, false},
		{"protected action only", map[string]any{"x-portal-action": "action.school_lms.access_control.view_users"}, false, false},
		{"shell and action", map[string]any{"x-portal-shell": true, "x-portal-action": "action.school_lms.assignments.view"}, false, true},
		{"unknown action", map[string]any{"x-portal-action": "action.school_lms.nope.view"}, false, true},
		{"unknown action in map", map[string]any{"x-portal-action": map[string]any{"param": "p", "map": map[string]any{"a": "action.x.y.z"}}}, false, true},
		{"map without param", map[string]any{"x-portal-action": map[string]any{"map": map[string]any{"a": "action.school_lms.assignments.view"}}}, false, true},
		{"empty map", map[string]any{"x-portal-action": map[string]any{"param": "p", "map": map[string]any{}}}, false, true},
		{"wrong type", map[string]any{"x-portal-action": 3}, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, gated, err := g.Bind(mk(c.ext))
			if (err != nil) != c.wantErr || gated != c.gated {
				t.Fatalf("gated %v err %v, want gated %v err %v", gated, err, c.gated, c.wantErr)
			}
		})
	}
}

func TestSetAccessors(t *testing.T) {
	s := featuregate.NewSet([]string{"menu.b", "menu.a"}, []string{"action.x"})
	if !s.MenuDisabled("menu.a") || s.MenuDisabled("menu.c") || !s.ActionDisabled("action.x") || s.ActionDisabled("menu.a") {
		t.Fatal("membership")
	}
	if m := s.Menus(); len(m) != 2 || m[0] != "menu.a" {
		t.Fatalf("menus %v", m)
	}
	if a := (featuregate.Set{}).Actions(); len(a) != 0 {
		t.Fatalf("zero set %v", a)
	}
}
