package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// CONFIG_DEFAULTS of the party (owner: PARTY_SYSTEM §3, §10; VALKEY_DESIGN for presence_ttl).
const (
	DefaultPartyMaxMembers   = 4                // party_max_members (2–8)
	DefaultInviteTTL         = 5 * time.Minute  // party_invite_ttl
	DefaultInvitePerMinute   = 10               // party_invite_rate: 10/min
	DefaultInvitePerDay      = 50               // party_invite_rate: 50/day
	DefaultRepeatWait        = 5 * time.Minute  // party_invite_repeat_rules: wait after a rejection
	DefaultRejectsPerDay     = 3                // party_invite_repeat_rules: rejections per day
	DefaultPendingInvitesMax = 20               // party_pending_invites_max
	DefaultCreatePerHour     = 10               // party_create_rate
	DefaultKickPerHour       = 10               // party_kick_rate
	DefaultKickReinvite      = 10 * time.Minute // party_kick_reinvite_block
	DefaultPresenceTTL       = 90 * time.Second // presence_ttl (shared with the gateway)
	DefaultExpiryInterval    = 15 * time.Second // job party-invitation-expiry (plan F100)
)

// Party is the configuration of the F100 party use cases.
type Party struct {
	UseCase        command.PartyConfig
	PresenceTTL    time.Duration
	ExpiryInterval time.Duration
}

// PartyFrom reads the F100 configuration (T003): EW_PARTY_MAX_MEMBERS, EW_PARTY_INVITE_TTL,
// EW_PARTY_INVITE_RATE_PER_MINUTE, EW_PARTY_INVITE_RATE_PER_DAY, EW_PARTY_INVITE_REPEAT_WAIT,
// EW_PARTY_INVITE_REJECTS_PER_DAY, EW_PARTY_PENDING_INVITES_MAX, EW_PARTY_CREATE_RATE_PER_HOUR,
// EW_PARTY_KICK_RATE_PER_HOUR, EW_PARTY_KICK_REINVITE_BLOCK, EW_PRESENCE_TTL and EW_PARTY_EXPIRY_INTERVAL; the defaults
// are the CONFIG_DEFAULTS values.
func PartyFrom(c *config.Config) (Party, error) {
	p := Party{
		UseCase: command.PartyConfig{
			DefaultMaxMembers: c.Int("EW_PARTY_MAX_MEMBERS", DefaultPartyMaxMembers),
			InviteTTL:         c.Duration("EW_PARTY_INVITE_TTL", DefaultInviteTTL),
			InvitePerMinute:   c.Int("EW_PARTY_INVITE_RATE_PER_MINUTE", DefaultInvitePerMinute),
			InvitePerDay:      c.Int("EW_PARTY_INVITE_RATE_PER_DAY", DefaultInvitePerDay),
			RepeatWait:        c.Duration("EW_PARTY_INVITE_REPEAT_WAIT", DefaultRepeatWait),
			RejectsPerDay:     c.Int("EW_PARTY_INVITE_REJECTS_PER_DAY", DefaultRejectsPerDay),
			PendingMax:        c.Int("EW_PARTY_PENDING_INVITES_MAX", DefaultPendingInvitesMax),
			CreatePerHour:     c.Int("EW_PARTY_CREATE_RATE_PER_HOUR", DefaultCreatePerHour),
			KickPerHour:       c.Int("EW_PARTY_KICK_RATE_PER_HOUR", DefaultKickPerHour),
			KickReinviteBlock: c.Duration("EW_PARTY_KICK_REINVITE_BLOCK", DefaultKickReinvite),
		},
		PresenceTTL:    c.Duration("EW_PRESENCE_TTL", DefaultPresenceTTL),
		ExpiryInterval: c.Duration("EW_PARTY_EXPIRY_INTERVAL", DefaultExpiryInterval),
	}
	return p, p.Validate()
}

// Validate checks the ranges.
func (p Party) Validate() error {
	u := p.UseCase
	var bad []string
	if u.DefaultMaxMembers < party.MinMembers || u.DefaultMaxMembers > party.MaxMembers {
		bad = append(bad, "EW_PARTY_MAX_MEMBERS")
	}
	if u.InviteTTL < 10*time.Second || u.InviteTTL > 24*time.Hour {
		bad = append(bad, "EW_PARTY_INVITE_TTL")
	}
	if u.InvitePerMinute < 1 || u.InvitePerDay < u.InvitePerMinute {
		bad = append(bad, "EW_PARTY_INVITE_RATE_PER_MINUTE/PER_DAY")
	}
	if u.RepeatWait < 0 || u.RejectsPerDay < 1 || u.PendingMax < 1 || u.CreatePerHour < 1 || u.KickPerHour < 1 {
		bad = append(bad, "EW_PARTY_* limits")
	}
	if u.KickReinviteBlock < 0 {
		bad = append(bad, "EW_PARTY_KICK_REINVITE_BLOCK")
	}
	if p.PresenceTTL < 10*time.Second {
		bad = append(bad, "EW_PRESENCE_TTL")
	}
	if p.ExpiryInterval < time.Second {
		bad = append(bad, "EW_PARTY_EXPIRY_INTERVAL")
	}
	if len(bad) > 0 {
		return fmt.Errorf("competition party config: invalid %s", strings.Join(bad, ", "))
	}
	return nil
}
