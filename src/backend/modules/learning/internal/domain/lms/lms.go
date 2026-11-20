// Package lms is the domain of the school LMS of module learning (SCHOOL_LMS §2–§4, F040): courses provisioned per class
// and subject, learning paths and the lock state of their nodes, schedule items and study groups (ADR-0084). No I/O.
package lms

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Course statuses (SCHOOL_LMS §3).
const (
	CourseActive   = "active"
	CourseHidden   = "hidden"
	CourseArchived = "archived"
)

// ErrCourseStatus: a school only toggles active/hidden; archived is set by the system (end of year, closed class).
var ErrCourseStatus = errors.New("lms: a course can only be shown or hidden")

// SetCourseStatus checks a status change asked by the school.
func SetCourseStatus(from, to string) error {
	if (to != CourseActive && to != CourseHidden) || from == CourseArchived {
		return ErrCourseStatus
	}
	return nil
}

// AcademicYearID is the id of an academic year of a tenant when the class projection has no academic_year_id yet:
// a name-based UUID of the tenant and the label "YYYY-YYYY" of class.synced, stable across deliveries [Proposed].
func AcademicYearID(tenant uuid.UUID, label string) uuid.UUID {
	return uuid.NewSHA1(tenant, []byte("academic_year:"+label))
}

// ---------- learning paths

// Unlock modes of a path.
const (
	ModeSequential  = "sequential"
	ModeFree        = "free"
	ModeMasteryGate = "mastery_gate"
)

// Node states seen by a student.
const (
	NodeLocked    = "locked"
	NodeUnlocked  = "unlocked"
	NodeCompleted = "completed"
)

// Content types a node may reference (PathNode.content_ref.content_type).
var NodeContentTypes = []string{"lesson", "activity", "question_set", "assessment_blueprint", "quest"}

// MaxNodes bounds the nodes of a path; MaxTitle the title.
const (
	MaxNodes = 200
	MaxTitle = 255
)

// Violation is one invalid field of an input (JSON pointer and reason).
type Violation struct {
	Field  string
	Reason string
}

// NodeInput is a node of LearningPathInput.
type NodeInput struct {
	Position         int
	ContentType      string
	ContentVersionID uuid.UUID
}

// ValidatePath checks a path input the schema cannot (F040-AC-06): a mode, a title, 1–200 nodes with distinct positions.
func ValidatePath(title, mode string, nodes []NodeInput) []Violation {
	var vs []Violation
	if t := strings.TrimSpace(title); t == "" || utf8.RuneCountInString(t) > MaxTitle {
		vs = append(vs, Violation{"/title", "1 to 255 characters"})
	}
	if !slices.Contains([]string{ModeSequential, ModeFree, ModeMasteryGate}, mode) {
		vs = append(vs, Violation{"/unlock_mode", "unknown unlock mode"})
	}
	if len(nodes) == 0 || len(nodes) > MaxNodes {
		vs = append(vs, Violation{"/nodes", "1 to 200 nodes"})
	}
	seen := map[int]bool{}
	for i, n := range nodes {
		if n.Position < 1 || seen[n.Position] {
			vs = append(vs, Violation{"/nodes/" + itoa(i) + "/position", "positions are distinct and start at 1"})
		}
		seen[n.Position] = true
		if !slices.Contains(NodeContentTypes, n.ContentType) {
			vs = append(vs, Violation{"/nodes/" + itoa(i) + "/content_ref/content_type", "unknown content type"})
		}
	}
	return vs
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// NodeProgress is what the state of a node of a student depends on.
type NodeProgress struct {
	NodeID       uuid.UUID
	Position     int
	ObjectiveIDs []uuid.UUID
	Completed    bool // the student completed the content (graded attempt, completed lesson)
	Unlocked     bool // manual unlock by the school (path_node_unlocks)
}

// levelRank orders the mastery levels.
var levelRank = map[string]int{"not_started": 0, "beginning": 1, "developing": 2, "proficient": 3, "mastered": 4}

// GateLevel is the level every objective of the previous node must reach in mastery_gate mode (SCHOOL_LMS §3).
const GateLevel = "developing"

// NodeStates computes the state of each node (F040-AC-09), nodes in position order: free opens every node; sequential
// opens a node when the previous one is completed; mastery_gate when every objective of the previous node is at least
// developing; a manual unlock always opens; a completed node is completed. levelOf is the level after decay.
func NodeStates(mode string, nodes []NodeProgress, levelOf func(uuid.UUID) string) []string {
	ordered := slices.Clone(nodes)
	slices.SortFunc(ordered, func(a, b NodeProgress) int { return a.Position - b.Position })
	byID := map[uuid.UUID]string{}
	for i, n := range ordered {
		var st string
		switch {
		case n.Completed:
			st = NodeCompleted
		case i == 0 || mode == ModeFree || n.Unlocked:
			st = NodeUnlocked
		case mode == ModeSequential:
			st = lockedUnless(ordered[i-1].Completed)
		default:
			st = lockedUnless(gatePassed(ordered[i-1].ObjectiveIDs, levelOf))
		}
		byID[n.NodeID] = st
	}
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = byID[n.NodeID]
	}
	return out
}

func lockedUnless(open bool) string {
	if open {
		return NodeUnlocked
	}
	return NodeLocked
}

func gatePassed(objectives []uuid.UUID, levelOf func(uuid.UUID) string) bool {
	for _, o := range objectives {
		if levelRank[levelOf(o)] < levelRank[GateLevel] {
			return false
		}
	}
	return true
}

// ---------- study groups

// Study group bounds (ADR-0084, F040-AC-22).
const (
	MaxGroupName    = 64
	MaxGroupMembers = 100
)

// ValidateGroup checks a study group input (F040-AC-22): a name of 1–64 characters, 1–100 distinct members.
func ValidateGroup(name string, members []uuid.UUID) []Violation {
	var vs []Violation
	if n := strings.TrimSpace(name); n == "" || utf8.RuneCountInString(n) > MaxGroupName {
		vs = append(vs, Violation{"/name", "1 to 64 characters"})
	}
	if len(members) == 0 || len(members) > MaxGroupMembers {
		vs = append(vs, Violation{"/member_ids", "1 to 100 members"})
	}
	seen := map[uuid.UUID]bool{}
	for _, m := range members {
		if seen[m] {
			vs = append(vs, Violation{"/member_ids", "members are distinct"})
			break
		}
		seen[m] = true
	}
	return vs
}

// Diff returns the members added and removed when old is replaced by next (F040-AC-19), each in the order of its list.
func Diff(old, next []uuid.UUID) (added, removed []uuid.UUID) {
	added, removed = []uuid.UUID{}, []uuid.UUID{}
	for _, m := range next {
		if !slices.Contains(old, m) {
			added = append(added, m)
		}
	}
	for _, m := range old {
		if !slices.Contains(next, m) {
			removed = append(removed, m)
		}
	}
	return added, removed
}

// ---------- schedule

// Schedule item kinds.
const (
	KindLesson        = "lesson"
	KindAssignmentDue = "assignment_due"
	KindExam          = "exam"
	KindEvent         = "event"
)

// MaxScheduleDays bounds the range of listMySchedule.
const MaxScheduleDays = 62
