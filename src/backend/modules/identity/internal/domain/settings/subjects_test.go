package settings

import (
	"errors"
	"strings"
	"testing"
)

// F015 T054: enabled_subject_codes is a non-empty list of distinct subject codes (1..64 characters, at most 50).
func TestValidateEnabledSubjects(t *testing.T) {
	many := make([]string, MaxEnabledSubjects+1)
	for i := range many {
		many[i] = "s" + strings.Repeat("x", i%10) + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	for name, codes := range map[string][]string{
		"empty":     {},
		"blank":     {""},
		"duplicate": {"math", "math"},
		"too long":  {strings.Repeat("m", 65)},
		"too many":  many,
	} {
		var ie ErrInvalid
		if err := ValidateEnabledSubjects(codes); !errors.As(err, &ie) || ie.Field != KeyEnabledSubjects {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := ValidateEnabledSubjects([]string{"math", "vietnamese", strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
}
