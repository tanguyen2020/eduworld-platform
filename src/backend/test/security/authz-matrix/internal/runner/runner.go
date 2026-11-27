package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/cases"
)

// Verdict of one request.
type Verdict string

// Verdicts.
const (
	Pass         Verdict = "pass"
	Fail         Verdict = "fail"
	Observed     Verdict = "observed"     // no assertion (positive access), answered without 5xx
	Inconclusive Verdict = "inconclusive" // the contract check (422) or a persistent 429 answered first
	Skipped      Verdict = "skipped"      // no credentials for the role, or not applicable to this target
)

// Result is the outcome of one case or probe. It never carries a token, a body or a response payload (R14).
type Result struct {
	Kind     string         `json:"kind"` // matrix | internal | maxlength | tenant_header
	OpID     string         `json:"operation_id,omitempty"`
	Role     cases.Role     `json:"role,omitempty"`
	Relation cases.Relation `json:"relation,omitempty"`
	Field    string         `json:"field,omitempty"`
	Target   string         `json:"target,omitempty"` // probe: gateway and path
	Rule     string         `json:"rule"`
	Expected string         `json:"expected"`
	Status   int            `json:"status,omitempty"`
	Code     string         `json:"code,omitempty"`
	Verdict  Verdict        `json:"verdict"`
	Note     string         `json:"note,omitempty"`
}

// Runner sends the cases. BaseURL is the public API base (…/api/v1); InternalURL the internal gateway API base
// (empty: everything through BaseURL, e.g. core-api directly on a developer machine).
type Runner struct {
	BaseURL     string
	InternalURL string
	HTTP        *http.Client
	Fixtures    Fixtures
	Creds       Credentials
	// Interval between two requests (politeness towards the shared environment and its rate limits).
	Interval time.Duration
	// MaxRetries on 429 honouring Retry-After (capped at 10 s per wait).
	MaxRetries int

	mu     sync.Mutex
	tokens map[cases.Role]string
	last   time.Time
}

func (r *Runner) client() *http.Client {
	if r.HTTP != nil {
		return r.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (r *Runner) pace(ctx context.Context) error {
	r.mu.Lock()
	wait := time.Until(r.last.Add(r.Interval))
	if wait < 0 {
		wait = 0
	}
	r.last = time.Now().Add(wait)
	r.mu.Unlock()
	if wait == 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type answer struct {
	status int
	code   string
	fields []fieldError
	header http.Header
}

type fieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// do sends one request (429 → wait Retry-After, at most MaxRetries times).
func (r *Runner) do(ctx context.Context, method, target string, hdr http.Header, body any) (answer, error) {
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return answer{}, err
		}
		raw = b
	}
	for attempt := 0; ; attempt++ {
		if err := r.pace(ctx); err != nil {
			return answer{}, err
		}
		req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(raw))
		if err != nil {
			return answer{}, err
		}
		for k, vs := range hdr {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		if raw != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := r.client().Do(req)
		if err != nil {
			return answer{}, err
		}
		a := answer{status: resp.StatusCode, header: resp.Header}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		var p struct {
			Code   string       `json:"code"`
			Errors []fieldError `json:"errors"`
		}
		if json.Unmarshal(b, &p) == nil {
			a.code, a.fields = p.Code, p.Errors
		}
		if a.status != http.StatusTooManyRequests || attempt >= r.MaxRetries {
			return a, nil
		}
		wait := time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			wait = time.Duration(s) * time.Second
		}
		if wait > 10*time.Second {
			wait = 10 * time.Second
		}
		select {
		case <-ctx.Done():
			return a, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// Token returns the access token of role (logged in once and cached). An empty token means no credentials.
func (r *Runner) Token(ctx context.Context, role cases.Role) (string, error) {
	r.mu.Lock()
	if t, ok := r.tokens[role]; ok {
		r.mu.Unlock()
		return t, nil
	}
	r.mu.Unlock()
	tok, pw := r.Creds(role)
	acc, hasAcc := r.Fixtures.Accounts[role]
	if tok == "" && pw != "" && hasAcc {
		var err error
		tok, err = r.login(ctx, acc, pw)
		if err != nil {
			return "", fmt.Errorf("runner: login %s: %w", role, err)
		}
	}
	r.mu.Lock()
	if r.tokens == nil {
		r.tokens = map[cases.Role]string{}
	}
	r.tokens[role] = tok
	r.mu.Unlock()
	return tok, nil
}

func (r *Runner) login(ctx context.Context, acc Account, password string) (string, error) {
	device := uuid.NewString()
	var path string
	var body map[string]any
	switch acc.Kind {
	case "game":
		path = "/auth/game/login"
		body = map[string]any{"username": acc.Username, "password": password, "device_id": device,
			"client_version": r.Fixtures.ClientVersion, "platform": r.Fixtures.Platform}
	case "portal":
		path = "/auth/portal/login"
		body = map[string]any{"username": acc.Username, "password": password, "device_id": device}
	default:
		return "", fmt.Errorf("account kind %q (game|portal)", acc.Kind)
	}
	raw, _ := json.Marshal(body)
	if err := r.pace(ctx); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(r.BaseURL, "/")+path, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", device)
	resp, err := r.client().Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		Tokens      struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", err
	}
	if out.Tokens.AccessToken != "" {
		return out.Tokens.AccessToken, nil
	}
	if out.AccessToken == "" {
		return "", errors.New("no access_token in the answer")
	}
	return out.AccessToken, nil
}

func (r *Runner) baseFor(op opRouting) string {
	if (op.internal || op.adminBearer) && r.InternalURL != "" {
		return strings.TrimRight(r.InternalURL, "/")
	}
	return strings.TrimRight(r.BaseURL, "/")
}

type opRouting struct{ internal, adminBearer bool }

// fillPath substitutes the path parameters from the fixture scope; synthetic reports a random UUID was used.
func (r *Runner) fillPath(path string, params []string, scope string) (string, bool) {
	synthetic := false
	for _, p := range params {
		v := r.Fixtures.IDs[scope][p]
		if v == "" {
			v, synthetic = uuid.NewString(), true
		}
		path = strings.ReplaceAll(path, "{"+p+"}", url.PathEscape(v))
	}
	return path, synthetic
}

func (r *Runner) headers(token string, tenant string, idempotent bool) http.Header {
	h := http.Header{}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	if tenant != "" {
		h.Set("X-Tenant-Id", tenant)
	}
	if idempotent {
		k, err := uuid.NewV7()
		if err != nil {
			k = uuid.New()
		}
		h.Set("Idempotency-Key", k.String())
	}
	h.Set("X-Device-Id", uuid.NewString())
	h.Set("If-Match", `"authz-matrix"`)
	h.Set("Accept", "application/json")
	return h
}

func expected(e cases.Expect) string {
	if e.Observe {
		return "observe (no 5xx)"
	}
	s := make([]string, 0, len(e.Statuses))
	for _, st := range e.Statuses {
		s = append(s, strconv.Itoa(st))
	}
	out := strings.Join(s, "|")
	if len(e.Codes) > 0 {
		out += " " + strings.Join(e.Codes, "|")
	}
	return out
}

// RunMatrix sends every case (F132-AC-15).
func (r *Runner) RunMatrix(ctx context.Context, cs []cases.Case) []Result {
	out := make([]Result, 0, len(cs))
	for _, c := range cs {
		res := Result{Kind: "matrix", OpID: c.OpID, Role: c.Role, Relation: c.Relation, Rule: c.Expect.Rule, Expected: expected(c.Expect)}
		token := ""
		if c.Role != cases.Anonymous {
			t, err := r.Token(ctx, c.Role)
			if err != nil || t == "" {
				res.Verdict, res.Note = Skipped, "no credentials for the role"
				if err != nil {
					res.Note = err.Error()
				}
				out = append(out, res)
				continue
			}
			token = t
		}
		path, synthetic := r.fillPath(c.Op.Path, c.Op.PathParams, scopeOf(c.Relation))
		tenant := ""
		if c.Op.TenantScoped && (!c.Role.Platform() || c.Op.TenantRequired) {
			tenant = r.Fixtures.Tenants.A
			if c.ForeignTenantHeader {
				tenant = r.Fixtures.Tenants.B
			}
		}
		var body any
		if c.Op.Body != nil {
			if b, ok := r.Fixtures.Bodies[c.OpID]; ok {
				body = b
			} else {
				body = cases.MinimalBody(c.Op.Body)
			}
		}
		target := r.baseFor(opRouting{c.Op.Internal, c.Op.AdminBearer}) + path
		a, err := r.do(ctx, c.Op.Method, target, r.headers(token, tenant, c.Op.Idempotent), body)
		if err != nil {
			res.Verdict, res.Note = Fail, "transport: "+err.Error()
			out = append(out, res)
			continue
		}
		res.Status, res.Code = a.status, a.code
		switch {
		case c.Expect.Satisfied(a.status, a.code) && c.Expect.Observe:
			res.Verdict = Observed
		case c.Expect.Satisfied(a.status, a.code):
			res.Verdict = Pass
		case a.status == http.StatusUnprocessableEntity && c.Expect.Rule == "cross_tenant_404_or_403":
			res.Verdict, res.Note = Inconclusive, "contract check answered first: add a fixture body"
		case a.status == http.StatusTooManyRequests:
			res.Verdict, res.Note = Inconclusive, "rate limited"
		default:
			res.Verdict = Fail
			if a.status >= 200 && a.status < 300 && c.Expect.Rule == "cross_tenant_404_or_403" {
				res.Note = "LEAK: resource of another tenant answered 2xx"
			}
		}
		if synthetic && res.Note == "" {
			res.Note = "synthetic_id"
		}
		out = append(out, res)
	}
	return out
}

// DirectTarget is a host:port of a backend binary probed for in-process facades exposed over HTTP.
type DirectTarget string

// RunInternal sends the probes of SCOPE=internal (F132-AC-16): 403/404 or a refused connection pass.
func (r *Runner) RunInternal(ctx context.Context, probes []cases.Probe, direct []DirectTarget, facadePaths []string) []Result {
	var out []Result
	split := r.InternalURL != "" && strings.TrimRight(r.InternalURL, "/") != strings.TrimRight(r.BaseURL, "/")
	for _, p := range probes {
		res := Result{Kind: "internal", Rule: p.Reason, Target: string(p.Gateway) + " " + p.Method + " " + p.Path, Expected: "403|404 or no connection"}
		base := r.BaseURL
		if p.Gateway == cases.InternalGateway {
			base = r.InternalURL
		}
		if base == "" || (p.Reason == "x_internal_not_public" && !split) {
			res.Verdict, res.Note = Skipped, "no separate gateway on this target"
			out = append(out, res)
			continue
		}
		target := strings.TrimRight(base, "/") + p.Path
		if p.FromOrigin {
			target = origin(base) + p.Path
		}
		out = append(out, r.probe(ctx, res, p.Method, target))
	}
	for _, d := range direct {
		for _, fp := range facadePaths {
			res := Result{Kind: "internal", Rule: "facade_not_on_binary_port", Target: string(d) + " GET " + fp, Expected: "404 or no connection"}
			out = append(out, r.probe(ctx, res, http.MethodGet, "http://"+string(d)+fp))
		}
	}
	return out
}

func (r *Runner) probe(ctx context.Context, res Result, method, target string) Result {
	a, err := r.do(ctx, method, target, http.Header{"Accept": {"application/json"}}, nil)
	var op *net.OpError
	switch {
	case err != nil && (errors.As(err, &op) || strings.Contains(err.Error(), "connection refused")):
		res.Verdict, res.Note = Pass, "no connection"
	case err != nil:
		res.Verdict, res.Note = Fail, "transport: "+err.Error()
	case a.status == http.StatusNotFound || (a.status == http.StatusForbidden && res.Rule != "facade_not_on_binary_port"):
		res.Status, res.Code, res.Verdict = a.status, a.code, Pass
	default:
		res.Status, res.Code, res.Verdict = a.status, a.code, Fail
	}
	return res
}

func origin(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return strings.TrimRight(base, "/")
	}
	return u.Scheme + "://" + u.Host
}

// guardCodes answer before the contract check when the role does not match the route group.
var guardCodes = map[string]bool{"PORTAL_ACCESS_STAFF_ONLY": true, "GAME_ACCESS_STUDENT_ONLY": true, "PERMISSION_DENIED": true, "TENANT_MISMATCH": true}

// rolesFor are the roles tried in order so the request passes the route guards and reaches the contract check.
func rolesFor(cl cases.Class) []cases.Role {
	switch cl {
	case cases.ClassPublic:
		return []cases.Role{cases.Anonymous}
	case cases.ClassPlayer:
		return []cases.Role{cases.Student}
	case cases.ClassPlatform:
		return []cases.Role{cases.SystemAdmin}
	case cases.ClassPortal:
		return []cases.Role{cases.TenantAdmin, cases.SchoolAdmin}
	}
	return []cases.Role{cases.Student, cases.TenantAdmin}
}

// RunLength sends the maxLength cases and the missing X-Tenant-Id cases (F132-AC-17).
func (r *Runner) RunLength(ctx context.Context, ls []cases.LengthCase, hs []cases.HeaderCase, ov cases.Overrides) []Result {
	var out []Result
	for _, c := range ls {
		res := Result{Kind: "maxlength", OpID: c.OpID, Field: c.Field, Rule: "max_length_422", Expected: "422 VALIDATION_FAILED errors[" + c.Field + "]=MAX_LENGTH"}
		res = r.lengthOne(ctx, res, c, cases.ClassOf(c.Op, ov))
		out = append(out, res)
	}
	for _, h := range hs {
		res := Result{Kind: "tenant_header", OpID: h.OpID, Rule: "tenant_header_required_400", Expected: "400 TENANT_HEADER_REQUIRED"}
		role := rolesFor(cases.ClassOf(h.Op, ov))[0]
		if role == cases.SystemAdmin { // a platform role may omit the header by design (TenantIdHeaderOptional)
			role = cases.TenantAdmin
		}
		tok, err := r.Token(ctx, role)
		if err != nil || tok == "" {
			res.Verdict, res.Note = Skipped, "no credentials for "+string(role)
			out = append(out, res)
			continue
		}
		path, _ := r.fillPath(h.Op.Path, h.Op.PathParams, "a1")
		var body any
		if h.Op.Body != nil {
			body = cases.MinimalBody(h.Op.Body)
		}
		a, err := r.do(ctx, h.Op.Method, r.baseFor(opRouting{h.Op.Internal, h.Op.AdminBearer})+path, r.headers(tok, "", h.Op.Idempotent), body)
		switch {
		case err != nil:
			res.Verdict, res.Note = Fail, "transport: "+err.Error()
		case a.status == http.StatusBadRequest && a.code == "TENANT_HEADER_REQUIRED":
			res.Status, res.Code, res.Verdict = a.status, a.code, Pass
		default:
			res.Status, res.Code, res.Verdict = a.status, a.code, Fail
		}
		out = append(out, res)
	}
	return out
}

func (r *Runner) lengthOne(ctx context.Context, res Result, c cases.LengthCase, cl cases.Class) Result {
	path, _ := r.fillPath(c.Op.Path, c.Op.PathParams, "a1")
	target := r.baseFor(opRouting{c.Op.Internal, c.Op.AdminBearer}) + path
	for _, role := range rolesFor(cl) {
		tok := ""
		if role != cases.Anonymous {
			t, err := r.Token(ctx, role)
			if err != nil || t == "" {
				continue
			}
			tok = t
		}
		tenant := ""
		if c.Op.TenantRequired || (c.Op.TenantScoped && !role.Platform()) {
			tenant = r.Fixtures.Tenants.A
		}
		a, err := r.do(ctx, c.Op.Method, target, r.headers(tok, tenant, c.Op.Idempotent), c.Body)
		if err != nil {
			res.Verdict, res.Note = Fail, "transport: "+err.Error()
			return res
		}
		res.Role, res.Status, res.Code = role, a.status, a.code
		if a.status == http.StatusForbidden && guardCodes[a.code] {
			continue // the role does not pass the route guard: try the next one
		}
		if a.status == http.StatusUnprocessableEntity && a.code == "VALIDATION_FAILED" {
			for _, f := range a.fields {
				if f.Field == c.Field && f.Code == "MAX_LENGTH" {
					res.Verdict = Pass
					return res
				}
			}
			res.Verdict, res.Note = Fail, "422 without errors["+c.Field+"]=MAX_LENGTH"
			return res
		}
		if a.status == http.StatusTooManyRequests {
			res.Verdict, res.Note = Inconclusive, "rate limited"
			return res
		}
		res.Verdict = Fail
		return res
	}
	if res.Status == 0 {
		res.Verdict, res.Note = Skipped, "no credentials for the route group"
	} else {
		res.Verdict, res.Note = Inconclusive, "no available role passes the route guard"
	}
	return res
}
