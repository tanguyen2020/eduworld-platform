package query

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
)

// The keyset cursor of getLeaderboardHistory round-trips (captured_at to the microsecond, any period key) and refuses
// the offset cursors of the former OFFSET paging and malformed values.
func TestPeriodCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 11, 16, 17, 5, 0, 123456000, time.UTC)
	for _, period := range []string{"2026-W46", "2026-11", "2026-11-16", "season.x"} {
		c := periodCursor(read.SnapshotPeriod{Period: period, Captured: at})
		k, err := periodKeysetOf(c)
		if err != nil || k == nil || k.Period != period || !k.Captured.Equal(at) {
			t.Fatalf("%s: %+v %v", c, k, err)
		}
	}
	if k, err := periodKeysetOf(""); k != nil || err != nil {
		t.Fatalf("first page: %+v %v", k, err)
	}
	for _, bad := range []string{"o3", "p", "p12", "p12.", "pabc.2026-W46", "x1.2026-W46"} {
		if _, err := periodKeysetOf(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

// split keeps the page [From, To] and the ranks within Around of me, in rank order (the shared cut of the SQL windows).
func TestSplitWindow(t *testing.T) {
	var rows []ldb.Ranked
	for r := 1; r <= 12; r++ {
		rows = append(rows, ldb.Ranked{Student: uuid.New(), Rank: r})
	}
	me := rows[9]
	v := split(42, rows, &me, read.RankWindowQuery{From: 1, To: 3, Around: 2})
	if v.total != 42 || len(v.top) != 3 || v.top[2].Rank != 3 || len(v.around) != 5 || v.around[0].Rank != 8 || v.around[4].Rank != 12 {
		t.Fatalf("%+v", v)
	}
	if v := split(5, rows[:3], nil, read.RankWindowQuery{From: 1, To: 10, Around: 3}); v.me != nil || v.around != nil || len(v.top) != 3 {
		t.Fatalf("no viewer: %+v", v)
	}
}
