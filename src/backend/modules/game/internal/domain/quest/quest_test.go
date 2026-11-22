package quest

import (
	"errors"
	"testing"
	"time"
)

func intp(v int) *int           { return &v }
func floatp(v float64) *float64 { return &v }
func boolp(v bool) *bool        { return &v }

var day = time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC) // 10:00 ICT

func txt(s string) Text { return Text{VI: s, EN: s} }

// sample is a learning quest of grade 3: talk_npc → go_to → activity → collect (optional) → talk_npc.
func sample() Version {
	return Version{SchemaVersion: 2, QuestID: "0193a5f4-6c1e-7b2a-9d1f-000000072001", Code: "math.g3.market", OwnerScope: "platform",
		QuestType: TypeLearning, Name: txt("Chợ"), Description: txt("Mô tả"), GradeLevels: []int{3}, SubjectID: "0193a5f4-6c1e-7b2a-9d1f-0000000000aa",
		SubjectCode: "math", ObjectiveIDs: []string{"0193a5f4-6c1e-7b2a-9d1f-0000000000b1"}, StartTrigger: Trigger{Kind: TriggerNPC, NPCID: "npc.mr_numbers"},
		PartyMode: "solo", CompletionRule: CompletionRule{Type: CompleteAllRequired},
		Steps: []Step{
			{ID: "meet", Order: 1, Type: StepTalkNPC, Title: txt("Gặp"), NPCID: "npc.mr_numbers", AreaID: "learning_zone.math"},
			{ID: "stall", Order: 2, Type: StepGoTo, Title: txt("Đến sạp"), AreaID: "learning_zone.math", InteractableID: "quest.math.fruit_stall"},
			{ID: "solve", Order: 3, Type: StepActivity, Title: txt("Giải"), ContentRef: &ContentRef{Kind: "activity", ID: "0193a5f4-6c1e-7b2a-9d1f-0000000000c1"},
				PassRule: &PassRule{MinCorrectRatio: floatp(0.75), MinQuestions: intp(3)}},
			{ID: "apples", Order: 4, Type: StepCollect, Title: txt("Táo"), Optional: true, AreaID: "learning_zone.math",
				InteractableIDs: []string{"quest.math.apple_1", "quest.math.apple_2"}, Count: 2},
			{ID: "back", Order: 5, Type: StepTalkNPC, Title: txt("Báo lại"), NPCID: "npc.mr_numbers"},
		}}
}

var math = Place{AreaID: "learning_zone.math", NPCs: []string{"npc.mr_numbers"},
	Interactables: []string{"npc.mr_numbers", "quest.math.fruit_stall", "quest.math.apple_1", "quest.math.apple_2"}}

func start(v Version) *Progress { return NewProgress(v, Start{At: day}) }

// T011 (F072-AC-01, AC-05): grade, subject, availability window, prerequisites, learning requirements; a mandatory
// learning quest is never blocked by min_level.
func TestDecide(t *testing.T) {
	v := sample()
	f := Facts{CompletedQuests: map[string]bool{}, OpenAreas: map[string]bool{}, Mastery: map[string]string{}, Discovered: map[string]bool{}}
	if d := Decide(v, Student{Grade: 3}, f, day); !d.Visible || !d.Met {
		t.Fatalf("grade 3: %+v", d)
	}
	if d := Decide(v, Student{Grade: 4}, f, day); d.Visible {
		t.Fatal("grade 4 sees a grade 3 quest")
	}
	if d := Decide(v, Student{Grade: 3, EnabledSubjects: []string{"vietnamese"}}, f, day); d.Visible {
		t.Fatal("math off at the school")
	}
	end := day.Add(-time.Hour)
	v.Availability = &Availability{EndAt: &end}
	if d := Decide(v, Student{Grade: 3}, f, day); d.Visible {
		t.Fatal("availability ended")
	}
	v.Availability = nil
	v.Prerequisites = &Prerequisites{MinLevel: 5, CompletedQuestIDs: []string{"q-prev"}, UnlockedAreaIDs: []string{"arena"}}
	v.LearningRequirements = []LearningRequirement{{ObjectiveID: "lo-1", MinMasteryLevel: "developing"}}
	d := Decide(v, Student{Grade: 3}, f, day)
	if !d.Visible || d.Met || len(d.Missing()) != 3 {
		t.Fatalf("missing prerequisites (min_level skipped for a learning quest): %+v", d.Missing())
	}
	for _, r := range d.Requirements {
		if r.Kind == ReqMinLevel {
			t.Fatal("min_level blocks a mandatory learning quest")
		}
	}
	v.QuestType = TypeSide
	if d := Decide(v, Student{Grade: 3}, f, day); len(d.Missing()) != 4 {
		t.Fatalf("side quest keeps min_level: %+v", d.Missing())
	}
	f.CompletedQuests["q-prev"], f.OpenAreas["arena"], f.Level, f.Mastery["lo-1"] = true, true, 5, "proficient"
	if d := Decide(v, Student{Grade: 3}, f, day); !d.Met {
		t.Fatalf("all met: %+v", d.Missing())
	}
	v.StartTrigger = Trigger{Kind: TriggerDiscovery, InteractableIDs: []string{"discovery.math.old_well"}}
	if d := Decide(v, Student{Grade: 3}, f, day); d.Visible {
		t.Fatal("secret quest visible before its discovery")
	}
	f.Discovered["discovery.math.old_well"] = true
	if d := Decide(v, Student{Grade: 3}, f, day); !d.Visible {
		t.Fatal("secret quest after its discovery")
	}
}

// F072-AC-09, AC-10, AC-11, AC-16, AC-19: steps open by order, reports are checked against the place, collect counts
// each interactable once, a learning step never completes from a report, the optional step never blocks.
func TestProgressSteps(t *testing.T) {
	v, reg := sample(), DefaultRegistry()
	p := start(v)
	if p.State("meet").Status != StepActive || p.State("stall").Status != StepLocked {
		t.Fatalf("initial: %+v", p.Steps)
	}
	if _, err := p.Report(v, reg, "stall", Report{InteractableID: "quest.math.fruit_stall"}, math, day); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("locked step: %v", err)
	}
	if _, err := p.Report(v, reg, "meet", Report{NPCID: "npc.mr_numbers"}, Place{AreaID: "main_hub", NPCs: []string{"npc.cloudy"}}, day); !errors.Is(err, ErrPrecondition) {
		t.Fatalf("wrong area: %v", err)
	}
	o, err := p.Report(v, reg, "meet", Report{NPCID: "npc.mr_numbers"}, math, day)
	if err != nil || !o.StepCompleted || len(o.Activated) != 1 || o.Activated[0] != "stall" {
		t.Fatalf("meet: %+v %v", o, err)
	}
	if _, err := p.Report(v, reg, "meet", Report{NPCID: "npc.mr_numbers"}, math, day); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("completed step: %v", err)
	}
	if _, err := p.Report(v, reg, "stall", Report{InteractableID: "quest.math.apple_1"}, math, day); !errors.Is(err, ErrPrecondition) {
		t.Fatalf("interactable of another step: %v", err)
	}
	if _, err := p.Report(v, reg, "stall", Report{InteractableID: "quest.math.fruit_stall"}, math, day); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Report(v, reg, "solve", Report{}, math, day); !errors.Is(err, ErrPrecondition) {
		t.Fatalf("report of a learning step: %v", err)
	}
	if _, err := p.CanStartActivity(v, reg, "solve"); err != nil {
		t.Fatal(err)
	}
	if o := p.ActivityResult(v, "solve", false, day); o.Changed || p.State("solve").Status != StepActive {
		t.Fatalf("not passed: %+v", o)
	}
	o = p.ActivityResult(v, "solve", true, day)
	if !o.StepCompleted || p.State("apples").Status != StepActive || p.State("back").Status != StepActive {
		t.Fatalf("after solve the optional step and the next are active: %+v", p.Steps)
	}
	o1, _ := p.Report(v, reg, "apples", Report{InteractableID: "quest.math.apple_1"}, math, day)
	o2, _ := p.Report(v, reg, "apples", Report{InteractableID: "quest.math.apple_1"}, math, day)
	if !o1.Changed || o2.Changed || p.State("apples").Count != 1 {
		t.Fatalf("collect once: %+v %+v %+v", o1, o2, p.State("apples"))
	}
	o, err = p.Report(v, reg, "back", Report{NPCID: "npc.mr_numbers"}, math, day)
	if err != nil || !o.Completed || p.Status != StatusCompleted || p.CompletedAt == nil {
		t.Fatalf("quest completed without the optional step: %+v %v", o, err)
	}
	if err := p.Abandon(day); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("abandon a completed quest: %v", err)
	}
}

// F072-AC-12: a discover step advances from discoveries only; any_n_of completes on n steps.
func TestDiscoverAndAnyN(t *testing.T) {
	v := sample()
	v.QuestType = TypeExploration
	v.Steps = []Step{
		{ID: "find", Order: 1, Type: StepDiscover, Title: txt("Tìm"), AreaID: "learning_zone.math", Count: 2,
			InteractableIDs: []string{"discovery.math.old_well", "discovery.math.number_bridge"}},
		{ID: "talk", Order: 1, Type: StepTalkNPC, Title: txt("Nói"), NPCID: "npc.mr_numbers"},
	}
	v.CompletionRule = CompletionRule{Type: CompleteAnyN, StepIDs: []string{"find", "talk"}, N: 1}
	reg := DefaultRegistry()
	p := start(v)
	if _, err := p.Report(v, reg, "find", Report{InteractableID: "discovery.math.old_well"}, math, day); !errors.Is(err, ErrPrecondition) {
		t.Fatalf("report of discover: %v", err)
	}
	if out := p.Discovery(v, reg, "main_hub", "discovery.math.old_well", day); len(out) != 0 {
		t.Fatal("other area")
	}
	p.Discovery(v, reg, "learning_zone.math", "discovery.math.old_well", day)
	p.Discovery(v, reg, "learning_zone.math", "discovery.math.old_well", day)
	if p.State("find").Count != 1 || p.Status != StatusActive {
		t.Fatalf("once: %+v", p.State("find"))
	}
	out := p.Discovery(v, reg, "learning_zone.math", "discovery.math.number_bridge", day)
	if len(out) != 1 || !out[0].Completed || p.Status != StatusCompleted {
		t.Fatalf("any_n_of: %+v %s", out, p.Status)
	}
}

// F072-AC-26: reset_step resets only the step of the condition (or the active steps) and restarts the time limit.
func TestFailurePolicy(t *testing.T) {
	v, reg := sample(), DefaultRegistry()
	v.Availability = &Availability{TimeLimitMinutes: intp(30)}
	p := start(v)
	if p.DeadlineAt == nil || !p.DeadlineAt.Equal(day.Add(30*time.Minute)) {
		t.Fatalf("deadline: %v", p.DeadlineAt)
	}
	if f, ok := v.Failure(FailTimeLimit); !ok || f.Policy() != OnFailReset {
		t.Fatalf("default time limit condition: %+v", f)
	}
	_, _ = p.Report(v, reg, "meet", Report{NPCID: "npc.mr_numbers"}, math, day)
	later := day.Add(time.Hour)
	if reset := p.ResetSteps(v, "", later); len(reset) != 1 || reset[0] != "stall" || p.State("meet").Status != StepCompleted {
		t.Fatalf("reset: %v %+v", reset, p.Steps)
	}
	if !p.DeadlineAt.Equal(later.Add(30 * time.Minute)) {
		t.Fatal("time limit restarts")
	}
	if err := p.Fail(later); err != nil || p.Status != StatusFailed || p.FailedAt == nil {
		t.Fatalf("fail: %v", err)
	}
}

// F072-AC-25: once, per_period, repeatable with cooldown and max_completions.
func TestCanRepeat(t *testing.T) {
	v := sample()
	if r, _ := CanRepeat(v, History{Completed: 1}, day); r != RepeatAlreadyCompleted {
		t.Fatal("once")
	}
	v.Repeatability = &Repeatability{Mode: RepeatRepeatable, CooldownHours: 24, MaxCompletions: 3}
	last := day.Add(-time.Hour)
	r, at := CanRepeat(v, History{Completed: 1, LastCompletedAt: &last}, day)
	if r != RepeatCooldown || at == nil || !at.Equal(last.Add(24*time.Hour)) {
		t.Fatalf("cooldown: %s %v", r, at)
	}
	old := day.Add(-25 * time.Hour)
	if r, _ := CanRepeat(v, History{Completed: 2, LastCompletedAt: &old}, day); r != RepeatOK {
		t.Fatal("after cooldown")
	}
	if r, _ := CanRepeat(v, History{Completed: 3, LastCompletedAt: &old}, day); r != RepeatAlreadyCompleted {
		t.Fatal("max_completions")
	}
	v.Repeatability = nil
	v.QuestType = TypeDaily
	if r, _ := CanRepeat(v, History{Completed: 5, CompletedInPeriod: false}, day); r != RepeatOK {
		t.Fatal("per_period next period")
	}
	if r, _ := CanRepeat(v, History{Completed: 5, CompletedInPeriod: true}, day); r != RepeatAlreadyCompleted {
		t.Fatal("per_period same period")
	}
}

// Pass rule (QUEST_SYSTEM §3.1) and its CONFIG_DEFAULTS default quest_pass_rule.
func TestPassRule(t *testing.T) {
	d := DefaultPassRule{MinCorrectRatio: 0.7, MinQuestions: 8, MaxAttemptsPerDay: 5}
	e := d.Effective(&PassRule{MinCorrectRatio: floatp(0.75), MinQuestions: intp(4)})
	if e.MaxAttemptsPerDay != 5 || e.MinQuestions != 4 {
		t.Fatalf("effective: %+v", e)
	}
	if !e.Passes(Graded{CorrectCount: intp(3), ItemCount: intp(4)}) || e.Passes(Graded{CorrectCount: intp(2), ItemCount: intp(4)}) ||
		e.Passes(Graded{CorrectCount: intp(3), ItemCount: intp(3)}) {
		t.Fatal("ratio and min_questions")
	}
	if !e.Passes(Graded{Passed: boolp(true)}) || e.Passes(Graded{}) {
		t.Fatal("activity without items")
	}
}

// PeriodKey/PeriodEnd: daily reset 04:00 ICT, weekly Monday 04:00 ICT.
func TestPeriods(t *testing.T) {
	r := Reset{Hour: 4, Location: time.FixedZone("ICT", 7*3600)}
	early := time.Date(2026, 11, 22, 20, 30, 0, 0, time.UTC) // 2026-11-23 03:30 ICT (Monday) → still Sunday's period
	if k := r.PeriodKey(RecurrenceDaily, early); k != "2026-11-22" {
		t.Fatalf("daily before reset: %s", k)
	}
	if e := r.PeriodEnd(RecurrenceDaily, early); !e.Equal(time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)) {
		t.Fatalf("daily end: %v", e)
	}
	if k := r.PeriodKey(RecurrenceWeekly, early); k != "2026-W47" {
		t.Fatalf("weekly before Monday reset: %s", k)
	}
	if k := r.PeriodKey(RecurrenceWeekly, early.Add(time.Hour)); k != "2026-W48" {
		t.Fatalf("weekly after Monday reset: %s", k)
	}
	if e := r.PeriodEnd(RecurrenceWeekly, day); !e.Equal(time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)) {
		t.Fatalf("weekly end: %v", e)
	}
}

// Periodic selection favours weak objectives and is stable.
func TestSelectPeriodic(t *testing.T) {
	c := []Candidate{{QuestID: "a", ObjectiveIDs: []string{"o1"}}, {QuestID: "b", ObjectiveIDs: []string{"o2"}}, {QuestID: "c"}}
	got := SelectPeriodic(c, map[string]string{"o2": "beginning"}, 2, "s:2026-11-22")
	if len(got) != 2 || got[0] != "b" {
		t.Fatalf("weak first: %v", got)
	}
	if again := SelectPeriodic(c, map[string]string{"o2": "beginning"}, 2, "s:2026-11-22"); again[1] != got[1] {
		t.Fatal("stable")
	}
}

// F072-AC-27 (structure and references): translations, handlers, learning blocks, objectives, references.
func TestValidate(t *testing.T) {
	cfg := ValidatorConfig{PassRule: DefaultPassRule{MinCorrectRatio: 0.7, MinQuestions: 8, MaxAttemptsPerDay: 5},
		BlockMaxByBand: map[string]int{"g1_3": 3, "g4_5": 5, "g6_9": 8}}
	cat := Catalog{Areas: map[string]AreaRef{"learning_zone.math": {NPCs: []string{"npc.mr_numbers"}, Interactables: map[string]string{
		"quest.math.fruit_stall": "quest_object", "quest.math.apple_1": "quest_object", "quest.math.apple_2": "quest_object"}}},
		Chapters: map[string]bool{}, Quests: map[string]bool{}}
	if err := Validate(sample(), cfg, cat); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		edit func(v *Version)
		want error
	}{
		"missing en":      {func(v *Version) { v.Steps[0].Title.EN = "" }, ErrTranslationIncomplete},
		"boss step":       {func(v *Version) { v.Steps[1].Type = StepBoss }, ErrInvalid},
		"lesson content":  {func(v *Version) { v.Steps[2].ContentRef.Kind = "lesson" }, ErrInvalid},
		"no objectives":   {func(v *Version) { v.ObjectiveIDs = nil }, ErrInvalid},
		"no subject code": {func(v *Version) { v.SubjectCode = "" }, ErrInvalid},
		"learning block":  {func(v *Version) { v.Steps[2].PassRule.MinQuestions = intp(4) }, ErrInvalid},
		"unknown npc":     {func(v *Version) { v.Steps[0].NPCID = "npc.ghost" }, ErrRefInvalid},
		"unknown area":    {func(v *Version) { v.Steps[1].AreaID = "nowhere" }, ErrRefInvalid},
		"unknown anchor":  {func(v *Version) { v.Steps[1].InteractableID = "quest.math.none" }, ErrRefInvalid},
		"unknown chapter": {func(v *Version) { v.StoryChapterCode = "chapter.9" }, ErrRefInvalid},
		"cycle":           {func(v *Version) { v.Steps[0].DependsOn = []string{"back"} }, ErrInvalid},
		"bad any_n_of": {func(v *Version) {
			v.CompletionRule = CompletionRule{Type: CompleteAnyN, StepIDs: []string{"meet"}, N: 2}
		}, ErrInvalid},
		"unknown prereq":   {func(v *Version) { v.Prerequisites = &Prerequisites{CompletedQuestIDs: []string{"x"}} }, ErrRefInvalid},
		"duplicated step":  {func(v *Version) { v.Steps[1].ID = "meet" }, ErrInvalid},
		"collect count":    {func(v *Version) { v.Steps[3].Count = 3 }, ErrInvalid},
		"schema version 1": {func(v *Version) { v.SchemaVersion = 1 }, ErrInvalid},
	}
	for name, c := range cases {
		v := sample()
		v.Steps = append([]Step(nil), v.Steps...)
		cr := *v.Steps[2].ContentRef
		pr := *v.Steps[2].PassRule
		v.Steps[2].ContentRef, v.Steps[2].PassRule = &cr, &pr
		c.edit(&v)
		if err := Validate(v, cfg, cat); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}
