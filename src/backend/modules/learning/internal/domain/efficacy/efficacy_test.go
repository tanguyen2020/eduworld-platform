package efficacy

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func studyInput() StudyInput {
	return StudyInput{Code: "math-g3-2026", Title: "Toán 3", SubjectID: uuid.New(), GradeLevel: 3,
		ObjectiveIDs: []uuid.UUID{uuid.New(), uuid.New()}, FormA: uuid.New(), FormB: uuid.New(), MinWeeks: 8}
}

func fields(vs []Violation) string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Field)
	}
	return strings.Join(out, ",")
}

// F035-AC-07: the fields of a study.
func TestStudyValidate(t *testing.T) {
	if vs := studyInput().Validate(); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	s := studyInput()
	s.Code, s.Title, s.GradeLevel, s.MinWeeks = "Bad Code", strings.Repeat("x", 256), 10, 0
	s.FormB = s.FormA
	s.ObjectiveIDs = append(s.ObjectiveIDs, s.ObjectiveIDs[0])
	got := fields(s.Validate())
	for _, f := range []string{"/code", "/title", "/grade_level", "/objective_ids", "/form_b_blueprint_version_id", "/min_intervention_weeks"} {
		if !strings.Contains(got, f) {
			t.Errorf("missing %s in %s", f, got)
		}
	}
	s = studyInput()
	s.ObjectiveIDs = make([]uuid.UUID, 101)
	for i := range s.ObjectiveIDs {
		s.ObjectiveIDs[i] = uuid.New()
	}
	if fields(s.Validate()) != "/objective_ids" {
		t.Fatal("101 objectives accepted")
	}
}

// F035-AC-07: a form is a published efficacy blueprint of the subject and grade covering the objectives (≥ 3 each).
func TestCheckForm(t *testing.T) {
	s := studyInput()
	g := 3
	ok := Form{Published: true, Efficacy: true, SubjectID: s.SubjectID, GradeLevel: &g,
		PerObjective: map[uuid.UUID]int{s.ObjectiveIDs[0]: 3, s.ObjectiveIDs[1]: 4}}
	if vs := CheckForm("/form_a_blueprint_version_id", s, ok, 3); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	bad := []Form{ok, ok, ok, ok, ok, ok}
	bad[0].Published = false
	bad[1].Efficacy = false
	bad[2].SubjectID = uuid.New()
	g4 := 4
	bad[3].GradeLevel = &g4
	bad[4].PerObjective = map[uuid.UUID]int{s.ObjectiveIDs[0]: 2, s.ObjectiveIDs[1]: 3}
	bad[5].PerObjective = map[uuid.UUID]int{s.ObjectiveIDs[0]: 3, s.ObjectiveIDs[1]: 3, uuid.New(): 3}
	for i, f := range bad {
		if vs := CheckForm("/form_a_blueprint_version_id", s, f, 3); len(vs) != 1 || vs[0].Field != "/form_a_blueprint_version_id" {
			t.Errorf("case %d: %v", i, vs)
		}
	}
}

func TestAgreementRules(t *testing.T) {
	as := []Agreement{{Status: AgreementRevoked, Covers: []string{CoversStudy}}, {Status: AgreementActive, Covers: []string{CoversExperiment}}}
	if Covers(as, CoversStudy) || !Covers(as, CoversExperiment) {
		t.Fatal("Covers")
	}
	if !ValidCovers([]string{CoversStudy, CoversExperiment}) || ValidCovers(nil) || ValidCovers([]string{CoversStudy, CoversStudy}) ||
		ValidCovers([]string{"selection_rule"}) {
		t.Fatal("ValidCovers")
	}
	if ValidReason("short") || !ValidReason("theo yêu cầu của trường") || ValidReason(strings.Repeat("x", 1001)) {
		t.Fatal("ValidReason")
	}
}

func ptr(n int) *int { return &n }

// F035-AC-09: two classes per pair, random arms reproducible from (study_id, pair_no), form order alternating by pair.
func TestActivate(t *testing.T) {
	study := uuid.New()
	classes := []Class{{ClassID: uuid.New(), PairNo: ptr(1)}, {ClassID: uuid.New(), PairNo: ptr(1)},
		{ClassID: uuid.New(), PairNo: ptr(2), Arm: ArmComparison, Method: MethodManual}, {ClassID: uuid.New(), PairNo: ptr(2)},
		{ClassID: uuid.New(), Arm: ArmValidation, Method: MethodManual}}
	out, vs := Activate(study, classes)
	if len(vs) != 0 {
		t.Fatalf("violations %v", vs)
	}
	again, _ := Activate(study, classes)
	for i := range out {
		if out[i].Arm != again[i].Arm || out[i].FormOrder != again[i].FormOrder || out[i].Method != again[i].Method {
			t.Fatalf("not reproducible at %d", i)
		}
	}
	arms := map[int]map[string]int{}
	for _, c := range out {
		if c.PairNo == nil {
			if c.Arm != ArmValidation || c.FormOrder != OrderAThenB {
				t.Fatalf("validation class %+v", c)
			}
			continue
		}
		if arms[*c.PairNo] == nil {
			arms[*c.PairNo] = map[string]int{}
		}
		arms[*c.PairNo][c.Arm]++
		want := OrderAThenB
		if *c.PairNo == 2 {
			want = OrderBThenA
		}
		if c.FormOrder != want {
			t.Fatalf("pair %d form order %s", *c.PairNo, c.FormOrder)
		}
	}
	for n, a := range arms {
		if a[ArmIntervention] != 1 || a[ArmComparison] != 1 {
			t.Fatalf("pair %d arms %v", n, a)
		}
	}
	if out[2].Arm != ArmComparison || out[2].Method != MethodManual || out[3].Arm != ArmIntervention || out[3].Method != MethodRandom {
		t.Fatalf("manual arm not kept: %+v %+v", out[2], out[3])
	}
	// Both values of the random bit occur over the pairs of a study (a fair coin, not a constant).
	seen := map[bool]bool{}
	for n := 1; n <= 20; n++ {
		seen[RandomBit(study, n)] = true
	}
	if len(seen) != 2 {
		t.Fatal("RandomBit is constant")
	}
}

func TestActivateRefusals(t *testing.T) {
	study := uuid.New()
	cases := map[string][]Class{
		"single class":   {{ClassID: uuid.New(), PairNo: ptr(1)}},
		"three classes":  {{ClassID: uuid.New(), PairNo: ptr(1)}, {ClassID: uuid.New(), PairNo: ptr(1)}, {ClassID: uuid.New(), PairNo: ptr(1)}},
		"same arm":       {{ClassID: uuid.New(), PairNo: ptr(1), Arm: ArmIntervention}, {ClassID: uuid.New(), PairNo: ptr(1), Arm: ArmIntervention}},
		"no pair":        {{ClassID: uuid.New(), Arm: ArmValidation}},
		"unpaired armed": {{ClassID: uuid.New(), Arm: ArmIntervention}},
		"empty study":    {},
	}
	for name, cs := range cases {
		if _, vs := Activate(study, cs); len(vs) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestNewClass(t *testing.T) {
	if c, vs := NewClass(Class{PairNo: ptr(1)}); len(vs) != 0 || c.Method != MethodRandom {
		t.Fatalf("random: %v %+v", vs, c)
	}
	if c, vs := NewClass(Class{PairNo: ptr(1), Arm: ArmIntervention}); len(vs) != 0 || c.Method != MethodManual {
		t.Fatalf("manual: %v", vs)
	}
	for _, c := range []Class{{Arm: ArmValidation, PairNo: ptr(1)}, {Arm: ArmIntervention}, {PairNo: ptr(201)}, {PairNo: ptr(1), Arm: "x"},
		{PairNo: ptr(1), FormOrder: "x"}} {
		if _, vs := NewClass(c); len(vs) == 0 {
			t.Errorf("accepted %+v", c)
		}
	}
}

// F035-AC-12, AC-13: wave kinds, windows, order and spacing.
func TestCheckWave(t *testing.T) {
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	pre := WaveRequest{Kind: WavePre, WindowStart: now.Add(time.Hour), WindowEnd: now.Add(3 * time.Hour)}
	if vs, err := CheckWave(pre, StudyActive, 8, nil, now); err != nil || len(vs) != 0 {
		t.Fatalf("pre: %v %v", vs, err)
	}
	if _, err := CheckWave(pre, StudyDraft, 8, nil, now); !errors.Is(err, ErrState) {
		t.Fatal("pre on a draft study")
	}
	existing := []Wave{{Kind: WavePre, WindowStart: pre.WindowStart, WindowEnd: pre.WindowEnd, Status: WaveScheduled}}
	if _, err := CheckWave(pre, StudyActive, 8, existing, now); !errors.Is(err, ErrState) {
		t.Fatal("second pre")
	}
	post := WaveRequest{Kind: WavePost, WindowStart: pre.WindowEnd.Add(8 * Week), WindowEnd: pre.WindowEnd.Add(8*Week + time.Hour)}
	if vs, err := CheckWave(post, StudyActive, 8, existing, now); err != nil || len(vs) != 0 {
		t.Fatalf("post: %v %v", vs, err)
	}
	early := post
	early.WindowStart = pre.WindowEnd.Add(8*Week - time.Minute)
	if vs, err := CheckWave(early, StudyActive, 8, existing, now); err != nil || len(vs) != 1 || vs[0].Field != "/window_start" {
		t.Fatalf("early post: %v %v", vs, err)
	}
	if _, err := CheckWave(post, StudyActive, 8, nil, now); !errors.Is(err, ErrState) {
		t.Fatal("post without pre")
	}
	cancelled := []Wave{{Kind: WavePre, Status: WaveCancelled}}
	if vs, err := CheckWave(pre, StudyActive, 8, cancelled, now); err != nil || len(vs) != 0 {
		t.Fatal("pre after a cancelled pre")
	}
	val := WaveRequest{Kind: WaveValidation, WindowStart: now.Add(time.Hour), WindowEnd: now.Add(2 * time.Hour)}
	if vs, _ := CheckWave(val, StudyDraft, 8, nil, now); len(vs) != 1 || vs[0].Field != "/form" {
		t.Fatalf("validation without form: %v", vs)
	}
	val.Form = FormA
	if vs, err := CheckWave(val, StudyDraft, 8, nil, now); err != nil || len(vs) != 0 {
		t.Fatalf("validation: %v %v", vs, err)
	}
	past := WaveRequest{Kind: WavePre, WindowStart: now.Add(-3 * time.Hour), WindowEnd: now.Add(-time.Hour), DurationSec: 30}
	if vs, _ := CheckWave(past, StudyActive, 8, nil, now); len(vs) != 2 {
		t.Fatalf("past window and short duration: %v", vs)
	}
}

func TestFormOf(t *testing.T) {
	cases := []struct{ kind, order, req, want string }{
		{WavePre, OrderAThenB, "", FormA}, {WavePost, OrderAThenB, "", FormB},
		{WavePre, OrderBThenA, "", FormB}, {WavePost, OrderBThenA, "", FormA},
		{WaveValidation, OrderAThenB, FormB, FormB},
	}
	for _, c := range cases {
		if got := FormOf(c.kind, c.order, c.req); got != c.want {
			t.Errorf("%+v: %s", c, got)
		}
	}
	if !TargetsWave(WavePre, ArmComparison) || TargetsWave(WavePre, ArmValidation) || !TargetsWave(WaveValidation, ArmValidation) ||
		TargetsWave(WaveValidation, ArmIntervention) {
		t.Fatal("TargetsWave")
	}
}

// F035-AC-33: codes stable within a study, different across studies, no key → error.
func TestPseudonymizer(t *testing.T) {
	if _, err := NewPseudonymizer([]byte("short")); !errors.Is(err, ErrNoKey) {
		t.Fatal("short key accepted")
	}
	p, err := NewPseudonymizer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	s1, s2, student := uuid.New(), uuid.New(), uuid.New()
	a, b, c := p.Code(s1, student), p.Code(s1, student), p.Code(s2, student)
	if a != b || a == c || len(a) != 16 {
		t.Fatalf("codes %s %s %s", a, b, c)
	}
	if strings.Contains(p.String(), "kkkk") {
		t.Fatal("String leaks the key")
	}
	other, _ := NewPseudonymizer([]byte(strings.Repeat("z", 32)))
	if other.Code(s1, student) == a {
		t.Fatal("key ignored")
	}
}
