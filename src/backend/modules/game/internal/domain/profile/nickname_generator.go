package profile

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxSuggestionRunes is the length limit of a generated nickname (NicknameSuggestion.nickname maxLength 24).
const MaxSuggestionRunes = 24

// MaxNicknameRunes is the length limit of a stored nickname (varchar 32, CHARACTER_SYSTEM §5.3).
const MaxNicknameRunes = 32

// Words are the approved nickname words of one language (CharacterCatalog.nickname_parts).
type Words struct {
	Adjectives []string `json:"adjectives"`
	Nouns      []string `json:"nouns"`
}

// Rand is the random source of the generator (math/rand/v2 in production, seeded in tests).
type Rand interface {
	IntN(n int) int
}

// NicknameGenerator builds nicknames from approved words and two digits (CHARACTER_SYSTEM §5.3): never free text,
// never a real name. Vietnamese puts the adjective after the noun ("Cú Mèo Chăm Chỉ 27"); English before it.
type NicknameGenerator struct {
	Rand Rand
}

// Language returns "vi" or "en" for an Accept-Language value (default vi, supported_locales).
func Language(acceptLanguage string) string {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case strings.HasPrefix(tag, "vi"):
			return "vi"
		case strings.HasPrefix(tag, "en"):
			return "en"
		}
	}
	return "vi"
}

// One builds one nickname in lang, or "" when the drawn words are too long.
func (g NicknameGenerator) One(lang string, w Words) string {
	if len(w.Adjectives) == 0 || len(w.Nouns) == 0 {
		return ""
	}
	adj := strings.TrimSpace(w.Adjectives[g.Rand.IntN(len(w.Adjectives))])
	noun := strings.TrimSpace(w.Nouns[g.Rand.IntN(len(w.Nouns))])
	num := strconv.Itoa(10 + g.Rand.IntN(90))
	var s string
	if lang == "en" {
		s = adj + " " + noun + " " + num
	} else {
		s = noun + " " + adj + " " + num
	}
	if adj == "" || noun == "" || utf8.RuneCountInString(s) > MaxSuggestionRunes {
		return ""
	}
	return s
}

// Candidates returns up to n distinct nicknames accepted by keep (not yet used in the tenant, …), trying at most
// tries draws.
func (g NicknameGenerator) Candidates(lang string, w Words, n, tries int, keep func(string) bool) []string {
	seen := map[string]bool{}
	out := make([]string, 0, n)
	for i := 0; i < tries && len(out) < n; i++ {
		s := g.One(lang, w)
		key := strings.ToLower(s)
		if s == "" || seen[key] {
			continue
		}
		seen[key] = true
		if keep == nil || keep(s) {
			out = append(out, s)
		}
	}
	return out
}
