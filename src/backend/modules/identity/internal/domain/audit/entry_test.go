package audit

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSanitize(t *testing.T) {
	in := map[string]any{"tenant_type": "school", "password": "x", "Email": "a@b.vn", "full_name": "A", "BadKey": "y",
		"nested": map[string]any{"password": "x"}, "long": strings.Repeat("a", 300), "count": float64(3), "ids": []any{"a", "b"}}
	out, removed := Sanitize(in)
	if _, ok := out["password"]; ok || out["Email"] != nil || out["full_name"] != nil || out["nested"] != nil || out["BadKey"] != nil {
		t.Fatalf("forbidden kept: %v", out)
	}
	if len(out["long"].(string)) != MaxMetadataValue || out["tenant_type"] != "school" || out["count"] != float64(3) || out["ids"] == nil {
		t.Fatalf("kept fields: %v", out)
	}
	if removed != 6 {
		t.Fatalf("removed = %d", removed)
	}
	many := map[string]any{}
	for i := 0; i < 80; i++ {
		many["k"+strings.Repeat("x", i%10)+string(rune('a'+i%26))+string(rune('a'+i/26))] = strings.Repeat("v", 250)
	}
	out, _ = Sanitize(many)
	if len(out) > MaxMetadataKeys {
		t.Fatalf("keys = %d", len(out))
	}
}

func TestValidateAndRange(t *testing.T) {
	e := Entry{ID: uuid.New(), OccurredAt: time.Now(), ActorType: "user", Action: "role.assigned", TargetType: "user", TargetID: "x",
		Result: "success", Producer: "identity/core"}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	e.Action = "Role"
	if e.Validate() == nil {
		t.Fatal("bad action accepted")
	}
	now := time.Now()
	if CheckRange(now.Add(-24*time.Hour), now, 0) != nil || CheckRange(now.Add(-100*24*time.Hour), now, 0) == nil ||
		CheckRange(time.Time{}, now, 0) == nil || CheckRange(now, now.Add(-time.Hour), 0) == nil {
		t.Fatal("range rules")
	}
}
