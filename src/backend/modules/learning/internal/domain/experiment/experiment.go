// Package experiment holds the rules of the A/B experiments on the mastery model and the recommendation rules (F035 US5,
// LEARNING_EFFICACY_PLAN §6, ADR-0080 mục 5): the arms and their allocation, the stable assignment of a unit, the
// transitions and the guardrail. Pure Go.
package experiment

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/recommendation"
)

// Kinds, units and statuses.
const (
	KindMastery        = "mastery_model"
	KindRecommendation = "recommendation_rule"

	UnitSchoolGrade = "school_grade" // the cluster (school, grade) of the student, ADR-0095 D3
	UnitStudent     = "student"

	StatusDraft     = "draft"
	StatusShadow    = "shadow"
	StatusLive      = "live"
	StatusStopped   = "stopped"
	StatusCompleted = "completed"

	StopManual    = "manual"
	StopGuardrail = "guardrail_breach"
	StopRevoked   = "agreement_revoked"
	StopEnded     = "ended"
)

// Arm is one arm of an experiment.
type Arm struct {
	Code          string          `json:"code"`
	AllocationPct int             `json:"allocation_pct"`
	IsControl     bool            `json:"is_control"`
	ModelVersion  string          `json:"model_version,omitempty"`
	RuleParams    json.RawMessage `json:"rule_params,omitempty"`
}

// Violation is one invalid field (422 VALIDATION_FAILED).
type Violation struct{ Field, Reason string }

// ErrState is a transition the experiment does not allow (422 INVALID_STATE_TRANSITION).
var ErrState = errors.New("experiment: invalid state transition")

var (
	codeRe    = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	armCodeRe = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
)

// AllowedRuleParams are the parameters of the recommendation engine an arm may change (F032 rule engine, R19: no
// parameter that adds play time or pressure).
var AllowedRuleParams = map[string]bool{
	"review_threshold":     true, // mastery under which an objective is recommended for review
	"max_recommendations":  true, // recommendations shown at once
	"prerequisite_depth":   true, // depth of the prerequisite walk
	"spaced_review_days":   true, // spacing of the review recommendations
	"practice_difficulty":  true, // target difficulty offset of practice (−1…+1)
	"diagnostic_threshold": true, // evidence count under which a diagnostic is proposed
}

// Definition is what an experiment is made of.
type Definition struct {
	Code       string
	Kind       string
	Unit       string
	Arms       []Arm
	TenantIDs  []uuid.UUID
	StartAt    time.Time
	EndAt      time.Time
	ActiveVer  string          // version of the active mastery model (control of mastery_model)
	KnownVers  map[string]bool // versions of mastery_model_configs
	NoConsent  []uuid.UUID     // tenants without an active agreement covering experiment
	Comparison bool            // the study given holds comparison/validation cohorts only (no intervention cohort)
}

// Validate applies F035-AC-21.
func (d Definition) Validate() []Violation {
	var vs []Violation
	if !codeRe.MatchString(d.Code) {
		vs = append(vs, Violation{"/code", "must match ^[a-z0-9_-]{1,64}$"})
	}
	if d.Kind != KindMastery && d.Kind != KindRecommendation {
		vs = append(vs, Violation{"/kind", "must be mastery_model or recommendation_rule"})
	}
	if d.Unit != UnitSchoolGrade && d.Unit != UnitStudent {
		vs = append(vs, Violation{"/unit", "must be school_grade or student (ADR-0095: no class)"})
	}
	if n := len(d.Arms); n < 2 || n > 5 {
		vs = append(vs, Violation{"/arms", "lists 2 to 5 arms"})
	}
	controls, total := 0, 0
	seen := map[string]bool{}
	for i, a := range d.Arms {
		p := "/arms/" + strconv.Itoa(i)
		if !armCodeRe.MatchString(a.Code) || seen[a.Code] {
			vs = append(vs, Violation{p + "/code", "must be a unique code ^[a-z0-9_]{1,32}$"})
		}
		seen[a.Code] = true
		if a.AllocationPct < 1 || a.AllocationPct > 99 {
			vs = append(vs, Violation{p + "/allocation_pct", "must be between 1 and 99"})
		}
		total += a.AllocationPct
		if a.IsControl {
			controls++
		}
		switch d.Kind {
		case KindMastery:
			switch {
			case a.ModelVersion == "" || !d.KnownVers[a.ModelVersion]:
				vs = append(vs, Violation{p + "/model_version", "is not a version of mastery_model_configs"})
			case a.IsControl && a.ModelVersion != d.ActiveVer:
				vs = append(vs, Violation{p + "/model_version", "the control arm uses the active model"})
			}
		case KindRecommendation:
			var params map[string]json.RawMessage
			if len(a.RuleParams) > 0 && json.Unmarshal(a.RuleParams, &params) != nil {
				vs = append(vs, Violation{p + "/rule_params", "must be an object"})
			}
			known := true
			for k := range params {
				if !AllowedRuleParams[k] {
					known = false
					vs = append(vs, Violation{p + "/rule_params/" + k, "is not a parameter of the recommendation engine"})
				}
			}
			if a.IsControl && len(params) > 0 {
				vs = append(vs, Violation{p + "/rule_params", "the control arm keeps the default parameters"})
			}
			if known && len(params) > 0 {
				rp, err := recommendation.ParseParams(a.RuleParams)
				if err != nil {
					vs = append(vs, Violation{p + "/rule_params", "has a value of the wrong type"})
				}
				for _, k := range sortedKeys(rp.Violations()) {
					vs = append(vs, Violation{p + "/rule_params/" + k, rp.Violations()[k]})
				}
			}
		}
	}
	if controls != 1 {
		vs = append(vs, Violation{"/arms", "exactly one arm is the control"})
	}
	if total != 100 {
		vs = append(vs, Violation{"/arms", "the allocations add up to 100"})
	}
	if n := len(d.TenantIDs); n < 1 || n > 200 {
		vs = append(vs, Violation{"/tenant_ids", "lists 1 to 200 tenants"})
	}
	for _, t := range d.NoConsent {
		vs = append(vs, Violation{"/tenant_ids", "tenant " + t.String() + " has no active agreement covering experiment"})
	}
	if d.Comparison {
		vs = append(vs, Violation{"/study_id", "the study has no intervention cohort (comparison and validation cohorts never take part)"})
	}
	if !d.EndAt.After(d.StartAt) {
		vs = append(vs, Violation{"/end_at", "must be after start_at"})
	}
	return vs
}

// Bucket is uint64(sha256(experiment_id ‖ unit_id)[:8]) mod 100 (research R4).
func Bucket(experimentID, unitID uuid.UUID) int {
	h := sha256.New()
	h.Write(experimentID[:])
	h.Write(unitID[:])
	return int(binary.BigEndian.Uint64(h.Sum(nil)[:8]) % 100)
}

// ArmOf is the first arm whose cumulated allocation exceeds the bucket (F035-AC-22).
func ArmOf(arms []Arm, bucket int) Arm {
	sum := 0
	for _, a := range arms {
		sum += a.AllocationPct
		if bucket < sum {
			return a
		}
	}
	return arms[len(arms)-1]
}

// Control returns the control arm.
func Control(arms []Arm) Arm {
	for _, a := range arms {
		if a.IsControl {
			return a
		}
	}
	return Arm{}
}

// Start checks a start (F035-AC-23, AC-24): shadow from draft, live from draft or shadow.
func Start(status, exposure string) (string, error) {
	switch {
	case exposure == StatusShadow && status == StatusDraft:
		return StatusShadow, nil
	case exposure == StatusLive && (status == StatusDraft || status == StatusShadow):
		return StatusLive, nil
	case exposure != StatusShadow && exposure != StatusLive:
		return "", errors.New("experiment: exposure must be shadow or live")
	}
	return "", ErrState
}

// Running reports an experiment that assigns units (shadow or live).
func Running(status string) bool { return status == StatusShadow || status == StatusLive }

// Breach reports the guardrail of F035-AC-26: an arm whose primary metric is at least dropPP percentage points under the
// control's (metrics are proportions 0–1).
func Breach(control, arm float64, dropPP float64) bool { return (control-arm)*100 >= dropPP-1e-9 }

// ArmLabel is the "<code>:<arm>" of a learning event (≤ 97 characters).
func ArmLabel(code, arm string) string { return code + ":" + arm }

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
