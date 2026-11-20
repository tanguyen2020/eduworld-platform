// Package efficacy holds the rules of the learning efficacy study of F035 (LEARNING_EFFICACY_PLAN §3–§8, ADR-0080): the
// research agreement of a tenant, the study and its two parallel forms, the classes paired into intervention and
// comparison arms, the validation/pre/post waves, the participants snapshotted at the pre wave, the withdrawals and the
// pseudonymous codes of the exported dataset. Pure Go: thresholds come from the caller (Config), never from settings.
package efficacy

import (
	"errors"
	"regexp"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Study statuses.
const (
	StudyDraft     = "draft"
	StudyActive    = "active"
	StudyCompleted = "completed"
	StudyCancelled = "cancelled"
)

// Arms of a class.
const (
	ArmIntervention = "intervention"
	ArmComparison   = "comparison"
	ArmValidation   = "validation"
)

// Form orders of a class (counterbalancing, LEARNING_EFFICACY_PLAN §3.2).
const (
	OrderAThenB = "a_then_b"
	OrderBThenA = "b_then_a"
)

// Forms.
const (
	FormA = "a"
	FormB = "b"
)

// Assignment methods of an arm.
const (
	MethodManual = "manual"
	MethodRandom = "random"
)

// Wave kinds and statuses.
const (
	WaveValidation = "validation"
	WavePre        = "pre"
	WavePost       = "post"

	WaveScheduled = "scheduled"
	WaveOpen      = "open"
	WaveClosed    = "closed"
	WaveCancelled = "cancelled"
)

// Participant statuses and exclusion reasons.
const (
	ParticipantActive    = "active"
	ParticipantWithdrawn = "withdrawn"
	ParticipantExcluded  = "excluded"

	ReasonWithdrawn            = "withdrawn"
	ReasonAnonymized           = "anonymized"
	ReasonProcessingRestricted = "processing_restricted"
)

// Agreement statuses and coverage.
const (
	AgreementActive  = "active"
	AgreementRevoked = "revoked"

	CoversStudy      = "efficacy_study"
	CoversExperiment = "experiment"
)

// Withdrawal scopes.
const (
	ScopeTenant   = "tenant"
	ScopeSchool   = "school"
	ScopeClass    = "class"
	ScopeStudents = "students"
)

// Limits of the contract (ADR-0041 lengths, data-model §1).
const (
	MaxObjectives       = 100
	MaxPairNo           = 200
	MaxWithdrawStudents = 500
	MaxCodeLen          = 64
	MaxTitleLen         = 255
	MaxRefLen           = 128
	MinReasonLen        = 10
	MaxReasonLen        = 1000
	MaxWeeks            = 52
)

// Config holds the study thresholds (CONFIG_DEFAULTS, owner LEARNING_EFFICACY_PLAN §3.2, §4, §5; analytics_min_cell_size
// and exam_reliability_warn owned by LEARNING_ANALYTICS §5 and EXAM_ENGINE §8.1).
type Config struct {
	MinQuestionsPerObjective int     // efficacy_min_questions_per_lo_per_form (3)
	AttritionWarn            float64 // efficacy_attrition_warn (0.20)
	RecommendedClassesPerArm int     // efficacy_recommended_classes_per_arm (6)
	ValidityRho              float64 // efficacy_validity_rho_min (0.4)
	ValidityCalibration      float64 // efficacy_validity_calibration_min (0.7)
	ValidityAUC              float64 // efficacy_validity_auc_min (0.7)
	MinCellSize              int     // analytics_min_cell_size (5)
	ReliabilityWarn          float64 // exam_reliability_warn (0.7)
}

// DefaultConfig returns the registered defaults.
func DefaultConfig() Config {
	return Config{MinQuestionsPerObjective: 3, AttritionWarn: 0.20, RecommendedClassesPerArm: 6, ValidityRho: 0.4,
		ValidityCalibration: 0.7, ValidityAUC: 0.7, MinCellSize: 5, ReliabilityWarn: 0.7}
}

// Violation is one invalid field (422 VALIDATION_FAILED).
type Violation struct {
	Field, Reason string
}

// ErrState is a transition the state of the study does not allow (422 INVALID_STATE_TRANSITION).
var ErrState = errors.New("efficacy: invalid state transition")

var codeRe = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// StudyInput is the definition of a study (create, or the merged result of an update).
type StudyInput struct {
	Code         string
	Title        string
	SubjectID    uuid.UUID
	GradeLevel   int
	ObjectiveIDs []uuid.UUID
	FormA, FormB uuid.UUID
	MinWeeks     int
}

// Validate checks the fields of a study (F035-AC-07, without the forms: see CheckForm).
func (s StudyInput) Validate() []Violation {
	var vs []Violation
	if !codeRe.MatchString(s.Code) {
		vs = append(vs, Violation{"/code", "must match ^[a-z0-9_-]{1,64}$"})
	}
	if n := utf8.RuneCountInString(s.Title); n == 0 || n > MaxTitleLen {
		vs = append(vs, Violation{"/title", "must have 1 to 255 characters"})
	}
	if s.SubjectID == uuid.Nil {
		vs = append(vs, Violation{"/subject_id", "is required"})
	}
	if s.GradeLevel < 1 || s.GradeLevel > 9 {
		vs = append(vs, Violation{"/grade_level", "must be between 1 and 9"})
	}
	if n := len(s.ObjectiveIDs); n < 1 || n > MaxObjectives {
		vs = append(vs, Violation{"/objective_ids", "must list 1 to 100 objectives"})
	} else if len(unique(s.ObjectiveIDs)) != n {
		vs = append(vs, Violation{"/objective_ids", "must not repeat an objective"})
	}
	if s.FormA == uuid.Nil || s.FormB == uuid.Nil || s.FormA == s.FormB {
		vs = append(vs, Violation{"/form_b_blueprint_version_id", "the two forms are two different blueprint versions"})
	}
	if s.MinWeeks < 1 || s.MinWeeks > MaxWeeks {
		vs = append(vs, Violation{"/min_intervention_weeks", "must be between 1 and 52"})
	}
	return vs
}

// Form is what the study reads of a blueprint version used as a form.
type Form struct {
	Published  bool
	Efficacy   bool
	SubjectID  uuid.UUID
	GradeLevel *int
	// PerObjective counts the questions of the form by objective (fixed questions of every rule).
	PerObjective map[uuid.UUID]int
}

// CheckForm checks one form against the study (F035-AC-07, LEARNING_EFFICACY_PLAN §3.2): a published efficacy
// blueprint of the subject and grade covering exactly the objectives of the study with at least min questions each.
func CheckForm(field string, s StudyInput, f Form, minPerObjective int) []Violation {
	switch {
	case !f.Published:
		return []Violation{{field, "is not a published blueprint version"}}
	case !f.Efficacy:
		return []Violation{{field, "is not an efficacy instrument"}}
	case f.SubjectID != s.SubjectID:
		return []Violation{{field, "is not of the subject of the study"}}
	case f.GradeLevel != nil && *f.GradeLevel != s.GradeLevel:
		return []Violation{{field, "is not of the grade of the study"}}
	}
	want := map[uuid.UUID]bool{}
	for _, o := range s.ObjectiveIDs {
		want[o] = true
		if f.PerObjective[o] < minPerObjective {
			return []Violation{{field, "has fewer than the minimum questions for an objective of the study"}}
		}
	}
	for o, n := range f.PerObjective {
		if n > 0 && !want[o] {
			return []Violation{{field, "covers an objective outside the study"}}
		}
	}
	return nil
}

// Agreement is a research agreement of a tenant.
type Agreement struct {
	Status string
	Covers []string
}

// Covers reports an active agreement covering what (efficacy_study or experiment).
func Covers(as []Agreement, what string) bool {
	for _, a := range as {
		if a.Status == AgreementActive && slices.Contains(a.Covers, what) {
			return true
		}
	}
	return false
}

// ValidCovers checks the coverage of an agreement (1–2 distinct values).
func ValidCovers(covers []string) bool {
	if len(covers) < 1 || len(covers) > 2 {
		return false
	}
	seen := map[string]bool{}
	for _, c := range covers {
		if (c != CoversStudy && c != CoversExperiment) || seen[c] {
			return false
		}
		seen[c] = true
	}
	return true
}

// ValidReason checks a withdrawal or revocation reason (10–1 000 characters).
func ValidReason(r string) bool {
	n := utf8.RuneCountInString(r)
	return n >= MinReasonLen && n <= MaxReasonLen
}

func unique(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	var out []uuid.UUID
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Week is the length of an intervention week.
const Week = 7 * 24 * time.Hour
