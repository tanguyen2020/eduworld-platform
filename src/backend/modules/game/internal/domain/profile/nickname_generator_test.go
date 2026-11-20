package profile_test

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
)

var words = map[string]profile.Words{
	"vi": {Adjectives: []string{"Chăm Chỉ", "Vui Vẻ", "Siêu Tốc Độ Ánh Sáng Rực Rỡ"}, Nouns: []string{"Cú Mèo", "Sóc Nâu"}},
	"en": {Adjectives: []string{"Brave", "Clever"}, Nouns: []string{"Owl", "Panda"}},
}

// F070-AC-07 (domain part): word order per language, two digits, at most 24 characters, distinct, filtered by keep.
func TestNicknameGenerator(t *testing.T) {
	g := profile.NicknameGenerator{Rand: rand.New(rand.NewPCG(1, 2))}
	vi := g.Candidates("vi", words["vi"], 6, 200, nil)
	if len(vi) != 6 {
		t.Fatalf("vi: %v", vi)
	}
	for _, s := range vi {
		parts := strings.Fields(s)
		if utf8.RuneCountInString(s) > profile.MaxSuggestionRunes || len(parts[len(parts)-1]) != 2 ||
			(!strings.HasPrefix(s, "Cú Mèo ") && !strings.HasPrefix(s, "Sóc Nâu ")) {
			t.Errorf("vi suggestion %q", s)
		}
	}
	en := g.Candidates("en", words["en"], 4, 200, func(s string) bool { return !strings.HasPrefix(s, "Brave") })
	for _, s := range en {
		if !strings.HasPrefix(s, "Clever ") {
			t.Errorf("en suggestion %q", s)
		}
	}
	if got := g.Candidates("vi", profile.Words{}, 3, 10, nil); len(got) != 0 {
		t.Fatalf("no words: %v", got)
	}
}

func TestLanguage(t *testing.T) {
	for in, want := range map[string]string{"": "vi", "en-US,en;q=0.9": "en", "vi-VN": "vi", "fr-FR, en;q=0.5": "en", "de": "vi"} {
		if got := profile.Language(in); got != want {
			t.Errorf("%q: %s", in, got)
		}
	}
}
