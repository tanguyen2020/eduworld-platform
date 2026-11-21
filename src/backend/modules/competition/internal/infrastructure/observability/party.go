package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
)

var (
	partyOnce = sync.Once{}

	partyCommands = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_party_command_total",
		Help: "Party commands by result (ok or the problem code)."}, []string{"command", "result"})
	partyInvitations = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_party_invitation_total",
		Help: "Party invitations by status reached (pending = sent)."}, []string{"result"})
	partyPolicyDenied = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_party_policy_denied_total",
		Help: "Party invitations and joins refused by a policy rule (PARTY_POLICY_VIOLATION)."}, []string{"rule"})
)

// RegisterParty registers the party metrics once (F100 T040; nil: process default registry not used).
func RegisterParty(reg prometheus.Registerer) {
	partyOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{partyCommands, partyInvitations, partyPolicyDenied} {
			_ = reg.Register(c)
		}
	})
}

// Party implements port.PartyMetrics (labels never carry ids).
type Party struct{}

var _ port.PartyMetrics = Party{}

// Command implements port.PartyMetrics.
func (Party) Command(command, result string) { partyCommands.WithLabelValues(command, result).Inc() }

// Invitation implements port.PartyMetrics.
func (Party) Invitation(result string) { partyInvitations.WithLabelValues(result).Inc() }

// PolicyDenied implements port.PartyMetrics.
func (Party) PolicyDenied(rule string) { partyPolicyDenied.WithLabelValues(rule).Inc() }

var (
	lobbyOnce = sync.Once{}

	lobbyUpdate = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "eduworld_party_lobby_update_seconds",
		Help:    "Time from a lobby command (ready, activity selection) to its pushes to the members (AC-PTY-003: p95 ≤ 1 s).",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5}})
	sessionStarts = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_party_session_start_total",
		Help: "startPartySession by activity type and result (ok or the problem code)."}, []string{"activity_type", "result"})
	leaderAutoTransfers = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_party_leader_auto_transfer_total",
		Help: "Leaders replaced after party_leader_grace (reason leader_disconnected)."})
	activityEnds = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_party_activity_end_total",
		Help: "Party activities ended by end reason."}, []string{"reason"})
)

// RegisterLobby registers the F101 metrics once (T038).
func RegisterLobby(reg prometheus.Registerer) {
	lobbyOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{lobbyUpdate, sessionStarts, leaderAutoTransfers, activityEnds} {
			_ = reg.Register(c)
		}
	})
}

// Lobby implements port.LobbyMetrics (labels never carry ids).
type Lobby struct{}

var _ port.LobbyMetrics = Lobby{}

// LobbyUpdate implements port.LobbyMetrics.
func (Lobby) LobbyUpdate(d time.Duration) { lobbyUpdate.Observe(d.Seconds()) }

// SessionStart implements port.LobbyMetrics.
func (Lobby) SessionStart(activityType, result string) {
	sessionStarts.WithLabelValues(activityType, result).Inc()
}

// LeaderAutoTransfer implements port.LobbyMetrics.
func (Lobby) LeaderAutoTransfer() { leaderAutoTransfers.Inc() }

// ActivityEnd implements port.LobbyMetrics.
func (Lobby) ActivityEnd(reason string) { activityEnds.WithLabelValues(reason).Inc() }
