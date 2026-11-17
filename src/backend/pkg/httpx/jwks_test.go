package httpx_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// F002-AC-04: JWKS kept in memory; an unknown kid triggers at most one refresh per MinUnknownKidInterval;
// no introspection endpoint is ever called (the fake IdP has none).
func TestJWKSRefreshOnUnknownKidRateLimited(t *testing.T) {
	e := newEnv(t, meOp)
	base := e.idp.jwksHits.Load()

	// Known kid: no network.
	c := e.idp.baseClaims(portalClient)
	staffFor(e, c, nil)
	for i := 0; i < 5; i++ {
		if r := e.do(t, http.MethodGet, "/api/v1/me", e.idp.sign(tok{claims: c}), nil); r.status != http.StatusOK {
			t.Fatalf("known kid: %d", r.status)
		}
	}
	if e.idp.jwksHits.Load() != base {
		t.Fatal("JWKS fetched for a known kid")
	}

	// Key rotation: new kid is fetched once, then served from memory.
	e.idp.addKey("k2")
	e.clk.Advance(31 * time.Second)
	for i := 0; i < 3; i++ {
		if r := e.do(t, http.MethodGet, "/api/v1/me", e.idp.sign(tok{kid: "k2", claims: c}), nil); r.status != http.StatusOK {
			t.Fatalf("rotated kid: %d %v", r.status, r.raw)
		}
	}
	if got := e.idp.jwksHits.Load() - base; got != 1 {
		t.Fatalf("JWKS fetches after rotation = %d, want 1", got)
	}

	// Random unknown kids within the interval cause no extra fetch (no amplification).
	for i := 0; i < 10; i++ {
		e.do(t, http.MethodGet, "/api/v1/me", e.idp.sign(tok{kid: "k2", key: e.idp.key("k1"), claims: c}), nil)
		e.do(t, http.MethodGet, "/api/v1/me", e.idp.sign(tok{kid: "zz-" + uuid.NewString(), key: e.idp.key("k1"), claims: c}), nil)
	}
	if got := e.idp.jwksHits.Load() - base; got != 1 {
		t.Fatalf("unknown kids caused %d fetches, want 1 (rate limit)", got)
	}
}

func TestJWKSBackgroundRefreshAndReadiness(t *testing.T) {
	p := newIDP(t)
	clk := clock.NewFake(now)
	jw, err := httpx.NewJWKS(httpx.JWKSOptions{Issuers: []string{p.issuer()}, HTTPClient: p.srv.Client(), Clock: clk,
		RefreshInterval: 10 * time.Minute, MinUnknownKidInterval: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	p.down.Store(true)
	if err := jw.Load(context.Background()); err == nil {
		t.Fatal("Load succeeded with IdP down")
	}
	if err := jw.Check(context.Background()); err == nil {
		t.Fatal("readyz jwks check passes without keys")
	}
	p.down.Store(false)
	if err := jw.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := jw.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A failed periodic refresh keeps the last good keys.
	p.down.Store(true)
	_ = jw.Load(context.Background())
	if err := jw.Check(context.Background()); err != nil {
		t.Fatalf("keys dropped after failed refresh: %v", err)
	}
}

// Edge case: JWKS never loaded → a request with a token gets 503 UNAVAILABLE (not 401, not allowed).
func TestJWKSUnavailableIs503(t *testing.T) {
	p := newIDP(t)
	clk := clock.NewFake(now)
	jw, _ := httpx.NewJWKS(httpx.JWKSOptions{Issuers: []string{p.issuer()}, HTTPClient: p.srv.Client(), Clock: clk,
		RefreshInterval: 10 * time.Minute, MinUnknownKidInterval: 30 * time.Second})
	p.down.Store(true)
	srv, err := httpx.NewServer(httpx.Deps{Auth: authCfg(p), Keys: jw, Resolver: &fakeResolver{}, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	_ = srv.Handle(meOp, http.HandlerFunc(echo))
	e := &env{idp: p, srv: srv}
	r := e.do(t, http.MethodGet, "/api/v1/me", p.sign(tok{claims: p.baseClaims(portalClient)}), nil)
	if r.status != http.StatusServiceUnavailable || r.code != "UNAVAILABLE" || r.raw["retryable"] != true {
		t.Fatalf("status %d code %q raw %v", r.status, r.code, r.raw)
	}
}
