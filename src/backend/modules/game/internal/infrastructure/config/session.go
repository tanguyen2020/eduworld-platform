// Package config reads the configuration of the game session of module game (F014 T003) from EW_* variables and
// secret files (X_FILE); nothing is hard-coded (AGENTS R3, R4). Missing or malformed values are reported by name,
// never by value.
package config

import (
	"errors"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS (owner documents linked there).
const (
	DefaultHeartbeatTimeout = 45 * time.Second // game_session_heartbeat_timeout
	DefaultTimeoutScan      = 15 * time.Second // job game-session-timeout scan interval
	DefaultWarningMinutes   = 10               // curfew_warning_minutes (daily play limit warning, OQ-17)
	DefaultPlatformCurfew   = "21:30-06:30"    // platform_curfew (ADR-0078), Asia/Ho_Chi_Minh
	DefaultLocales          = "vi-VN,en-US"    // supported_locales (ADR-0056)
	CurfewTimeZone          = "Asia/Ho_Chi_Minh"
	DefaultTicketTTL        = 60 * time.Second // realtime_ticket_ttl (ADR-0008)
	DefaultTicketRate       = 10               // ws_ticket_rate per minute and student
)

// Session is the configuration of the game session use cases, client-config and the jobs.
type Session struct {
	UseCase      command.SessionConfig
	ClientConfig query.ClientConfigSource
	// IPHashKey keys the HMAC of game.login_events.ip_hash: EW_GAME_IP_HASH_KEY(_FILE); empty = no IP hash.
	IPHashKey []byte
	// Tickets configures createRealtimeTicket (F090 US1).
	Tickets command.TicketConfig
	// TimeoutScan is the period of the job session-timeout.
	TimeoutScan time.Duration
}

// Location returns Asia/Ho_Chi_Minh (fixed UTC+7 when the time zone database is missing).
func Location() *time.Location {
	loc, err := time.LoadLocation(CurfewTimeZone)
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}

// SessionFrom reads:
//   - EW_GAME_MIN_CLIENT_VERSION (min_client_version, shared with module identity, required),
//     EW_GAME_LATEST_CLIENT_VERSION (default: the minimum);
//   - EW_GAME_SESSION_HEARTBEAT_TIMEOUT (game_session_heartbeat_timeout, 45s), EW_GAME_SESSION_TIMEOUT_SCAN (15s);
//   - EW_GAME_MAINTENANCE (bool), EW_GAME_MAINTENANCE_MESSAGE, EW_GAME_MAINTENANCE_UNTIL (RFC 3339);
//   - EW_PLATFORM_CURFEW (platform_curfew, 21:30-06:30 ICT): only the initial default until the consumer
//     game.platform_settings has projected platform_settings.changed (key platform_curfew) of module identity into
//     ref.platform_settings; from then on the setting of the System Admin applies without a restart (OQ F014-2);
//   - EW_GAME_LOGIN_URL (login.login_url, optional), EW_GAME_REMEMBER_USERNAME_DEFAULT (game_remember_username,
//     true), EW_SUPPORTED_LOCALES (supported_locales);
//   - EW_CURFEW_WARNING_MINUTES (curfew_warning_minutes, 10; shared with module identity): warning before the daily
//     play limit ends a session (OQ-17);
//   - EW_GAME_IP_HASH_KEY(_FILE) (secret; rotated by redeploying a new key);
//   - EW_REALTIME_PUBLIC_URL (url of the realtime gateway returned with each ticket, wss://…/realtime/v1; empty →
//     createRealtimeTicket answers 503 UNAVAILABLE), EW_RT_TICKET_TTL (realtime_ticket_ttl, 60s),
//     EW_WS_TICKET_RATE (ws_ticket_rate, 10 per minute and student) — F090.
func SessionFrom(c *config.Config) (Session, error) {
	if err := c.Require("EW_GAME_MIN_CLIENT_VERSION"); err != nil {
		return Session{}, err
	}
	minVersion := c.String("EW_GAME_MIN_CLIENT_VERSION")
	if _, err := gamesession.ParseVersion(minVersion); err != nil {
		return Session{}, errors.New("game config: EW_GAME_MIN_CLIENT_VERSION must be major.minor.patch")
	}
	latest := c.StringOr("EW_GAME_LATEST_CLIENT_VERSION", minVersion)
	if v, err := gamesession.ParseVersion(latest); err != nil {
		return Session{}, errors.New("game config: EW_GAME_LATEST_CLIENT_VERSION must be major.minor.patch")
	} else if m, _ := gamesession.ParseVersion(minVersion); v.Less(m) {
		return Session{}, errors.New("game config: EW_GAME_LATEST_CLIENT_VERSION is lower than EW_GAME_MIN_CLIENT_VERSION")
	}
	loc := Location()
	curfew, err := gamesession.ParseWindow(c.StringOr("EW_PLATFORM_CURFEW", DefaultPlatformCurfew), loc)
	if err != nil {
		return Session{}, errors.New("game config: EW_PLATFORM_CURFEW must be HH:MM-HH:MM")
	}
	timeout := c.Duration("EW_GAME_SESSION_HEARTBEAT_TIMEOUT", DefaultHeartbeatTimeout)
	if timeout <= 0 {
		return Session{}, errors.New("game config: EW_GAME_SESSION_HEARTBEAT_TIMEOUT must be positive")
	}
	m := command.Maintenance{Active: c.Bool("EW_GAME_MAINTENANCE", false), Message: c.String("EW_GAME_MAINTENANCE_MESSAGE")}
	if u := c.String("EW_GAME_MAINTENANCE_UNTIL"); u != "" {
		at, err := time.Parse(time.RFC3339, u)
		if err != nil {
			return Session{}, errors.New("game config: EW_GAME_MAINTENANCE_UNTIL must be RFC 3339")
		}
		m.Until = &at
	}
	if len([]rune(m.Message)) > 1000 {
		return Session{}, errors.New("game config: EW_GAME_MAINTENANCE_MESSAGE is longer than 1000 characters")
	}
	warning := c.Int("EW_CURFEW_WARNING_MINUTES", DefaultWarningMinutes)
	if warning < 0 || warning > 60 {
		return Session{}, errors.New("game config: EW_CURFEW_WARNING_MINUTES must be 0..60")
	}
	locales := c.List("EW_SUPPORTED_LOCALES")
	if len(locales) == 0 {
		locales = strings.Split(DefaultLocales, ",")
	}
	out := Session{
		UseCase: command.SessionConfig{MinClientVersion: minVersion, HeartbeatTimeout: timeout, Maintenance: m,
			PlatformCurfew: curfew, CurfewLocation: loc, WarningBefore: time.Duration(warning) * time.Minute},
		ClientConfig: query.ClientConfigSource{MinClientVersion: minVersion, LatestClientVersion: latest,
			LoginURL: c.String("EW_GAME_LOGIN_URL"), RememberUsernameDefault: c.Bool("EW_GAME_REMEMBER_USERNAME_DEFAULT", true),
			SupportedLocales: locales, Maintenance: func() query.Maintenance {
				return query.Maintenance{Active: m.Active, Message: m.Message, Until: m.Until}
			}},
		TimeoutScan: c.Duration("EW_GAME_SESSION_TIMEOUT_SCAN", DefaultTimeoutScan),
		Tickets: command.TicketConfig{URL: c.String("EW_REALTIME_PUBLIC_URL"), TTL: c.Duration("EW_RT_TICKET_TTL", DefaultTicketTTL),
			Rate: c.Int("EW_WS_TICKET_RATE", DefaultTicketRate), RateWindow: time.Minute},
	}
	if t := out.Tickets; t.TTL <= 0 || t.TTL > 5*time.Minute || t.Rate <= 0 {
		return Session{}, errors.New("game config: EW_RT_TICKET_TTL must be 1s..5m and EW_WS_TICKET_RATE positive")
	}
	if u := out.Tickets.URL; u != "" && !strings.HasPrefix(u, "ws://") && !strings.HasPrefix(u, "wss://") {
		return Session{}, errors.New("game config: EW_REALTIME_PUBLIC_URL must be a ws:// or wss:// URL")
	}
	if v := c.String("EW_GAME_IP_HASH_KEY"); v != "" {
		if len(v) < 32 {
			return Session{}, errors.New("game config: EW_GAME_IP_HASH_KEY must have at least 32 characters")
		}
		out.IPHashKey = []byte(v)
	}
	return out, nil
}
