package curriculum

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// F021-AC-03: the LO code follows <SUBJECT>.G<grade>.<STRAND>.<SKILL>[.<n>] and its grade is the topic's grade.
func TestLoCode(t *testing.T) {
	valid := []string{"MATH.G3.NUM.MUL_TABLE.2", "MATH.G1.NUM.COUNT_10", "VIET.G5.READ.MAIN_IDEA", "ENG.G9.STAT.X", "SCI.G4.GEO.A1_B.12"}
	for _, c := range valid {
		if _, err := ParseLoCode(c); err != nil {
			t.Errorf("%s refused: %v", c, err)
		}
	}
	invalidCodes := []string{"", "math-g3-x", "MATH.G0.NUM.X", "MATH.G10.NUM.X", "M.G3.NUM.X", "MATH.G3.NUMBERS.X", "MATH.G3.NUM.mul",
		"MATH.G3.NUM.X.123", "MATH.G3.NUM", "MATH.G3.NUM.1X", "MATHEMATIC.G3.NUM.X", "MATH.G3.NUM.X.", "MATH.G3.NUM.X Y"}
	for _, c := range invalidCodes {
		var e contentmodel.ErrInvalid
		if _, err := ParseLoCode(c); !errors.As(err, &e) || e.Field != "code" {
			t.Errorf("%q accepted or wrong error: %v", c, err)
		}
	}
	c, _ := ParseLoCode("MATH.G3.NUM.MUL_TABLE.2")
	if c.Subject != "MATH" || c.Grade != 3 || c.Strand != "NUM" || c.Skill != "MUL_TABLE" || c.Detail != ".2" {
		t.Fatalf("parsed %+v", c)
	}
	if CheckLoCodeForGrade("MATH.G3.NUM.MUL_TABLE.2", 3) != nil || CheckLoCodeForGrade("MATH.G3.NUM.MUL_TABLE.2", 4) == nil {
		t.Fatal("G<grade> must match the topic grade")
	}
}

func fields(err error) []string {
	var v contentmodel.Violations
	if !errors.As(err, &v) {
		return nil
	}
	out := make([]string, len(v))
	for i, x := range v {
		out[i] = x.Field
	}
	return out
}

// F021-AC-04, AC-11: yccd_ref ≤ 64, at most 20 competencies/prerequisites, no self prerequisite.
func TestObjectiveValidate(t *testing.T) {
	id := uuid.New()
	ok := Objective{ID: id, Code: "MATH.G3.NUM.MUL_TABLE.2", Description: "Thuộc bảng nhân 6, 7, 8, 9", BloomLevel: "remember",
		YccdRef: strings.Repeat("y", 64), TopicGrade: 3, PrerequisiteIDs: []uuid.UUID{uuid.New()}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid objective: %v", err)
	}
	bad := ok
	bad.YccdRef = strings.Repeat("y", 65)
	bad.BloomLevel = "memorize"
	bad.Description = ""
	bad.PrerequisiteIDs = []uuid.UUID{id}
	bad.CompetencyIDs = make([]uuid.UUID, 21)
	for i := range bad.CompetencyIDs {
		bad.CompetencyIDs[i] = uuid.New()
	}
	if got := fields(bad.Validate()); !slices.Equal(got, []string{"description", "bloom_level", "yccd_ref", "competency_ids", "prerequisite_ids"}) {
		t.Fatalf("violations %v", got)
	}
	many := ok
	many.PrerequisiteIDs = make([]uuid.UUID, 21)
	for i := range many.PrerequisiteIDs {
		many.PrerequisiteIDs[i] = uuid.New()
	}
	if got := fields(many.Validate()); !slices.Equal(got, []string{"prerequisite_ids"}) {
		t.Fatalf("21 prerequisites: %v", got)
	}
	wrongGrade := ok
	wrongGrade.TopicGrade = 2
	if got := fields(wrongGrade.Validate()); !slices.Equal(got, []string{"code"}) {
		t.Fatalf("grade mismatch: %v", got)
	}
}

// F021-AC-05: the parent topic is in the same version, subject and grade; the tree has no loop.
func TestTopicParent(t *testing.T) {
	v, s := uuid.New(), uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	node := func(id uuid.UUID) TopicNode { return TopicNode{ID: id, VersionID: v, SubjectID: s, GradeLevel: 3} }
	parents := map[uuid.UUID]*uuid.UUID{a: nil, b: &a, c: &b} // a ← b ← c
	if err := CheckParent(TopicNode{VersionID: v, SubjectID: s, GradeLevel: 3}, node(c), parents); err != nil {
		t.Fatalf("new topic under c: %v", err)
	}
	if err := CheckParent(node(a), node(c), parents); err == nil {
		t.Fatal("a under its descendant c must be a loop")
	}
	if err := CheckParent(node(a), node(a), parents); err == nil {
		t.Fatal("self parent")
	}
	other := node(b)
	other.GradeLevel = 4
	if err := CheckParent(node(c), other, parents); err == nil {
		t.Fatal("parent of another grade")
	}
	other = node(b)
	other.VersionID = uuid.New()
	var e contentmodel.ErrInvalid
	if err := CheckParent(node(c), other, parents); !errors.As(err, &e) || e.Field != "parent_topic_id" {
		t.Fatalf("parent of another version: %v", err)
	}
}

func TestCodesAndKinds(t *testing.T) {
	for _, c := range []string{"GDPT2018", "GDPT2018_SAMPLE", "TEST_MATH"} {
		if CheckCurriculumCode("code", c) != nil {
			t.Errorf("%s refused", c)
		}
	}
	for _, c := range []string{"", "g", "gdpt", "1ABC", "A-B"} {
		if CheckCurriculumCode("code", c) == nil {
			t.Errorf("%q accepted", c)
		}
	}
	if CheckSubjectCode("math") != nil || CheckSubjectCode("Math") == nil {
		t.Fatal("subject code")
	}
	if CheckVersionLabel("2018.1") != nil || CheckVersionLabel("1") == nil || CheckVersionLabel("1.0.0") == nil {
		t.Fatal("version label")
	}
	if CheckPageRange("12-15") != nil || CheckPageRange("15-12") == nil || CheckPageRange("x") == nil {
		t.Fatal("page range")
	}
	if CheckCompetencyKind(KindSubjectCompetency, true) != nil || CheckCompetencyKind(KindSubjectCompetency, false) == nil ||
		CheckCompetencyKind(KindQuality, true) == nil || CheckCompetencyKind("skill", false) == nil {
		t.Fatal("competency kind")
	}
	if CheckLearningName("name", map[string]string{"vi": "Toán"}) != nil || CheckLearningName("name", map[string]string{"fr": "x"}) == nil ||
		CheckLearningName("name", map[string]string{}) == nil || CheckLearningName("name", map[string]string{"vi": strings.Repeat("x", 256)}) == nil {
		t.Fatal("learning name")
	}
}
