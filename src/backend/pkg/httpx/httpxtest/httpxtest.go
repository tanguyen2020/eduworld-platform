// Package httpxtest helps modules test their HTTP presentation through the real pkg/httpx chain
// (authentication, X-Tenant-Id, principal, guards, OpenAPI validation, idempotency): a fake OIDC issuer
// signing tokens, a principal resolver keyed by subject, and the AuthConfig matching them. Test-only.
package httpxtest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// Client ids and audience of the fake realm.
const (
	Audience     = "eduworld-api"
	PortalClient = "eduworld-portal"
	GameClient   = "eduworld-game-backend"
)

// IDP is a fake OIDC issuer (discovery + JWKS) with one RSA key.
type IDP struct {
	t   testing.TB
	srv *httptest.Server
	key *rsa.PrivateKey
}

// NewIDP starts the issuer; it stops at the end of the test.
func NewIDP(t testing.TB) *IDP {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &IDP{t: t, key: k}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": p.Issuer(), "jwks_uri": p.srv.URL + "/certs"})
	})
	mux.HandleFunc("/certs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &k.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

// Issuer is the iss of the tokens.
func (p *IDP) Issuer() string { return p.srv.URL }

// AuthConfig is the httpx configuration trusting this issuer (MFA = amr otp).
func (p *IDP) AuthConfig() httpx.AuthConfig {
	return httpx.AuthConfig{Issuers: []string{p.Issuer()}, Audience: Audience, AllowedAZP: []string{GameClient, PortalClient},
		ClockSkew: 30 * time.Second, PortalClientID: PortalClient, GameClientID: GameClient,
		PlatformRequiredAMR: []string{"otp"}, PlatformMaxAuthAge: 8 * time.Hour}
}

// Token signs an access token for sub through client azp at now; mfa adds amr otp.
func (p *IDP) Token(sub, azp string, mfa bool, now time.Time) string {
	p.t.Helper()
	return p.sign(sub, azp, "sid-1", mfa, now)
}

// TokenSID is Token with the Keycloak session id sid (game tokens of two devices, F014 takeover).
func (p *IDP) TokenSID(sub, azp, sid string, now time.Time) string {
	p.t.Helper()
	return p.sign(sub, azp, sid, false, now)
}

func (p *IDP) sign(sub, azp, sid string, mfa bool, now time.Time) string {
	p.t.Helper()
	claims := map[string]any{"iss": p.Issuer(), "sub": sub, "aud": []string{Audience}, "azp": azp, "typ": "Bearer",
		"exp": now.Add(5 * time.Minute).Unix(), "iat": now.Add(-time.Minute).Unix(), "nbf": now.Add(-time.Minute).Unix(), "sid": sid}
	if mfa {
		claims["amr"] = []string{"pwd", "otp"}
	}
	s, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: p.key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		p.t.Fatal(err)
	}
	raw, err := jwt.Signed(s).Claims(claims).Serialize()
	if err != nil {
		p.t.Fatal(err)
	}
	return raw
}

// Resolver resolves principals by subject. A tenant user asking another tenant gets ErrNotMember; System
// Admin is resolved into the requested tenant (session tenant = X-Tenant-Id).
type Resolver struct {
	mu   sync.Mutex
	byID map[string]authz.PrincipalData
}

// Set registers the principal data of a subject.
func (r *Resolver) Set(sub string, d authz.PrincipalData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID == nil {
		r.byID = map[string]authz.PrincipalData{}
	}
	r.byID[sub] = d
}

// Resolve implements authz.PrincipalResolver.
func (r *Resolver) Resolve(_ context.Context, req authz.ResolveRequest) (authz.Principal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.byID[req.Subject.Subject]
	if !ok {
		return authz.Principal{}, authz.ErrNotMember
	}
	if req.TenantID != nil {
		if d.IsSystemAdmin {
			t := *req.TenantID
			d.TenantID = &t
		} else if d.TenantID == nil || *d.TenantID != *req.TenantID {
			return authz.Principal{}, authz.ErrNotMember
		}
	}
	return authz.NewPrincipal(d), nil
}

// JWKS returns a loaded key source for this issuer (keys fetched once).
func (p *IDP) JWKS(t testing.TB) *httpx.JWKS {
	t.Helper()
	jw, err := httpx.NewJWKS(httpx.JWKSOptions{Issuers: []string{p.Issuer()}, HTTPClient: p.srv.Client(),
		RefreshInterval: 10 * time.Minute, MinUnknownKidInterval: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := jw.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return jw
}
