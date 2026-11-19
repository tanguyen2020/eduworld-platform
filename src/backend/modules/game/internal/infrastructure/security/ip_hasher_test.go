package security

import (
	"bytes"
	"strings"
	"testing"
)

func TestIPHasher(t *testing.T) {
	h := NewIPHasher([]byte(strings.Repeat("k", 32)))
	a, b := h.Hash("203.0.113.7:51234"), h.Hash("203.0.113.7")
	if len(a) != 32 || !bytes.Equal(a, b) {
		t.Fatalf("hash: %x %x", a, b)
	}
	if bytes.Contains(a, []byte("203.0.113.7")) || bytes.Equal(a, h.Hash("203.0.113.8")) {
		t.Fatal("hash leaks or collides")
	}
	if other := NewIPHasher([]byte(strings.Repeat("x", 32))).Hash("203.0.113.7"); bytes.Equal(a, other) {
		t.Fatal("key not used")
	}
	if NewIPHasher(nil).Hash("203.0.113.7") != nil || h.Hash("") != nil {
		t.Fatal("no key or no address must give no hash")
	}
}
