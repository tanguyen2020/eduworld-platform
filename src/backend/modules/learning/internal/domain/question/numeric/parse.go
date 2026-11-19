// Package numeric parses and compares the answers of numeric questions exactly (QUESTION_BANK §3.6, F022-AC-18):
// numbers are big.Rat, never float; "," and "." are both accepted as decimal separators and the thousands separator
// follows the content language of the question (vi: ".", en: ","), so an ambiguous 1.234 is read by that language;
// fractions a/b are accepted as an equivalent form.
package numeric

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
)

// ErrNotANumber: the text is not a number of the accepted forms.
var ErrNotANumber = errors.New("numeric: not a number")

// Options of a parse.
type Options struct {
	Language        string // vi (default) or en
	AllowFraction   bool   // a/b accepted (allow_fraction_input)
	EquivalentForms bool   // a/b accepted (accept_equivalent_forms)
}

var (
	canonical = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?$`)
	fraction  = regexp.MustCompile(`^([+-]?[0-9]+)/([0-9]+)$`)
	groups    = regexp.MustCompile(`^[+-]?[0-9]{1,3}(\x00[0-9]{3})*$`)
)

// MaxLen is the longest accepted answer text (numericResponse.value).
const MaxLen = 64

// Parse reads s as an exact number.
func Parse(s string, o Options) (*big.Rat, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > MaxLen {
		return nil, ErrNotANumber
	}
	s = strings.NewReplacer(" ", "", " ", "", " ", "", "'", "").Replace(s)
	if m := fraction.FindStringSubmatch(s); m != nil {
		if !o.AllowFraction && !o.EquivalentForms {
			return nil, ErrNotANumber
		}
		num, ok1 := new(big.Int).SetString(m[1], 10)
		den, ok2 := new(big.Int).SetString(m[2], 10)
		if !ok1 || !ok2 || den.Sign() == 0 {
			return nil, ErrNotANumber
		}
		return new(big.Rat).SetFrac(num, den), nil
	}
	thousands := "."
	if o.Language == "en" {
		thousands = ","
	}
	canon, err := canonicalize(s, thousands)
	if err != nil {
		return nil, err
	}
	r, ok := new(big.Rat).SetString(canon)
	if !ok {
		return nil, ErrNotANumber
	}
	return r, nil
}

// canonicalize rewrites s with "." as the only decimal separator and no thousands separators.
func canonicalize(s, thousands string) (string, error) {
	dots, commas := strings.Count(s, "."), strings.Count(s, ",")
	intPart, frac := s, ""
	switch {
	case dots > 0 && commas > 0:
		// The last separator is the decimal one; the other separates thousands.
		dec := "."
		if strings.LastIndex(s, ",") > strings.LastIndex(s, ".") {
			dec = ","
		}
		i := strings.LastIndex(s, dec)
		intPart, frac = s[:i], s[i+1:]
		if strings.Contains(frac, ".") || strings.Contains(frac, ",") {
			return "", ErrNotANumber
		}
	case dots+commas == 1:
		sep := "."
		if commas == 1 {
			sep = ","
		}
		i := strings.Index(s, sep)
		if sep == thousands && len(s)-i-1 == 3 && groups.MatchString(strings.Replace(s, sep, "\x00", 1)) {
			intPart = s // a group separator of the language: 1.234 (vi) = 1234
		} else {
			intPart, frac = s[:i], s[i+1:]
			if frac == "" {
				return "", ErrNotANumber
			}
		}
	case dots+commas > 1:
		// Several separators of one kind: thousands groups only.
	}
	if intPart != s || dots+commas > 1 {
		sep := "."
		if strings.Contains(intPart, ",") {
			sep = ","
		}
		if strings.Contains(intPart, sep) && !groups.MatchString(strings.ReplaceAll(intPart, sep, "\x00")) {
			return "", ErrNotANumber
		}
	}
	intPart = strings.NewReplacer(".", "", ",", "").Replace(intPart)
	canon := intPart
	if frac != "" {
		canon += "." + frac
	}
	if !canonical.MatchString(canon) {
		return "", ErrNotANumber
	}
	return canon, nil
}

// FromJSON reads the answer key value (a JSON number literal) exactly.
func FromJSON(lit string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(lit))
	if !ok {
		return nil, ErrNotANumber
	}
	return r, nil
}

// Tolerance of a numeric answer.
type Tolerance struct {
	Type  string // exact (default), absolute, relative
	Value *big.Rat
}

// Within reports x = v (exact), |x − v| ≤ t (absolute) or |x − v| ≤ t × |v| (relative).
func Within(x, v *big.Rat, t Tolerance) bool {
	diff := new(big.Rat).Sub(x, v)
	diff.Abs(diff)
	if t.Value == nil || t.Type == "" || t.Type == "exact" {
		return diff.Sign() == 0
	}
	limit := new(big.Rat).Set(t.Value)
	if t.Type == "relative" {
		limit.Mul(limit, new(big.Rat).Abs(v))
	}
	return diff.Cmp(limit) <= 0
}
