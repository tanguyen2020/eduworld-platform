// Package config reads the configuration of the realtime gateway (F090 T001) from EW_* variables; every default is
// a parameter of CONFIG_DEFAULTS (ws_*, resume_window, nats_request_timeout, presence_ttl), nothing is hard-coded
// (AGENTS R3, R33). Malformed values are reported by name, never by value.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS (owner REALTIME_ARCHITECTURE §3, §8, §11).
const (
	DefaultMaxConnections = 4000             // ws_per_gateway_node
	DefaultAcceptRate     = 200              // ws_accept_rate (new connections per second and node)
	DefaultHeartbeat      = 15 * time.Second // ws_heartbeat
	DefaultTimeout        = 45 * time.Second // ws_timeout
	DefaultPing           = 20 * time.Second // ws_ping
	DefaultMaxFrame       = 16 * 1024        // ws_max_frame
	DefaultResumeWindow   = 120 * time.Second
	DefaultResumeMax      = 500 // resume_window: ≤ 500 messages
	DefaultCmdTimeout     = 2 * time.Second
	DefaultFrameRate      = 30 // ws_frame_rate: 30/s, burst 60; hub.player.move 12/s; > 3 violations/min → 4429
	DefaultFrameBurst     = 60
	DefaultMoveRate       = 12
	ClientMoveHz          = 10 // hub.player.move sent by the client ≤ 10 Hz (REALTIME_ARCHITECTURE §6), welcome.limits
	DefaultRateViolations = 3
	DefaultProtocolErrors = 5 // [Proposed] repeated protocol errors per minute before 4400 (F090 assumption A-4)
	DefaultInflight       = 8 // ws_inflight_commands
	DefaultSendQueue      = 256
	DefaultSendStall      = 5 * time.Second  // ws_send_queue: a non-droppable message stuck 5 s → 4408
	DefaultWriteTimeout   = 10 * time.Second // ws_send_queue: write deadline
	DefaultDrainWindow    = 15 * time.Second // ≤ 30 s (F090-AC-27); within EW_HTTP_SHUTDOWN_TIMEOUT
	DefaultPresenceTTL    = 90 * time.Second // presence_ttl: TTL of ew:gateway:rt:conn:<user_id>
)

// Config is the gateway configuration.
type Config struct {
	NodeID         string
	MaxConnections int
	AcceptRate     int
	Heartbeat      time.Duration
	Timeout        time.Duration
	Ping           time.Duration
	MaxFrame       int
	ResumeWindow   time.Duration
	ResumeMax      int
	CmdTimeout     time.Duration
	FrameRate      int
	FrameBurst     int
	MoveRate       int
	RateViolations int
	ProtocolErrors int
	Inflight       int
	SendQueue      int
	SendStall      time.Duration
	WriteTimeout   time.Duration
	DrainWindow    time.Duration
	PresenceTTL    time.Duration
	// AllowedOrigins is EW_CORS_ALLOWED_ORIGINS: a handshake carrying an Origin header (Web client) must match one
	// of them exactly; the Godot client sends no Origin.
	AllowedOrigins []string
}

// From reads EW_NODE_ID (default: host name), EW_WS_MAX_CONNECTIONS, EW_WS_ACCEPT_RATE, EW_WS_HEARTBEAT,
// EW_WS_TIMEOUT, EW_WS_PING, EW_WS_MAX_FRAME, EW_WS_RESUME_WINDOW, EW_WS_RESUME_MAX, EW_CMD_TIMEOUT,
// EW_WS_FRAME_RATE, EW_WS_FRAME_BURST, EW_WS_MOVE_RATE, EW_WS_RATE_VIOLATIONS, EW_WS_PROTOCOL_ERRORS,
// EW_WS_INFLIGHT_COMMANDS, EW_WS_SEND_QUEUE, EW_WS_SEND_STALL, EW_WS_WRITE_TIMEOUT, EW_WS_DRAIN_WINDOW,
// EW_PRESENCE_TTL and EW_CORS_ALLOWED_ORIGINS.
func From(c *config.Config) (Config, error) {
	node := c.String("EW_NODE_ID")
	if node == "" {
		node, _ = os.Hostname()
	}
	out := Config{
		NodeID:         node,
		MaxConnections: c.Int("EW_WS_MAX_CONNECTIONS", DefaultMaxConnections),
		AcceptRate:     c.Int("EW_WS_ACCEPT_RATE", DefaultAcceptRate),
		Heartbeat:      c.Duration("EW_WS_HEARTBEAT", DefaultHeartbeat),
		Timeout:        c.Duration("EW_WS_TIMEOUT", DefaultTimeout),
		Ping:           c.Duration("EW_WS_PING", DefaultPing),
		MaxFrame:       c.Int("EW_WS_MAX_FRAME", DefaultMaxFrame),
		ResumeWindow:   c.Duration("EW_WS_RESUME_WINDOW", DefaultResumeWindow),
		ResumeMax:      c.Int("EW_WS_RESUME_MAX", DefaultResumeMax),
		CmdTimeout:     c.Duration("EW_CMD_TIMEOUT", DefaultCmdTimeout),
		FrameRate:      c.Int("EW_WS_FRAME_RATE", DefaultFrameRate),
		FrameBurst:     c.Int("EW_WS_FRAME_BURST", DefaultFrameBurst),
		MoveRate:       c.Int("EW_WS_MOVE_RATE", DefaultMoveRate),
		RateViolations: c.Int("EW_WS_RATE_VIOLATIONS", DefaultRateViolations),
		ProtocolErrors: c.Int("EW_WS_PROTOCOL_ERRORS", DefaultProtocolErrors),
		Inflight:       c.Int("EW_WS_INFLIGHT_COMMANDS", DefaultInflight),
		SendQueue:      c.Int("EW_WS_SEND_QUEUE", DefaultSendQueue),
		SendStall:      c.Duration("EW_WS_SEND_STALL", DefaultSendStall),
		WriteTimeout:   c.Duration("EW_WS_WRITE_TIMEOUT", DefaultWriteTimeout),
		DrainWindow:    c.Duration("EW_WS_DRAIN_WINDOW", DefaultDrainWindow),
		PresenceTTL:    c.Duration("EW_PRESENCE_TTL", DefaultPresenceTTL),
		AllowedOrigins: c.List("EW_CORS_ALLOWED_ORIGINS"),
	}
	return out, out.Validate()
}

// Defaults returns the configuration of CONFIG_DEFAULTS for node.
func Defaults(node string) Config {
	c, _ := From(config.FromGetenv(func(k string) string {
		if k == "EW_NODE_ID" {
			return node
		}
		return ""
	}))
	return c
}

// Validate reports values outside their meaningful range.
func (c Config) Validate() error {
	var bad []string
	if c.NodeID == "" || len(c.NodeID) > 64 || strings.ContainsAny(c.NodeID, ".*>: \t") {
		bad = append(bad, "EW_NODE_ID")
	}
	for name, v := range map[string]int{"EW_WS_MAX_CONNECTIONS": c.MaxConnections, "EW_WS_ACCEPT_RATE": c.AcceptRate,
		"EW_WS_MAX_FRAME": c.MaxFrame, "EW_WS_RESUME_MAX": c.ResumeMax, "EW_WS_FRAME_RATE": c.FrameRate,
		"EW_WS_FRAME_BURST": c.FrameBurst, "EW_WS_MOVE_RATE": c.MoveRate, "EW_WS_RATE_VIOLATIONS": c.RateViolations,
		"EW_WS_PROTOCOL_ERRORS": c.ProtocolErrors, "EW_WS_INFLIGHT_COMMANDS": c.Inflight, "EW_WS_SEND_QUEUE": c.SendQueue} {
		if v <= 0 {
			bad = append(bad, name)
		}
	}
	for name, v := range map[string]time.Duration{"EW_WS_HEARTBEAT": c.Heartbeat, "EW_WS_TIMEOUT": c.Timeout, "EW_WS_PING": c.Ping,
		"EW_WS_RESUME_WINDOW": c.ResumeWindow, "EW_CMD_TIMEOUT": c.CmdTimeout, "EW_WS_SEND_STALL": c.SendStall,
		"EW_WS_WRITE_TIMEOUT": c.WriteTimeout, "EW_PRESENCE_TTL": c.PresenceTTL} {
		if v <= 0 {
			bad = append(bad, name)
		}
	}
	if c.DrainWindow < 0 || c.DrainWindow > 30*time.Second {
		bad = append(bad, "EW_WS_DRAIN_WINDOW")
	}
	if len(bad) > 0 {
		return fmt.Errorf("realtime config: invalid %s", strings.Join(sortStrings(bad), ", "))
	}
	if c.Timeout <= c.Heartbeat {
		return errors.New("realtime config: EW_WS_TIMEOUT must be longer than EW_WS_HEARTBEAT")
	}
	return nil
}

func sortStrings(s []string) []string {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s
}
