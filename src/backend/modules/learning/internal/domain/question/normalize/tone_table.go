package normalize

import "strings"

// Tone placement (QUESTION_BANK §4 step 5): the "new" style puts the tone mark on the second vowel of oa, oe, uy
// (hoà, thuỷ), the "old" style on the first (hòa, thủy). Both are rewritten to the old style so they compare equal.
// This is a table of Vietnamese orthography, not learning content (research R2).
var toneReplacer = strings.NewReplacer(tonePairs()...)

func tonePairs() []string {
	tones := map[rune][5]string{
		'a': {"à", "á", "ả", "ã", "ạ"},
		'e': {"è", "é", "ẻ", "ẽ", "ẹ"},
		'o': {"ò", "ó", "ỏ", "õ", "ọ"},
		'u': {"ù", "ú", "ủ", "ũ", "ụ"},
		'y': {"ỳ", "ý", "ỷ", "ỹ", "ỵ"},
	}
	var pairs []string
	for _, p := range [][2]rune{{'o', 'a'}, {'o', 'e'}, {'u', 'y'}} {
		first, second := p[0], p[1]
		for i := 0; i < 5; i++ {
			pairs = append(pairs, string(first)+tones[second][i], tones[first][i]+string(second))
		}
	}
	return pairs
}
