package gamelogin

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNormalizeUsername(t *testing.T) {
	for in, want := range map[string]string{" HS.An.3A ": "hs.an.3a", "hs.an": "hs.an"} {
		if got, ok := NormalizeUsername(in); !ok || got != want {
			t.Fatalf("%q → %q %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "  ", "hs@school.vn"} {
		if _, ok := NormalizeUsername(in); ok {
			t.Fatalf("%q accepted", in)
		}
	}
}

func TestSupported(t *testing.T) {
	cases := []struct {
		client, min string
		ok          bool
	}{
		{"1.4.0", "1.4.0", true}, {"1.4.1", "1.4.0", true}, {"1.10.0", "1.9.9", true}, {"1.3.9", "1.4.0", false},
		{"1.4.0-beta.1", "1.4.0", false}, {"1.4.0+build.7", "1.4.0", true}, {"x.y.z", "1.0.0", false}, {"0.0.1", "", true},
	}
	for _, c := range cases {
		if got := Supported(c.client, c.min); got != c.ok {
			t.Fatalf("Supported(%q,%q) = %v", c.client, c.min, got)
		}
	}
}

func TestDecide(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	st := func(id uuid.UUID, name, typ, status string) Membership {
		return Membership{TenantID: id, TenantName: name, TenantType: typ, TenantStatus: status, MemberKind: "student"}
	}
	if d := Decide(nil); d.Outcome != NoTenant {
		t.Fatal("no membership")
	}
	if d := Decide([]Membership{st(a, "A", "school", "active"), {TenantID: b, MemberKind: "staff", TenantStatus: "active"}}); d.Outcome != OneTenant || d.Tenant.TenantID != a || d.Resolution != ResolutionSingle {
		t.Fatalf("single = %+v", d)
	}
	if d := Decide([]Membership{st(c, "Chung", "common", "active")}); d.Resolution != ResolutionCommon {
		t.Fatalf("common = %+v", d)
	}
	d := Decide([]Membership{st(b, "B", "school", "active"), st(a, "A", "unit", "active"), st(c, "C", "school", "suspended"), st(a, "A", "unit", "active")})
	if d.Outcome != Choose || len(d.Choices) != 2 || d.Choices[0].TenantID != a {
		t.Fatalf("choose = %+v", d)
	}
	if _, ok := Find(d.Choices, c); ok {
		t.Fatal("suspended tenant found")
	}
	var many []Membership
	for i := range 25 {
		many = append(many, st(uuid.New(), fmt.Sprintf("T%02d", i), "school", "active"))
	}
	if d := Decide(many); len(d.Choices) != MaxChoices {
		t.Fatalf("choices = %d", len(d.Choices))
	}
}

func TestCurfew(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	c, err := ParseCurfew("21:30-06:30", ict)
	if err != nil {
		t.Fatal(err)
	}
	at := func(h, m int) time.Time { return time.Date(2026, 11, 19, h, m, 0, 0, ict) }
	if on, reopen := c.Active(at(21, 35)); !on || !reopen.Equal(time.Date(2026, 11, 20, 6, 30, 0, 0, ict)) {
		t.Fatalf("21:35 = %v %v", on, reopen)
	}
	if on, reopen := c.Active(at(6, 25)); !on || !reopen.Equal(at(6, 30)) {
		t.Fatalf("06:25 = %v %v", on, reopen)
	}
	for _, x := range []time.Time{at(6, 30), at(12, 0), at(21, 29)} {
		if on, _ := c.Active(x); on {
			t.Fatalf("%v in curfew", x)
		}
	}
	school, _ := ParseCurfew("21:00-06:30", ict)
	if on, _ := school.Active(at(21, 0)); !on {
		t.Fatal("school curfew from 21:00")
	}
	day, _ := ParseCurfew("12:00-13:00", ict)
	if on, _ := day.Active(at(12, 30)); !on {
		t.Fatal("same-day window")
	}
	if on, _ := (Curfew{}).Active(at(23, 0)); on {
		t.Fatal("zero curfew")
	}
	if _, err := ParseCurfew("25:00-06:00", ict); err == nil {
		t.Fatal("bad time accepted")
	}
}

func TestSessionRotateClassifyRevoke(t *testing.T) {
	dev := uuid.New()
	s := &Session{DeviceID: dev, CurrentHash: HashToken("rt-0")}
	if s.Classify(HashToken("rt-0"), dev) != Current {
		t.Fatal("current")
	}
	if s.Classify(HashToken("rt-0"), uuid.New()) != DeviceMismatch {
		t.Fatal("device mismatch")
	}
	now := time.Now()
	for i := 1; i <= 25; i++ {
		s.Rotate(HashToken(fmt.Sprint("rt-", i)), now)
	}
	if len(s.PreviousHashes) != MaxPreviousHashes || s.PreviousHashes[0] != HashToken("rt-24") {
		t.Fatalf("previous = %d", len(s.PreviousHashes))
	}
	if s.Classify(HashToken("rt-24"), dev) != Reused {
		t.Fatal("reused")
	}
	if s.Classify(HashToken("rt-24"), uuid.New()) != DeviceMismatch {
		t.Fatal("rotated token from another device")
	}
	if !s.Revoke(RevokeLogout, now) || s.Revoke(RevokeAdmin, now) || s.RevokeReason != RevokeLogout {
		t.Fatal("revoke idempotent")
	}
	if s.Classify(HashToken("rt-25"), dev) != Revoked {
		t.Fatal("revoked")
	}
	if !ValidRevokeReason(RevokeCleanup) || ValidRevokeReason("other") {
		t.Fatal("reasons")
	}
}
