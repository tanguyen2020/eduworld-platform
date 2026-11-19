package curriculum

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Bloom levels (CURRICULUM_MODEL §3.3).
var bloomLevels = []string{"remember", "understand", "apply", "analyze", "evaluate", "create"}

// ValidBloom reports a known Bloom level.
func ValidBloom(s string) bool { return slices.Contains(bloomLevels, s) }

// Objective is the data of a learning objective checked before it is saved.
type Objective struct {
	ID              uuid.UUID
	Code            string
	Description     string
	BloomLevel      string // empty = not set
	YccdRef         string // empty = not set
	CompetencyIDs   []uuid.UUID
	PrerequisiteIDs []uuid.UUID
	TopicGrade      int
}

// Validate checks the fields of the objective (F021-AC-03, AC-04, AC-11 except the version membership of the
// referenced ids, which the use case checks against the version): every invalid field is listed.
func (o Objective) Validate() error {
	var v contentmodel.Violations
	add := func(field, reason string) { v = append(v, contentmodel.Violation{Field: field, Reason: reason}) }
	var ce contentmodel.ErrInvalid
	if err := CheckLoCodeForGrade(o.Code, o.TopicGrade); errors.As(err, &ce) {
		add("code", ce.Reason)
	}
	if strings.TrimSpace(o.Description) == "" {
		v = append(v, contentmodel.Violation{Field: "description", Required: true, Reason: "is required"})
	} else if utf8.RuneCountInString(o.Description) > MaxDescriptionLen {
		add("description", "is longer than 1000 characters")
	}
	if o.BloomLevel != "" && !ValidBloom(o.BloomLevel) {
		add("bloom_level", "is not a Bloom level")
	}
	if utf8.RuneCountInString(o.YccdRef) > MaxYccdRefLen {
		add("yccd_ref", "is longer than 64 characters")
	}
	if len(o.CompetencyIDs) > MaxCompetencies || hasDuplicates(o.CompetencyIDs) {
		add("competency_ids", "must hold at most 20 distinct ids")
	}
	switch {
	case len(o.PrerequisiteIDs) > MaxPrerequisites || hasDuplicates(o.PrerequisiteIDs):
		add("prerequisite_ids", "must hold at most 20 distinct ids")
	case o.ID != uuid.Nil && slices.Contains(o.PrerequisiteIDs, o.ID):
		add("prerequisite_ids", "an objective cannot be its own prerequisite")
	}
	if len(v) == 0 {
		return nil
	}
	return v
}

func hasDuplicates(ids []uuid.UUID) bool {
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			return true
		}
		seen[id] = true
	}
	return false
}

// Competency kinds (CURRICULUM_MODEL §3.4).
const (
	KindGeneralCompetency = "general_competency"
	KindSubjectCompetency = "subject_competency"
	KindQuality           = "quality"
)

// CheckCompetencyKind validates kind and its subject: subject_id is set exactly for subject_competency.
func CheckCompetencyKind(kind string, hasSubject bool) error {
	switch kind {
	case KindGeneralCompetency, KindQuality:
		if hasSubject {
			return invalid("subject_id", "is only for subject_competency")
		}
	case KindSubjectCompetency:
		if !hasSubject {
			return contentmodel.ErrInvalid{Field: "subject_id", Reason: "is required for subject_competency"}
		}
	default:
		return invalid("kind", "must be general_competency, subject_competency or quality")
	}
	return nil
}
