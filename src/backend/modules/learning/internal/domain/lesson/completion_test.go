package lesson

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// F031-AC-26 (T047): all_required_activities needs every required activity graded in the session (optional ones never
// block); any_activity needs one; view_only never completes through activities; completed is not passed.
func TestCompletionRule(t *testing.T) {
	a1, a2, opt := uuid.New(), uuid.New(), uuid.New()
	raw, _ := json.Marshal(map[string]any{"blocks": []any{
		map[string]any{"kind": "rich_text", "content": map[string]any{}},
		map[string]any{"kind": "activity", "activity_id": a1},
		map[string]any{"kind": "activity", "activity_id": a2, "required": true},
		map[string]any{"kind": "activity", "activity_id": opt, "required": false},
	}})
	acts := Activities(raw)
	if len(acts) != 3 || !acts[0].Required || acts[2].Required || !Contains(acts, opt) || Contains(acts, uuid.New()) {
		t.Fatalf("activities: %+v", acts)
	}
	if ok, missing := Satisfied(RuleAllRequired, acts, []uuid.UUID{a1}); ok || len(missing) != 1 || missing[0] != a2 {
		t.Fatalf("one required missing: %v %v", ok, missing)
	}
	if ok, _ := Satisfied(RuleAllRequired, acts, []uuid.UUID{a1, a2}); !ok {
		t.Fatal("all required done")
	}
	if ok, _ := Satisfied(RuleAllRequired, acts, []uuid.UUID{opt}); ok {
		t.Fatal("optional only")
	}
	if ok, _ := Satisfied(RuleAny, acts, []uuid.UUID{opt}); !ok {
		t.Fatal("any_activity with the optional one")
	}
	if ok, _ := Satisfied(RuleAny, acts, nil); ok {
		t.Fatal("any_activity with nothing")
	}
	if ok, _ := Satisfied(RuleViewOnly, acts, []uuid.UUID{a1, a2, opt}); ok {
		t.Fatal("view_only completed through activities")
	}
	// A version without required activity completes with its first graded activity, never with none.
	optOnly := []ActivityRef{{ActivityID: opt}}
	if ok, _ := Satisfied(RuleAllRequired, optOnly, nil); ok {
		t.Fatal("completed with nothing")
	}
	b, err := ParseBody(json.RawMessage(`{"blocks":[]}`))
	if err != nil || b.CompletionRule != RuleAllRequired {
		t.Fatalf("default rule: %+v %v", b, err)
	}
}
