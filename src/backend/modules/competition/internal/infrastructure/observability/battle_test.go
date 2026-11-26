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
