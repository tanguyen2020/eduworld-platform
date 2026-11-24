package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Sanction type and reason codes of the moderation lock (EVENT_DESIGN §4.9, F124).
const (
	SanctionAccountLock         = "account_lock"
	ReasonModerationSanction    = "moderation_sanction"
	AuditUserLocked             = "user.locked"
	AuditUserUnlocked           = "user.unlocked"
	AuditUserLockReleased       = "user.lock_released"
	AuditSecurityTenantMismatch = "security.tenant_mismatch"
)

// ErrSanctionTenantMismatch: the student of a sanction event is not a member of the tenant of the envelope (the
// consumer audits security.tenant_mismatch and sends the event to the DLQ, F124-AC-04).
var ErrSanctionTenantMismatch = errors.New("identity: sanction event of another tenant")

// ErrSanctionUnknownUser: the sanction names no account of identity (DLQ for an operator).
var ErrSanctionUnknownUser = errors.New("identity: sanction of an unknown account")

// SanctionEvent is the part of sanction.applied / sanction.lifted v1 used by module identity (ids and codes only).
type SanctionEvent struct {
	EventID      uuid.UUID
	TenantID     uuid.UUID // envelope tenant
	SanctionID   uuid.UUID
	UserID       uuid.UUID
	CaseID       *uuid.UUID
	SanctionType string
	At           time.Time
}

// LockMetrics observes the moderation locks (eduworld_account_locks_total{source,action}).
type LockMetrics interface {
	Lock(source, action string)
}

// SanctionLocks enforces the sanction account_lock of the Platform Safety Team on the account (F124 US1, FR-MOD-006,
// FR-AUTH-010): sanction.applied adds a lock of source moderation_sanction (sanction and case ids) and runs the revocation
// chain of F016 (user.status_changed + user.access_revoked in the transaction of the consumer: Keycloak disable and
// logout, game sessions, realtime); sanction.lifted releases exactly that lock and restores access only when no lock of
// another source remains. Both are idempotent per sanction (a lifted-before-applied sanction leaves a released lock that
// never locks the account). Other sanction types, staff accounts and accounts outside the envelope tenant are ignored
// (the latter is ErrSanctionTenantMismatch). It runs inside the consumer transaction (scope of the envelope tenant).
type SanctionLocks struct {
	Repo    write.LifecycleRepository
	Locks   write.AccountLockRepository
	Events  write.EventPublisher
	Audit   write.Auditor
	Metrics LockMetrics // nil: none
}

func (h SanctionLocks) metric(action string) {
	if h.Metrics != nil {
		h.Metrics.Lock(user.LockModeration, action)
	}
}

// target loads the student of a sanction in the scope tenant: nil + nil for an ignored account (staff), an error for an
// unknown account or a student of another tenant.
func (h SanctionLocks) target(ctx context.Context, ev SanctionEvent) (*write.LifecycleTarget, error) {
	t, err := h.Repo.Target(ctx, ev.UserID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, ErrSanctionUnknownUser
	}
	if t.UserType != string(authz.UserTypeStudent) {
		return nil, nil // a sanction account_lock locks students only (F124-AC-04)
	}
	if t.StudentTenantID == nil || *t.StudentTenantID != ev.TenantID {
		return nil, ErrSanctionTenantMismatch
	}
	return t, nil
}

func (h SanctionLocks) status(ctx context.Context, t *write.LifecycleTarget, to, eventReason string, ev SanctionEvent) error {
	if err := h.Repo.SetStatus(ctx, t.UserID, t.Status, to); err != nil {
		return err
	}
	tenant := ev.TenantID
	if err := h.Repo.RecordStatusChange(ctx, write.StatusChange{UserID: t.UserID, TenantID: &tenant, From: t.Status, To: to,
		Reason: eventReason, Source: "moderation"}); err != nil {
		return err
	}
	return h.Events.Publish(ctx, write.Event{Type: "user.status_changed", SubjectType: "user", SubjectID: t.UserID.String(), TenantID: &tenant,
		Payload: map[string]any{"user_id": t.UserID, "from": t.Status, "to": to, "reason": eventReason}})
}

func (h SanctionLocks) record(ctx context.Context, action string, t *write.LifecycleTarget, ev SanctionEvent, meta map[string]any) error {
	meta["reason_code"], meta["sanction_id"], meta["lock_source"] = ReasonModerationSanction, ev.SanctionID.String(), user.LockModeration
	if ev.CaseID != nil {
		meta["case_id"] = ev.CaseID.String() // staff audit only: never shown to the student (F124-AC-06)
	}
	e := audit.Event{Action: action, TargetType: "user", TargetID: t.UserID.String(), Result: audit.ResultSuccess, Metadata: meta}
	if t.StudentSchoolID != nil {
		e.SchoolID = t.StudentSchoolID.String()
	}
	return h.Audit.Record(ctx, e)
}

// Applied handles sanction.applied (F124-AC-01, AC-03, AC-04).
func (h SanctionLocks) Applied(ctx context.Context, ev SanctionEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SanctionLocks.Applied")
	defer func() { done(err) }()
	if ev.SanctionType != SanctionAccountLock {
		return nil
	}
	t, err := h.target(ctx, ev)
	if err != nil || t == nil {
		return err
	}
	existing, err := h.Locks.BySanction(ctx, ev.SanctionID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // delivered twice, or lifted before applied: never lock again (F124-AC-03)
	}
	tenant, sanction, eventID := ev.TenantID, ev.SanctionID, ev.EventID
	if _, err := h.Locks.Add(ctx, write.AccountLock{TenantID: &tenant, UserID: t.UserID, Source: user.LockModeration, SourceRefID: &sanction,
		CaseID: ev.CaseID, CreatedAt: ev.At, SourceEventID: &eventID}); err != nil {
		return err
	}
	h.metric("applied")
	switch t.Status {
	case "active", "pending":
		if err := h.status(ctx, t, "locked", ReasonModerationSanction, ev); err != nil {
			return err
		}
	case "locked":
		// Already locked by another source: the access is revoked again (idempotent for the consumers), no status change.
	default:
		return h.record(ctx, AuditUserLocked, t, ev, map[string]any{"from": t.Status, "to": t.Status})
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "user.access_revoked", SubjectType: "user", SubjectID: t.UserID.String(), TenantID: &tenant,
		Payload: map[string]any{"user_id": t.UserID, "reason": "locked", "tenant_id": nil}}); err != nil {
		return err
	}
	return h.record(ctx, AuditUserLocked, t, ev, map[string]any{"from": t.Status, "to": "locked"})
}

// Lifted handles sanction.lifted (F124-AC-02, AC-03): the lock of that sanction only; active again when no lock remains.
func (h SanctionLocks) Lifted(ctx context.Context, ev SanctionEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SanctionLocks.Lifted")
	defer func() { done(err) }()
	if ev.SanctionType != SanctionAccountLock {
		return nil
	}
	t, err := h.target(ctx, ev)
	if err != nil || t == nil {
		return err
	}
	tenant, sanction, eventID := ev.TenantID, ev.SanctionID, ev.EventID
	existing, err := h.Locks.BySanction(ctx, ev.SanctionID)
	if err != nil {
		return err
	}
	if existing == nil {
		// Lifted before applied (out of order): a released lock keeps a later sanction.applied from locking (F124-AC-03).
		at := ev.At
		_, err := h.Locks.Add(ctx, write.AccountLock{TenantID: &tenant, UserID: t.UserID, Source: user.LockModeration, SourceRefID: &sanction,
			CaseID: ev.CaseID, CreatedAt: ev.At, ReleasedAt: &at, SourceEventID: &eventID})
		return err
	}
	released, err := h.Locks.ReleaseSanction(ctx, ev.SanctionID, &eventID, ev.At)
	if err != nil || !released {
		return err
	}
	h.metric("lifted")
	remaining, err := h.Locks.ActiveSources(ctx, t.UserID)
	if err != nil {
		return err
	}
	if !user.AfterRelease(remaining).Active || t.Status != "locked" {
		return h.record(ctx, AuditUserLockReleased, t, ev, map[string]any{"status": t.Status, "remaining_locks": len(remaining)})
	}
	if err := h.status(ctx, t, "active", ReasonModerationSanction+"_lifted", ev); err != nil {
		return err
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "user.access_restored", SubjectType: "user", SubjectID: t.UserID.String(), TenantID: &tenant,
		Payload: map[string]any{"user_id": t.UserID, "reason": "unlocked", "tenant_id": nil}}); err != nil {
		return err
	}
	return h.record(ctx, AuditUserUnlocked, t, ev, map[string]any{"from": "locked", "to": "active"})
}
