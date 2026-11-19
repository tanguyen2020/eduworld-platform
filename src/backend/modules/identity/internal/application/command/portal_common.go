package command

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Limit is one fixed-window rate limit (count per window).
type Limit struct {
	N      int
	Window time.Duration
}

// PortalRateLimits is portal_login_rate (CONFIG_DEFAULTS, SECURITY_ARCHITECTURE §14): the second layer
// after APISIX, by hashed username/identifier, device_id and globally — never by IP (AGENTS R30).
type PortalRateLimits struct {
	LoginPerUsername    Limit
	LoginPerDevice      Limit
	LoginGlobal         Limit
	ForgotPerIdentifier Limit
	ForgotPerDevice     Limit
	// SetupPerTicket: setup:password has no device_id in its body, so the limit applies per setup ticket.
	SetupPerTicket Limit
	ResetPerDevice Limit
	// RefreshPerSession: token:refresh has no device_id, so the limit applies per Keycloak session (sid).
	RefreshPerSession Limit
}

// DefaultPortalRateLimits are the [Proposed] values of portal_login_rate (CONFIG_DEFAULTS) until they are
// read from platform settings.
func DefaultPortalRateLimits() PortalRateLimits {
	return PortalRateLimits{
		LoginPerUsername: Limit{10, time.Minute}, LoginPerDevice: Limit{10, time.Minute}, LoginGlobal: Limit{20, time.Second},
		ForgotPerIdentifier: Limit{3, time.Hour}, ForgotPerDevice: Limit{10, time.Hour},
		SetupPerTicket: Limit{10, time.Minute}, ResetPerDevice: Limit{10, time.Minute}, RefreshPerSession: Limit{30, time.Minute},
	}
}

// Defaults of CONFIG_DEFAULTS until platform settings are read (KEYCLOAK_SSO §3.3).
const (
	DefaultPortalSetupTicketTTL   = 10 * time.Minute // portal_setup_ticket_ttl [Confirmed]
	DefaultPortalPasswordResetTTL = 30 * time.Minute // portal_password_reset_ttl [Proposed]
)

// PortalAuthConfig is the configuration of the portal login use cases (no secret here).
type PortalAuthConfig struct {
	// Issuers are the trusted OIDC issuers (EW_OIDC_ISSUERS): a Keycloak user id found through the Admin
	// API is mapped to the EduWorld user through its external identity at one of them.
	Issuers []string
	// MFAAMR are the amr values proving MFA (EW_OIDC_PLATFORM_REQUIRED_AMR, default otp).
	MFAAMR []string
	// MaxAuthAge bounds refreshes of MFA sessions (EW_OIDC_PLATFORM_MAX_AUTH_AGE, portal_session_sso).
	MaxAuthAge     time.Duration
	SetupTicketTTL time.Duration // portal_setup_ticket_ttl
	ResetTTL       time.Duration // portal_password_reset_ttl
	// PortalBaseURL is the origin of the portal for the reset link (EW_PORTAL_BASE_URL).
	PortalBaseURL string
	Limits        PortalRateLimits
}

func (c PortalAuthConfig) withDefaults() PortalAuthConfig {
	if len(c.MFAAMR) == 0 {
		c.MFAAMR = []string{"otp"}
	}
	if c.MaxAuthAge <= 0 {
		c.MaxAuthAge = 8 * time.Hour
	}
	if c.SetupTicketTTL <= 0 {
		c.SetupTicketTTL = DefaultPortalSetupTicketTTL
	}
	if c.ResetTTL <= 0 {
		c.ResetTTL = DefaultPortalPasswordResetTTL
	}
	if c.Limits == (PortalRateLimits{}) {
		c.Limits = DefaultPortalRateLimits()
	}
	return c
}

// RetryAfter is 429 RATE_LIMITED with the delay the presentation sends as Retry-After.
type RetryAfter struct {
	After time.Duration
	Err   *problem.Error
}

func (e *RetryAfter) Error() string { return e.Err.Error() }

// Unwrap exposes the problem error (problem.From finds it).
func (e *RetryAfter) Unwrap() error { return e.Err }

// Seconds is the Retry-After value (at least 1).
func (e *RetryAfter) Seconds() string {
	s := int((e.After + time.Second - 1) / time.Second)
	if s < 1 {
		s = 1
	}
	return strconv.Itoa(s)
}

type rateCheck struct {
	key string
	l   Limit
}

// limit applies the checks in order. The limiter is a second layer after APISIX and Keycloak brute-force
// detection: when Valkey fails it allows the request and logs (no login outage because of a cache).
func limit(ctx context.Context, l port.RateLimiter, checks ...rateCheck) error {
	for _, c := range checks {
		if c.l.N <= 0 || c.l.Window <= 0 {
			continue
		}
		ok, after, err := l.Allow(ctx, "portal_rl:"+c.key, c.l.N, c.l.Window)
		if err != nil {
			slog.WarnContext(ctx, "portal rate limiter unavailable: request allowed", "error", err)
			continue
		}
		if !ok {
			return &RetryAfter{After: after, Err: problem.New(problem.RateLimited, "")}
		}
	}
	return nil
}

// unavailable is 503 UNAVAILABLE (Keycloak, database or Valkey failure); the cause is logged, never sent.
func unavailable(err error) error {
	return problem.New(problem.Unavailable, "").Retry().Wrap(err)
}

// loadAccount runs fn in a platform-scoped transaction of the identity system component portal-auth:
// the role check reads user roles of every tenant and platform roles (explicit conditions, AGENTS R27).
func loadAccount(ctx context.Context, uow write.UnitOfWork, fn func(ctx context.Context) (*write.PortalAccount, error)) (acct *write.PortalAccount, err error) {
	sc, err := authz.SystemPlatformScope("portal-auth")
	if err != nil {
		return nil, err
	}
	err = uow.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		acct, err = fn(ctx)
		return err
	})
	return acct, err
}
