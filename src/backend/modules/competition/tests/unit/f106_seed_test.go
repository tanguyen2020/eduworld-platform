package unit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// T002 (F106): each seed literal of the F106 migration equals its seed file (seed/ranking_rules.json,
// leaderboard_definitions.json, seasons.json) — one source for the defaults (R33).
func TestF106SeedMatchesMigration(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	mig, err := os.ReadFile(filepath.Join(root, "migrations", "20261231000005_f106_ranking.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ranking_rules.json", "leaderboard_definitions.json", "seasons.json"} {
		block := string(mig)
		start := strings.Index(block, "-- BEGIN seed "+name)
		end := strings.Index(block, "-- END seed "+name)
		if start < 0 || end < start {
			t.Fatalf("no seed block for %s", name)
		}
		block = block[start:end]
		i := strings.Index(block, "'{")
		j := strings.Index(block[i:], "}',")
		if i < 0 || j < 0 {
			t.Fatalf("no payload literal for %s", name)
		}
		lit := strings.ReplaceAll(block[i+1:i+j+1], "''", "'")
		var got, want any
		if err := json.Unmarshal([]byte(lit), &got); err != nil {
			t.Fatalf("%s literal: %v", name, err)
		}
		b, err := os.ReadFile(filepath.Join(root, "seed", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: the migration literal differs from the seed file", name)
		}
	}
}
