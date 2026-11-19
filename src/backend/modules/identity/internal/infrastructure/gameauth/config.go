package gameauth

import (
	"encoding/base64"
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Config is the configuration of the game login (F013 T001). Keys are secrets (Docker secret files through
// X_FILE in staging/production); nothing is hardcoded (AGENTS R3, R4).
type Config struct {
	UseCase command.GameAuthConfig
	// TicketKey seals login tickets (AES-256-GCM): EW_GAME_LOGIN_TICKET_KEY(_FILE), base64 of 32 bytes.
	TicketKey []byte
	// IPHashKey keys the HMAC of login_events.ip_hash: EW_LOGIN_IP_HASH_KEY(_FILE); empty = no IP hash.
	IPHashKey []byte
	// Curfew is platform_curfew (EW_PLATFORM_CURFEW, default 21:30-06:30) until F015 reads platform settings.
	Curfew string
}

// ConfigFrom reads EW_GAME_MIN_CLIENT_VERSION (min_client_version), EW_GAME_LOGIN_TICKET_TTL
// (game_login_ticket_ttl, 2m), EW_GAME_LOGIN_TICKET_KEY, EW_LOGIN_IP_HASH_KEY and EW_PLATFORM_CURFEW. The
// Keycloak client of eduworld-game-backend is read by keycloak.GameConfigFrom. Missing or malformed values
// are reported by name only.
func ConfigFrom(c *config.Config) (Config, error) {
	if err := c.Require("EW_GAME_MIN_CLIENT_VERSION", "EW_GAME_LOGIN_TICKET_KEY"); err != nil {
		return Config{}, err
	}
	minVersion := c.String("EW_GAME_MIN_CLIENT_VERSION")
	if _, err := gamelogin.ParseVersion(minVersion); err != nil {
		return Config{}, errors.New("gameauth: EW_GAME_MIN_CLIENT_VERSION must be major.minor.patch")
	}
	key, err := base64.StdEncoding.DecodeString(c.String("EW_GAME_LOGIN_TICKET_KEY"))
	if err != nil || len(key) != 32 {
		return Config{}, errors.New("gameauth: EW_GAME_LOGIN_TICKET_KEY must be base64 of 32 bytes")
	}
	out := Config{
		UseCase: command.GameAuthConfig{MinClientVersion: minVersion, TicketTTL: c.Duration("EW_GAME_LOGIN_TICKET_TTL", 2*time.Minute),
			CurfewWarningMinutes: c.Int("EW_CURFEW_WARNING_MINUTES", 10), Limits: command.DefaultGameRateLimits()},
		TicketKey: key,
		Curfew:    c.StringOr("EW_PLATFORM_CURFEW", DefaultPlatformCurfew),
	}
	if v := c.String("EW_LOGIN_IP_HASH_KEY"); v != "" {
		out.IPHashKey = []byte(v)
	}
	if _, err := gamelogin.ParseCurfew(out.Curfew, time.UTC); err != nil {
		return Config{}, errors.New("gameauth: EW_PLATFORM_CURFEW must be HH:MM-HH:MM")
	}
	return out, nil
}
