package achievement

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var ict = time.FixedZone("ICT", 7*3600)

func def(kind string, tiers ...Tier) Definition {
	d := Definition{AchievementID: uuid.New(), Code: "progress.math_quests", Version: 1, Category: "progress", Visibility: "visible",
		Name: Text{"Nhà thám hiểm", "Explorer"}, Metric: Metric{Kind: kind}, Tiers: tiers}
	d.Trigger.EventTypes = []string{"quest.completed"}
	return d
}

func ev(payload map[string]any, at time.Time) Event {
	return Event{Type: "quest.completed", ID: uuid.New(), At: at, Payload: payload}
}

// F075-AC-21: filters select the events; count reaches the tiers in order; a tier is reported once (above the highest).
func TestCountTiersAndFilters(t *testing.T) {
	math := uuid.New()
	d := def("count", Tier{Tier: "bronze", Threshold: 2}, Tier{Tier: "silver", Threshold: 4})
	d.Trigger.Filters.SubjectIDs = []uuid.UUID{math}
	d.Trigger.Filters.QuestTypes = []string{"side", "learning"}
	if err := d.Validate(func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if d.Matches(ev(map[string]any{"subject_id": uuid.NewString(), "quest_type": "side"}, now)) ||
		d.Matches(ev(map[string]any{"subject_id": math.String(), "quest_type": "main"}, now)) ||
		d.Matches(Event{Type: "lesson.completed", Payload: map[string]any{"subject_id": math.String(), "quest_type": "side"}}) {
		t.Fatal("filtered event matches")
	}
	ok := ev(map[string]any{"subject_id": math.String(), "quest_type": "side"}, now)
	if !d.Matches(ok) {
		t.Fatal("matching event refused")
	}
	st := State{}
	d.Advance(&st, ok, ict)
	if len(d.NewTiers(st)) != 0 {
		t.Fatal("tier at 1")
	}
	d.Advance(&st, ok, ict)
	nt := d.NewTiers(st)
	if len(nt) != 1 || nt[0].Tier != "bronze" || *d.Target(st) != 2 {
		t.Fatalf("bronze: %+v", nt)
	}
	st.Tier = "bronze"
	if len(d.NewTiers(st)) != 0 || *d.Target(st) != 4 {
		t.Fatal("bronze reported twice")
	}
	st.Progress = 10
	nt = d.NewTiers(st)
	if len(nt) != 1 || nt[0].Tier != "silver" {
		t.Fatalf("silver: %+v", nt)
	}
	st.Tier = "silver"
	if d.Target(st) != nil {
		t.Fatal("target after the last tier")
	}
}

// F075-AC-24: a new version keeps the progress (never lowered) and the tiers already unlocked.
func TestNewVersionNeverLowers(t *testing.T) {
	v1 := def("distinct_count", Tier{Tier: "single", Threshold: 3})
	v1.Metric.DistinctField = "objective_id"
	st := State{}
	for _, o := range []string{"a", "b", "a", "c"} {
		v1.Advance(&st, ev(map[string]any{"objective_id": o}, time.Now()), ict)
	}
	if st.Progress != 3 || len(v1.NewTiers(st)) != 1 {
		t.Fatalf("v1: %+v", st)
	}
	st.Tier = "single"
	v2 := v1
	v2.Version, v2.Tiers = 2, []Tier{{Tier: "single", Threshold: 5}}
	v2.Advance(&st, ev(map[string]any{"objective_id": "a"}, time.Now()), ict)
	if st.Progress != 3 || len(v2.NewTiers(st)) != 0 {
		t.Fatalf("v2 lowered or unlocked again: %+v", st)
	}
	sum := def("sum", Tier{Tier: "single", Threshold: 10})
	sum.Metric.SumField = "correct_count"
	s := State{Progress: 4}
	if sum.Advance(&s, ev(map[string]any{"correct_count": -3.0}, time.Now()), ict) || s.Progress != 4 {
		t.Fatal("negative sum lowers")
	}
	sum.Advance(&s, ev(map[string]any{"correct_count": 6.0}, time.Now()), ict)
	if s.Progress != 10 {
		t.Fatalf("sum: %d", s.Progress)
	}
}

// distinct days of the ICT week: a new week starts counting again but the best week stays.
func TestDistinctDaysInWeek(t *testing.T) {
	d := def("distinct_days_in_week", Tier{Tier: "single", Threshold: 3})
	st := State{}
	mon := time.Date(2026, 11, 23, 2, 0, 0, 0, ict) // Monday
	for _, day := range []int{0, 0, 1, 3} {
		d.Advance(&st, ev(nil, mon.AddDate(0, 0, day)), ict)
	}
	if st.Progress != 3 {
		t.Fatalf("week 1: %+v", st)
	}
	d.Advance(&st, ev(nil, mon.AddDate(0, 0, 7)), ict)
	if st.Progress != 3 || len(st.Days) != 1 {
		t.Fatalf("week 2: %+v", st)
	}
}

// F075-AC-23: play time, logins, sessions, messages, official scores, unknown triggers and bad tiers are refused.
func TestValidateRefusesUnsafeAchievements(t *testing.T) {
	yes := func(string) bool { return true }
	cases := map[string]func(*Definition){
		"trigger":    func(d *Definition) { d.Trigger.EventTypes = []string{"student.game_logged_in"} },
		"play time":  func(d *Definition) { d.Metric = Metric{Kind: "sum", SumField: "duration_ms"} },
		"logins":     func(d *Definition) { d.Metric = Metric{Kind: "distinct_count", DistinctField: "login_date"} },
		"messages":   func(d *Definition) { d.Metric = Metric{Kind: "sum", SumField: "message_count"} },
		"exam score": func(d *Definition) { d.Metric = Metric{Kind: "sum", SumField: "score"} },
		"online":     func(d *Definition) { d.Metric = Metric{Kind: "distinct_count", DistinctField: "online_day"} },
		"tier order": func(d *Definition) { d.Tiers = []Tier{{Tier: "silver", Threshold: 2}, {Tier: "bronze", Threshold: 4}} },
		"threshold":  func(d *Definition) { d.Tiers = []Tier{{Tier: "bronze", Threshold: 4}, {Tier: "silver", Threshold: 4}} },
		"xp reward": func(d *Definition) {
			d.Tiers = []Tier{{Tier: "single", Threshold: 1, Rewards: []map[string]any{{"type": "xp", "amount": 5.0}}}}
		},
		"category": func(d *Definition) { d.Category = "login" },
		"metric":   func(d *Definition) { d.Metric = Metric{Kind: "streak"} },
		"too many days": func(d *Definition) {
			d.Metric = Metric{Kind: "distinct_days_in_week"}
			d.Tiers = []Tier{{Tier: "single", Threshold: 8}}
		},
	}
	for name, mut := range cases {
		d := def("count", Tier{Tier: "single", Threshold: 1})
		mut(&d)
		var fe *FieldError
		if err := d.Validate(yes); !errors.As(err, &fe) || !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	ok := def("count", Tier{Tier: "single", Threshold: 1, Rewards: []map[string]any{{"type": "badge", "badge_id": "badge_explorer"}}})
	if err := ok.Validate(func(t string) bool { return t != "badge" }); err == nil {
		t.Fatal("reward without applier accepted")
	}
}
