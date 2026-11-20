package recommendation

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Levels of the mastery model (LEARNING_ANALYTICS §3.2).
const (
	levelBeginning  = "beginning"
	levelDeveloping = "developing"
	levelProficient = "proficient"
	levelMastered   = "mastered"
)

// Thresholds of the rules (LEARNING_ANALYTICS §4, §6).
const (
	// WeakMinEvidence: weak_objectives are levels ≤ developing with at least 3 evidence.
	WeakMinEvidence = 3
	// DecayDrop: R4 recommends a review when decay lowered M_eff by at least 0.15.
	DecayDrop = 0.15
	// DiagnosticWindow: R7 does not recommend a warm-up again within 120 days of the last one.
	DiagnosticWindow = 120 * 24 * time.Hour
	// lowPrior: a diagnostic prior under the developing threshold is weak (R3).
	lowPrior = 0.40
)

// weak: level beginning or developing (or, with the review_threshold of an experiment arm, M_eff under it) with at least
// WeakMinEvidence evidence.
func (s Snapshot) weak(o Objective) bool {
	if o.EvidenceCount < WeakMinEvidence {
		return false
	}
	if t := s.Params.ReviewThreshold; t != nil {
		return o.Value < *t
	}
	return o.Level == levelBeginning || o.Level == levelDeveloping
}

func strong(o Objective) bool { return o.Level == levelProficient || o.Level == levelMastered }

// weakPrerequisite: a prerequisite at beginning/developing, or not started with a low prior of a diagnostic.
func weakPrerequisite(o Objective) bool {
	if o.Level == levelBeginning || o.Level == levelDeveloping {
		return true
	}
	return o.EvidenceCount == 0 && o.PriorValue != nil && *o.PriorValue < lowPrior
}

func subjectOf(o Objective) *uuid.UUID {
	s := o.SubjectID
	return &s
}

func objectiveCandidate(rule, kind, reason string, o Objective, priority int, also ...uuid.UUID) Candidate {
	return Candidate{RuleCode: rule, Kind: kind, TargetType: TargetSelection, TargetID: o.ID, SubjectID: subjectOf(o),
		ObjectiveIDs: append([]uuid.UUID{o.ID}, also...), Reason: reason, ReasonCode: "rec." + reason, Params: map[string]string{"objective": o.Name},
		Priority: priority}
}

// R2 — a weak objective whose prerequisites are proficient: practice it (prefer_weak_objectives).
type R2 struct{}

// Code implements Rule.
func (R2) Code() string { return "R2" }

// Evaluate implements Rule.
func (R2) Evaluate(s Snapshot) []Candidate {
	var out []Candidate
	for _, o := range s.Objectives {
		if !s.weak(o) || !o.Available || !prerequisitesStrong(s, o) {
			continue
		}
		out = append(out, objectiveCandidate("R2", KindObjectiveReview, "weak_objective", o, 2))
	}
	return out
}

func prerequisitesStrong(s Snapshot, o Objective) bool {
	for _, id := range o.Prerequisites {
		if p, ok := s.objective(id); ok && !strong(p) {
			return false
		}
	}
	return true
}

// R3 — a weak objective with a weak prerequisite (diagnostic prior included): review the prerequisite first.
type R3 struct{}

// Code implements Rule.
func (R3) Code() string { return "R3" }

// Evaluate implements Rule.
func (R3) Evaluate(s Snapshot) []Candidate {
	var out []Candidate
	for _, o := range s.Objectives {
		if !s.weak(o) {
			continue
		}
		for _, p := range s.prerequisites(o) {
			if !p.Available || !weakPrerequisite(p) {
				continue
			}
			c := objectiveCandidate("R3", KindPrerequisiteReview, "prerequisite_weak", p, 2, o.ID)
			c.Params["for"] = o.Name
			out = append(out, c)
		}
	}
	return out
}

// prerequisites walks the prerequisites of o known to the snapshot breadth first, to the prerequisite_depth of the
// arm (1 = direct prerequisites, the default).
func (s Snapshot) prerequisites(o Objective) []Objective {
	seen := map[uuid.UUID]bool{o.ID: true}
	level := o.Prerequisites
	var out []Objective
	for d := 0; d < s.Params.depth() && len(level) > 0; d++ {
		var next []uuid.UUID
		for _, id := range level {
			if seen[id] {
				continue
			}
			seen[id] = true
			p, ok := s.objective(id)
			if !ok {
				continue
			}
			out = append(out, p)
			next = append(next, p.Prerequisites...)
		}
		level = next
	}
	return out
}

// R4 — a proficient or mastered objective whose decay lowered M_eff by at least 0.15: spaced review.
type R4 struct{}

// Code implements Rule.
func (R4) Code() string { return "R4" }

// Evaluate implements Rule.
func (R4) Evaluate(s Snapshot) []Candidate {
	var out []Candidate
	for _, o := range s.Objectives {
		if o.EvidenceCount == 0 || !o.Available || (o.StoredLevel != levelProficient && o.StoredLevel != levelMastered) ||
			o.StoredValue-o.Value < DecayDrop || (o.LastEvidenceAt != nil && s.Now.Sub(*o.LastEvidenceAt) < s.Params.spacing()) {
			continue
		}
		out = append(out, objectiveCandidate("R4", KindObjectiveReview, "decay_review", o, 3))
	}
	return out
}

// R7 — start of the school year (no evidence in a subject of the grade) or a new topic mostly without evidence, and
// no diagnostic of that subject or topic in the last 120 days: take the warm-up diagnostic (LEARNING_ANALYTICS §3.5).
type R7 struct{}

// Code implements Rule.
func (R7) Code() string { return "R7" }

// Evaluate implements Rule.
func (R7) Evaluate(s Snapshot) []Candidate {
	if s.Grade == nil {
		return nil
	}
	grade := *s.Grade
	recent := func(ids ...uuid.UUID) bool {
		for _, id := range ids {
			if t, ok := s.LastDiagnostic[id]; ok && s.Now.Sub(t) < DiagnosticWindow {
				return true
			}
		}
		return false
	}
	bySubject := map[uuid.UUID][]Objective{}
	var subjects []uuid.UUID
	for _, o := range s.Objectives {
		if o.Grade != grade {
			continue
		}
		if _, ok := bySubject[o.SubjectID]; !ok {
			subjects = append(subjects, o.SubjectID)
		}
		bySubject[o.SubjectID] = append(bySubject[o.SubjectID], o)
	}
	var out []Candidate
	for _, subject := range subjects {
		los := bySubject[subject]
		if !anyAvailable(los) {
			continue
		}
		var latest *Objective
		for i := range los {
			if los[i].LastEvidenceAt != nil && (latest == nil || los[i].LastEvidenceAt.After(*latest.LastEvidenceAt)) {
				latest = &los[i]
			}
		}
		g := grade
		sub := subject
		if latest == nil {
			if recent(subject) {
				continue
			}
			out = append(out, Candidate{RuleCode: "R7", Kind: KindDiagnostic, TargetType: TargetSelection, TargetID: subject, SubjectID: &sub,
				Grade: &g, ObjectiveIDs: firstIDs(los), Reason: "diagnostic_due", ReasonCode: "rec.diagnostic_due.subject",
				Params: map[string]string{}, Priority: 2})
			continue
		}
		var topic []Objective
		for _, o := range los {
			if o.TopicID == latest.TopicID {
				topic = append(topic, o)
			}
		}
		without := 0
		for _, o := range topic {
			if s.Params.notStarted(o) {
				without++
			}
		}
		if 2*without <= len(topic) || recent(subject, latest.TopicID) || !anyAvailable(topic) {
			continue
		}
		out = append(out, Candidate{RuleCode: "R7", Kind: KindDiagnostic, TargetType: TargetSelection, TargetID: latest.TopicID, SubjectID: &sub,
			Grade: &g, ObjectiveIDs: firstIDs(topic), Reason: "diagnostic_due", ReasonCode: "rec.diagnostic_due.topic",
			Params: map[string]string{"topic": topicName(s, latest.TopicID)}, Priority: 2})
	}
	return out
}

func anyAvailable(os []Objective) bool {
	return slices.ContainsFunc(os, func(o Objective) bool { return o.Available })
}

func firstIDs(os []Objective) []uuid.UUID {
	var out []uuid.UUID
	for _, o := range os {
		if len(out) == MaxObjectives {
			break
		}
		out = append(out, o.ID)
	}
	return out
}

func topicName(s Snapshot, id uuid.UUID) string {
	for _, t := range s.Topics {
		if t.ID == id {
			return t.Name
		}
	}
	return ""
}

// R8 — at least 3 proficient/mastered objectives over at least 2 topics of a subject and no R2/R3 pending in it:
// interleaved practice (LEARNING_EXPERIENCE §5.2).
type R8 struct{}

// Code implements Rule.
func (R8) Code() string { return "R8" }

// Evaluate implements Rule.
func (R8) Evaluate(s Snapshot) []Candidate {
	pending := map[uuid.UUID]bool{}
	for _, c := range append(R2{}.Evaluate(s), R3{}.Evaluate(s)...) {
		if c.SubjectID != nil {
			pending[*c.SubjectID] = true
		}
	}
	type acc struct {
		los    []uuid.UUID
		topics map[uuid.UUID]bool
	}
	by := map[uuid.UUID]*acc{}
	var subjects []uuid.UUID
	for _, o := range s.Objectives {
		if !strong(o) || !o.Available {
			continue
		}
		a, ok := by[o.SubjectID]
		if !ok {
			a = &acc{topics: map[uuid.UUID]bool{}}
			by[o.SubjectID] = a
			subjects = append(subjects, o.SubjectID)
		}
		a.los = append(a.los, o.ID)
		a.topics[o.TopicID] = true
	}
	var out []Candidate
	for _, subject := range subjects {
		a := by[subject]
		if len(a.los) < 3 || len(a.topics) < 2 || pending[subject] {
			continue
		}
		sub := subject
		out = append(out, Candidate{RuleCode: "R8", Kind: KindInterleave, TargetType: TargetSelection, TargetID: subject, SubjectID: &sub,
			ObjectiveIDs: a.los, Reason: "interleave_ready", ReasonCode: "rec.interleave_ready",
			Params: map[string]string{"topics": strconv.Itoa(len(a.topics))}, Priority: 3})
	}
	return out
}

// R9 — first-try wrong questions due for review (retry_wrong schedule): review them, siblings first.
type R9 struct{}

// Code implements Rule.
func (R9) Code() string { return "R9" }

// Evaluate implements Rule.
func (R9) Evaluate(s Snapshot) []Candidate {
	var out []Candidate
	for _, w := range s.WrongDue {
		if w.Count <= 0 || w.SubjectID == uuid.Nil {
			continue
		}
		sub := w.SubjectID
		out = append(out, Candidate{RuleCode: "R9", Kind: KindRetryWrong, TargetType: TargetSelection, TargetID: w.SubjectID, SubjectID: &sub,
			ObjectiveIDs: w.ObjectiveIDs, Reason: "due_review", ReasonCode: "rec.due_review",
			Params: map[string]string{"count": strconv.Itoa(w.Count)}, Priority: 3})
	}
	return out
}

// Default is the engine of F032 with R2, R3, R4, R7, R8 and R9 (R1, R5, R6 are registered by F041, F042, F040).
func Default() *Engine { return NewEngine(R2{}, R3{}, R4{}, R7{}, R8{}, R9{}) }

// R6 — the next lesson of the LMS schedule of a course whose objectives have no evidence yet: study it (priority 4,
// LEARNING_ANALYTICS §6; registered by F040). One recommendation per subject, the earliest scheduled.
type R6 struct{}

// Code implements Rule.
func (R6) Code() string { return "R6" }

// Evaluate implements Rule.
func (R6) Evaluate(s Snapshot) []Candidate {
	evidence := map[uuid.UUID]bool{}
	for _, o := range s.Objectives {
		if o.EvidenceCount > 0 {
			evidence[o.ID] = true
		}
	}
	upcoming := slices.Clone(s.Upcoming)
	slices.SortStableFunc(upcoming, func(a, b Upcoming) int { return strings.Compare(a.Date, b.Date) })
	done := map[uuid.UUID]bool{}
	var out []Candidate
	for _, u := range upcoming {
		if done[u.SubjectID] || len(u.ObjectiveIDs) == 0 || slices.ContainsFunc(u.ObjectiveIDs, func(id uuid.UUID) bool { return evidence[id] }) {
			continue
		}
		done[u.SubjectID] = true
		kind := KindActivity
		if u.ContentType == "lesson" {
			kind = KindLesson
		}
		sub := u.SubjectID
		out = append(out, Candidate{RuleCode: "R6", Kind: kind, TargetType: u.ContentType, TargetID: u.ContentID, SubjectID: &sub,
			ObjectiveIDs: u.ObjectiveIDs, Reason: "next_topic", ReasonCode: "rec.next_topic", Params: map[string]string{"topic": u.Title},
			Priority: 4})
	}
	return out
}

// R1Window is how close the personal due date of an assignment makes R1 recommend it (48 hours, LEARNING_ANALYTICS §6).
const R1Window = 48 * time.Hour

// R1 — an open school assignment not submitted whose personal due date is less than 48 hours ahead: finish it first
// (priority 1, LEARNING_ANALYTICS §6; registered by F041). One recommendation per assignment.
type R1 struct{}

// Code implements Rule.
func (R1) Code() string { return "R1" }

// Evaluate implements Rule.
func (R1) Evaluate(s Snapshot) []Candidate {
	var out []Candidate
	for _, a := range s.Assignments {
		left := a.DueAt.Sub(s.Now)
		if left <= 0 || left > R1Window {
			continue
		}
		sub := a.SubjectID
		los := a.ObjectiveIDs
		if len(los) > MaxObjectives {
			los = los[:MaxObjectives]
		}
		out = append(out, Candidate{RuleCode: "R1", Kind: KindAssignment, TargetType: "assignment", TargetID: a.ID, SubjectID: &sub,
			ObjectiveIDs: los, Reason: "assignment_soon", ReasonCode: "rec.assignment_soon", Params: map[string]string{"title": a.Title}, Priority: 1})
	}
	return out
}

// R5ExamRatio is the share of correct items of an objective in an exam below which R5 recommends it (60 %, EXAM_ENGINE §10).
const R5ExamRatio = 0.6

// R5 — after an exam, an objective answered correctly less than 60 % of the time whose mastery is below proficient:
// review it (priority 2, LEARNING_ANALYTICS §6, EXAM_ENGINE §10; registered by F042). One recommendation per objective.
type R5 struct{}

// Code implements Rule.
func (R5) Code() string { return "R5" }

// Evaluate implements Rule.
func (R5) Evaluate(s Snapshot) []Candidate {
	byID := map[uuid.UUID]Objective{}
	for _, o := range s.Objectives {
		byID[o.ID] = o
	}
	done := map[uuid.UUID]bool{}
	var out []Candidate
	for _, r := range s.ExamResults {
		o, ok := byID[r.ObjectiveID]
		if !ok || done[r.ObjectiveID] || r.Total == 0 || float64(r.Correct)/float64(r.Total) >= R5ExamRatio ||
			o.Level == levelProficient || o.Level == levelMastered {
			continue
		}
		done[r.ObjectiveID] = true
		out = append(out, objectiveCandidate("R5", KindObjectiveReview, "exam_followup", o, 2))
	}
	return out
}
