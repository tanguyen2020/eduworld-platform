package page

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCursorRoundTripAndTamper(t *testing.T) {
	c, err := NewCodec([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	in := Cursor{Key: "th-demo-a", ID: uuid.New()}
	s := c.Encode(in)
	out, err := c.Decode(s)
	if err != nil || *out != in {
		t.Fatalf("round trip = %+v %v", out, err)
	}
	if cur, err := c.Decode(""); cur != nil || err != nil {
		t.Fatal("empty cursor is the first page")
	}
	other, _ := NewCodec([]byte(strings.Repeat("x", 32)))
	if _, err := other.Decode(s); !errors.Is(err, ErrInvalidCursor) {
		t.Fatal("cursor of another key accepted")
	}
	b := []byte(s)
	b[3] ^= 1
	for _, bad := range []string{string(b), "!!", "abc"} {
		if _, err := c.Decode(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("tampered cursor %q accepted", bad)
		}
	}
	if _, err := NewCodec([]byte("short")); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestLimit(t *testing.T) {
	for in, want := range map[int]int{0: 20, -1: 20, 5: 5, 100: 100, 101: 100} {
		if got := Limit(in); got != want {
			t.Errorf("Limit(%d) = %d, want %d", in, got, want)
		}
	}
}
