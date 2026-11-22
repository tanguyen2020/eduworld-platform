//go:build integration

package integration

import (
	"context"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// T032 — F106-AC-22, AC-23, AC-24 (AC-RNK-004): the scheduler ends the pre-season when the first season starts — the
// season results (final rating, RD, peak tier, games, points) first, then season.ended once — and starts the new season
// with the soft reset (1500 + 0.5·(r − 1500), RD ≥ 150, volatility kept), Competition Points from 0 and season.started
// once; the old season stays readable (listSeasons/getSeason, peak tier of the student).
func TestF106SeasonLifecycle(t *testing.T) {
	r := setupRank(t)
	a, b := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	r.setRating(a, "battle_1v1", 1900, 60, 40, ptr("T6"))
	r.points(a, 120, r.clock.Now())
	s1 := uuid.New()
	starts := r.clock.Now().Add(time.Hour)
	r.exec(`INSERT INTO competition.seasons (season_id, code, name, theme_code, starts_at, ends_at, status, rules)
		VALUES ($1, '2027-s1', 'Mùa Phép Nhân', 'multiplication', $2, $3, 'scheduled', '{}')`, s1, starts, starts.Add(8*7*24*time.Hour))
	if n, err := r.seasons.Tick(jobCtx()); err != nil || n != 0 {
		t.Fatalf("before the start: %d %v", n, err)
	}
	r.clock.Add(2 * time.Hour)
	if n, err := r.seasons.Tick(jobCtx()); err != nil || n != 2 {
		t.Fatalf("tick: %d %v", n, err)
	}
	if n, err := r.seasons.Tick(jobCtx()); err != nil || n != 0 {
		t.Fatalf("second tick: %d %v", n, err)
	}
	if len(r.events("season.ended")) != 1 || len(r.events("season.started")) != 1 {
		t.Fatalf("season events: %d ended, %d started", len(r.events("season.ended")), len(r.events("season.started")))
	}
	if st := r.events("season.started")[0]; st["season_id"] != s1.String() || st["previous_season_id"] != preSeason.String() {
		t.Fatalf("season.started: %+v", st)
	}
	if seen := r.checkSchemasOf([]string{"season.started", "season.ended"}); seen["season.started"] != 1 || seen["season.ended"] != 1 {
		t.Fatalf("schemas %+v", seen)
	}
	var final, rd float64
	var peak string
	var games, pts int
	if err := r.su.QueryRowContext(context.Background(), `SELECT final_rating::float8, final_rd::float8, peak_tier_code, games_played, competition_points
		FROM competition.season_results WHERE season_id = $1 AND student_id = $2`, preSeason, a.id).Scan(&final, &rd, &peak, &games, &pts); err != nil {
		t.Fatal(err)
	}
	if final != 1900 || rd != 60 || peak != "T6" || games != 40 || pts != 120 {
		t.Fatalf("season result: %v %v %s %d %d", final, rd, peak, games, pts)
	}
	nr, nrd, ng, nt := r.rating(a, "battle_1v1", s1)
	if math.Abs(nr-1700) > 1e-6 || nrd != 150 || ng != 0 || nt != nil {
		t.Fatalf("soft reset: %v/%v games %d tier %v", nr, nrd, ng, nt)
	}
	var vol float64
	_ = r.su.QueryRowContext(context.Background(), `SELECT volatility::float8 FROM competition.ratings WHERE student_id = $1 AND season_id = $2`, a.id, s1).Scan(&vol)
	if vol != 0.06 {
		t.Fatalf("volatility %v", vol)
	}
	// a match of the new season: rating from the reset, points from 0 in the new season
	r.duel(a, b, 5, 1)
	if n := r.count(`SELECT count(*) FROM competition.competition_point_ledger WHERE student_id = $1 AND season_id = $2`, a.id, s1); n != 1 {
		t.Fatalf("new season ledger %d", n)
	}
	if ra, _, g, _ := r.rating(a, "battle_1v1", s1); g != 1 || ra <= 1700 {
		t.Fatalf("first match of the season: %v games %d", ra, g)
	}
	l := r.get(a, "/competition/seasons")
	if l.status != http.StatusOK || len(l.data()) != 2 || l.data()[0]["season_id"] != s1.String() || l.data()[1]["status"] != "ended" ||
		l.data()[1]["my_peak_tier_code"] != "T6" {
		t.Fatalf("listSeasons: %d %s", l.status, l.raw)
	}
	g := r.get(a, "/competition/seasons/"+preSeason.String())
	if g.status != http.StatusOK || g.body["status"] != "ended" || g.body["code"] != "pre-season" {
		t.Fatalf("getSeason: %d %s", g.status, g.raw)
	}
	staff := r.staffGet(r.staffAt(r.tenantA, r.schoolA, httpxtest.PortalClient), r.tenantA, "/competition/seasons")
	if staff.status != http.StatusOK || len(staff.data()) != 2 {
		t.Fatalf("staff listSeasons: %d %s", staff.status, staff.raw)
	}
	if _, ok := staff.data()[1]["my_peak_tier_code"]; ok {
		t.Fatal("staff gets a peak tier")
	}
	if res := r.get(a, "/competition/seasons/"+uuid.NewString()); res.status != http.StatusNotFound {
		t.Fatalf("unknown season: %d", res.status)
	}
	if res := r.do(http.MethodGet, "/competition/seasons", a.token, map[string]string{"X-Tenant-Id": r.tenantB.String()}, nil, ""); res.status !=
		http.StatusForbidden || res.code() != "TENANT_MISMATCH" {
		t.Fatalf("tenant mismatch: %d %s", res.status, res.raw)
	}
	if res := r.staffGet(r.platformToken("system_admin"), r.tenantA, "/competition/seasons"); res.status < 400 {
		t.Fatalf("platform principal without tenant: %d", res.status)
	}
	// past the end of the new season: ended, results kept
	r.clock.Add(9 * 7 * 24 * time.Hour)
	if n, err := r.seasons.Tick(jobCtx()); err != nil || n != 1 {
		t.Fatalf("end of S1: %d %v", n, err)
	}
	if n := r.count(`SELECT count(*) FROM competition.season_results WHERE season_id = $1`, s1); n != 2 {
		t.Fatalf("S1 results %d", n)
	}
	authztest.Cover(t, "listSeasons", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getSeason", authztest.CrossTenant, authztest.RoleDenied)
}
