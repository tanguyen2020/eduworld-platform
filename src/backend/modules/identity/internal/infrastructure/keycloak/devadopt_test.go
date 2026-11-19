package keycloak

import (
	"context"
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// realmUsers is a TEST DOUBLE (R17) of the user lookup of the Keycloak Admin port; any other call panics.
type realmUsers struct {
	port.KeycloakAdmin
	ids map[string]string
}

func (f realmUsers) FindUserByUsername(_ context.Context, username string) (port.KCUser, error) {
	id, ok := f.ids[username]
	if !ok {
		return port.KCUser{}, port.ErrKCNotFound
	}
	return port.KCUser{ID: id, Username: username, Enabled: true}, nil
}

func TestDevAdopterAdoptsRealmImportUsers(t *testing.T) {
	ctx := context.Background()
	a := DevAdopter{KeycloakAdmin: realmUsers{ids: map[string]string{"hs.an.3a": "kc-1"}}}
	id, err := a.CreateUser(ctx, port.KCNewUser{Username: "hs.an.3a", Attributes: map[string][]string{"eduworld_user_id": {"x"}}})
	if err != nil || id != "kc-1" {
		t.Fatalf("CreateUser = %q, %v; want the realm-import user", id, err)
	}
	if _, err := a.CreateUser(ctx, port.KCNewUser{Username: "hs.unknown"}); !errors.Is(err, ErrNotInRealmImport) {
		t.Fatalf("unknown user: %v, want ErrNotInRealmImport (never created)", err)
	}
	// The demo password, the account state and the mailbox are never touched (the embedded port would panic).
	if a.SetPassword(ctx, "kc-1", "x", false) != nil || a.DisableUser(ctx, "kc-1") != nil || a.ExecuteActionsEmail(ctx, "kc-1", "UPDATE_PASSWORD") != nil {
		t.Fatal("DevAdopter must not change realm-import users")
	}
}
