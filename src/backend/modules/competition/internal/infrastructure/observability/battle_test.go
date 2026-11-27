package observability

import (
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// MONITORING §3 (alert DuplicateMatchResults): eduworld_match_results_total{mode,result} is registered with the battle
// metrics and counts recorded and duplicate_rejected results.
func TestMatchResultsMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	btOnce = sync.Once{} // register on this test registry
	RegisterBattle(reg)
	Battle{}.MatchResult("duel", "recorded")
	Battle{}.MatchResult("duel", "duplicate_rejected")
	Battle{}.MatchResult("duel", "duplicate_rejected")
	want := `
# HELP eduworld_match_results_total RecordMatchResult calls by mode and result (recorded, duplicate_rejected — alert DuplicateMatchResults).
# TYPE eduworld_match_results_total counter
eduworld_match_results_total{mode="duel",result="duplicate_rejected"} 2
eduworld_match_results_total{mode="duel",result="recorded"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "eduworld_match_results_total"); err != nil {
		t.Fatal(err)
	}
}

// ANTI_CHEAT §10: eduworld_matches_voided_total{mode} is registered with the battle metrics, every mode exposed at 0
// from start, and counts one void per call.
func TestMatchesVoidedMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	btOnce = sync.Once{}
	RegisterBattle(reg)
	if n, err := testutil.GatherAndCount(reg, "eduworld_matches_voided_total"); err != nil || n != len(VoidModes) {
		t.Fatalf("eduworld_matches_voided_total series = %d (%v), want %d (every mode at 0)", n, err, len(VoidModes))
	}
	before := testutil.ToFloat64(MatchesVoided().WithLabelValues("battle_1v1"))
	Battle{}.MatchVoided("battle_1v1")
	if got := testutil.ToFloat64(MatchesVoided().WithLabelValues("battle_1v1")) - before; got != 1 {
		t.Fatalf("eduworld_matches_voided_total{mode=battle_1v1} += %v, want 1", got)
	}
}
