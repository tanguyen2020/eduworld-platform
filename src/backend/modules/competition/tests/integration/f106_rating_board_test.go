//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// F106-AC-05, AC-07, AC-12 (FR-LDB-001 "Season"): a rating board (metric rating, mode × subject, season, grades 6–9)
// ranks only non-provisional ratings and shows tier + rank, never the rating number.
func TestF106RatingBoard(t *testing.T) {
	r := setupRank(t)
	board := uuid.New()
	r.exec(`INSERT INTO competition.leaderboard_definitions (leaderboard_id, code, title, metric, scope_type, period, filters, grade_levels, display)
		VALUES ($1, 'rating_math', 'Xếp hạng rating', 'rating', 'grade', 'season', $2, '[6,7,8,9]', '{}')`, board,
		`{"mode":"battle_1v1","subject_id":"`+r.subject.String()+`"}`)
	a, b, c := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	r.setRating(a, "battle_1v1", 1820, 70, 25, ptr("T5"))
	r.setRating(b, "battle_1v1", 1610, 90, 14, ptr("T4"))
	r.setRating(c, "battle_1v1", 1990, 200, 4, nil) // provisional: 4 games, RD 200
	res := r.board(board, c, "")
	es := entriesOf(res)
	if res.status != 200 || len(es) != 2 || nickOf(es[0]) != a.nickname || es[0]["tier_code"] != "T5" || es[1]["tier_code"] != "T4" {
		t.Fatalf("rating board: %d %s", res.status, res.raw)
	}
	for _, e := range es {
		if _, ok := e["value"]; ok {
			t.Fatalf("rating board shows a value: %s", res.raw)
		}
	}
	if res.body["me"] != nil {
		t.Fatalf("a provisional student is ranked: %s", res.raw)
	}
	if strings.Contains(res.raw, "1820") || strings.Contains(res.raw, "1610") || strings.Contains(res.raw, `"rd"`) {
		t.Fatalf("rating numbers leak: %s", res.raw)
	}
	if res := r.board(board, r.kid(r.tenantA, r.schoolA, 5), ""); res.status != 404 {
		t.Fatalf("grade 5 opens a rating board: %d", res.status)
	}
}
