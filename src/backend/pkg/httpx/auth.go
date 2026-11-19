package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AuthConfig comes from EW_OIDC_* and EW_KC_*_CLIENT_ID (KEYCLOAK_SSO §11); nothing is hard-coded (R3).
type AuthConfig struct {
	Issuers             []string
	Audience            string
	AllowedAZP          []string
	ClockSkew           time.Duration
	PortalClientID      string
	GameClientID        string
	PlatformRequiredACR []string
	// PlatformRequiredAMR: amr values that prove MFA. Keycloak direct grant (ADR-0091) sets amr ["pwd","otp"]
	// but keeps acr at 1 (LoA is a browser-flow concept) — F004 spike R5.
	PlatformRequiredAMR []string
	PlatformMaxAuthAge  time.Duration
}

// AuthConfigFrom reads the configuration; missing variables are reported by name only.
func AuthConfigFrom(c *config.Config) (AuthConfig, error) {
	if err := c.Require("EW_OIDC_ISSUERS", "EW_OIDC_AUDIENCE", "EW_OIDC_ALLOWED_AZP", "EW_KC_PORTAL_CLIENT_ID",
		"EW_KC_GAME_CLIENT_ID"); err != nil {
		return AuthConfig{}, err
	}
	skew := c.Duration("EW_OIDC_ALLOWED_CLOCK_SKEW", 30*time.Second)
	age := c.Duration("EW_OIDC_PLATFORM_MAX_AUTH_AGE", 8*time.Hour)
	amr := c.List("EW_OIDC_PLATFORM_REQUIRED_AMR")
	if len(amr) == 0 {
		amr = []string{"otp"}
	}
	return AuthConfig{
		Issuers: c.List("EW_OIDC_ISSUERS"), Audience: c.String("EW_OIDC_AUDIENCE"), AllowedAZP: c.List("EW_OIDC_ALLOWED_AZP"),
		ClockSkew: skew, PortalClientID: c.String("EW_KC_PORTAL_CLIENT_ID"), GameClientID: c.String("EW_KC_GAME_CLIENT_ID"),
		PlatformRequiredACR: c.List("EW_OIDC_PLATFORM_REQUIRED_ACR"), PlatformRequiredAMR: amr, PlatformMaxAuthAge: age,
	}, nil
}

// KeySource returns the verification key of (issuer, kid); implemented by *JWKS.
type KeySource interface {
	Key(ctx context.Context, iss, kid string) (any, error)
}

// Asymmetric algorithms only: "none" and HMAC (public-key confusion) are rejected by the parser.
var allowedAlgs = []jose.SignatureAlgorithm{jose.RS256, jose.RS384, jose.RS512, jose.PS256, jose.PS384, jose.PS512,
	jose.ES256, jose.ES384, jose.ES512, jose.EdDSA}

type tokenClaims struct {
	jwt.Claims
	AZP               string   `json:"azp"`
	Typ               string   `json:"typ"`
	SID               string   `json:"sid"`
	ACR               string   `json:"acr"`
	AMR               []string `json:"amr"`
	AuthTime          int64    `json:"auth_time"`
	ClientID          string   `json:"client_id"`
	PreferredUsername string   `json:"preferred_username"`
	// Organization is the Keycloak Organizations claim: a list of aliases or an object keyed by alias.
	Organization json.RawMessage `json:"organization"`
}

// Verifier checks access tokens itself, independent of APISIX (F002-AC-01, ADR-0009).
type Verifier struct {
	cfg   AuthConfig
	keys  KeySource
	clock clock.Clock
}

// NewVerifier builds a verifier.
func NewVerifier(cfg AuthConfig, keys KeySource, clk clock.Clock) (*Verifier, error) {
	if len(cfg.Issuers) == 0 || cfg.Audience == "" || len(cfg.AllowedAZP) == 0 || keys == nil {
		return nil, errors.New("httpx: verifier needs issuers, audience, allowed azp and a key source")
	}
	if clk == nil {
		clk = clock.System()
	}
	return &Verifier{cfg: cfg, keys: keys, clock: clk}, nil
}

func unauthenticated(detail string) *problem.Error {
	return problem.New(problem.Unauthenticated, detail)
}

// Verify returns the claims of a valid user access token, or a problem error: 401 UNAUTHENTICATED,
// 401 TOKEN_EXPIRED, or 503 UNAVAILABLE when the signing keys were never loaded.
func (v *Verifier) Verify(ctx context.Context, raw string) (authz.Claims, error) {
	tok, err := jwt.ParseSigned(raw, allowedAlgs)
	if err != nil || len(tok.Headers) != 1 {
		return authz.Claims{}, unauthenticated("malformed token")
	}
	var unverified tokenClaims
	if err := tok.UnsafeClaimsWithoutVerification(&unverified); err != nil {
		return authz.Claims{}, unauthenticated("malformed token")
	}
	if !slices.Contains(v.cfg.Issuers, unverified.Issuer) {
		return authz.Claims{}, unauthenticated("issuer not allowed")
	}
	key, err := v.keys.Key(ctx, unverified.Issuer, tok.Headers[0].KeyID)
	if errors.Is(err, ErrKeysUnavailable) {
		return authz.Claims{}, problem.New(problem.Unavailable, "signing keys unavailable").Retry()
	}
	if err != nil {
		return authz.Claims{}, unauthenticated("unknown signing key")
	}
	var c tokenClaims
	if err := tok.Claims(key, &c); err != nil {
		return authz.Claims{}, unauthenticated("invalid signature")
	}
	err = c.ValidateWithLeeway(jwt.Expected{Issuer: unverified.Issuer, AnyAudience: jwt.Audience{v.cfg.Audience}, Time: v.clock.Now()}, v.cfg.ClockSkew)
	switch {
	case errors.Is(err, jwt.ErrExpired):
		return authz.Claims{}, problem.New(problem.TokenExpired, "")
	case err != nil:
		return authz.Claims{}, unauthenticated(fmt.Sprintf("invalid claims: %v", trimJWTErr(err)))
	case c.Expiry == nil:
		return authz.Claims{}, unauthenticated("token without exp")
	case !strings.EqualFold(c.Typ, "Bearer"):
		return authz.Claims{}, unauthenticated("not an access token")
	case !slices.Contains(v.cfg.AllowedAZP, c.AZP):
		return authz.Claims{}, unauthenticated("client not allowed")
	case c.Subject == "" || c.ClientID != "" || strings.HasPrefix(c.PreferredUsername, "service-account-"):
		// Client-credentials tokens have no user and never reach application routes (F002-AC-05).
		return authz.Claims{}, unauthenticated("token has no user")
	}
	out := authz.Claims{Issuer: c.Issuer, Subject: c.Subject, AZP: c.AZP, SessionID: c.SID, ACR: c.ACR, AMR: c.AMR,
		Expiry: c.Expiry.Time(), Organizations: organizations(c.Organization)}
	switch {
	case c.AuthTime > 0:
		out.AuthTime = time.Unix(c.AuthTime, 0).UTC()
	case c.IssuedAt != nil:
		// Keycloak direct grant (ADR-0091) emits no auth_time (F004 spike R5): fall back to iat; module identity
		// refuses portal refreshes of platform staff older than EW_OIDC_PLATFORM_MAX_AUTH_AGE (F012).
		out.AuthTime = c.IssuedAt.Time().UTC()
	}
	return out, nil
}

// organizations reads the aliases of the organization claim (list of aliases, or object keyed by alias).
func organizations(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		out := make([]string, 0, len(obj))
		for k := range obj {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	return nil
}

func trimJWTErr(err error) string { return strings.TrimPrefix(err.Error(), "go-jose/go-jose/jwt: ") }

// channel maps azp to the client kind.
func (v *Verifier) channel(azp string) authz.Channel {
	switch azp {
	case v.cfg.PortalClientID:
		return authz.ChannelPortal
	case v.cfg.GameClientID:
		return authz.ChannelGame
	}
	return authz.ChannelUnknown
}

// mfa reports a platform-grade authentication: acr in PlatformRequiredACR or an amr value in
// PlatformRequiredAMR, and auth_time recent enough.
func (v *Verifier) mfa(c authz.Claims) bool {
	strong := slices.Contains(v.cfg.PlatformRequiredACR, c.ACR)
	for _, m := range c.AMR {
		strong = strong || slices.Contains(v.cfg.PlatformRequiredAMR, m)
	}
	if !strong || c.AuthTime.IsZero() {
		return false
	}
	return v.clock.Now().Sub(c.AuthTime) <= v.cfg.PlatformMaxAuthAge
}
