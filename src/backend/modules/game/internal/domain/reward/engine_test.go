package reward

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func ptr[T any](v T) *T { return &v }

// policy is the CONFIG_DEFAULTS policy: xp_daily_cap 1000, xp_repeat_multipliers 100/50/25/0 in 7 days,
// xp_below_grade_multiplier 0.25 at ≥ 2 grades below.
func policy() Policy {
	return Policy{DailyCap: 1000, Repeat: Diminishing{Multipliers: []float64{1, 0.5, 0.25, 0}, WindowDays: 7}, BelowGradeMultiplier: 0.25,
		BelowGradeGap: 2}
}

func activityRule(amount, perCorrect int) Rule {
	return Rule{RuleID: uuid.New(), VersionID: uuid.New(), Version: 1, Code: "xp.activity_completed", SourceCode: "activity.completed",
		EventType: "activity.graded", Grants: []Grant{{Type: "xp", Amount: amount, PerCorrect: perCorrect}}, Caps: Caps{XPPerDay: 600}}
}

func graded(firstTry int) Trigger {
	cv := uuid.New()
	return Trigger{EventType: "activity.graded", SourceType: "activity", SourceID: uuid.New(), SourceCodes: []string{"activity.completed"},
		ContentVersionID: &cv, Mode: "practice", CorrectCount: ptr(firstTry), ItemCount: ptr(10), FirstTryCorrect: ptr(firstTry)}
}

func eval(p Policy, t Trigger, rules []Rule, grade int, h History, granted map[string]int) []Award {
	if granted == nil {
		granted = map[string]int{}
	}
	return p.Evaluate(t, rules, Student{Grade: grade}, func(Rule) History { return h }, granted)
}

func amount(t *testing.T, as []Award) int {
	t.Helper()
	if len(as) != 1 {
		t.Fatalf("awards: %+v", as)
	}
	return as[0].Amount
}

// F074-AC-04: the n-th completion of one content version in the window gets the n-th multiplier, the last one for
// every later completion.
func TestRepeatMultipliers(t *testing.T) {
	r := activityRule(20, 0)
	for repeats, want := range []int{20, 10, 5, 0, 0, 0} {
		if got := amount(t, eval(policy(), graded(5), []Rule{r}, 3, History{Repeats: repeats}, nil)); got != want {
			t.Fatalf("completion %d: %d, want %d", repeats+1, got, want)
		}
	}
	// A rule with its own diminishing_returns uses it.
	r.Diminishing = &Diminishing{Multipliers: []float64{1, 0.8}, WindowDays: 3}
	if got := amount(t, eval(policy(), graded(5), []Rule{r}, 3, History{Repeats: 4}, nil)); got != 16 {
		t.Fatalf("rule multipliers: %d", got)
	}
	if w := policy().RepeatWindowDays(r); w != 3 {
		t.Fatalf("window %d", w)
	}
}

// F074-AC-05: the daily cap and the rule cap only let the remainder through; capped is set, also on a 0 XP posting.
func TestCaps(t *testing.T) {
	r := activityRule(20, 0)
	as := eval(policy(), graded(5), []Rule{r}, 3, History{}, map[string]int{TotalBucket: 990})
	if amount(t, as) != 10 || !as[0].Capped {
		t.Fatalf("partial: %+v", as)
	}
	as = eval(policy(), graded(5), []Rule{r}, 3, History{}, map[string]int{TotalBucket: 1000})
	if amount(t, as) != 0 || !as[0].Capped {
		t.Fatalf("exhausted: %+v", as)
	}
	as = eval(policy(), graded(5), []Rule{r}, 3, History{}, map[string]int{"xp:src:activity": 595})
	if amount(t, as) != 5 || !as[0].Capped || as[0].Bucket != "xp:src:activity" {
		t.Fatalf("rule cap: %+v", as)
	}
	r.Caps.Bucket = "xp:activity_rule"
	as = eval(policy(), graded(5), []Rule{r}, 3, History{}, map[string]int{"xp:src:activity": 595})
	if amount(t, as) != 20 || as[0].Capped || as[0].Bucket != "xp:activity_rule" {
		t.Fatalf("own bucket: %+v", as)
	}
	// Two rules of one event share the counters in priority order.
	a, b := activityRule(15, 0), activityRule(15, 0)
	a.Priority, b.Priority = 10, 5
	as = eval(policy(), graded(5), []Rule{b, a}, 3, History{}, map[string]int{TotalBucket: 980})
	if len(as) != 2 || as[0].Rule.RuleID != a.RuleID || as[0].Amount != 15 || as[1].Amount != 5 || !as[1].Capped {
		t.Fatalf("shared: %+v", as)
	}
}

// F074-AC-06: XP per first-try correct answer; wrong, rapid-guess or after-explanation answers give 0.
func TestFirstTryCorrect(t *testing.T) {
	r := activityRule(0, 3)
	r.Grants[0].Amount = 0
	if got := amount(t, eval(policy(), graded(4), []Rule{r}, 3, History{}, nil)); got != 12 {
		t.Fatalf("4 first-try: %d", got)
	}
	as := eval(policy(), graded(0), []Rule{r}, 3, History{}, nil)
	if amount(t, as) != 0 || as[0].Capped {
		t.Fatalf("0 first-try: %+v", as)
	}
	tr := graded(7)
	tr.FirstTryCorrect = nil
	tr.RapidGuess = true
	if got := amount(t, eval(policy(), tr, []Rule{r}, 3, History{}, nil)); got != 0 {
		t.Fatalf("rapid guess: %d", got)
	}
	tr.RapidGuess = false
	if got := amount(t, eval(policy(), tr, []Rule{r}, 3, History{}, nil)); got != 21 {
		t.Fatalf("fallback correct_count: %d", got)
	}
}

// F074-AC-07: content ≥ 2 grades below the student × 0.25, unless recommended or assigned; the repeat factor still applies.
func TestBelowGrade(t *testing.T) {
	r := activityRule(20, 0)
	low := graded(5)
	low.ContentGrades = []int{3}
	if got := amount(t, eval(policy(), low, []Rule{r}, 5, History{}, nil)); got != 5 {
		t.Fatalf("below: %d", got)
	}
	if got := amount(t, eval(policy(), low, []Rule{r}, 4, History{}, nil)); got != 20 {
		t.Fatalf("one grade below: %d", got)
	}
	rec := low
	rec.RecommendationID = ptr(uuid.New())
	if got := amount(t, eval(policy(), rec, []Rule{r}, 5, History{Repeats: 1}, nil)); got != 10 {
		t.Fatalf("recommended (2nd completion): %d", got)
	}
	asg := low
	asg.Mode = "assignment"
	if got := amount(t, eval(policy(), asg, []Rule{r}, 5, History{}, nil)); got != 20 {
		t.Fatalf("assignment: %d", got)
	}
}

// Rule selection: event type and source code must match; attempt modes (exam only when listed); conditions.
func TestSelection(t *testing.T) {
	r := activityRule(20, 0)
	other := r
	other.SourceCode = "lesson.completed"
	if as := eval(policy(), graded(5), []Rule{other}, 3, History{}, nil); len(as) != 0 {
		t.Fatalf("source mismatch: %+v", as)
	}
	exam := graded(5)
	exam.Mode = "exam"
	if as := eval(policy(), exam, []Rule{r}, 3, History{}, nil); len(as) != 0 {
		t.Fatalf("exam without exam mode: %+v", as)
	}
	withExam := r
	withExam.AttemptModes = []string{"exam"}
	if amount(t, eval(policy(), exam, []Rule{withExam}, 3, History{}, nil)) != 20 {
		t.Fatal("exam rule")
	}
	if as := eval(policy(), graded(5), []Rule{withExam}, 3, History{}, nil); len(as) != 0 {
		t.Fatalf("practice on an exam-only rule: %+v", as)
	}
	ratio := r
	ratio.Conditions.MinCorrectRatio = ptr(0.8)
	if as := eval(policy(), graded(7), []Rule{ratio}, 3, History{}, nil); len(as) != 0 {
		t.Fatalf("ratio: %+v", as)
	}
	first := r
	first.Conditions.FirstCompletionOnly = true
	if as := eval(policy(), graded(5), []Rule{first}, 3, History{Ever: 1}, nil); len(as) != 0 {
		t.Fatalf("first completion: %+v", as)
	}
	party := r
	party.Conditions.MinPartySize = ptr(2)
	if as := eval(policy(), graded(5), []Rule{party}, 3, History{}, nil); len(as) != 0 {
		t.Fatalf("party condition on a solo event: %+v", as)
	}
	grade := r
	grade.Conditions.GradeLevels = []int{4, 5}
	if as := eval(policy(), graded(5), []Rule{grade}, 3, History{}, nil); len(as) != 0 {
		t.Fatalf("grade condition: %+v", as)
	}
}

// F074-AC-23 (rule part): what the JSON Schema leaves to the server.
func TestRuleValidate(t *testing.T) {
	ok := func(typ string) bool { return typ == "xp" || typ == "title" }
	if err := activityRule(20, 0).Validate(ok); err != nil {
		t.Fatal(err)
	}
	for field, edit := range map[string]func(r *Rule){
		"caps/xp_per_day":    func(r *Rule) { r.Caps.XPPerDay = 0 },
		"source_code":        func(r *Rule) { r.SourceCode = "login.daily" },
		"trigger/event_type": func(r *Rule) { r.EventType = "game_session.started" },
		"grants/0/type":      func(r *Rule) { r.Grants[0] = Grant{Type: "item", ItemID: "hat"} },
		"grants":             func(r *Rule) { r.Grants[0] = Grant{Type: "xp"} },
	} {
		r := activityRule(20, 0)
		edit(&r)
		var fe *FieldError
		if err := r.Validate(ok); !errors.As(err, &fe) || fe.Field != field {
			t.Fatalf("%s: %v", field, r.Validate(ok))
		}
	}
	// F074-AC-09: no trigger of the catalog rewards logging in, online time, speed or streaks.
	for _, e := range TriggerEvents {
		for _, banned := range []string{"login", "session", "online", "streak", "speed"} {
			if contains(e, banned) {
				t.Fatalf("trigger %s", e)
			}
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
