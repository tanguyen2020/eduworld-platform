package staffmail

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/staffmail"
)

// F124 T022 / F124-AC-09, AC-10: the parameters a template may store and render (no student, case, category or content
// in the school emails) and the retry schedule (1, 2, 4 … minutes up to one hour; failed after the maximum).
func TestStaffMailParamsAndBackoff(t *testing.T) {
	p := staffmail.Params(staffmail.TemplateSchoolNotification, map[string]string{"school_id": "s", "notification_id": "n", "ack_due_at": "2026",
		"case_id": "c", "category_code": "asked_personal_info", "student_name": "x", "summary": "y"})
	if len(p) != 3 || p["case_id"] != "" || p["category_code"] != "" || p["student_name"] != "" || p["summary"] != "" {
		t.Fatalf("school params: %v", p)
	}
	if q := staffmail.Params(staffmail.TemplateOncallPage, map[string]string{"case_id": "c", "severity": "p1", "level": "primary", "tenant_id": "t"}); len(q) != 3 ||
		q["tenant_id"] != "" {
		t.Fatalf("oncall params: %v", q)
	}
	if b := staffmail.Params(staffmail.TemplateBreakGlassRequest, map[string]string{"grant_id": "g", "reason": "r"}); len(b) != 1 {
		t.Fatalf("break-glass params: %v", b)
	}
	for n, want := range map[int]time.Duration{1: time.Minute, 2: 2 * time.Minute, 3: 4 * time.Minute, 7: time.Hour, 20: time.Hour} {
		if got := staffmail.Backoff(n); got != want {
			t.Errorf("backoff %d: %s, want %s", n, got, want)
		}
	}
	if staffmail.Exhausted(7, 8) || !staffmail.Exhausted(8, 8) {
		t.Fatal("exhausted")
	}
	if !staffmail.School(staffmail.TemplateSchoolReminder) || staffmail.School(staffmail.TemplateOncallPage) || staffmail.Known("x") {
		t.Fatal("templates")
	}
}
