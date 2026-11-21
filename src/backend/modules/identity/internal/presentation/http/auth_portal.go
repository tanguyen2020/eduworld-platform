package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Cookies and header of the portal session on the browser (ADR-0091 §4, F012-AC-27, AC-30).
const (
	PortalRefreshCookie = "__Secure-ew_portal_rt"
	PortalCSRFCookie    = "__Host-ew_portal_csrf"
	PortalCSRFHeader    = "X-EW-CSRF"
	portalCookiePath    = httpx.BasePath + "/auth/portal"
)

// PortalAuthHandlers serve /auth/portal/* (ADR-0091). Bodies, cookies and the CSRF header are never logged,
// traced or audited (F012-AC-31): the handlers read them into memory only, and errors carry codes only.
type PortalAuthHandlers struct {
	Session command.PortalSessionHandler
	// Setup serves setupPortalPassword, forgotPortalPassword and resetPortalPassword (F012 US8).
	Setup   command.PortalAccountSetupHandler
	Metrics port.PortalAuthMetrics
}

func portalOp(id, pattern string, status int) httpx.Operation {
	return httpx.Operation{ID: id, Module: "identity", Method: http.MethodPost, Pattern: pattern, Public: true,
		Tenant: httpx.TenantNone, Idempotency: httpx.IdempotencyNone, SuccessStatus: status}
}

// PortalAuthOperations are the public, token-less operations of the portal login (no X-Tenant-Id, no
// Idempotency-Key: ADR-0091 §5); the contract test checks them against the merged spec.
var PortalAuthOperations = map[string]httpx.Operation{
	"portalLogin":          portalOp("portalLogin", "/auth/portal/login", http.StatusOK),
	"refreshPortalToken":   portalOp("refreshPortalToken", "/auth/portal/token:refresh", http.StatusOK),
	"portalLogout":         portalOp("portalLogout", "/auth/portal/logout", http.StatusNoContent),
	"setupPortalPassword":  portalOp("setupPortalPassword", "/auth/portal/setup:password", http.StatusOK),
	"forgotPortalPassword": portalOp("forgotPortalPassword", "/auth/portal/password:forgot", http.StatusAccepted),
	"resetPortalPassword":  portalOp("resetPortalPassword", "/auth/portal/password:reset", http.StatusNoContent),
}

// RegisterPortalAuth mounts the portal login operations. refreshPortalToken and portalLogout check the
// CSRF double-submit before contract validation and before any Keycloak call (F012-AC-30).
func RegisterPortalAuth(s *httpx.Server, h PortalAuthHandlers) error {
	if h.Session == nil || h.Setup == nil {
		return errors.New("identity: portal auth needs the session and setup use cases")
	}
	routes := map[string]http.HandlerFunc{
		"portalLogin":          h.portalLogin,
		"refreshPortalToken":   h.refreshPortalToken,
		"portalLogout":         h.portalLogout,
		"setupPortalPassword":  h.setupPortalPassword,
		"forgotPortalPassword": h.forgotPortalPassword,
		"resetPortalPassword":  h.resetPortalPassword,
	}
	for id, fn := range routes {
		o, ok := PortalAuthOperations[id]
		if !ok {
			return problem.New(problem.Internal, "no operation "+id)
		}
		if id == "refreshPortalToken" || id == "portalLogout" {
			o.PreValidate = h.csrf(id, id == "refreshPortalToken")
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

// csrf checks the double-submit (header X-EW-CSRF = cookie __Host-ew_portal_csrf, constant-time) and, for
// refresh, the presence of the refresh cookie — before validation and before Keycloak (F012-AC-30).
func (h PortalAuthHandlers) csrf(op string, needRefresh bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get(PortalCSRFHeader)
			c, err := r.Cookie(PortalCSRFCookie)
			if err != nil || c.Value == "" || hdr == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(hdr)) != 1 {
				if h.Metrics != nil {
					h.Metrics.CSRFRejected(op)
				}
				httpx.WriteError(w, r, problem.New(problem.CsrfTokenInvalid, ""))
				return
			}
			if needRefresh {
				if rc, err := r.Cookie(PortalRefreshCookie); err != nil || rc.Value == "" {
					clearPortalCookies(w)
					httpx.WriteError(w, r, problem.New(problem.RefreshTokenInvalid, ""))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (h PortalAuthHandlers) portalLogin(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username string    `json:"username"`
		Password string    `json:"password"`
		DeviceID uuid.UUID `json:"device_id"`
		Locale   string    `json:"locale"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := dto.PortalLogin{Username: b.Username, Password: []byte(b.Password), DeviceID: b.DeviceID, Locale: b.Locale}
	b.Password = ""
	out, err := h.Session.Login(r.Context(), in)
	if err != nil {
		portalFail(w, r, err, false)
		return
	}
	writePortalSession(w, out)
}

func (h PortalAuthHandlers) refreshPortalToken(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie(PortalRefreshCookie) // presence checked by csrf
	out, err := h.Session.Refresh(r.Context(), c.Value)
	if err != nil {
		portalFail(w, r, err, true)
		return
	}
	writePortalSession(w, out)
}

func (h PortalAuthHandlers) portalLogout(w http.ResponseWriter, r *http.Request) {
	rt := ""
	if c, err := r.Cookie(PortalRefreshCookie); err == nil {
		rt = c.Value
	}
	if err := h.Session.Logout(r.Context(), rt); err != nil {
		portalFail(w, r, err, true)
		return
	}
	clearPortalCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

// portalFail writes the problem with Retry-After for 429; when the session is over (refresh token invalid,
// account refused) both cookies are cleared.
func portalFail(w http.ResponseWriter, r *http.Request, err error, session bool) {
	var ra *command.RetryAfter
	if errors.As(err, &ra) {
		w.Header().Set("Retry-After", ra.Seconds())
	}
	if session {
		switch problem.From(err, "").Code {
		case problem.RefreshTokenInvalid, problem.AccountDeactivated, problem.PortalAccessStaffOnly:
			clearPortalCookies(w)
		}
	}
	httpx.WriteError(w, r, err)
}

func writePortalSession(w http.ResponseWriter, s dto.PortalSession) {
	setPortalCookies(w, s.RefreshToken, s.CSRF, s.RefreshExpiresIn)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		AccessToken      string `json:"access_token"`
		TokenType        string `json:"token_type"`
		ExpiresIn        int    `json:"expires_in"`
		RefreshExpiresIn int    `json:"refresh_expires_in"`
		AuthTime         string `json:"auth_time"`
	}{s.AccessToken, "Bearer", s.ExpiresIn, s.RefreshExpiresIn, s.AuthTime.UTC().Format(time.RFC3339)})
}

// setPortalCookies writes the refresh cookie (HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/portal)
// and the CSRF cookie (readable by the portal; Secure; SameSite=Strict; Path=/; __Host- so no Domain).
func setPortalCookies(w http.ResponseWriter, refresh, csrf string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: PortalRefreshCookie, Value: refresh, Path: portalCookiePath, MaxAge: maxAge,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: PortalCSRFCookie, Value: csrf, Path: "/", MaxAge: maxAge,
		Secure: true, SameSite: http.SameSiteStrictMode})
}

// clearPortalCookies expires both cookies (Max-Age=0).
func clearPortalCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: PortalRefreshCookie, Value: "", Path: portalCookiePath, MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: PortalCSRFCookie, Value: "", Path: "/", MaxAge: -1,
		Secure: true, SameSite: http.SameSiteStrictMode})
}

func (h PortalAuthHandlers) setupPortalPassword(w http.ResponseWriter, r *http.Request) {
	var b struct {
		SetupTicket string `json:"setup_ticket"`
		NewPassword string `json:"new_password"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := dto.PortalSetupPassword{SetupTicket: b.SetupTicket, NewPassword: []byte(b.NewPassword)}
	b.NewPassword = ""
	out, err := h.Setup.SetupPassword(r.Context(), in)
	if err != nil {
		portalFail(w, r, err, false)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, struct {
		RemainingSteps []string `json:"remaining_steps"`
	}{out.RemainingSteps})
}

func (h PortalAuthHandlers) forgotPortalPassword(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Identifier string    `json:"identifier"`
		DeviceID   uuid.UUID `json:"device_id"`
		Locale     string    `json:"locale"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Setup.ForgotPassword(r.Context(), dto.PortalForgotPassword{Identifier: b.Identifier, DeviceID: b.DeviceID, Locale: b.Locale}); err != nil {
		portalFail(w, r, err, false)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h PortalAuthHandlers) resetPortalPassword(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Token       string    `json:"token"`
		NewPassword string    `json:"new_password"`
		DeviceID    uuid.UUID `json:"device_id"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := dto.PortalResetPassword{Token: b.Token, NewPassword: []byte(b.NewPassword), DeviceID: b.DeviceID}
	b.NewPassword = ""
	if err := h.Setup.ResetPassword(r.Context(), in); err != nil {
		portalFail(w, r, err, false)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
