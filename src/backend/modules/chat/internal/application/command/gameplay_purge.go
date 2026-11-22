package command

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Names of the gameplay chat jobs of worker-chat (ADR-0033, DEPLOYMENT §7).
const (
	JobGameplayChatPurge  = "gameplay-chat-purge"
	JobPartitionMaintain  = "chat-partition-maintainer"
	EventGameplayPurged   = "chat.gameplay_purged"
	EventDailyStats       = "chat.daily_stats"
	AuditGameplayPurgeRun = "chat.gameplay_purge.run"
	// ReasonModerationUnavailable: the moderation sources are missing (F117-AC-19, research R3).
	ReasonModerationUnavailable = "moderation_unavailable"
	ReasonOpenCase              = "open_case"
	ReasonOpenReport            = "open_report"
)

// PurgeConfig of the job (CONFIG_DEFAULTS, ADR-0033).
type PurgeConfig struct {
	EvidenceContext    int           // evidence_context: ±messages copied with an evidence message
	Batch              int           // rows per DELETE (1 000)
	BacklogAlert       time.Duration // expired for longer → alert (2 days)
	EphemeralRetention time.Duration // fallback of chat_retention.ephemeral_retention_hours (24 h)
	MaxChannels        int           // channels per run (default 5 000; the rest at the next run)
	LockTTL            time.Duration // lease of ew:lock:job:gameplay-chat-purge (default 1 h)
}

// GameplayPurge is the job gameplay-chat-purge (F117 US4, ADR-0033): the ONLY physical delete of the platform (R21). It
// runs in worker-chat on the pool of chat_purger, one instance at a time (ew:lock:job:gameplay-chat-purge). Fail-closed
// everywhere: without the moderation sources nothing is deleted (run blocked, F117-AC-19); a channel with an open report
// or case is deferred (F117-AC-18); a failed evidence copy rolls the channel back (it stays); referenced messages are
// copied into chat.moderation_evidence before any delete (F117-AC-17). Each run records its aggregates (no content),
// an audit, chat.gameplay_purged and chat.daily_stats (F117-AC-22).
type GameplayPurge struct {
	UoW     write.UnitOfWork // the purger pool (chat_purger)
	Store   port.PurgeStore
	Events  write.EventPublisher
	Audit   write.Auditor
	Lock    port.JobLock     // nil: no lease (tests)
	Facts   FactsInvalidator // drops the cached facts of a purged channel (core-api then answers 404 at once); nil: none
	Metrics port.PurgeMetrics
	Config  PurgeConfig
	Now     func() time.Time
}

func (h GameplayPurge) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

type tenantTally struct {
	channels, deferred, failed, evidence, ephemeralEvidence int
	messages, ephemeral                                     int64
	stats                                                   map[string]*port.DayStats
	ephemeralRan, ephemeralFailed                           bool
}

// RunResult summarizes one run (tests, job count).
type RunResult struct {
	RunID             uuid.UUID
	Blocked           bool
	ChannelsPurged    int
	Deferred          int
	Failed            int
	PartitionsDropped int
	EphemeralDeleted  int64
	EphemeralDropped  int
}

// Run is one run of the job; it returns the number of channels purged.
func (h GameplayPurge) Run(ctx context.Context) (int, error) {
	r, err := h.RunOnce(ctx)
	return r.ChannelsPurged, err
}

// RunOnce runs the purge once.
func (h GameplayPurge) RunOnce(ctx context.Context) (res RunResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.job.GameplayChatPurge")
	defer func() { done(err) }()
	if h.Lock != nil {
		ttl := h.Config.LockTTL
		if ttl <= 0 {
			ttl = time.Hour
		}
		ok, err := h.Lock.Acquire(ctx, JobGameplayChatPurge, ttl)
		if err != nil || !ok {
			return res, err // another worker runs it (edge case: two workers)
		}
	}
	start := h.now()
	res.RunID = ids.New()
	var ready bool
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		ready, err = h.Store.ModerationReady(ctx)
		return err
	})
	if err != nil || !ready {
		// Fail-closed (F117-AC-19): keeping data longer than its retention is recoverable, losing evidence is not.
		slog.ErrorContext(ctx, "chat: gameplay purge blocked: moderation sources unavailable, nothing deleted", "alert", "P2",
			"error", err)
		h.Metrics.Blocked(ReasonModerationUnavailable, true)
		res.Blocked = true
		for _, kind := range []string{port.KindGameplay, port.KindEphemeral} {
			if rerr := h.record(ctx, port.PurgeRun{RunID: res.RunID, Kind: kind, StartedAt: start, FinishedAt: h.now(),
				Status: port.RunBlocked}); rerr != nil {
				return res, rerr
			}
		}
		return res, nil
	}
	h.Metrics.Blocked(ReasonModerationUnavailable, false)
	tallies := map[uuid.UUID]*tenantTally{}
	tally := func(t uuid.UUID) *tenantTally {
		if tallies[t] == nil {
			tallies[t] = &tenantTally{stats: map[string]*port.DayStats{}}
		}
		return tallies[t]
	}
	if err := h.gameplay(ctx, start, &res, tally); err != nil {
		return res, err
	}
	h.ephemeral(ctx, start, &res, tally)
	return res, h.finish(ctx, start, &res, tallies)
}

func (h GameplayPurge) maxChannels() int {
	if h.Config.MaxChannels > 0 {
		return h.Config.MaxChannels
	}
	return 5000
}

func (h GameplayPurge) gameplay(ctx context.Context, now time.Time, res *RunResult, tally func(uuid.UUID) *tenantTally) error {
	var expired []port.ExpiredChannel
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		expired, err = h.Store.Expired(ctx, now, h.maxChannels())
		return err
	}); err != nil {
		h.Metrics.Failed("list")
		return err
	}
	var oldest time.Duration
	for _, c := range expired {
		oldest = max(oldest, now.Sub(c.PurgeAfter))
	}
	h.Metrics.Backlog(len(expired), oldest)
	if h.Config.BacklogAlert > 0 && oldest > h.Config.BacklogAlert {
		slog.ErrorContext(ctx, "chat: gameplay purge backlog older than the alert threshold", "alert", "P2",
			"channels", len(expired), "oldest_hours", int(oldest.Hours()))
	}
	for _, c := range expired {
		t := tally(c.TenantID)
		var out port.ChannelPurge
		var holds port.Holds
		err := h.UoW.InTenantTx(ctx, mustScope(c.TenantID), func(ctx context.Context) error {
			var err error
			if holds, err = h.Store.Holds(ctx, c.TenantID, c.ID); err != nil || holds.Held() {
				return err
			}
			out, err = h.Store.PurgeChannel(ctx, c, h.Config.EvidenceContext, h.Config.Batch, now)
			return err
		})
		switch {
		case err != nil:
			// The transaction rolled back: the channel and its messages stay until the next run (fail-closed).
			slog.ErrorContext(ctx, "chat: gameplay channel not purged (evidence or delete failed), kept", "channel_id", c.ID, "error", err)
			h.Metrics.Failed("channel")
			t.failed++
			res.Failed++
		case holds.Held():
			reason := ReasonOpenCase
			if holds.OpenCases == 0 {
				reason = ReasonOpenReport
			}
			slog.InfoContext(ctx, "chat: gameplay channel purge deferred", "channel_id", c.ID, "reason", reason)
			h.Metrics.Deferred(reason)
			t.deferred++
			res.Deferred++
		default:
			if h.Facts != nil {
				if err := h.Facts.InvalidateChannel(context.WithoutCancel(ctx), c.TenantID, c.ID); err != nil {
					slog.WarnContext(ctx, "chat: cached facts of a purged channel not dropped", "error", err)
				}
			}
			t.channels++
			t.messages += out.Messages
			t.evidence += out.Evidence
			res.ChannelsPurged++
			for _, s := range out.Stats {
				key := s.Day.Format("2006-01-02") + "|" + s.ChannelType
				if t.stats[key] == nil {
					cp := s
					cp.Channels, cp.Messages, cp.Masked, cp.Blocked, cp.Reported = 0, 0, 0, 0, 0
					t.stats[key] = &cp
				}
				acc := t.stats[key]
				acc.Channels += s.Channels
				acc.Messages += s.Messages
				acc.Masked += s.Masked
				acc.Blocked += s.Blocked
				acc.Reported += s.Reported
			}
		}
	}
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res.PartitionsDropped, err = h.Store.DropGameplayPartitions(ctx)
		return err
	})
	if err != nil {
		slog.WarnContext(ctx, "chat: gameplay partitions not dropped", "error", err)
		h.Metrics.Failed("partition")
	}
	return nil
}

// ephemeral purges world/area messages older than ephemeral_chat_retention (F117-AC-21): evidence first for every
// tenant; the daily partitions are dropped only when every copy succeeded, and the remaining rows of a tenant are deleted
// only when its copy succeeded.
func (h GameplayPurge) ephemeral(ctx context.Context, now time.Time, res *RunResult, tally func(uuid.UUID) *tenantTally) {
	retention := h.Config.EphemeralRetention
	var tenants []uuid.UUID
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		if d, err := h.Store.EphemeralRetention(ctx); err != nil {
			return err
		} else if d != nil && *d >= time.Hour {
			retention = *d
		}
		var err error
		tenants, err = h.Store.EphemeralTenants(ctx, now.Add(-retention))
		return err
	})
	if err != nil {
		slog.WarnContext(ctx, "chat: ephemeral purge skipped", "error", err)
		h.Metrics.Failed("ephemeral")
		return
	}
	if retention < time.Hour {
		retention = time.Hour
	}
	cutoff := now.Add(-retention)
	allCopied := true
	for _, tenant := range tenants {
		t := tally(tenant)
		t.ephemeralRan = true
		var n int
		err := h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
			var err error
			n, err = h.Store.CopyEphemeralEvidence(ctx, tenant, cutoff, h.Config.EvidenceContext, now)
			return err
		})
		if err != nil {
			slog.ErrorContext(ctx, "chat: ephemeral evidence not copied, the tenant's messages are kept", "error", err)
			h.Metrics.Failed("ephemeral_evidence")
			t.ephemeralFailed, allCopied = true, false
			continue
		}
		t.ephemeralEvidence += n
	}
	if allCopied {
		if err := h.UoW.Do(ctx, func(ctx context.Context) error {
			var err error
			res.EphemeralDropped, err = h.Store.DropEphemeralPartitions(ctx, cutoff)
			return err
		}); err != nil {
			slog.WarnContext(ctx, "chat: ephemeral partitions not dropped", "error", err)
			h.Metrics.Failed("ephemeral_partition")
		}
	}
	for _, tenant := range tenants {
		t := tally(tenant)
		if t.ephemeralFailed {
			continue
		}
		var n int64
		err := h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
			var err error
			n, err = h.Store.DeleteEphemeral(ctx, tenant, cutoff, h.Config.Batch)
			return err
		})
		if err != nil {
			h.Metrics.Failed("ephemeral_delete")
			t.ephemeralFailed = true
			continue
		}
		t.ephemeral += n
		res.EphemeralDeleted += n
	}
}

func status(failed int) string {
	if failed > 0 {
		return port.RunPartial
	}
	return port.RunSucceeded
}

// finish writes, per tenant, the run rows, the summary audit, chat.gameplay_purged and chat.daily_stats; then the
// run-level row (partitions dropped across tenants).
func (h GameplayPurge) finish(ctx context.Context, start time.Time, res *RunResult, tallies map[uuid.UUID]*tenantTally) error {
	end := h.now()
	tenants := make([]uuid.UUID, 0, len(tallies))
	for id := range tallies {
		tenants = append(tenants, id)
	}
	sort.Slice(tenants, func(i, j int) bool { return tenants[i].String() < tenants[j].String() })
	for _, tenant := range tenants {
		t := tallies[tenant]
		id := tenant
		runs := []port.PurgeRun{}
		if t.channels+t.deferred+t.failed > 0 {
			runs = append(runs, port.PurgeRun{RunID: res.RunID, TenantID: &id, Kind: port.KindGameplay, StartedAt: start, FinishedAt: end,
				ChannelsPurged: t.channels, MessagesPurged: t.messages, DeferredOpenCase: t.deferred, EvidenceCopied: t.evidence,
				Status: status(t.failed)})
		}
		if t.ephemeralRan {
			st := port.RunSucceeded
			if t.ephemeralFailed {
				st = port.RunPartial
			}
			runs = append(runs, port.PurgeRun{RunID: res.RunID, TenantID: &id, Kind: port.KindEphemeral, StartedAt: start, FinishedAt: end,
				MessagesPurged: t.ephemeral, EvidenceCopied: t.ephemeralEvidence, Status: st})
		}
		stats := make([]port.DayStats, 0, len(t.stats))
		for _, s := range t.stats {
			stats = append(stats, *s)
		}
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].Day.Before(stats[j].Day) || (stats[i].Day.Equal(stats[j].Day) && stats[i].ChannelType < stats[j].ChannelType)
		})
		if err := h.record(ctx, runs, stats...); err != nil {
			return err
		}
		h.Metrics.Purged(tenant.String(), port.KindGameplay, t.channels)
		if t.ephemeralRan {
			h.Metrics.Purged(tenant.String(), port.KindEphemeral, int(t.ephemeral))
		}
	}
	failed := res.Failed
	return h.record(ctx, []port.PurgeRun{{RunID: res.RunID, Kind: port.KindGameplay, StartedAt: start, FinishedAt: end,
		ChannelsPurged: res.ChannelsPurged, PartitionsDropped: res.PartitionsDropped, DeferredOpenCase: res.Deferred,
		Status: status(failed)}, {RunID: res.RunID, Kind: port.KindEphemeral, StartedAt: start, FinishedAt: end,
		MessagesPurged: res.EphemeralDeleted, PartitionsDropped: res.EphemeralDropped, Status: port.RunSucceeded}})
}

// record writes run rows of one tenant (or the run-level rows, TenantID nil) with their audit and events, in one
// transaction of that tenant (no scope for the run-level rows).
func (h GameplayPurge) record(ctx context.Context, runs any, stats ...port.DayStats) error {
	var list []port.PurgeRun
	switch v := runs.(type) {
	case port.PurgeRun:
		list = []port.PurgeRun{v}
	case []port.PurgeRun:
		list = v
	}
	if len(list) == 0 {
		return nil
	}
	tenant := list[0].TenantID
	fn := func(ctx context.Context) error {
		for _, r := range list {
			r := r
			if err := h.Store.RecordRun(ctx, r); err != nil {
				return err
			}
			payload := map[string]any{"run_id": r.RunID, "kind": r.Kind, "status": r.Status, "channels": r.ChannelsPurged,
				"messages": r.MessagesPurged, "partitions_dropped": r.PartitionsDropped, "deferred_open_case": r.DeferredOpenCase,
				"evidence_copied": r.EvidenceCopied, "started_at": r.StartedAt, "finished_at": r.FinishedAt}
			ev := write.Event{Type: EventGameplayPurged, SubjectType: "chat_purge_run", SubjectID: r.RunID.String(),
				PartitionKey: r.RunID.String(), OccurredAt: r.FinishedAt, Payload: payload}
			if r.TenantID != nil {
				payload["tenant_id"] = *r.TenantID
				ev.TenantID = r.TenantID
			} else {
				payload["tenant_id"] = nil
				ev.Platform = true
			}
			if err := h.Events.Publish(ctx, ev); err != nil {
				return err
			}
			meta := map[string]any{"run_id": r.RunID.String(), "kind": r.Kind, "status": r.Status, "channels": r.ChannelsPurged,
				"messages": r.MessagesPurged, "partitions_dropped": r.PartitionsDropped, "deferred_open_case": r.DeferredOpenCase,
				"evidence_copied": r.EvidenceCopied}
			result := audit.ResultSuccess
			if r.Status == port.RunBlocked || r.Status == port.RunFailed {
				result = audit.ResultFailure
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: AuditGameplayPurgeRun, TargetType: "chat_purge_run",
				TargetID: r.RunID.String(), Result: result, Metadata: meta}); err != nil {
				return err
			}
		}
		for _, s := range stats {
			if err := h.Events.Publish(ctx, write.Event{Type: EventDailyStats, SubjectType: "chat_daily_stats",
				SubjectID: tenant.String() + ":" + s.Day.Format("2006-01-02") + ":" + s.ChannelType, PartitionKey: tenant.String(),
				TenantID: tenant, OccurredAt: h.now(), Payload: map[string]any{"tenant_id": *tenant, "stat_date": s.Day.Format("2006-01-02"),
					"channel_type": s.ChannelType, "channels": s.Channels, "messages": s.Messages, "masked": s.Masked,
					"blocked": s.Blocked, "reported": s.Reported}}); err != nil {
				return err
			}
		}
		return nil
	}
	if tenant == nil {
		return h.UoW.Do(ctx, fn)
	}
	return h.UoW.InTenantTx(ctx, mustScope(*tenant), fn)
}
