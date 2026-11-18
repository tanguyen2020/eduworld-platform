// Package port declares the ports of module identity's application layer that infrastructure
// implements. Write-side repositories live in port/write, read models in port/read; external
// integrations (Keycloak Admin API) are declared here.
package port

import (
	"context"
	"errors"
)

// Keycloak Admin errors. Transient failures (timeouts, 5xx after retries, open circuit) are returned as
// other errors: consumers Nak and retry them (F010-AC-23), never record a fake state.
var (
	ErrKCNotFound = errors.New("keycloak: not found")
	ErrKCConflict = errors.New("keycloak: conflict")
)

// KCOrganization is a Keycloak Organization (one per tenant, KEYCLOAK_SSO §2.1).
type KCOrganization struct {
	ID         string
	Alias      string
	Name       string
	Enabled    bool
	Attributes map[string][]string
}

// KCUser is a Keycloak user reference.
type KCUser struct {
	ID       string
	Username string
	Enabled  bool
}

// KCNewUser is a local Keycloak user created by module identity (common tenant, ACCOUNT_PROVISIONING §3.4).
type KCNewUser struct {
	Username   string
	FirstName  string
	Enabled    bool
	Attributes map[string][]string
}

// KeycloakAdmin is the only path to the Keycloak Admin API (client svc-identity, ADR-0074 §5, AGENTS R29):
// every call has a timeout; only idempotent calls are retried; a circuit breaker protects Keycloak.
type KeycloakAdmin interface {
	// FindOrganizationByAlias returns ErrKCNotFound when no Organization has the alias.
	FindOrganizationByAlias(ctx context.Context, alias string) (KCOrganization, error)
	// EnsureOrganization creates the Organization or, when the alias exists, returns its id (idempotent).
	EnsureOrganization(ctx context.Context, org KCOrganization) (string, error)
	UpdateOrganization(ctx context.Context, org KCOrganization) error
	ListOrganizationMembers(ctx context.Context, orgID string) ([]KCUser, error)
	// AddOrganizationMember and RemoveOrganizationMember succeed when already in the target state.
	AddOrganizationMember(ctx context.Context, orgID, userID string) error
	RemoveOrganizationMember(ctx context.Context, orgID, userID string) error
	// FindUserByUsername returns ErrKCNotFound when absent.
	FindUserByUsername(ctx context.Context, username string) (KCUser, error)
	// CreateUser returns ErrKCConflict when the username is taken (never retried).
	CreateUser(ctx context.Context, u KCNewUser) (string, error)
	// SetPassword sets the password; the value is never logged or traced.
	SetPassword(ctx context.Context, userID, password string, temporary bool) error
	AddRealmRoles(ctx context.Context, userID string, roles ...string) error
	RemoveRealmRoles(ctx context.Context, userID string, roles ...string) error
	// LogoutUser revokes every Keycloak session of the user.
	LogoutUser(ctx context.Context, userID string) error
}
