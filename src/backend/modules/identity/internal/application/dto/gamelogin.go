package dto

import (
	"time"

	"github.com/google/uuid"
)

// GameLogin is the body of gameLogin. Password is a byte slice zeroed by the use case after the Keycloak
// call (F013-AC-16); the struct is never logged.
type GameLogin struct {
	Username      string
	Password      []byte
	DeviceID      uuid.UUID
	ClientVersion string
	Platform      string
	Locale        string
	IPHash        []byte // HMAC of the client IP (never the raw IP), nil when the key is not configured
}

// GameTenantSelect is the body of selectGameLoginTenant.
type GameTenantSelect struct {
	LoginTicket string
	TenantID    uuid.UUID
	DeviceID    uuid.UUID
	IPHash      []byte
}

// GameRefresh is the body of refreshGameToken plus the X-Tenant-Id header.
type GameRefresh struct {
	RefreshToken  string
	DeviceID      uuid.UUID
	Mode          string // renew | resume
	ClientVersion string
	TenantID      uuid.UUID
}

// Refresh modes of GameTokenRefreshRequest.mode.
const (
	RefreshRenew  = "renew"
	RefreshResume = "resume"
)

// GameTokenSet is the GameTokenSet schema.
type GameTokenSet struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

// GameLoginResult is GameLoginResponse (login, tenant select) and GameTokenRefreshResponse (bootstrap only
// with mode = resume).
type GameLoginResult struct {
	Tokens    GameTokenSet
	Bootstrap *PlayerBootstrap
}

// GameTenantChoice is GameLoginTenantChoice.
type GameTenantChoice struct {
	TenantID   uuid.UUID `json:"tenant_id"`
	TenantType string    `json:"tenant_type"`
	Name       string    `json:"name"`
}

// BootstrapUser is the Principal schema of the bootstrap (roles are portal roles: none for a student).
type BootstrapUser struct {
	UserID      uuid.UUID `json:"user_id"`
	UserType    string    `json:"user_type"`
	Status      string    `json:"status"`
	DisplayName string    `json:"display_name,omitempty"`
	Roles       []any     `json:"roles"`
}

// LocalizedText is TenantLocalizedName / LocalizedName.
type LocalizedText struct {
	VI string `json:"vi"`
	EN string `json:"en,omitempty"`
}

// PlayerSchool is the PlayerSchool schema.
type PlayerSchool struct {
	SchoolID    uuid.UUID     `json:"school_id"`
	Name        LocalizedText `json:"name"`
	Province    *ProvinceRef  `json:"province,omitempty"`
	SchoolLevel string        `json:"school_level,omitempty"`
	IsVirtual   bool          `json:"is_virtual"`
}

// CurfewPolicy is PlayerBootstrap.policies.play_time.curfew.
type CurfewPolicy struct {
	Start          string `json:"start"`
	End            string `json:"end"`
	WarningMinutes int    `json:"warning_minutes"`
}

// PlayTimePolicy is PlayerBootstrap.policies.play_time.
type PlayTimePolicy struct {
	Curfew *CurfewPolicy `json:"curfew,omitempty"`
}

// BootstrapPolicies is PlayerBootstrap.policies (display only; the server checks). Chat and party policies
// come from F015 settings and are omitted until then.
type BootstrapPolicies struct {
	PlayTime *PlayTimePolicy `json:"play_time,omitempty"`
}

// PlayerBootstrap is the PlayerBootstrap schema built from module identity data only (no game profile).
type PlayerBootstrap struct {
	User           BootstrapUser     `json:"user"`
	TenantID       uuid.UUID         `json:"tenant_id"`
	TenantType     string            `json:"tenant_type"`
	IsCommonTenant bool              `json:"is_common_tenant"`
	TenantName     string            `json:"tenant_name,omitempty"`
	Student        *StudentSummary   `json:"student,omitempty"`
	School         *PlayerSchool     `json:"school,omitempty"`
	GradeBand      string            `json:"grade_band,omitempty"`
	Policies       BootstrapPolicies `json:"policies"`
	FeatureFlags   map[string]bool   `json:"feature_flags"`
	// SchoolID is the school of the student (curfew of the school, F015); not serialized.
	SchoolID *uuid.UUID `json:"-"`
	// Placed reports an effective student with a school and a grade (NO_ACTIVE_ENROLLMENT otherwise, ADR-0095); not
	// serialized.
	Placed bool `json:"-"`
}

// GradeBand maps a grade level 1–9 to the GradeBand enum.
func GradeBand(grade int) string {
	switch {
	case grade >= 1 && grade <= 3:
		return "g1_3"
	case grade <= 5 && grade >= 4:
		return "g4_5"
	case grade <= 7 && grade >= 6:
		return "g6_7"
	case grade <= 9 && grade >= 8:
		return "g8_9"
	}
	return ""
}

// TenantNotSelected carries the data of 403 TENANT_NOT_SELECTED.
type TenantNotSelected struct {
	Choices   []GameTenantChoice
	Ticket    string
	ExpiresAt time.Time
}
