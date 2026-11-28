package profile_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
)

// nickRules are the CONFIG_DEFAULTS values nickname_min_length = 3, nickname_max_digits = 4 (ADR-0105).
var nickRules = profile.NicknameRules{MinLength: 3, MaxDigits: 4}

// F136-AC-10 (TC-GAM-013): normalisation (NFC, trim, collapsed spaces) then the format nickRules — length
// nickname_min_length..32 code points, Latin letters including precomposed Vietnamese letters, digits 0-9 and single
// spaces, at least one letter, at most nickname_max_digits digits. Every violation is the same error.
func TestParseNickname(t *testing.T) {
	ok := []struct{ in, want string }{
		{"Siêu Nhân Sao Băng", "Siêu Nhân Sao Băng"},
		{"  Siêu   Nhân\tSao  ", "Siêu Nhân Sao"},
		{"Siêu Nhân", "Siêu Nhân"}, // decomposed input becomes NFC
		{"Đom Đóm Vui", "Đom Đóm Vui"},
		{"An An", "An An"},
		{"Minh 2024", "Minh 2024"},
		{"abc", "abc"},
		{"Ỷ Ỹ Ựa", "Ỷ Ỹ Ựa"},
		{strings.Repeat("a", 32), strings.Repeat("a", 32)},
		{"  " + strings.Repeat("b", 32) + "  ", strings.Repeat("b", 32)},
	}
	for _, c := range ok {
		n, err := profile.ParseNickname(c.in, nickRules)
		if err != nil {
			t.Errorf("ParseNickname(%q): %v", c.in, err)
			continue
		}
		if n.String() != c.want {
			t.Errorf("ParseNickname(%q) = %q, want %q", c.in, n.String(), c.want)
		}
	}
	refused := []string{
		"",
		"   ",
		"Mn",                         // shorter than nickname_min_length
		"  M n ",                     // 3 code points after trim but one is a space: "M n" ok? no — see below
		strings.Repeat("a", 33),      // longer than 32
		"Siêu Nhân 123456",           // more than nickname_max_digits digits
		"Minh 0912 345 678",          // a phone number: too many digits
		"1234",                       // no letter
		"@minh_123",                  // special characters
		"[b]An[/b] Vui",              // markup characters (F136-AC-27)
		"An Vui \U0001F600",          // emoji
		"Ñandú Vui",                  // Latin letters outside the Vietnamese alphabet
		"Аn Vui",                     // Cyrillic A (look-alike letters, ADR-0105 §2)
		"An\u200bVui",                // zero-width space
		"an-an",                      // hyphen
		"Nhâń Vui",                  // a combining mark that does not compose
		"mail a còng gmail chấm com", // longer than 32? no: 26 — allowed by format; screened by moderation (US3)
	}
	for _, in := range refused {
		_, err := profile.ParseNickname(in, nickRules)
		switch in {
		case "  M n ", "mail a còng gmail chấm com":
			// Valid format: "M n" has 3 code points (letters and a single space); the spelled-out address is the
			// moderation pipeline's (PII step), not the format's.
			if err != nil {
				t.Errorf("ParseNickname(%q): %v, want the format to accept it", in, err)
			}
			continue
		}
		if !errors.Is(err, profile.ErrNicknameNotAllowed) {
			t.Errorf("ParseNickname(%q) = %v, want ErrNicknameNotAllowed", in, err)
		}
	}
}

// F136-AC-13 (domain part): the uniqueness key is the normalised nickname in lower case ("An An" and "an an" collide).
func TestNicknameKey(t *testing.T) {
	cases := map[string]string{
		"An An":       "an an",
		"  AN   an ":  "an an",
		"SIÊU NHÂN":   "siêu nhân",
		"Đom Đóm":     "đom đóm",
		"Siêu Nhân": "siêu nhân",
	}
	for in, want := range cases {
		n, err := profile.ParseNickname(in, nickRules)
		if err != nil {
			t.Fatalf("ParseNickname(%q): %v", in, err)
		}
		if n.Key() != want {
			t.Errorf("Key(%q) = %q, want %q", in, n.Key(), want)
		}
	}
	if got := profile.NormalizeNickname("  Siêu \t Nhân "); got != "Siêu Nhân" {
		t.Errorf("NormalizeNickname = %q", got)
	}
}

// The nickRules come from configuration: a stricter digit limit refuses what the default allows.
func TestParseNicknameRules(t *testing.T) {
	if _, err := profile.ParseNickname("Minh 12", profile.NicknameRules{MinLength: 3, MaxDigits: 1}); !errors.Is(err, profile.ErrNicknameNotAllowed) {
		t.Errorf("max digits 1: %v", err)
	}
	if _, err := profile.ParseNickname("Minh", profile.NicknameRules{MinLength: 5, MaxDigits: 4}); !errors.Is(err, profile.ErrNicknameNotAllowed) {
		t.Errorf("min length 5: %v", err)
	}
}
