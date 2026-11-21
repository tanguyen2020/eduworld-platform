package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// CONFIG_DEFAULTS of the Party Lobby and the party sessions (owner: PARTY_SYSTEM §9, §10; MULTIPLAYER_GAMEPLAY §9.2;
// WORLD_DESIGN for private_instance_capacity).
const (
	DefaultLobbyCmdRate        = 5                // party_lobby_cmd_rate: 5 lobby commands per second
	DefaultPrivateCapacity     = 8                // private_instance_capacity (≤ 8)
	DefaultLeaderGrace         = time.Minute      // party_leader_grace: 60 s
	DefaultDormantTimeout      = 30 * time.Minute // party_dormant_timeout: 30 min
	DefaultSessionGrace        = 2 * time.Minute  // disconnect_grace of private_instance (quest co-op) and boss: 120 s
	DefaultMinMembersTimeout   = 2 * time.Minute  // party_activity_min_members_timeout: 120 s
	DefaultSweepBatch          = 1000             // parties per sweep cycle
	maxPrivateInstanceCapacity = party.MaxMembers
)

// LobbyFrom reads the F101 configuration (T037): EW_PARTY_LOBBY_CMD_RATE, EW_PRIVATE_INSTANCE_CAPACITY,
// EW_PARTY_LEADER_GRACE, EW_PARTY_DORMANT_TIMEOUT, EW_PARTY_SESSION_DISCONNECT_GRACE,
// EW_PARTY_ACTIVITY_MIN_MEMBERS_TIMEOUT and EW_PARTY_SWEEP_BATCH; the defaults are the CONFIG_DEFAULTS values.
func LobbyFrom(c *config.Config) (command.LobbyConfig, error) {
	l := command.LobbyConfig{
		CmdPerSecond:      c.Int("EW_PARTY_LOBBY_CMD_RATE", DefaultLobbyCmdRate),
		PrivateCapacity:   c.Int("EW_PRIVATE_INSTANCE_CAPACITY", DefaultPrivateCapacity),
		LeaderGrace:       c.Duration("EW_PARTY_LEADER_GRACE", DefaultLeaderGrace),
		DormantTimeout:    c.Duration("EW_PARTY_DORMANT_TIMEOUT", DefaultDormantTimeout),
		MemberGrace:       c.Duration("EW_PARTY_SESSION_DISCONNECT_GRACE", DefaultSessionGrace),
		MinMembersTimeout: c.Duration("EW_PARTY_ACTIVITY_MIN_MEMBERS_TIMEOUT", DefaultMinMembersTimeout),
		SweepBatch:        c.Int("EW_PARTY_SWEEP_BATCH", DefaultSweepBatch),
	}
	return l, ValidateLobby(l)
}

// ValidateLobby checks the ranges.
func ValidateLobby(l command.LobbyConfig) error {
	var bad []string
	if l.CmdPerSecond < 1 {
		bad = append(bad, "EW_PARTY_LOBBY_CMD_RATE")
	}
	if l.PrivateCapacity < party.MinMembers || l.PrivateCapacity > maxPrivateInstanceCapacity {
		bad = append(bad, "EW_PRIVATE_INSTANCE_CAPACITY")
	}
	if l.LeaderGrace < 5*time.Second || l.DormantTimeout < l.LeaderGrace || l.MemberGrace < 5*time.Second || l.MinMembersTimeout < 5*time.Second {
		bad = append(bad, "EW_PARTY_LEADER_GRACE/DORMANT_TIMEOUT/SESSION_DISCONNECT_GRACE/ACTIVITY_MIN_MEMBERS_TIMEOUT")
	}
	if l.SweepBatch < 1 {
		bad = append(bad, "EW_PARTY_SWEEP_BATCH")
	}
	if len(bad) > 0 {
		return fmt.Errorf("competition lobby config: invalid %s", strings.Join(bad, ", "))
	}
	return nil
}
