package i18n

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// F032-AC-29 (T003): every reason code exists in Vietnamese and English and renders its parameters, within 300 characters.
func TestReasons(t *testing.T) {
	r, err := LoadReasons()
	if err != nil {
		t.Fatal(err)
	}
	for code := range r.byLang["vi"] {
		if _, ok := r.byLang["en"][code]; !ok {
			t.Fatalf("%s has no English text", code)
		}
	}
	if len(r.byLang["vi"]) != len(r.byLang["en"]) {
		t.Fatal("the two languages differ")
	}
	got := r.Text("rec.weak_objective", "vi", map[string]string{"objective": "Bảng nhân 7"})
	if !strings.Contains(got, "Bảng nhân 7") || strings.Contains(got, "{") {
		t.Fatalf("vi: %s", got)
	}
	if en := r.Text("rec.due_review", "en", map[string]string{"count": "2"}); !strings.HasPrefix(en, "2 questions") {
		t.Fatalf("en: %s", en)
	}
	if long := r.Text("rec.weak_objective", "vi", map[string]string{"objective": strings.Repeat("a", 500)}); utf8.RuneCountInString(long) > MaxReasonText {
		t.Fatalf("too long: %d", utf8.RuneCountInString(long))
	}
	if r.Text("rec.unknown", "en", nil) != "rec.unknown" {
		t.Fatal("unknown code")
	}
}
