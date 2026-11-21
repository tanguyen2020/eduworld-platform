package world

import (
	"slices"

	"github.com/google/uuid"
)

// Student is what the access rules read of a student: grade and the subjects the school turned on.
type Student struct {
	Grade int
	// EnabledSubjects nil = every subject is on (school settings without enabled_subject_codes).
	EnabledSubjects []string
}

// SubjectEnabled reports a subject the school did not turn off (WORLD_DESIGN §7.1).
func (s Student) SubjectEnabled(subject string) bool {
	return subject == "" || s.EnabledSubjects == nil || slices.Contains(s.EnabledSubjects, subject)
}

// Access is the decision on one area for one student.
type Access struct {
	Visible      bool
	Accessible   bool
	LockReason   string
	Requirements []Requirement // every leaf of the rule (UnlockRequirement), only for a rule-gated area
	RuleMet      bool          // the rule is met now (an unlock row may still be missing: the caller records it)
}

// Decide applies the access rules of WORLD_DESIGN §7 (F071-AC-02, AC-03, AC-08): an area of a subject the school
// turned off is hidden; a grade outside access_policy.grade_levels is locked (grade_not_allowed); a learning area of
// the student's grade and subject is always open whatever the level, coins or story; any other area is open when it
// has no rule, when it was unlocked, or when its rule is met now.
func Decide(a Area, s Student, unlocked bool, f Facts) Access {
	if a.IsLearning() && !s.SubjectEnabled(a.Subject()) {
		return Access{}
	}
	out := Access{Visible: true}
	if !a.AdmitsGrade(s.Grade) {
		out.LockReason = LockGradeNotAllowed
		return out
	}
	if a.IsLearning() || a.UnlockRule == nil {
		out.Accessible = true
		return out
	}
	out.RuleMet, out.Requirements = a.UnlockRule.Evaluate(f)
	out.Accessible = unlocked || out.RuleMet
	if !out.Accessible {
		out.LockReason = LockRequirementsNotMet
	}
	return out
}

// instanceNS is the namespace of the v0 instance partition ids.
var instanceNS = uuid.MustParse("6f1d7c2a-0710-5b7e-9c1e-0000000f0710")

// InstanceKey is the partition of a public area (MULTIPLAYER_GAMEPLAY §6, AC-WLD-003): tenant, school, area and
// grade band — two students of different schools never share it.
type InstanceKey struct {
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	AreaID    string
	GradeBand string
}

// RoomID is the stable id of the partition room (allocator v0 — ALLOCATOR TẠM: the hub instance itself is assigned
// by the room host of F091 through hub.instance.join, which applies the same partition).
func (k InstanceKey) RoomID() uuid.UUID {
	return uuid.NewSHA1(instanceNS, []byte(k.TenantID.String()+"|"+k.SchoolID.String()+"|"+k.AreaID+"|"+k.GradeBand))
}

// contentNS names the string-keyed content items (world_area, story_chapter) as UUIDs in the content manifest and the
// pinned versions of a level session (ContentManifestItem.item_id is a UUID).
var contentNS = uuid.MustParse("6f1d7c2a-0710-5b7e-9c1e-00000c071000")

// ContentItemID is the UUIDv5 of a string-keyed content item ("<item_type>:<code>").
func ContentItemID(itemType, code string) uuid.UUID {
	return uuid.NewSHA1(contentNS, []byte(itemType+":"+code))
}
