// Package normalize builds the "views" of a chat text the moderation pipeline matches on (F116-AC-02, MODERATION §3.1):
// Unicode NFC → lowercase, homoglyphs → Latin, leetspeak, collapsed repeated letters, the Vietnamese fold (no
// diacritics), teencode expansion and the merge of single characters split by separators ("n.g.u" → one token). It is
// pure (no I/O): the maps come from the versioned configuration (seed/normalization), never a sensitive word (R15).
package normalize

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Config is the normalisation configuration (seed/normalization/normalization.v1.json).
type Config struct {
	Version   int               `json:"version"`
	Teencode  map[string]string `json:"teencode"`
	Leet      map[string]string `json:"leet"`
	Homoglyph map[string]string `json:"homoglyph"`
}

// Normalizer turns a text into Views. Build it once (New) and share it: it is read-only.
type Normalizer struct {
	teencode  map[string][]string // lowercase key → canonical tokens of the expansion
	leet      map[rune]rune
	homoglyph map[rune]rune
	version   int
}

// New compiles c.
func New(c Config) *Normalizer {
	n := &Normalizer{teencode: map[string][]string{}, leet: map[rune]rune{}, homoglyph: map[rune]rune{}, version: c.Version}
	for k, v := range c.Homoglyph {
		kr, _ := utf8.DecodeRuneInString(k)
		vr, _ := utf8.DecodeRuneInString(v)
		if kr != utf8.RuneError && vr != utf8.RuneError {
			n.homoglyph[kr] = vr
		}
	}
	for k, v := range c.Leet {
		kr, _ := utf8.DecodeRuneInString(k)
		vr, _ := utf8.DecodeRuneInString(v)
		if kr != utf8.RuneError && vr != utf8.RuneError {
			n.leet[kr] = vr
		}
	}
	for k, v := range c.Teencode {
		var exp []string
		for _, t := range n.Tokens(v) {
			exp = append(exp, t.Canon)
		}
		if len(exp) > 0 {
			n.teencode[strings.ToLower(norm.NFC.String(k))] = exp
		}
	}
	return n
}

// Version is the version of the configuration.
func (n *Normalizer) Version() int { return n.version }

// Token is one word of a text: Start/End are byte offsets in the NFC text (masking replaces them), Raw the lowercase
// form (homoglyphs mapped), Canon the matching form with diacritics (leetspeak decoded, repeated letters collapsed),
// Fold the same without diacritics; Diacritic reports that Canon carries Vietnamese marks.
type Token struct {
	Start, End int
	Raw        string
	Canon      string
	Fold       string
	Diacritic  bool
}

// View is a text and its token sequences: Tokens (teencode expanded) and Merged (single characters separated by
// spaces or punctuation joined into one token, then teencode expanded).
type View struct {
	Text   string
	Tokens []Token
	Merged []Token
	// Lower is the lowercase text (homoglyphs mapped) and LowerPos maps a byte offset of Lower to one of Text.
	Lower    string
	LowerPos []int
}

// View normalises text (NFC first: every offset refers to the NFC text).
func (n *Normalizer) View(text string) View {
	text = norm.NFC.String(text)
	raw := n.Tokens(text)
	v := View{Text: text, Tokens: n.expand(raw), Merged: n.expand(n.merge(text, raw))}
	var b strings.Builder
	pos := make([]int, 0, len(text)+1)
	for i, r := range text {
		l := n.lowerRune(r)
		before := b.Len()
		b.WriteRune(l)
		for j := before; j < b.Len(); j++ {
			pos = append(pos, i)
		}
	}
	pos = append(pos, len(text))
	v.Lower, v.LowerPos = b.String(), pos
	return v
}

func (n *Normalizer) lowerRune(r rune) rune {
	l := unicode.ToLower(r)
	if m, ok := n.homoglyph[l]; ok {
		return m
	}
	return l
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// Tokens splits text into words (letters, digits and combining marks; a leetspeak symbol between two word characters
// stays inside the word: "ng@c").
func (n *Normalizer) Tokens(text string) []Token {
	var out []Token
	runes := []rune(text)
	offs := make([]int, len(runes)+1)
	o := 0
	for i, r := range runes {
		offs[i] = o
		o += utf8.RuneLen(r)
	}
	offs[len(runes)] = o
	inWord := func(i int) bool {
		if i < 0 || i >= len(runes) {
			return false
		}
		if wordRune(runes[i]) {
			return true
		}
		_, leet := n.leet[runes[i]]
		return leet && i > 0 && i+1 < len(runes) && wordRune(runes[i-1]) && wordRune(runes[i+1])
	}
	for i := 0; i < len(runes); {
		if !inWord(i) {
			i++
			continue
		}
		j := i
		for j < len(runes) && inWord(j) {
			j++
		}
		out = append(out, n.token(text[offs[i]:offs[j]], offs[i], offs[j]))
		i = j
	}
	return out
}

func (n *Normalizer) token(s string, start, end int) Token {
	var raw strings.Builder
	letters := false
	for _, r := range s {
		l := n.lowerRune(r)
		raw.WriteRune(l)
		if unicode.IsLetter(l) {
			letters = true
		}
	}
	rs := norm.NFC.String(raw.String())
	var canon strings.Builder
	var prev rune = -1
	for _, r := range rs {
		if letters {
			if m, ok := n.leet[r]; ok {
				r = m
			}
		}
		if r == prev { // repeated characters collapse to one ("ngooooc" → "ngoc"), on both terms and texts
			continue
		}
		canon.WriteRune(r)
		prev = r
	}
	c := canon.String()
	f := Fold(c)
	return Token{Start: start, End: end, Raw: rs, Canon: c, Fold: f, Diacritic: f != c}
}

// merge joins runs of ≥ 2 single-character tokens separated only by separators ("n.g.u", "n g u", "n_g_u").
func (n *Normalizer) merge(text string, toks []Token) []Token {
	out := make([]Token, 0, len(toks))
	for i := 0; i < len(toks); {
		j := i
		for j < len(toks) && utf8.RuneCountInString(toks[j].Canon) == 1 {
			j++
		}
		if j-i >= 2 {
			var b strings.Builder
			for _, t := range toks[i:j] {
				b.WriteString(t.Raw)
			}
			t := n.token(b.String(), toks[i].Start, toks[j-1].End)
			out = append(out, t)
			i = j
			continue
		}
		out = append(out, toks[i])
		i++
	}
	_ = text
	return out
}

// expand replaces teencode tokens by their expansion (each expanded token keeps the span of the original one).
func (n *Normalizer) expand(toks []Token) []Token {
	if len(n.teencode) == 0 {
		return toks
	}
	out := make([]Token, 0, len(toks))
	for _, t := range toks {
		exp, ok := n.teencode[t.Raw]
		if !ok {
			out = append(out, t)
			continue
		}
		for _, c := range exp {
			f := Fold(c)
			out = append(out, Token{Start: t.Start, End: t.End, Raw: t.Raw, Canon: c, Fold: f, Diacritic: f != c})
		}
	}
	return out
}

// Fold removes Vietnamese diacritics (vi_fold): NFD, combining marks dropped, đ → d.
func Fold(s string) string {
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
	return b.String()
}

// HasDiacritics reports a Vietnamese mark anywhere in s.
func HasDiacritics(s string) bool { return Fold(s) != s }

// Phrase is a pattern of tokens (a term, an allowlist phrase, a PII phrase) in matching form.
type Phrase []Token

// Phrase normalises a configured phrase with the same rules as the texts.
func (n *Normalizer) Phrase(s string) Phrase { return Phrase(n.Tokens(norm.NFC.String(s))) }

// TokenMatches reports whether a text token matches a phrase token: same canonical form, or — only when the text token
// carries no diacritic — the same folded form ("ngủ" never matches "ngu"; "do ngoc" matches "đồ ngốc", AC-MOD-002).
func TokenMatches(text, pat Token) bool {
	if text.Canon == pat.Canon {
		return true
	}
	return !text.Diacritic && text.Fold == pat.Fold
}

// MatchAt reports whether p matches toks starting at i.
func (p Phrase) MatchAt(toks []Token, i int) bool {
	if len(p) == 0 || i+len(p) > len(toks) {
		return false
	}
	for k, pt := range p {
		if !TokenMatches(toks[i+k], pt) {
			return false
		}
	}
	return true
}
