package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/portalauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the game login (plan F013 "Tích hợp và phụ thuộc").
const (
	AuditAccountNotProvisioned      = "auth.account_not_provisioned"
	AuditGameAccessDeniedNonStudent = "auth.game_access_denied_non_student"
	AuditGameTenantMismatch         = "auth.tenant_mismatch"
	AuditGameLogout                 = "auth.game_logout"
	AuditGameSessionRevoked         = "auth.game_session_revoked"
	AuditRefreshTokenReused         = "auth.refresh_token_reused"
	AuditRefreshDeviceMismatch      = "auth.refresh_device_mismatch"
)

// GameClientID is the azp of game tokens and the client of identity.login_events rows.
const GameClientID = "eduworld-game-backend"

// gameAuthJob is the platform scope of session lookups by token hash and of journal rows without tenant.
const gameAuthJob = "game-auth"

// GameRateLimits is login_rate (CONFIG_DEFAULTS): global → 429 LOGIN_QUEUE_BUSY; per device and per hashed
// username → 429 RATE_LIMITED; never per IP (AGENTS R30). RefreshPerSession bounds token:refresh
// (api_rate_default) per Keycloak session.
type GameRateLimits struct {
	Global            Limit
	PerDevice         Limit
	PerUsername       Limit
	RefreshPerSession Limit
}

// DefaultGameRateLimits are the [Proposed] values of login_rate until platform settings are read.
func DefaultGameRateLimits() GameRateLimits {
	return GameRateLimits{Global: Limit{120, time.Second}, PerDevice: Limit{10, time.Minute}, PerUsername: Limit{10, time.Minute},
		RefreshPerSession: Limit{30, time.Minute}}
}

// GameAuthConfig configures the game login use cases (no secret here).
type GameAuthConfig struct {
	MinClientVersion     string        // min_client_version (EW_GAME_MIN_CLIENT_VERSION)
	TicketTTL            time.Duration // game_login_ticket_ttl (2 min)
	CurfewWarningMinutes int           // curfew_warning_minutes (10)
	Limits               GameRateLimits
}

func (c GameAuthConfig) withDefaults() GameAuthConfig {
	if c.TicketTTL <= 0 {
		c.TicketTTL = 2 * time.Minute
	}
	if c.CurfewWarningMinutes <= 0 {
		c.CurfewWarningMinutes = 10
	}
	if c.Limits == (GameRateLimits{}) {
		c.Limits = DefaultGameRateLimits()
	}
	return c
}

// LoginSyncFunc runs login sync (F011 loginsync.LoginSync.Ensure) for the student under the TenantScope on
// ctx and returns the SIS connection of the student (nil without SIS link). It returns 403
// NO_ACTIVE_ENROLLMENT when branch B ends without an effective school/grade (ADR-0095).
type LoginSyncFunc func(ctx context.Context, userID uuid.UUID) (sisConnectionID *uuid.UUID, err error)

// GameSessionHandler is the typed interface of gameLogin, selectGameLoginTenant, refreshGameToken and
// gameLogout (ADR-0087: no mediator).
type GameSessionHandler interface {
	Login(ctx context.Context, in dto.GameLogin) (dto.GameLoginResult, error)
	SelectTenant(ctx context.Context, in dto.GameTenantSelect) (dto.GameLoginResult, error)
	Refresh(ctx context.Context, in dto.GameRefresh) (dto.GameLoginResult, error)
	Logout(ctx context.Context, refreshToken string) error
}

// LoginRestorer restores a student removed by the inactive cleanup when the Keycloak login succeeded
// (F016-AC-17, AC-18; command.StudentRestore).
type LoginRestorer interface {
	RestoreOnLogin(ctx context.Context, userID uuid.UUID) (restored bool, err error)
}

// GameAuthSessionRevoker revokes game login sessions (F013-AC-29): consumers of user.access_revoked,
// user.status_changed and student.cleaned_up, takeover (F014) and password change (F016).
type GameAuthSessionRevoker interface {
	RevokeSessions(ctx context.Context, f write.SessionFilter, reason string) (int, error)
}

// GameAuth implements the game login through the backend (F013, ADR-0068, ADR-0083, ADR-0086). Only
// Keycloak verifies passwords (AGENTS R30); module identity checks the EduWorld side (student, status,
// tenant, enrollment, curfew), never calls module game and keeps only SHA-256 hashes of refresh tokens.
type GameAuth struct {
	UoW         write.UnitOfWork
	Accounts    write.GameLoginRepository
	Memberships write.LoginMembershipReader
	Sessions    write.GameAuthSessionRepository
	Bootstrap   read.BootstrapReader
	Events      write.EventPublisher
	Audit       write.Auditor
	IdP         port.PortalIdP // token client of eduworld-game-backend
	KC          port.KeycloakSessionAdmin
	Tickets     port.LoginTicketStore
	Limiter     port.RateLimiter
	Curfew      port.CurfewSource
	Hours       port.AccessHoursSource    // F015: school/tenant access hours (nil = none)
	DailyLimit  port.DailyPlayLimitSource // OQ-17: daily play limit of the school (nil = none)
	Chat        port.ChatDirectSource     // delta F112-7: policies.chat.direct_message_enabled (nil = omitted)
	Sync        LoginSyncFunc
	Restore     LoginRestorer // F016 US4: restore of a cleaned-up student (nil = ACCOUNT_NOT_PROVISIONED)
	Metrics     port.GameAuthMetrics
	Cfg         GameAuthConfig
	Clock       func() time.Time
}

var (
	_ GameSessionHandler     = GameAuth{}
	_ GameAuthSessionRevoker = GameAuth{}
)

func (h GameAuth) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

func (h GameAuth) loginMetric(result string, start time.Time) {
	if h.Metrics != nil {
		h.Metrics.Login(result, h.now().Sub(start))
	}
}

// record writes one audit event in its own transaction; a failed audit write is logged only.
func (h GameAuth) record(ctx context.Context, e audit.Event) {
	if err := h.Audit.Record(ctx, e); err != nil {
		slog.ErrorContext(ctx, "game auth audit failed", "action", e.Action, "error", err)
	}
}

// revokeKC ends the Keycloak session of a token set that EduWorld refuses (best effort).
func (h GameAuth) revokeKC(ctx context.Context, refreshToken string) {
	if err := h.IdP.Logout(ctx, refreshToken); err != nil {
		slog.WarnContext(ctx, "game auth: revoking the Keycloak session failed", "error", err)
	}
}

// journal appends one identity.login_events row: in the tenant scope when the tenant is known, else in the
// platform scope game-auth (identity_platform, ADR-0089 §2). A failed journal write is logged only.
func (h GameAuth) journal(ctx context.Context, e write.LoginEvent) {
	e.Client, e.OccurredAt = GameClientID, h.now()
	var sc tenancy.Scope
	var err error
	if e.TenantID != nil {
		sc, err = authz.EventScope(*e.TenantID)
	} else {
		sc, err = authz.SystemPlatformScope(gameAuthJob)
	}
	if err == nil {
		err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error { return h.Accounts.RecordLoginEvent(ctx, e) })
	}
	if err != nil {
		slog.ErrorContext(ctx, "game auth: login journal failed", "result", e.Result, "error", err)
	}
}

func (h GameAuth) inPlatform(ctx context.Context, fn func(ctx context.Context) error) error {
	sc, err := authz.SystemPlatformScope(gameAuthJob)
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

func (h GameAuth) account(ctx context.Context, f func(ctx context.Context) (*write.GameAccount, error)) (a *write.GameAccount, err error) {
	err = h.inPlatform(ctx, func(ctx context.Context) error {
		a, err = f(ctx)
		return err
	})
	return a, err
}

// accountCheck applies IDENTITY_MAPPING §4.1 steps 3–4 and 8b to the EduWorld account: none →
// ACCOUNT_NOT_PROVISIONED; staff → GAME_ACCESS_STUDENT_ONLY; locked → ACCOUNT_LOCKED; deactivated or
// anonymized → ACCOUNT_DEACTIVATED.
func accountCheck(a *write.GameAccount) (problem.Code, string) {
	switch {
	case a == nil:
		return problem.AccountNotProvisioned, port.GameLoginNotProvisioned
	case a.UserType != string(authz.UserTypeStudent):
		return problem.GameAccessStudentOnly, port.GameLoginStudentOnly
	case a.Status == "locked":
		return problem.AccountLocked, port.GameLoginLocked
	case a.Status != "active" && a.Status != "pending":
		return problem.AccountDeactivated, port.GameLoginDeactivated
	}
	return "", ""
}

// Login implements gameLogin (F013 US1, US2, US4): validate → client version → rate limit (global queue,
// device, username) → Keycloak password grant → zero the password → account → memberships → login sync →
// tenant decision → curfew → bootstrap, session and journal.
func (h GameAuth) Login(ctx context.Context, in dto.GameLogin) (out dto.GameLoginResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.GameLogin")
	defer func() { done(err) }()
	defer clear(in.Password)
	start := h.now()
	cfg := h.Cfg.withDefaults()
	device := in.DeviceID
	base := write.LoginEvent{DeviceID: &device, IPHash: in.IPHash}
	fail := func(result string, e error) (dto.GameLoginResult, error) {
		h.loginMetric(result, start)
		ev := base
		ev.Result = result
		h.journal(ctx, ev)
		return dto.GameLoginResult{}, e
	}
	username, ok := gamelogin.NormalizeUsername(in.Username)
	if !ok {
		return dto.GameLoginResult{}, problem.Invalid("username", problem.SubInvalid, "username must not be an email")
	}
	if !gamelogin.Supported(in.ClientVersion, cfg.MinClientVersion) {
		return fail(port.GameLoginVersionUnsupported, problem.New(problem.ClientVersionUnsupported, ""))
	}
	if err := h.limitLogin(ctx, cfg, username, in.DeviceID); err != nil {
		result := port.GameLoginRateLimited
		if problem.From(err, "").Code == problem.LoginQueueBusy {
			result = port.GameLoginQueueBusy
		}
		return fail(result, err)
	}
	tok, err := h.IdP.PasswordGrant(ctx, username, in.Password)
	clear(in.Password)
	switch {
	case errors.Is(err, port.ErrPortalInvalidCredentials), errors.Is(err, port.ErrPortalSetupRequired):
		// One status, code and message for every credential failure (F013-AC-17); a pending required action
		// is not a game flow (the school resets the student password).
		return fail(port.GameLoginInvalidCredentials, problem.New(problem.InvalidCredentials, ""))
	case errors.Is(err, port.ErrPortalAccessDenied):
		// The direct grant flow refused a non-student after the password step (KEYCLOAK_SSO §3.1b).
		h.record(ctx, audit.Event{Action: AuditGameAccessDeniedNonStudent, Result: audit.ResultDenied,
			Reason: string(problem.GameAccessStudentOnly), Metadata: map[string]any{"username_hash": portalauth.Key("login", username)}})
		return fail(port.GameLoginStudentOnly, problem.New(problem.GameAccessStudentOnly, ""))
	case err != nil:
		return fail(port.GameLoginUnavailable, unavailable(err))
	}
	acct, err := h.account(ctx, func(ctx context.Context) (*write.GameAccount, error) {
		return h.Accounts.AccountBySubject(ctx, tok.Issuer, tok.Subject)
	})
	if err != nil {
		h.revokeKC(ctx, tok.RefreshToken)
		return fail(port.GameLoginUnavailable, unavailable(err))
	}
	if acct == nil && h.Restore != nil {
		if acct, err = h.restoreOnLogin(ctx, tok.Issuer, tok.Subject); err != nil {
			h.revokeKC(ctx, tok.RefreshToken)
			if problem.From(err, "").Code == problem.AccountDeactivated {
				return fail(port.GameLoginDeactivated, err)
			}
			return fail(port.GameLoginUnavailable, unavailable(err))
		}
	}
	if acct != nil {
		id := acct.UserID
		base.UserID = &id
	}
	if code, result := accountCheck(acct); code != "" {
		h.revokeKC(ctx, tok.RefreshToken)
		h.denyAudit(ctx, code, acct)
		return fail(result, problem.New(code, ""))
	}
	ms, err := h.Memberships.LoginMemberships(ctx, acct.UserID)
	if err != nil {
		h.revokeKC(ctx, tok.RefreshToken)
		return fail(port.GameLoginUnavailable, unavailable(err))
	}
	d := gamelogin.Decide(ms)
	if d.Outcome == gamelogin.NoTenant {
		d, err = h.withoutMembership(ctx, acct)
		if err != nil {
			h.revokeKC(ctx, tok.RefreshToken)
			code := problem.From(err, "").Code
			result := port.GameLoginUnavailable
			switch code {
			case problem.NoActiveEnrollment:
				result = port.GameLoginNoActiveEnrollment
			case problem.AccountNotProvisioned:
				result = port.GameLoginNotProvisioned
				h.denyAudit(ctx, code, acct)
			default:
				err = unavailable(err)
			}
			return fail(result, err)
		}
	}
	if d.Outcome == gamelogin.Choose {
		return h.chooseTenant(ctx, cfg, in, acct, tok, d, base, start)
	}
	return h.complete(ctx, cfg, completion{acct: *acct, tenant: d.Tenant, resolution: d.Resolution, tok: tok,
		deviceID: in.DeviceID, platform: in.Platform, ipHash: in.IPHash, start: start})
}

// limitLogin applies login_rate: the global bucket first (LOGIN_QUEUE_BUSY), then device and username.
func (h GameAuth) limitLogin(ctx context.Context, cfg GameAuthConfig, username string, device uuid.UUID) error {
	if err := limit(ctx, h.Limiter, rateCheck{"game_login:g", cfg.Limits.Global}); err != nil {
		var ra *RetryAfter
		if errors.As(err, &ra) {
			ra.Err = problem.New(problem.LoginQueueBusy, "")
		}
		if h.Metrics != nil {
			h.Metrics.QueueRejected()
		}
		return err
	}
	checks := []struct {
		key  string
		name string
		l    Limit
	}{
		{"game_login:d:" + portalauth.Key("device", device.String()), "device", cfg.Limits.PerDevice},
		{"game_login:u:" + portalauth.Key("login", username), "username", cfg.Limits.PerUsername},
	}
	for _, c := range checks {
		if err := limit(ctx, h.Limiter, rateCheck{c.key, c.l}); err != nil {
			if h.Metrics != nil {
				h.Metrics.RateLimited(c.name)
			}
			return err
		}
	}
	return nil
}

func (h GameAuth) denyAudit(ctx context.Context, code problem.Code, acct *write.GameAccount) {
	action := AuditAccountNotProvisioned
	switch code {
	case problem.AccountNotProvisioned:
	case problem.GameAccessStudentOnly:
		action = AuditGameAccessDeniedNonStudent
	default:
		return // locked/deactivated are recorded by the login journal
	}
	tt, tid := targetUser(uuid.Nil)
	if acct != nil {
		tt, tid = targetUser(acct.UserID)
	}
	h.record(ctx, audit.Event{Action: action, TargetType: tt, TargetID: tid, Result: audit.ResultDenied, Reason: string(code)})
}

// withoutMembership handles a student with no live membership: when the student belongs to a regular
// tenant (primary tenant with a student row), login sync branch B may place the student (school + grade); afterwards
// the memberships are read again. A student row that is not effective (no school/grade in force, ADR-0095) is
// NO_ACTIVE_ENROLLMENT; no student row anywhere is ACCOUNT_NOT_PROVISIONED (never moved to the common tenant, ADR-0063).
func (h GameAuth) withoutMembership(ctx context.Context, acct *write.GameAccount) (gamelogin.Decision, error) {
	if acct.PrimaryTenantID == nil {
		return gamelogin.Decision{}, problem.New(problem.AccountNotProvisioned, "")
	}
	sc, err := authz.EventScope(*acct.PrimaryTenantID)
	if err != nil {
		return gamelogin.Decision{}, err
	}
	tctx := tenancy.WithScope(ctx, sc)
	if h.Sync != nil {
		if _, err := h.Sync(tctx, acct.UserID); err != nil {
			return gamelogin.Decision{}, err
		}
	}
	ms, err := h.Memberships.LoginMemberships(ctx, acct.UserID)
	if err != nil {
		return gamelogin.Decision{}, err
	}
	if d := gamelogin.Decide(ms); d.Outcome != gamelogin.NoTenant {
		return d, nil
	}
	b, err := h.Bootstrap.Bootstrap(tctx, acct.UserID)
	if err != nil {
		return gamelogin.Decision{}, err
	}
	if b.Student != nil && !b.IsCommonTenant {
		return gamelogin.Decision{}, problem.New(problem.NoActiveEnrollment, "")
	}
	return gamelogin.Decision{}, problem.New(problem.AccountNotProvisioned, "")
}

// chooseTenant answers 403 TENANT_NOT_SELECTED with the choices and a one-time login ticket bound to the
// device; the token set waits encrypted in Valkey (research R1). Without Valkey there is no ticket: 503.
func (h GameAuth) chooseTenant(ctx context.Context, cfg GameAuthConfig, in dto.GameLogin, acct *write.GameAccount, tok port.PortalTokens,
	d gamelogin.Decision, base write.LoginEvent, start time.Time) (dto.GameLoginResult, error) {
	ticket, err := portalauth.NewSecret(portalauth.SecretBytes)
	expires := h.now().Add(cfg.TicketTTL)
	choices := make([]dto.GameTenantChoice, 0, len(d.Choices))
	ids := make([]uuid.UUID, 0, len(d.Choices))
	for _, c := range d.Choices {
		choices = append(choices, dto.GameTenantChoice{TenantID: c.TenantID, TenantType: c.TenantType, Name: c.TenantName})
		ids = append(ids, c.TenantID)
	}
	if err == nil {
		err = h.Tickets.Save(ctx, gamelogin.HashToken(ticket), port.LoginTicket{UserID: acct.UserID, ExternalIdentityID: acct.ExternalIdentityID,
			DeviceID: in.DeviceID, Platform: in.Platform, ClientVersion: in.ClientVersion, Choices: ids, Tokens: tok, ExpiresAt: expires})
	}
	ev := base
	if err != nil {
		h.revokeKC(ctx, tok.RefreshToken)
		ev.Result = port.GameLoginUnavailable
		h.journal(ctx, ev)
		h.loginMetric(port.GameLoginUnavailable, start)
		return dto.GameLoginResult{}, unavailable(err)
	}
	ev.Result = port.GameLoginTenantNotSelected
	h.journal(ctx, ev)
	h.loginMetric(port.GameLoginTenantNotSelected, start)
	return dto.GameLoginResult{}, problem.New(problem.TenantNotSelected, "").
		With("choices", choices).With("login_ticket", ticket).With("login_ticket_expires_at", expires.Format(time.RFC3339))
}

// SelectTenant implements selectGameLoginTenant (F013-AC-10, AC-11): the ticket is taken once (GETDEL);
// another device or a tenant outside the choices ends the Keycloak session and issues no token.
func (h GameAuth) SelectTenant(ctx context.Context, in dto.GameTenantSelect) (out dto.GameLoginResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SelectGameLoginTenant")
	defer func() { done(err) }()
	start := h.now()
	cfg := h.Cfg.withDefaults()
	if err := limit(ctx, h.Limiter, rateCheck{"game_select:d:" + portalauth.Key("device", in.DeviceID.String()), cfg.Limits.PerDevice}); err != nil {
		return out, err
	}
	t, err := h.Tickets.Take(ctx, gamelogin.HashToken(in.LoginTicket))
	switch {
	case errors.Is(err, port.ErrLoginTicketInvalid):
		h.loginMetric(port.GameLoginTicketInvalidOutcome, start)
		return out, problem.New(problem.LoginTicketInvalid, "")
	case err != nil:
		return out, unavailable(err)
	}
	if t.DeviceID != in.DeviceID {
		h.revokeKC(ctx, t.Tokens.RefreshToken)
		h.loginMetric(port.GameLoginTicketInvalidOutcome, start)
		return out, problem.New(problem.LoginTicketInvalid, "")
	}
	device, user := in.DeviceID, t.UserID
	base := write.LoginEvent{DeviceID: &device, UserID: &user, IPHash: in.IPHash}
	deny := func(result string, code problem.Code) (dto.GameLoginResult, error) {
		h.revokeKC(ctx, t.Tokens.RefreshToken)
		ev := base
		ev.Result = result
		h.journal(ctx, ev)
		h.loginMetric(result, start)
		return dto.GameLoginResult{}, problem.New(code, "")
	}
	if !slices.Contains(t.Choices, in.TenantID) {
		h.record(ctx, audit.Event{Action: AuditGameTenantMismatch, TargetType: "user", TargetID: t.UserID.String(),
			Result: audit.ResultDenied, Reason: string(problem.TenantMismatch)})
		return deny(port.GameLoginTenantMismatch, problem.TenantMismatch)
	}
	acct, err := h.account(ctx, func(ctx context.Context) (*write.GameAccount, error) { return h.Accounts.AccountByID(ctx, t.UserID) })
	if err != nil {
		h.revokeKC(ctx, t.Tokens.RefreshToken)
		return out, unavailable(err)
	}
	if code, result := accountCheck(acct); code != "" {
		return deny(result, code)
	}
	ms, err := h.Memberships.LoginMemberships(ctx, t.UserID)
	if err != nil {
		h.revokeKC(ctx, t.Tokens.RefreshToken)
		return out, unavailable(err)
	}
	m, ok := gamelogin.Find(ms, in.TenantID)
	if !ok {
		return deny(port.GameLoginTenantMismatch, problem.TenantMismatch)
	}
	acct.ExternalIdentityID = t.ExternalIdentityID
	return h.complete(ctx, cfg, completion{acct: *acct, tenant: m, resolution: gamelogin.ResolutionSelected, tok: t.Tokens,
		deviceID: in.DeviceID, platform: t.Platform, ipHash: in.IPHash, start: start})
}

type completion struct {
	acct       write.GameAccount
	tenant     gamelogin.Membership
	resolution string
	tok        port.PortalTokens
	deviceID   uuid.UUID
	platform   string
	ipHash     []byte
	start      time.Time
}

// complete finishes a login in the chosen tenant: login sync, enrollment, curfew (after SSO and account
// status, F013-AC-32), then in one transaction of the tenant: pending → active (+ user.activated), the
// GameAuthSession (hash of the refresh token) and the journal row (F013-AC-07, T039).
func (h GameAuth) complete(ctx context.Context, cfg GameAuthConfig, c completion) (dto.GameLoginResult, error) {
	tenant := c.tenant.TenantID
	user := c.acct.UserID
	device := c.deviceID
	ev := write.LoginEvent{TenantID: &tenant, UserID: &user, DeviceID: &device, IPHash: c.ipHash, TenantResolution: c.resolution}
	deny := func(result string, e error) (dto.GameLoginResult, error) {
		h.revokeKC(ctx, c.tok.RefreshToken)
		ev.Result = result
		h.journal(ctx, ev)
		h.loginMetric(result, c.start)
		return dto.GameLoginResult{}, e
	}
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return deny(port.GameLoginUnavailable, unavailable(err))
	}
	tctx := tenancy.WithScope(ctx, sc)
	if h.Sync != nil {
		conn, err := h.Sync(tctx, user)
		if problem.From(err, "").Code == problem.NoActiveEnrollment {
			return deny(port.GameLoginNoActiveEnrollment, err)
		}
		if err != nil {
			return deny(port.GameLoginUnavailable, unavailable(err))
		}
		ev.SisConnectionID = conn
	}
	b, err := h.Bootstrap.Bootstrap(tctx, user)
	if err != nil {
		return deny(port.GameLoginUnavailable, unavailable(err))
	}
	if !b.IsCommonTenant && !b.Placed {
		return deny(port.GameLoginNoActiveEnrollment, problem.New(problem.NoActiveEnrollment, ""))
	}
	if err := h.applyHours(ctx, &b); err != nil {
		return deny(port.GameLoginOutsideHours, err)
	}
	if err := h.applyCurfew(ctx, cfg, &b); err != nil {
		return deny(port.GameLoginCurfewActive, err)
	}
	h.applyChat(ctx, &b)
	if err := h.applyDailyLimit(ctx, &b, user); err != nil {
		return deny(port.GameLoginDailyLimitReached, err)
	}
	now := h.now()
	s := &gamelogin.Session{ID: uuid.Must(uuid.NewV7()), TenantID: tenant, UserID: user, KeycloakSID: c.tok.SessionID,
		DeviceID: c.deviceID, Platform: c.platform, CurrentHash: gamelogin.HashToken(c.tok.RefreshToken), CreatedAt: now, LastRefreshedAt: now}
	ev.Result, ev.Client, ev.OccurredAt = port.GameLoginSuccess, GameClientID, now
	err = h.UoW.Do(tctx, func(ctx context.Context) error {
		activated, err := h.Accounts.MarkLogin(ctx, c.acct, now)
		if err != nil {
			return err
		}
		if activated {
			b.User.Status = "active"
			if err := h.Events.Publish(ctx, write.Event{Type: "user.activated", SubjectType: "user", SubjectID: user.String(),
				Payload: map[string]any{"user_id": user, "external_identity_id": c.acct.ExternalIdentityID}}); err != nil {
				return err
			}
		}
		if err := h.Sessions.Create(ctx, s); err != nil {
			return err
		}
		return h.Accounts.RecordLoginEvent(ctx, ev)
	})
	if err != nil {
		h.revokeKC(ctx, c.tok.RefreshToken)
		h.loginMetric(port.GameLoginUnavailable, c.start)
		return dto.GameLoginResult{}, unavailable(err)
	}
	h.loginMetric(port.GameLoginSuccess, c.start)
	return dto.GameLoginResult{Tokens: tokenSet(c.tok), Bootstrap: &b}, nil
}

// applyHours refuses outside the access hours of the school (else the tenant) with 403
// ACCESS_OUTSIDE_ALLOWED_HOURS (IDENTITY_MAPPING §4.1 step 8, F015-AC-18) and shows the window in the bootstrap.
func (h GameAuth) applyHours(ctx context.Context, b *dto.PlayerBootstrap) error {
	if h.Hours == nil {
		return nil
	}
	hours, loc, err := h.Hours.AllowedHours(ctx, b.TenantID, b.SchoolID)
	if err != nil {
		return unavailable(err)
	}
	if hours == nil || len(hours.Windows) == 0 {
		return nil
	}
	if !hours.Contains(h.now(), loc) {
		return problem.New(problem.AccessOutsideAllowedHours, "")
	}
	return nil
}

// applyCurfew sets policies.play_time.curfew and refuses with 403 CURFEW_ACTIVE + reopen_at inside it.
func (h GameAuth) applyCurfew(ctx context.Context, cfg GameAuthConfig, b *dto.PlayerBootstrap) error {
	if h.Curfew == nil {
		return nil
	}
	cf, err := h.Curfew.EffectiveCurfew(ctx, b.TenantID, b.SchoolID)
	if err != nil {
		return unavailable(err)
	}
	if cf.Zero() {
		return nil
	}
	b.Policies.PlayTime = &dto.PlayTimePolicy{Curfew: &dto.CurfewPolicy{Start: hhmm(cf.Start), End: hhmm(cf.End), WarningMinutes: cfg.CurfewWarningMinutes}}
	if on, reopen := cf.Active(h.now()); on {
		return problem.New(problem.CurfewActive, "").With("reopen_at", reopen.UTC().Format(time.RFC3339))
	}
	return nil
}

// applyChat sets policies.chat.direct_message_enabled from the projection of the chat policy levels (delta F112-7). It
// is display only, so an unreadable projection never fails the login: the field is left out (logged) and the client
// shows the button, module chat deciding on every command.
func (h GameAuth) applyChat(ctx context.Context, b *dto.PlayerBootstrap) {
	if h.Chat == nil {
		return
	}
	v, err := h.Chat.DirectMessageEnabled(ctx, b.TenantID, b.SchoolID, b.IsCommonTenant)
	if err != nil {
		slog.WarnContext(ctx, "identity: chat direct policy unavailable for the bootstrap", "error", err)
		return
	}
	if v != nil {
		b.Policies.Chat = &dto.ChatPolicy{DirectMessageEnabled: v}
	}
}

// applyDailyLimit refuses with 403 DAILY_PLAY_LIMIT_REACHED + reopen_at (00:00 ICT the next day) a student whose
// school set daily_play_limit_minutes and who already played that long today (OQ-17, F013-AC-33). The time played
// is the projection of game_session.ended (module game is never called, R30); the common tenant has no limit.
func (h GameAuth) applyDailyLimit(ctx context.Context, b *dto.PlayerBootstrap, user uuid.UUID) error {
	if h.DailyLimit == nil || b.IsCommonTenant {
		return nil
	}
	now := h.now()
	limit, played, loc, err := h.DailyLimit.DailyPlayLimit(ctx, b.TenantID, b.SchoolID, user, now)
	if err != nil {
		return unavailable(err)
	}
	if settings.DailyLimitReached(limit, played) {
		return problem.New(problem.DailyPlayLimitReached, "").With("reopen_at", settings.NextMidnight(now, loc).UTC().Format(time.RFC3339))
	}
	return nil
}

func hhmm(minutes int) string { return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60) }

func tokenSet(t port.PortalTokens) dto.GameTokenSet {
	exp, rexp := t.ExpiresIn, t.RefreshExpiresIn
	if exp < 1 {
		exp = 1
	}
	if rexp < 1 {
		rexp = 1
	}
	return dto.GameTokenSet{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, TokenType: "Bearer", ExpiresIn: exp, RefreshExpiresIn: rexp}
}

// restoreOnLogin runs the restore of a cleaned-up student (F016 US4) and reads the account again; nil when
// the subject is not a cleaned-up student (ACCOUNT_NOT_PROVISIONED follows).
func (h GameAuth) restoreOnLogin(ctx context.Context, issuer, subject string) (*write.GameAccount, error) {
	var id *uuid.UUID
	if err := h.inPlatform(ctx, func(ctx context.Context) (err error) {
		id, err = h.Accounts.CleanedUserBySubject(ctx, issuer, subject)
		return err
	}); err != nil || id == nil {
		return nil, err
	}
	restored, err := h.Restore.RestoreOnLogin(ctx, *id)
	if err != nil || !restored {
		return nil, err
	}
	return h.account(ctx, func(ctx context.Context) (*write.GameAccount, error) {
		return h.Accounts.AccountBySubject(ctx, issuer, subject)
	})
}
