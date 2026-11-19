package command

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Ways a cleaned-up student comes back (student.restored.restored_via).
const (
	RestoredViaLogin = "login"
	RestoredViaAdmin = "admin"
)

// restoreJob is the platform scope of the login-time lookups of the restore (no tenant yet).
const restoreJob = "student-restore"

// StudentRestore implements the restore of a student removed by the inactive cleanup (F016 US4, FR-AUTH-024,
// ACCOUNT_PROVISIONING §6b): saga student_restore (restoreStudentData of every participant, then identity
// clears the inactive_cleanup soft-delete, adds the tenant membership back and publishes student.restored).
// The Keycloak Organization membership comes back through tenant_membership.changed (F010 worker).
type StudentRestore struct {
	UoW          write.UnitOfWork
	Repo         write.CleanupRepository
	Saga         saga.Engine
	Participants []port.StudentDataParticipant
	Lifecycle    write.LifecycleRepository
	Events       write.EventPublisher
	Audit        write.Auditor
}

// StudentRestorer is the trash-side port of F045 (restoreTenantTrashItem, resource_type student).
var _ interface {
	RestoreStudentByAdmin(ctx context.Context, tenantID, studentID uuid.UUID) error
} = StudentRestore{}

// restoreSubject is the subject of a student_restore saga.
type restoreSubject struct {
	StudentID   uuid.UUID `json:"student_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	CleanedUpAt time.Time `json:"cleaned_up_at"`
	Via         string    `json:"restored_via"`
}

// Definition is the student_restore saga (also used by the resume job).
func (h StudentRestore) Definition() saga.Definition {
	subj := func(run write.SagaRun) (restoreSubject, error) {
		var s restoreSubject
		return s, json.Unmarshal(run.Subject, &s)
	}
	def := saga.Definition{Type: SagaStudentRestore}
	for _, p := range h.Participants {
		p := p
		def.Steps = append(def.Steps, saga.Step{Service: p.Name(), Action: "restoreStudentData",
			Do: func(ctx context.Context, run write.SagaRun) (saga.Result, error) {
				s, err := subj(run)
				if err != nil {
					return saga.Result{}, problem.New(problem.ValidationFailed, "saga subject").Wrap(err)
				}
				if ctx, err = tenantCtx(ctx, s.TenantID); err != nil {
					return saga.Result{}, err
				}
				r, err := p.Restore(ctx, s.StudentID, s.Via == RestoredViaLogin)
				return saga.Result{Outcome: r.Status, Response: map[string]any{"status": r.Status}}, err
			}})
	}
	// Identity is restored before the saga starts (restoreIdentity); nothing is left for the end of the saga.
	def.Finish = func(context.Context, write.SagaRun) error { return nil }
	return def
}

// restoreIdentity clears the inactive_cleanup soft-delete of the student in one transaction of its tenant and
// publishes student.restored + tenant_membership.changed (added); false when it was already restored.
func (h StudentRestore) restoreIdentity(ctx context.Context, tenant, student uuid.UUID, via string, by *uuid.UUID) (bool, error) {
	tctx, err := tenantCtx(ctx, tenant)
	if err != nil {
		return false, err
	}
	restored := false
	err = h.UoW.Do(tctx, func(ctx context.Context) error {
		at, err := h.Repo.RestoreStudent(ctx, student)
		if err != nil || at == nil {
			return err // already restored: idempotent
		}
		restored = true
		if err := h.Lifecycle.RecordStatusChange(ctx, write.StatusChange{UserID: student, TenantID: &tenant, From: "cleaned_up",
			To: "active", Source: "restore", Reason: "restored_via_" + via, By: by}); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: "student.restored", SubjectType: "student", SubjectID: student.String(),
			TenantID: &tenant, Payload: map[string]any{"student_id": student, "tenant_id": tenant,
				"cleaned_up_at": at.UTC().Format(time.RFC3339), "restored_via": via}}); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: "tenant_membership.changed", SubjectType: "user", SubjectID: student.String(),
			TenantID: &tenant, Payload: map[string]any{"user_id": student, "tenant_id": tenant, "op": "added",
				"source": "inactive_cleanup_restore", "user_type": "student"}}); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: "student.restored", TargetType: "student", TargetID: student.String(),
			Result: audit.ResultSuccess, Reason: "inactive_cleanup_restore", Metadata: map[string]any{"restored_via": via}})
	})
	return restored, err
}

// run restores identity, then starts (or joins) the restore saga over the participants. The idempotency key
// carries the cleanup time, so concurrent logins share one run and a later cleanup gets a new one. A
// participant that is temporarily unavailable does not fail the restore (OQ F016-13): the saga-retry job
// resumes the run and every module also reacts to student.restored.
func (h StudentRestore) run(ctx context.Context, tenant, student uuid.UUID, cleanedAt time.Time, via string, by *uuid.UUID) error {
	if _, err := h.restoreIdentity(ctx, tenant, student, via, by); err != nil {
		return err
	}
	subject := restoreSubject{StudentID: student, TenantID: tenant, CleanedUpAt: cleanedAt.UTC(), Via: via}
	key := fmt.Sprintf("restore:%s:%d", student, cleanedAt.UnixMicro())
	o, err := h.Saga.Start(ctx, h.Definition(), &tenant, subject, key, by)
	if err != nil || o.Status != saga.StatusSucceeded {
		slog.WarnContext(ctx, "student restore saga not finished", "saga_status", o.Status, "error", err)
	}
	return nil
}

// RestoreStudentByAdmin restores a cleaned-up student from the tenant trash (F016-AC-19). The caller
// (F045 restoreTenantTrashItem) has authorized tenant.trash.restore, set the TenantScope and writes the
// trash audit; a student of another tenant, or not removed by the cleanup, is 404. [Proposed] The SIS
// standing is not checked here: the Tenant Admin decides (OQ F016-9).
func (h StudentRestore) RestoreStudentByAdmin(ctx context.Context, tenantID, studentID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RestoreStudentByAdmin")
	defer func() { done(err) }()
	var tenant *uuid.UUID
	var at *time.Time
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		tenant, at, err = h.Repo.CleanedStudent(ctx, studentID)
		return err
	}); err != nil {
		return err
	}
	if tenant == nil || *tenant != tenantID {
		return problem.New(problem.NotFound, "")
	}
	var by *uuid.UUID
	if p, ok := authz.PrincipalFrom(ctx); ok {
		id := p.UserID
		by = &id
	}
	return h.run(ctx, tenantID, studentID, *at, RestoredViaAdmin, by)
}

// RestoreOnLogin restores a cleaned-up student whose Keycloak login succeeded (F016-AC-17, AC-18): the
// student must still be valid in the SIS (active record of the active identity link; without a SIS link —
// common tenant or manual account — no SIS check). restored is false when the user is not a cleaned-up
// student; an invalid SIS record is 403 ACCOUNT_DEACTIVATED.
func (h StudentRestore) RestoreOnLogin(ctx context.Context, userID uuid.UUID) (restored bool, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RestoreStudentOnLogin")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope(restoreJob)
	if err != nil {
		return false, err
	}
	pctx := tenancy.WithScope(ctx, sc)
	var tenant *uuid.UUID
	var at *time.Time
	var linked, valid bool
	if err := h.UoW.Do(pctx, func(ctx context.Context) error {
		if tenant, at, err = h.Repo.CleanedStudent(ctx, userID); err != nil || tenant == nil {
			return err
		}
		linked, valid, err = h.Repo.SisStanding(ctx, userID)
		return err
	}); err != nil {
		return false, err
	}
	if tenant == nil {
		return false, nil
	}
	if linked && !valid {
		h.deny(pctx, *tenant, userID)
		return false, problem.New(problem.AccountDeactivated, "")
	}
	return true, h.run(ctx, *tenant, userID, *at, RestoredViaLogin, nil)
}

func (h StudentRestore) deny(ctx context.Context, tenant, userID uuid.UUID) {
	tctx, err := tenantCtx(ctx, tenant)
	if err != nil {
		return
	}
	_ = h.UoW.Do(tctx, func(ctx context.Context) error {
		return h.Audit.Record(ctx, audit.Event{Action: "student.restore_denied", TargetType: "student", TargetID: userID.String(),
			Result: audit.ResultDenied, Reason: string(problem.AccountDeactivated), Metadata: map[string]any{"via": RestoredViaLogin}})
	})
}
