package httpx_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// F078: a shared operation (x-service-routing) is served by several modules on one route; each value of the parameter
// reaches the chain of its module; another value is 404 after authentication; a value served twice is refused.
func TestHandleRouted(t *testing.T) {
	e := newEnv(t)
	op := httpx.Operation{ID: "listGameConfigs", Method: http.MethodGet, Pattern: "/game-admin/{config_type}", Tenant: httpx.TenantNone}
	served := map[string]string{}
	mk := func(module string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			served[r.URL.Path] = module
			echo(w, r)
		}
	}
	game, chat := op, op
	game.Module, chat.Module = "game", "chat"
	if err := e.srv.HandleRouted(game, "config_type", []string{"quests", "items"}, mk("game")); err != nil {
		t.Fatal(err)
	}
	if err := e.srv.HandleRouted(chat, "config_type", []string{"gifs"}, mk("chat")); err != nil {
		t.Fatal(err)
	}
	if err := e.srv.HandleRouted(chat, "config_type", []string{"quests"}, mk("chat")); err == nil {
		t.Fatal("a value served twice must be refused")
	}
	if err := e.srv.Handle(op, mk("x")); err == nil {
		t.Fatal("a routed operation cannot be mounted again")
	}
	c := e.idp.baseClaims(portalClient)
	e.res.set(c["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff,
		Grants: []authz.Grant{{Role: "game_designer", ScopeKind: authz.RoleScopePlatform, Permissions: []authz.Permission{authz.PermGameQuestWrite}}}})
	tok := e.idp.sign(tok{claims: c})
	for path, want := range map[string]string{"/api/v1/game-admin/quests": "game", "/api/v1/game-admin/gifs": "chat"} {
		if r := e.do(t, http.MethodGet, path, tok, nil); r.status != http.StatusOK || served[path] != want {
			t.Fatalf("%s: %d served by %q", path, r.status, served[path])
		}
	}
	if r := e.do(t, http.MethodGet, "/api/v1/game-admin/seasons", tok, nil); r.status != http.StatusNotFound {
		t.Fatalf("unserved value: %d", r.status)
	}
	if r := e.do(t, http.MethodGet, "/api/v1/game-admin/seasons", "", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("unserved value without token: %d", r.status)
	}
}
