package httpx_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

var meOp = httpx.Operation{ID: "getMe", Method: http.MethodGet, Pattern: "/me", Tenant: httpx.TenantNone}

func staffFor(e *env, claims map[string]any, tenant *uuid.UUID) {
	e.res.set(claims["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: tenant})
}

// F002-AC-01: a valid token passes every check without APISIX.
func TestAuthAcceptsValidToken(t *testing.T) {
	e := newEnv(t, meOp)
	c := e.idp.baseClaims(portalClient)
	staffFor(e, c, nil)
	r := e.do(t, http.MethodGet, "/api/v1/me", e.idp.sign(tok{claims: c}), nil)
	if r.status != http.StatusOK || r.body.UserID == "" || r.body.Channel != "portal" {
		t.Fatalf("status %d body %+v raw %v", r.status, r.body, r.raw)
	}
	if got := e.res.last.Subject; got.Issuer != e.idp.issuer() || got.Subject != c["sub"] {
		t.Fatalf("resolver got %+v", got)
	}
}

// F002-AC-02, F002-AC-03 (TC-AUTH-009).
func TestAuthRejects(t *testing.T) {
	e := newEnv(t, meOp)
	foreign := e.idp.addKey("unused")
	e.idp.mu.Lock()
	delete(e.idp.keys, "unused") // signed with a key that is not published
	e.idp.mu.Unlock()
	mut := func(f func(m map[string]any)) map[string]any {
		c := e.idp.baseClaims(portalClient)
		f(c)
		return c
	}
	cases := []struct {
		name  string
		token string
		code  string
	}{
		{"missing", "", "UNAUTHENTICATED"},
		{"garbage", "abc.def.ghi", "UNAUTHENTICATED"},
		{"alg none", e.idp.sign(tok{unsigned: true, claims: e.idp.baseClaims(portalClient)}), "UNAUTHENTICATED"},
		{"forged signature", e.idp.sign(tok{key: foreign, claims: e.idp.baseClaims(portalClient)}), "UNAUTHENTICATED"},
		{"HS256 with public key confusion", hsToken(t, e), "UNAUTHENTICATED"},
		{"wrong audience", e.idp.sign(tok{claims: mut(func(m map[string]any) { m["aud"] = "other-api" })}), "UNAUTHENTICATED"},
		{"wrong issuer", e.idp.sign(tok{claims: mut(func(m map[string]any) { m["iss"] = "https://evil/realms/x" })}), "UNAUTHENTICATED"},
		{"azp not allowed", e.idp.sign(tok{claims: mut(func(m map[string]any) { m["azp"] = "account-console" })}), "UNAUTHENTICATED"},
		{"typ refresh", e.idp.sign(tok{claims: mut(func(m map[string]any) { m["typ"] = "Refresh" })}), "UNAUTHENTICATED"},
		{"not yet valid", e.idp.sign(tok{claims: mut(func(m map[string]any) { m["nbf"] = now.Add(5 * time.Minute).Unix() })}), "UNAUTHENTICATED"},
		{"expired", e.idp.sign(tok{claims: mut(func(m map[string]any) { m["exp"] = now.Add(-time.Minute).Unix() })}), "TOKEN_EXPIRED"},
		{"no subject", e.idp.sign(tok{claims: mut(func(m map[string]any) { delete(m, "sub") })}), "UNAUTHENTICATED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := e.do(t, http.MethodGet, "/api/v1/me", c.token, nil)
			if r.status != http.StatusUnauthorized || r.code != c.code || r.ctype != "application/problem+json" {
				t.Fatalf("status %d code %q ctype %q", r.status, r.code, r.ctype)
			}
		})
	}
	// Within the clock skew an expired token is still accepted.
	c := mut(func(m map[string]any) { m["exp"] = now.Add(-10 * time.Second).Unix() })
	staffFor(e, c, nil)
	if r := e.do(t, http.MethodGet, "/api/v1/me", e.idp.sign(tok{claims: c}), nil); r.status != http.StatusOK {
		t.Fatalf("token within skew rejected: %d %v", r.status, r.raw)
	}
}

func hsToken(t *testing.T, e *env) string {
	t.Helper()
	k := e.idp.key("k1")
	secret := k.N.Bytes()
	s, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: secret}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	obj, err := s.Sign([]byte(`{"iss":"` + e.idp.issuer() + `","sub":"x","aud":"eduworld-api","azp":"eduworld-portal","typ":"Bearer","exp":` +
		strconv.FormatInt(now.Add(time.Hour).Unix(), 10) + `}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := obj.CompactSerialize()
	return raw
}

// F002-AC-05: client-credentials tokens never reach application routes; /internal/v1/* is never served.
func TestNoServiceTokensNoInternalRoutes(t *testing.T) {
	e := newEnv(t, meOp)
	svc := e.idp.baseClaims("svc-identity")
	if r := e.do(t, http.MethodGet, "/api/v1/me", e.idp.sign(tok{claims: svc}), nil); r.status != http.StatusUnauthorized {
		t.Fatalf("svc azp: %d", r.status)
	}
	// Even with an allowed azp, a service-account token (client_id claim) has no user.
	sa := e.idp.baseClaims(portalClient)
	sa["client_id"] = portalClient
	sa["preferred_username"] = "service-account-eduworld-portal"
	if r := e.do(t, http.MethodGet, "/api/v1/me", e.idp.sign(tok{claims: sa}), nil); r.status != http.StatusUnauthorized {
		t.Fatalf("service account: %d", r.status)
	}
	for _, p := range []string{"/internal/v1/principals:resolve", "/api/v1/internal/v1/x", "/internal/"} {
		if r := e.do(t, http.MethodPost, p, "", nil); r.status != http.StatusNotFound {
			t.Fatalf("%s: %d", p, r.status)
		}
	}
	if err := e.srv.Handle(httpx.Operation{ID: "x", Method: http.MethodPost, Pattern: "/internal/v1/principals:resolve"},
		http.HandlerFunc(echo)); err == nil {
		t.Fatal("server registered an /internal/ route")
	}
}
