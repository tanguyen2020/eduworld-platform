package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
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
	GuardNone        Guard = 0
	GuardPortalStaff Guard = 1 << iota // azp = portal client and user_type = staff
	GuardGamePlayer                    // azp = game client and user_type = student (ADR-0031)
	GuardPlatformMFA                   // platform role + acr/auth_time (ADR-0027)
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
		g |= GuardPlatformMFA
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
	// Middlewares run first for every request (request id, OTel, access log — package observability).
	Middlewares []func(http.Handler) http.Handler
}

// Server is the public HTTP server: chi router + per-operation middleware chain.
type Server struct {
	d        Deps
	verifier *Verifier
	router   chi.Router
	api      chi.Router
	log      *slog.Logger
}

// NewServer builds the server; it registers no route of its own (health endpoints live on the admin port).
func NewServer(d Deps) (*Server, error) {
	if d.Resolver == nil {
		return nil, errors.New("httpx: server needs a principal resolver")
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
		if err := d.Registerer.Register(tenantMismatch); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, err
			}
		}
	}
	s := &Server{d: d, verifier: v, log: d.Logger}
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

// Handle registers h for op behind auth → tenant → principal → guards. Patterns under /internal/ are
// refused (F002-AC-05, ADR-0074: no internal HTTP between modules).
func (s *Server) Handle(op Operation, h http.Handler) error {
	if op.ID == "" || op.Method == "" || !strings.HasPrefix(op.Pattern, "/") {
		return fmt.Errorf("httpx: operation needs id, method and a pattern starting with /")
	}
	if strings.HasPrefix(op.Pattern, "/internal/") || strings.Contains(op.Pattern, "/internal/v1/") {
		return fmt.Errorf("httpx: operation %s: /internal/ routes are never served", op.ID)
	}
	if op.Public && effectiveGuard(op) != GuardNone {
		return fmt.Errorf("httpx: operation %s: a public operation cannot be guarded", op.ID)
	}
	chain := h
	if s.d.Spec != nil {
		route, err := specRoute(s.d.Spec, op)
		if err != nil {
			return err
		}
		chain = validate(route, chain)
	}
	if !op.Public {
		chain = s.guards(effectiveGuard(op), chain)
		chain = s.principal(op, chain)
	}
	chain = s.tenant(op.Tenant, chain)
	if !op.Public {
		chain = s.authenticate(chain)
	}
	s.api.Method(op.Method, op.Pattern, withOperation(op, chain))
	return nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.router.ServeHTTP(w, r) }

type opKey struct{}

func withOperation(op Operation, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observability.InfoFrom(r.Context()).SetRoute(BasePath+op.Pattern, op.ID, op.Module)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), opKey{}, op)))
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
				WriteError(w, r, errors.New("panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// WriteError writes RFC 9457 Problem Details with the trace id of the request; unknown errors become
// 500 INTERNAL without internal detail (F002-AC-20).
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	observability.InfoFrom(r.Context()).SetCode(string(problem.From(err, "").Code))
	problem.Write(w, r, err, TraceID(r.Context()))
}

// TraceID returns the W3C trace id of ctx or "".
func TraceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}
