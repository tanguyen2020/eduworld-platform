package challenge

import (
	"errors"
	"slices"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)

func rules() Rules {
	return Rules{DefaultLives: map[string]int{Easy: 5, Normal: 4, Hard: 3, Expert: 3, Legendary: 2},
		DefaultSeconds: map[string]int{Easy: 40, Normal: 30, Hard: 25, Expert: 20, Legendary: 20},
		AttemptsPerDay: 5, FailCooldown: time.Minute, CooldownFrom: Hard, G13TimeFactor: 1.5, G13MaxLevel: Hard,
		ResumeWindow: 24 * time.Hour, DeadlineGrace: 2 * time.Second, RegenStreak: 3}
}

func five() []Profile {
	return []Profile{{Level: Easy, Min: 1, Max: 2, RewardMultiplier: 1}, {Level: Normal, Min: 2, Max: 3, RewardMultiplier: 1.3},
		{Level: Hard, Min: 3, Max: 4, RewardMultiplier: 1.6}, {Level: Expert, Min: 4, Max: 5, RewardMultiplier: 2, GradeLevels: []int{4, 5, 6, 7}},
		{Level: Legendary, Min: 5, Max: 5, RewardMultiplier: 2.5, GradeLevels: []int{6, 7}}}
}

func version(typ string) Version {
	end := t0.Add(48 * time.Hour)
	start := t0.Add(-time.Hour)
	return Version{ChallengeID: "0193a5f4-6c1e-7b2a-9d1f-000000076101", Code: "chl.math.g3_7", Type: typ,
		Name: Text{VI: "Thử thách", EN: "Challenge"}, GradeLevels: []int{3, 4, 5, 6, 7}, SubjectID: "0193a5f4-6c1e-7b2a-9d1f-000000000201",
		ObjectiveIDs: []string{"0193a5f4-6c1e-7b2a-9d1f-000000000301"}, Mode: ModeSolo, Difficulties: five(),
		Source: Source{Kind: SourceActivities, ActivityRefs: []ActivityRef{{ActivityType: "quiz", ActivityID: "0193a5f4-6c1e-7b2a-9d1f-000000030001"},
			{ActivityType: "quiz", ActivityID: "0193a5f4-6c1e-7b2a-9d1f-000000030002"}}},
		Goal: Goal{Metric: MetricCorrect, Target: 5}, TimeLimitSeconds: 300, Availability: &Availability{StartAt: &start, EndAt: &end}}
}

// F076-AC-01: khối 1–3 chỉ easy–hard (g1_3_challenge_rules); profile.grade_levels giới hạn thêm; thứ tự mức giữ nguyên.
func TestDifficultiesForGrade(t *testing.T) {
	r := rules()
	v := version(TypeTime)
	if got := r.Difficulties(v, 3); !slices.Equal(got, []string{Easy, Normal, Hard}) {
		t.Fatalf("grade 3: %v", got)
	}
	if got := r.Difficulties(v, 7); !slices.Equal(got, Levels) {
		t.Fatalf("grade 7: %v", got)
	}
	if got := r.Difficulties(v, 5); !slices.Equal(got, []string{Easy, Normal, Hard, Expert}) {
		t.Fatalf("grade 5 (legendary for 6–7 only): %v", got)
	}
	if got := r.Difficulties(v, 9); len(got) != 0 {
		t.Fatalf("grade outside the challenge: %v", got)
	}
	p, _ := v.Profile(Hard)
	if !r.Allowed(v, p, 3) {
		t.Fatal("hard for grade 3")
	}
	p, _ = v.Profile(Expert)
	if r.Allowed(v, p, 3) {
		t.Fatal("expert never for grade 1–3")
	}
}

// F076-AC-01, AC-02, AC-05: khối, mở khóa (level, quest, vùng), thời gian; loại nhóm và party_required không phải solo.
func TestVisible(t *testing.T) {
	v := version(TypeTime)
	v.Unlock = &Unlock{MinLevel: 3, CompletedQuestIDs: []string{"q1"}, UnlockedAreaIDs: []string{"challenge_mountain"}}
	st := Student{Grade: 3, Level: 3, CompletedQuests: map[string]bool{"q1": true}, OpenAreas: map[string]bool{"challenge_mountain": true}}
	if ok, why := v.Visible(st, t0); !ok {
		t.Fatalf("visible: %s", why)
	}
	cases := map[string]func(*Version, *Student, *time.Time){
		ReasonGrade:   func(_ *Version, s *Student, _ *time.Time) { s.Grade = 2 },
		ReasonLocked:  func(_ *Version, s *Student, _ *time.Time) { s.Level = 2 },
		"quest":       func(_ *Version, s *Student, _ *time.Time) { s.CompletedQuests = nil },
		"area":        func(_ *Version, s *Student, _ *time.Time) { s.OpenAreas = map[string]bool{} },
		ReasonWindow:  func(_ *Version, _ *Student, at *time.Time) { *at = t0.Add(72 * time.Hour) },
		"not started": func(_ *Version, _ *Student, at *time.Time) { *at = t0.Add(-2 * time.Hour) },
	}
	for name, mut := range cases {
		vv, s, at := v, st, t0
		mut(&vv, &s, &at)
		if ok, _ := vv.Visible(s, at); ok {
			t.Errorf("%s: visible", name)
		}
	}
	if !v.Solo() {
		t.Fatal("time is solo")
	}
	for _, typ := range []string{TypeBoss, TypeCoop, TypeParty, TypeTournament} {
		if vv := version(typ); vv.Solo() {
			t.Errorf("%s served solo", typ)
		}
	}
	vv := version(TypeTime)
	vv.Mode = ModePartyRequired
	if vv.Solo() {
		t.Fatal("party_required is not solo")
	}
	vv.Mode = ModePartyOptional
	if !vv.Solo() {
		t.Fatal("party_optional is playable solo")
	}
}

// F076-AC-04: hạn chót theo giờ server = time_limit_seconds (× 1,5 khối 1–3); không giới hạn → nil; sức bền chỉ survival.
func TestDeadlineAndLives(t *testing.T) {
	r := rules()
	v := version(TypeTime)
	if d := r.Deadline(v, 7, t0); d == nil || !d.Equal(t0.Add(300*time.Second)) {
		t.Fatalf("grade 7: %v", d)
	}
	if d := r.Deadline(v, 3, t0); d == nil || !d.Equal(t0.Add(450*time.Second)) {
		t.Fatalf("grade 3 ×1.5: %v", d)
	}
	v.TimeLimitSeconds = 0
	if d := r.Deadline(v, 3, t0); d != nil {
		t.Fatal("no time limit")
	}
	p, _ := v.Profile(Hard)
	if r.Lives(v, p) != nil {
		t.Fatal("lives only in survival")
	}
	s := version(TypeSurvival)
	if l := r.Lives(s, p); l == nil || *l != 3 {
		t.Fatalf("hard survival: %v", l)
	}
	p.Lives = 4
	if l := r.Lives(s, p); *l != 4 {
		t.Fatal("profile lives win")
	}
	// Per-item time of the level (× 1.5 for grades 1–3) for a one-item survival round.
	if s := r.ItemSeconds(p, 3); s != 37 {
		t.Fatalf("hard g3: %d", s)
	}
}

// F076-AC-06: lượt/ngày (version hoặc mặc định) và nghỉ chờ sau failed từ mức Hard (version thắng nếu có).
func TestAttemptsAndCooldown(t *testing.T) {
	r := rules()
	v := version(TypeTime)
	if r.MaxAttempts(v) != 5 {
		t.Fatal("default per day")
	}
	v.Retry = &Retry{MaxAttemptsPerDay: 2}
	if r.MaxAttempts(v) != 2 {
		t.Fatal("version per day")
	}
	if c := r.Cooldown(v, Normal); c != 0 {
		t.Fatalf("normal: %v", c)
	}
	if c := r.Cooldown(v, Legendary); c != time.Minute {
		t.Fatalf("legendary: %v", c)
	}
	zero := 0
	v.Retry.CooldownSecondsAfterFail = &zero
	if c := r.Cooldown(v, Hard); c != 0 {
		t.Fatalf("version cooldown: %v", c)
	}
}

// F076-AC-20: kiểm cấu hình ngoài JSON Schema (min > max, khối 1–3 vượt Hard, mode solo cho loại nhóm, time thiếu
// time_limit_seconds, nguồn question_selection chưa hỗ trợ, goal nhóm cho loại solo) → field; thiếu en → TRANSLATION.
func TestValidate(t *testing.T) {
	r := rules()
	if err := version(TypeTime).Validate(r); err != nil {
		t.Fatal(err)
	}
	field := func(err error) string {
		var fe *FieldError
		if errors.As(err, &fe) {
			return fe.Field
		}
		return ""
	}
	cases := map[string]struct {
		mut   func(*Version)
		field string
	}{
		"min > max":       {func(v *Version) { v.Difficulties[0].Min, v.Difficulties[0].Max = 3, 2 }, "difficulties/0/question_difficulty_min"},
		"expert g1_3":     {func(v *Version) { v.Difficulties[3].GradeLevels = nil }, "difficulties/3/grade_levels"},
		"profile grade":   {func(v *Version) { v.Difficulties[4].GradeLevels = []int{8} }, "difficulties/4/grade_levels"},
		"level twice":     {func(v *Version) { v.Difficulties[1].Level = Easy }, "difficulties/1/level"},
		"boss solo":       {func(v *Version) { v.Type = TypeBoss; v.Mode = ModeSolo }, "mode"},
		"time no limit":   {func(v *Version) { v.TimeLimitSeconds = 0 }, "time_limit_seconds"},
		"question set":    {func(v *Version) { v.Source = Source{Kind: SourceQuestionSet, QuestionSetID: v.SubjectID} }, "activity_source/kind"},
		"bad activity":    {func(v *Version) { v.Source.ActivityRefs[1].ActivityID = "x" }, "activity_source/activity_refs/1/activity_id"},
		"collective goal": {func(v *Version) { v.Goal.Metric = MetricCollective }, "goal/metric"},
		"accuracy cycle":  {func(v *Version) { v.Goal = Goal{Metric: MetricAccuracy, Target: 0.9} }, "goal/metric"},
		"window":          {func(v *Version) { e := v.Availability.StartAt.Add(-time.Hour); v.Availability.EndAt = &e }, "availability/end_at"},
	}
	for name, c := range cases {
		v := version(TypeTime)
		v.Difficulties = five()
		v.Source.ActivityRefs = slices.Clone(v.Source.ActivityRefs)
		c.mut(&v)
		err := v.Validate(r)
		if !errors.Is(err, ErrInvalid) || field(err) != c.field {
			t.Errorf("%s: %v (field %q)", name, err, field(err))
		}
	}
	p := version(TypePerfect)
	p.Goal = Goal{Metric: MetricAccuracy, Target: 1}
	if err := p.Validate(r); err != nil {
		t.Fatalf("perfect accuracy: %v", err)
	}
	s := version(TypeSeasonal)
	s.Availability = nil
	if field(s.Validate(r)) != "availability" {
		t.Fatal("seasonal needs a window")
	}
	m := version(TypeTime)
	m.Name.EN = ""
	if err := m.Validate(r); !errors.Is(err, ErrTranslationIncomplete) {
		t.Fatalf("missing en: %v", err)
	}
}

func TestParse(t *testing.T) {
	raw := []byte(`{"schema_version":1,"challenge_id":"0193a5f4-6c1e-7b2a-9d1f-000000076101","code":"chl.math.g3","challenge_type":"survival",
	"name":{"vi":"Sinh tồn","en":"Survival"},"grade_levels":[3],"subject_id":"0193a5f4-6c1e-7b2a-9d1f-000000000201",
	"objective_ids":["0193a5f4-6c1e-7b2a-9d1f-000000000301"],"difficulties":[{"level":"easy","question_difficulty_min":1,
	"question_difficulty_max":2,"reward_multiplier":1}],"activity_source":{"kind":"activity_ref_list","activity_refs":[{"activity_type":"quiz",
	"activity_id":"0193a5f4-6c1e-7b2a-9d1f-000000030001"}]},"goal":{"metric":"rounds_survived","target":5},"time_limit_seconds":600,"rewards":{}}`)
	v, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != TypeSurvival || v.Mode != ModeSolo || len(v.Source.ActivityRefs) != 1 || v.Goal.Target != 5 {
		t.Fatalf("parsed: %+v", v)
	}
	if _, err := Parse([]byte(`[]`)); !errors.Is(err, ErrInvalid) {
		t.Fatal("not an object")
	}
}
