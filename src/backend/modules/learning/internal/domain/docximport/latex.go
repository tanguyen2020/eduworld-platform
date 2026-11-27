package docximport

import "strings"

// NormalizeLatex puts a formula written by Pandoc from Office Math into the form stored with the content (research
// R2): line breaks inside environments become spaces, runs of spaces one space, and the leading/trailing spaces and
// control spaces ("\ " that Pandoc appends after \right.) are removed. The meaning of the formula is unchanged.
func NormalizeLatex(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for {
		t := strings.TrimSpace(s)
		if strings.HasSuffix(t, `\`) && !strings.HasSuffix(t, `\\`) {
			t = t[:len(t)-1]
		}
		if t == s {
			return t
		}
		s = t
	}
}
