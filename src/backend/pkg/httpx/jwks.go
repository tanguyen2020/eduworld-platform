// Package httpx is the HTTP server kit of the public API (F002 US1–US4): JWT verification against an
// in-memory JWKS, tenant header, Principal resolution, route guards and Problem Details. It never
// registers /internal/ routes: modules call each other in process (ADR-0074).
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
)

// JWKSOptions configures JWKS (ADR-0045, BACKEND_DESIGN §6.1).
type JWKSOptions struct {
	Issuers               []string
	HTTPClient            *http.Client
	RefreshInterval       time.Duration // background refresh (10 min)
	MinUnknownKidInterval time.Duration // at most one refresh per issuer per interval on unknown kid
	Clock                 clock.Clock
	Registerer            prometheus.Registerer
}

type issuerKeys struct {
	keys        map[string]jose.JSONWebKey
	lastAttempt time.Time
	loaded      bool
}

// JWKS keeps the signing keys of every allowed issuer in memory. It never calls token introspection.
type JWKS struct {
	o       JWKSOptions
	mu      sync.Mutex
	fetchMu sync.Mutex
	byIss   map[string]*issuerKeys
	refresh *prometheus.CounterVec
}

var (
	// ErrUnknownKey: the kid is not published by the issuer (→ 401).
	ErrUnknownKey = errors.New("httpx: unknown signing key")
	// ErrKeysUnavailable: the issuer's JWKS was never loaded (→ 503).
	ErrKeysUnavailable = errors.New("httpx: signing keys unavailable")
)

// NewJWKS validates options; call Load (startup) and Run (background) afterwards.
func NewJWKS(o JWKSOptions) (*JWKS, error) {
	if len(o.Issuers) == 0 || o.RefreshInterval <= 0 || o.MinUnknownKidInterval <= 0 {
		return nil, errors.New("httpx: JWKS needs issuers, RefreshInterval and MinUnknownKidInterval")
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	if o.Clock == nil {
		o.Clock = clock.System()
	}
	j := &JWKS{o: o, byIss: map[string]*issuerKeys{}}
	for _, iss := range o.Issuers {
		j.byIss[iss] = &issuerKeys{}
	}
	j.refresh = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_jwks_refresh_total", Help: "JWKS refreshes by result.",
	}, []string{"result"})
	if o.Registerer != nil {
		if err := o.Registerer.Register(j.refresh); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, err
			}
			j.refresh = are.ExistingCollector.(*prometheus.CounterVec)
		}
	}
	return j, nil
}

// Load fetches every issuer's JWKS; it returns the first error but keeps keys already loaded.
func (j *JWKS) Load(ctx context.Context) error {
	var first error
	for _, iss := range j.o.Issuers {
		if err := j.fetch(ctx, iss); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Run refreshes in the background until ctx ends.
func (j *JWKS) Run(ctx context.Context) {
	t := time.NewTicker(j.o.RefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = j.Load(ctx)
		}
	}
}

// Check is the readiness check `jwks`: every issuer has keys.
func (j *JWKS) Check(context.Context) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	for iss, k := range j.byIss {
		if !k.loaded {
			return fmt.Errorf("jwks not loaded for issuer %d", indexOf(j.o.Issuers, iss))
		}
	}
	return nil
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// Key returns the public key (iss, kid); an unknown kid refreshes the issuer at most once per
// MinUnknownKidInterval.
func (j *JWKS) Key(ctx context.Context, iss, kid string) (any, error) {
	if k, ok, loaded := j.lookup(iss, kid); ok {
		return k, nil
	} else if !j.mayRefresh(iss) {
		if !loaded {
			return nil, ErrKeysUnavailable
		}
		return nil, ErrUnknownKey
	}
	_ = j.fetch(ctx, iss)
	k, ok, loaded := j.lookup(iss, kid)
	switch {
	case ok:
		return k, nil
	case !loaded:
		return nil, ErrKeysUnavailable
	default:
		return nil, ErrUnknownKey
	}
}

func (j *JWKS) lookup(iss, kid string) (any, bool, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	ik, ok := j.byIss[iss]
	if !ok {
		return nil, false, true
	}
	k, ok := ik.keys[kid]
	return k.Key, ok, ik.loaded
}

func (j *JWKS) mayRefresh(iss string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	ik, ok := j.byIss[iss]
	if !ok {
		return false
	}
	return ik.lastAttempt.IsZero() || j.o.Clock.Now().Sub(ik.lastAttempt) >= j.o.MinUnknownKidInterval
}

func (j *JWKS) fetch(ctx context.Context, iss string) error {
	j.fetchMu.Lock()
	defer j.fetchMu.Unlock()
	j.mu.Lock()
	j.byIss[iss].lastAttempt = j.o.Clock.Now()
	j.mu.Unlock()

	keys, err := j.download(ctx, iss)
	if err != nil {
		j.refresh.WithLabelValues("error").Inc()
		return err
	}
	j.mu.Lock()
	j.byIss[iss].keys, j.byIss[iss].loaded = keys, true
	j.mu.Unlock()
	j.refresh.WithLabelValues("ok").Inc()
	return nil
}

func (j *JWKS) download(ctx context.Context, iss string) (map[string]jose.JSONWebKey, error) {
	var disc struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := j.getJSON(ctx, strings.TrimSuffix(iss, "/")+"/.well-known/openid-configuration", &disc); err != nil {
		return nil, fmt.Errorf("httpx: oidc discovery: %w", err)
	}
	if disc.Issuer != iss || disc.JWKSURI == "" {
		return nil, errors.New("httpx: oidc discovery: issuer mismatch or no jwks_uri")
	}
	var set jose.JSONWebKeySet
	if err := j.getJSON(ctx, disc.JWKSURI, &set); err != nil {
		return nil, fmt.Errorf("httpx: jwks: %w", err)
	}
	out := map[string]jose.JSONWebKey{}
	for _, k := range set.Keys {
		if k.KeyID != "" && k.IsPublic() && (k.Use == "" || k.Use == "sig") {
			out[k.KeyID] = k
		}
	}
	if len(out) == 0 {
		return nil, errors.New("httpx: jwks has no signing key")
	}
	return out, nil
}

func (j *JWKS) getJSON(ctx context.Context, url string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := j.o.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(v)
}
