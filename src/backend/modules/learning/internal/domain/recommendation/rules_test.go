package recommendation

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

var now = time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)

func lo(subject, topic uuid.UUID, level string, n int, pre ...uuid.UUID) Objective {
	t := now.Add(-time.Hour)
	o := Objective{ID: uuid.New(), Name: "LO", SubjectID: subject, TopicID: topic, Grade: 3, Level: level, StoredLevel: level, EvidenceCount: n,
		Prerequisites: pre, Available: true}
	if n > 0 {
		o.LastEvidenceAt = &t
	}
	return o
}

func codes(cs []Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.RuleCode + ":" + c.Kind
	}
	return out
}

// F032-AC-30 (T046): each rule of LEARNING_ANALYTICS §6 on its condition.
func TestRules(t *testing.T) {
	sub, t1, t2 := uuid.New(), uuid.New(), uuid.New()
	g := 3
	pre := lo(sub, t1, "proficient", 5)
	w := lo(sub, t1, "developing", 3, pre.ID)
	few := lo(sub, t1, "developing", 2, pre.ID)
	s := Snapshot{Now: now, Grade: &g, Objectives: []Objective{pre, w, few}}
	if got := (R2{}).Evaluate(s); len(got) != 1 || got[0].TargetID != w.ID || got[0].Priority != 2 || got[0].Reason != "weak_objective" ||
		got[0].Params["objective"] != "LO" {
		t.Fatalf("R2: %+v", got)
	}
	// R3: the prerequisite is weak too (beginning), or not started with a low diagnostic prior.
	weakPre := lo(sub, t1, "beginning", 4)
	low := 0.3
	priorPre := lo(sub, t1, "not_started", 0)
	priorPre.PriorSource, priorPre.PriorValue = "diagnostic", &low
	w2 := lo(sub, t1, "beginning", 3, weakPre.ID, priorPre.ID)
	s3 := Snapshot{Now: now, Grade: &g, Objectives: []Objective{weakPre, priorPre, w2}}
	r3 := (R3{}).Evaluate(s3)
	if len(r3) != 2 || r3[0].Kind != KindPrerequisiteReview || r3[0].ObjectiveIDs[1] != w2.ID {
		t.Fatalf("R3: %+v", r3)
	}
	if got := (R2{}).Evaluate(s3); len(got) != 1 || got[0].TargetID != weakPre.ID {
		t.Fatalf("R2 skips a weak objective with a weak prerequisite: %+v", got)
	}
	// R4: decay of at least 0.15.
	d := lo(sub, t1, "developing", 6)
	d.StoredLevel, d.StoredValue, d.Value = "mastered", 0.85, 0.64
	d2 := lo(sub, t1, "proficient", 6)
	d2.StoredValue, d2.Value = 0.70, 0.60
	if got := (R4{}).Evaluate(Snapshot{Now: now, Objectives: []Objective{d, d2}}); len(got) != 1 || got[0].TargetID != d.ID || got[0].Reason != "decay_review" {
		t.Fatalf("R4: %+v", got)
	}
	// R7: no evidence in the subject → warm-up of the subject; a recent diagnostic → nothing; a new topic mostly
	// without evidence → warm-up of the topic.
	fresh := Snapshot{Now: now, Grade: &g, Objectives: []Objective{lo(sub, t1, "not_started", 0), lo(sub, t2, "not_started", 0)}}
	if got := (R7{}).Evaluate(fresh); len(got) != 1 || got[0].TargetID != sub || got[0].ReasonCode != "rec.diagnostic_due.subject" || *got[0].Grade != 3 {
		t.Fatalf("R7 subject: %+v", got)
	}
	fresh.LastDiagnostic = map[uuid.UUID]time.Time{sub: now.Add(-100 * 24 * time.Hour)}
	if got := (R7{}).Evaluate(fresh); len(got) != 0 {
		t.Fatalf("R7 within 120 days: %+v", got)
	}
	fresh.LastDiagnostic = map[uuid.UUID]time.Time{sub: now.Add(-130 * 24 * time.Hour)}
	if got := (R7{}).Evaluate(fresh); len(got) != 1 {
		t.Fatalf("R7 after 120 days: %+v", got)
	}
	started := lo(sub, t2, "developing", 1)
	newTopic := Snapshot{Now: now, Grade: &g, Objectives: []Objective{lo(sub, t1, "proficient", 6), started, lo(sub, t2, "not_started", 0),
		lo(sub, t2, "not_started", 0)}, Topics: []Topic{{ID: t2, SubjectID: sub, Grade: 3, Name: "Phép chia"}}}
	newTopic.Objectives[0].LastEvidenceAt = ptr(now.Add(-48 * time.Hour))
	if got := (R7{}).Evaluate(newTopic); len(got) != 1 || got[0].TargetID != t2 || got[0].Params["topic"] != "Phép chia" {
		t.Fatalf("R7 topic: %+v", got)
	}
	// R8: 3 strong objectives over 2 topics and nothing weak pending.
	strongs := []Objective{lo(sub, t1, "proficient", 5), lo(sub, t1, "mastered", 6), lo(sub, t2, "proficient", 5)}
	if got := (R8{}).Evaluate(Snapshot{Now: now, Objectives: strongs}); len(got) != 1 || got[0].TargetID != sub || len(got[0].ObjectiveIDs) != 3 {
		t.Fatalf("R8: %+v", got)
	}
	if got := (R8{}).Evaluate(Snapshot{Now: now, Objectives: append(strongs, lo(sub, t1, "developing", 3))}); len(got) != 0 {
		t.Fatalf("R8 with R2 pending: %+v", got)
	}
	// R9: due wrong questions per subject.
	if got := (R9{}).Evaluate(Snapshot{WrongDue: []WrongDue{{SubjectID: sub, Count: 2}}}); len(got) != 1 || got[0].Params["count"] != "2" {
		t.Fatalf("R9: %+v", got)
	}
	// Unavailable content is never recommended (F032-AC-31).
	w.Available = false
	if got := (R2{}).Evaluate(Snapshot{Now: now, Objectives: []Objective{pre, w}}); len(got) != 0 {
		t.Fatalf("unavailable: %+v", got)
	}
}

func ptr(t time.Time) *time.Time { return &t }

// assignmentRule is a TEST rule standing for R1 of F041: registered without changing the engine.
type assignmentRule struct{ target uuid.UUID }

func (assignmentRule) Code() string { return "R1" }
func (r assignmentRule) Evaluate(Snapshot) []Candidate {
	return []Candidate{{RuleCode: "R1", Kind: KindAssignment, TargetType: "assignment", TargetID: r.target, Reason: "assignment_soon",
		ReasonCode: "rec.assignment_soon", Priority: 1}}
}

// F032-AC-29, AC-31, AC-33 (T046): a rule of another feature registers through the interface; ordering by priority and
// the order of Mây; a blocked target (skipped three times) is dropped; same rule and target merge.
func TestEngine(t *testing.T) {
	sub, t1 := uuid.New(), uuid.New()
	g := 3
	pre := lo(sub, t1, "proficient", 5)
	w := lo(sub, t1, "developing", 3, pre.ID)
	weakPre := lo(sub, t1, "beginning", 4)
	w2 := lo(sub, t1, "beginning", 3, weakPre.ID)
	w3 := lo(sub, t1, "beginning", 3, weakPre.ID)
	s := Snapshot{Now: now, Grade: &g, Objectives: []Objective{pre, w, weakPre, w2, w3}, WrongDue: []WrongDue{{SubjectID: sub, Count: 1}}}
	e := Default()
	e.Register(assignmentRule{target: uuid.New()})
	if len(e.Codes()) != 7 {
		t.Fatalf("codes: %v", e.Codes())
	}
	got := e.Run(s, nil)
	want := []string{"R1:assignment", "R3:prerequisite_review", "R2:objective_review", "R2:objective_review", "R9:retry_wrong"}
	if c := codes(got); len(c) != len(want) {
		t.Fatalf("run: %v", c)
	} else {
		for i := range want {
			if c[i] != want[i] {
				t.Fatalf("order: %v", c)
			}
		}
	}
	if len(got[1].ObjectiveIDs) != 3 { // R3 for weakPre merged from w2 and w3
		t.Fatalf("merge: %+v", got[1])
	}
	blocked := got[2].Key()
	if again := e.Run(s, func(c Candidate) bool { return c.Key() == blocked }); len(again) != len(got)-1 {
		t.Fatalf("blocked: %v", codes(again))
	}
}

// F040-AC-12 (T018): R6 recommends the earliest scheduled lesson without evidence of each subject, priority 4.
func TestR6(t *testing.T) {
	sub := uuid.New()
	learned := lo(sub, uuid.New(), "developing", 2)
	fresh := uuid.New()
	l1, l2 := uuid.New(), uuid.New()
	s := Snapshot{Now: now, Objectives: []Objective{learned}, Upcoming: []Upcoming{
		{ContentType: "lesson", ContentID: l2, SubjectID: sub, Title: "Phép chia", ObjectiveIDs: []uuid.UUID{fresh}, Date: "2026-11-25"},
		{ContentType: "lesson", ContentID: uuid.New(), SubjectID: sub, Title: "Đã học", ObjectiveIDs: []uuid.UUID{learned.ID}, Date: "2026-11-21"},
		{ContentType: "activity", ContentID: l1, SubjectID: sub, Title: "Sớm hơn", ObjectiveIDs: []uuid.UUID{fresh}, Date: "2026-11-22"},
	}}
	got := (R6{}).Evaluate(s)
	if len(got) != 1 || got[0].TargetID != l1 || got[0].Priority != 4 || got[0].Kind != KindActivity || got[0].TargetType != "activity" ||
		got[0].Params["topic"] != "Sớm hơn" {
		t.Fatalf("R6: %+v", got)
	}
	e := Default()
	e.Register(R6{})
	if !slices.Contains(e.Codes(), "R6") {
		t.Fatal("R6 registered")
	}
}
