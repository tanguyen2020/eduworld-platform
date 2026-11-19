package keycloak

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// GameConfigFrom reads the token client of the confidential client eduworld-game-backend (F013 T001, T006,
// ADR-0068): realm endpoints from EW_KC_ADMIN_BASE_URL, client id EW_KC_GAME_CLIENT_ID, secret
// EW_KC_GAME_CLIENT_SECRET(_FILE) (Docker secret), timeout EW_KC_TOKEN_TIMEOUT (keycloak_token_call_timeout,
// default 3 s). The client is the same token client as the portal (password grant without OTP, rotation,
// logout; circuit breaker; no retry of grants). Missing variables are reported by name only (AGENTS R3).
func GameConfigFrom(c *config.Config) (PortalConfig, error) {
	if err := c.Require("EW_KC_ADMIN_BASE_URL", "EW_KC_GAME_CLIENT_ID", "EW_KC_GAME_CLIENT_SECRET"); err != nil {
		return PortalConfig{}, err
	}
	realm, err := realmURL(strings.TrimRight(c.String("EW_KC_ADMIN_BASE_URL"), "/"))
	if err != nil {
		return PortalConfig{}, err
	}
	ratio := 0.5
	if v := c.String("EW_EXT_CLIENT_CB_FAILURE_RATIO"); v != "" {
		if _, err := fmt.Sscanf(v, "%g", &ratio); err != nil || ratio <= 0 || ratio > 1 {
			return PortalConfig{}, errors.New("keycloak: EW_EXT_CLIENT_CB_FAILURE_RATIO must be in (0,1]")
		}
	}
	return PortalConfig{
		TokenURL: realm + "/protocol/openid-connect/token", LogoutURL: realm + "/protocol/openid-connect/logout",
		ClientID: c.String("EW_KC_GAME_CLIENT_ID"), ClientSecret: c.String("EW_KC_GAME_CLIENT_SECRET"),
		Timeout: c.Duration("EW_KC_TOKEN_TIMEOUT", 3*time.Second), MaxRetries: c.Int("EW_EXT_CLIENT_RETRY_MAX", 2),
		FailureRatio: ratio, Kind: "game",
	}, nil
}

var _ port.KeycloakSessionAdmin = (*Admin)(nil)

// DeleteSession implements port.KeycloakSessionAdmin: DELETE /sessions/{sid} ends one user session (all
// its client sessions, including the refresh tokens of eduworld-game-backend). An unknown or already ended
// session (404) is success; the call is idempotent and retried.
func (a *Admin) DeleteSession(ctx context.Context, sid string) error {
	if sid == "" {
		return nil
	}
	_, err := a.call(ctx, "delete_session", http.MethodDelete, "/sessions/"+url.PathEscape(sid), nil, nil, true)
	if errors.Is(err, port.ErrKCNotFound) {
		return nil
	}
	return err
}

// EnableUser sets enabled=true on the user (unlock, F016-AC-04; idempotent PUT of the representation).
func (a *Admin) EnableUser(ctx context.Context, userID string) error {
	var cur map[string]any
	if _, err := a.call(ctx, "get_user", http.MethodGet, "/users/"+url.PathEscape(userID), nil, &cur, true); err != nil {
		return err
	}
	cur["enabled"] = true
	_, err := a.call(ctx, "enable_user", http.MethodPut, "/users/"+url.PathEscape(userID), cur, nil, true)
	return err
}

// AnonymizeUser disables the user and clears its profile (F016 US5): email, first/last name and attributes.
// The username stays (realm editUsernameAllowed false, OQ F016-10); the user is never deleted.
func (a *Admin) AnonymizeUser(ctx context.Context, userID string) error {
	var cur map[string]any
	if _, err := a.call(ctx, "get_user", http.MethodGet, "/users/"+url.PathEscape(userID), nil, &cur, true); err != nil {
		return err
	}
	cur["enabled"] = false
	cur["email"], cur["firstName"], cur["lastName"] = "", "", ""
	cur["emailVerified"] = false
	cur["attributes"] = map[string]any{}
	_, err := a.call(ctx, "anonymize_user", http.MethodPut, "/users/"+url.PathEscape(userID), cur, nil, true)
	return err
}
