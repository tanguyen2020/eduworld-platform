package config

import (
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of the gameplay chat lifecycle and purge (CONFIG_DEFAULTS: gameplay_chat_retention_days, ephemeral_chat_retention,
// evidence_context, gameplay_chat_purge_interval, gameplay_chat_purge_batch, gameplay_chat_purge_backlog_alert; ADR-0033).
// The retention days are the fallback while identity has projected no chat_retention (the setting of record is
// platform_settings/tenant settings of identity, never an environment value: R24, R33).
const (
	DefaultGameplayRetentionDays = 7
	DefaultEphemeralRetention    = 24 * time.Hour
	DefaultEvidenceContext       = 20
	DefaultPurgeInterval         = time.Hour
	DefaultPurgeBatch            = 1000
	DefaultPurgeBacklogAlert     = 48 * time.Hour
	DefaultPartitionDaysAhead    = 9
)

// Purge is the configuration of the gameplay chat jobs of worker-chat (T002). The DSN of chat_purger is not here: it is
// read by the registry (EW_DB_CHAT_PURGER_DSN / EW_DB_CHAT_PURGER_URL_FILE, secret file in staging/production) only by the
// purge job of worker-chat.
type Purge struct {
	GameplayRetentionDays int           // EW_GAMEPLAY_CHAT_RETENTION_DAYS (same key as identity: fallback only)
	EphemeralRetention    time.Duration // EW_EPHEMERAL_CHAT_RETENTION_HOURS (fallback of chat_retention.ephemeral_retention_hours)
	EvidenceContext       int           // EW_CHAT_EVIDENCE_CONTEXT (±messages copied around an evidence message)
	Interval              time.Duration // EW_CHAT_PURGE_INTERVAL (hourly, ADR-0033)
	Batch                 int           // EW_CHAT_PURGE_BATCH (rows per DELETE)
	BacklogAlert          time.Duration // EW_CHAT_PURGE_BACKLOG_ALERT (expired for longer → alert)
	PartitionDaysAhead    int           // EW_CHAT_PARTITION_DAYS_AHEAD (daily message partitions created ahead)
}

// PurgeFrom reads Purge.
func PurgeFrom(c *config.Config) (Purge, error) {
	p := Purge{
		GameplayRetentionDays: c.Int("EW_GAMEPLAY_CHAT_RETENTION_DAYS", DefaultGameplayRetentionDays),
		EphemeralRetention:    time.Duration(c.Int("EW_EPHEMERAL_CHAT_RETENTION_HOURS", int(DefaultEphemeralRetention/time.Hour))) * time.Hour,
		EvidenceContext:       c.Int("EW_CHAT_EVIDENCE_CONTEXT", DefaultEvidenceContext),
		Interval:              c.Duration("EW_CHAT_PURGE_INTERVAL", DefaultPurgeInterval),
		Batch:                 c.Int("EW_CHAT_PURGE_BATCH", DefaultPurgeBatch),
		BacklogAlert:          c.Duration("EW_CHAT_PURGE_BACKLOG_ALERT", DefaultPurgeBacklogAlert),
		PartitionDaysAhead:    c.Int("EW_CHAT_PARTITION_DAYS_AHEAD", DefaultPartitionDaysAhead),
	}
	switch {
	case p.GameplayRetentionDays < 1 || p.GameplayRetentionDays > 365:
		return p, errors.New("chat config: EW_GAMEPLAY_CHAT_RETENTION_DAYS must be 1..365 (the minimum 3 of ADR-0033 applies anyway)")
	case p.EphemeralRetention < time.Hour || p.EphemeralRetention > 168*time.Hour:
		return p, errors.New("chat config: EW_EPHEMERAL_CHAT_RETENTION_HOURS must be 1..168")
	case p.EvidenceContext < 0 || p.EvidenceContext > 50:
		return p, errors.New("chat config: EW_CHAT_EVIDENCE_CONTEXT must be 0..50")
	case p.Interval < time.Minute:
		return p, errors.New("chat config: EW_CHAT_PURGE_INTERVAL must be ≥ 1m")
	case p.Batch < 1 || p.Batch > 10000:
		return p, errors.New("chat config: EW_CHAT_PURGE_BATCH must be 1..10000")
	case p.BacklogAlert < time.Hour:
		return p, errors.New("chat config: EW_CHAT_PURGE_BACKLOG_ALERT must be ≥ 1h")
	case p.PartitionDaysAhead < 2 || p.PartitionDaysAhead > 62:
		return p, errors.New("chat config: EW_CHAT_PARTITION_DAYS_AHEAD must be 2..62")
	}
	return p, nil
}
