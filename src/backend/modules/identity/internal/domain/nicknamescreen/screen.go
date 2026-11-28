// Package nicknamescreen decides whether a typed nickname holds the real name or the school name of the student
// (screenNicknamePersonalData, F136-AC-12, ADR-0105 §5). Pure: the names come from the read model of the caller and
// never leave module identity (R16).
package nicknamescreen

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Personal is the personal data a nickname is compared with: the real names of the student (full name, display name)
// and the names of the school (name and its translations).
type Personal struct {
	Names   []string
	Schools []string
}

// Words folds s for comparison: no diacritics (đ → d), lower case, every character other than a letter or digit a
// separator.
func Words(s string) []string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r == 'đ' || r == 'Đ':
			b.WriteRune('d')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Fields(b.String())
}

// schoolPrefixes are the usual school type prefixes (folded), stripped from the start of a school name so that the
// distinctive part ("Hòa Bình" of "Trường Tiểu học Hòa Bình") is compared too. Longest first.
var schoolPrefixes = [][]string{
	{"trung", "hoc", "pho", "thong"}, {"trung", "hoc", "co", "so"}, {"tieu", "hoc"}, {"trung", "hoc"}, {"mam", "non"},
	{"lien", "cap"}, {"pho", "thong"}, {"phan", "hieu"}, {"diem", "truong"}, {"truong"}, {"thpt"}, {"thcs"}, {"th"}, {"mn"},
	{"school"}, {"primary"}, {"secondary"},
}

func hasPrefix(words, prefix []string) bool {
	if len(prefix) > len(words) {
		return false
	}
	for i, p := range prefix {
		if words[i] != p {
			return false
		}
	}
	return true
}

// core strips the type prefixes of a folded school name; ok is false when what is left is too short to be
// distinctive (one word shorter than 4 letters, e.g. "Trường TH A"), so a single letter never blocks every nickname.
func core(words []string) ([]string, bool) {
	for stripped := true; stripped; {
		stripped = false
		for _, p := range schoolPrefixes {
			if len(words) > len(p) && hasPrefix(words, p) {
				words, stripped = words[len(p):], true
				break
			}
		}
	}
	if len(words) == 0 || (len(words) == 1 && len([]rune(words[0])) < 4) {
		return nil, false
	}
	return words, true
}

// containsSeq reports whether seq appears in words as consecutive whole words.
func containsSeq(words, seq []string) bool {
	if len(seq) == 0 {
		return false
	}
	for i := 0; i+len(seq) <= len(words); i++ {
		if hasPrefix(words[i:], seq) {
			return true
		}
	}
	return false
}

// Blocked reports whether nickname holds personal data (F136-AC-12): equal to a real name of the student, holding two
// or more distinct words of it (any order), or holding a school name of the student — the full name or its
// distinctive part without the type prefix. Comparison without diacritics, in lower case.
func Blocked(nickname string, p Personal) bool {
	nick := Words(nickname)
	if len(nick) == 0 {
		return false
	}
	have := map[string]bool{}
	for _, w := range nick {
		have[w] = true
	}
	joined := strings.Join(nick, " ")
	for _, name := range p.Names {
		words := Words(name)
		if len(words) == 0 {
			continue
		}
		if strings.Join(words, " ") == joined {
			return true
		}
		seen, hits := map[string]bool{}, 0
		for _, w := range words {
			if !seen[w] && have[w] {
				hits++
			}
			seen[w] = true
		}
		if hits >= 2 {
			return true
		}
	}
	for _, school := range p.Schools {
		words := Words(school)
		if containsSeq(nick, words) {
			return true
		}
		if c, ok := core(words); ok && containsSeq(nick, c) {
			return true
		}
	}
	return false
}
