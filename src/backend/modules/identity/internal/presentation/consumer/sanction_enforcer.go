package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SanctionEnforcerName is the durable consumer that enforces the sanction account_lock of the Safety Team (F124 US1).
const SanctionEnforcerName = "identity.sanction_enforcer"

// SanctionEnforcerSubjects are the events it handles (stream EVT_CHAT).
var SanctionEnforcerSubjects = []string{"ew.evt.chat.sanction.applied", "ew.evt.chat.sanction.lifted"}

// SanctionEnforcer dispatches sanction.applied / sanction.lifted to command.SanctionLocks in the consumer transaction
// (scope of the envelope tenant, idempotent through infra.processed_events). A student of another tenant is audited
// (security.tenant_mismatch, in its own transaction so it survives the rollback) and dead-lettered; a malformed payload or
// an unknown account is dead-lettered.
type SanctionEnforcer struct {
	Locks command.SanctionLocks
	Audit write.Auditor
}

type sanctionPayload struct {
	TenantID     uuid.UUID  `json:"tenant_id"`
	SanctionID   uuid.UUID  `json:"sanction_id"`
	UserID       uuid.UUID  `json:"user_id"`
	CaseID       *uuid.UUID `json:"case_id"`
	SanctionType string     `json:"sanction_type"`
	StartsAt     *time.Time `json:"starts_at"`
	LiftedAt     *time.Time `json:"lifted_at"`
}

// Handle implements outbox.Handler.
func (c SanctionEnforcer) Handle(ctx context.Context, e outbox.Envelope) error {
	var p sanctionPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.SanctionID == uuid.Nil || p.UserID == uuid.Nil || p.SanctionType == "" {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	if e.TenantID == nil || *e.TenantID != p.TenantID {
		return outbox.Permanent(fmt.Errorf("%s: payload tenant differs from the envelope", e.EventType))
	}
	ev := command.SanctionEvent{EventID: e.EventID, TenantID: *e.TenantID, SanctionID: p.SanctionID, UserID: p.UserID, CaseID: p.CaseID,
		SanctionType: p.SanctionType, At: e.OccurredAt.UTC()}
	var err error
	switch e.EventType {
	case "sanction.applied":
		if p.StartsAt != nil {
			ev.At = p.StartsAt.UTC()
		}
		err = c.Locks.Applied(ctx, ev)
	case "sanction.lifted":
		if p.LiftedAt != nil {
			ev.At = p.LiftedAt.UTC()
		}
		err = c.Locks.Lifted(ctx, ev)
	default:
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	switch {
	case errors.Is(err, command.ErrSanctionTenantMismatch):
		c.mismatch(ctx, e, p)
		return outbox.Permanent(err)
	case errors.Is(err, command.ErrSanctionUnknownUser):
		return outbox.Permanent(err)
	}
	return err
}

// mismatch audits security.tenant_mismatch outside the consumer transaction (which rolls back before the DLQ): ids only.
func (c SanctionEnforcer) mismatch(ctx context.Context, e outbox.Envelope, p sanctionPayload) {
	if c.Audit == nil {
		return
	}
	dctx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(ctx))
	if sc, err := authz.EventScope(*e.TenantID); err == nil {
		dctx = tenancy.WithModule(tenancy.WithScope(dctx, sc), "identity")
	}
	if err := c.Audit.Record(dctx, audit.Event{Action: command.AuditSecurityTenantMismatch, TargetType: "user", TargetID: p.UserID.String(),
		Result: audit.ResultFailure, Metadata: map[string]any{"event_id": e.EventID.String(), "event_type": e.EventType,
			"sanction_id": p.SanctionID.String(), "consumer": SanctionEnforcerName}}); err != nil {
		slog.WarnContext(ctx, "identity: tenant mismatch audit failed", "event_id", e.EventID.String(), "error", err)
	}
}
