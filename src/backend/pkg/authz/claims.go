package authz

import (
	"context"
	"time"
)

// Claims are the verified access-token claims the middleware auth puts in the context (F002 T013).
// They never carry an EduWorld user_id: the principal is resolved from (Issuer, Subject).
type Claims struct {
	Issuer    string
	Subject   string
	AZP       string // authorized party = client id
	SessionID string // sid
	ACR       string
	AMR       []string
	AuthTime  time.Time
	Expiry    time.Time
	// Organizations are the Keycloak Organization aliases of the token (claim organization); none for the
	// clients of the realm today. More than one → 403 TENANT_NOT_SELECTED (IDENTITY_MAPPING §4.1 step 5).
	Organizations []string
}

// SubjectRef returns the identity of the token subject.
func (c Claims) SubjectRef() SubjectRef { return SubjectRef{Issuer: c.Issuer, Subject: c.Subject} }

type claimsKey struct{}

// WithClaims stores verified claims in ctx.
func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// ClaimsFrom returns the verified claims of ctx.
func ClaimsFrom(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(Claims)
	return c, ok && c.Subject != ""
}

// HasPlatformRole reports a grant of a platform role (role_scope_kind = platform).
func (p Principal) HasPlatformRole() bool {
	for _, g := range p.Grants {
		if g.ScopeKind == RoleScopePlatform {
			return true
		}
	}
	return false
}

// RecordDenied audits an authorization denial decided outside Require (guards, tenant mismatch) as
// authz.denied with the problem code as reason. Audit failures never turn a denial into an allow.
func RecordDenied(ctx context.Context, reason string, metadata map[string]any) {
	rec := recorder()
	if rec == nil {
		return
	}
	_ = rec.Record(ctx, deniedEvent(reason, metadata))
}
