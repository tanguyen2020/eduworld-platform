package ids_test

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

func TestNewIsUUIDv7AndOrdered(t *testing.T) {
	a := ids.New()
	time.Sleep(2 * time.Millisecond)
	b := ids.New()
	if a.Version() != 7 || b.Version() != 7 {
		t.Fatalf("want UUIDv7, got v%d v%d", a.Version(), b.Version())
	}
	if a.String() >= b.String() {
		t.Fatalf("UUIDv7 must sort by time: %s >= %s", a, b)
	}
}

func TestParseV7(t *testing.T) {
	id := ids.New()
	got, err := ids.ParseV7(id.String())
	if err != nil || got != id {
		t.Fatalf("ParseV7(%s) = %v, %v", id, got, err)
	}
	if _, err := ids.ParseV7("4d3c1b6e-8f3a-4b1c-9d2e-0a1b2c3d4e5f"); err == nil {
		t.Fatal("v4 UUID accepted as v7")
	}
	if _, err := ids.ParseV7("abc"); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestTime(t *testing.T) {
	before := time.Now().Add(-time.Second)
	id := ids.New()
	ts, ok := ids.Time(id)
	if !ok || ts.Before(before) || ts.After(time.Now().Add(time.Second)) {
		t.Fatalf("Time(%s) = %v, %v", id, ts, ok)
	}
}
