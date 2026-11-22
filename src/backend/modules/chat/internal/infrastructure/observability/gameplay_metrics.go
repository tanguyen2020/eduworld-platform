package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// Gameplay chat metrics (F117 T035, plan §Quan sát): labels tenant/kind/reason/channel_type only — never a user, a
// channel id or content (R14). Alerts: eduworld_chat_purge_blocked > 0, eduworld_chat_purge_backlog_oldest_seconds >
// 2 days, eduworld_chat_purge_failures_total increasing.
var (
	gameplayOnce sync.Once

	purged = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_purge_total",
		Help: "Gameplay channels (kind gameplay) or world/area messages (kind ephemeral) physically purged (ADR-0033)."},
		[]string{"tenant", "kind"})
	deferred = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_purge_deferred_total",
		Help: "Expired gameplay channels kept because of an open report or case."}, []string{"reason"})
	blocked = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_chat_purge_blocked",
		Help: "1 while the purge deletes nothing (fail-closed), by reason."}, []string{"reason"})
	backlog = prometheus.NewGauge(prometheus.GaugeOpts{Name: "eduworld_chat_purge_backlog_channels",
		Help: "Closed gameplay channels past purge_after at the start of the last run."})
	backlogOldest = prometheus.NewGauge(prometheus.GaugeOpts{Name: "eduworld_chat_purge_backlog_oldest_seconds",
		Help: "Age past purge_after of the oldest expired gameplay channel (alert above 2 days)."})
	purgeFailures = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_purge_failures_total",
		Help: "Purge steps that failed and kept their data (fail-closed), by stage."}, []string{"stage"})
	partitionsAhead = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_chat_message_partition_days_ahead",
		Help: "Consecutive daily partitions of the chat message tables from today."}, []string{"table"})
	activeChannels = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "eduworld_chat_active_channels",
		Help: "Gameplay channels opened minus closed by this process, by channel type."}, []string{"channel_type"})
	gameplaySkipped = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_chat_gameplay_member_skipped_total",
		Help: "Participants of a party/match event not added to the gameplay channel (tenant mismatch, not a student, closed, stale)."},
		[]string{"reason"})
	retentionFloored = prometheus.NewCounter(prometheus.CounterOpts{Name: "eduworld_chat_retention_floored_total",
		Help: "Gameplay channels closed with the minimum retention because the configured value was lower (ADR-0033)."})
)

// RegisterGameplay registers the gameplay chat metrics once.
func RegisterGameplay(reg prometheus.Registerer) {
	gameplayOnce.Do(func() {
		if reg == nil {
			return
		}
		for _, c := range []prometheus.Collector{purged, deferred, blocked, backlog, backlogOldest, purgeFailures, partitionsAhead,
			activeChannels, gameplaySkipped, retentionFloored} {
			_ = reg.Register(c)
		}
	})
}

// Gameplay implements port.GameplayMetrics.
type Gameplay struct{}

var _ port.GameplayMetrics = Gameplay{}

// Skipped implements port.GameplayMetrics.
func (Gameplay) Skipped(reason string) { gameplaySkipped.WithLabelValues(reason).Inc() }

// Opened implements port.GameplayMetrics.
func (Gameplay) Opened(channelType string) { activeChannels.WithLabelValues(channelType).Inc() }

// Closed implements port.GameplayMetrics.
func (Gameplay) Closed(channelType string) { activeChannels.WithLabelValues(channelType).Dec() }

// RetentionFloored implements port.GameplayMetrics.
func (Gameplay) RetentionFloored() { retentionFloored.Inc() }

// Purge implements port.PurgeMetrics.
type Purge struct{}

var _ port.PurgeMetrics = Purge{}

// Purged implements port.PurgeMetrics.
func (Purge) Purged(tenant, kind string, n int) { purged.WithLabelValues(tenant, kind).Add(float64(n)) }

// Deferred implements port.PurgeMetrics.
func (Purge) Deferred(reason string) { deferred.WithLabelValues(reason).Inc() }

// Blocked implements port.PurgeMetrics.
func (Purge) Blocked(reason string, on bool) {
	v := 0.0
	if on {
		v = 1
	}
	blocked.WithLabelValues(reason).Set(v)
}

// Backlog implements port.PurgeMetrics.
func (Purge) Backlog(channels int, oldest time.Duration) {
	backlog.Set(float64(channels))
	backlogOldest.Set(oldest.Seconds())
}

// Failed implements port.PurgeMetrics.
func (Purge) Failed(stage string) { purgeFailures.WithLabelValues(stage).Inc() }

// PartitionsAhead implements port.PurgeMetrics.
func (Purge) PartitionsAhead(table string, days int) {
	partitionsAhead.WithLabelValues(table).Set(float64(days))
}
