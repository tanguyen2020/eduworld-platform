// Package portalauth holds the rules of the Admin Portal login through the backend (ADR-0091, F012
// US7/US8): who may enter the portal, the account setup steps, one-way keys of identifiers (rate limits,
// audit) and the random secrets (CSRF, setup ticket, reset token). Standard library only.
package portalauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Setup steps of PortalSetupStep (OpenAPI) and the Keycloak required actions behind them. The portal has no
// second factor (ADR-0094): update_password is the only step.
const (
	StepUpdatePassword = "update_password"

	actionUpdatePassword = "UPDATE_PASSWORD"
	// ActionLegacyConfigureTOTP is the Keycloak OTP enrolment of ADR-0091. ADR-0094 removed OTP: the realm
	// disables it and module identity drops it from any account that still carries it, so nobody is blocked
	// by "Account is not fully set up".
	ActionLegacyConfigureTOTP = "CONFIGURE_TOTP"
)

// RealmRolePlatformStaff is the Keycloak realm role of the EduWorld platform team (KEYCLOAK_SSO §4): their
// portal sessions are bounded by the platform session age (ADR-0094 §2).
const RealmRolePlatformStaff = "platform_staff"

// Steps maps Keycloak required actions to the portal setup steps. Other required actions (VERIFY_EMAIL,
// UPDATE_PROFILE, the legacy CONFIGURE_TOTP…) have no portal screen and are ignored.
func Steps(actions []string) []string {
	if slices.Contains(actions, actionUpdatePassword) {
		return []string{StepUpdatePassword}
	}
	return nil
}

// PlatformStaff reports the realm role platform_staff among the roles of a token.
func PlatformStaff(realmRoles []string) bool {
	return slices.Contains(realmRoles, RealmRolePlatformStaff)
}

// Decision is the outcome of the EduWorld checks after Keycloak authenticated the account.
type Decision int

const (
	Allowed     Decision = iota
	StaffOnly            // not a staff account, or no valid portal/platform role (PORTAL_ACCESS_STAFF_ONLY)
	Deactivated          // the EduWorld user is deactivated, locked or anonymized (ACCOUNT_DEACTIVATED)
)

// Account is the EduWorld view of the authenticated user.
type Account struct {
	UserType      string // student | staff (ADR-0081)
	Status        string // pending | active | locked | deactivated | anonymized
	HasPortalRole bool   // a valid UserRole in an active tenant, or a platform role
}

// Decide applies F012-AC-27/AC-29: staff only, live status, at least one portal or platform role. pending
// is accepted (first login, IDENTITY_MAPPING §5).
func Decide(a *Account) Decision {
	switch {
	case a == nil || a.UserType != "staff":
		return StaffOnly
	case a.Status != "active" && a.Status != "pending":
		return Deactivated
	case !a.HasPortalRole:
		return StaffOnly
	}
	return Allowed
}

// NormalizeLogin trims, applies Unicode NFC and lower-cases a username or an email (ADR-0067, ADR-0069).
func NormalizeLogin(s string) string {
	return strings.ToLower(norm.NFC.String(strings.TrimSpace(s)))
}

// Key is a one-way, non-secret key of an identifier for rate limiting and audit: the raw value (username,
// email, device id, session id) never reaches Valkey keys, logs or audit (F012-AC-28, AC-32).
func Key(kind, value string) string {
	sum := sha256.Sum256([]byte(kind + ":" + value))
	return hex.EncodeToString(sum[:16])
}

// SessionKey is the key of a Keycloak session (sid) shared by the session store and the refresh rate limit.
func SessionKey(sid string) string { return Key("sid", sid) }

// HashToken is the SHA-256 hex of a setup ticket or reset token (only the hash is stored).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ErrRandom: the system random source failed.
var ErrRandom = errors.New("portalauth: random source unavailable")

// NewSecret returns n random bytes as base64url without padding (32 bytes → 43 characters, the CSRF value,
// setup ticket and reset token of the contract).
func NewSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", ErrRandom
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SecretBytes is the size of every portal secret (≥ 32 bytes, F012-AC-36).
const SecretBytes = 32
