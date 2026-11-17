package httpx_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

const (
	testAudience = "eduworld-api"
	portalClient = "eduworld-portal"
	gameClient   = "eduworld-game-backend"
)

var now = time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)

// idp is a fake OIDC issuer: discovery + JWKS, keys rotated by the test.
type idp struct {
	t        *testing.T
	srv      *httptest.Server
	mu       sync.Mutex
	keys     map[string]*rsa.PrivateKey // kid → key published in JWKS
	jwksHits atomic.Int32
	down     atomic.Bool
}

func newIDP(t *testing.T) *idp {
	t.Helper()
	p := &idp{t: t, keys: map[string]*rsa.PrivateKey{}}
	p.addKey("k1")
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		if p.down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": p.issuer(), "jwks_uri": p.srv.URL + "/certs"})
	})
	mux.HandleFunc("/certs", func(w http.ResponseWriter, r *http.Request) {
		p.jwksHits.Add(1)
		if p.down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		var set jose.JSONWebKeySet
		for kid, k := range p.keys {
			set.Keys = append(set.Keys, jose.JSONWebKey{Key: &k.PublicKey, KeyID: kid, Algorithm: "RS256", Use: "sig"})
		}
		_ = json.NewEncoder(w).Encode(set)
	})
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *idp) issuer() string { return p.srv.URL }

func (p *idp) addKey(kid string) *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		p.t.Fatal(err)
	}
	p.mu.Lock()
	p.keys[kid] = k
	p.mu.Unlock()
	return k
}

func (p *idp) key(kid string) *rsa.PrivateKey {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.keys[kid]
}

type tok struct {
	kid      string
	key      *rsa.PrivateKey // nil = p.key(kid)
	alg      jose.SignatureAlgorithm
	claims   map[string]any
	unsigned bool // alg=none
}

func (p *idp) baseClaims(azp string) map[string]any {
	return map[string]any{
		"iss": p.issuer(), "sub": "kc-" + uuid.NewString(), "aud": []string{testAudience, "account"}, "azp": azp,
		"typ": "Bearer", "exp": now.Add(5 * time.Minute).Unix(), "iat": now.Add(-time.Minute).Unix(),
		"nbf": now.Add(-time.Minute).Unix(), "sid": "sid-1", "acr": "1", "auth_time": now.Add(-time.Minute).Unix(),
	}
}

func (p *idp) sign(tk tok) string {
	p.t.Helper()
	if tk.kid == "" {
		tk.kid = "k1"
	}
	if tk.alg == "" {
		tk.alg = jose.RS256
	}
	if tk.unsigned {
		h, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT", "kid": tk.kid})
		b, _ := json.Marshal(tk.claims)
		return b64(h) + "." + b64(b) + "."
	}
	key := tk.key
	if key == nil {
		key = p.key(tk.kid)
	}
	s, err := jose.NewSigner(jose.SigningKey{Algorithm: tk.alg, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", tk.kid))
	if err != nil {
		p.t.Fatal(err)
	}
	raw, err := jwt.Signed(s).Claims(tk.claims).Serialize()
	if err != nil {
		p.t.Fatal(err)
	}
	return raw
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func authCfg(p *idp) httpx.AuthConfig {
	return httpx.AuthConfig{
		Issuers: []string{p.issuer()}, Audience: testAudience, AllowedAZP: []string{gameClient, portalClient},
		ClockSkew: 30 * time.Second, PortalClientID: portalClient, GameClientID: gameClient,
		PlatformRequiredACR: []string{"mfa"}, PlatformMaxAuthAge: 8 * time.Hour,
	}
}

// fakeResolver returns principals per subject; ErrNotMember when the tenant differs.
type fakeResolver struct {
	mu   sync.Mutex
	byID map[string]authz.PrincipalData
	err  error
	last authz.ResolveRequest
}

func (f *fakeResolver) set(sub string, d authz.PrincipalData) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.byID == nil {
		f.byID = map[string]authz.PrincipalData{}
	}
	f.byID[sub] = d
}

func (f *fakeResolver) Resolve(_ context.Context, req authz.ResolveRequest) (authz.Principal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last = req
	if f.err != nil {
		return authz.Principal{}, f.err
	}
	d, ok := f.byID[req.Subject.Subject]
	if !ok {
		return authz.Principal{}, authz.ErrNotMember
	}
	if req.TenantID != nil && !d.IsSystemAdmin && (d.TenantID == nil || *d.TenantID != *req.TenantID) {
		return authz.Principal{}, authz.ErrNotMember
	}
	if req.TenantID != nil && d.IsSystemAdmin {
		t := *req.TenantID
		d.TenantID = &t
	}
	return authz.NewPrincipal(d), nil
}

type env struct {
	idp      *idp
	jwks     *httpx.JWKS
	res      *fakeResolver
	srv      *httpx.Server
	clk      *clock.Fake
	audits   *[]audit.Event
	auditsMu *sync.Mutex
}

// newEnv builds a server with the given operations; each handler echoes the principal it saw.
func newEnv(t *testing.T, ops ...httpx.Operation) *env {
	t.Helper()
	p := newIDP(t)
	clk := clock.NewFake(now)
	jw, err := httpx.NewJWKS(httpx.JWKSOptions{Issuers: []string{p.issuer()}, HTTPClient: p.srv.Client(), Clock: clk,
		RefreshInterval: 10 * time.Minute, MinUnknownKidInterval: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := jw.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	res := &fakeResolver{}
	var events []audit.Event
	var mu sync.Mutex
	authz.Configure(authz.Hooks{Recorder: audit.RecorderFunc(func(_ context.Context, e audit.Event) error {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, e)
		return nil
	})})
	t.Cleanup(func() { authz.Configure(authz.Hooks{}) })
	srv, err := httpx.NewServer(httpx.Deps{Auth: authCfg(p), Keys: jw, Resolver: res, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if err := srv.Handle(op, http.HandlerFunc(echo)); err != nil {
			t.Fatal(err)
		}
	}
	return &env{idp: p, jwks: jw, res: res, srv: srv, clk: clk, audits: &events, auditsMu: &mu}
}

func (e *env) auditCount(action string) int {
	e.auditsMu.Lock()
	defer e.auditsMu.Unlock()
	n := 0
	for _, a := range *e.audits {
		if a.Action == action {
			n++
		}
	}
	return n
}

type echoBody struct {
	UserID   string   `json:"user_id"`
	TenantID string   `json:"tenant_id"`
	Scope    []string `json:"scope"`
	Platform bool     `json:"platform"`
	MFA      bool     `json:"mfa"`
	Channel  string   `json:"channel"`
}

func echo(w http.ResponseWriter, r *http.Request) {
	var b echoBody
	if p, ok := authz.PrincipalFrom(r.Context()); ok {
		b.UserID = p.UserID.String()
		if p.TenantID != nil {
			b.TenantID = p.TenantID.String()
		}
		b.MFA, b.Channel = p.MFA, string(p.Channel)
	}
	if s, ok := tenancy.ScopeFrom(r.Context()); ok {
		for _, id := range s.TenantIDs() {
			b.Scope = append(b.Scope, id.String())
		}
		b.Platform = s.PlatformOnly()
	}
	_ = json.NewEncoder(w).Encode(b)
}

type resp struct {
	status int
	code   string
	ctype  string
	body   echoBody
	raw    map[string]any
}

func (e *env) do(t *testing.T, method, path, token string, hdr map[string]string) resp {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	out := resp{status: rec.Code, ctype: rec.Header().Get("Content-Type")}
	if rec.Code >= 400 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out.raw)
		out.code, _ = out.raw["code"].(string)
	} else {
		_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	}
	return out
}

func uptr(u uuid.UUID) *uuid.UUID { return &u }
