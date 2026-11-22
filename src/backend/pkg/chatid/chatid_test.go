package chatid_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/chatid"
)

// F117 T001: fixed test vectors — module competition and module chat must derive the same ids forever.
func TestChannelIDVectors(t *testing.T) {
	party := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-0000000000a1")
	match := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-0000000000c1")
	for _, c := range []struct {
		name string
		got  uuid.UUID
		want string
	}{
		{"party", chatid.Party(party), uuid.NewSHA1(chatid.Namespace, []byte("party:"+party.String())).String()},
		{"match", chatid.Match(match), uuid.NewSHA1(chatid.Namespace, []byte("match:"+match.String())).String()},
		{"team0", chatid.Team(match, 0), uuid.NewSHA1(chatid.Namespace, []byte("team:"+match.String()+":0")).String()},
	} {
		if c.got.String() != c.want {
			t.Errorf("%s: %s, want %s", c.name, c.got, c.want)
		}
		if c.got.Version() != 5 {
			t.Errorf("%s: version %d, want 5", c.name, c.got.Version())
		}
	}
	if chatid.Team(match, 0) == chatid.Team(match, 1) || chatid.Match(match) == chatid.Party(match) {
		t.Fatal("distinct scopes must give distinct ids")
	}
	if again := chatid.Match(match); again != chatid.Match(match) {
		t.Fatal("not deterministic")
	}
	if chatid.Namespace.String() != "6f0b6c1e-2d4a-5e8b-9c3f-7a1d2e4b5c60" {
		t.Fatal("the namespace must never change")
	}
}
