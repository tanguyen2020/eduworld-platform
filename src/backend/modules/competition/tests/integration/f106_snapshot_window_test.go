//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// snapshotFixture writes the snapshot of one closed period of lbArenaWeekly in the grade scope of kids (rank i+1 for
// kids[i], members = len(kids)) as the period close would (append-only rows, one capture instant).
func (r *renv) snapshotFixture(kids []kid, period string, captured time.Time) {
	r.t.Helper()
	k0 := kids[0]
	scope := ldb.GradeKey(k0.school, k0.grade)
	for i, k := range kids {
		r.exec(`INSERT INTO competition.leaderboard_snapshots (snapshot_id, tenant_id, leaderboard_id, scope_id, school_id, grade_level,
			period_key, student_id, rank, value, members, captured_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			uuid.New(), k.tenant, lbArenaWeekly, scope, k.school, k.grade, period, k.id, i+1, 1000-i, len(kids), captured)
	}
}

// inTenant runs fn in a transaction of tenant (RLS on), like the read session of a request of that tenant.
func (r *renv) inTenant(tenant uuid.UUID, fn func(ctx context.Context) error) {
	r.t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := r.uow.InTenantTx(tenancy.WithModule(context.Background(), "competition"), sc, fn); err != nil {
		r.t.Fatal(err)
	}
}

// oldSnapshotView is the reference of the in-memory implementation replaced on 2026-11-24 (owner rule: never cut a page
// in memory): read the top 100 of the period, the viewer's row, then cut the page [off+1, off+size] and the window
// [rank − around, rank + around] out of the top 100 in Go.
func oldSnapshotView(ctx context.Context, tenant uuid.UUID, scope, period string, viewer *uuid.UUID, off, size, around int) (bool, int,
	[]ldb.Ranked, *ldb.Ranked, []ldb.Ranked, error) {
	t, _ := entx.Tx(ctx)
	var all []ldb.Ranked
	found, total := false, 0
	// The rows are closed before the next statement of the transaction (one statement at a time per connection).
	if err := func() error {
		rows, err := t.QueryContext(ctx, `SELECT student_id, rank, value::float8, members FROM competition.leaderboard_snapshots WHERE tenant_id = $1
		AND leaderboard_id = $2 AND scope_id = $3 AND period_key = $4 AND (rank IS NULL OR rank <= 100) ORDER BY rank NULLS FIRST LIMIT 101`,
			tenant, lbArenaWeekly, scope, period)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			found = true
			var st uuid.NullUUID
			var rank, members sql.NullInt64
			var v float64
			if err := rows.Scan(&st, &rank, &v, &members); err != nil {
				return err
			}
			if !st.Valid || !rank.Valid {
				continue
			}
			all = append(all, ldb.Ranked{Student: st.UUID, Rank: int(rank.Int64), Value: v})
			if members.Valid {
				total = int(members.Int64)
			}
		}
		return rows.Err()
	}(); err != nil {
		return false, 0, nil, nil, nil, err
	}
	if !found {
		return false, 0, nil, nil, nil, nil
	}
	cut := func(from, to int) []ldb.Ranked {
		from, to = max(from, 1), min(to, len(all))
		if from > to {
			return nil
		}
		return append([]ldb.Ranked(nil), all[from-1:to]...)
	}
	top := cut(off+1, off+size)
	var me *ldb.Ranked
	var aroundRows []ldb.Ranked
	if viewer != nil {
		var rank int
		var v float64
		err := t.QueryRowContext(ctx, `SELECT rank, value::float8 FROM competition.leaderboard_snapshots WHERE tenant_id = $1 AND leaderboard_id = $2
			AND scope_id = $3 AND period_key = $4 AND student_id = $5`, tenant, lbArenaWeekly, scope, period, *viewer).Scan(&rank, &v)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, 0, nil, nil, nil, err
		}
		if err == nil {
			me = &ldb.Ranked{Student: *viewer, Rank: rank, Value: v}
			aroundRows = cut(rank-around, rank+around)
		}
	}
	return true, max(total, len(all)), top, me, aroundRows, nil
}

// splitView cuts the rows of a SnapshotView the way the query side does: the page [from, to] and the window around me.
func splitView(v *read.SnapshotView, from, to, around int) ([]ldb.Ranked, []ldb.Ranked) {
	var top, near []ldb.Ranked
	for _, x := range v.Rows {
		if x.Rank >= from && x.Rank <= to {
			top = append(top, x)
		}
		if v.Me != nil && x.Rank >= v.Me.Rank-around && x.Rank <= v.Me.Rank+around {
			near = append(near, x)
		}
	}
	return top, near
}

// Owner rule 2026-11-24 (API_DESIGN §5, CODE_PLAN §4.3): a closed period read from leaderboard_snapshots cuts the page
// and the ±N window around the student in SQL. The SQL result equals the replaced in-memory result on a matrix of
// pages, viewers (top, middle, rank 100, beyond the top 100, not a member) and windows; tenant B reads nothing of A.
func TestF106SnapshotWindowEqualsInMemory(t *testing.T) {
	r := setupRank(t)
	var kids []kid
	for i := 0; i < 104; i++ {
		kids = append(kids, r.kid(r.tenantA, r.schoolA, 7))
	}
	bk := []kid{r.kid(r.tenantB, r.schoolB, 7), r.kid(r.tenantB, r.schoolB, 7)}
	captured := time.Date(2026, 11, 16, 17, 5, 0, 0, time.UTC)
	r.snapshotFixture(kids, "2026-W46", captured)
	r.snapshotFixture(bk, "2026-W46", captured)
	scope := ldb.GradeKey(r.schoolA, 7)
	stranger := uuid.New()
	viewers := []*uuid.UUID{nil, &kids[0].id, &kids[49].id, &kids[99].id, &kids[102].id, &stranger}
	pages := [][2]int{{0, 40}, {40, 40}, {80, 40}, {0, 10}, {94, 10}, {100, 20}, {0, 100}}
	cases := 0
	r.inTenant(r.tenantA, func(ctx context.Context) error {
		for _, viewer := range viewers {
			for _, pg := range pages {
				for _, around := range []int{0, 3} {
					off, size := pg[0], pg[1]
					found, total, top, me, near, err := oldSnapshotView(ctx, r.tenantA, scope, "2026-W46", viewer, off, size, around)
					if err != nil {
						return err
					}
					v, err := persistence.RankingRead{}.SnapshotWindow(ctx, r.tenantA, lbArenaWeekly, scope, "2026-W46",
						read.RankWindowQuery{From: off + 1, To: off + size, Viewer: viewer, Around: around})
					if err != nil {
						return err
					}
					name := fmt.Sprintf("viewer %v page %v around %d", viewer, pg, around)
					if !found || v == nil {
						t.Fatalf("%s: found old %v new %v", name, found, v != nil)
					}
					gotTop, gotNear := splitView(v, off+1, off+size, around)
					if v.Total != total || !reflect.DeepEqual(gotTop, top) || !reflect.DeepEqual(v.Me, me) || !reflect.DeepEqual(gotNear, near) {
						t.Fatalf("%s:\n new total %d top %v me %v around %v\n old total %d top %v me %v around %v", name, v.Total, gotTop, v.Me,
							gotNear, total, top, me, near)
					}
					if !v.Captured.Equal(captured) {
						t.Fatalf("%s: captured %v", name, v.Captured)
					}
					cases++
				}
			}
		}
		// not snapshotted (another period, another tenant's scope): nil, the caller falls back to the ledgers
		for _, q := range [][2]string{{scope, "2026-W45"}, {ldb.GradeKey(r.schoolB, 7), "2026-W46"}} {
			if v, err := (persistence.RankingRead{}).SnapshotWindow(ctx, r.tenantA, lbArenaWeekly, q[0], q[1], read.RankWindowQuery{From: 1, To: 10}); err != nil || v != nil {
				t.Fatalf("%v: %+v %v", q, v, err)
			}
		}
		return nil
	})
	if cases != len(viewers)*len(pages)*2 {
		t.Fatalf("cases %d", cases)
	}
	// RLS: even with the scope of A, tenant B reads nothing of tenant A
	r.inTenant(r.tenantB, func(ctx context.Context) error {
		v, err := persistence.RankingRead{}.SnapshotWindow(ctx, r.tenantB, lbArenaWeekly, scope, "2026-W46", read.RankWindowQuery{From: 1, To: 100})
		if err != nil || v != nil {
			t.Fatalf("tenant B read tenant A snapshot: %+v %v", v, err)
		}
		return nil
	})
}

// Owner rule 2026-11-24: staff pages of a closed period walk the snapshot (top 100) in rank order without duplicates or
// gaps; a student beyond the top 100 still gets its own rank; getLeaderboardHistory pages the closed periods by keyset
// (captured_at, period_key) in SQL — newest first, more periods than one page, a stable cursor — and tenant B never
// sees a period or a member of tenant A.
func TestF106SnapshotPagesAndHistoryKeyset(t *testing.T) {
	r := setupRank(t)
	var kids []kid
	for i := 0; i < 104; i++ {
		kids = append(kids, r.kid(r.tenantA, r.schoolA, 7))
	}
	bk := []kid{r.kid(r.tenantB, r.schoolB, 7), r.kid(r.tenantB, r.schoolB, 7)}
	// W40..W46 closed, each captured on its Monday; W46 holds every member, the others the first 3
	var periods []string
	for w := 40; w <= 46; w++ {
		p := fmt.Sprintf("2026-W%d", w)
		at := time.Date(2026, 10, 5, 17, 5, 0, 0, time.UTC).AddDate(0, 0, 7*(w-40))
		members := kids[:3]
		if w == 46 {
			members = kids
		}
		r.snapshotFixture(members, p, at)
		periods = append([]string{p}, periods...)
	}
	r.snapshotFixture(bk, "2026-W46", time.Date(2026, 11, 16, 17, 5, 0, 0, time.UTC))
	staff := r.staffAt(r.tenantA, r.schoolA, httpxtest.PortalClient)
	base := "/leaderboards/" + lbArenaWeekly.String() + "?scope_id=" + r.schoolA.String() + "&grade_level=7&period_key=2026-W46&limit=40"
	var ranks []int
	cursor := ""
	for i := 0; i < 5; i++ {
		path := base
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		res := r.staffGet(staff, r.tenantA, path)
		if res.status != http.StatusOK || res.body["stale"] != false {
			t.Fatalf("page %d: %d %s", i+1, res.status, res.raw)
		}
		for _, e := range entriesOf(res) {
			ranks = append(ranks, int(e["rank"].(float64)))
		}
		pg := res.body["page"].(map[string]any)
		if pg["has_more"] != true {
			break
		}
		cursor = pg["next_cursor"].(string)
	}
	if len(ranks) != 100 {
		t.Fatalf("staff pages hold %d rows, want the top 100", len(ranks))
	}
	for i, rk := range ranks {
		if rk != i+1 {
			t.Fatalf("ranks %v: gap or duplicate at %d", ranks, i)
		}
	}
	// a student beyond the top 100 keeps its own rank; a student of the middle sees ±3 around itself
	far := r.board(lbArenaWeekly, kids[102], "period_key=2026-W46")
	if m, _ := far.body["me"].(map[string]any); far.status != http.StatusOK || m == nil || m["rank"].(float64) != 103 {
		t.Fatalf("rank 103: %d %s", far.status, far.raw)
	}
	mid := r.board(lbArenaWeekly, kids[59], "period_key=2026-W46")
	near, _ := mid.body["around_me"].([]any)
	if mid.status != http.StatusOK || len(near) != 7 || near[0].(map[string]any)["rank"].(float64) != 57 {
		t.Fatalf("around rank 60: %d %s", mid.status, mid.raw)
	}
	// history: pages of 3 periods, newest first, keyset cursor
	var got []string
	cursor = ""
	for i := 0; i < 5; i++ {
		path := "/leaderboards/" + lbArenaWeekly.String() + "/history?limit=3"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		h := r.get(kids[0], path)
		if h.status != http.StatusOK || len(h.data()) > 3 {
			t.Fatalf("history page %d: %d %s", i+1, h.status, h.raw)
		}
		for _, d := range h.data() {
			got = append(got, d["period_key"].(string))
			if es := d["entries"].([]any); len(es) == 0 || es[0].(map[string]any)["rank"].(float64) != 1 {
				t.Fatalf("history entries of %s: %v", d["period_key"], es)
			}
		}
		pg := h.body["page"].(map[string]any)
		if pg["has_more"] != true {
			if pg["next_cursor"] != nil {
				t.Fatalf("last history page with a cursor: %s", h.raw)
			}
			break
		}
		cursor = pg["next_cursor"].(string)
		// the same cursor answers the same page (stable)
		again := r.get(kids[0], "/leaderboards/"+lbArenaWeekly.String()+"/history?limit=3&cursor="+cursor)
		third := r.get(kids[0], "/leaderboards/"+lbArenaWeekly.String()+"/history?limit=3&cursor="+cursor)
		if again.raw != third.raw {
			t.Fatalf("cursor not stable: %s / %s", again.raw, third.raw)
		}
	}
	if strings.Join(got, ",") != strings.Join(periods, ",") {
		t.Fatalf("history = %v, want %v", got, periods)
	}
	if bad := r.get(kids[0], "/leaderboards/"+lbArenaWeekly.String()+"/history?cursor=o3"); bad.status != http.StatusUnprocessableEntity {
		t.Fatalf("offset cursor accepted: %d %s", bad.status, bad.raw)
	}
	// tenant B: its own period and members only
	hb := r.get(bk[0], "/leaderboards/"+lbArenaWeekly.String()+"/history?limit=100")
	if hb.status != http.StatusOK || len(hb.data()) != 1 || strings.Contains(hb.raw, kids[0].nickname) {
		t.Fatalf("tenant B history: %d %s", hb.status, hb.raw)
	}
	if res := r.board(lbArenaWeekly, bk[1], "period_key=2026-W46"); res.status != http.StatusOK || strings.Contains(res.raw, kids[0].nickname) ||
		len(entriesOf(res)) != 2 {
		t.Fatalf("tenant B closed week: %d %s", res.status, res.raw)
	}
}
