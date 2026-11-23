package liveops

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var t0 = time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)

// F077-AC-01, AC-03, AC-05 (T011): the run state machine — the scheduler moves scheduled → live → ended → archived at
// its times, cancelled → archived at once, a suspended run waits; an operator suspends a live run, resumes a suspended
// run, cancels a scheduled/live/suspended run with a reason of 3..1000 characters; anything else is refused.
func TestRunStateMachine(t *testing.T) {
	r := Run{Status: StatusScheduled, StartsAt: t0, EndsAt: t0.Add(48 * time.Hour), ClaimUntil: t0.Add(120 * time.Hour)}
	if r.Due(t0.Add(-time.Second)) != "" || r.Due(t0) != StatusLive {
		t.Fatal("scheduled → live at starts_at")
	}
	r.Status = StatusLive
	if r.Due(t0.Add(47*time.Hour)) != "" || r.Due(r.EndsAt) != StatusEnded {
		t.Fatal("live → ended at ends_at")
	}
	r.Status = StatusEnded
	if r.Due(r.EndsAt) != "" || r.Due(r.ClaimUntil) != StatusArchived {
		t.Fatal("ended → archived at claim_until")
	}
	if (Run{Status: StatusCancelled}).Due(t0) != StatusArchived || (Run{Status: StatusSuspended, EndsAt: t0}).Due(t0.Add(time.Hour)) != "" ||
		(Run{Status: StatusArchived}).Due(t0) != "" {
		t.Fatal("cancelled → archived; suspended and archived wait")
	}
	ok := []struct{ from, action, to string }{
		{StatusLive, ActionSuspend, StatusSuspended}, {StatusSuspended, ActionResume, StatusLive}, {StatusScheduled, ActionCancel, StatusCancelled},
		{StatusLive, ActionCancel, StatusCancelled}, {StatusSuspended, ActionCancel, StatusCancelled},
	}
	for _, c := range ok {
		if to, err := Operate(c.from, c.action, "Sự cố hiển thị"); err != nil || to != c.to {
			t.Fatalf("%s %s: %s %v", c.from, c.action, to, err)
		}
	}
	bad := []struct{ from, action string }{
		{StatusScheduled, ActionSuspend}, {StatusLive, ActionResume}, {StatusEnded, ActionCancel}, {StatusArchived, ActionCancel},
		{StatusCancelled, ActionResume}, {StatusSuspended, ActionSuspend},
	}
	for _, c := range bad {
		if _, err := Operate(c.from, c.action, "Sự cố hiển thị"); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("%s %s: %v", c.from, c.action, err)
		}
	}
	long := make([]rune, 1001)
	for i := range long {
		long[i] = 'a'
	}
	for _, reason := range []string{"", "  ab ", string(long)} {
		if _, err := Operate(StatusLive, ActionSuspend, reason); !errors.Is(err, ErrReason) {
			t.Fatalf("reason %d: %v", len(reason), err)
		}
	}
	if EventFor(StatusScheduled, StatusLive) != "event.started" || EventFor(StatusSuspended, StatusLive) != "event.resumed" ||
		EventFor(StatusLive, StatusCancelled) != "event.cancelled" {
		t.Fatal("events of the transitions")
	}
}

// F077-AC-07: who sees a run — the audience (tenant, school, grade; never a class) and the visible statuses.
func TestAudienceAndVisibility(t *testing.T) {
	ta, tb, sa, sb := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	a := Audience{Scope: ScopeSchool, SchoolIDs: []uuid.UUID{sa}, GradeLevels: []int{3, 4, 5}}
	if !a.Includes(&ta, ta, sa, 3) || a.Includes(&ta, ta, sb, 3) || a.Includes(&ta, ta, sa, 7) || a.Includes(&ta, tb, sa, 3) {
		t.Fatal("tenant event of school sa, grades 3–5")
	}
	p := Audience{Scope: ScopePlatform, GradeLevels: []int{3}}
	if !p.Includes(nil, tb, sb, 3) || p.Includes(nil, tb, sb, 4) {
		t.Fatal("platform audience: every tenant of grade 3")
	}
	p.TenantIDs = []uuid.UUID{ta}
	if p.Includes(nil, tb, sb, 3) || !p.Includes(nil, ta, sa, 3) {
		t.Fatal("platform audience of listed tenants")
	}
	if got := p.Tenants(nil, []uuid.UUID{ta, tb}); len(got) != 1 || got[0] != ta {
		t.Fatalf("tenants: %v", got)
	}
	r := Run{Status: StatusScheduled, StartsAt: t0.Add(10 * 24 * time.Hour), EndsAt: t0.Add(20 * 24 * time.Hour)}
	if Visible(r, t0, 7*24*time.Hour) || !Visible(r, t0.Add(3*24*time.Hour), 7*24*time.Hour) {
		t.Fatal("upcoming 7 days before starts_at")
	}
	if !Visible(Run{Status: StatusEnded, ClaimUntil: t0.Add(time.Hour)}, t0, 0) || Visible(Run{Status: StatusEnded, ClaimUntil: t0}, t0, 0) ||
		Visible(Run{Status: StatusArchived}, t0, 0) || Visible(Run{Status: StatusCancelled}, t0, 0) || !Visible(Run{Status: StatusSuspended}, t0, 0) {
		t.Fatal("visible statuses")
	}
	if err := (Audience{Scope: ScopePlatform, GradeLevels: []int{3}}).Validate(OwnerTenant); err == nil {
		t.Fatal("a tenant event cannot have a platform audience")
	}
}

// F077-AC-09, AC-11, AC-13: points under the daily cap, milestones never lowered, the goal metrics (no contribution from a
// suspicious_speed attempt) and the goal milestones reached once.
func TestPointsMilestonesAndGoal(t *testing.T) {
	if c, capped := Earn(30, 180, 200); c != 20 || !capped {
		t.Fatalf("cap: %d %v", c, capped)
	}
	if c, capped := Earn(30, 200, 200); c != 0 || !capped {
		t.Fatal("cap reached")
	}
	if c, capped := Earn(30, 0, 200); c != 30 || capped {
		t.Fatal("under cap")
	}
	ms := []Milestone{{MilestoneID: "m1", EventPoints: 50}, {MilestoneID: "m2", EventPoints: 150}}
	if got := Reached(ms, 60, nil); len(got) != 1 || got[0] != "m1" {
		t.Fatalf("reached: %v", got)
	}
	raised := []Milestone{{MilestoneID: "m1", EventPoints: 100}, {MilestoneID: "m2", EventPoints: 150}}
	if got := Reached(raised, 60, []string{"m1"}); len(got) != 1 || got[0] != "m1" {
		t.Fatal("a milestone reached under an older version stays reached")
	}
	if got := Unclaimed([]string{"m1", "m2"}, []string{"m1"}, nil); len(got) != 1 || got[0] != "m2" {
		t.Fatalf("unclaimed: %v", got)
	}
	q := uuid.New()
	f := Fact{EventType: "quest.completed", QuestID: &q, QuestType: "daily"}
	codes := f.SourceCodes(func(id uuid.UUID) bool { return id == q })
	if code, pts := PointsFor([]Source{{SrcQuestCompleted, 10}, {SrcDailyQuest, 20}, {SrcEventActivity, 15}}, codes); code != SrcDailyQuest || pts != 20 {
		t.Fatalf("best source: %s %d", code, pts)
	}
	g := Fact{EventType: "activity.graded", Correct: 4}
	if g.MetricAmount("correct_answers", nil) != 4 || g.MetricAmount("activities_completed", nil) != 1 || g.MetricAmount("quests_completed", nil) != 0 {
		t.Fatal("metrics of a graded activity")
	}
	g.RapidGuess = true
	if g.MetricAmount("correct_answers", nil) != 0 || len(g.SourceCodes(nil)) != 0 {
		t.Fatal("a suspicious_speed attempt never contributes")
	}
	sub := uuid.New()
	g = Fact{EventType: "activity.graded", Correct: 2, SubjectID: &sub}
	if g.MetricAmount("correct_answers", &MetricFilter{SubjectIDs: []uuid.UUID{uuid.New()}}) != 0 ||
		g.MetricAmount("correct_answers", &MetricFilter{SubjectIDs: []uuid.UUID{sub}}) != 2 {
		t.Fatal("subject filter")
	}
	if Threshold(10, 25) != 3 || Threshold(100, 50) != 50 {
		t.Fatal("threshold ceil")
	}
	if got := Crossed(100, 60, []int{25, 50, 75, 100}, []int{25}); len(got) != 1 || got[0] != 50 {
		t.Fatalf("crossed: %v", got)
	}
	rows := VisibleGrades([]GradeRow{{Grade: 3, Contributors: 9}, {Grade: 5, Contributors: 4}, {Grade: 4, Contributors: 5, Progress: 30}}, 5)
	if len(rows) != 1 || rows[0].Grade != 4 {
		t.Fatalf("grades 4–9 with ≥ 5 contributors: %+v", rows)
	}
}

func platformEvent() Definition {
	cap := 200
	return Definition{EventID: uuid.New(), Code: "event.lantern", OwnerScope: OwnerPlatform, Kind: KindWorldFestival,
		Name: Localized{"vi": "Lễ hội Trăng Rằm", "en": "Moon Festival"}, Description: Localized{"vi": "Mô tả", "en": "Description"},
		Schedule:         Schedule{StartAt: t0, EndAt: t0.Add(10 * 24 * time.Hour), Timezone: Timezone},
		Audience:         Audience{Scope: ScopePlatform, GradeLevels: []int{3, 4, 5}},
		EventPoints:      &Points{Enabled: true, DailyCap: cap, Sources: []Source{{SrcQuestCompleted, 20}}},
		MilestoneRewards: []Milestone{{MilestoneID: "m1", EventPoints: 50, Rewards: []Reward{{"type": "currency", "currency_code": "xu", "amount": 20.0}}}}}
}

var policy = Policy{FestivalMinDays: 7, ExtraDailyQuestsMax: 2, PointsDailyCap: 200, BundleMaxBytes: 20 * 1024 * 1024, GoalDailyCap: 50}

// F077-AC-26, AC-27, AC-28 (T034): bilingual platform text, healthy rules (festival ≥ 7 days, no check-in or streak
// source, no level gate on a learning event, at most 2 extra daily quests), decoration budget and Low preset.
func TestEventHealthRules(t *testing.T) {
	if err := platformEvent().CheckTranslations(); err != nil {
		t.Fatal(err)
	}
	if err := platformEvent().Validate(policy, 2); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(d *Definition) (error, error){
		"missing en": func(d *Definition) (error, error) {
			d.Name = Localized{"vi": "Lễ hội"}
			return d.CheckTranslations(), ErrTranslationIncomplete
		},
		"missing tagline en": func(d *Definition) (error, error) {
			d.ShortTagline = Localized{"vi": "Vui"}
			return d.CheckTranslations(), ErrTranslationIncomplete
		},
		"short festival": func(d *Definition) (error, error) {
			d.Schedule.EndAt = t0.Add(5 * 24 * time.Hour)
			return d.Validate(policy, 0), ErrInvalid
		},
		"check-in source": func(d *Definition) (error, error) {
			d.EventPoints.Sources = []Source{{"daily_login", 10}}
			return d.Validate(policy, 0), ErrInvalid
		},
		"streak source": func(d *Definition) (error, error) {
			d.EventPoints.Sources = []Source{{"streak.day", 10}}
			return d.Validate(policy, 0), ErrInvalid
		},
		"level gate on learning": func(d *Definition) (error, error) {
			d.Kind = KindLearningCampaign
			lvl := 10
			d.Eligibility = &Eligibility{MinLevel: &lvl}
			return d.Validate(policy, 0), ErrInvalid
		},
		"extra daily quests": func(d *Definition) (error, error) { return d.Validate(policy, 3), ErrInvalid },
		"xp reward": func(d *Definition) (error, error) {
			d.MilestoneRewards[0].Rewards = []Reward{{"type": "xp", "amount": 50.0}}
			return d.Validate(policy, 0), ErrInvalid
		},
		"bundle over budget": func(d *Definition) (error, error) {
			d.Components.Decoration = &Decoration{BundleID: "lantern", LowPresetBundleID: "lantern_low", BundleSizeBytes: 21 * 1024 * 1024,
				Areas: []string{"main_hub"}}
			return d.Validate(policy, 0), ErrInvalid
		},
		"no low preset": func(d *Definition) (error, error) {
			d.Components.Decoration = &Decoration{BundleID: "lantern", BundleSizeBytes: 1024, Areas: []string{"main_hub"}}
			return d.Validate(policy, 0), ErrInvalid
		},
	}
	for name, c := range cases {
		d := platformEvent()
		if err, want := c(&d); !errors.Is(err, want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := platformEvent()
	tenant.OwnerScope, tenant.Kind = OwnerTenant, KindSchool
	tenant.Name = Localized{"vi": "Tuần lễ Toán học"}
	if err := tenant.CheckTranslations(); err != nil {
		t.Fatal("a tenant event needs vi only")
	}
}

// F077-AC-11: a new version of a live run applies at 04:00 ICT the next day; of a run that is not live, at once.
func TestRuleChangeEffective(t *testing.T) {
	at := time.Date(2026, 11, 20, 15, 0, 0, 0, ICT)
	if got := RuleChangeEffective(at, true, 4); !got.Equal(time.Date(2026, 11, 21, 4, 0, 0, 0, ICT)) {
		t.Fatalf("live: %v", got)
	}
	early := time.Date(2026, 11, 20, 2, 0, 0, 0, ICT)
	if got := RuleChangeEffective(early, true, 4); !got.Equal(time.Date(2026, 11, 21, 4, 0, 0, 0, ICT)) {
		t.Fatalf("next day even before 04:00: %v", got)
	}
	if got := RuleChangeEffective(at, false, 4); !got.Equal(at) {
		t.Fatal("not live: at once")
	}
}

// F077-AC-22: a school event takes only the parameters of its template (options, duration, reward option).
func TestTemplateParams(t *testing.T) {
	q1, q2 := uuid.New(), uuid.New()
	tpl := Template{TemplateID: uuid.New(), Code: "tpl.math_week", Kind: KindSchool, Name: Localized{"vi": "Tuần lễ", "en": "Week"},
		AllowedParams: []string{ParamDescription, ParamGoalTarget, ParamQuests, ParamRewardOption}, MaxDurationDays: 14,
		QuestOptions: []uuid.UUID{q1}, RewardOptions: []RewardOption{{RewardOptionID: "badge", Label: Localized{"vi": "Huy hiệu", "en": "Badge"},
			Rewards: []Reward{{"type": "badge", "badge_id": "badge_school_01"}}}},
		CommunityGoal: TemplateGoal{GoalCode: "solve", Metric: "correct_answers", Scope: ScopeSchool, DefaultTarget: 1000,
			Milestones: []GoalMilestone{{Percent: 50}, {Percent: 100}}}}
	if err := tpl.Validate(); err != nil {
		t.Fatal(err)
	}
	opt := "badge"
	p := SchoolParams{Name: Localized{"vi": "Tuần lễ Toán học"}, StartsAt: t0, EndsAt: t0.Add(7 * 24 * time.Hour), QuestIDs: []uuid.UUID{q1},
		RewardOption: &opt, Set: map[string]bool{ParamQuests: true, ParamRewardOption: true}}
	if err := tpl.Check(p, 28); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(p *SchoolParams){
		"param outside":   func(p *SchoolParams) { p.Set[ParamChallenges] = true },
		"quest outside":   func(p *SchoolParams) { p.QuestIDs = []uuid.UUID{q2} },
		"reward outside":  func(p *SchoolParams) { o := "other"; p.RewardOption = &o },
		"too long":        func(p *SchoolParams) { p.EndsAt = t0.Add(15 * 24 * time.Hour) },
		"grades outside":  func(p *SchoolParams) { p.Set[ParamGrades] = true },
		"schools outside": func(p *SchoolParams) { p.Set[ParamSchools] = true },
	}
	for name, edit := range bad {
		c := p
		c.Set = map[string]bool{ParamQuests: true, ParamRewardOption: true}
		edit(&c)
		if err := tpl.Check(c, 28); !errors.Is(err, ErrParamNotAllowed) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tn := uuid.New()
	d := tpl.Build(uuid.New(), tn, "school.math_week", 1, p, []int{3})
	if d.OwnerScope != OwnerTenant || *d.TenantID != tn || d.Components.CommunityGoal.Target != 1000 || len(d.Components.CommunityGoal.Milestone(100).Rewards) != 1 {
		t.Fatalf("built: %+v", d)
	}
	if err := d.Validate(policy, 0); err != nil {
		t.Fatal(err)
	}
}
