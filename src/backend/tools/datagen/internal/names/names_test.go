package names

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFullIsDeterministicAndBounded(t *testing.T) {
	a, b := rand.New(rand.NewPCG(1, 2)), rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 200; i++ {
		x, y := Full(a), Full(b)
		if x != y {
			t.Fatalf("same seed, different names %q %q", x, y)
		}
		if len(strings.Fields(x)) != 3 || utf8.RuneCountInString(x) > 128 { // full_name maxLength 128 (SIS contract)
			t.Fatalf("bad name %q", x)
		}
	}
}
