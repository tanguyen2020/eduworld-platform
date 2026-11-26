package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/featuregate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// BasePath is the public API prefix.
const BasePath = "/api/v1"

// TenantMode says how an operation treats X-Tenant-Id (generated from the TenantIdHeader parameter of
// the spec; exempt routes such as /me/tenants use TenantNone — `tenant_header_required`, ADR-0065).
type TenantMode int

const (
	TenantRequired TenantMode = iota // missing → 400 TENANT_HEADER_REQUIRED
	TenantOptional                   // used when present (System Admin cross-tenant reads omit it)
	TenantNone                       // never read
)

// Guard is a route guard (bit set).
type Guard uint8

const (
	GuardNone         Guard = 0
	GuardPortalStaff  Guard = 1 << iota // azp = portal client and user_type = staff
	GuardGamePlayer                     // azp = game client and user_type = student (ADR-0031)
	GuardPlatformRole                   // a platform role (ADR-0027; no MFA since ADR-0094)
)

// Operation describes one public operation (generated from OpenAPI in later features).
type Operation struct {
	ID      string
	Module  string // owning module (metric/log label)
	Method  string
	Pattern string // path relative to BasePath, chi syntax
	Public  bool   // no token (login, refresh, client-config)
	Tenant  TenantMode
	Guard   Guard
	// Idempotency and SuccessStatus come from the IdempotencyKey parameter and the 2xx response of the spec.
	Idempotency   IdempotencyMode
	SuccessStatus int
	// PreValidate (public operations only) runs before contract validation, for checks the contract says
	// answer first: the CSRF double-submit of /auth/portal/token:refresh and /logout must be 403
	// CSRF_TOKEN_INVALID, not 422, and a missing refresh cookie 401 (ADR-0091 §4, F012-AC-30).
	PreValidate func(http.Handler) http.Handler
}

var (
	portalPrefixes   = []string{"/cms/", "/lms/", "/tenant/", "/schools/", "/game-admin/", "/admin/"}
	platformPrefixes = []string{"/game-admin/", "/admin/"}
)

func hasPrefix(p string, prefixes []string) bool {
	for _, pre := range prefixes {
		if strings.HasPrefix(p, pre) || p+"/" == pre {
			return true
		}
	}
	return false
}

// effectiveGuard adds guards implied by the route group so an operation cannot forget them (API_DESIGN §11).
func effectiveGuard(op Operation) Guard {
	g := op.Guard
	if hasPrefix(op.Pattern, portalPrefixes) {
		g |= GuardPortalStaff
	}
	if hasPrefix(op.Pattern, platformPrefixes) {
		g |= GuardPlatformRole
	}
	return g
}

// Deps are the collaborators of the server, wired in the composition root (src/backend/cmd/core-api).
type Deps struct {
	Auth       AuthConfig
	Keys       KeySource
	Resolver   authz.PrincipalResolver
	Clock      clock.Clock
	Registerer prometheus.Registerer
	Logger     *slog.Logger
	// Spec is the merged public OpenAPI spec; when set, every operation must exist in it and requests are
	// validated against it (ADR-0041, F002-AC-25).
	Spec *openapi3.T
	// Idempotency is the store of the module database for idempotent operations (F003 US3).
	Idempotency *idempotency.Store
	// IdempotencyStores, when set, holds the store of each module database by Operation.Module (a binary serving
	// several modules: the record must live in the database of the module whose transaction writes it).
	// Operations of a module missing from the map use Idempotency.
	IdempotencyStores map[string]*idempotency.Store
	// Middlewares run first for every request (request id, OTel, access log — package observability).
	Middlewares []func(http.Handler) http.Handler
	// FeatureGate blocks Admin Portal operations whose x-portal-action (or parent menu) is turned off
	// (ADR-0090 §3, F012 US9). Optional: nil = off. It needs Spec, which carries x-portal-action.
	FeatureGate *featuregate.Gate
}

// Server is the public HTTP server: chi router + per-operation middleware chain.
type Server struct {
	d        Deps
	verifier *Verifier
	router   chi.Router
	api      chi.Router
	log      *slog.Logger
	ops      map[string]Operation // mounted operations by ID (each at most once)
	routed   map[string]*routedOp // shared operations dispatched by a path parameter (x-service-routing)
}

// NewServer builds the server; it registers no route of its own (health endpoints live on the admin port).
func NewServer(d Deps) (*Server, error) {
	if d.Resolver == nil {
		return nil, errors.New("httpx: server needs a principal resolver")
	}
	if d.FeatureGate != nil && d.Spec == nil {
		return nil, errors.New("httpx: featuregate needs the OpenAPI spec (x-portal-action)")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	v, err := NewVerifier(d.Auth, d.Keys, d.Clock)
	if err != nil {
		return nil, err
	}
	if d.Registerer != nil {
		_ = d.Registerer.Register(idempotencyReplays)
		if err := d.Registerer.Register(tenantMismatch); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, err
			}
		}
	}
	s := &Server{d: d, verifier: v, log: d.Logger, ops: map[string]Operation{}, routed: map[string]*routedOp{}}
	r := chi.NewRouter()
	for _, m := range d.Middlewares {
		r.Use(m)
	}
	r.Use(s.recoverer)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, problem.New(problem.NotFound, "")) })
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, problem.New(problem.NotFound, "")) })
	s.router = r
	s.api = chi.NewRouter()
	s.api.NotFound(func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, problem.New(problem.NotFound, "")) })
	s.api.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, problem.New(problem.NotFound, "")) })
	r.Mount(BasePath, s.api)
	return s, nil
}

// Handle registers h for op behind auth → tenant → principal → guards → featuregate → idempotency →
// validation. Patterns under /internal/ are refused (F002-AC-05, ADR-0074: no internal HTTP between
// modules). featuregate (ADR-0090 §3) runs only when Deps.FeatureGate is set and the spec operation has
// x-portal-action; it answers before idempotency so a FEATURE_DISABLED reply is never stored for replay.
func (s *Server) Handle(op Operation, h http.Handler) error {
	if op.ID == "" || op.Method == "" || !strings.HasPrefix(op.Pattern, "/") {
		return fmt.Errorf("httpx: operation needs id, method and a pattern starting with /")
	}
	if strings.HasPrefix(op.Pattern, "/internal/") || strings.Contains(op.Pattern, "/internal/v1/") {
		return fmt.Errorf("httpx: operation %s: /internal/ routes are never served", op.ID)
	}
	if _, dup := s.ops[op.ID]; dup {
		return fmt.Errorf("httpx: operation %s is already mounted", op.ID)
	}
	chain, err := s.chain(op, h)
	if err != nil {
		return err
	}
	s.api.Method(op.Method, op.Pattern, withOperation(op, chain))
	s.ops[op.ID] = op
	return nil
}

// chain builds the middleware chain of an operation (validation, idempotency, feature gate, guards, principal,
// tenant, authentication) around h.
func (s *Server) chain(op Operation, h http.Handler) (http.Handler, error) {
	if op.Public && effectiveGuard(op) != GuardNone {
		return nil, fmt.Errorf("httpx: operation %s: a public operation cannot be guarded", op.ID)
	}
	chain := h
	var gate func(http.Handler) http.Handler
	if s.d.Spec != nil {
		route, err := specRoute(s.d.Spec, op)
		if err != nil {
			return nil, err
		}
		chain = validate(route, chain)
		if s.d.FeatureGate != nil {
			rule, gated, err := s.d.FeatureGate.Bind(route.Operation)
			if err != nil {
				return nil, err
			}
			if gated && op.Public {
				return nil, fmt.Errorf("httpx: operation %s: a public operation cannot carry x-portal-action", op.ID)
			}
			if gated {
				gate = s.d.FeatureGate.Middleware(rule, WriteError)
			}
		}
	}
	if op.PreValidate != nil {
		if !op.Public {
			return nil, fmt.Errorf("httpx: operation %s: PreValidate is for public operations only", op.ID)
		}
		chain = op.PreValidate(chain)
	}
	if op.Idempotency != IdempotencyNone {
		store := s.d.Idempotency
		if st, ok := s.d.IdempotencyStores[op.Module]; ok {
			store = st
		}
		if store == nil || op.Public {
			return nil, fmt.Errorf("httpx: operation %s is idempotent but the server has no idempotency store (or is public)", op.ID)
		}
		chain = Idempotency(store, op)(chain)
	}
	if gate != nil {
		chain = gate(chain)
	}
	if !op.Public {
		chain = s.guards(effectiveGuard(op), chain)
		chain = s.principal(op, chain)
	}
	chain = s.tenant(op.Tenant, chain)
	if !op.Public {
		chain = s.authenticate(chain)
	}
	return chain, nil
}

// routedOp is a shared operation (several modules serve one path, x-service-routing): each module chain serves the
// values of a path parameter; other values answer 404 NOT_FOUND after authentication.
type routedOp struct {
	op     Operation
	param  string
	chains map[string]http.Handler
}

// HandleRouted registers h for the values of a path parameter of a shared operation (e.g. /game-admin/{config_type},
// served by modules game, competition and chat): the first registration mounts the route, the next ones add their
// values. Each module keeps its own chain (idempotency store, metric label, caller module); a value nobody serves is
// 404 NOT_FOUND once the request is authenticated and guarded.
func (s *Server) HandleRouted(op Operation, param string, values []string, h http.Handler) error {
	if param == "" || len(values) == 0 {
		return fmt.Errorf("httpx: routed operation %s needs a parameter and values", op.ID)
	}
	r, mounted := s.routed[op.ID]
	if !mounted {
		if _, dup := s.ops[op.ID]; dup {
			return fmt.Errorf("httpx: operation %s is already mounted", op.ID)
		}
		if strings.HasPrefix(op.Pattern, "/internal/") || !strings.HasPrefix(op.Pattern, "/") || op.Public {
			return fmt.Errorf("httpx: routed operation %s needs a guarded public-API pattern", op.ID)
		}
		fb, err := s.chain(op, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			WriteError(w, req, problem.New(problem.NotFound, ""))
		}))
		if err != nil {
			return err
		}
		r = &routedOp{op: op, param: param, chains: map[string]http.Handler{}}
		s.api.Method(op.Method, op.Pattern, r.dispatch(withOperation(op, fb)))
		s.ops[op.ID] = op
		s.routed[op.ID] = r
	}
	if r.param != param || r.op.Method != op.Method || r.op.Pattern != op.Pattern {
		return fmt.Errorf("httpx: routed operation %s is mounted as %s %s by %s", op.ID, r.op.Method, r.op.Pattern, r.param)
	}
	chain, err := s.chain(op, h)
	if err != nil {
		return err
	}
	for _, v := range values {
		if _, dup := r.chains[v]; dup {
			return fmt.Errorf("httpx: routed operation %s: %s=%s is already served", op.ID, param, v)
		}
		r.chains[v] = withOperation(op, chain)
	}
	return nil
}

// dispatch serves a routed operation: the chain of the parameter value, else the fallback chain (404 after auth).
func (r *routedOp) dispatch(fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if c, ok := r.chains[chi.URLParam(req, r.param)]; ok {
			c.ServeHTTP(w, req)
			return
		}
		fallback.ServeHTTP(w, req)
	})
}

// Operations returns the mounted operations sorted by ID (composition-root wiring tests).
func (s *Server) Operations() []Operation {
	out := make([]Operation, 0, len(s.ops))
	for _, o := range s.ops {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.router.ServeHTTP(w, r) }

type opKey struct{}

func withOperation(op Operation, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observability.InfoFrom(r.Context()).SetRoute(BasePath+op.Pattern, op.ID, op.Module)
		ctx := context.WithValue(r.Context(), opKey{}, op)
		if op.Module != "" {
			ctx = tenancy.WithModule(ctx, op.Module) // caller module of facade calls and metric label
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OperationFrom returns the operation of the request (route label of metrics, logs).
func OperationFrom(ctx context.Context) (Operation, bool) {
	op, ok := ctx.Value(opKey{}).(Operation)
	return op, ok
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		raw, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || strings.TrimSpace(raw) == "" {
			WriteError(w, r, unauthenticated("missing bearer token"))
			return
		}
		c, err := s.verifier.Verify(r.Context(), strings.TrimSpace(raw))
		if err != nil {
			WriteError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(authz.WithClaims(r.Context(), c)))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if e, ok := v.(error); ok && errors.Is(e, http.ErrAbortHandler) {
					panic(v)
				}
				s.log.ErrorContext(r.Context(), "panic", "panic", fmt.Sprint(v), "stack", string(debug.Stack()))
				// eduworld_panics_recovered_total + GlitchTip event (module of the matched route, F131-AC-17)
				observability.RecoveredPanic(r.Context(), "", observability.EntrypointHTTP, v)
				WriteError(w, r, observability.MarkReported(errors.New("panic")))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// WriteError writes RFC 9457 Problem Details with the trace id of the request; unknown errors become
// 500 INTERNAL without internal detail (F002-AC-20) and are reported to error tracking as unexpected 5xx
// (F131-AC-17) unless already reported as a recovered panic.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	code := problem.From(err, "").Code
	observability.InfoFrom(r.Context()).SetCode(string(code))
	if code == problem.Internal {
		observability.ReportUnexpected(r.Context(), "", observability.EntrypointHTTP, err)
	}
	problem.Write(w, r, err, TraceID(r.Context()))
}

// TraceID returns the W3C trace id of ctx or "".
func TraceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}
