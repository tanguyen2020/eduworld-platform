package portalauth

import (
	"regexp"
	"slices"
	"testing"
)

func TestSteps(t *testing.T) {
	if s := Steps([]string{"CONFIGURE_TOTP", "VERIFY_EMAIL", "UPDATE_PASSWORD"}); !slices.Equal(s, []string{StepUpdatePassword, StepConfigureOTP}) {
		t.Fatalf("steps = %v", s)
	}
	if s := Steps([]string{"CONFIGURE_TOTP"}); !OnlyOTPLeft(s) {
		t.Fatalf("otp only = %v", s)
	}
	if OnlyOTPLeft(Steps([]string{"UPDATE_PASSWORD", "CONFIGURE_TOTP"})) || len(Steps(nil)) != 0 {
		t.Fatal("unexpected")
	}
}

// F012-AC-27, AC-29: staff with a live status and a portal or platform role only.
func TestDecide(t *testing.T) {
	cases := []struct {
		a    *Account
		want Decision
	}{
		{nil, StaffOnly},
		{&Account{UserType: "student", Status: "active", HasPortalRole: true}, StaffOnly},
		{&Account{UserType: "staff", Status: "active", HasPortalRole: false}, StaffOnly},
		{&Account{UserType: "staff", Status: "deactivated", HasPortalRole: true}, Deactivated},
		{&Account{UserType: "staff", Status: "locked", HasPortalRole: true}, Deactivated},
		{&Account{UserType: "staff", Status: "anonymized", HasPortalRole: true}, Deactivated},
		{&Account{UserType: "staff", Status: "active", HasPortalRole: true}, Allowed},
		{&Account{UserType: "staff", Status: "pending", HasPortalRole: true}, Allowed},
	}
	for _, c := range cases {
		if got := Decide(c.a); got != c.want {
			t.Errorf("%+v → %v, want %v", c.a, got, c.want)
		}
	}
}

func TestMFAAndKeys(t *testing.T) {
	if !MFA([]string{"pwd", "otp"}, []string{"otp"}) || MFA([]string{"pwd"}, []string{"otp"}) {
		t.Fatal("mfa")
	}
	if NormalizeLogin("  Ad.Truong.A@Example.TEST ") != "ad.truong.a@example.test" {
		t.Fatal("normalize")
	}
	k := Key("login", "ad.truong.a")
	if k == Key("device", "ad.truong.a") || len(k) != 32 || regexp.MustCompile(`truong`).MatchString(k) {
		t.Fatalf("key %s", k)
	}
	if SessionKey("s1") != Key("sid", "s1") || len(HashToken("x")) != 64 {
		t.Fatal("hashes")
	}
}

func TestNewSecret(t *testing.T) {
	a, err := NewSecret(SecretBytes)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewSecret(SecretBytes)
	if a == b || len(a) != 43 || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(a) {
		t.Fatalf("secret %q", a)
	}
}
