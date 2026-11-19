package numeric

import (
	"math/big"
	"testing"
)

func rat(s string) *big.Rat { r, _ := new(big.Rat).SetString(s); return r }

// F022-AC-18: decimal parse without float, "," and "." by content language, thousands, fractions, tolerances.
func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		o    Options
		want string // "" = error
	}{
		{"12,5", Options{}, "12.5"},
		{"12.5", Options{}, "12.5"},
		{"12,50", Options{}, "12.5"},
		{" 12,5 ", Options{}, "12.5"},
		{"-3,25", Options{}, "-3.25"},
		{"1.234", Options{Language: "vi"}, "1234"},
		{"1.234", Options{Language: "en"}, "1.234"},
		{"1,234", Options{Language: "en"}, "1234"},
		{"1,234", Options{Language: "vi"}, "1.234"},
		{"1.234.567", Options{}, "1234567"},
		{"1.234.567,5", Options{}, "1234567.5"},
		{"1,234,567.5", Options{Language: "en"}, "1234567.5"},
		{"1 234 567", Options{}, "1234567"},
		{"25/2", Options{EquivalentForms: true}, "12.5"},
		{"25 / 2", Options{AllowFraction: true}, "12.5"},
		{"25/2", Options{}, ""},
		{"1/0", Options{EquivalentForms: true}, ""},
		{"0,1", Options{}, "0.1"},
		{"1.23.4", Options{}, ""},
		{"12,5,3", Options{}, ""},
		{"abc", Options{}, ""},
		{"1e3", Options{}, ""},
		{"", Options{}, ""},
		{"0x10", Options{}, ""},
		{"1.2345", Options{Language: "vi"}, "1.2345"},
		{"12.", Options{}, ""},
	}
	for _, c := range cases {
		got, err := Parse(c.in, c.o)
		if c.want == "" {
			if err == nil {
				t.Errorf("%q %+v: %v, want error", c.in, c.o, got)
			}
			continue
		}
		if err != nil || got.Cmp(rat(c.want)) != 0 {
			t.Errorf("%q %+v: %v %v, want %s", c.in, c.o, got, err, c.want)
		}
	}
}

func TestWithin(t *testing.T) {
	v := rat("12.5")
	if !Within(rat("12.5"), v, Tolerance{}) || Within(rat("12.51"), v, Tolerance{Type: "exact"}) {
		t.Fatal("exact")
	}
	if !Within(rat("12.51"), v, Tolerance{Type: "absolute", Value: rat("0.01")}) || Within(rat("12.52"), v, Tolerance{Type: "absolute", Value: rat("0.01")}) {
		t.Fatal("absolute")
	}
	if !Within(rat("13"), v, Tolerance{Type: "relative", Value: rat("0.04")}) || Within(rat("13.1"), v, Tolerance{Type: "relative", Value: rat("0.04")}) {
		t.Fatal("relative")
	}
	// 0.1 + 0.2 = 0.3 exactly (no float error).
	sum := new(big.Rat).Add(rat("0.1"), rat("0.2"))
	if !Within(sum, rat("0.3"), Tolerance{}) {
		t.Fatal("decimal exactness")
	}
	if r, err := FromJSON("12.5"); err != nil || r.Cmp(v) != 0 {
		t.Fatal("FromJSON")
	}
}
