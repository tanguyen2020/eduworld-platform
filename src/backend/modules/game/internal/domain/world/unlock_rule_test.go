package world

import (
	"encoding/json"
	"testing"
)

func rule(t *testing.T, s string) Rule {
	t.Helper()
	var r Rule
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

const q0 = "0193a5f4-6c1e-7b2a-9d1f-000000071001"
const lo1, lo2, lo3 = "0193a5f4-6c1e-7b2a-9d1f-0000000710a1", "0193a5f4-6c1e-7b2a-9d1f-0000000710a2", "0193a5f4-6c1e-7b2a-9d1f-0000000710a3"

// F071-AC-14 (T028): every condition kind of WORLD_DESIGN §7.2, AND/OR, and the requirements still missing.
func TestUnlockRuleEvaluate(t *testing.T) {
	empty := Facts{}
	cases := []struct {
		name    string
		rule    string
		facts   Facts
		met     bool
		missing []string // kinds still missing
	}{
		{"quest missing", `{"quest_completed":{"quest_id":"` + q0 + `"}}`, empty, false, []string{"quest"}},
		{"quest done", `{"quest_completed":{"quest_id":"` + q0 + `"}}`, Facts{Quests: set(q0)}, true, nil},
		{"chapters 1/2", `{"chapters_completed":{"min":2,"subject_chapters_only":true}}`, Facts{SubjectChapters: 1, Chapters: 2}, false, []string{"chapters_completed"}},
		{"chapters 2/2", `{"chapters_completed":{"min":2,"subject_chapters_only":true}}`, Facts{SubjectChapters: 2}, true, nil},
		{"all chapters", `{"chapters_completed":{"min":2}}`, Facts{Chapters: 2}, true, nil},
		{"objectives 2 of 3 developing", `{"objective_progress":{"objective_ids":["` + lo1 + `","` + lo2 + `","` + lo3 + `"],"min_count":3,"min_level":"developing"}}`,
			Facts{Mastery: map[string]string{lo1: "developing", lo2: "mastered", lo3: "beginning"}}, false, []string{"learning_objective"}},
		{"objectives 3 of 3", `{"objective_progress":{"objective_ids":["` + lo1 + `","` + lo2 + `","` + lo3 + `"],"min_count":3,"min_level":"developing"}}`,
			Facts{Mastery: map[string]string{lo1: "developing", lo2: "mastered", lo3: "proficient"}}, true, nil},
		{"level 9 < 10", `{"min_level":{"level":10}}`, Facts{Level: 9}, false, []string{"level"}},
		{"level 10", `{"min_level":{"level":10}}`, Facts{Level: 10}, true, nil},
		{"achievement", `{"achievement_unlocked":{"achievement_id":"ach.ocean_explorer"}}`, Facts{Achievements: set("ach.ocean_explorer")}, true, nil},
		{"event missing", `{"event_participation":{"event_id":"evt.mid_autumn"}}`, empty, false, []string{"event"}},
		{"and: one missing", `{"all":[{"quest_completed":{"quest_id":"` + q0 + `"}},{"min_level":{"level":5}}]}`, Facts{Quests: set(q0), Level: 3}, false, []string{"level"}},
		{"and: both", `{"all":[{"quest_completed":{"quest_id":"` + q0 + `"}},{"min_level":{"level":5}}]}`, Facts{Quests: set(q0), Level: 5}, true, nil},
		{"or: one met", `{"any":[{"achievement_unlocked":{"achievement_id":"a"}},{"min_level":{"level":5}}]}`, Facts{Level: 7}, true, []string{"achievement"}},
		{"or: none", `{"any":[{"achievement_unlocked":{"achievement_id":"a"}},{"min_level":{"level":5}}]}`, Facts{Level: 1}, false, []string{"achievement", "level"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			met, reqs := rule(t, c.rule).Evaluate(c.facts)
			if met != c.met {
				t.Fatalf("met = %v, want %v", met, c.met)
			}
			var missing []string
			for _, r := range reqs {
				if !r.Met {
					missing = append(missing, r.Kind)
				}
			}
			if len(missing) != len(c.missing) {
				t.Fatalf("missing = %v, want %v", missing, c.missing)
			}
			for i := range missing {
				if missing[i] != c.missing[i] {
					t.Fatalf("missing = %v, want %v", missing, c.missing)
				}
			}
		})
	}
}

func TestUnlockRuleProgress(t *testing.T) {
	_, reqs := rule(t, `{"chapters_completed":{"min":2,"subject_chapters_only":true}}`).Evaluate(Facts{SubjectChapters: 1})
	if len(reqs) != 1 || reqs[0].Current != 1 || reqs[0].Target != 2 {
		t.Fatalf("requirement progress: %+v", reqs)
	}
	met, reqs := rule(t, `{"quest_completed":{"quest_id":"`+q0+`"}}`).Evaluate(Facts{Quests: set(q0)})
	if !met || UnlockedBy(reqs) != "quest" {
		t.Fatalf("unlocked_by: %v %+v", met, reqs)
	}
}

// F071-AC-16 (T030 domain part): a learning area never holds min_level or event_participation; unknown (payment)
// condition kinds are rejected by the schema decoder.
func TestLearningAreaRuleValidation(t *testing.T) {
	base := func(rule string) []byte {
		return []byte(`{"schema_version":1,"area_id":"learning_zone.math","region_code":"math_kingdom","name":{"vi":"Vương quốc Số","en":"Math Kingdom"},
			"area_class":"learning","subject_code":"math","scene_ref":"res://scenes/world/zones/learning_zone_math/Zone.tscn","bundle_id":"zone.learning_zone.math",
			"capacity":40,"max_speed_mps":7,"bounds":{"min":[-1,-1,-1],"max":[1,1,1]},"access_policy":{"room_type":"hub_instance","public":true,"grade_levels":[1,2,3]},"unlock_rule":` + rule + `,
			"layout":{"spawn_points":[{"spawn_point_id":"learning_zone.math.default","position":[0,0,0]}]}}`)
	}
	if _, err := ParseArea(base(`null`)); err != nil {
		t.Fatalf("open learning area: %v", err)
	}
	for _, r := range []string{`{"min_level":{"level":5}}`, `{"all":[{"quest_completed":{"quest_id":"` + q0 + `"}},{"event_participation":{"event_id":"e"}}]}`,
		`{"pay_coins":{"amount":10}}`} {
		if _, err := ParseArea(base(r)); err == nil || !IsInvalid(err) {
			t.Fatalf("rule %s: want invalid, got %v", r, err)
		}
	}
}

func set(ids ...string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}
