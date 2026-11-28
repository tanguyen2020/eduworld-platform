package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Metrics of the game profile (plan F070 "Quan sát"): no label carries a nickname or a student id (R14).
var (
	avatarCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_game_avatar_created_total",
		Help: "First avatars saved (avatar.created, once per student).",
	})
	avatarRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_avatar_update_rejected_total",
		Help: "Refused avatar configs by reason (validation_failed, item_not_owned, equip_slot_invalid).",
	}, []string{"reason"})
	nicknameConflicts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_game_nickname_conflict_total",
		Help: "Nickname choices refused with NICKNAME_UNAVAILABLE (expired option, other student, taken meanwhile).",
	})
	// F136 (plan "Quan sát"): typed nickname checks by result and facade latency; no user or nickname label (R14).
	nicknameChecks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_nickname_checks_total",
		Help: "Typed nickname submissions by result (allowed, format, moderation, personal, taken, rate_limited, unavailable).",
	}, []string{"result"})
	nicknameScreen = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_nickname_screen_duration_seconds",
		Help:    "Latency of the facade calls of a typed nickname (chat moderation, identity screen).",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2},
	}, []string{"facade"})
)

// ProfileMetrics implements port.ProfileMetrics.
type ProfileMetrics struct{}

var (
	_ port.ProfileMetrics  = ProfileMetrics{}
	_ port.NicknameMetrics = ProfileMetrics{}
)

// NicknameCheck implements port.NicknameMetrics.
func (ProfileMetrics) NicknameCheck(result string) { nicknameChecks.WithLabelValues(result).Inc() }

// NicknameScreenLatency implements port.NicknameMetrics.
func (ProfileMetrics) NicknameScreenLatency(facade string, d time.Duration) {
	nicknameScreen.WithLabelValues(facade).Observe(d.Seconds())
}

// NicknameChecks is eduworld_nickname_checks_total{result} (tests).
func NicknameChecks() *prometheus.CounterVec { return nicknameChecks }

// AvatarCreated implements port.ProfileMetrics.
func (ProfileMetrics) AvatarCreated() { avatarCreated.Inc() }

// AvatarRejected implements port.ProfileMetrics.
func (ProfileMetrics) AvatarRejected(reason string) { avatarRejected.WithLabelValues(reason).Inc() }

// NicknameConflict implements port.ProfileMetrics.
func (ProfileMetrics) NicknameConflict() { nicknameConflicts.Inc() }

// AvatarRejections is eduworld_game_avatar_update_rejected_total{reason} (tests).
func AvatarRejections() *prometheus.CounterVec { return avatarRejected }

// RegisterProfile adds the profile metrics to r; already registered collectors are ignored.
func RegisterProfile(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{avatarCreated, avatarRejected, nicknameConflicts, nicknameChecks, nicknameScreen} {
		_ = r.Register(c)
	}
}
