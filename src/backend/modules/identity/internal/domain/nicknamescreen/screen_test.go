package nicknamescreen_test

import (
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/nicknamescreen"
)

// F136-AC-12 (TC-GAM-013, TC-SVC-005): compared without diacritics, in lower case, spaces collapsed — the full name,
// two or more words of it, or the school name (with or without its usual type prefix) is blocked; other words are
// allowed. Names are synthetic test data.
func TestBlocked(t *testing.T) {
	p := nicknamescreen.Personal{Names: []string{"Nguyễn Văn Minh"}, Schools: []string{"Trường Tiểu học Hòa Bình"}}
	blocked := []string{
		"Nguyễn Văn Minh",
		"nguyen van minh",
		"NGUYEN  VĂN MINH",
		"Minh Nguyễn",        // two words of the name, any order
		"Siêu Nhân Văn Minh", // two words of the name inside
		"Truong Tieu hoc Hoa Binh",
		"Hòa Bình Siêu Nhân", // school name without its type prefix
		"Sao Hoa Binh 2024",
	}
	for _, n := range blocked {
		if !nicknamescreen.Blocked(n, p) {
			t.Errorf("%q: allowed, want blocked", n)
		}
	}
	allowed := []string{
		"Siêu Nhân Sao Băng",
		"Minh Siêu Nhân", // one word of the name is not personal data (common given names)
		"Binh Minh Vui",  // one word of the school name, one of the name
		"Hoa Hồng Nhỏ",
		"Nguyenvan Sao", // not a word of the name
	}
	for _, n := range allowed {
		if nicknamescreen.Blocked(n, p) {
			t.Errorf("%q: blocked, want allowed", n)
		}
	}
}

// A student without a real name (self-registered, F135) is screened against the school only; a short school core
// ("Trường TH A") is compared as the full school name only, never as a single letter.
func TestBlockedWithoutName(t *testing.T) {
	p := nicknamescreen.Personal{Schools: []string{"Trường TH A", "THCS Đống Đa"}}
	if nicknamescreen.Blocked("An Vui Vẻ", p) || nicknamescreen.Blocked("Sao A Vui", p) {
		t.Fatal("a one-letter school core must not block every nickname")
	}
	if !nicknamescreen.Blocked("Truong TH A", p) || !nicknamescreen.Blocked("Dong Da Sieu Nhan", p) {
		t.Fatal("school names must be blocked")
	}
	if nicknamescreen.Blocked("Siêu Nhân", nicknamescreen.Personal{}) {
		t.Fatal("no personal data: allowed")
	}
}
