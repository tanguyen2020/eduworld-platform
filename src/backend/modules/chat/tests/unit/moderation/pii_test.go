package moderation

import (
	"slices"
	"testing"
)

// T010 / F116-AC-04 (TC-CHAT-010, AC-MOD-001, AC-MOD-003): the PII table of MODERATION §3.3. Contact details are
// invented examples (no real person).
func TestPIIDetector(t *testing.T) {
	cases := []struct{ text, decision, code string }{
		{"số mình 0912345678 nè", "block", "pii"},
		{"số mình 0912 345 678 nè", "block", "pii"},
		{"0912.345.678", "block", "pii"},
		{"+84 912 345 678", "block", "pii"},
		{"không chín một hai ba bốn năm sáu bảy tám", "block", "pii"},
		{"khong chin mot hai ba bon nam sau bay", "block", "pii"},
		{"cccd 012345678901", "block", "pii"},
		{"mail mình là ten.ho@example.com", "block", "pii"},
		{"ten a còng example chấm com", "block", "pii"},
		{"tenho at example dot com", "block", "pii"},
		{"vào www.example.vn chơi", "block", "pii"},
		{"http://example.org", "block", "pii"},
		{"trang example.com hay lắm", "block", "pii"},
		{"trang example chấm vn", "block", "pii"},
		{"kết bạn zalo nhé", "block", "pii"},
		{"add fb mình đi", "block", "pii"},
		{"f.b mình nè", "block", "pii"},
		{"Z A L O", "block", "pii"},
		{"follow @tenho_abc nha", "block", "pii"},
		{"nhà ở quận 7", "hold_for_review", "pii"},
		{"ngõ 12 nhé", "hold_for_review", "pii"},
		{"mai gặp ở công viên nhé", "hold_for_review", "grooming_scam"},
		{"mấy giờ ra", "hold_for_review", "grooming_scam"},
		// no false positive on ordinary study chat
		{"câu 3.5 nhé", "allow", ""},
		{"xong rồi. co len", "allow", ""},
		{"đường thẳng song song", "allow", ""},
		{"2 + 2 = 4, 10 x 10 = 100", "allow", ""},
		{"năm nay mình học lớp ba", "allow", ""},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			o := eval(t, c.text)
			if o.Decision != c.decision {
				t.Fatalf("decision %s, want %s (codes %v, pii %+v)", o.Decision, c.decision, o.Codes, o.PII)
			}
			if c.code != "" && !slices.Contains(o.Codes, c.code) {
				t.Fatalf("codes %v, want %s", o.Codes, c.code)
			}
		})
	}
}
