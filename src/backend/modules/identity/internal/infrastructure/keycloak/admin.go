// Package keycloak implements port.KeycloakAdmin over the Keycloak Admin REST API (Keycloak 26) with the
// client-credentials service account svc-identity (KEYCLOAK_SSO §3, F004-AC-04). Each request has a
// timeout; idempotent requests are retried with backoff on network errors, 429 and 5xx; a circuit breaker
// stops calls while Keycloak is failing (ADR-0074 §5, AGENTS R29). Bodies are never logged or traced.
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Config of the adapter (LOCAL_DEVELOPMENT §4, KEYCLOAK_SSO §8). No URL, realm or credential is hardcoded.
type Config struct {
	BaseURL      string // EW_KC_ADMIN_BASE_URL, e.g. http://keycloak:8180/admin/realms/eduworld
	ClientID     string // EW_KC_ADMIN_CLIENT_ID (svc-identity)
	ClientSecret string // EW_KC_ADMIN_CLIENT_SECRET(_FILE)
	Timeout      time.Duration
	MaxRetries   int
	FailureRatio float64
}

// ConfigFrom reads the adapter configuration; it fails when a required variable is missing.
func ConfigFrom(c *config.Config) (Config, error) {
	if err := c.Require("EW_KC_ADMIN_BASE_URL", "EW_KC_ADMIN_CLIENT_ID", "EW_KC_ADMIN_CLIENT_SECRET"); err != nil {
		return Config{}, err
	}
	ratio := 0.5
	if v := c.String("EW_EXT_CLIENT_CB_FAILURE_RATIO"); v != "" {
		if _, err := fmt.Sscanf(v, "%g", &ratio); err != nil || ratio <= 0 || ratio > 1 {
			return Config{}, fmt.Errorf("keycloak: EW_EXT_CLIENT_CB_FAILURE_RATIO must be in (0,1]")
		}
	}
	return Config{
		BaseURL:      strings.TrimRight(c.String("EW_KC_ADMIN_BASE_URL"), "/"),
		ClientID:     c.String("EW_KC_ADMIN_CLIENT_ID"),
		ClientSecret: c.String("EW_KC_ADMIN_CLIENT_SECRET"),
		Timeout:      c.Duration("EW_EXT_CLIENT_TIMEOUT", 2*time.Second),
		MaxRetries:   c.Int("EW_EXT_CLIENT_RETRY_MAX", 2),
		FailureRatio: ratio,
	}, nil
}

// Admin is the Keycloak Admin adapter.
type Admin struct {
	cfg      Config
	tokenURL string
	http     *http.Client
	retry    failsafe.Executor[result]
	once     failsafe.Executor[result]

	mu     sync.Mutex
	token  string
	expiry time.Time
}

var _ port.KeycloakAdmin = (*Admin)(nil)

// orgSync counts Keycloak Admin calls by operation and result (MONITORING: eduworld_identity_org_sync_total).
var orgSync = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_identity_org_sync_total",
	Help: "Keycloak Admin API calls of module identity by operation and result."}, []string{"op", "result"})

// OrgSyncCounter exposes eduworld_identity_org_sync_total for registration by the composition root.
func OrgSyncCounter() *prometheus.CounterVec { return orgSync }

// errTransient marks responses worth retrying (429, 5xx).
var errTransient = errors.New("keycloak: transient")

// New builds the adapter. BaseURL must be <keycloak>/admin/realms/<realm>; the token endpoint is derived.
func New(cfg Config, client *http.Client) (*Admin, error) {
	i := strings.Index(cfg.BaseURL, "/admin/realms/")
	if i < 0 || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("keycloak: EW_KC_ADMIN_BASE_URL must end with /admin/realms/<realm> and credentials are required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if client == nil {
		client = &http.Client{}
	}
	realm := cfg.BaseURL[i+len("/admin/realms/"):]
	transient := func(_ result, err error) bool {
		return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, port.ErrKCConflict) && !errors.Is(err, port.ErrKCNotFound) &&
			!errors.Is(err, errClient)
	}
	cb := circuitbreaker.NewBuilder[result]().HandleIf(transient).
		WithFailureRateThreshold(cfg.FailureRatio, 10, time.Minute).WithDelay(30 * time.Second).WithSuccessThreshold(2).Build()
	rp := retrypolicy.NewBuilder[result]().HandleIf(transient).
		WithMaxRetries(cfg.MaxRetries).WithBackoff(100*time.Millisecond, time.Second).ReturnLastFailure().Build()
	return &Admin{
		cfg:      cfg,
		tokenURL: cfg.BaseURL[:i] + "/realms/" + realm + "/protocol/openid-connect/token",
		http:     client,
		retry:    failsafe.With[result](rp, cb),
		once:     failsafe.With[result](cb),
	}, nil
}

// errClient marks 4xx responses other than 404/409 (a bug or a misconfiguration: not retried).
var errClient = errors.New("keycloak: request rejected")

func (a *Admin) accessToken(ctx context.Context, refresh bool) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !refresh && a.token != "" && time.Until(a.expiry) > 30*time.Second {
		return a.token, nil
	}
	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
	defer cancel()
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {a.cfg.ClientID}, "client_secret": {a.cfg.ClientSecret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("keycloak token: status %d: %w", resp.StatusCode, statusErr(resp.StatusCode))
	}
	var t struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil || t.AccessToken == "" {
		return "", fmt.Errorf("keycloak token: invalid response")
	}
	a.token, a.expiry = t.AccessToken, time.Now().Add(time.Duration(t.ExpiresIn)*time.Second)
	return a.token, nil
}

func statusErr(code int) error {
	switch {
	case code == http.StatusNotFound:
		return port.ErrKCNotFound
	case code == http.StatusConflict:
		return port.ErrKCConflict
	case code == http.StatusTooManyRequests || code >= 500:
		return errTransient
	default:
		return errClient
	}
}

// call performs one Admin API request (path relative to BaseURL) and decodes a JSON response into out.
// It returns the Location header of 201 responses. idempotent selects the retrying executor.
func (a *Admin) call(ctx context.Context, op, method, path string, body, out any, idempotent bool) (string, error) {
	ctx, span := otel.Tracer("eduworld/identity/keycloak").Start(ctx, "keycloak."+op)
	defer span.End()
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return "", err
		}
	}
	exec := a.once
	if idempotent {
		exec = a.retry
	}
	refreshed := false
	res, err := exec.WithContext(ctx).Get(func() (result, error) {
		tok, err := a.accessToken(ctx, false)
		if err != nil {
			return result{}, err
		}
		res, err := a.do(ctx, method, path, tok, payload, body != nil)
		if err != nil {
			return result{}, err
		}
		if res.status == http.StatusUnauthorized && !refreshed {
			refreshed = true
			if _, err := a.accessToken(ctx, true); err != nil {
				return result{}, err
			}
			return result{}, errTransient
		}
		if res.status >= 300 {
			return result{}, fmt.Errorf("keycloak %s: status %d: %w", op, res.status, statusErr(res.status))
		}
		return res, nil
	})
	if err != nil {
		if !errors.Is(err, port.ErrKCNotFound) {
			span.SetStatus(codes.Error, op)
		}
		span.SetAttributes(attribute.String("error.kind", errKind(err)))
		orgSync.WithLabelValues(op, errKind(err)).Inc()
		return "", err
	}
	orgSync.WithLabelValues(op, "ok").Inc()
	span.SetAttributes(attribute.Int("http.response.status_code", res.status))
	if out != nil {
		if err := json.Unmarshal(res.body, out); err != nil {
			return "", fmt.Errorf("keycloak %s: decode: %w", op, err)
		}
	}
	return res.location, nil
}

// result is a fully read Admin API response (bodies are small JSON documents, capped at maxBody).
type result struct {
	status   int
	location string
	body     []byte
}

const maxBody = 4 << 20

// do sends one request with its own timeout and reads the whole response.
func (a *Admin) do(ctx context.Context, method, path, token string, payload []byte, hasBody bool) (result, error) {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, a.cfg.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return result{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return result{}, err
	}
	return result{status: resp.StatusCode, location: resp.Header.Get("Location"), body: b}, nil
}

func errKind(err error) string {
	switch {
	case errors.Is(err, port.ErrKCNotFound):
		return "not_found"
	case errors.Is(err, port.ErrKCConflict):
		return "conflict"
	case errors.Is(err, circuitbreaker.ErrOpen):
		return "circuit_open"
	case errors.Is(err, errClient):
		return "rejected"
	default:
		return "transient"
	}
}

func idFromLocation(loc string) (string, error) {
	i := strings.LastIndexByte(loc, '/')
	if i < 0 || i == len(loc)-1 {
		return "", fmt.Errorf("keycloak: missing Location of created resource")
	}
	return loc[i+1:], nil
}

type orgRep struct {
	ID         string              `json:"id,omitempty"`
	Name       string              `json:"name"`
	Alias      string              `json:"alias"`
	Enabled    bool                `json:"enabled"`
	Attributes map[string][]string `json:"attributes,omitempty"`
	Domains    []domainRep         `json:"domains,omitempty"`
}

type domainRep struct {
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
}

func (o orgRep) port() port.KCOrganization {
	return port.KCOrganization{ID: o.ID, Alias: o.Alias, Name: o.Name, Enabled: o.Enabled, Attributes: o.Attributes}
}

// placeholderDomain: identity-first login by domain is outside Release 1.0 (KEYCLOAK_SSO §2.1, ADR-0063);
// a reserved, unverified domain per alias satisfies Keycloak versions that require one.
func placeholderDomain(alias string) []domainRep {
	return []domainRep{{Name: alias + ".tenant.invalid"}}
}

const pageSize = 100

func (a *Admin) FindOrganizationByAlias(ctx context.Context, alias string) (port.KCOrganization, error) {
	for first := 0; ; first += pageSize {
		var page []orgRep
		q := url.Values{"first": {fmt.Sprint(first)}, "max": {fmt.Sprint(pageSize)}, "briefRepresentation": {"false"}}
		if _, err := a.call(ctx, "find_organization", http.MethodGet, "/organizations?"+q.Encode(), nil, &page, true); err != nil {
			return port.KCOrganization{}, err
		}
		for _, o := range page {
			if o.Alias == alias {
				return o.port(), nil
			}
		}
		if len(page) < pageSize {
			return port.KCOrganization{}, port.ErrKCNotFound
		}
	}
}

func (a *Admin) EnsureOrganization(ctx context.Context, org port.KCOrganization) (string, error) {
	if o, err := a.FindOrganizationByAlias(ctx, org.Alias); err == nil {
		return o.ID, nil
	} else if !errors.Is(err, port.ErrKCNotFound) {
		return "", err
	}
	rep := orgRep{Name: org.Name, Alias: org.Alias, Enabled: org.Enabled, Attributes: org.Attributes, Domains: placeholderDomain(org.Alias)}
	loc, err := a.call(ctx, "create_organization", http.MethodPost, "/organizations", rep, nil, false)
	if errors.Is(err, port.ErrKCConflict) { // created concurrently or by an earlier attempt
		o, ferr := a.FindOrganizationByAlias(ctx, org.Alias)
		if ferr != nil {
			return "", ferr
		}
		return o.ID, nil
	}
	if err != nil {
		return "", err
	}
	return idFromLocation(loc)
}

func (a *Admin) UpdateOrganization(ctx context.Context, org port.KCOrganization) error {
	var cur orgRep
	if _, err := a.call(ctx, "get_organization", http.MethodGet, "/organizations/"+url.PathEscape(org.ID), nil, &cur, true); err != nil {
		return err
	}
	cur.Name, cur.Enabled = org.Name, org.Enabled
	if org.Attributes != nil {
		cur.Attributes = org.Attributes
	}
	_, err := a.call(ctx, "update_organization", http.MethodPut, "/organizations/"+url.PathEscape(org.ID), cur, nil, true)
	return err
}

type userRep struct {
	ID         string              `json:"id,omitempty"`
	Username   string              `json:"username"`
	Enabled    bool                `json:"enabled"`
	FirstName  string              `json:"firstName,omitempty"`
	Attributes map[string][]string `json:"attributes,omitempty"`
}

func (a *Admin) ListOrganizationMembers(ctx context.Context, orgID string) ([]port.KCUser, error) {
	var out []port.KCUser
	for first := 0; ; first += pageSize {
		var page []userRep
		q := url.Values{"first": {fmt.Sprint(first)}, "max": {fmt.Sprint(pageSize)}}
		if _, err := a.call(ctx, "list_members", http.MethodGet, "/organizations/"+url.PathEscape(orgID)+"/members?"+q.Encode(), nil, &page, true); err != nil {
			return nil, err
		}
		for _, u := range page {
			out = append(out, port.KCUser{ID: u.ID, Username: u.Username, Enabled: u.Enabled})
		}
		if len(page) < pageSize {
			return out, nil
		}
	}
}

// ListMemberOrganizations returns the Organizations of a member (Keycloak 26: /organizations/members/{id}/organizations).
func (a *Admin) ListMemberOrganizations(ctx context.Context, userID string) ([]port.KCOrganization, error) {
	var orgs []orgRep
	if _, err := a.call(ctx, "member_organizations", http.MethodGet, "/organizations/members/"+url.PathEscape(userID)+"/organizations?briefRepresentation=false", nil, &orgs, true); err != nil {
		return nil, err
	}
	out := make([]port.KCOrganization, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, o.port())
	}
	return out, nil
}

func (a *Admin) AddOrganizationMember(ctx context.Context, orgID, userID string) error {
	_, err := a.call(ctx, "add_member", http.MethodPost, "/organizations/"+url.PathEscape(orgID)+"/members", userID, nil, true)
	if errors.Is(err, port.ErrKCConflict) {
		return nil // already a member
	}
	return err
}

func (a *Admin) RemoveOrganizationMember(ctx context.Context, orgID, userID string) error {
	_, err := a.call(ctx, "remove_member", http.MethodDelete, "/organizations/"+url.PathEscape(orgID)+"/members/"+url.PathEscape(userID), nil, nil, true)
	if errors.Is(err, port.ErrKCNotFound) {
		return nil // not a member
	}
	return err
}

func (a *Admin) FindUserByUsername(ctx context.Context, username string) (port.KCUser, error) {
	var users []userRep
	q := url.Values{"username": {username}, "exact": {"true"}}
	if _, err := a.call(ctx, "find_user", http.MethodGet, "/users?"+q.Encode(), nil, &users, true); err != nil {
		return port.KCUser{}, err
	}
	for _, u := range users {
		if strings.EqualFold(u.Username, username) {
			return port.KCUser{ID: u.ID, Username: u.Username, Enabled: u.Enabled}, nil
		}
	}
	return port.KCUser{}, port.ErrKCNotFound
}

func (a *Admin) CreateUser(ctx context.Context, u port.KCNewUser) (string, error) {
	loc, err := a.call(ctx, "create_user", http.MethodPost, "/users",
		userRep{Username: u.Username, Enabled: u.Enabled, FirstName: u.FirstName, Attributes: u.Attributes}, nil, false)
	if err != nil {
		return "", err
	}
	return idFromLocation(loc)
}

func (a *Admin) SetPassword(ctx context.Context, userID, password string, temporary bool) error {
	body := map[string]any{"type": "password", "value": password, "temporary": temporary}
	_, err := a.call(ctx, "set_password", http.MethodPut, "/users/"+url.PathEscape(userID)+"/reset-password", body, nil, true)
	return err
}

type roleRep struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *Admin) roles(ctx context.Context, names []string) ([]roleRep, error) {
	out := make([]roleRep, 0, len(names))
	for _, n := range names {
		var r roleRep
		if _, err := a.call(ctx, "get_role", http.MethodGet, "/roles/"+url.PathEscape(n), nil, &r, true); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (a *Admin) AddRealmRoles(ctx context.Context, userID string, roles ...string) error {
	reps, err := a.roles(ctx, roles)
	if err != nil {
		return err
	}
	_, err = a.call(ctx, "add_realm_roles", http.MethodPost, "/users/"+url.PathEscape(userID)+"/role-mappings/realm", reps, nil, true)
	return err
}

func (a *Admin) RemoveRealmRoles(ctx context.Context, userID string, roles ...string) error {
	reps, err := a.roles(ctx, roles)
	if err != nil {
		return err
	}
	_, err = a.call(ctx, "remove_realm_roles", http.MethodDelete, "/users/"+url.PathEscape(userID)+"/role-mappings/realm", reps, nil, true)
	return err
}

// DisableUser sets enabled=false on the user (idempotent PUT of the representation).
func (a *Admin) DisableUser(ctx context.Context, userID string) error {
	var cur map[string]any
	if _, err := a.call(ctx, "get_user", http.MethodGet, "/users/"+url.PathEscape(userID), nil, &cur, true); err != nil {
		return err
	}
	cur["enabled"] = false
	_, err := a.call(ctx, "disable_user", http.MethodPut, "/users/"+url.PathEscape(userID), cur, nil, true)
	return err
}

func (a *Admin) LogoutUser(ctx context.Context, userID string) error {
	_, err := a.call(ctx, "logout_user", http.MethodPost, "/users/"+url.PathEscape(userID)+"/logout", nil, nil, true)
	return err
}
