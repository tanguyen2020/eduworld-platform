package observability

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// Metrics of the boss battles and co-op quests (F103 T039, plan "Quan sát"); the room host metrics are pkg/roomhost
// (rooms per type boss / party_quest, takeovers, timer lag). No label carries a student, an answer or a score (R14).
var (
	bossStarted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_boss_encounters_started_total", Help: "Boss encounters created by mode (solo, party) (F103).",
	}, []string{"mode"})
	bossAnswers = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_boss_answer_total", Help: "Boss answers by result (correct, wrong, not_counted, duplicate, error code) (F103).",
	}, []string{"result"})
	bossRetries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_boss_phase_retry_total", Help: "Boss phases started again with new questions after their deadline (F103).",
	})
	bossEnded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_boss_encounter_end_total", Help: "Boss encounters ended by outcome (defeated, time_up, abandoned) (F103).",
	}, []string{"outcome"})
	partyStepShared = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_party_quest_step_shared_total", Help: "Frame steps of co-op quests shared with the party by policy (F103).",
	}, []string{"policy"})
	partySessionEnded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_party_quest_session_end_total", Help: "Shared quest sessions ended by outcome (completed, failed, abandoned) (F103).",
	}, []string{"outcome"})
)

// RegisterBoss registers the F103 metrics.
func RegisterBoss(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{bossStarted, bossAnswers, bossRetries, bossEnded, partyStepShared, partySessionEnded} {
		_ = r.Register(c)
	}
}

// BossMetrics implements command.BossMetrics and command.PartyQuestMetrics.
type BossMetrics struct{}

var (
	_ command.BossMetrics       = BossMetrics{}
	_ command.PartyQuestMetrics = BossMetrics{}
)

// Answer counts one boss answer.
func (BossMetrics) Answer(result string) { bossAnswers.WithLabelValues(result).Inc() }

// PhaseRetry counts one phase retry.
func (BossMetrics) PhaseRetry() { bossRetries.Inc() }

// Ended counts one encounter ended.
func (BossMetrics) Ended(outcome string) { bossEnded.WithLabelValues(outcome).Inc() }

// Started counts one encounter created.
func (BossMetrics) Started(mode string) { bossStarted.WithLabelValues(mode).Inc() }

// StepShared counts one frame step shared.
func (BossMetrics) StepShared(policy string) { partyStepShared.WithLabelValues(policy).Inc() }

// SessionEnded counts one shared quest session ended.
func (BossMetrics) SessionEnded(outcome string) { partySessionEnded.WithLabelValues(outcome).Inc() }
