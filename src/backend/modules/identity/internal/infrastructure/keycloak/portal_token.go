package keycloak

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/portalauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// PortalConfig configures the token client of the confidential client eduworld-portal (ADR-0091 §2).
// The realm endpoints are derived from EW_KC_ADMIN_BASE_URL; the secret comes from
// EW_KC_PORTAL_CLIENT_SECRET(_FILE) (Docker secret kc_portal). Nothing is hardcoded (AGENTS R3).
type PortalConfig struct {
	TokenURL     string
	LogoutURL    string
	ClientID     string // EW_KC_PORTAL_CLIENT_ID
	ClientSecret string // EW_KC_PORTAL_CLIENT_SECRET(_FILE)
	Timeout      time.Duration
	MaxRetries   int // logout only (idempotent)
	FailureRatio float64
	// Kind prefixes span names and metric labels: portal (default) or game (eduworld-game-backend, F013).
	Kind string
}

// PortalConfigFrom reads the configuration; missing variables are reported by name only.
func PortalConfigFrom(c *config.Config) (PortalConfig, error) {
	if err := c.Require("EW_KC_ADMIN_BASE_URL", "EW_KC_PORTAL_CLIENT_ID", "EW_KC_PORTAL_CLIENT_SECRET"); err != nil {
		return PortalConfig{}, err
	}
	realm, err := realmURL(strings.TrimRight(c.String("EW_KC_ADMIN_BASE_URL"), "/"))
	if err != nil {
		return PortalConfig{}, err
	}
	ratio := 0.5
	if v := c.String("EW_EXT_CLIENT_CB_FAILURE_RATIO"); v != "" {
		if _, err := fmt.Sscanf(v, "%g", &ratio); err != nil || ratio <= 0 || ratio > 1 {
			return PortalConfig{}, fmt.Errorf("keycloak: EW_EXT_CLIENT_CB_FAILURE_RATIO must be in (0,1]")
		}
	}
	return PortalConfig{
		TokenURL: realm + "/protocol/openid-connect/token", LogoutURL: realm + "/protocol/openid-connect/logout",
		ClientID: c.String("EW_KC_PORTAL_CLIENT_ID"), ClientSecret: c.String("EW_KC_PORTAL_CLIENT_SECRET"),
		Timeout: c.Duration("EW_EXT_CLIENT_TIMEOUT", 2*time.Second), MaxRetries: c.Int("EW_EXT_CLIENT_RETRY_MAX", 2),
		FailureRatio: ratio,
	}, nil
}

// realmURL turns <keycloak>/admin/realms/<realm> into <keycloak>/realms/<realm>.
func realmURL(adminBase string) (string, error) {
	i := strings.Index(adminBase, "/admin/realms/")
	if i < 0 || i+len("/admin/realms/") >= len(adminBase) {
		return "", fmt.Errorf("keycloak: EW_KC_ADMIN_BASE_URL must end with /admin/realms/<realm>")
	}
	return adminBase[:i] + "/realms/" + adminBase[i+len("/admin/realms/"):], nil
}

// tokenCallDuration is eduworld_keycloak_token_call_duration_seconds{op} (plan F013 "Quan sát"): every call
// to the token and logout endpoints, including timeouts and open-circuit refusals.
var tokenCallDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "eduworld_keycloak_token_call_duration_seconds",
	Help: "Duration of calls from module identity to the Keycloak token and logout endpoints.", Buckets: []float64{.025, .05, .1, .25, .5, 1, 2, 3, 5}},
	[]string{"op"})

// TokenCallDuration exposes the histogram for registration by the binary.
func TokenCallDuration() *prometheus.HistogramVec { return tokenCallDuration }

// PortalTokens is the token client of eduworld-portal. The password grant and the refresh are never
// retried (a retry would count twice for brute-force detection, and a rotated refresh token is single use);
// logout is. Only transport errors, 429 and 5xx count for the circuit breaker, so wrong passwords cannot
// open it. Bodies (credentials, tokens) are never logged or traced.
type PortalTokens struct {
	cfg   PortalConfig
	http  *http.Client
	once  failsafe.Executor[tokenResult]
	retry failsafe.Executor[tokenResult]
}

var _ port.PortalIdP = (*PortalTokens)(nil)

// errMisconfigured: Keycloak rejected the client itself (invalid_client, unauthorized_client…): 503, logged.
var errMisconfigured = errors.New("keycloak: portal client rejected")

// NewPortalTokens builds the client.
func NewPortalTokens(cfg PortalConfig, client *http.Client) (*PortalTokens, error) {
	if cfg.TokenURL == "" || cfg.LogoutURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("keycloak: portal token client needs endpoints and client credentials")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.Kind == "" {
		cfg.Kind = "portal"
	}
	if cfg.FailureRatio <= 0 || cfg.FailureRatio > 1 {
		cfg.FailureRatio = 0.5
	}
	if client == nil {
		client = &http.Client{}
	}
	transient := func(_ tokenResult, err error) bool {
		return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, port.ErrPortalInvalidCredentials) &&
			!errors.Is(err, port.ErrPortalAccessDenied) && !errors.Is(err, port.ErrPortalSetupRequired) &&
			!errors.Is(err, port.ErrPortalRefreshInvalid) && !errors.Is(err, errMisconfigured)
	}
	cb := circuitbreaker.NewBuilder[tokenResult]().HandleIf(transient).
		WithFailureRateThreshold(cfg.FailureRatio, 10, time.Minute).WithDelay(30 * time.Second).WithSuccessThreshold(2).Build()
	rp := retrypolicy.NewBuilder[tokenResult]().HandleIf(transient).
		WithMaxRetries(cfg.MaxRetries).WithBackoff(100*time.Millisecond, time.Second).ReturnLastFailure().Build()
	return &PortalTokens{cfg: cfg, http: client, once: failsafe.With[tokenResult](cb), retry: failsafe.With[tokenResult](rp, cb)}, nil
}

type tokenResult struct {
	status      int
	contentType string
	body        []byte
}

const maxTokenBody = 64 << 10

// PasswordGrant implements port.PortalIdP: username and password only (ADR-0094). The form buffer holding the
// password is zeroed after the call (best effort, F012-AC-31); the caller zeroes its own copies.
func (p *PortalTokens) PasswordGrant(ctx context.Context, username string, password []byte) (port.PortalTokens, error) {
	form := p.clientForm("password")
	form = appendField(form, "username", []byte(username))
	form = appendField(form, "password", password)
	defer clear(form)
	res, err := p.post(ctx, p.cfg.Kind+"_password_grant", p.cfg.TokenURL, form, p.once)
	if err != nil {
		return port.PortalTokens{}, err
	}
	return parseTokens(res, false)
}

// Refresh implements port.PortalIdP.
func (p *PortalTokens) Refresh(ctx context.Context, refreshToken string) (port.PortalTokens, error) {
	if refreshToken == "" {
		return port.PortalTokens{}, port.ErrPortalRefreshInvalid
	}
	form := appendField(p.clientForm("refresh_token"), "refresh_token", []byte(refreshToken))
	defer clear(form)
	res, err := p.post(ctx, p.cfg.Kind+"_refresh", p.cfg.TokenURL, form, p.once)
	if err != nil {
		return port.PortalTokens{}, err
	}
	return parseTokens(res, true)
}

// Logout implements port.PortalIdP: the OIDC logout endpoint with the refresh token ends the user session
// (sid) of the client. An invalid or already ended session counts as logged out.
func (p *PortalTokens) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	form := p.clientForm("")
	form = appendField(form, "refresh_token", []byte(refreshToken))
	defer clear(form)
	res, err := p.post(ctx, p.cfg.Kind+"_logout", p.cfg.LogoutURL, form, p.retry)
	if err != nil {
		return err
	}
	switch {
	case res.status < 300:
		return nil
	case oauthError(res.body) == "invalid_grant":
		return nil // session already ended
	default:
		return fmt.Errorf("keycloak portal_logout: status %d: %w", res.status, errMisconfigured)
	}
}

// SessionKey implements port.PortalIdP (portalauth.SessionKey of the sid of the refresh token).
func (p *PortalTokens) SessionKey(refreshToken string) string {
	var c struct {
		SID string `json:"sid"`
	}
	if decodeClaims(refreshToken, &c) == nil && c.SID != "" {
		return portalauth.SessionKey(c.SID)
	}
	return portalauth.Key("rt", refreshToken)
}

func (p *PortalTokens) clientForm(grant string) []byte {
	var b []byte
	if grant != "" {
		b = appendField(b, "grant_type", []byte(grant))
	}
	b = appendField(b, "client_id", []byte(p.cfg.ClientID))
	return appendField(b, "client_secret", []byte(p.cfg.ClientSecret))
}

// post sends one form request with its own timeout through exec (circuit breaker, optional retry).
func (p *PortalTokens) post(ctx context.Context, op, url string, form []byte, exec failsafe.Executor[tokenResult]) (tokenResult, error) {
	ctx, span := otel.Tracer("eduworld/identity/keycloak").Start(ctx, "keycloak."+op)
	defer span.End()
	start := time.Now()
	defer func() { tokenCallDuration.WithLabelValues(op).Observe(time.Since(start).Seconds()) }()
	res, err := exec.WithContext(ctx).Get(func() (tokenResult, error) {
		rctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(rctx, http.MethodPost, url, bytes.NewReader(form))
		if err != nil {
			return tokenResult{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		resp, err := p.http.Do(req)
		if err != nil {
			return tokenResult{}, fmt.Errorf("keycloak %s: %w", op, err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenBody))
		if err != nil {
			return tokenResult{}, fmt.Errorf("keycloak %s: read: %w", op, err)
		}
		r := tokenResult{status: resp.StatusCode, contentType: resp.Header.Get("Content-Type"), body: body}
		if r.status == http.StatusTooManyRequests || r.status >= 500 {
			return tokenResult{}, fmt.Errorf("keycloak %s: status %d: %w", op, r.status, errTransient)
		}
		return r, nil
	})
	if err != nil {
		span.SetStatus(codes.Error, op)
		span.SetAttributes(attribute.String("error.kind", errKind(err)))
		return tokenResult{}, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", res.status))
	return res, nil
}

// parseTokens maps a token endpoint response (F004 research R5): 200 → tokens; 401 text/html → the flow
// denied the account (deny-access-authenticator); invalid_grant "Account is not fully set up" → setup
// required; other invalid_grant → invalid credentials (password grant) or invalid refresh token; anything
// else (invalid_client, unauthorized_client, invalid_request) is a misconfiguration.
func parseTokens(res tokenResult, refresh bool) (port.PortalTokens, error) {
	grantErr := port.ErrPortalInvalidCredentials
	if refresh {
		grantErr = port.ErrPortalRefreshInvalid
	}
	if res.status != http.StatusOK {
		mt, _, _ := mime.ParseMediaType(res.contentType)
		if mt == "text/html" && res.status >= 400 && res.status < 500 {
			if refresh {
				return port.PortalTokens{}, grantErr
			}
			return port.PortalTokens{}, port.ErrPortalAccessDenied
		}
		var e struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(res.body, &e)
		if e.Error == "invalid_grant" {
			if !refresh && strings.Contains(strings.ToLower(e.Description), "not fully set up") {
				return port.PortalTokens{}, port.ErrPortalSetupRequired
			}
			return port.PortalTokens{}, grantErr
		}
		return port.PortalTokens{}, fmt.Errorf("keycloak token: status %d error %q: %w", res.status, trimCode(e.Error), errMisconfigured)
	}
	var t struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int    `json:"expires_in"`
		RefreshExpiresIn int    `json:"refresh_expires_in"`
		SessionState     string `json:"session_state"`
	}
	if err := json.Unmarshal(res.body, &t); err != nil || t.AccessToken == "" || t.RefreshToken == "" {
		return port.PortalTokens{}, fmt.Errorf("keycloak token: invalid response: %w", errMisconfigured)
	}
	var c struct {
		Iss   string   `json:"iss"`
		Sub   string   `json:"sub"`
		SID   string   `json:"sid"`
		Roles []string `json:"roles"` // realm roles (scope eduworld-roles)
		Iat   int64    `json:"iat"`
	}
	if err := decodeClaims(t.AccessToken, &c); err != nil || c.Sub == "" || c.Iss == "" {
		return port.PortalTokens{}, fmt.Errorf("keycloak token: unreadable access token: %w", errMisconfigured)
	}
	if c.SID == "" {
		c.SID = t.SessionState
	}
	out := port.PortalTokens{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresIn: t.ExpiresIn,
		RefreshExpiresIn: t.RefreshExpiresIn, Issuer: c.Iss, Subject: c.Sub, SessionID: c.SID, RealmRoles: c.Roles}
	if c.Iat > 0 {
		out.IssuedAt = time.Unix(c.Iat, 0).UTC()
	}
	return out, nil
}

// trimCode keeps an OAuth error code printable and short (it goes into an internal error, never a body).
func trimCode(s string) string {
	if len(s) > 40 {
		s = s[:40]
	}
	return strings.Map(func(r rune) rune {
		if r == '_' || (r >= 'a' && r <= 'z') {
			return r
		}
		return -1
	}, s)
}

func oauthError(body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	return e.Error
}

// decodeClaims reads the payload of a JWT received directly from the token endpoint over TLS (OIDC Core
// §3.1.3.7: the TLS server identity stands for the signature). Callers never trust it from clients: the
// access token is verified by pkg/httpx on every API call.
func decodeClaims(token string, v any) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// appendField appends key=value (application/x-www-form-urlencoded) to a byte buffer without building
// intermediate strings of the value.
func appendField(b []byte, key string, val []byte) []byte {
	if len(b) > 0 {
		b = append(b, '&')
	}
	b = appendEscaped(b, []byte(key))
	b = append(b, '=')
	return appendEscaped(b, val)
}

const hexDigits = "0123456789ABCDEF"

func appendEscaped(b, s []byte) []byte {
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b = append(b, c)
		case c == ' ':
			b = append(b, '+')
		default:
			b = append(b, '%', hexDigits[c>>4], hexDigits[c&15])
		}
	}
	return b
}
