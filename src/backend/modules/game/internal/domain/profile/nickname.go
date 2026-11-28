package profile

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// ErrNicknameNotAllowed is any format violation of a typed nickname (422 NICKNAME_NOT_ALLOWED, F136-AC-10). One error
// for every reason: the response never says which rule failed (ADR-0105 §6).
var ErrNicknameNotAllowed = errors.New("nickname: not allowed")

// NicknameRules are the format parameters of a typed nickname (CONFIG_DEFAULTS nickname_min_length,
// nickname_max_digits; the maximum is MaxNicknameRunes, CHARACTER_SYSTEM §5.3).
type NicknameRules struct {
	MinLength int
	MaxDigits int
}

// Nickname is a typed nickname that passed the format rules, in normalised form (ADR-0105 §2). Moderation, personal
// data and uniqueness are checked by the application against it.
type Nickname struct{ text string }

// String returns the normalised nickname (the value stored in game.game_profiles.nickname).
func (n Nickname) String() string { return n.text }

// Key returns the case-insensitive uniqueness key (game.game_profiles.nickname_key): the normalised nickname in lower
// case, so "An An" and "an an" collide in a tenant (ADR-0105 §3).
func (n Nickname) Key() string { return NicknameKey(n.text) }

// NicknameKey is the uniqueness key of an already normalised nickname.
func NicknameKey(normalised string) string { return strings.ToLower(normalised) }

// NormalizeNickname applies the normalisation of ADR-0105 §2: Unicode NFC, leading/trailing white space removed, every
// run of white space collapsed into one space.
func NormalizeNickname(s string) string {
	return strings.Join(strings.FieldsFunc(norm.NFC.String(s), unicode.IsSpace), " ")
}

// vietnameseLetters are the precomposed letters of the Vietnamese alphabet beyond ASCII (lower case; the upper case
// forms are accepted through unicode.ToLower).
const vietnameseLetters = "àáảãạăằắẳẵặâầấẩẫậđèéẻẽẹêềếểễệìíỉĩịòóỏõọôồốổỗộơờớởỡợùúủũụưừứửữựỳýỷỹỵ"

// isLetter reports an ASCII letter or a precomposed Vietnamese letter: no other script and no other Latin letter, so
// look-alike characters cannot imitate another nickname (ADR-0105 §2).
func isLetter(r rune) bool {
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
		return true
	}
	return r > utf8.RuneSelf && strings.ContainsRune(vietnameseLetters, unicode.ToLower(r))
}

// ParseNickname normalises raw and checks the format rules (F136-AC-10): nickname_min_length..32 code points, only
// letters (isLetter), digits 0-9 and single spaces, at least one letter, at most nickname_max_digits digits. Any
// violation is ErrNicknameNotAllowed.
func ParseNickname(raw string, rules NicknameRules) (Nickname, error) {
	s := NormalizeNickname(raw)
	n := utf8.RuneCountInString(s)
	if n == 0 || n < rules.MinLength || n > MaxNicknameRunes {
		return Nickname{}, ErrNicknameNotAllowed
	}
	letters, digits := 0, 0
	for _, r := range s {
		switch {
		case isLetter(r):
			letters++
		case r >= '0' && r <= '9':
			digits++
		case r == ' ':
		default:
			return Nickname{}, ErrNicknameNotAllowed
		}
	}
	if letters == 0 || digits > rules.MaxDigits {
		return Nickname{}, ErrNicknameNotAllowed
	}
	return Nickname{text: s}, nil
}
