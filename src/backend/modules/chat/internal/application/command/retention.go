package command

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// AuditRetentionApplied is the aggregate audit of one tenant per run of persistent-chat-retention (counts, no content).
const AuditRetentionApplied = "chat.retention_applied"

// RetentionConfig is the configuration of the job (persistent_chat_retention_days fallback, batch, evidence context).
type RetentionConfig struct {
	Days            int
	Batch           int
	EvidenceContext int
	MaxBatches      int // per tenant and run (0: 100)
}

// RetentionMetrics counts the anonymised messages (eduworld_chat_retention_anonymized_total{tenant}).
type RetentionMetrics interface {
	Anonymized(tenant string, n int)
}

// PersistentRetention is the job persistent-chat-retention (F120 US6, FR-CHAT-006): for each tenant, in batches, the
// messages of the persistent channels (club team, direct) past their retention are anonymised — body, body_display, quote
// and the revisions overwritten, anonymized_at set — after the messages a report or case references were copied to
// chat.moderation_evidence (F120-AC-24, AC-25). Never DELETE, never a partition drop (R21, ADR-0070); one aggregate audit
// and one persistent_retention_runs row per tenant. A failure of one tenant does not stop the others.
type PersistentRetention struct {
	UoW     write.UnitOfWork
	Store   port.RetentionStore
	Audit   write.Auditor
	Lock    port.JobLock // nil: no lease (tests)
	Metrics RetentionMetrics
	Config  RetentionConfig
	Now     func() time.Time
}

func (h PersistentRetention) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// Run implements the job runner: the number of messages anonymised.
func (h PersistentRetention) Run(ctx context.Context) (total int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.job.PersistentChatRetention")
	defer func() { done(err) }()
	if h.Lock != nil {
		ok, err := h.Lock.Acquire(ctx, "chat.persistent-chat-retention", time.Hour)
		if err != nil || !ok {
			return 0, err
		}
	}
	scope, err := authz.SystemPlatformScope("persistent-chat-retention")
	if err != nil {
		return 0, err
	}
	var tenants []uuid.UUID
	days := h.Config.Days
	err = h.UoW.InTenantTx(ctx, scope, func(ctx context.Context) error {
		var err error
		if tenants, err = h.Store.Tenants(ctx); err != nil {
			return err
		}
		d, err := h.Store.PlatformDays(ctx)
		if err == nil && d != nil {
			days = *d
		}
		return err
	})
	if err != nil {
		return 0, err
	}
	for _, t := range tenants {
		n, err := h.tenant(ctx, t, days)
		total += n
		if err != nil {
			slog.ErrorContext(ctx, "chat: persistent retention failed for a tenant", "tenant_id", t, "error", err)
		}
	}
	return total, nil
}

// tenant anonymises the expired messages of one tenant, batch by batch (one transaction each).
func (h PersistentRetention) tenant(ctx context.Context, tenant uuid.UUID, days int) (int, error) {
	start := h.now()
	run := port.RetentionRun{ID: ids.New(), TenantID: tenant, StartedAt: start, Status: "succeeded"}
	batch := h.Config.Batch
	if batch <= 0 {
		batch = 500
	}
	maxBatches := h.Config.MaxBatches
	if maxBatches <= 0 {
		maxBatches = 100
	}
	channels := map[uuid.UUID]bool{}
	var runErr error
	for range maxBatches {
		n := 0
		err := h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
			msgs, err := h.Store.Expired(ctx, tenant, days, batch, start)
			if err != nil || len(msgs) == 0 {
				return err
			}
			list := make([]uuid.UUID, len(msgs))
			for i, m := range msgs {
				list[i], channels[m.ChannelID] = m.MessageID, true
			}
			copied, err := h.Store.CopyEvidence(ctx, tenant, list, h.Config.EvidenceContext, start)
			if err != nil {
				return err
			}
			run.EvidenceCopied += copied
			n, err = h.Store.Anonymize(ctx, tenant, list, start)
			return err
		})
		if err != nil {
			runErr = err
			break
		}
		run.Anonymized += n
		if n < batch {
			break
		}
	}
	run.Channels, run.FinishedAt = len(channels), h.now()
	if runErr != nil {
		run.Status, run.ErrorCode = "failed", "anonymize_failed"
	}
	if run.Anonymized == 0 && runErr == nil {
		return 0, nil // nothing to do: no run row, no audit
	}
	err := h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		if err := h.Store.RecordRun(ctx, run); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditRetentionApplied, TargetType: "chat_retention_run", TargetID: run.ID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"messages_anonymized": run.Anonymized, "channels_scanned": run.Channels,
				"evidence_copied": run.EvidenceCopied, "status": run.Status, "retention_days": days}})
	})
	if h.Metrics != nil {
		h.Metrics.Anonymized(tenant.String(), run.Anonymized)
	}
	if runErr != nil {
		return run.Anonymized, runErr
	}
	return run.Anonymized, err
}
