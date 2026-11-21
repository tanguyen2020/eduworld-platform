package efficacy

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"strconv"

	"github.com/google/uuid"
)

// Cohort is a cluster (school, grade) of a study as the pairing sees it (ADR-0095 D3): a study targets one grade, so a
// cohort is a school of a tenant in the study. Conflict marks a cohort migrated from classes that sat in different arms,
// pairs or form orders (never merged automatically).
type Cohort struct {
	SchoolID   uuid.UUID
	TenantID   uuid.UUID
	Conflict   bool
	PairNo     *int
	Arm        string // "" until set (manual) or assigned (random)
	Method     string
	FormOrder  string // "" until set or assigned
	GradeLevel int
}

// NewCohort checks a cohort added to a draft study (F035-AC-08): an optional pair number 1–200, an optional arm; a
// validation cohort has no pair, a paired cohort is not a validation cohort.
func NewCohort(c Cohort) (Cohort, []Violation) {
	var vs []Violation
	if c.PairNo != nil && (*c.PairNo < 1 || *c.PairNo > MaxPairNo) {
		vs = append(vs, Violation{"/pair_no", "must be between 1 and 200"})
	}
	switch c.Arm {
	case "", ArmIntervention, ArmComparison:
		if c.PairNo == nil {
			vs = append(vs, Violation{"/pair_no", "is required for an intervention or comparison cohort"})
		}
	case ArmValidation:
		if c.PairNo != nil {
			vs = append(vs, Violation{"/pair_no", "a validation cohort has no pair"})
		}
	default:
		vs = append(vs, Violation{"/arm", "must be intervention, comparison or validation"})
	}
	switch c.FormOrder {
	case "", OrderAThenB, OrderBThenA:
	default:
		vs = append(vs, Violation{"/form_order", "must be a_then_b or b_then_a"})
	}
	c.Method = MethodManual
	if c.Arm == "" {
		c.Method = MethodRandom
	}
	return c, vs
}

// RandomBit is the stable random choice of a pair (research R4): the lowest bit of sha256(study_id ‖ pair_no).
func RandomBit(studyID uuid.UUID, pairNo int) bool {
	h := sha256.New()
	h.Write(studyID[:])
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(pairNo)) //nolint:gosec // 1–200
	h.Write(b[:])
	sum := h.Sum(nil)
	return sum[len(sum)-1]&1 == 1
}

// Activate checks the pairs of a study and assigns what is left (F035-AC-09, LEARNING_EFFICACY_PLAN §3.1, §3.2): no cohort
// in conflict; every pair has exactly two cohorts of two different schools, one intervention and one comparison; an unset
// arm is chosen by RandomBit (the first cohort by school_id is intervention when the bit is set); an unset form order alternates by pair (odd a_then_b, even
// b_then_a); a validation cohort takes no arm change and a_then_b. The same input always gives the same output.
func Activate(studyID uuid.UUID, cohorts []Cohort) ([]Cohort, []Violation) {
	out := make([]Cohort, len(cohorts))
	copy(out, cohorts)
	pairs := map[int][]int{}
	var vs []Violation
	for i, c := range out {
		if c.Conflict {
			vs = append(vs, Violation{"/cohorts/" + c.SchoolID.String() + "/conflict", "the classes of this school sat in different arms, pairs or form orders: remove the cohort and add it again"})
			continue
		}
		if c.Arm == ArmValidation {
			if out[i].FormOrder == "" {
				out[i].FormOrder = OrderAThenB
			}
			continue
		}
		if c.PairNo == nil {
			vs = append(vs, Violation{"/cohorts/" + c.SchoolID.String() + "/pair_no", "an intervention or comparison cohort needs a pair"})
			continue
		}
		pairs[*c.PairNo] = append(pairs[*c.PairNo], i)
	}
	if len(pairs) == 0 && len(vs) == 0 {
		vs = append(vs, Violation{"/cohorts", "the study has no pair of cohorts"})
	}
	nos := make([]int, 0, len(pairs))
	for n := range pairs {
		nos = append(nos, n)
	}
	sort.Ints(nos)
	for _, n := range nos {
		idx := pairs[n]
		field := "/pairs/" + strconv.Itoa(n)
		if len(idx) != 2 {
			vs = append(vs, Violation{field, "a pair has exactly two cohorts"})
			continue
		}
		sort.Slice(idx, func(a, b int) bool { return out[idx[a]].SchoolID.String() < out[idx[b]].SchoolID.String() })
		if out[idx[0]].SchoolID == out[idx[1]].SchoolID {
			vs = append(vs, Violation{field, "the two cohorts of a pair are in two different schools"})
			continue
		}
		a, b := &out[idx[0]], &out[idx[1]]
		switch {
		case a.Arm == "" && b.Arm == "":
			if RandomBit(studyID, n) {
				a.Arm, b.Arm = ArmIntervention, ArmComparison
			} else {
				a.Arm, b.Arm = ArmComparison, ArmIntervention
			}
			a.Method, b.Method = MethodRandom, MethodRandom
		case a.Arm == "":
			a.Arm, a.Method = opposite(b.Arm), MethodRandom
		case b.Arm == "":
			b.Arm, b.Method = opposite(a.Arm), MethodRandom
		}
		if a.Arm == b.Arm || a.Arm == "" || b.Arm == "" {
			vs = append(vs, Violation{field, "a pair has one intervention and one comparison cohort"})
			continue
		}
		order := OrderAThenB
		if n%2 == 0 {
			order = OrderBThenA
		}
		for _, c := range []*Cohort{a, b} {
			if c.FormOrder == "" {
				c.FormOrder = order
			}
		}
	}
	if len(vs) > 0 {
		return cohorts, vs
	}
	return out, nil
}

func opposite(arm string) string {
	switch arm {
	case ArmIntervention:
		return ArmComparison
	case ArmComparison:
		return ArmIntervention
	}
	return ""
}

// ArmCounts counts the cohorts of each arm (descriptive_only and the sample warning of F035-AC-31).
func ArmCounts(cohorts []Cohort) map[string]int {
	out := map[string]int{}
	for _, c := range cohorts {
		out[c.Arm]++
	}
	return out
}
