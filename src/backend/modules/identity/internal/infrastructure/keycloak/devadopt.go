package keycloak

import (
	"context"
	"errors"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// ErrNotInRealmImport: the dev seed links only users of the local realm import (deploy/keycloak/realm-eduworld.dev.json).
var ErrNotInRealmImport = errors.New("keycloak: user is not in the local realm import (run make keycloak-import)")

// DevAdopter is a LOCAL-ONLY decorator of the Keycloak Admin port for the dev seed (LOCAL_DEVELOPMENT §5): the
// realm import already holds every demo user with its password (password only, ADR-0094), so the use cases that
// create a Keycloak user (createTenantUser, the SIS student provisioning) adopt the user with the same username
// instead of creating one, never rewrite its password, never disable it and never send an action email. Every
// other call goes to the real adapter. The seed binary refuses to run unless EW_ENV=local; production never wires it.
type DevAdopter struct {
	port.KeycloakAdmin
}

// CreateUser returns the id of the realm-import user with the same username (ErrNotInRealmImport when absent).
func (a DevAdopter) CreateUser(ctx context.Context, u port.KCNewUser) (string, error) {
	found, err := a.FindUserByUsername(ctx, u.Username)
	if errors.Is(err, port.ErrKCNotFound) {
		return "", fmt.Errorf("%w: %s", ErrNotInRealmImport, u.Username)
	}
	if err != nil {
		return "", err
	}
	return found.ID, nil
}

// SetPassword keeps the demo password of the realm import.
func (DevAdopter) SetPassword(context.Context, string, string, bool) error { return nil }

// DisableUser never disables a realm-import user (compensation of a failed staff creation).
func (DevAdopter) DisableUser(context.Context, string) error { return nil }

// ExecuteActionsEmail sends nothing: demo users already have their password.
func (DevAdopter) ExecuteActionsEmail(context.Context, string, ...string) error { return nil }
