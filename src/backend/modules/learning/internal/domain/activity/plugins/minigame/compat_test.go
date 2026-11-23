package minigame_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
)

// The table of MINIGAME_ENGINE §2 with the question type names of the OpenAPI QuestionType (data-model §3).
var engineTable = map[string][]string{
	"multiple_choice":    {"single_choice", "multiple_choice", "image_based"},
	"true_false":         {"true_false"},
	"fill_in_blank":      {"fill_in_blank", "short_answer"},
	"drag_and_drop":      {"drag_drop", "image_based"},
	"matching":           {"matching"},
	"ordering":           {"ordering"},
	"calculation":        {"numeric"},
	"puzzle":             {"single_choice", "multiple_choice", "true_false", "fill_in_blank", "short_answer", "numeric", "matching", "ordering", "drag_drop", "image_based", "interactive"},
	"word_game":          {"fill_in_blank", "short_answer", "ordering"},
	"science_simulation": {"interactive"},
	"timed_challenge":    {"single_choice", "true_false", "numeric"},
}

// F073-AC-01, FR-MG-001 (T022): the compatibility table equals MINIGAME_ENGINE §2 row by row, for the 11 types and
// nothing else.
func TestCompatibilityTable(t *testing.T) {
	if len(minigame.Types) != 11 || len(engineTable) != 11 {
		t.Fatalf("types: %v", minigame.Types)
	}
	for _, mt := range minigame.Types {
		want, ok := engineTable[mt]
		if !ok {
			t.Fatalf("%s is not a type of MINIGAME_ENGINE §2", mt)
		}
		got := minigame.Allowed(mt)
		if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
			t.Errorf("%s: %v, want %v", mt, got, want)
		}
		for _, qt := range minigame.AutoGraded {
			if minigame.Accepts(mt, qt) != slices.Contains(want, qt) {
				t.Errorf("%s accepts %s: %v", mt, qt, !slices.Contains(want, qt))
			}
		}
	}
	if minigame.Allowed("drag_drop") != nil || minigame.Valid("adaptive") {
		t.Fatal("unknown type accepted")
	}
	if !minigame.AcceptsEveryType("puzzle") || minigame.AcceptsEveryType("matching") {
		t.Fatal("every type: puzzle only")
	}
}

// The rows of the table in docs/08-game-client/MINIGAME_ENGINE.md §2 are the same as the code (one source of truth, R33):
// the doc names drag_drop as `drag_drop` and puzzle as "every auto-graded type".
func TestCompatibilityTableMatchesDoc(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "..", "..", "..")
	b, err := os.ReadFile(filepath.Join(root, "docs", "08-game-client", "MINIGAME_ENGINE.md"))
	if err != nil {
		t.Skipf("MINIGAME_ENGINE.md not found: %v", err)
	}
	row := regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\|[^|]*\\| ([^|]*) \\|")
	seen := 0
	for _, m := range row.FindAllStringSubmatch(string(b), -1) {
		mt, cell := m[1], m[2]
		want, ok := engineTable[mt]
		if !ok {
			continue
		}
		seen++
		if mt == "puzzle" {
			continue
		}
		var got []string
		for _, q := range regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(cell, -1) {
			got = append(got, q[1])
		}
		if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
			t.Errorf("doc row %s: %v, code %v (%s)", mt, got, want, strings.TrimSpace(cell))
		}
	}
	if seen != 11 {
		t.Fatalf("%d rows of the doc table found", seen)
	}
}

// F073-AC-02 (TC-ACT-003): the publish reasons of a question source.
func TestCheckSource(t *testing.T) {
	codes := func(rs []minigame.SourceReason) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.Code)
		}
		return out
	}
	if rs := minigame.Check("matching", minigame.SourceCheck{Types: []string{"matching"}, Available: 6, QuestionCount: 6}); len(rs) != 0 {
		t.Fatalf("compatible: %v", rs)
	}
	if got := codes(minigame.Check("matching", minigame.SourceCheck{Types: []string{"matching", "single_choice"}, Available: 6, QuestionCount: 6})); !slices.Equal(got, []string{minigame.ReasonIncompatibleType}) {
		t.Fatalf("incompatible: %v", got)
	}
	if got := codes(minigame.Check("matching", minigame.SourceCheck{Unbounded: true, Available: 9, QuestionCount: 6})); !slices.Equal(got, []string{minigame.ReasonUnboundedTypes}) {
		t.Fatalf("unbounded: %v", got)
	}
	if rs := minigame.Check("puzzle", minigame.SourceCheck{Unbounded: true, Available: 9, QuestionCount: 6}); len(rs) != 0 {
		t.Fatalf("puzzle unbounded: %v", rs)
	}
	if got := codes(minigame.Check("true_false", minigame.SourceCheck{Types: []string{"true_false"}, Available: 2, QuestionCount: 6})); !slices.Equal(got, []string{minigame.ReasonInsufficient}) {
		t.Fatalf("insufficient: %v", got)
	}
	if got := codes(minigame.Check("true_false", minigame.SourceCheck{SetNotFound: true, QuestionCount: 6})); !slices.Equal(got, []string{minigame.ReasonSetNotPublished}) {
		t.Fatalf("not published: %v", got)
	}
}
