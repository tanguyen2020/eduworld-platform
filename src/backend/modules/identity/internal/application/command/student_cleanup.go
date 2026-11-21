package command

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Saga types of F016.
const (
	SagaStudentCleanup = "student_cleanup"
	SagaStudentRestore = "student_restore"
)

// CleanupMetrics records eduworld_student_cleanup_total{result} and the large-batch alert.
type CleanupMetrics interface {
	Cleanup(tenant uuid.UUID, result string, n int)
	LargeBatch(tenant uuid.UUID)
}

// StudentCleanup implements the worker student-inactive-cleanup and its approval (F016 US3, ADR-0032,
// ACCOUNT_PROVISIONING §6b): effective days from the tenant/platform settings (0/null = off), notices,
// exclusions, the approval threshold, then one saga per student (participants first, identity last).
type StudentCleanup struct {
	UoW          write.UnitOfWork
	Repo         write.CleanupRepository
	Settings     write.SettingsRepository
	Defaults     settings.Defaults
	Saga         saga.Engine
	Participants []port.StudentDataParticipant
	Lifecycle    write.LifecycleRepository
	Students     write.StudentRepository // student.placement_changed (source lifecycle, ADR-0095); nil = not published
	Events       write.EventPublisher
	Audit        write.Auditor
	Metrics      CleanupMetrics
	Cursors      *page.Codec // preview paging
	Clock        func() time.Time
}

func (h StudentCleanup) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

// cleanupSubject is the subject of a student_cleanup saga.
type cleanupSubject struct {
	StudentID   uuid.UUID  `json:"student_id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	RunID       uuid.UUID  `json:"run_id"`
	CutoffDays  int        `json:"cutoff_days"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

func tenantCtx(ctx context.Context, tenant uuid.UUID) (context.Context, error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(ctx, sc), nil
}

// Policy returns the effective cleanup days of a tenant (nil = off) and the platform policy.
func (h StudentCleanup) Policy(ctx context.Context, tenant uuid.UUID) (days *int, p settings.StudentCleanup, err error) {
	ps, err := h.Settings.Platform(ctx, settings.KeyStudentCleanup)
	if err != nil {
		return nil, p, err
	}
	pc, err := dto.StudentCleanupOf(h.Defaults, ps.Value, ps.Version)
	if err != nil {
		return nil, p, err
	}
	ts, found, err := h.Settings.Tenant(ctx, tenant)
	if err != nil || !found {
		return nil, pc.StudentCleanup, err
	}
	t, err := dto.TenantSettingsOf(ts.Value, ts.Version, pc.StudentCleanup)
	if err != nil {
		return nil, pc.StudentCleanup, err
	}
	if t.EffectiveCleanupDays == nil || *t.EffectiveCleanupDays <= 0 {
		return nil, pc.StudentCleanup, nil
	}
	return t.EffectiveCleanupDays, pc.StudentCleanup, nil
}

// Definition is the student_cleanup saga (also used by the resume job).
func (h StudentCleanup) Definition() saga.Definition {
	subj := func(run write.SagaRun) (cleanupSubject, error) {
		var s cleanupSubject
		return s, json.Unmarshal(run.Subject, &s)
	}
	def := saga.Definition{Type: SagaStudentCleanup}
	for _, p := range h.Participants {
		p := p
		def.Steps = append(def.Steps, saga.Step{Service: p.Name(), Action: "cleanupStudentData",
			Do: func(ctx context.Context, run write.SagaRun) (saga.Result, error) {
				s, err := subj(run)
				if err != nil {
					return saga.Result{}, problem.New(problem.ValidationFailed, "saga subject").Wrap(err)
				}
				if ctx, err = tenantCtx(ctx, s.TenantID); err != nil {
					return saga.Result{}, err
				}
				r, err := p.Cleanup(ctx, s.StudentID, s.RunID, s.LastLoginAt)
				return saga.Result{Outcome: r.Status, Response: map[string]any{"status": r.Status, "blocked_reason": r.BlockedReason}}, err
			},
			Undo: func(ctx context.Context, run write.SagaRun) error {
				s, err := subj(run)
				if err != nil {
					return err
				}
				if ctx, err = tenantCtx(ctx, s.TenantID); err != nil {
					return err
				}
				_, err = p.Restore(ctx, s.StudentID, false)
				return err
			}})
	}
	def.Finish = func(ctx context.Context, run write.SagaRun) error {
		s, err := subj(run)
		if err != nil {
			return problem.New(problem.ValidationFailed, "saga subject").Wrap(err)
		}
		tctx, err := tenantCtx(ctx, s.TenantID)
		if err != nil {
			return err
		}
		return h.UoW.Do(tctx, func(ctx context.Context) error {
			var tr *placementTracker
			if h.Students != nil {
				if tr, err = trackPlacement(ctx, h.Students, h.Events, s.StudentID); err != nil {
					return err
				}
			}
			removed, err := h.Repo.SoftDeleteStudent(ctx, s.StudentID)
			if err != nil || !removed {
				return err
			}
			if tr != nil {
				if err := tr.done(ctx, PlacementChange{Source: placement.SourceLifecycle}); err != nil {
					return err
				}
			}
			tenant := s.TenantID
			if err := h.Lifecycle.RecordStatusChange(ctx, write.StatusChange{UserID: s.StudentID, TenantID: &tenant, From: "active",
				To: "cleaned_up", Source: "cleanup"}); err != nil {
				return err
			}
			var last any
			if s.LastLoginAt != nil {
				last = s.LastLoginAt.UTC().Format(time.RFC3339)
			}
			if err := h.Events.Publish(ctx, write.Event{Type: "student.cleaned_up", SubjectType: "student", SubjectID: s.StudentID.String(),
				TenantID: &tenant, Payload: map[string]any{"student_id": s.StudentID, "tenant_id": tenant, "last_login_at": last,
					"cutoff_days": s.CutoffDays, "run_id": s.RunID}}); err != nil {
				return err
			}
			if err := h.Events.Publish(ctx, write.Event{Type: "tenant_membership.changed", SubjectType: "user", SubjectID: s.StudentID.String(),
				TenantID: &tenant, Payload: map[string]any{"user_id": s.StudentID, "tenant_id": tenant, "op": "removed",
					"source": "inactive_cleanup", "user_type": "student"}}); err != nil {
				return err
			}
			return h.Audit.Record(ctx, audit.Event{Action: "student.cleaned_up", TargetType: "student", TargetID: s.StudentID.String(),
				Result: audit.ResultSuccess, Reason: "inactive_cleanup", Metadata: map[string]any{"run_id": s.RunID.String(), "cutoff_days": s.CutoffDays}})
		})
	}
	return def
}

// RunTenant runs the cleanup of one tenant: notices, candidates, approval threshold, sagas. An approved run
// waiting for the tenant is applied first. Returns the run (nil when the cleanup is off).
func (h StudentCleanup) RunTenant(ctx context.Context, tenant uuid.UUID) (out *write.CleanupRun, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RunStudentCleanup")
	defer func() { done(err) }()
	tctx, err := tenantCtx(ctx, tenant)
	if err != nil {
		return nil, err
	}
	var (
		days     *int
		policy   settings.StudentCleanup
		cands    []write.CleanupCandidate
		notices  []write.CleanupCandidate
		total    int
		approved *write.CleanupRun
	)
	now := h.now()
	if err := h.UoW.Do(tctx, func(ctx context.Context) error {
		if days, policy, err = h.Policy(ctx, tenant); err != nil || days == nil {
			return err
		}
		if cands, err = h.Repo.Candidates(ctx, now.AddDate(0, 0, -*days), now, nil); err != nil {
			return err
		}
		if notices, err = h.Repo.Candidates(ctx, now.AddDate(0, 0, policy.NoticeDays-*days), now, nil); err != nil {
			return err
		}
		total, err = h.Repo.StudentCount(ctx)
		return err
	}); err != nil || days == nil {
		return nil, err
	}
	if err := h.platform(ctx, func(ctx context.Context) error {
		approved, err = h.Repo.ApprovedRun(ctx, tenant)
		return err
	}); err != nil {
		return nil, err
	}
	if err := h.notify(tctx, tenant, *days, cands, notices); err != nil {
		return nil, err
	}
	run := approved
	if run == nil {
		run = &write.CleanupRun{ID: uuid.Must(uuid.NewV7()), TenantID: tenant, Mode: "apply", Status: "running", CutoffDays: *days,
			Candidates: len(cands), StartedAt: now}
		if err := h.platform(ctx, func(ctx context.Context) error { return h.Repo.CreateRun(ctx, run) }); err != nil {
			return nil, err
		}
		limit := int(math.Ceil(float64(total) * float64(policy.ApprovalThresholdPercent) / 100))
		if len(cands) > 0 && len(cands) > limit {
			run.Status = "awaiting_approval"
			if h.Metrics != nil {
				h.Metrics.LargeBatch(tenant)
			}
			return run, h.platform(ctx, func(ctx context.Context) error { return h.Repo.SaveRun(ctx, run) })
		}
	}
	run.Candidates = len(cands)
	for _, c := range cands {
		subject := cleanupSubject{StudentID: c.StudentID, TenantID: tenant, RunID: run.ID, CutoffDays: *days, LastLoginAt: c.LastLoginAt}
		o, err := h.Saga.Start(ctx, h.Definition(), &tenant, subject, run.ID.String()+":"+c.StudentID.String(), nil)
		switch {
		case err != nil:
			return run, err
		case o.Status == saga.StatusSucceeded:
			run.Cleaned++
		case o.Status == saga.StatusBlocked:
			run.SkippedActive++
		}
	}
	if h.Metrics != nil {
		h.Metrics.Cleanup(tenant, "cleaned", run.Cleaned)
		h.Metrics.Cleanup(tenant, "skipped_active", run.SkippedActive)
	}
	finished := h.now()
	run.Status, run.FinishedAt = "completed", &finished
	return run, h.platform(ctx, func(ctx context.Context) error { return h.Repo.SaveRun(ctx, run) })
}

// notify publishes student.cleanup_scheduled for the students entering the notice window (not yet due).
func (h StudentCleanup) notify(ctx context.Context, tenant uuid.UUID, days int, due, window []write.CleanupCandidate) error {
	isDue := map[uuid.UUID]bool{}
	for _, c := range due {
		isDue[c.StudentID] = true
	}
	byDate := map[string][]uuid.UUID{}
	for _, c := range window {
		if isDue[c.StudentID] {
			continue
		}
		d := c.DueAt(days).UTC().Truncate(24 * time.Hour).Format(time.RFC3339)
		byDate[d] = append(byDate[d], c.StudentID)
	}
	if len(byDate) == 0 {
		return nil
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		for d, ids := range byDate {
			for i := 0; i < len(ids); i += 500 {
				end := min(i+500, len(ids))
				if err := h.Events.Publish(ctx, write.Event{Type: "student.cleanup_scheduled", SubjectType: "tenant", SubjectID: tenant.String(),
					TenantID: &tenant, Payload: map[string]any{"tenant_id": tenant, "student_ids": ids[i:end], "scheduled_cleanup_at": d,
						"cutoff_days": days}}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (h StudentCleanup) platform(ctx context.Context, fn func(ctx context.Context) error) error {
	sc, err := authz.SystemPlatformScope("student-inactive-cleanup")
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

// RunAll is the job student-inactive-cleanup over every active tenant.
func (h StudentCleanup) RunAll(ctx context.Context) (int, error) {
	var tenants []uuid.UUID
	if err := h.platform(ctx, func(ctx context.Context) error {
		var err error
		tenants, err = h.Repo.CleanupTenants(ctx)
		return err
	}); err != nil {
		return 0, err
	}
	n := 0
	for _, t := range tenants {
		run, err := h.RunTenant(ctx, t)
		if err != nil {
			return n, fmt.Errorf("tenant %s: %w", t, err)
		}
		if run != nil {
			n += run.Cleaned
		}
	}
	return n, nil
}

// Approve implements approveStudentCleanupRun (platform.student_cleanup.approve, F016-AC-12): the run moves
// to approved; the next job tick applies it.
func (h StudentCleanup) Approve(ctx context.Context, runID uuid.UUID) (out write.CleanupRun, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ApproveStudentCleanupRun")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if !p.IsSystemAdmin && p.TenantID != nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformStudentCleanupApprove, authz.Resource{}); err != nil {
		return out, err
	}
	err = h.platform(ctx, func(ctx context.Context) error {
		run, err := h.Repo.Run(ctx, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return problem.New(problem.NotFound, "")
		}
		if run.Status != "awaiting_approval" {
			return problem.New(problem.Conflict, "the run is not awaiting approval")
		}
		by := p.UserID
		run.Status, run.ApprovedBy = "approved", &by
		if err := h.Repo.SaveRun(ctx, run); err != nil {
			return err
		}
		out = *run
		return h.Audit.Record(ctx, audit.Event{Action: "student_cleanup.approved", TargetType: "student_cleanup_run", TargetID: runID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"tenant_id": run.TenantID.String(), "candidates": run.Candidates}})
	})
	return out, err
}

// Cancel implements cancelStudentCleanupRun (platform.student_cleanup.approve, OQ F057-2): a run awaiting
// approval moves to cancelled with the reason in the audit; no student is cleaned up. A later job tick starts a
// fresh run from the settings in force.
func (h StudentCleanup) Cancel(ctx context.Context, runID uuid.UUID, reason string) (out write.CleanupRun, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CancelStudentCleanupRun")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if !p.IsSystemAdmin && p.TenantID != nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformStudentCleanupApprove, authz.Resource{}); err != nil {
		return out, err
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(reason)); n < 3 || n > 500 {
		return out, problem.Invalid("reason", problem.SubInvalid, "must be 3 to 500 characters")
	}
	err = h.platform(ctx, func(ctx context.Context) error {
		run, err := h.Repo.Run(ctx, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return problem.New(problem.NotFound, "")
		}
		if run.Status != "awaiting_approval" {
			return problem.New(problem.Conflict, "the run is not awaiting approval")
		}
		finished := h.now()
		run.Status, run.FinishedAt = "cancelled", &finished
		if err := h.Repo.SaveRun(ctx, run); err != nil {
			return err
		}
		out = *run
		return h.Audit.Record(ctx, audit.Event{Action: "student_cleanup.cancelled", TargetType: "student_cleanup_run", TargetID: runID.String(),
			Result: audit.ResultSuccess, Reason: strings.TrimSpace(reason),
			Metadata: map[string]any{"tenant_id": run.TenantID.String(), "candidates": run.Candidates}})
	})
	return out, err
}
