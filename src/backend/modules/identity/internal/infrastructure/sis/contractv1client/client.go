// Package contractv1client is the HTTP client of a contract_v1 SIS connection (F017, SIS_INTEGRATION §4.2):
// the operations of specs/sis/eduworld-sis-contract.v1.yaml with OAuth2 client credentials, API key or mTLS
// credentials read from the secret store, a per-connection rate limit (sis_client_rate), exponential backoff
// with jitter (sis_retry_backoff) honouring Retry-After, and validation of every response against the JSON
// Schema extracted from the contract (tools/sisgen): unknown fields are dropped and counted, records
// breaking the contract are returned as violations. Request and response bodies are never logged (PII).
//
// The DTOs are the contract's own (validated maps typed by the embedded JSON Schema) instead of
// oapi-codegen types: the generator does not read OpenAPI 3.1 (type: [string, 'null']) and the contract stays
// the single source through tools/sisgen (OQ F017-2 [Proposed]).
package contractv1client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
)

// Defaults of CONFIG_DEFAULTS (sis_client_rate, sis_retry_backoff; SIS_INTEGRATION §4.2, §7).
const (
	DefaultRate          = 10               // requests per second per connection
	DefaultTimeout       = 15 * time.Second // per request
	DefaultMaxRetries    = 6
	DefaultBaseDelay     = time.Second
	DefaultMaxRetryAfter = 5 * time.Minute // cap of a Retry-After honoured by a sync [Proposed]
	DefaultPageLimit     = 500             // limit of list operations (contract default)
	slowDownFor          = time.Minute     // after a 429 the connection is paced at half rate [Proposed]
	maxBody              = 32 << 20        // bound of one response body
)

// Factory builds clients; rate limiters are shared by every client of a connection (all runs of a worker).
type Factory struct {
	Secrets   port.SecretStore
	Validator *contract.Validator
	// Transport is the base transport (egress proxy, tests); nil = http.DefaultTransport clone with TLS 1.2+.
	Transport  *http.Transport
	Rate       float64
	Timeout    time.Duration
	MaxRetries int
	BaseDelay  time.Duration
	Sleep      func(ctx context.Context, d time.Duration) error // tests
	Now        func() time.Time

	mu       sync.Mutex
	limiters map[string]*pacer
}

var _ port.SisProviderFactory = (*Factory)(nil)

// Client is the contract_v1 client of one connection.
type Client struct {
	f       *Factory
	conn    port.ContractConnection
	base    string
	http    *http.Client
	pacer   *pacer
	apiKey  string
	oauthID string
	oauthSK string

	mu       sync.Mutex
	token    string
	expiry   time.Time
	requests int
	retries  int
}

var _ port.SisProvider = (*Client)(nil)

func (f *Factory) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

func (f *Factory) sleep(ctx context.Context, d time.Duration) error {
	if f.Sleep != nil {
		return f.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (f *Factory) pacerFor(id string) *pacer {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.limiters == nil {
		f.limiters = map[string]*pacer{}
	}
	p := f.limiters[id]
	if p == nil {
		rate := f.Rate
		if rate <= 0 {
			rate = DefaultRate
		}
		p = &pacer{interval: time.Duration(float64(time.Second) / rate)}
		f.limiters[id] = p
	}
	return p
}

// oauthSecret is the secret file of an OAuth2 connection [Proposed: JSON {client_id, client_secret}, OQ F017-3].
type oauthSecret struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// For reads the credential of the connection and returns its client. A secret that cannot be read or
// parsed is port.ErrSisSecret (the run fails, the connection goes auth_error).
func (f *Factory) For(ctx context.Context, c port.ContractConnection) (port.SisProvider, error) {
	if f.Validator == nil || f.Secrets == nil {
		return nil, errors.New("contractv1client: factory needs a validator and a secret store")
	}
	secret, err := f.Secrets.Get(ctx, c.SecretRef)
	if err != nil {
		return nil, err
	}
	tr := f.Transport
	if tr == nil {
		tr = http.DefaultTransport.(*http.Transport).Clone()
	} else {
		tr = tr.Clone()
	}
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{}
	}
	tr.TLSClientConfig.MinVersion = tls.VersionTLS12 // SIS_INTEGRATION §9: TLS 1.2+, certificates verified
	cl := &Client{f: f, conn: c, base: strings.TrimRight(c.BaseURL, "/"), pacer: f.pacerFor(c.ID.String())}
	switch c.AuthType {
	case sis.AuthAPIKey:
		cl.apiKey = strings.TrimSpace(string(secret))
	case sis.AuthOAuth2:
		var s oauthSecret
		if json.Unmarshal(secret, &s) != nil || s.ClientID == "" || s.ClientSecret == "" {
			return nil, fmt.Errorf("contractv1client: oauth2 secret: %w", port.ErrSisSecret)
		}
		cl.oauthID, cl.oauthSK = s.ClientID, s.ClientSecret
	case sis.AuthMTLS:
		cert, err := tls.X509KeyPair(secret, secret) // one PEM bundle: certificate chain + private key
		if err != nil {
			return nil, fmt.Errorf("contractv1client: mtls secret: %w", port.ErrSisSecret)
		}
		if blk, _ := pem.Decode(secret); blk == nil {
			return nil, fmt.Errorf("contractv1client: mtls secret: %w", port.ErrSisSecret)
		}
		tr.TLSClientConfig.Certificates = []tls.Certificate{cert}
		if tr.TLSClientConfig.RootCAs == nil {
			tr.TLSClientConfig.RootCAs, _ = x509.SystemCertPool()
		}
	default:
		return nil, fmt.Errorf("contractv1client: auth type %q: %w", c.AuthType, port.ErrSisSecret)
	}
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	cl.http = &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // never follow a redirect with the credential
	}}
	return cl, nil
}

// Stats returns the request and retry counters of the client.
func (c *Client) Stats() port.SisClientStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return port.SisClientStats{Requests: c.requests, Retries: c.retries}
}

// ---------- pacing (sis_client_rate) ----------

type pacer struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
	slowTill time.Time
}

// wait returns how long to wait before the next request and books the slot.
func (p *pacer) reserve(now time.Time) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	step := p.interval
	if now.Before(p.slowTill) {
		step *= 2 // after a 429 the connection is called at half rate (F017-AC-11)
	}
	at := p.next
	if at.Before(now) {
		at = now
	}
	p.next = at.Add(step)
	return at.Sub(now)
}

func (p *pacer) slowDown(until time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if until.After(p.slowTill) {
		p.slowTill = until
	}
}

// ---------- requests ----------

type httpError struct {
	status     int
	retryAfter time.Duration
}

func (e *httpError) Error() string { return "sis: http " + strconv.Itoa(e.status) }

func statusClass(code int) string {
	switch {
	case code == http.StatusTooManyRequests:
		return "429"
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	default:
		return "2xx"
	}
}

func (c *Client) count(retry bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests++
	if retry {
		c.retries++
	}
}

// get calls one operation with retries (only GET: idempotent, R29) and decodes the JSON body.
func (c *Client) get(ctx context.Context, op, path string, q url.Values, out any, retry bool) error {
	maxRetries := c.f.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}
	if !retry {
		maxRetries = 0
	}
	base := c.f.BaseDelay
	if base <= 0 {
		base = DefaultBaseDelay
	}
	refreshed := false
	for attempt := 0; ; attempt++ {
		if retry {
			if d := c.pacer.reserve(c.f.now()); d > 0 {
				if err := c.f.sleep(ctx, d); err != nil {
					return err
				}
			}
		}
		c.count(attempt > 0)
		err := c.once(ctx, op, path, q, out)
		if err == nil {
			return nil
		}
		var he *httpError
		switch {
		case errors.As(err, &he) && (he.status == http.StatusUnauthorized || he.status == http.StatusForbidden):
			if he.status == http.StatusUnauthorized && c.oauthID != "" && !refreshed {
				refreshed = true // the token may have expired during the run: one new token, then auth_error
				c.mu.Lock()
				c.token = ""
				c.mu.Unlock()
				attempt--
				continue
			}
			return fmt.Errorf("sis %s: %w", op, port.ErrSisAuth)
		case errors.Is(err, port.ErrSisAuth):
			return err
		case errors.As(err, &he) && he.status == http.StatusNotFound:
			return fmt.Errorf("sis %s: %w", op, port.ErrSisNotFound)
		case errors.As(err, &he) && he.status == http.StatusGone:
			return fmt.Errorf("sis %s: %w", op, port.ErrSisWatermarkExpired)
		case errors.Is(err, port.ErrSisBadResponse), errors.Is(err, context.Canceled):
			return err
		case errors.As(err, &he) && he.status < 500 && he.status != http.StatusTooManyRequests && he.status != http.StatusRequestTimeout:
			return fmt.Errorf("sis %s: http %d: %w", op, he.status, port.ErrSisBadResponse)
		}
		if ctx.Err() != nil {
			return fmt.Errorf("sis %s: %w: %w", op, port.ErrSisUnavailable, ctx.Err())
		}
		if attempt >= maxRetries {
			return fmt.Errorf("sis %s: %w", op, port.ErrSisUnavailable)
		}
		wait := base << attempt
		wait += time.Duration(rand.Int64N(int64(base))) //nolint:gosec // jitter, not security
		if errors.As(err, &he) && he.retryAfter > 0 {
			wait = min(he.retryAfter, DefaultMaxRetryAfter)
		}
		if errors.As(err, &he) && he.status == http.StatusTooManyRequests {
			c.pacer.slowDown(c.f.now().Add(slowDownFor))
		}
		if err := c.f.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func (c *Client) once(ctx context.Context, op, path string, q url.Values, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("sis %s: request: %w", op, port.ErrSisBadResponse)
	}
	req.Header.Set("Accept", "application/json")
	if err := c.authorize(ctx, req); err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		metrics.HTTPRequest(c.conn.ID, op, "error")
		return fmt.Errorf("sis %s: transport: %w", op, err) // retried as unavailable
	}
	defer func() { _ = resp.Body.Close() }()
	metrics.HTTPRequest(c.conn.ID, op, statusClass(resp.StatusCode))
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return &httpError{status: resp.StatusCode, retryAfter: retryAfter(resp.Header.Get("Retry-After"), c.f.now())}
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxBody))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("sis %s: body: %w", op, port.ErrSisBadResponse)
	}
	return nil
}

// retryAfter parses Retry-After (seconds or HTTP date).
func retryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

func (c *Client) authorize(ctx context.Context, req *http.Request) error {
	switch {
	case c.apiKey != "":
		req.Header.Set("X-Api-Key", c.apiKey)
	case c.oauthID != "":
		tok, err := c.accessToken(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return nil
}

// accessToken returns a cached client-credentials token (refreshed 30 s before expiry).
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.f.now().Before(c.expiry.Add(-30*time.Second)) {
		return c.token, nil
	}
	scope := c.conn.OAuthScope
	if scope == "" {
		scope = "eduworld.sis.read"
	}
	form := url.Values{"grant_type": {"client_credentials"}, "scope": {scope}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.conn.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("sis token: %w", port.ErrSisAuth)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(c.oauthID), url.QueryEscape(c.oauthSK))
	resp, err := c.http.Do(req)
	if err != nil {
		metrics.HTTPRequest(c.conn.ID, "token", "error")
		return "", fmt.Errorf("sis token: transport: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	metrics.HTTPRequest(c.conn.ID, "token", statusClass(resp.StatusCode))
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return "", &httpError{status: resp.StatusCode, retryAfter: retryAfter(resp.Header.Get("Retry-After"), c.f.now())}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sis token: http %d: %w", resp.StatusCode, port.ErrSisAuth)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil || body.AccessToken == "" {
		return "", fmt.Errorf("sis token: body: %w", port.ErrSisAuth)
	}
	if body.ExpiresIn <= 0 {
		body.ExpiresIn = 300
	}
	c.token, c.expiry = body.AccessToken, c.f.now().Add(time.Duration(body.ExpiresIn)*time.Second)
	return c.token, nil
}
