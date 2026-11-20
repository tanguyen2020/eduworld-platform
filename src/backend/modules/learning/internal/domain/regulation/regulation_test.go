package regulation

import (
	"testing"
	"time"
)

// counts builds the counts of n objectives in scope from the levels of the covered ones (≥ 3 evidence each).
func counts(n int, levels map[string]int) Counts {
	c := Counts{InScope: n}
	for lv, k := range levels {
		c.Covered += k
		switch lv {
		case "mastered":
			c.Mastered += k
			c.AtLeastProficient += k
			c.AtLeastDeveloping += k
		case "proficient":
			c.AtLeastProficient += k
			c.AtLeastDeveloping += k
		case "developing":
			c.AtLeastDeveloping += k
		}
	}
	return c
}

// TestPrimaryTT27 checks F044-AC-11 (AC-ARP-001, AC-ARP-002, TC-LRN-018): 85 % ≥ proficient, 45 % mastered, coverage 90 %
// → excellent ("Hoàn thành tốt"); coverage 50 % → insufficient_data; ≥ 60 % developing → completed; otherwise not_completed.
func TestPrimaryTT27(t *testing.T) {
	r := DefaultRules()
	// 20 objectives: 9 mastered, 8 proficient (17 = 85 %), 1 beginning; 18 covered (90 %).
	c := counts(20, map[string]int{"mastered": 9, "proficient": 8, "beginning": 1})
	if got := Map(ProfilePrimaryTT27, r, c); got.Level != LevelExcellent || got.Coverage != 0.9 || got.ProficientRatio != 0.85 || got.MasteredRatio != 0.45 {
		t.Fatalf("Map = %+v", got)
	}
	if got := Map(ProfilePrimaryTT27, r, counts(20, map[string]int{"mastered": 10})); got.Level != LevelInsufficientData {
		t.Fatalf("coverage 50 %% = %+v", got)
	}
	if got := Map(ProfilePrimaryTT27, r, counts(10, map[string]int{"developing": 6, "beginning": 2})); got.Level != LevelCompleted {
		t.Fatalf("completed = %+v", got)
	}
	if got := Map(ProfilePrimaryTT27, r, counts(10, map[string]int{"developing": 5, "beginning": 3})); got.Level != LevelNotCompleted {
		t.Fatalf("not completed = %+v", got)
	}
	if got := Map(ProfilePrimaryTT27, r, Counts{}); got.Level != LevelInsufficientData {
		t.Fatalf("empty scope = %+v", got)
	}
}

// TestLowerSecondaryTT22: ≥ 60 % developing → pass, else not_pass; coverage < 70 % → insufficient_data.
func TestLowerSecondaryTT22(t *testing.T) {
	r := DefaultRules()
	if got := Map(ProfileLowerSecondaryTT22, r, counts(10, map[string]int{"developing": 6, "beginning": 2})); got.Level != LevelPass {
		t.Fatalf("pass = %+v", got)
	}
	if got := Map(ProfileLowerSecondaryTT22, r, counts(10, map[string]int{"developing": 5, "beginning": 3})); got.Level != LevelNotPass {
		t.Fatalf("not pass = %+v", got)
	}
	if got := Map(ProfileLowerSecondaryTT22, r, counts(10, map[string]int{"developing": 6})); got.Level != LevelInsufficientData {
		t.Fatalf("coverage 60 %% = %+v", got)
	}
}

// TestProfileForGradeAndPeriods: grades 1–5 use TT27, 6–9 TT22; the default calendar and the curriculum share of a period.
func TestProfileForGradeAndPeriods(t *testing.T) {
	if ProfileFor(4) != ProfilePrimaryTT27 || ProfileFor(7) != ProfileLowerSecondaryTT22 || ProfileFor(10) != "" {
		t.Fatal("ProfileFor")
	}
	for _, c := range []struct {
		at   time.Time
		want string
		year int
	}{{date(2026, 10, 1), PeriodMidTerm1, 2026}, {date(2026, 12, 20), PeriodEndTerm1, 2026}, {date(2027, 2, 1), PeriodMidTerm2, 2026},
		{date(2027, 5, 1), PeriodEndTerm2, 2026}, {date(2027, 7, 1), PeriodEndTerm2, 2026}, {date(2027, 8, 20), PeriodEndTerm2, 2026}} {
		p, y := CurrentPeriod(c.at)
		if p != c.want || y != c.year {
			t.Errorf("CurrentPeriod(%v) = %s %d, want %s %d", c.at, p, y, c.want, c.year)
		}
	}
	from, to := PeriodRange(2026, PeriodEndTerm1)
	if !from.Equal(date(2026, 9, 1)) || !to.Equal(date(2027, 1, 15)) {
		t.Fatalf("range = %v %v", from, to)
	}
	if ScopeShare(PeriodMidTerm1) != 0.25 || ScopeShare(PeriodEndTerm2) != 1 || !ValidPeriod(PeriodMidTerm2) || ValidPeriod("x") {
		t.Fatal("ScopeShare/ValidPeriod")
	}
	if DefaultDisclaimer()["vi"] == "" || len([]rune(DefaultDisclaimer()["vi"])) > 500 {
		t.Fatal("disclaimer")
	}
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
