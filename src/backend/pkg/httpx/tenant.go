package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// TenantHeader is the tenant header of the public API (ADR-0065).
const TenantHeader = "X-Tenant-Id"

// maxTenantIDs bounds the tenant_ids parameter of System Admin cross-tenant reads.
const maxTenantIDs = 100

var tenantMismatch = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "eduworld_auth_tenant_mismatch_total",
	Help: "Requests refused with TENANT_MISMATCH (X-Tenant-Id outside the user's memberships).",
})

// TenantMismatchCounter exposes eduworld_auth_tenant_mismatch_total (tests).
func TenantMismatchCounter() prometheus.Counter { return tenantMismatch }

type headerTenantKey struct{}

// HeaderTenant returns the parsed X-Tenant-Id of a request. Authenticated handlers must use
// Principal.TenantID instead (F002-AC-08); this is for public operations such as token refresh.
func HeaderTenant(ctx context.Context) (uuid.UUID, bool) {
	t, ok := ctx.Value(headerTenantKey{}).(uuid.UUID)
	return t, ok
}

func (s *Server) tenant(mode TenantMode, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mode == TenantNone {
			next.ServeHTTP(w, r)
			return
		}
		raw := strings.TrimSpace(r.Header.Get(TenantHeader))
		if raw == "" {
			if mode == TenantRequired {
				WriteError(w, r, problem.New(problem.TenantHeaderRequired, ""))
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || len(raw) != 36 {
			WriteError(w, r, problem.New(problem.BadRequest, "X-Tenant-Id must be a UUID"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), headerTenantKey{}, id)))
	})
}

// principal resolves the Principal once per request, checks the tenant against memberships and puts the
// Principal and its data scope in the context (F002-AC-07, F002-AC-08, F002-AC-09).
func (s *Server) principal(op Operation, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, ok := authz.ClaimsFrom(ctx)
		if !ok {
			WriteError(w, r, unauthenticated(""))
			return
		}
		if len(c.Organizations) > 1 {
			// A token of several organizations never selects a tenant by itself (F013-AC-25, ADR-0063).
			authz.RecordDenied(ctx, string(problem.TenantNotSelected), map[string]any{"operation": op.ID})
			WriteError(w, r, problem.New(problem.TenantNotSelected, ""))
			return
		}
		var hdr *uuid.UUID
		if t, ok := HeaderTenant(ctx); ok {
			hdr = &t
		}
		p, err := s.d.Resolver.Resolve(ctx, authz.ResolveRequest{Subject: c.SubjectRef(), TenantID: hdr, ClientID: c.AZP})
		if errors.Is(err, authz.ErrNotMember) || (err == nil && hdr != nil && (p.TenantID == nil || *p.TenantID != *hdr)) {
			tenantMismatch.Inc()
			authz.RecordDenied(ctx, string(problem.TenantMismatch), map[string]any{"operation": op.ID})
			WriteError(w, r, problem.New(problem.TenantMismatch, ""))
			return
		}
		if err != nil {
			var pe *problem.Error
			if !errors.As(err, &pe) {
				err = problem.New(problem.Unavailable, "identity unavailable").Retry().Wrap(err)
			}
			WriteError(w, r, err)
			return
		}
		d := p.PrincipalData
		d.ClientID, d.SessionID = c.AZP, c.SessionID
		d.Channel, d.MFA = s.verifier.channel(c.AZP), s.verifier.mfa(c)
		p = authz.NewPrincipal(d)

		scope, err := s.scopeFor(r, p)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		tenant := ""
		if p.TenantID != nil {
			tenant = p.TenantID.String()
		}
		observability.InfoFrom(ctx).SetPrincipal(p.UserID.String(), tenant)
		ctx = authz.WithPrincipal(ctx, p)
		if scope != nil {
			ctx = tenancy.WithScope(ctx, scope)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// scopeFor: tenant principal → TenantScope (tenant_ids ignored); platform principal with tenant_ids and
// platform.cross_tenant.read → CrossTenantScope (MFA required); other platform staff → PlatformScope.
func (s *Server) scopeFor(r *http.Request, p authz.Principal) (tenancy.Scope, error) {
	if p.TenantID != nil {
		sc, err := authz.ScopeFor(p)
		if err != nil {
			return nil, problem.New(problem.Internal, "").Wrap(err)
		}
		return sc, nil
	}
	if p.UserType != authz.UserTypeStaff {
		return nil, nil // students always have a tenant; nothing to scope
	}
	raw := r.URL.Query().Get("tenant_ids")
	if raw != "" && p.Has(authz.PermPlatformCrossTenantRead) {
		if !p.MFA {
			return nil, problem.New(problem.MfaRequired, "")
		}
		parts := strings.Split(raw, ",")
		if len(parts) > maxTenantIDs {
			return nil, problem.Invalid("tenant_ids", problem.SubInvalid, "too many tenant ids")
		}
		ids := make([]uuid.UUID, 0, len(parts))
		for _, part := range parts {
			id, err := uuid.Parse(strings.TrimSpace(part))
			if err != nil {
				return nil, problem.Invalid("tenant_ids", problem.SubInvalid, "tenant_ids must be UUIDs")
			}
			ids = append(ids, id)
		}
		sc, err := authz.CrossTenantScope(p, ids, tenancy.ReasonSystemAdminCrossTenant)
		if err != nil {
			authz.RecordDenied(r.Context(), string(problem.PermissionDenied), map[string]any{"why": "cross_tenant_scope"})
			return nil, problem.New(problem.PermissionDenied, "")
		}
		return sc, nil
	}
	if !p.IsPlatform() {
		return nil, nil
	}
	sc, err := authz.PlatformScope(p)
	if err != nil {
		return nil, problem.New(problem.Internal, "").Wrap(err)
	}
	return sc, nil
}
