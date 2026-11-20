package lms

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// F040-AC-09 (T016): free opens all, sequential follows completion, mastery_gate follows the level of the previous
// node's objectives, a manual unlock always opens, completed stays completed.
func TestNodeStates(t *testing.T) {
	lo1, lo2 := uuid.New(), uuid.New()
	n1 := NodeProgress{NodeID: uuid.New(), Position: 1, ObjectiveIDs: []uuid.UUID{lo1, lo2}}
	n2 := NodeProgress{NodeID: uuid.New(), Position: 2}
	n3 := NodeProgress{NodeID: uuid.New(), Position: 3}
	levels := map[uuid.UUID]string{lo1: "developing", lo2: "beginning"}
	levelOf := func(id uuid.UUID) string { return levels[id] }
	nodes := []NodeProgress{n3, n1, n2} // any order
	if got := NodeStates(ModeFree, nodes, levelOf); !slices.Equal(got, []string{NodeUnlocked, NodeUnlocked, NodeUnlocked}) {
		t.Fatalf("free: %v", got)
	}
	if got := NodeStates(ModeMasteryGate, nodes, levelOf); !slices.Equal(got, []string{NodeUnlocked, NodeUnlocked, NodeLocked}) {
		t.Fatalf("gate with a beginning objective: %v", got) // n3 open because n2 has no objective
	}
	nodes[2].Unlocked = true // n2 manual
	levels[lo2] = "proficient"
	if got := NodeStates(ModeMasteryGate, nodes, levelOf); got[2] != NodeUnlocked {
		t.Fatalf("gate passed: %v", got)
	}
	seq := []NodeProgress{n1, n2, n3}
	if got := NodeStates(ModeSequential, seq, levelOf); !slices.Equal(got, []string{NodeUnlocked, NodeLocked, NodeLocked}) {
		t.Fatalf("sequential: %v", got)
	}
	seq[0].Completed = true
	if got := NodeStates(ModeSequential, seq, levelOf); !slices.Equal(got, []string{NodeCompleted, NodeUnlocked, NodeLocked}) {
		t.Fatalf("sequential after completion: %v", got)
	}
	seq[2].Unlocked = true
	if got := NodeStates(ModeSequential, seq, levelOf); got[2] != NodeUnlocked {
		t.Fatalf("manual unlock: %v", got)
	}
}

// F040-AC-19, AC-22 (T029): diff of the members, bounds of the name and of the members.
func TestStudyGroupRules(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	added, removed := Diff([]uuid.UUID{a, b}, []uuid.UUID{b, c})
	if !slices.Equal(added, []uuid.UUID{c}) || !slices.Equal(removed, []uuid.UUID{a}) {
		t.Fatalf("diff: %v %v", added, removed)
	}
	if vs := ValidateGroup("Ôn chia có dư", []uuid.UUID{a}); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	many := make([]uuid.UUID, 101)
	for i := range many {
		many[i] = uuid.New()
	}
	for _, c := range []struct {
		name    string
		members []uuid.UUID
		field   string
	}{{"", []uuid.UUID{a}, "/name"}, {strings.Repeat("x", 65), []uuid.UUID{a}, "/name"}, {"g", nil, "/member_ids"},
		{"g", many, "/member_ids"}, {"g", []uuid.UUID{a, a}, "/member_ids"}} {
		if vs := ValidateGroup(c.name, c.members); len(vs) != 1 || vs[0].Field != c.field {
			t.Fatalf("%q/%d: %v", c.name, len(c.members), vs)
		}
	}
}

// F040-AC-05, AC-06 (T011, T019): status changes of a course; path input checks.
func TestCourseAndPathRules(t *testing.T) {
	if SetCourseStatus(CourseActive, CourseHidden) != nil || SetCourseStatus(CourseHidden, CourseActive) != nil {
		t.Fatal("toggle")
	}
	if SetCourseStatus(CourseArchived, CourseActive) == nil || SetCourseStatus(CourseActive, CourseArchived) == nil {
		t.Fatal("archived is the system's")
	}
	if vs := ValidatePath("Lộ trình", ModeMasteryGate, []NodeInput{{1, "lesson", uuid.New()}, {2, "activity", uuid.New()}}); len(vs) != 0 {
		t.Fatalf("valid path: %v", vs)
	}
	if vs := ValidatePath(" ", "x", []NodeInput{{1, "lesson", uuid.New()}, {1, "video", uuid.New()}}); len(vs) != 4 {
		t.Fatalf("invalid path: %v", vs)
	}
	ten := uuid.New()
	y1, y2, other := AcademicYearID(ten, "2026-2027"), AcademicYearID(ten, "2026-2027"), AcademicYearID(uuid.New(), "2026-2027")
	if y1 != y2 || y1 == other {
		t.Fatal("academic year id")
	}
}
