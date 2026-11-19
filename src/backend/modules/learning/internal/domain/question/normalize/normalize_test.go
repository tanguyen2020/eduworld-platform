package normalize

import (
	"encoding/csv"
	"os"
	"testing"
)

// F022-AC-17, SC-002: the Vietnamese/English normalization pairs of testdata/normalize_vi_pairs.csv (≥ 200 pairs,
// written by the backend team and marked for review by the EdTech Specialist — spec Assumptions).
func TestNormalizationPairs(t *testing.T) {
	f, err := os.Open("testdata/normalize_vi_pairs.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows)-1 < 200 {
		t.Fatalf("%d pairs, want ≥ 200", len(rows)-1)
	}
	for i, r := range rows[1:] {
		m := Match{Accepted: []string{r[1]}, Mode: r[3], AllowDiacriticsMissing: r[2] == "true"}
		if got := Equal(r[0], m, r[4]); got != (r[5] == "true") {
			t.Errorf("row %d: %q vs %q (%s, %s, allow=%s) = %v, want %s", i+2, r[0], r[1], r[3], r[4], r[2], got, r[5])
		}
	}
}

func TestNormalizationRules(t *testing.T) {
	if Text("  Hoà   Bình. ", "vi", false) != Text("hòa bình", "vi", false) {
		t.Fatal("tone placement + whitespace + punctuation")
	}
	if StripDiacritics("Đường Hồ Chí Minh") != "Duong Ho Chi Minh" {
		t.Fatal(StripDiacritics("Đường Hồ Chí Minh"))
	}
	if Equal("ha noi", Match{Accepted: []string{"Hà Nội"}}, "vi") || !Equal("ha noi", Match{Accepted: []string{"Hà Nội"}, AllowDiacriticsMissing: true}, "vi") {
		t.Fatal("allow_diacritics_missing")
	}
	if Equal("HÀ NỘI", Match{Accepted: []string{"Hà Nội"}, CaseSensitive: true}, "vi") {
		t.Fatal("case_sensitive")
	}
	fz := &Fuzzy{Enabled: true, MaxEditDistance: 1, MinLength: 5}
	if !Equal("elephnt", Match{Accepted: []string{"elephant"}, Fuzzy: fz}, "en") || Equal("cat", Match{Accepted: []string{"car"}, Fuzzy: fz}, "en") {
		t.Fatal("fuzzy")
	}
	if !Equal("5", Match{Accepted: []string{"năm", "5"}}, "vi") || Equal("5", Match{Accepted: []string{"năm"}}, "vi") {
		t.Fatal("numbers only when listed (step 8)")
	}
	if levenshtein("kitten", "sitting") != 3 {
		t.Fatal("levenshtein")
	}
}
