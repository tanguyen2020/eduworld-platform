package observability

import (
	"sync"

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
