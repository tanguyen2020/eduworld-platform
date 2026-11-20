package command

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/recommendation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// TEST fakes: only the methods the adaptive plan calls (the embedded interfaces are nil).
type fakeSelfReader struct {
	write.SelfStudyReader
	rows []write.MasteryRow
}

func (f fakeSelfReader) StudentMasteries(context.Context, uuid.UUID) ([]write.MasteryRow, error) {
	return f.rows, nil
}
func (fakeSelfReader) RecentQuestions(context.Context, uuid.UUID, time.Time) ([]uuid.UUID, error) {
	return nil, nil
}

type fakeModels struct{ write.MasteryStore }

func (fakeModels) ActiveModel(context.Context) (write.ModelConfig, error) {
	return write.ModelConfig{ModelVersion: "we-1.0"}, nil
}

type fakeDelivery struct {
	port.QuestionDelivery
	got *selection.Request
}

func (f fakeDelivery) Select(_ context.Context, req selection.Request) ([]selection.Candidate, error) {
	*f.got = req
	return nil, nil
}

type fakeExperience struct{ arm ExperienceArm }

func (f fakeExperience) Experience(context.Context, uuid.UUID, uuid.UUID) (ExperienceArm, error) {
	return f.arm, nil
}

// F035-AC-24 (T051): the adaptive practice difficulty follows the live arm — its model and states set the level, its
// practice_difficulty shifts the range; without an experiment the plan of F032 is unchanged.
func TestPracticeDifficultyByArm(t *testing.T) {
	now := time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)
	tenant, student, objective := uuid.New(), uuid.New(), uuid.New()
	active := mastery.State{Value: 0.30, EvidenceCount: 6, DistinctDays: 3, LastEvidenceAt: &now} // beginning
	var got selection.Request
	h := Attempts{Clock: func() time.Time { return now }, Delivery: fakeDelivery{got: &got},
		Self: &SelfStudy{Reader: fakeSelfReader{rows: []write.MasteryRow{{ObjectiveID: objective, State: active}}}, Masteries: fakeModels{},
			Config: DefaultPracticeConfig()}}
	p := authz.Principal{PrincipalData: authz.PrincipalData{UserID: student, TenantID: &tenant, UserType: authz.UserTypeStudent}}
	src := AdHocSource{Kind: "selection", ObjectiveID: &objective}
	plan := func(h Attempts) selection.Range {
		t.Helper()
		if _, err := h.plan(context.Background(), p, attempt.ModePractice, src, nil, 1, now); err != nil {
			t.Fatal(err)
		}
		return *got.Rule.Filters.Difficulty
	}
	if r := plan(h); r != (selection.Range{Min: 1, Max: 2}) {
		t.Fatalf("no experiment: %+v", r)
	}
	up := 1
	h.Experience = fakeExperience{ExperienceArm{Rules: recommendation.Params{PracticeDifficulty: &up}}}
	if r := plan(h); r != (selection.Range{Min: 2, Max: 3}) {
		t.Fatalf("practice_difficulty +1: %+v", r)
	}
	m := mastery.WeightedEvidence{ModelVersion: "we-1.1", P: mastery.DefaultParams()}
	h.Experience = fakeExperience{ExperienceArm{Model: &m, States: map[uuid.UUID]mastery.State{
		objective: {Value: 0.70, EvidenceCount: 6, DistinctDays: 3, LastEvidenceAt: &now}}}} // proficient under the arm model
	if r := plan(h); r != (selection.Range{Min: 3, Max: 4}) {
		t.Fatalf("arm model: %+v", r)
	}
	// An explicit difficulty of the student is kept whatever the arm.
	d := 5
	src.Difficulty = &d
	if r := plan(h); r != (selection.Range{Min: 5, Max: 5}) {
		t.Fatalf("explicit difficulty: %+v", r)
	}
}

func TestExperienceArmMastery(t *testing.T) {
	o := uuid.New()
	base := mastery.WeightedEvidence{ModelVersion: "we-1.0", P: mastery.DefaultParams()}
	s := mastery.State{Value: 0.3}
	if m, st := (ExperienceArm{}).Mastery(base, o, s); m.ModelVersion != "we-1.0" || st.Value != 0.3 {
		t.Fatal("control experience changed")
	}
	arm := mastery.WeightedEvidence{ModelVersion: "we-1.1", P: mastery.DefaultParams()}
	x := ExperienceArm{Model: &arm, States: map[uuid.UUID]mastery.State{o: {Value: 0.6}}}
	if m, st := x.Mastery(base, o, s); m.ModelVersion != "we-1.1" || st.Value != 0.6 {
		t.Fatal("arm state not used")
	}
	if _, st := x.Mastery(base, uuid.New(), s); st.Value != 0.3 {
		t.Fatal("an objective without an arm state keeps the active state")
	}
}
