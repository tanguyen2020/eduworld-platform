package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
)

// Env returns the trimmed value of a required environment variable (no default in code, AGENTS R3).
func Env(k string) (string, error) {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return "", fmt.Errorf("set %s", k)
	}
	return v, nil
}

// LoadJSON reads a seed file of the repository and validates it when it has a Validate method.
func LoadJSON(path string, v any) error {
	b, err := os.ReadFile(path) //nolint:gosec // seed file of the repository
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if val, ok := v.(interface{ Validate() error }); ok {
		if err := val.Validate(); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// GameClient is the identity of the seed as a game client (GameLoginRequest client_version/platform, ADR-0068).
type GameClient struct {
	Version  string // ≥ EW_GAME_MIN_CLIENT_VERSION
	Platform string // web
}

// GameLogin signs a student in through POST /auth/game/login (module identity, ADR-0068) exactly once and returns the
// session with the headers of the game client; Logout revokes the refresh token (gameLogout). The password is never
// logged.
func GameLogin(ctx context.Context, hc *http.Client, base, username, password string, gc GameClient) (StudentSession, error) {
	device := uuid.NewString()
	var out struct {
		Tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
		Bootstrap struct {
			TenantID string `json:"tenant_id"`
		} `json:"bootstrap"`
	}
	body := map[string]any{"username": username, "password": password, "device_id": device, "client_version": gc.Version,
		"platform": gc.Platform, "locale": "vi-VN"}
	if _, err := Do(ctx, hc, base, "", http.MethodPost, "/auth/game/login", nil, body, &out); err != nil {
		return StudentSession{}, fmt.Errorf("game login %s: %w", username, err)
	}
	if out.Tokens.AccessToken == "" || out.Bootstrap.TenantID == "" {
		return StudentSession{}, fmt.Errorf("game login %s: no token or tenant", username)
	}
	h := map[string]string{"X-Device-Id": device, "X-Client-Version": gc.Version, "X-Client-Platform": gc.Platform}
	s := StudentSession{Token: out.Tokens.AccessToken, TenantID: out.Bootstrap.TenantID, Headers: h}
	refresh := out.Tokens.RefreshToken
	s.Logout = func(ctx context.Context) error {
		lh := map[string]string{"X-Tenant-Id": s.TenantID}
		for k, v := range h {
			lh[k] = v
		}
		_, err := Do(ctx, hc, base, s.Token, http.MethodPost, "/auth/game/logout", lh, map[string]any{"refresh_token": refresh}, nil)
		return err
	}
	return s, nil
}
