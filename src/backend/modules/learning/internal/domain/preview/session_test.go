package preview

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLocalize(t *testing.T) {
	var v any
	_ = json.Unmarshal([]byte(`{"stem":{"vi":"Mèo","en":"Cat"},"options":[{"id":"o_a","text":{"vi":"Một"}}],"n":1}`), &v)
	b, _ := json.Marshal(Localize(v, "en"))
	if string(b) != `{"n":1,"options":[{"id":"o_a","text":"Một"}],"stem":"Cat"}` {
		t.Fatalf("got %s", b)
	}
	if Language("en-US", "vi") != "en" || Language("", "en") != "en" || Language("vi-VN", "en") != "vi" {
		t.Fatal("language")
	}
}

func TestSessionOwnership(t *testing.T) {
	a, b, p := uuid.New(), uuid.New(), uuid.New()
	s := Session{PrincipalID: p, TenantID: &a, ExpiresAt: time.Now().Add(time.Minute), Items: []Item{{Position: 1}}}
	if !s.OwnedBy(p, &a) || s.OwnedBy(p, &b) || s.OwnedBy(p, nil) || s.OwnedBy(uuid.New(), &a) {
		t.Fatal("ownership")
	}
	if s.Expired(time.Now()) || !s.Expired(s.ExpiresAt) {
		t.Fatal("expiry")
	}
	if _, ok := s.Item(2); ok {
		t.Fatal("item 2")
	}
}
