// Package gamelogin holds the rules of the game login through the backend (F013, ADR-0068, ADR-0083,
// ADR-0086): username form, minimum client version, the tenant decision after Keycloak authenticated the
// student, the curfew window (ADR-0078) and the GameAuthSession aggregate (refresh rotation, reuse
// detection, device binding, revocation). Standard library and uuid only.
package gamelogin

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

// NormalizeUsername trims, applies NFC and lower-cases a game username (KEYCLOAK_SSO §3.3). ok is false for
// an empty username or one containing "@": the game form never accepts emails (F013-AC-06, ADR-0069).
func NormalizeUsername(s string) (string, bool) {
	u := strings.ToLower(norm.NFC.String(strings.TrimSpace(s)))
	if u == "" || strings.Contains(u, "@") {
		return "", false
	}
	return u, true
}

// Version is a client version major.minor.patch with an optional pre-release (GameLoginRequest.client_version).
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

// ErrVersion: the value is not major.minor.patch[-pre|+build].
var ErrVersion = errors.New("gamelogin: invalid client version")

// ParseVersion parses a client version; build metadata (+...) is ignored, a pre-release (-...) ranks
// below the release of the same numbers (semver §11).
func ParseVersion(s string) (Version, error) {
	core, pre := s, ""
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		core = s[:i]
		if s[i] == '-' {
			pre = s[i+1:]
			if j := strings.IndexByte(pre, '+'); j >= 0 {
				pre = pre[:j]
			}
		}
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, ErrVersion
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 || p == "" {
			return Version{}, ErrVersion
		}
		n[i] = v
	}
	return Version{Major: n[0], Minor: n[1], Patch: n[2], Pre: pre}, nil
}

// Less reports v < o.
func (v Version) Less(o Version) bool {
	if c := compareInts([]int{v.Major, v.Minor, v.Patch}, []int{o.Major, o.Minor, o.Patch}); c != 0 {
		return c < 0
	}
	switch {
	case v.Pre == o.Pre:
		return false
	case v.Pre == "":
		return false
	case o.Pre == "":
		return true
	}
	return v.Pre < o.Pre
}

func compareInts(a, b []int) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Supported reports whether client meets min (min_client_version, EW_GAME_MIN_CLIENT_VERSION). An empty
// min accepts every version; an unparsable client is unsupported.
func Supported(client, minVersion string) bool {
	if minVersion == "" {
		return true
	}
	m, err := ParseVersion(minVersion)
	if err != nil {
		return true // a broken configuration must not lock every student out; it is reported at startup
	}
	c, err := ParseVersion(client)
	if err != nil {
		return false
	}
	return !c.Less(m)
}

// Tenant resolutions recorded in identity.login_events.tenant_resolution.
const (
	ResolutionSingle   = "single_membership"
	ResolutionCommon   = "common"
	ResolutionSelected = "selected"
	ResolutionMismatch = "mismatch"
)

// Membership is one tenant of a student (identity.memberships_for_user joined with platform.tenants).
type Membership struct {
	TenantID     uuid.UUID
	TenantType   string // school | unit | common
	TenantName   string
	TenantStatus string
	MemberKind   string // student | staff
	SchoolID     *uuid.UUID
}

// Outcome of the tenant decision.
type Outcome int

const (
	NoTenant  Outcome = iota // ACCOUNT_NOT_PROVISIONED (or NO_ACTIVE_ENROLLMENT, decided by the caller)
	OneTenant                // enter the tenant directly
	Choose                   // TENANT_NOT_SELECTED + choices + login_ticket
)

// Decision is the result of Decide.
type Decision struct {
	Outcome    Outcome
	Tenant     Membership   // OneTenant
	Choices    []Membership // Choose (ordered by tenant name, then id)
	Resolution string
}

// MaxChoices bounds GameLoginProblem.choices (OpenAPI maxItems).
const MaxChoices = 20

// Decide applies IDENTITY_MAPPING §3: only student memberships of active tenants count, one row per
// tenant; none → NoTenant; one → OneTenant (common tenant recorded as such); several → Choose.
func Decide(ms []Membership) Decision {
	seen := map[uuid.UUID]bool{}
	var live []Membership
	for _, m := range ms {
		if m.MemberKind != "student" || m.TenantStatus != "active" || seen[m.TenantID] {
			continue
		}
		seen[m.TenantID] = true
		live = append(live, m)
	}
	switch len(live) {
	case 0:
		return Decision{Outcome: NoTenant}
	case 1:
		r := ResolutionSingle
		if live[0].TenantType == "common" {
			r = ResolutionCommon
		}
		return Decision{Outcome: OneTenant, Tenant: live[0], Resolution: r}
	}
	slices.SortFunc(live, func(a, b Membership) int {
		if c := strings.Compare(a.TenantName, b.TenantName); c != 0 {
			return c
		}
		return strings.Compare(a.TenantID.String(), b.TenantID.String())
	})
	if len(live) > MaxChoices {
		live = live[:MaxChoices]
	}
	return Decision{Outcome: Choose, Choices: live}
}

// Find returns the membership of tenant among ms (student, active tenant).
func Find(ms []Membership, tenant uuid.UUID) (Membership, bool) {
	for _, m := range ms {
		if m.TenantID == tenant && m.MemberKind == "student" && m.TenantStatus == "active" {
			return m, true
		}
	}
	return Membership{}, false
}

// Curfew is a daily window [Start, End) in minutes after midnight of its location (ADR-0078); End < Start
// spans midnight (21:30–06:30). The zero value is "no curfew".
type Curfew struct {
	Start, End int
	Loc        *time.Location
}

// ParseCurfew parses "HH:MM-HH:MM" (platform_curfew); an empty string is no curfew.
func ParseCurfew(s string, loc *time.Location) (Curfew, error) {
	if s == "" {
		return Curfew{}, nil
	}
	a, b, ok := strings.Cut(strings.ReplaceAll(s, "–", "-"), "-")
	if !ok {
		return Curfew{}, errors.New("gamelogin: curfew must be HH:MM-HH:MM")
	}
	start, err := clock(strings.TrimSpace(a))
	if err != nil {
		return Curfew{}, err
	}
	end, err := clock(strings.TrimSpace(b))
	if err != nil {
		return Curfew{}, err
	}
	if loc == nil {
		loc = time.UTC
	}
	return Curfew{Start: start, End: end, Loc: loc}, nil
}

func clock(s string) (int, error) {
	h, m, ok := strings.Cut(s, ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, errors.New("gamelogin: curfew time must be HH:MM")
	}
	return hh*60 + mm, nil
}

// Zero reports the absence of a curfew.
func (c Curfew) Zero() bool { return c.Loc == nil || c.Start == c.End }

// Active reports whether now falls in the curfew and, if so, when it ends (Problem.reopen_at).
func (c Curfew) Active(now time.Time) (bool, time.Time) {
	if c.Zero() {
		return false, time.Time{}
	}
	t := now.In(c.Loc)
	minute := t.Hour()*60 + t.Minute()
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, c.Loc)
	endToday := midnight.Add(time.Duration(c.End) * time.Minute)
	if c.Start < c.End {
		if minute >= c.Start && minute < c.End {
			return true, endToday
		}
		return false, time.Time{}
	}
	switch {
	case minute >= c.Start: // evening part: ends tomorrow
		return true, midnight.AddDate(0, 0, 1).Add(time.Duration(c.End) * time.Minute)
	case minute < c.End: // morning part
		return true, endToday
	}
	return false, time.Time{}
}

// HashToken is the SHA-256 hex of a refresh token (only the hash is stored, F013-AC-15).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// MaxPreviousHashes bounds the rotated hashes kept for reuse detection (game_session_sso).
const MaxPreviousHashes = 20

// Revoke reasons (identity.game_auth_sessions.revoke_reason).
const (
	RevokeLogout          = "logout"
	RevokeTakeover        = "takeover"
	RevokePasswordChanged = "password_changed"
	RevokeAccountLocked   = "account_locked"
	RevokeSanction        = "sanction"
	RevokeAccessRevoked   = "access_revoked"
	RevokeCleanup         = "cleanup"
	RevokeRefreshReuse    = "refresh_reuse"
	RevokeAdmin           = "admin"
)

// ValidRevokeReason reports whether r is one of the revoke reasons.
func ValidRevokeReason(r string) bool {
	switch r {
	case RevokeLogout, RevokeTakeover, RevokePasswordChanged, RevokeAccountLocked, RevokeSanction, RevokeAccessRevoked,
		RevokeCleanup, RevokeRefreshReuse, RevokeAdmin:
		return true
	}
	return false
}

// Session is the GameAuthSession aggregate (ADR-0086): one game login on one device.
type Session struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	UserID          uuid.UUID
	KeycloakSID     string
	DeviceID        uuid.UUID
	Platform        string
	CurrentHash     string
	PreviousHashes  []string // newest first
	CreatedAt       time.Time
	LastRefreshedAt time.Time
	RevokedAt       *time.Time
	RevokeReason    string
	Version         int64
}

// Class is the classification of a presented refresh token.
type Class int

const (
	Current        Class = iota // the live token of the session from its device: refresh with Keycloak
	Reused                      // a rotated token: REFRESH_TOKEN_REUSED, revoke the whole session
	DeviceMismatch              // another device: REFRESH_TOKEN_DEVICE_MISMATCH, the session stays
	Revoked                     // the session is revoked: REFRESH_TOKEN_INVALID
)

// Classify compares the hash of the presented token with the session (constant time). A rotated token from
// another device is a device mismatch: a caller that only knows an old token and a wrong device id must not
// be able to end the session of the real device (research R4).
func (s *Session) Classify(hash string, device uuid.UUID) Class {
	if s.RevokedAt != nil {
		return Revoked
	}
	if eq(hash, s.CurrentHash) {
		if device != s.DeviceID {
			return DeviceMismatch
		}
		return Current
	}
	if device != s.DeviceID {
		return DeviceMismatch
	}
	return Reused
}

// Rotate records a new current token; the previous one joins the rotated hashes (newest first, at most
// MaxPreviousHashes).
func (s *Session) Rotate(newHash string, at time.Time) {
	prev := append([]string{s.CurrentHash}, s.PreviousHashes...)
	if len(prev) > MaxPreviousHashes {
		prev = prev[:MaxPreviousHashes]
	}
	s.PreviousHashes = prev
	s.CurrentHash = newHash
	s.LastRefreshedAt = at
}

// Revoke ends the session; false when it was already revoked (idempotent).
func (s *Session) Revoke(reason string, at time.Time) bool {
	if s.RevokedAt != nil {
		return false
	}
	s.RevokedAt = &at
	s.RevokeReason = reason
	return true
}

func eq(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
