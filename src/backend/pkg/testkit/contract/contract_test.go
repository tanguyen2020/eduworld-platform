package contract_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

// F003-AC-32 (TC-SVC-015): the helpers catch a response that drifts from the public spec, a facade
// response object that drifts from the internal spec and an event payload that drifts from its schema.
func TestPublicResponse(t *testing.T) {
	p, err := contract.LoadPublic()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/tenants", nil)
	h := http.Header{"Content-Type": {"application/problem+json"}}
	ok := []byte(`{"type":"about:blank","title":"Unauthenticated","status":401,"code":"UNAUTHENTICATED"}`)
	if err := p.CheckResponse("listMyTenants", req, 401, h, ok); err != nil {
		t.Fatalf("valid problem rejected: %v", err)
	}
	if err := p.CheckResponse("listMyTenants", req, 401, h, []byte(`{"title":"x","status":"401"}`)); err == nil {
		t.Fatal("problem without code (status as string) accepted")
	}
	if err := p.CheckResponse("noSuchOperation", req, 200, h, nil); err == nil {
		t.Fatal("unknown operation accepted")
	}
}

func TestFacadeResponse(t *testing.T) {
	f, err := contract.LoadFacade("identity")
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]any{"user_id": uuid.NewString(), "tenant_id": nil, "tenant_type": nil, "user_type": "staff", "status": "active",
		"is_system_admin": true, "roles": []string{"system_admin"}, "permissions": []string{"platform.tenant.manage"}, "scopes": []any{},
		"grants":  []any{map[string]any{"role_code": "system_admin", "role_scope_kind": "platform", "permissions": []string{"platform.tenant.manage"}}},
		"version": 1}
	if err := f.CheckResponse("resolvePrincipal", 200, valid); err != nil {
		t.Fatalf("valid principal rejected: %v", err)
	}
	delete(valid, "user_id")
	valid["user_type"] = "teacher"
	if err := f.CheckResponse("resolvePrincipal", 200, valid); err == nil {
		t.Fatal("principal without user_id and with a removed user_type accepted")
	}
	if _, _, err := f.Examples("resolvePrincipal"); err != nil {
		t.Fatal(err)
	}
}

func TestEventPayload(t *testing.T) {
	ok := map[string]any{"actor_id": nil, "actor_type": "system", "action": "tenant.created", "target_type": "tenant",
		"target_id": uuid.NewString(), "result": "success"}
	if err := contract.CheckPayload("common", "audit.recorded", 1, ok); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	ok["metadata"] = map[string]any{"Email": "a@b.vn"}
	if err := contract.CheckPayload("common", "audit.recorded", 1, ok); err == nil {
		t.Fatal("payload with an invalid metadata key accepted")
	}
	if err := contract.CheckEnvelope(map[string]any{"event_id": "x"}); err == nil {
		t.Fatal("incomplete envelope accepted")
	}
}
