//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	mmdomain "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// T010 — F106-AC-01, AC-02 (TC-CMP-008, TC-CMP-009, AC-RNK-002): a rated 1v1 updates both ratings once by Glicko-2 with
// the Ranking Rules in effect; rating_changes one row per student; rating.updated after the commit with its schema; a
// replay of the same event (processed_events) and a second emission of the match (rating_changes unique) change nothing.
func TestF106RatingAppliedOnce(t *testing.T) {
	r := setupRank(t)
	a, b := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	env := r.duel(a, b, 5, 3)
	ra, rda, ga, ta := r.rating(a, "battle_1v1", preSeason)
	rb, _, gb, _ := r.rating(b, "battle_1v1", preSeason)
	p := ranking.Glicko2Params{R0: 1500, RD0: 350, Sigma0: 0.06, Tau: 0.5, RDMin: 50, RDMax: 350, Epsilon: 1e-6}
	want := ranking.Update(p, p.Initial(), []ranking.Opponent{{R: 1500, RD: 350, Score: 1}})
	if math.Abs(ra-want.R) > 0.01 || math.Abs(rda-want.RD) > 0.01 || ga != 1 || gb != 1 || ta != nil || rb >= 1500 {
		t.Fatalf("a %.4f/%.4f games %d tier %v (want %.4f/%.4f), b %.4f", ra, rda, ga, ta, want.R, want.RD, rb)
	}
	count := func(q string, args ...any) int { return r.count(q, args...) }
	if n := count(`SELECT count(*) FROM competition.rating_changes WHERE student_id = ANY($1::uuid[])`, uuidArr(a.id, b.id)); n != 2 {
		t.Fatalf("rating_changes %d", n)
	}
	if len(r.events("rating.updated")) != 2 {
		t.Fatalf("rating.updated %d", len(r.events("rating.updated")))
	}
	// replay of the same event: the consumer sees it in processed_events
	r.played(env)
	// the match emitted again (new event id): rating_changes (match_id, student_id) keeps the first update
	env.EventID = uuid.New()
	r.played(env)
	if ra2, _, ga2, _ := r.rating(a, "battle_1v1", preSeason); ra2 != ra || ga2 != 1 {
		t.Fatalf("replayed: %.4f games %d", ra2, ga2)
	}
	if count(`SELECT count(*) FROM competition.rating_changes WHERE student_id = ANY($1::uuid[])`, uuidArr(a.id, b.id)) != 2 ||
		len(r.events("rating.updated")) != 2 || count(`SELECT count(*) FROM competition.competition_point_ledger WHERE student_id = $1`, a.id) != 1 {
		t.Fatal("a replay changed the ledgers (SC-002)")
	}
	ev := r.events("rating.updated")[0]
	if ev["rule_version_id"] != rankingRulesV1ID.String() || ev["season_id"] != preSeason.String() {
		t.Fatalf("rating.updated: %+v", ev)
	}
	if seen := r.checkSchemasOf(f106EventTypes); seen["rating.updated"] != 2 || seen["competition_points.awarded"] != 2 {
		t.Fatalf("schemas checked: %+v", seen)
	}
	// competition.tier.updated pushed to each student: tier only, never a number (F106-AC-07)
	pushes := r.push.of(a.id, command.RealtimeTierUpdated)
	if len(pushes) != 1 || pushes[0]["provisional"] != true {
		t.Fatalf("tier push: %+v", pushes)
	}
	for _, k := range []string{"rating", "rd", "volatility"} {
		if _, ok := pushes[0][k]; ok {
			t.Fatalf("tier push carries %s", k)
		}
	}
}

func uuidArr(ids ...uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// T010 — F106-AC-03 (AC-RNK-003): unranked, bot, friend duel, tournament, cancelled matches and modes without rating
// change neither the ratings nor the Competition Points.
func TestF106UnratedMatchesChangeNothing(t *testing.T) {
	r := setupRank(t)
	a, b := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	tour := uuid.New()
	for name, edit := range map[string]func(p map[string]any){
		"unranked":   func(p map[string]any) { p["ranked"], p["rated"], p["matchmaking_type"] = false, false, "random" },
		"bot":        func(p map[string]any) { p["is_bot"], p["rated"], p["matchmaking_type"] = true, false, "bot" },
		"friend":     func(p map[string]any) { p["matchmaking_type"], p["ranked"] = "friend", true },
		"tournament": func(p map[string]any) { p["tournament_id"], p["matchmaking_type"] = tour, "tournament" },
		"cancelled":  func(p map[string]any) { p["status"], p["end_reason"], p["rated"] = "cancelled", "all_left", false },
		"coop":       func(p map[string]any) { p["mode"] = "coop_challenge" },
		"rated off":  func(p map[string]any) { p["rated"] = false },
	} {
		r.played(r.matchOf(a.tenant, "battle_1v1", []line{{a, 0, "win", 5}, {b, 1, "loss", 1}}, edit))
		if n := r.count(`SELECT count(*) FROM competition.ratings WHERE student_id = ANY($1::uuid[])`, uuidArr(a.id, b.id)); n != 0 {
			t.Fatalf("%s: %d ratings", name, n)
		}
		if n := r.count(`SELECT count(*) FROM competition.competition_point_ledger WHERE student_id = ANY($1::uuid[])`, uuidArr(a.id, b.id)); n != 0 {
			t.Fatalf("%s: %d ledger entries", name, n)
		}
	}
}

// T010 — F106-AC-04: a team battle member plays a virtual opponent with μ, φ averaged over the other team; an abandoned
// member scores 0, the others their team result.
func TestF106TeamBattleVirtualOpponent(t *testing.T) {
	r := setupRank(t)
	a1, a2, a3 := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	b1, b2, b3 := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	r.setRating(b1, "team_battle", 1400, 100, 12, ptr("T2"))
	r.setRating(b2, "team_battle", 1600, 200, 12, ptr("T4"))
	r.setRating(b3, "team_battle", 1500, 300, 12, ptr("T3"))
	r.played(r.matchOf(r.tenantA, "team_battle", []line{{a1, 0, "win", 4}, {a2, 0, "win", 4}, {a3, 0, "abandoned", 1},
		{b1, 1, "loss", 2}, {b2, 1, "loss", 2}, {b3, 1, "loss", 2}}, nil))
	p := ranking.Glicko2Params{R0: 1500, RD0: 350, Sigma0: 0.06, Tau: 0.5, RDMin: 50, RDMax: 350, Epsilon: 1e-6}
	v := ranking.Virtual([]ranking.Rating{{R: 1400, RD: 100, Sigma: 0.06}, {R: 1600, RD: 200, Sigma: 0.06}, {R: 1500, RD: 300, Sigma: 0.06}})
	win := ranking.Update(p, p.Initial(), []ranking.Opponent{{R: v.R, RD: v.RD, Score: 1}})
	lose := ranking.Update(p, p.Initial(), []ranking.Opponent{{R: v.R, RD: v.RD, Score: 0}})
	for _, c := range []struct {
		k    kid
		want float64
	}{{a1, win.R}, {a2, win.R}, {a3, lose.R}} {
		if got, _, _, _ := r.rating(c.k, "team_battle", preSeason); math.Abs(got-c.want) > 0.01 {
			t.Fatalf("%s: %.4f want %.4f", c.k.nickname, got, c.want)
		}
	}
	// team battles also credit team_points (Đội board) to the members who earned points
	if n := r.count(`SELECT count(*) FROM competition.leaderboard_contributions WHERE metric = 'team_points' AND student_id = $1`, a1.id); n != 1 {
		t.Fatalf("team_points of a1: %d", n)
	}
}

// T014 — F106-AC-05: the real RatingReader of matchmaking (F104) reads μ/RD of the active season (provisional or not),
// the default rating otherwise.
func TestF106RatingReaderForMatchmaking(t *testing.T) {
	r := setupRank(t)
	a, b, c := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	r.duel(a, b, 5, 1)
	ra, rda, _, _ := r.rating(a, "battle_1v1", preSeason)
	sc, _ := authz.EventScope(r.tenantA)
	var got map[uuid.UUID]mmdomain.Rating
	err := r.uow.InTenantTx(tenancy.WithModule(context.Background(), "competition"), sc, func(ctx context.Context) error {
		var err error
		got, err = persistence.RatingReader{}.Ratings(ctx, r.tenantA, []uuid.UUID{a.id, c.id}, "battle_1v1", r.subject, mmdomain.Rating{Mu: 1500, RD: 350})
		return err
	})
	if err != nil || math.Abs(got[a.id].Mu-ra) > 1e-6 || math.Abs(got[a.id].RD-rda) > 1e-6 || got[c.id].Mu != 1500 || got[c.id].RD != 350 {
		t.Fatalf("ratings %+v %v", got, err)
	}
}

// T011 — F106-AC-06: the weekly job raises the RD of ratings not played for a week by the no-games step (never above RD
// max), never changes the rating; a second run in the same week changes nothing.
func TestF106RDInflationJob(t *testing.T) {
	r := setupRank(t)
	a, b := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	r.setRating(a, "battle_1v1", 1720, 80, 30, ptr("T4"))
	r.setRating(b, "battle_1v1", 1500, 349.99, 30, ptr("T3"))
	r.exec(`UPDATE competition.ratings SET last_played_at = $1, updated_at = $1 WHERE student_id = ANY($2::uuid[])`,
		r.clock.Now().Add(-8*24*time.Hour), uuidArr(a.id, b.id))
	n, err := r.seasons.InflateRD(jobCtx())
	if err != nil || n != 2 {
		t.Fatalf("inflate: %d %v", n, err)
	}
	ra, rda, _, _ := r.rating(a, "battle_1v1", preSeason)
	_, rdb, _, _ := r.rating(b, "battle_1v1", preSeason)
	if ra != 1720 || rda <= 80 || rdb != 350 {
		t.Fatalf("a %.4f/%.4f b rd %.4f", ra, rda, rdb)
	}
	r.exec(`UPDATE competition.ratings SET last_played_at = $1 WHERE student_id = ANY($2::uuid[])`, r.clock.Now().Add(-8*24*time.Hour),
		uuidArr(a.id, b.id))
	if n, err := r.seasons.InflateRD(jobCtx()); err != nil || n != 0 {
		t.Fatalf("second run the same week: %d %v", n, err)
	}
}

// T016/T018 — F106-AC-05, AC-07, AC-08: getMyRatings returns tier code and name (Accept-Language), provisional and its
// progress, games played — never rating/RD; provisional students have no tier ("Đang xếp hạng (x/10)"); staff with a
// game token is refused, another tenant in X-Tenant-Id is TENANT_MISMATCH.
func TestF106GetMyRatings(t *testing.T) {
	r := setupRank(t)
	a, b := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	for i := 0; i < 3; i++ {
		r.duel(a, b, 5, 2)
	}
	res := r.get(a, "/competition/ratings/me")
	if res.status != http.StatusOK || len(res.data()) != 1 {
		t.Fatalf("getMyRatings: %d %s", res.status, res.raw)
	}
	d := res.data()[0]
	if d["provisional"] != true || d["tier_code"] != nil || d["games_played"].(float64) != 3 || d["provisional_games_required"].(float64) != 10 {
		t.Fatalf("provisional: %+v", d)
	}
	for _, k := range []string{`"rating"`, `"rd"`, `"volatility"`} {
		if strings.Contains(res.raw, k) {
			t.Fatalf("getMyRatings leaks %s: %s", k, res.raw)
		}
	}
	r.setRating(a, "battle_1v1", 1520, 80, 12, ptr("T3"))
	res = r.do(http.MethodGet, "/competition/ratings/me", a.token, map[string]string{"X-Tenant-Id": a.tenant.String(), "Accept-Language": "en-US"}, nil, "")
	d = res.data()[0]
	if d["provisional"] != false || d["tier_code"] != "T3" || d["tier_name"] != "Young Bud" {
		t.Fatalf("tier: %+v", d)
	}
	res = r.get(a, "/competition/ratings/me")
	if res.data()[0]["tier_name"] != "Chồi Non" {
		t.Fatalf("vi-VN name: %s", res.raw)
	}
	if res := r.get(a, "/competition/ratings/me?season_id="+uuid.NewString()); res.status != http.StatusNotFound {
		t.Fatalf("unknown season: %d", res.status)
	}
	if res := r.staffGet(r.staffAt(r.tenantA, r.schoolA, httpxtest.GameClient), r.tenantA, "/competition/ratings/me"); res.status != http.StatusForbidden {
		t.Fatalf("staff: %d %s", res.status, res.raw)
	}
	if res := r.do(http.MethodGet, "/competition/ratings/me", a.token, map[string]string{"X-Tenant-Id": r.tenantB.String()}, nil, ""); res.status !=
		http.StatusForbidden || res.code() != "TENANT_MISMATCH" {
		t.Fatalf("tenant mismatch: %d %s", res.status, res.raw)
	}
	authztest.Cover(t, "getMyRatings", authztest.CrossTenant, authztest.RoleDenied)
}

// T019 — F106-AC-10, AC-11 (TC-CMP-009, AC-RNK-006): the point table (completed, win, draw, correct answers capped,
// higher tier beaten, abandon never below 0), a loss never deducted, the daily cap with capped entries, the 21st match
// of the day 0; one ledger entry per (source, student) and competition_points.awarded.
func TestF106CompetitionPoints(t *testing.T) {
	r := setupRank(t)
	a, b, c := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	// a (T3) beats c (T5): upset
	r.setRating(a, "battle_1v1", 1500, 80, 20, ptr("T3"))
	r.setRating(c, "battle_1v1", 1800, 80, 20, ptr("T5"))
	r.played(r.matchOf(r.tenantA, "battle_1v1", []line{{a, 0, "win", 12}, {c, 1, "loss", 0}}, nil))
	amount := func(k kid) []int {
		rows, err := r.su.QueryContext(context.Background(), `SELECT amount FROM competition.competition_point_ledger WHERE student_id = $1
			ORDER BY occurred_at, entry_id`, k.id)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		var out []int
		for rows.Next() {
			var n int
			_ = rows.Scan(&n)
			out = append(out, n)
		}
		return out
	}
	if got := amount(a); len(got) != 1 || got[0] != 10+15+10+5 {
		t.Fatalf("upset win: %v", got)
	}
	if got := amount(c); len(got) != 1 || got[0] != 10 {
		t.Fatalf("loss: %v (never deducted)", got)
	}
	// abandon: −5 but the day never below 0 (b has nothing today)
	r.played(r.matchOf(r.tenantA, "battle_1v1", []line{{b, 0, "abandoned", 0}, {c, 1, "win", 3}}, nil))
	if got := amount(b); len(got) != 1 || got[0] != 0 {
		t.Fatalf("abandon on an empty day: %v", got)
	}
	// 21 wins in a day: the total never exceeds 300 and the 21st gives 0 with reason capped
	d := r.kid(r.tenantA, r.schoolA, 7)
	for i := 0; i < 21; i++ {
		r.played(r.matchOf(r.tenantA, "battle_1v1", []line{{d, 0, "win", 10}, {b, 1, "loss", 0}}, nil))
		r.clock.Add(time.Minute)
	}
	got := amount(d)
	total := 0
	for _, n := range got {
		total += n
	}
	if len(got) != 21 || total != 300 || got[20] != 0 {
		t.Fatalf("21 matches: total %d %v", total, got)
	}
	if n := r.count(`SELECT count(*) FROM competition.competition_point_ledger WHERE student_id = $1 AND reason = 'capped'`, d.id); n < 2 {
		t.Fatalf("capped entries %d", n)
	}
	var ev map[string]any
	for _, e := range r.events("competition_points.awarded") {
		if e["student_id"] == d.id.String() {
			ev = e
		}
	}
	if ev == nil || ev["balance_after"].(float64) != 300 || ev["capped"] != true || ev["points"].(float64) != 0 {
		t.Fatalf("competition_points.awarded: %+v", ev)
	}
	if seen := r.checkSchemasOf([]string{"competition_points.awarded"}); seen["competition_points.awarded"] < 24 {
		t.Fatalf("schemas: %+v", seen)
	}
	b2, _ := json.Marshal(ev)
	if strings.Contains(string(b2), "nickname") {
		t.Fatal("event carries a nickname")
	}
}
