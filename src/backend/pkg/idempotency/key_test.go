package idempotency_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func v7At(t *testing.T, at time.Time) uuid.UUID {
	t.Helper()
	k := uuid.Must(uuid.NewV7())
	ms := uint64(at.UnixMilli())
	for i := 0; i < 6; i++ {
		k[i] = byte(ms >> (40 - 8*i))
	}
	return k
}

func isBadRequest(err error) bool {
	var pe *problem.Error
	return errors.As(err, &pe) && pe.Code == problem.BadRequest
}

// F003-AC-37: UUIDv7 only, timestamp within ttl, key_issued_on from the key (retry after midnight).
func TestParseKey(t *testing.T) {
	ttl := 24 * time.Hour
	now := time.Date(2026, 11, 18, 0, 1, 0, 0, time.UTC)
	for name, raw := range map[string]string{"v4": uuid.NewString(), "garbage": "abc", "empty": ""} {
		if _, _, err := idempotency.ParseKey(raw, now, ttl); !isBadRequest(err) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	if _, _, err := idempotency.ParseKey(v7At(t, now.Add(-25*time.Hour)).String(), now, ttl); !isBadRequest(err) {
		t.Fatalf("stale key accepted: %v", err)
	}
	if _, _, err := idempotency.ParseKey(v7At(t, now.Add(25*time.Hour)).String(), now, ttl); !isBadRequest(err) {
		t.Fatalf("future key accepted: %v", err)
	}
	k := v7At(t, time.Date(2026, 11, 17, 23, 59, 0, 0, time.UTC))
	got, day, err := idempotency.ParseKey(k.String(), now, ttl)
	if err != nil || got != k || !day.Equal(time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("issued on %v (%v): must be the key's day, not the server's", day, err)
	}
}

func TestHashIgnoresKeyOrder(t *testing.T) {
	if idempotency.Hash([]byte(`{"a":1,"b":[1,2]}`)) != idempotency.Hash([]byte("{ \"b\":[1,2], \"a\":1 }")) {
		t.Fatal("hash depends on key order/whitespace")
	}
	if idempotency.Hash([]byte(`{"a":1}`)) == idempotency.Hash([]byte(`{"a":2}`)) {
		t.Fatal("different payloads share a hash")
	}
}
