//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/liveopsschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/liveopssql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// livePush records the live-ops realtime pushes (TEST DOUBLE of port.LiveOpsRealtime, R17; the Valkey adapter is the
// shared pkg/realtime publisher covered by realtime_valkey_test.go).
type livePush struct {
	mu   sync.Mutex
	msgs []livePushMsg
}

type livePushMsg struct {
	typ     string
	student uuid.UUID
	payload map[string]any
}

func (p *livePush) Push(_ context.Context, typ string, students []uuid.UUID, payload map[string]any, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range students {
		p.msgs = append(p.msgs, livePushMsg{typ: typ, student: s, payload: payload})
	}
	return nil
}

func (p *livePush) of(typ string) []livePushMsg {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []livePushMsg
	for _, m := range p.msgs {
		if m.typ == typ {
			out = append(out, m)
		}
	}
	return out
}

// fakeScreener is a TEST DOUBLE of port.TextScreener (R17; the chat facade adapter has its caller-side facade test): a
// text holding the placeholder flag is a violation (no real sensitive word in tests — R15); down = module chat unavailable.
type fakeScreener struct {
	mu    sync.Mutex
	down  bool
	calls int
}

// flagged is the placeholder the fake screener flags (never a real word, R15).
const flagged = "PLACEHOLDER-FLAGGED"

func (f *fakeScreener) Screen(_ context.Context, _ uuid.UUID, _ string, texts []string) ([]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.down {
		return nil, errors.Join(port.ErrScreeningUnavailable, errors.New("TEST DOUBLE: chat down"))
	}
	out := []int{}
	for i, t := range texts {
		if strings.Contains(t, flagged) {
			out = append(out, i)
		}
	}
	return out, nil
}

type liveBundle struct {
	uc     *command.LiveOps
	push   *livePush
	screen *fakeScreener
}

var liveBundles sync.Map // *env → *liveBundle

// livePolicy is the CONFIG_DEFAULTS policy of live-ops.
func livePolicy() command.LiveOpsPolicy {
	return command.LiveOpsPolicy{Rules: liveops.Policy{FestivalMinDays: 7, ExtraDailyQuestsMax: 2, PointsDailyCap: 200, BundleMaxBytes: 20 << 20,
		GoalDailyCap: 50}}.WithDefaults()
}

// live returns the live-ops use cases of the env wired as module.go does (registry of the economy, real store).
func (e *env) live() *liveBundle {
	if v, ok := liveBundles.Load(e); ok {
		return v.(*liveBundle)
	}
	b := &liveBundle{push: &livePush{}, screen: &fakeScreener{}}
	man := command.GameManifest{UoW: e.uow, Store: writerepo.ManifestRepo{}, Tenants: writerepo.ManifestRepo{}, Content: worldsql.Content{},
		Quests: questsql.Content{}, LiveOps: liveopssql.Store{}, Events: writerepo.Publisher{W: e.writer}, Metrics: observability.ManifestMetrics{},
		Now: e.clock.Now}
	b.uc = &command.LiveOps{UoW: e.uow, AfterCommit: entx.AfterCommit, Store: liveopssql.Store{}, Students: worldsql.Content{},
		Tenants: writerepo.ManifestRepo{}, Manifests: man, Grants: writerepo.ProgressionRepo{}, Registry: e.eco().eco.Registry,
		EventSchema: liveopsschema.Event{}, Events: writerepo.Publisher{W: e.writer}, Audit: writerepo.Auditor{R: e.rec}, Screener: b.screen,
		Realtime: b.push, Metrics: observability.LiveOpsMetrics{}, Policy: livePolicy(), Now: e.clock.Now}
	v, _ := liveBundles.LoadOrStore(e, b)
	return v.(*liveBundle)
}

// liveHook delegates the publish hook to the live-ops use case of the env (resolved at call time).
type liveHook struct{ e *env }

func (h liveHook) EffectiveFrom(ctx context.Context, event uuid.UUID, d liveops.Definition, at time.Time) (time.Time, error) {
	return h.e.live().uc.EffectiveFrom(ctx, event, d, at)
}

func (h liveHook) EventPublished(ctx context.Context, d liveops.Definition, version uuid.UUID, versionNo int, at time.Time) error {
	return h.e.live().uc.EventPublished(ctx, d, version, versionNo, at)
}

// liveOpsRules are the publish checks of the F077 content.
func (e *env) liveOpsRules() *command.LiveOpsPublishRules {
	return &command.LiveOpsPublishRules{EventSchema: liveopsschema.Event{}, TemplateSchema: liveopsschema.Template{}, TrackSchema: liveopsschema.Track{},
		Reader: liveopssql.Store{}, Hook: liveHook{e}, Policy: livePolicy().Rules}
}

func (e *env) liveQueries() query.LiveOps {
	codec, _ := page.NewCodec([]byte(strings.Repeat("c", 32)))
	return query.LiveOps{Read: e.read, Data: liveopssql.Store{}, Students: worldsql.Content{}, Counter: e.live().uc.Counter, Cursors: codec,
		Policy: query.LiveOpsPolicy{UpcomingVisible: 7 * 24 * time.Hour, PointsDailyCap: 200, MinCell: 5}, Now: e.clock.Now}
}

type liveQueryProxy struct{ e *env }

func (p liveQueryProxy) ListGameEvents(ctx context.Context, s []string, l int, c string) (query.Page[query.EventSummaryView], error) {
	return p.e.liveQueries().ListGameEvents(ctx, s, l, c)
}
func (p liveQueryProxy) GetGameEvent(ctx context.Context, id uuid.UUID) (query.EventDetailView, error) {
	return p.e.liveQueries().GetGameEvent(ctx, id)
}
func (p liveQueryProxy) GetCommunityGoal(ctx context.Context, id uuid.UUID) (query.GoalView, error) {
	return p.e.liveQueries().GetCommunityGoal(ctx, id)
}
func (p liveQueryProxy) GetSeasonTrack(ctx context.Context) (query.SeasonTrackView, error) {
	return p.e.liveQueries().GetSeasonTrack(ctx)
}
func (p liveQueryProxy) ListSeasonTrackHistory(ctx context.Context, l int, c string) (query.Page[query.SeasonHistoryView], error) {
	return p.e.liveQueries().ListSeasonTrackHistory(ctx, l, c)
}
func (p liveQueryProxy) GetEventRunStats(ctx context.Context, ev, run uuid.UUID) (map[string]any, error) {
	return p.e.liveQueries().GetEventRunStats(ctx, ev, run)
}
func (p liveQueryProxy) ListEventTemplates(ctx context.Context, l int, c string) (query.Page[query.TemplateView], error) {
	return p.e.liveQueries().ListEventTemplates(ctx, l, c)
}
func (p liveQueryProxy) ListSchoolEvents(ctx context.Context, st string, l int, c string) (query.Page[map[string]any], error) {
	return p.e.liveQueries().ListSchoolEvents(ctx, st, l, c)
}
func (p liveQueryProxy) GetSchoolEvent(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	return p.e.liveQueries().GetSchoolEvent(ctx, id)
}
func (p liveQueryProxy) GetSchoolEventProgress(ctx context.Context, id uuid.UUID, school *uuid.UUID) (query.SchoolEventProgressView, error) {
	return p.e.liveQueries().GetSchoolEventProgress(ctx, id, school)
}

type liveCommandProxy struct{ e *env }

func (p liveCommandProxy) JoinEvent(ctx context.Context, id uuid.UUID) (command.JoinResult, error) {
	return p.e.live().uc.JoinEvent(ctx, id)
}
func (p liveCommandProxy) ClaimMilestone(ctx context.Context, id uuid.UUID, m string) (command.ClaimResult, error) {
	return p.e.live().uc.ClaimMilestone(ctx, id, m)
}
func (p liveCommandProxy) ClaimTier(ctx context.Context, tier int) (command.ClaimResult, error) {
	return p.e.live().uc.ClaimTier(ctx, tier)
}
func (p liveCommandProxy) OperateRun(ctx context.Context, op command.RunOperation) (map[string]any, error) {
	return p.e.live().uc.OperateRun(ctx, op)
}
func (p liveCommandProxy) CreateSchoolEvent(ctx context.Context, in command.SchoolEventInput) (port.SchoolEventRow, error) {
	return p.e.live().uc.CreateSchoolEvent(ctx, in)
}
func (p liveCommandProxy) UpdateSchoolEvent(ctx context.Context, id uuid.UUID, v int64, in command.SchoolEventInput) (port.SchoolEventRow, error) {
	return p.e.live().uc.UpdateSchoolEvent(ctx, id, v, in)
}
func (p liveCommandProxy) CancelSchoolEvent(ctx context.Context, id uuid.UUID, reason string) (port.SchoolEventRow, error) {
	return p.e.live().uc.CancelSchoolEvent(ctx, id, reason)
}

// festival is a platform world festival for grade 3 of every tenant, starting in one hour for eight days: event points
// (quest.completed 20, learning_activity.completed 5, daily cap 200), milestones m1 (50 → 20 Xu) and m2 (150 → badge), a
// school community goal (correct_answers, target 100, 50/student/day, 25/50/100 %).
func (e *env) festival(id uuid.UUID, edit func(m map[string]any)) command.ContentItem {
	start := e.clock.Now().Add(time.Hour)
	m := map[string]any{"schema_version": 1, "event_id": id.String(), "code": "festival.f077_" + strings.ReplaceAll(id.String()[:8], "-", ""),
		"owner_scope": "platform", "tenant_id": nil, "event_kind": "world_festival",
		"name":        map[string]any{"vi": "Lễ hội Trăng Rằm", "en": "Moon Festival"},
		"description": map[string]any{"vi": "Thắp đèn lồng cùng cả trường", "en": "Light lanterns with your school"},
		"schedule": map[string]any{"start_at": start.Format(time.RFC3339), "end_at": start.Add(8 * 24 * time.Hour).Format(time.RFC3339),
			"timezone": "Asia/Ho_Chi_Minh", "claim_grace_hours": 72},
		"audience": map[string]any{"scope": "platform", "grade_levels": []int{3}},
		"components": map[string]any{"community_goal": map[string]any{"goal_code": "lanterns", "metric": "correct_answers", "scope": "school",
			"target": 100, "per_student_daily_cap": 50, "show_grade_contributions": true, "milestones": []any{
				map[string]any{"percent": 25, "rewards": []any{xuGrant(5)}}, map[string]any{"percent": 50, "rewards": []any{xuGrant(10)}},
				map[string]any{"percent": 100, "rewards": []any{xuGrant(15)}}}}},
		"event_points": map[string]any{"enabled": true, "daily_cap": 200, "sources": []any{
			map[string]any{"source_code": "quest.completed", "points": 20}, map[string]any{"source_code": "learning_activity.completed", "points": 5}}},
		"milestone_rewards": []any{
			map[string]any{"milestone_id": "m1", "event_points": 50, "rewards": []any{xuGrant(20)}},
			map[string]any{"milestone_id": "m2", "event_points": 150, "rewards": []any{map[string]any{"type": "badge", "badge_id": "badge_explorer_01"}}}},
	}
	if edit != nil {
		edit(m)
	}
	b, _ := json.Marshal(m)
	return command.ContentItem{Type: write.ItemEvent, Payload: b}
}

// publishEvent publishes items and fails the test on an error.
func (e *env) publishEvent(ctx context.Context, t *testing.T, items ...command.ContentItem) {
	t.Helper()
	if _, err := e.publish(ctx, t, items, "f077"); err != nil {
		t.Fatal(err)
	}
}

// runOf returns the newest run of event (read as the platform role).
func (e *env) runOf(ctx context.Context, t *testing.T, event uuid.UUID) (uuid.UUID, string) {
	t.Helper()
	var id uuid.UUID
	var status string
	if err := e.platform.QueryRowContext(ctx, `SELECT event_run_id, status FROM game.event_runs WHERE event_id = $1 ORDER BY created_at DESC LIMIT 1`,
		event).Scan(&id, &status); err != nil {
		t.Fatal(err)
	}
	return id, status
}

// tick runs one scheduler tick.
func (e *env) tick(ctx context.Context, t *testing.T) int {
	t.Helper()
	n, err := e.live().uc.RunScheduler(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// liveDeliver hands an event to the consumer game.liveops_progress in a transaction of the envelope tenant.
func (e *env) liveDeliver(ctx context.Context, t *testing.T, env outbox.Envelope) {
	t.Helper()
	c := consumer.LiveOpsProgress{LiveOps: e.live().uc}
	if err := e.inTenant(ctx, t, *env.TenantID, func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatalf("%s: %v", env.EventType, err)
	}
}

// grantsOf counts the reward grants of kind of s.
func (e *env) grantsOf(ctx context.Context, t *testing.T, s student, kind string) int {
	t.Helper()
	return count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_grants WHERE student_id = $1 AND trigger_kind = $2`, s.id, kind)
}

// platformToken is a portal token of a platform staff principal of role.
func (e *env) platformToken(role string) string {
	return e.idp.Token(e.platformStaff(role), "eduworld-portal", nowWall())
}

// staffToken is a portal token of a tenant staff principal.
func (e *env) staffToken(sub string) string { return e.idp.Token(sub, "eduworld-portal", nowWall()) }

func idem() map[string]string { return map[string]string{"Idempotency-Key": newKey()} }

func withTenant(h map[string]string, tenant uuid.UUID) map[string]string {
	h["X-Tenant-Id"] = tenant.String()
	return h
}

// operate calls a run operation through HTTP.
func (e *env) operate(t *testing.T, token string, event, run uuid.UUID, action, reason string) resp {
	t.Helper()
	return e.do(t, http.MethodPost, "/game-admin/events/"+event.String()+"/runs/"+run.String()+":"+action, token, idem(),
		map[string]any{"reason": reason})
}

// systemScope runs fn under the system platform scope (as a platform consumer).
func (e *env) systemScope(ctx context.Context, t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	ps, err := authz.SystemPlatformScope("test")
	if err != nil {
		t.Fatal(err)
	}
	return e.uow.InTenantTx(ctx, ps, fn)
}
