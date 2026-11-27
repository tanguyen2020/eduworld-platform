package message

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Limits of a search highlight (MessageSearchHit.highlight maxLength, highlight_ranges maxItems).
const (
	HighlightMaxRunes  = 1000
	HighlightMaxRanges = 32
)

// Range is one match of a highlight: Start and Length count Unicode code points of the highlight text.
type Range struct {
	Start  int
	Length int
}

// Highlight returns the plain-text highlight of a search hit and the ranges of the words matching q (delta F112-8,
// MessageSearchHit.highlight / highlight_ranges): the display text (already masked by the moderation pipeline) in NFC,
// cut to HighlightMaxRunes, with no markup at all; a range marks the prefix of a word of the text equal to one word of q
// (the words of the search, at most maxWords), ignoring case and Vietnamese diacritics — the matching of the search
// itself (prefix tsquery over the unaccented text). The ranges only cover text present in the display body, so they
// never point to a masked part or to a term of a moderation list (R15). Ranges are increasing and never overlap.
func Highlight(display, q string, maxWords int) (string, []Range) {
	runes := []rune(norm.NFC.String(display))
	if len(runes) > HighlightMaxRunes {
		runes = runes[:HighlightMaxRunes]
	}
	text := string(runes)
	words := searchWords(q, maxWords)
	if len(words) == 0 || len(runes) == 0 {
		return text, nil
	}
	folded := make([]rune, len(runes))
	for i, r := range runes {
		folded[i] = foldRune(r)
	}
	var out []Range
	for i := 0; i < len(runes) && len(out) < HighlightMaxRanges; {
		if !wordRune(runes[i]) {
			i++
			continue
		}
		j := i
		for j < len(runes) && wordRune(runes[j]) {
			j++
		}
		best := 0
		for _, w := range words {
			best = max(best, prefixLen(folded[i:j], w))
		}
		if best > 0 {
			out = append(out, Range{Start: i, Length: best})
		}
		i = j
	}
	return text, out
}

// searchWords are the folded words of a search text, as TSQuery splits them (letters and digits), at most maxWords.
func searchWords(q string, maxWords int) [][]rune {
	fields := strings.FieldsFunc(q, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	var out [][]rune
	for _, f := range fields {
		if maxWords > 0 && len(out) == maxWords {
			break
		}
		var w []rune
		for _, r := range norm.NFC.String(f) {
			if fr := foldRune(r); fr != 0 {
				w = append(w, fr)
			}
		}
		if len(w) > 0 {
			out = append(out, w)
		}
	}
	return out
}

// prefixLen is the number of code points of word (folded, 0 = a combining mark) whose letters spell w at its start, the
// combining marks right after included; 0 when word does not start with w.
func prefixLen(word, w []rune) int {
	p, k := 0, 0
	for p < len(word) && k < len(w) {
		if word[p] == 0 {
			p++
			continue
		}
		if word[p] != w[k] {
			return 0
		}
		p++
		k++
	}
	if k < len(w) {
		return 0
	}
	for p < len(word) && word[p] == 0 {
		p++
	}
	return p
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// foldRune lowers r and drops its Vietnamese diacritics (đ → d); a combining mark folds to 0.
func foldRune(r rune) rune {
	if unicode.Is(unicode.Mn, r) {
		return 0
	}
	r = unicode.ToLower(r)
	if r == 'đ' {
		return 'd'
	}
	for _, d := range norm.NFD.String(string(r)) {
		if !unicode.Is(unicode.Mn, d) {
			return d
		}
	}
	return r
}
