package observability

import (
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
)

// ProfileMetrics implements port.ProfileMetrics.
type ProfileMetrics struct{}

var _ port.ProfileMetrics = ProfileMetrics{}

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
	for _, c := range []prometheus.Collector{avatarCreated, avatarRejected, nicknameConflicts} {
		_ = r.Register(c)
	}
}
