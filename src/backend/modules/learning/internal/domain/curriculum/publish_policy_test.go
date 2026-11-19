package curriculum

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// F021-AC-14: GDPT2018 is published only when every LO has yccd_ref; GDPT2018_SAMPLE and others are exempt.
func TestMissingYccd(t *testing.T) {
	los := []ObjectiveRef{{ID: uuid.New(), Code: "MATH.G3.NUM.B"}, {ID: uuid.New(), Code: "MATH.G3.NUM.A", YccdRef: "  "},
		{ID: uuid.New(), Code: "MATH.G3.NUM.C", YccdRef: "TOAN3.1"}}
	if got := MissingYccd(GDPT2018, los); !slices.Equal(got, []string{"MATH.G3.NUM.A", "MATH.G3.NUM.B"}) {
		t.Fatalf("missing %v", got)
	}
	if got := MissingYccd("GDPT2018_SAMPLE", los); got != nil {
		t.Fatalf("sample curriculum must be exempt: %v", got)
	}
}

// F021-AC-26: every LO of the superseded version needs a mapping; removed has no target.
func TestMappingCoverage(t *testing.T) {
	o := ids(3)
	nw := ids(2)
	old := map[uuid.UUID]bool{o[0]: true, o[1]: true, o[2]: true}
	cur := map[uuid.UUID]bool{nw[0]: true, nw[1]: true}
	set := MappingSet{{FromLO: o[0], ToLO: &nw[0], Relation: RelationSame}, {FromLO: o[1], ToLO: &nw[0], Relation: RelationSplit},
		{FromLO: o[1], ToLO: &nw[1], Relation: RelationSplit}}
	if err := set.Validate(old, cur); err != nil {
		t.Fatal(err)
	}
	if got := set.Uncovered(o); len(got) != 1 || got[0] != o[2] {
		t.Fatalf("uncovered %v", got)
	}
	set = append(set, Mapping{FromLO: o[2], Relation: RelationRemoved})
	if err := set.Validate(old, cur); err != nil || len(set.Uncovered(o)) != 0 {
		t.Fatalf("removed mapping: %v", err)
	}
	for _, bad := range []MappingSet{
		{{FromLO: o[0], Relation: RelationSame}},                                                                     // to missing
		{{FromLO: o[0], ToLO: &nw[0], Relation: RelationRemoved}},                                                    // removed with target
		{{FromLO: o[0], ToLO: &nw[0], Relation: "moved"}},                                                            // unknown relation
		{{FromLO: nw[0], ToLO: &nw[1], Relation: RelationSame}},                                                      // from not in old version
		{{FromLO: o[0], ToLO: &o[1], Relation: RelationSame}},                                                        // to not in new version
		{{FromLO: o[0], ToLO: &nw[0], Relation: RelationSame}, {FromLO: o[0], ToLO: &nw[0], Relation: RelationSame}}, // duplicate
	} {
		if bad.Validate(old, cur) == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	same := SameByStableKey(map[string]uuid.UUID{"A": o[0], "B": o[1]}, map[string]uuid.UUID{"A": nw[0]})
	if len(same) != 1 || same[0].FromLO != o[0] || *same[0].ToLO != nw[0] {
		t.Fatalf("same by stable key %+v", same)
	}
}

func TestVersionRules(t *testing.T) {
	if EnsureDraft(contentmodel.StatusDraft) != nil || EnsureDraft(contentmodel.StatusInReview) == nil || EnsureDraft(contentmodel.StatusPublished) == nil {
		t.Fatal("only draft is editable")
	}
	if !Open(contentmodel.StatusDraft) || !Open(contentmodel.StatusInReview) || Open(contentmodel.StatusApproved) {
		t.Fatal("open versions")
	}
	day := func(s string) *time.Time { d, _ := time.Parse(time.DateOnly, s); return &d }
	if CheckEffective(day("2025-09-01"), day("2025-08-31")) == nil || CheckEffective(day("2025-09-01"), nil) != nil {
		t.Fatal("effective range")
	}
	v := Range{VersionID: uuid.New(), From: day("2025-09-01"), To: day("2026-05-31")}
	if w := OverlapWarnings(v, []Range{{VersionID: uuid.New(), From: day("2026-05-01")}}); len(w) != 1 || w[0].Code != WarnEffectiveRangeOverlap {
		t.Fatalf("overlap: %v", w)
	}
	if w := OverlapWarnings(v, []Range{{VersionID: uuid.New(), From: day("2026-06-01")}, v}); len(w) != 0 {
		t.Fatalf("no overlap: %v", w)
	}
	if !v.Contains(*day("2025-12-01")) || v.Contains(*day("2026-06-01")) || !(Range{}).Contains(*day("2000-01-01")) {
		t.Fatal("contains")
	}
}
