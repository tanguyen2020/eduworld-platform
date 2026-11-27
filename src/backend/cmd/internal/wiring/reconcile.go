package wiring

import (
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
)

// ReconcileCallDeadline is the deadline of one reconciliation facade call: a page holds up to 5000 ids, so the
// reconcile job uses the saga-step value of module_call_deadline (5 s, CONFIG_DEFAULTS) instead of the 800 ms read
// value (F134 Clarifications 2026-11-27); no retry, no circuit breaker (ADR-0074).
var ReconcileCallDeadline = 5 * time.Second

// ReconcilePeers returns the reconcile.Peer of every composed module, called with caller as the calling module (the
// restored module of `worker reconcile --module=<caller>`, checked against x-allowed-callers by pkg/modcall).
func ReconcilePeers(c *Composition, caller string) map[string]reconcile.Peer {
	out := map[string]reconcile.Peer{}
	if c == nil {
		return out
	}
	if c.Identity != nil {
		if f := c.Identity.Facade(); f != nil {
			out["identity"] = identityPeer{caller: caller, f: f}
		}
	}
	if c.Learning != nil {
		if f := c.Learning.Facade(); f != nil {
			out["learning"] = learningPeer{caller: caller, f: f}
		}
	}
	if c.Game != nil {
		if f := c.Game.Facade(); f != nil {
			out["game"] = gamePeer{caller: caller, f: f}
		}
	}
	if c.Competition != nil {
		if f := c.Competition.Facade(); f != nil {
			out["competition"] = competitionPeer{caller: caller, f: f}
		}
	}
	if c.Chat != nil {
		if f := c.Chat.Facade(); f != nil {
			out["chat"] = chatPeer{caller: caller, f: f}
		}
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
