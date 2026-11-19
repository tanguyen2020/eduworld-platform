package user

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeUsername(t *testing.T) {
	for in, want := range map[string]string{" HS.TuDo ": "hs.tudo", "hs_an-3a": "hs_an-3a", "ABC": "abc"} {
		got, err := NormalizeUsername(in)
		if err != nil || got != want {
			t.Errorf("%q → %q, %v (want %q)", in, got, err, want)
		}
	}
	for _, bad := range []string{"ab", "hs@truong.vn", "hs tudo", "học.sinh", strings.Repeat("a", 129), ""} {
		var inv ErrInvalid
		if _, err := NormalizeUsername(bad); !errors.As(err, &inv) || inv.Field != "username" {
			t.Errorf("%q accepted (%v)", bad, err)
		}
	}
}

func TestGrade(t *testing.T) {
	for _, g := range []int{1, 9} {
		if CheckGrade(g) != nil {
			t.Fatalf("grade %d rejected", g)
		}
	}
	for _, g := range []int{0, 10} {
		if CheckGrade(g) == nil {
			t.Fatalf("grade %d accepted", g) // F010-AC-25
		}
	}
	if TemporaryPassword(5) || !TemporaryPassword(6) || !TemporaryPassword(9) {
		t.Fatal("temporary password: grades 6–9 only (F010-AC-29)")
	}
}

func TestGeneratePassword(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		p, err := GeneratePassword("hs.tudo")
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != passwordLength || strings.ContainsAny(p, "0o1li") {
			t.Fatalf("password %q: wrong length or ambiguous characters", p)
		}
		if seen[p] {
			t.Fatalf("duplicate password %q in 200 draws", p)
		}
		seen[p] = true
	}
}

func TestDisplayNameAndExternalRef(t *testing.T) {
	if n, err := CheckDisplayName("  Nguyễn An "); err != nil || n != "Nguyễn An" {
		t.Fatalf("display name = %q %v", n, err)
	}
	if _, err := CheckDisplayName(" "); err == nil {
		t.Fatal("empty display name accepted")
	}
	if _, err := CheckExternalRef(strings.Repeat("x", 65)); err == nil {
		t.Fatal("external_ref > 64 accepted")
	}
}

// F011-AC-12.
func TestExternalIdentityKey(t *testing.T) {
	allow := []string{"https://sso.eduworld.vn/realms/eduworld", "https://idp.school.vn"}
	k, err := NewExternalIdentityKey(allow, " https://idp.school.vn ", " 1234-abcd ")
	if err != nil || k != (ExternalIdentityKey{Issuer: "https://idp.school.vn", Subject: "1234-abcd"}) {
		t.Fatalf("key = %+v %v", k, err)
	}
	for _, c := range [][2]string{{"https://evil.example", "1"}, {"", "1"}, {ExternalRefIssuer, "1"}, {allow[0], ""}} {
		if _, err := NewExternalIdentityKey(allow, c[0], c[1]); err == nil {
			t.Fatalf("%v accepted", c)
		}
	}
	if _, err := NewExternalIdentityKey(nil, allow[0], "1"); err == nil {
		t.Fatal("empty allowlist accepted")
	}
}
