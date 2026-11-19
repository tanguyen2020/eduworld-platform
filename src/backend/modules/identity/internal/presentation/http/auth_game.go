package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// GameAuthHandlers serve /auth/game/* (F013, ADR-0068, ADR-0086). Bodies (password, refresh token, login
// ticket) are read into memory only: never logged, traced, audited or stored for idempotency (F013-AC-16).
type GameAuthHandlers struct {
	Session command.GameSessionHandler
	// IPHashKey is the HMAC key of login_events.ip_hash (EW_LOGIN_IP_HASH_KEY); nil = no IP hash.
	IPHashKey []byte
}

func gameAuthOp(id, pattern string, tenant httpx.TenantMode, public bool, status int) httpx.Operation {
	o := httpx.Operation{ID: id, Module: "identity", Method: http.MethodPost, Pattern: pattern, Public: public,
		Tenant: tenant, Idempotency: httpx.IdempotencyNone, SuccessStatus: status}
	if !public {
		o.Guard = httpx.GuardGamePlayer
	}
	return o
}

// GameAuthOperations are the operations of the game login. Login, tenant select and refresh are public (the
// refresh token in the body is the credential); logout needs a student game token (game_player guard,
// ADR-0031). None takes an Idempotency-Key (tokens are never stored for replay).
var GameAuthOperations = map[string]httpx.Operation{
	"gameLogin":             gameAuthOp("gameLogin", "/auth/game/login", httpx.TenantNone, true, http.StatusOK),
	"selectGameLoginTenant": gameAuthOp("selectGameLoginTenant", "/auth/game/tenant:select", httpx.TenantNone, true, http.StatusOK),
	"refreshGameToken":      gameAuthOp("refreshGameToken", "/auth/game/token:refresh", httpx.TenantRequired, true, http.StatusOK),
	"gameLogout":            gameAuthOp("gameLogout", "/auth/game/logout", httpx.TenantRequired, false, http.StatusNoContent),
}

// RegisterGameAuth mounts the game login operations.
func RegisterGameAuth(s *httpx.Server, h GameAuthHandlers) error {
	if h.Session == nil {
		return errors.New("identity: game auth needs the session use cases")
	}
	routes := map[string]http.HandlerFunc{
		"gameLogin":             h.gameLogin,
		"selectGameLoginTenant": h.selectGameLoginTenant,
		"refreshGameToken":      h.refreshGameToken,
		"gameLogout":            h.gameLogout,
	}
	for id, fn := range routes {
		o, ok := GameAuthOperations[id]
		if !ok {
			return problem.New(problem.Internal, "no operation "+id)
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

// ipHash is the HMAC-SHA-256 of the client address (never the raw IP, ADR-0046 §8, AGENTS R23).
func (h GameAuthHandlers) ipHash(r *http.Request) []byte {
	if len(h.IPHashKey) == 0 {
		return nil
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	m := hmac.New(sha256.New, h.IPHashKey)
	_, _ = m.Write([]byte(host))
	return m.Sum(nil)
}

func (h GameAuthHandlers) gameLogin(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username      string    `json:"username"`
		Password      string    `json:"password"`
		DeviceID      uuid.UUID `json:"device_id"`
		ClientVersion string    `json:"client_version"`
		Platform      string    `json:"platform"`
		Locale        string    `json:"locale"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := dto.GameLogin{Username: b.Username, Password: []byte(b.Password), DeviceID: b.DeviceID, ClientVersion: b.ClientVersion,
		Platform: b.Platform, Locale: b.Locale, IPHash: h.ipHash(r)}
	b.Password = ""
	out, err := h.Session.Login(r.Context(), in)
	if err != nil {
		gameFail(w, r, err)
		return
	}
	writeGameLogin(w, out, true)
}

func (h GameAuthHandlers) selectGameLoginTenant(w http.ResponseWriter, r *http.Request) {
	var b struct {
		LoginTicket string    `json:"login_ticket"`
		TenantID    uuid.UUID `json:"tenant_id"`
		DeviceID    uuid.UUID `json:"device_id"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Session.SelectTenant(r.Context(), dto.GameTenantSelect{LoginTicket: b.LoginTicket, TenantID: b.TenantID,
		DeviceID: b.DeviceID, IPHash: h.ipHash(r)})
	if err != nil {
		gameFail(w, r, err)
		return
	}
	writeGameLogin(w, out, true)
}

func (h GameAuthHandlers) refreshGameToken(w http.ResponseWriter, r *http.Request) {
	var b struct {
		RefreshToken  string    `json:"refresh_token"`
		DeviceID      uuid.UUID `json:"device_id"`
		Mode          string    `json:"mode"`
		ClientVersion string    `json:"client_version"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	tenant, _ := httpx.HeaderTenant(r.Context()) // TenantRequired: present and a UUID
	out, err := h.Session.Refresh(r.Context(), dto.GameRefresh{RefreshToken: b.RefreshToken, DeviceID: b.DeviceID, Mode: b.Mode,
		ClientVersion: b.ClientVersion, TenantID: tenant})
	if err != nil {
		gameFail(w, r, err)
		return
	}
	writeGameLogin(w, out, false)
}

func (h GameAuthHandlers) gameLogout(w http.ResponseWriter, r *http.Request) {
	var b struct {
		RefreshToken string `json:"refresh_token"`
	}
	if r.ContentLength != 0 {
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
	}
	if err := h.Session.Logout(r.Context(), b.RefreshToken); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// gameFail writes the problem with Retry-After for 429 and 503.
func gameFail(w http.ResponseWriter, r *http.Request, err error) {
	var ra *command.RetryAfter
	if errors.As(err, &ra) {
		w.Header().Set("Retry-After", ra.Seconds())
	} else if problem.From(err, "").Code == problem.Unavailable {
		w.Header().Set("Retry-After", "5")
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteError(w, r, err)
}

// writeGameLogin writes GameLoginResponse ({tokens, bootstrap}) or GameTokenRefreshResponse (token set
// fields at the top level, bootstrap only for resume).
func writeGameLogin(w http.ResponseWriter, out dto.GameLoginResult, login bool) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if login {
		_ = json.NewEncoder(w).Encode(struct {
			Tokens    dto.GameTokenSet     `json:"tokens"`
			Bootstrap *dto.PlayerBootstrap `json:"bootstrap"`
		}{out.Tokens, out.Bootstrap})
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		dto.GameTokenSet
		Bootstrap *dto.PlayerBootstrap `json:"bootstrap,omitempty"`
	}{out.Tokens, out.Bootstrap})
}
