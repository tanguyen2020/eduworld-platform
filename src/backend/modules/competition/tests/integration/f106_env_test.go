//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Seeded ids of the F106 migration (seed/*.json).
var (
	preSeason        = uuid.MustParse("01936a00-0000-7000-8000-000000106001")
	lbGradeGoal      = uuid.MustParse("01936a00-0000-7000-8000-000000106101")
	lbArenaWeekly    = uuid.MustParse("01936a00-0000-7000-8000-000000106102")
	lbArenaSeason    = uuid.MustParse("01936a00-0000-7000-8000-000000106103")
	lbFriendsPoints  = uuid.MustParse("01936a00-0000-7000-8000-000000106104")
	lbGlobalSeason   = uuid.MustParse("01936a00-0000-7000-8000-000000106108")
	rankingRulesV1ID = uuid.MustParse("01936a00-0000-7000-8000-000000001061")
)

// renv is the F106 integration environment: the party environment (migrated eduworld_competition with the F106 seed,
// pkg/httpx chain, fake OIDC, tenants A/B/U) plus a real Valkey with the leaderboard store, the F106 use cases on the
// same server and the settable clock (Saturday 2026-11-21 10:00 ICT).
type renv struct {
	*penv
	vk      valkey.Client
	store   cache.LeaderboardStore
	ranks   command.Ranking
	boards  command.Leaderboards
	seasons command.Seasons
	q       query.Rankings
	subject uuid.UUID
}

func setupRank(t *testing.T) *renv {
	t.Helper()
	r := &renv{subject: uuid.New()}
	_, r.vk = valkeyLobby(t)
	r.store = cache.LeaderboardStore{C: r.vk}
	r.penv = setupParty(t, popts{extra: func(e *penv, h *competitionhttp.Handlers) {
		repo := persistence.RankingStore{}
		r.ranks = command.Ranking{Repo: repo, Boards: command.Boards{Store: r.store, Repo: repo, Metrics: observability.Ranking{}, Now: e.clock.Now},
			Events: e.parties.Events, Push: e.push, Metrics: observability.Ranking{}, AfterCommit: entx.AfterCommit}
		r.boards = command.Leaderboards{UoW: e.uow, Repo: repo, Ranking: repo, Seasons: repo, Store: r.store, Events: e.parties.Events,
			Audit: e.parties.Audit, Metrics: observability.Ranking{}, Now: e.clock.Now}
		r.seasons = command.Seasons{UoW: e.uow, Repo: repo, Ranking: repo, Events: e.parties.Events, Audit: e.parties.Audit, Now: e.clock.Now}
		r.q = query.Rankings{Read: e.read, Reader: persistence.RankingRead{}, Store: r.store, Now: e.clock.Now}
		h.Rankings = r.q
	}})
	return r
}

// school projects ref.schools with a settings subset (allow_cross_school_listing, disabled_leaderboard_definitions…).
func (r *renv) school(tenant, school uuid.UUID, settings map[string]any) {
	r.t.Helper()
	b, _ := json.Marshal(settings)
	r.exec(`INSERT INTO ref.schools (school_id, tenant_id, status, settings_subset, source_version) VALUES ($1, $2, 'active', $3, 1)
		ON CONFLICT (school_id) DO UPDATE SET settings_subset = $3`, school, tenant, string(b))
}

// consume runs a handler like pkg/outbox.Consumer: one transaction with the event scope, processed_events first (an
// event id seen before is a duplicate and the handler does not run).
func (r *renv) consume(name string, handle func(context.Context, outbox.Envelope) error, env outbox.Envelope) {
	r.t.Helper()
	sc, err := authz.EventScope(*env.TenantID)
	if err != nil {
		r.t.Fatal(err)
	}
	ctx := tenancy.WithModule(context.Background(), "competition")
	err = r.uow.InTenantTx(ctx, sc, func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		res, err := tx.ExecContext(ctx, `INSERT INTO infra.processed_events (consumer, event_id, event_occurred_at) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, name, env.EventID, env.OccurredAt.UTC())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		return handle(ctx, env)
	})
	if err != nil {
		r.t.Fatalf("%s: %v", name, err)
	}
}

// line is one participant of a match fixture.
type line struct {
	k       kid
	team    int
	outcome string
	correct int
}

// matchOf builds the envelope of a match.completed of the test subject (rated unless overridden).
func (r *renv) matchOf(tenant uuid.UUID, mode string, lines []line, edit func(p map[string]any)) outbox.Envelope {
	results := []map[string]any{}
	grade := 7
	for _, l := range lines {
		grade = l.k.grade
		results = append(results, map[string]any{"student_id": l.k.id, "tenant_id": l.k.tenant, "school_id": l.k.school, "team_no": l.team,
			"outcome": l.outcome, "score": l.correct * 100, "correct_count": l.correct})
	}
	id := uuid.New()
	p := map[string]any{"match_id": id, "room_id": id, "mode": mode, "matchmaking_type": "skill", "ranked": true, "rated": true,
		"is_bot": false, "subject_id": r.subject, "grade_level": grade, "status": "completed", "end_reason": "finished", "winner_team": 0,
		"rule_version_id": uuid.New(), "tournament_id": nil, "tournament_node_id": nil, "ended_at": r.clock.Now().UTC(), "results": results}
	if mode == "team_battle" {
		p["matchmaking_type"] = "team"
	}
	if edit != nil {
		edit(p)
	}
	b, _ := json.Marshal(p)
	t := tenant
	return outbox.Envelope{EventID: uuid.New(), EventType: "match.completed", OccurredAt: r.clock.Now(), TenantID: &t, Payload: b}
}

// played delivers a match to both match.completed consumers (ratings, then points).
func (r *renv) played(env outbox.Envelope) {
	r.t.Helper()
	r.consume(messaging.RankingMatchName, consumer.MatchCompleted{H: r.ranks.ApplyMatchRating}.Handle, env)
	r.consume(messaging.PointsMatchName, consumer.MatchCompleted{H: r.ranks.AwardMatchPoints}.Handle, env)
}

// duel plays a rated 1v1 of the test subject: a wins against b with correct answers each.
func (r *renv) duel(a, b kid, ca, cb int) outbox.Envelope {
	r.t.Helper()
	env := r.matchOf(a.tenant, "battle_1v1", []line{{a, 0, "win", ca}, {b, 1, "loss", cb}}, nil)
	r.played(env)
	return env
}

// graded delivers an activity.graded of k (attempt id returned).
func (r *renv) graded(k kid, rapid, review bool) uuid.UUID {
	r.t.Helper()
	attempt := uuid.New()
	p := map[string]any{"attempt_id": attempt, "student_id": k.id, "evaluation_id": uuid.New(), "activity_type": "question_set",
		"mode": "practice", "source": "system", "score": 1, "max_score": 1, "result_release": "immediate", "objective_ids": []string{},
		"rapid_guess_flagged": rapid}
	if review {
		p["assignment_id"] = uuid.New()
	}
	b, _ := json.Marshal(p)
	t := k.tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: "activity.graded", OccurredAt: r.clock.Now(), TenantID: &t, Payload: b}
	r.consume(messaging.LeaderboardEffortName, consumer.LeaderboardEffort{H: r.ranks.RecordEffort}.Handle, env)
	return attempt
}

// rating returns (rating, rd, games, tier) of k for the mode and the test subject in the season.
func (r *renv) rating(k kid, mode string, season uuid.UUID) (float64, float64, int, *string) {
	r.t.Helper()
	var rt, rd float64
	var games int
	var tier *string
	err := r.su.QueryRowContext(context.Background(), `SELECT rating::float8, rd::float8, games_played, tier_code FROM competition.ratings
		WHERE student_id = $1 AND mode = $2 AND subject_id = $3 AND season_id = $4 AND deleted_at IS NULL`, k.id, mode, r.subject, season).
		Scan(&rt, &rd, &games, &tier)
	if err != nil {
		r.t.Fatalf("rating of %s: %v", k.nickname, err)
	}
	return rt, rd, games, tier
}

// setRating forces a rating of k (fixture: many games played, low RD).
func (r *renv) setRating(k kid, mode string, rating, rd float64, games int, tier *string) {
	r.t.Helper()
	r.exec(`INSERT INTO competition.ratings (rating_id, tenant_id, school_id, student_id, mode, subject_id, season_id, grade_level, rating, rd,
		volatility, games_played, tier_code, peak_tier_code, last_played_at, rule_version_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 0.06, $11, $12, $12, $13, $14)
		ON CONFLICT (student_id, mode, subject_id, season_id) WHERE deleted_at IS NULL DO UPDATE SET rating = $9, rd = $10, games_played = $11,
		tier_code = $12, peak_tier_code = $12`, uuid.New(), k.tenant, k.school, k.id, mode, r.subject, preSeason, k.grade, rating, rd, games,
		tier, r.clock.Now(), rankingRulesV1ID)
}

// staffAt is the portal token of a school_admin of school (lms.student.read in that school).
func (r *renv) staffAt(tenant, school uuid.UUID, client string) string {
	sub := "kc-staff-" + uuid.NewString()
	r.res.Set(sub, authztest.Staff(tenant, "school_admin", authztest.AtSchool(school)).PrincipalData)
	return r.idp.Token(sub, client, time.Now())
}

// get calls a GET route as k.
func (r *renv) get(k kid, path string) presp {
	r.t.Helper()
	return r.call(k, http.MethodGet, path, "", nil)
}

// staffGet calls a GET route with a staff token of tenant.
func (r *renv) staffGet(token string, tenant uuid.UUID, path string) presp {
	r.t.Helper()
	return r.do(http.MethodGet, path, token, map[string]string{"X-Tenant-Id": tenant.String()}, nil, "")
}

// f106EventTypes are the events of module competition owned by F106.
var f106EventTypes = []string{"rating.updated", "competition_points.awarded", "leaderboard.period_closed", "season.started", "season.ended"}

func ptr[T any](v T) *T { return &v }

// set moves the test clock to t.
func (c *pclock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}
