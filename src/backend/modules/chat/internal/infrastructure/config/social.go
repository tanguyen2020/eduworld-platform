// Package config reads the configuration of module chat from the environment (pkg/config), with the defaults of
// CONFIG_DEFAULTS; nothing is hard-coded in the use cases (AGENTS R3, R33).
package config

import (
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS (owner: SOCIAL_SYSTEM §4.1, VALKEY_DESIGN).
const (
	DefaultMaxPending       = 10                 // friend_limits: pending requests
	DefaultMaxPerDay        = 20                 // friend_limits: requests per day
	DefaultMaxFriends       = 100                // friend_limits: friends
	DefaultFriendRequestTTL = 7 * 24 * time.Hour // friend_request_ttl
	DefaultFriendCodeTTL    = 10 * time.Minute   // friend_code_ttl
	DefaultPresenceTTL      = 90 * time.Second   // presence_ttl
	DefaultExpiryInterval   = time.Minute        // job expire-friend-requests
	DefaultTimeZone         = "Asia/Ho_Chi_Minh" // platform day of the per-day limit
)

// Social is the configuration of the F095 use cases.
type Social struct {
	UseCase        command.SocialConfig
	PresenceTTL    time.Duration
	ExpiryInterval time.Duration
}

// SocialFrom reads the F095 configuration (T002): EW_SOCIAL_MAX_PENDING_REQUESTS, EW_SOCIAL_MAX_REQUESTS_PER_DAY,
// EW_SOCIAL_MAX_FRIENDS (friend_limits 10/20/100), EW_SOCIAL_FRIEND_REQUEST_TTL (friend_request_ttl, 7 days),
// EW_SOCIAL_FRIEND_CODE_TTL (friend_code_ttl, 10 min), EW_PRESENCE_TTL (presence_ttl, 90 s, shared with the gateway),
// EW_SOCIAL_EXPIRY_INTERVAL (1 min) and EW_PLATFORM_TIMEZONE (day of the per-day limit).
func SocialFrom(c *config.Config) (Social, error) {
	loc, err := time.LoadLocation(c.StringOr("EW_PLATFORM_TIMEZONE", DefaultTimeZone))
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600) // no tzdata in the image: Asia/Ho_Chi_Minh has no DST
	}
	out := Social{
		UseCase: command.SocialConfig{
			Limits: social.Limits{MaxPending: c.Int("EW_SOCIAL_MAX_PENDING_REQUESTS", DefaultMaxPending),
				MaxPerDay:  c.Int("EW_SOCIAL_MAX_REQUESTS_PER_DAY", DefaultMaxPerDay),
				MaxFriends: c.Int("EW_SOCIAL_MAX_FRIENDS", DefaultMaxFriends), DayLocation: loc},
			RequestTTL: c.Duration("EW_SOCIAL_FRIEND_REQUEST_TTL", DefaultFriendRequestTTL),
			CodeTTL:    c.Duration("EW_SOCIAL_FRIEND_CODE_TTL", DefaultFriendCodeTTL),
		},
		PresenceTTL:    c.Duration("EW_PRESENCE_TTL", DefaultPresenceTTL),
		ExpiryInterval: c.Duration("EW_SOCIAL_EXPIRY_INTERVAL", DefaultExpiryInterval),
	}
	l := out.UseCase.Limits
	switch {
	case l.MaxPending < 1 || l.MaxPerDay < 1 || l.MaxFriends < 1:
		return out, errors.New("chat config: EW_SOCIAL_MAX_* must be ≥ 1")
	case out.UseCase.RequestTTL < time.Hour:
		return out, errors.New("chat config: EW_SOCIAL_FRIEND_REQUEST_TTL must be ≥ 1h")
	case out.UseCase.CodeTTL < time.Minute || out.UseCase.CodeTTL > 24*time.Hour:
		return out, errors.New("chat config: EW_SOCIAL_FRIEND_CODE_TTL must be 1m..24h")
	case out.PresenceTTL < 10*time.Second:
		return out, errors.New("chat config: EW_PRESENCE_TTL must be ≥ 10s")
	case out.ExpiryInterval < time.Second:
		return out, errors.New("chat config: EW_SOCIAL_EXPIRY_INTERVAL must be ≥ 1s")
	}
	return out, nil
}
