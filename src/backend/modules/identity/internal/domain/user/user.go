// Package user holds the user rules of module identity: login identifiers (ADR-0067, ADR-0069), user
// types (ADR-0081) and the initial credentials of common-tenant students (ACCOUNT_PROVISIONING §3.4).
package user

import (
	"crypto/rand"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Type of user (ADR-0081): there is no teacher/admin type.
type Type string

const (
	TypeStudent Type = "student"
	TypeStaff   Type = "staff"
)

// ErrInvalid is a rule violation on one field.
type ErrInvalid struct{ Field, Reason string }

func (e ErrInvalid) Error() string { return "user: " + e.Field + " " + e.Reason }

var usernameRe = regexp.MustCompile(`^[a-z0-9._-]{3,128}$`)

// NormalizeUsername trims, applies Unicode NFC and lower-cases (ADR-0067), then checks the shared pattern
// ^[A-Za-z0-9._-]{3,128}$ (no "@": emails are a separate identifier).
func NormalizeUsername(s string) (string, error) {
	u := strings.ToLower(norm.NFC.String(strings.TrimSpace(s)))
	if !usernameRe.MatchString(u) {
		return "", ErrInvalid{Field: "username", Reason: "must match ^[A-Za-z0-9._-]{3,128}$"}
	}
	return u, nil
}

// CheckDisplayName validates a display name (1–128 characters).
func CheckDisplayName(s string) (string, error) {
	n := strings.TrimSpace(norm.NFC.String(s))
	if l := utf8.RuneCountInString(n); l < 1 || l > 128 {
		return "", ErrInvalid{Field: "display_name", Reason: "must be 1–128 characters"}
	}
	return n, nil
}

// CheckGrade validates a grade level 1–9 (FR-ORG-017).
func CheckGrade(g int) error {
	if g < 1 || g > 9 {
		return ErrInvalid{Field: "grade_level", Reason: "must be 1–9"}
	}
	return nil
}

// TemporaryPassword reports whether the initial password must be changed at first login: grades 6–9
// yes, grades 1–5 no (ACCOUNT_PROVISIONING §3.4 [Proposed]).
func TemporaryPassword(grade int) bool { return grade >= 6 }

// passwordAlphabet avoids look-alike characters (0/o, 1/l/i) because the password is printed on a
// login card for young students; 10 characters ≈ 49 bits, above the realm policy length(6).
const (
	passwordAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	passwordLength   = 10
)

// ErrRandom: the system random source failed.
var ErrRandom = errors.New("user: random source unavailable")

// GeneratePassword returns a random initial password; it is never stored or logged (AGENTS R14).
func GeneratePassword(username string) (string, error) {
	for {
		b := make([]byte, passwordLength)
		max := big.NewInt(int64(len(passwordAlphabet)))
		for i := range b {
			n, err := rand.Int(rand.Reader, max)
			if err != nil {
				return "", ErrRandom
			}
			b[i] = passwordAlphabet[n.Int64()]
		}
		p := string(b)
		if !strings.Contains(username, p) { // realm policy notUsername
			return p, nil
		}
	}
}

// ExternalRefIssuer is the issuer of common-tenant external references: never an allowed login issuer.
const ExternalRefIssuer = "platform_common"

// CheckExternalRef validates an optional external reference (≤ 64 characters).
func CheckExternalRef(s string) (string, error) {
	v := strings.TrimSpace(s)
	if utf8.RuneCountInString(v) > 64 {
		return "", ErrInvalid{Field: "external_ref", Reason: "must be at most 64 characters"}
	}
	return v, nil
}

// ExternalIdentityKey is the key of an external identity: (issuer, subject) only — never an email, a
// display name or preferred_username (F011-AC-12, IDENTITY_MAPPING).
type ExternalIdentityKey struct {
	Issuer  string
	Subject string
}

// NewExternalIdentityKey checks the issuer against the configured allowlist (EW_OIDC_ISSUERS) and the
// lengths of identity.external_identities (issuer 512, subject 255).
func NewExternalIdentityKey(allowlist []string, issuer, subject string) (ExternalIdentityKey, error) {
	issuer, subject = strings.TrimSpace(issuer), strings.TrimSpace(subject)
	if subject == "" || len(subject) > 255 {
		return ExternalIdentityKey{}, ErrInvalid{Field: "subject", Reason: "must be 1–255 characters"}
	}
	for _, a := range allowlist {
		if a != "" && a == issuer && len(issuer) <= 512 {
			return ExternalIdentityKey{Issuer: issuer, Subject: subject}, nil
		}
	}
	return ExternalIdentityKey{}, ErrInvalid{Field: "issuer", Reason: "not in the issuer allowlist"}
}
