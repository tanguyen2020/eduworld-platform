// Package recommendation is the rule engine of the automatic study recommendations (LEARNING_ANALYTICS §6,
// LEARNING_EXPERIENCE §6.2, F032 US6): each rule reads a snapshot of the learning state of one student and proposes
// explainable recommendations; the engine merges, filters and orders them. Rules are strategies registered at the
// composition root, so F040 (R6), F041 (R1) and F042 (R5) add theirs without changing the engine (F032-AC-33). No I/O.
package recommendation

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Kinds of recommendations (RecommendationList.items[].kind).
const (
	KindObjectiveReview    = "objective_review"
	KindPrerequisiteReview = "prerequisite_review"
	KindRetryWrong         = "retry_wrong"
	KindDiagnostic         = "diagnostic"
	KindInterleave         = "interleave_practice"
	KindAssignment         = "assignment"
	KindLesson             = "lesson"
	KindActivity           = "activity"
	KindQuest              = "quest"
)

// TargetSelection is a recommendation started with startAdHocAttempt (target_id = objective, topic or subject).
const TargetSelection = "selection"

// MaxObjectives bounds objective_ids of a recommendation.
const MaxObjectives = 10

// Candidate is one recommendation proposed by a rule.
type Candidate struct {
	RuleCode     string
	Kind         string
	TargetType   string
	TargetID     uuid.UUID
	SubjectID    *uuid.UUID
	Grade        *int
	ObjectiveIDs []uuid.UUID
	Reason       string            // RecommendationList reason
	ReasonCode   string            // key of the reason text (vi/en)
	Params       map[string]string // values of the reason text (names, counts)
	Priority     int               // 1 (highest) … 9
}

// Key identifies the target of a recommendation (a target skipped three times is not recommended again).
func (c Candidate) Key() string { return c.Kind + "|" + c.TargetType + "|" + c.TargetID.String() }

// Rule proposes recommendations from a snapshot.
type Rule interface {
	Code() string
	Evaluate(s Snapshot) []Candidate
}

// Objective is the state of one learning objective of the student's curriculum.
type Objective struct {
	ID             uuid.UUID
	Code           string
	Name           string
	TopicID        uuid.UUID
	SubjectID      uuid.UUID
	Grade          int
	Prerequisites  []uuid.UUID
	Level          string  // level after decay at Snapshot.Now
	Value          float64 // value after decay
	StoredLevel    string  // level of the stored value (no decay)
	StoredValue    float64
	EvidenceCount  int
	LastEvidenceAt *time.Time
	PriorSource    string
	PriorValue     *float64
	// Available: published questions of this objective are in the pool of the tenant (F032-AC-31).
	Available bool
}

// Topic is a topic of the student's grade.
type Topic struct {
	ID        uuid.UUID
	SubjectID uuid.UUID
	Grade     int
	SortOrder int
	Name      string
}

// WrongDue counts the due entries of the review list of one subject.
type WrongDue struct {
	SubjectID    uuid.UUID
	Count        int
	ObjectiveIDs []uuid.UUID
}

// Snapshot is the learning state of one student read by the rules.
type Snapshot struct {
	Now        time.Time
	Grade      *int
	Objectives []Objective
	Topics     []Topic
	WrongDue   []WrongDue
	// LastDiagnostic is the start of the latest diagnostic attempt per subject or topic id.
	LastDiagnostic map[uuid.UUID]time.Time
	// Upcoming are the next lessons of the LMS schedule of the student's courses (filled by F040 for R6).
	Upcoming []Upcoming
	// Assignments are the open school assignments the student has not submitted (filled by F041 for R1).
	Assignments []PendingAssignment
}

// PendingAssignment is an open school assignment of the student not submitted yet, with its personal due date.
type PendingAssignment struct {
	ID           uuid.UUID
	Title        string
	SubjectID    uuid.UUID
	ObjectiveIDs []uuid.UUID
	DueAt        time.Time
}

// Upcoming is a lesson (or activity) of the LMS schedule of a course of the student.
type Upcoming struct {
	ContentType  string
	ContentID    uuid.UUID
	SubjectID    uuid.UUID
	Title        string
	ObjectiveIDs []uuid.UUID
	Date         string // YYYY-MM-DD
}

func (s Snapshot) objective(id uuid.UUID) (Objective, bool) {
	for _, o := range s.Objectives {
		if o.ID == id {
			return o, true
		}
	}
	return Objective{}, false
}

// tieOrder orders rules of the same priority as Mây presents them (LEARNING_EXPERIENCE §6.2): assignments, warm-up,
// weak objectives (prerequisites first), exam follow-up, due reviews, decay, interleaving, next topic [Proposed].
var tieOrder = []string{"R1", "R7", "R3", "R2", "R5", "R9", "R4", "R8", "R6"}

func rank(code string) int {
	if i := slices.Index(tieOrder, code); i >= 0 {
		return i
	}
	return len(tieOrder)
}

// Engine runs the registered rules.
type Engine struct {
	rules []Rule
}

// NewEngine returns an engine with rules.
func NewEngine(rules ...Rule) *Engine { return &Engine{rules: slices.Clone(rules)} }

// Register adds a rule (F040/F041/F042 at the composition root); a rule with the same code replaces the previous one.
func (e *Engine) Register(r Rule) {
	for i, x := range e.rules {
		if x.Code() == r.Code() {
			e.rules[i] = r
			return
		}
	}
	e.rules = append(e.rules, r)
}

// Codes lists the codes of the registered rules.
func (e *Engine) Codes() []string {
	out := make([]string, len(e.rules))
	for i, r := range e.rules {
		out[i] = r.Code()
	}
	return out
}

// Run evaluates every rule, drops the targets blocked (skipped three times), merges the candidates of the same rule and
// target (union of objectives) and orders them by priority, then the order of Mây, then target id.
func (e *Engine) Run(s Snapshot, blocked func(Candidate) bool) []Candidate {
	byKey := map[string]int{}
	var out []Candidate
	for _, r := range e.rules {
		for _, c := range r.Evaluate(s) {
			if blocked != nil && blocked(c) {
				continue
			}
			k := c.RuleCode + "|" + c.Key()
			if i, ok := byKey[k]; ok {
				out[i].ObjectiveIDs = union(out[i].ObjectiveIDs, c.ObjectiveIDs)
				continue
			}
			c.ObjectiveIDs = union(nil, c.ObjectiveIDs)
			byKey[k] = len(out)
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b Candidate) int {
		if a.Priority != b.Priority {
			return a.Priority - b.Priority
		}
		if ra, rb := rank(a.RuleCode), rank(b.RuleCode); ra != rb {
			return ra - rb
		}
		return strings.Compare(a.TargetID.String(), b.TargetID.String())
	})
	return out
}

func union(a, b []uuid.UUID) []uuid.UUID {
	out := slices.Clone(a)
	for _, x := range b {
		if !slices.Contains(out, x) && len(out) < MaxObjectives {
			out = append(out, x)
		}
	}
	return out
}
