package persistence

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// Sanctions implements write.SanctionRepository on chat.sanctions (F116: system mutes).
type Sanctions struct{}

var _ write.SanctionRepository = Sanctions{}

// Insert implements write.SanctionRepository.
func (Sanctions) Insert(ctx context.Context, s write.NewSanction) error {
	_, err := exec(ctx, `INSERT INTO chat.sanctions (sanction_id, tenant_id, created_at, created_by, updated_at, school_id, user_id, type,
		scope, channel_id, reason, issued_by, starts_at, ends_at) VALUES ($1, $2, now(), NULL, now(), $3, $4, $5, $6, $7, $8, NULL, $9, $10)`,
		s.ID, s.TenantID, s.SchoolID, s.UserID, s.Type, s.Scope, s.ChannelID, s.Reason, s.StartsAt, s.EndsAt)
	if err != nil {
		return fmt.Errorf("chat: insert sanction: %w", err)
	}
	return nil
}

// CountSince implements write.SanctionRepository.
func (Sanctions) CountSince(ctx context.Context, tenant, user uuid.UUID, reason string, since time.Time) (int, error) {
	return count(ctx, `SELECT count(*) FROM chat.sanctions s WHERE s.tenant_id = $1 AND s.user_id = $2 AND s.reason = $3
		AND s.created_at >= $4 AND s.deleted_at IS NULL`, tenant, user, reason, since)
}

// OpenMute implements write.SanctionRepository.
func (Sanctions) OpenMute(ctx context.Context, tenant, user uuid.UUID, reason string) (bool, error) {
	n, err := count(ctx, `SELECT count(*) FROM chat.sanctions s WHERE s.tenant_id = $1 AND s.user_id = $2 AND s.reason = $3
		AND s.type = 'mute' AND s.ends_at IS NULL AND s.revoked_at IS NULL AND s.deleted_at IS NULL`, tenant, user, reason)
	return n > 0, err
}

const insertDecisionSQL = `INSERT INTO chat.moderation_decisions (decision_id, tenant_id, channel_id, message_id, sender_id, decision,
	category_codes, steps_run, steps_skipped, catalog_version_platform, catalog_version_tenant, switch_snapshot_id, latency_ms, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7::varchar[], $8::varchar[], $9::varchar[], $10, $11, $12, $13, $14) ON CONFLICT DO NOTHING`

func strs(s []string) []string {
	if s == nil {
		return []string{}
	}
	if len(s) > 20 {
		return s[:20]
	}
	return s
}

func insertDecision(ctx context.Context, d port.DecisionRecord) error {
	_, err := exec(ctx, insertDecisionSQL, d.DecisionID, d.TenantID, d.ChannelID, d.MessageID, d.SenderID, d.Decision, strs(d.CategoryCodes),
		strs(d.StepsRun), strs(d.StepsSkipped), d.CatalogVersionPlatform, d.CatalogVersionTenant, d.SwitchSnapshotID, max(d.LatencyMS, 0),
		d.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("chat: decision log: %w", err)
	}
	return nil
}

// EnvelopeDecision is the decision row of a stored message (decision_id = message_id: idempotent on redelivery).
func EnvelopeDecision(e message.Envelope) port.DecisionRecord {
	id := e.MessageID
	return port.DecisionRecord{DecisionID: e.MessageID, TenantID: e.TenantID, ChannelID: e.ChannelID, MessageID: &id, SenderID: e.SenderID,
		Decision: string(e.Moderation.Decision), CategoryCodes: e.Moderation.CategoryCodes, StepsRun: e.Moderation.StepsRun,
		StepsSkipped: e.Moderation.StepsSkipped, CatalogVersionPlatform: e.Moderation.CatalogVersion,
		CatalogVersionTenant: e.Moderation.CatalogVersionTenant, SwitchSnapshotID: e.Moderation.SwitchSnapshotID,
		LatencyMS: e.Moderation.LatencyMS, CreatedAt: e.CreatedAt}
}

// DecisionWriter implements port.DecisionLog for blocked messages (research R2): a bounded buffer flushed in batches,
// one transaction per tenant (RLS), metadata only. A full buffer drops the record with a metric (best effort: a
// blocked message is never delivered, the log is for audit/dashboards).
type DecisionWriter struct {
	UoW       write.UnitOfWork
	Size      int
	Interval  time.Duration
	OnDropped func()

	once sync.Once
	ch   chan port.DecisionRecord
}

func (w *DecisionWriter) init() {
	w.once.Do(func() {
		size := w.Size
		if size <= 0 {
			size = 1024
		}
		w.ch = make(chan port.DecisionRecord, size)
	})
}

// Record implements port.DecisionLog.
func (w *DecisionWriter) Record(_ context.Context, d port.DecisionRecord) {
	w.init()
	select {
	case w.ch <- d:
	default:
		if w.OnDropped != nil {
			w.OnDropped()
		}
		slog.Warn("chat: decision log buffer full, record dropped")
	}
}

// Run flushes the buffer until ctx ends (then flushes what is left).
func (w *DecisionWriter) Run(ctx context.Context) error {
	w.init()
	interval := w.Interval
	if interval <= 0 {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	var batch []port.DecisionRecord
	for {
		select {
		case d := <-w.ch:
			batch = append(batch, d)
			if len(batch) >= 100 {
				w.flush(context.WithoutCancel(ctx), batch)
				batch = nil
			}
		case <-t.C:
			if len(batch) > 0 {
				w.flush(context.WithoutCancel(ctx), batch)
				batch = nil
			}
		case <-ctx.Done():
			for {
				select {
				case d := <-w.ch:
					batch = append(batch, d)
				default:
					if len(batch) > 0 {
						w.flush(context.WithoutCancel(ctx), batch)
					}
					return nil
				}
			}
		}
	}
}

// Flush writes everything buffered now (tests).
func (w *DecisionWriter) Flush(ctx context.Context) {
	w.init()
	var batch []port.DecisionRecord
	for {
		select {
		case d := <-w.ch:
			batch = append(batch, d)
		default:
			w.flush(ctx, batch)
			return
		}
	}
}

func (w *DecisionWriter) flush(ctx context.Context, batch []port.DecisionRecord) {
	byTenant := map[uuid.UUID][]port.DecisionRecord{}
	for _, d := range batch {
		byTenant[d.TenantID] = append(byTenant[d.TenantID], d)
	}
	for tenant, ds := range byTenant {
		s, err := authz.EventScope(tenant)
		if err != nil {
			continue
		}
		err = w.UoW.InTenantTx(ctx, s, func(ctx context.Context) error {
			for _, d := range ds {
				if err := insertDecision(ctx, d); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			slog.WarnContext(ctx, "chat: decision log not written", "count", len(ds), "error", err)
			if w.OnDropped != nil {
				for range ds {
					w.OnDropped()
				}
			}
		}
	}
}
