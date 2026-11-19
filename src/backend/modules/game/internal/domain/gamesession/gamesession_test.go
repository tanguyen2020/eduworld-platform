package gamesession

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDecide(t *testing.T) {
	now := time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC)
	timeout := 45 * time.Second
	devA, devB := uuid.New(), uuid.New()
	live := &Session{DeviceID: devA, LastHeartbeatAt: now.Add(-10 * time.Second)}
	stale := &Session{DeviceID: devA, LastHeartbeatAt: now.Add(-46 * time.Second)}
	ended := &Session{DeviceID: devA, LastHeartbeatAt: now, EndedAt: &now}
	cases := []struct {
		name     string
		active   *Session
		device   uuid.UUID
		takeover bool
		want     Decision
		reason   string
	}{
		{"none", nil, devB, false, NoActive, ""},
		{"ended", ended, devB, false, NoActive, ""},
		{"other device live", live, devB, false, Conflict, ""},
		{"other device takeover", live, devB, true, TakeOver, ReasonReplaced},
		{"other device stale", stale, devB, false, EndTimedOut, ReasonTimeout},
		{"same device", live, devA, false, EndSameDevice, ReasonLogout},
		{"same device takeover", live, devA, true, EndSameDevice, ReasonLogout},
	}
	for _, c := range cases {
		got := Decide(c.active, c.device, c.takeover, now, timeout)
		if got != c.want || got.EndReason() != c.reason {
			t.Errorf("%s: %v %q, want %v %q", c.name, got, got.EndReason(), c.want, c.reason)
		}
	}
}

func TestSessionLifecycle(t *testing.T) {
	start := time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC)
	s := &Session{StartedAt: start, LastHeartbeatAt: start}
	if err := s.Heartbeat(start.Add(time.Minute)); err != nil || !s.LastHeartbeatAt.Equal(start.Add(time.Minute)) {
		t.Fatalf("heartbeat: %v %v", err, s.LastHeartbeatAt)
	}
	_ = s.Heartbeat(start) // never moves back
	if !s.LastHeartbeatAt.Equal(start.Add(time.Minute)) {
		t.Fatal("heartbeat moved back")
	}
	by := uuid.New()
	if !s.Replace(by, start.Add(2*time.Minute)) || s.EndReason != ReasonReplaced || *s.ReplacedBy != by {
		t.Fatalf("replace: %+v", s)
	}
	if s.End(ReasonLogout, start.Add(3*time.Minute)) || s.EndReason != ReasonReplaced {
		t.Fatal("second end changed the session")
	}
	if err := s.Heartbeat(start.Add(4 * time.Minute)); !errors.Is(err, ErrEnded) {
		t.Fatalf("heartbeat after end: %v", err)
	}
	if s.Duration() != 2*time.Minute {
		t.Fatalf("duration %v", s.Duration())
	}
}

func TestEndReasonsAndPlatforms(t *testing.T) {
	for _, r := range []string{"logout", "replaced_by_other_device", "timeout", "account_locked", "access_revoked", "cleanup", "client_crash", "curfew"} {
		if !ValidEndReason(r) {
			t.Errorf("%s refused", r)
		}
	}
	if ValidEndReason("admin") || ValidPlatform("ps5") || !ValidPlatform("android") {
		t.Fatal("validation")
	}
	long := "Samsung Galaxy Tab A7 Lite Wi-Fi 32GB Gray Edition with a very long marketing name"
	if l := DeviceLabel(&long); len([]rune(*l)) != MaxDeviceLabel {
		t.Fatalf("label %q", *l)
	}
	empty := ""
	if DeviceLabel(nil) != nil || DeviceLabel(&empty) != nil {
		t.Fatal("empty label")
	}
}

func TestSupported(t *testing.T) {
	cases := []struct {
		client, min string
		want        bool
	}{
		{"1.2.0", "1.2.0", true}, {"1.10.0", "1.9.9", true}, {"0.9.0", "1.0.0", false}, {"1.0.0-rc1", "1.0.0", false},
		{"garbage", "1.0.0", false}, {"0.0.1", "", true}, {"0.0.1", "broken", true}, {"2.0.0+build5", "2.0.0", true},
	}
	for _, c := range cases {
		if got := Supported(c.client, c.min); got != c.want {
			t.Errorf("Supported(%q, %q) = %v", c.client, c.min, got)
		}
	}
}

func TestCurfew(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	platform, err := ParseWindow("21:30-06:30", ict)
	if err != nil {
		t.Fatal(err)
	}
	at := func(h, m int) time.Time { return time.Date(2026, 11, 19, h, m, 0, 0, ict) }
	if ok, _ := (Curfew{platform}).Active(at(21, 29)); ok {
		t.Fatal("21:29 is open")
	}
	ok, reopen := (Curfew{platform}).Active(at(21, 35))
	if !ok || !reopen.Equal(time.Date(2026, 11, 20, 6, 30, 0, 0, ict)) {
		t.Fatalf("21:35: %v %v", ok, reopen)
	}
	if ok, reopen := (Curfew{platform}).Active(at(5, 0)); !ok || !reopen.Equal(at(6, 30)) {
		t.Fatalf("05:00: %v %v", ok, reopen)
	}
	// A school extends the curfew: 20:00–22:00 and 06:00–07:00 chain with the platform window.
	evening, _ := NewWindow("20:00", "22:00", ict)
	morning, _ := NewWindow("06:00", "07:00", ict)
	c := Curfew{platform, evening, morning}
	if ok, reopen := c.Active(at(20, 15)); !ok || !reopen.Equal(time.Date(2026, 11, 20, 7, 0, 0, 0, ict)) {
		t.Fatalf("school extension: %v %v", ok, reopen)
	}
	if ok, _ := (Curfew{}).Active(at(23, 0)); ok {
		t.Fatal("no curfew")
	}
	if _, err := ParseWindow("25:00-06:00", ict); err == nil {
		t.Fatal("bad window accepted")
	}
}
