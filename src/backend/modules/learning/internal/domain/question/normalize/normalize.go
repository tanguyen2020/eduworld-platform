// Package normalize compares typed answers with accepted answers (QUESTION_BANK §4, F022-AC-17): Vietnamese
// normalization in 8 steps (NFC, whitespace, lower case, edge punctuation, old/new tone placement, optional missing
// diacritics, optional fuzzy match, numbers only when listed) and the English rules for content_language = en.
package normalize

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// Match is a textMatch of an answer key (question.schema.json#/$defs/textMatch).
type Match struct {
	Accepted               []string
	Mode                   string // normalized (default) or exact
	AllowDiacriticsMissing bool
	CaseSensitive          bool
	Fuzzy                  *Fuzzy
}

// Fuzzy is the optional edit-distance tolerance.
type Fuzzy struct {
	Enabled         bool
	MaxEditDistance int
	MinLength       int
}

// edgePunct is trimmed at both ends (step 4), with the typographic quotes.
const edgePunct = ".,!?;:\"'“”‘’…"

var (
	lowerVI = cases.Lower(language.Vietnamese)
	lowerEN = cases.Lower(language.English)
)

// Exact applies steps 1–2: NFC, trim and collapse whitespace.
func Exact(s string) string {
	return strings.Join(strings.Fields(norm.NFC.String(s)), " ")
}

// Text applies the normalization of language lang (vi: steps 1–5; en: NFC, whitespace, quotes, lower case, edge
// punctuation). caseSensitive keeps the letter case.
func Text(s, lang string, caseSensitive bool) string {
	s = Exact(s)
	if lang == "en" {
		s = strings.NewReplacer("’", "'", "‘", "'", "“", "\"", "”", "\"").Replace(s)
		if !caseSensitive {
			s = lowerEN.String(s)
		}
		return strings.TrimFunc(s, func(r rune) bool { return strings.ContainsRune(edgePunct, r) || unicode.IsSpace(r) })
	}
	if !caseSensitive {
		s = lowerVI.String(s)
	}
	s = strings.TrimFunc(s, func(r rune) bool { return strings.ContainsRune(edgePunct, r) || unicode.IsSpace(r) })
	return toneReplacer.Replace(norm.NFC.String(s))
}

// StripDiacritics removes Vietnamese diacritics (NFD, drop combining marks, đ → d) for step 6.
func StripDiacritics(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r == 'đ':
			b.WriteRune('d')
		case r == 'Đ':
			b.WriteRune('D')
		default:
			b.WriteRune(r)
		}
	}
	return norm.NFC.String(b.String())
}

// Equal reports whether the typed answer matches one accepted answer under m for a question of language lang.
func Equal(typed string, m Match, lang string) bool {
	for _, a := range m.Accepted {
		if one(typed, a, m, lang) {
			return true
		}
	}
	return false
}

func one(typed, accepted string, m Match, lang string) bool {
	if m.Mode == "exact" {
		return Exact(typed) == Exact(accepted)
	}
	x, y := Text(typed, lang, m.CaseSensitive), Text(accepted, lang, m.CaseSensitive)
	if x == y {
		return true
	}
	if m.AllowDiacriticsMissing && lang != "en" && StripDiacritics(x) == StripDiacritics(y) {
		return true
	}
	if f := m.Fuzzy; f != nil && f.Enabled && f.MaxEditDistance > 0 && len([]rune(y)) >= max(f.MinLength, 1) {
		return levenshtein(x, y) <= f.MaxEditDistance
	}
	return false
}

// levenshtein is the edit distance in runes.
func levenshtein(a, b string) int {
	x, y := []rune(a), []rune(b)
	prev := make([]int, len(y)+1)
	cur := make([]int, len(y)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(x); i++ {
		cur[0] = i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(y)]
}
