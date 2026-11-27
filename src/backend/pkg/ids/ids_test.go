package ids_test

import (
	"bytes"
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

func TestFloor(t *testing.T) {
	base := time.Date(2026, 11, 20, 1, 2, 3, 4_000_000, time.UTC)
	f := ids.Floor(base)
	if f.Version() != 7 {
		t.Fatalf("version %d", f.Version())
	}
	if got, _ := ids.Time(f); !got.Equal(base.Truncate(time.Millisecond)) {
		t.Fatalf("time %v", got)
	}
	for i := 0; i < 100; i++ { // ids of the same millisecond or later compare ≥ ids.Floor(t)
		id := ids.New()
		at, _ := ids.Time(id)
		lo, hi := ids.Floor(at), ids.Floor(at.Add(time.Millisecond))
		if bytes.Compare(id[:], lo[:]) < 0 || bytes.Compare(hi[:], id[:]) <= 0 {
			t.Fatalf("%s outside [ids.Floor(%v), ids.Floor(+1ms))", id, at)
		}
	}
}
