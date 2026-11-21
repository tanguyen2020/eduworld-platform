package dto

import (
	"time"

	"github.com/google/uuid"
)

// PortalLogin is the input of portalLogin: username and password only (ADR-0094). The password is zeroed by
// the use case after the Keycloak call (best effort, F012-AC-31).
type PortalLogin struct {
	Username string
	Password []byte
	DeviceID uuid.UUID
	Locale   string
}

// PortalSession is a successful login or refresh: the access token goes to the body, the refresh token and
// CSRF value to cookies (ADR-0091 §4).
type PortalSession struct {
	AccessToken      string
	RefreshToken     string
	CSRF             string
	ExpiresIn        int
	RefreshExpiresIn int
	AuthTime         time.Time // password entry of the session
}

// PortalSetupPassword is the input of setupPortalPassword.
type PortalSetupPassword struct {
	SetupTicket string
	NewPassword []byte
}

// PortalSetupResult is the output of setupPortalPassword.
type PortalSetupResult struct {
	RemainingSteps []string
}

// PortalForgotPassword is the input of forgotPortalPassword.
type PortalForgotPassword struct {
	Identifier string
	DeviceID   uuid.UUID
	Locale     string
}

// PortalResetPassword is the input of resetPortalPassword.
type PortalResetPassword struct {
	Token       string
	NewPassword []byte
	DeviceID    uuid.UUID
}
