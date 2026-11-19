package expr

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

func vars(kv ...int64) map[string]*big.Rat {
	names := []string{"a", "b", "c"}
	out := map[string]*big.Rat{}
	for i, v := range kv {
		out[names[i]] = big.NewRat(v, 1)
	}
	return out
}

var declared = map[string]bool{"a": true, "b": true, "c": true}

// F023-AC-15: numbers, declared variables, + − × ÷ ^, comparisons, logic and parentheses only.
func TestEvaluator(t *testing.T) {
	cases := map[string]string{
		"a * b":       "56",
		"a × b":       "56",
		"(a + b) ÷ 3": "5",
		"2 ^ 3 ^ 2":   "512",
		"-a + 10":     "3",
		"a / 4":       "7/4",
		"0.5 * b":     "4",
		"2 ^ -2":      "1/4",
		"a - b - c":   "-2",
		"10 : 4":      "5/2",
	}
	for src, want := range cases {
		n, err := Parse(src, declared)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		v, err := Number(n, vars(7, 8, 1))
		if err != nil || v.RatString() != want {
			t.Errorf("%s = %v (%v), want %s", src, v, err, want)
		}
	}
	truths := map[string]bool{"a != b": true, "a < b && b <= 8": true, "a > b or not (a == 7)": false, "!(a >= b)": true, "a = 7": true}
	for src, want := range truths {
		n, err := Parse(src, declared)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		got, err := Truth(n, vars(7, 8, 1))
		if err != nil || got != want {
			t.Errorf("%s = %v (%v)", src, got, err)
		}
	}
	refused := map[string]error{
		"d + 1":   ErrIdentifier,
		"sqrt(a)": ErrIdentifier,
		"os.exit": ErrSyntax,
		"a +":     ErrSyntax,
		"(a + b":  ErrSyntax,
		"a ; b":   ErrSyntax,
		strings.Repeat("(", 40) + "a" + strings.Repeat(")", 40): ErrTooDeep,
		strings.Repeat("a+", 130) + "a":                         ErrTooLong,
	}
	for src, want := range refused {
		if _, err := Parse(src, declared); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", src, err, want)
		}
	}
	for src, want := range map[string]error{"a / (b - 8)": ErrDivZero, "a ^ 11": ErrExponent, "a ^ 0.5": ErrExponent, "a + (a < b)": ErrType} {
		n, err := Parse(src, declared)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if _, err := Number(n, vars(7, 8, 1)); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", src, err, want)
		}
	}
}

// F023-AC-15, SC-004 (T025): no input panics, recurses past the depth limit or evaluates anything but the grammar.
func FuzzEvaluator(f *testing.F) {
	for _, s := range []string{"a * b", "((a))", "2^10^10", "a/0", "not not a < b", "sqrt(2)", "", "1e999", "9999999999^10"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n, err := Parse(s, declared)
		if err != nil {
			return
		}
		_, _ = Eval(n, vars(3, 4, 5))
	})
}
