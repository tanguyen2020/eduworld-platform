package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Sanction events and audit actions (F116).
const (
	EventSanctionApplied = "sanction.applied"
	AuditSanctionApplied = "chat.sanction.applied"
)

// Reasons of system mutes (pipeline.ReasonAutoSpam / ReasonBlockAndAlert).
const (
	MuteReasonAutoSpam      = "auto_spam"
	MuteReasonBlockAndAlert = "block_and_alert_hold"
)

// SanctionPusher pushes chat.sanction.applied to the sanctioned student (best effort).
type SanctionPusher interface {
	SanctionApplied(ctx context.Context, user uuid.UUID, payload map[string]any) error
}

// ApplySystemMute is the system mute of the pipeline (F116-AC-06, F116-AC-07, MODERATION §3.4, §6.3): auto_spam mutes
// for auto_mute_spam (10 minutes, 1 hour when repeated within 24 hours); block_and_alert_hold mutes until the Safety
// Team handles the message (no end; revocation is F121). One transaction: the sanction, sanction.applied and the audit;
// then the sanction cache of the user is cleared (the next send reloads it).
type ApplySystemMute struct {
	UoW    write.UnitOfWork
	Repo   write.SanctionRepository
	Events write.EventPublisher
	Audit  write.Auditor
	Cache  port.SanctionCache
	Push   SanctionPusher // nil: none
	Now    func() time.Time
}

func (h ApplySystemMute) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// Mute applies the mute of reason to user.
func (h ApplySystemMute) Mute(ctx context.Context, tenant uuid.UUID, school *uuid.UUID, user uuid.UUID, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ApplySystemMute")
	defer func() { done(err) }()
	now := h.now()
	s := write.NewSanction{ID: ids.New(), TenantID: tenant, SchoolID: school, UserID: user, Type: "mute", Scope: "all", Reason: reason,
		StartsAt: now}
	skip := false
	err = h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		switch reason {
		case MuteReasonAutoSpam:
			d := moderation.AutoMuteFirst
			n, err := h.Repo.CountSince(ctx, tenant, user, reason, now.Add(-moderation.AutoMuteRepeatSince))
			if err != nil {
				return err
			}
			if n > 0 {
				d = moderation.AutoMuteRepeat
			}
			end := now.Add(d)
			s.EndsAt = &end
		case MuteReasonBlockAndAlert:
			open, err := h.Repo.OpenMute(ctx, tenant, user, reason)
			if err != nil {
				return err
			}
			if open {
				skip = true
				return nil
			}
		default:
			return fmt.Errorf("chat: unknown system mute reason %q", reason)
		}
		if err := h.Repo.Insert(ctx, s); err != nil {
			return err
		}
		// sanction.applied v1 (EVENT_DESIGN §4.9; schema extended by F121): the reason code, never an internal reason.
		payload := sanctionAppliedPayload(write.SafetySanction{ID: s.ID, TenantID: tenant, UserID: user, Type: s.Type, Scope: s.Scope,
			ReasonCode: reason, IssuedByKind: "system", StartsAt: now, EndsAt: s.EndsAt})
		t := tenant
		if err := h.Events.Publish(ctx, write.Event{Type: EventSanctionApplied, SubjectType: "chat_sanction", SubjectID: s.ID.String(),
			PartitionKey: user.String(), TenantID: &t, OccurredAt: now, Payload: payload}); err != nil {
			return err
		}
		ev := audit.Event{Action: AuditSanctionApplied, TargetType: "student", TargetID: user.String(), Result: audit.ResultSuccess,
			Reason: reason, Metadata: map[string]any{"sanction_id": s.ID.String(), "type": s.Type, "scope": s.Scope}}
		if school != nil {
			ev.SchoolID = school.String()
		}
		return h.Audit.Record(ctx, ev)
	})
	bg := context.WithoutCancel(ctx)
	if err != nil {
		// PostgreSQL down: the student is still restricted at once through the cache (never left unprotected); the
		// sanction is not durable, which the error log reports (R9).
		ttl := moderation.AutoMuteFirst
		if s.EndsAt == nil {
			ttl = time.Hour
		}
		end := now.Add(ttl)
		if cerr := h.Cache.Set(bg, user, []port.Sanction{{ID: s.ID, Type: "mute", Scope: "all", EndsAt: &end}}, ttl); cerr != nil {
			slog.ErrorContext(ctx, "chat: system mute neither stored nor cached", "error", cerr)
		}
		return err
	}
	if skip {
		return nil
	}
	if cerr := h.Cache.Invalidate(bg, user); cerr != nil { // the next send reloads the list with the mute
		slog.WarnContext(ctx, "chat: sanction cache not cleared", "error", cerr)
	}
	if h.Push != nil {
		p := map[string]any{"sanction_type": "mute", "reason_code": reason}
		if s.EndsAt != nil {
			p["until"] = s.EndsAt.UnixMilli()
		}
		if perr := h.Push.SanctionApplied(bg, user, p); perr != nil {
			slog.WarnContext(ctx, "chat: sanction not pushed", "error", perr)
		}
	}
	return nil
}
