package curriculum

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// ErrNotEditable: the content of a version changes only while it is draft (F021-AC-07, 409 CONTENT_NOT_EDITABLE).
var ErrNotEditable = errors.New("curriculum: the version is not a draft")

// ErrOpenDraft: a curriculum has at most one draft/in_review version (F021-AC-17, 409 CONFLICT).
var ErrOpenDraft = errors.New("curriculum: the curriculum already has an open draft or in-review version")

// EnsureDraft returns ErrNotEditable unless status is draft.
func EnsureDraft(s contentmodel.Status) error {
	if s != contentmodel.StatusDraft {
		return ErrNotEditable
	}
	return nil
}

// Open reports a version that blocks the creation of another one (data-model §3: one draft/in_review per curriculum).
func Open(s contentmodel.Status) bool {
	return s == contentmodel.StatusDraft || s == contentmodel.StatusInReview
}

// CheckEffective validates the effective range of a version.
func CheckEffective(from, to *time.Time) error {
	if from != nil && to != nil && to.Before(*from) {
		return contentmodel.ErrInvalid{Field: "effective_to", Reason: "is before effective_from"}
	}
	return nil
}

// Range is the effective range of a version (nil bound = open).
type Range struct {
	VersionID uuid.UUID
	From, To  *time.Time
}

// Overlaps reports whether two ranges share at least one day.
func (r Range) Overlaps(o Range) bool {
	if r.To != nil && o.From != nil && r.To.Before(*o.From) {
		return false
	}
	if o.To != nil && r.From != nil && o.To.Before(*r.From) {
		return false
	}
	return true
}

// Contains reports whether day is inside the range.
func (r Range) Contains(day time.Time) bool {
	if r.From != nil && day.Before(*r.From) {
		return false
	}
	return r.To == nil || !day.After(*r.To)
}

// Warning is a non-blocking finding of a transition (CurriculumVersion.warnings).
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// WarnEffectiveRangeOverlap is the code of the overlap warning given at submit_review (spec edge case).
const WarnEffectiveRangeOverlap = "EFFECTIVE_RANGE_OVERLAP"

// OverlapWarnings warns when the range of v overlaps another published or open version of the same
// curriculum that shares a subject × grade (sharing is decided by the caller, which passes only those).
func OverlapWarnings(v Range, others []Range) []Warning {
	var out []Warning
	for _, o := range others {
		if o.VersionID != v.VersionID && v.Overlaps(o) {
			out = append(out, Warning{Code: WarnEffectiveRangeOverlap,
				Message: "the effective range overlaps version " + o.VersionID.String() + " for a shared subject × grade"})
		}
	}
	return out
}

// GDPT2018 is the code of the official curriculum: its versions are published only when every LO has a
// yccd_ref (CURRICULUM_MODEL §3.3, F021-AC-14). GDPT2018_SAMPLE (seed) is exempt.
const GDPT2018 = "GDPT2018"

// ObjectiveRef is what the publish policy needs of an LO.
type ObjectiveRef struct {
	ID      uuid.UUID
	Code    string
	YccdRef string
}

// MissingYccd returns the codes of the LOs without yccd_ref when the curriculum is GDPT2018 (sorted), else nil.
func MissingYccd(curriculumCode string, los []ObjectiveRef) []string {
	if curriculumCode != GDPT2018 {
		return nil
	}
	var out []string
	for _, lo := range los {
		if strings.TrimSpace(lo.YccdRef) == "" {
			out = append(out, lo.Code)
		}
	}
	slices.Sort(out)
	return out
}
