// Package config reads the hub parameters of module competition (F091 T003). Defaults are CONFIG_DEFAULTS values;
// an environment variable may only override them for an environment (load tests), never hold business content.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// CONFIG_DEFAULTS of the hub (owners: WORLD_DESIGN, MULTIPLAYER_GAMEPLAY §9.2, REALTIME_ARCHITECTURE §6, CHARACTER_SYSTEM).
const (
	DefaultTickHz    = 10               // hub_sync_rate
	DefaultCapacity  = 40               // hub_instance_capacity
	DefaultIdleClose = 5 * time.Minute  // hub_instance_idle_close
	DefaultGrace     = 30 * time.Second // disconnect_grace (hub)
	DefaultEmoteRate = 2 * time.Second  // emote_rate: 1 emote / 2 s
	// DefaultSilence: no input at all for this long (the client sends ≥ 1 Hz keepalive) holds the seat like a lost
	// connection when the presence signal was lost [Proposed F091 A-4].
	DefaultSilence    = 10 * time.Second
	DefaultLeaseTTL   = 10 * time.Second // REALTIME_ARCHITECTURE §5.1: lease EX 10, renewed every 3 s
	DefaultLeaseEvery = 3 * time.Second
	DefaultWatchEvery = 2 * time.Second
)

// Hub is the configuration of the hub room host.
type Hub struct {
	InstanceID string
	TickHz     int
	Capacity   int
	IdleClose  time.Duration
	Grace      time.Duration
	EmoteRate  time.Duration
	Silence    time.Duration
	LeaseTTL   time.Duration
	LeaseEvery time.Duration
	WatchEvery time.Duration
}

// HubFrom reads EW_INSTANCE_ID (default: host name), EW_HUB_TICK_HZ, EW_HUB_INSTANCE_CAPACITY, EW_HUB_IDLE_CLOSE,
// EW_HUB_DISCONNECT_GRACE.
func HubFrom(c *config.Config) (Hub, error) {
	id := c.String("EW_INSTANCE_ID")
	if id == "" {
		id, _ = os.Hostname()
	}
	h := Hub{InstanceID: id, TickHz: c.Int("EW_HUB_TICK_HZ", DefaultTickHz), Capacity: c.Int("EW_HUB_INSTANCE_CAPACITY", DefaultCapacity),
		IdleClose: c.Duration("EW_HUB_IDLE_CLOSE", DefaultIdleClose), Grace: c.Duration("EW_HUB_DISCONNECT_GRACE", DefaultGrace),
		EmoteRate: DefaultEmoteRate, Silence: DefaultSilence, LeaseTTL: DefaultLeaseTTL, LeaseEvery: DefaultLeaseEvery,
		WatchEvery: DefaultWatchEvery}
	return h, h.Validate()
}

// Validate checks the ranges.
func (h Hub) Validate() error {
	var bad []string
	if h.InstanceID == "" || len(h.InstanceID) > 64 || strings.ContainsAny(h.InstanceID, " \t") {
		bad = append(bad, "EW_INSTANCE_ID")
	}
	if h.TickHz < 1 || h.TickHz > 30 {
		bad = append(bad, "EW_HUB_TICK_HZ")
	}
	if h.Capacity < 1 || h.Capacity > 40 {
		bad = append(bad, "EW_HUB_INSTANCE_CAPACITY")
	}
	if h.IdleClose <= 0 {
		bad = append(bad, "EW_HUB_IDLE_CLOSE")
	}
	if h.Grace <= 0 {
		bad = append(bad, "EW_HUB_DISCONNECT_GRACE")
	}
	if len(bad) > 0 {
		return fmt.Errorf("competition hub config: invalid %s", strings.Join(bad, ", "))
	}
	return nil
}
