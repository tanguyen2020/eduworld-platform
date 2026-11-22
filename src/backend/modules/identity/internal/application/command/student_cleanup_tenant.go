package command

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// CleanupCandidateView is StudentCleanupCandidate.
type CleanupCandidateView struct {
	StudentID          uuid.UUID  `json:"student_id"`
	DisplayName        string     `json:"display_name"`
	SchoolID           uuid.UUID  `json:"school_id"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	ProvisionedAt      time.Time  `json:"provisioned_at"`
	ScheduledCleanupAt time.Time  `json:"scheduled_cleanup_at"`
	ExcludedUntil      *time.Time `json:"excluded_until"`
}

// CleanupCandidatePage is StudentCleanupCandidatePage.
type CleanupCandidatePage struct {
	Data []CleanupCandidateView `json:"data"`
	Page page.Info              `json:"page"`
}

// cleanupTenantScope returns the tenant scope and the schools the principal is limited to (nil = every school)
// for perm (F016-AC-16: School Admin sees their school only).
func cleanupTenantScope(ctx context.Context, perm authz.Permission) (context.Context, authz.Principal, []uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, nil, err
	}
	if p.TenantID == nil || !p.Has(perm) {
		return ctx, p, nil, problem.New(problem.PermissionDenied, "")
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, p, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	var schools []uuid.UUID
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, perm) {
			continue
		}
		if g.SchoolID == nil {
			return tenancy.WithScope(ctx, sc), p, nil, nil
		}
		schools = append(schools, *g.SchoolID)
	}
	return tenancy.WithScope(ctx, sc), p, schools, nil
}

// Preview implements previewTenantStudentCleanup: the students already due or entering the notice window,
// with their scheduled date (tenant of the session; School Admin limited to their schools).
func (h StudentCleanup) Preview(ctx context.Context, limit int, cursor *page.Cursor) (out CleanupCandidatePage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.PreviewTenantStudentCleanup")
	defer func() { done(err) }()
	ctx, p, schools, err := cleanupTenantScope(ctx, authz.PermTenantStudentCleanupRead)
	if err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	out.Page.Limit = limit
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		days, policy, err := h.Policy(ctx, *p.TenantID)
		if err != nil || days == nil {
			return err
		}
		q := write.CandidatePageQuery{DueBefore: now.AddDate(0, 0, policy.NoticeDays-*days), Now: now, Schools: schools, Limit: limit + 1}
		if cursor != nil {
			at, err := time.Parse(time.RFC3339Nano, cursor.Key)
			if err != nil {
				return problem.New(problem.BadRequest, "invalid cursor")
			}
			base := at.AddDate(0, 0, -*days) // the cursor carries the scheduled date = base + days
			q.AfterBase, q.AfterID = &base, cursor.ID
		}
		cands, err := h.Repo.CandidatePage(ctx, q)
		if err != nil {
			return err
		}
		for _, c := range cands {
			if len(out.Data) == limit {
				last := out.Data[limit-1]
				if h.Cursors != nil {
					next := h.Cursors.Encode(page.Cursor{Key: last.ScheduledCleanupAt.Format(time.RFC3339Nano), ID: last.StudentID})
					out.Page.NextCursor = &next
				}
				out.Page.HasMore = true
				break
			}
			out.Data = append(out.Data, CleanupCandidateView{StudentID: c.StudentID, DisplayName: c.DisplayName, SchoolID: c.SchoolID,
				LastLoginAt: c.LastLoginAt, ProvisionedAt: c.ProvisionedAt, ScheduledCleanupAt: c.DueAt(*days).UTC()})
		}
		return nil
	})
	if out.Data == nil {
		out.Data = []CleanupCandidateView{}
	}
	return out, err
}

// Exclude implements createStudentCleanupExclusions (tenant.student_cleanup.exclude, reason required,
// F016-AC-11) and Unexclude deleteStudentCleanupExclusions (soft-delete of the exclusions).
func (h StudentCleanup) Exclude(ctx context.Context, students []uuid.UUID, until *time.Time, reason string, remove bool) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.StudentCleanupExclusions")
	defer func() { done(err) }()
	if n := len(strings.TrimSpace(reason)); n < 3 || n > 500 {
		return problem.Invalid("reason", problem.SubInvalid, "3..500 characters")
	}
	if len(students) == 0 || len(students) > 500 {
		return problem.Invalid("student_ids", problem.SubInvalid, "1..500 students")
	}
	ctx, p, schools, err := cleanupTenantScope(ctx, authz.PermTenantStudentCleanupExclude)
	if err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		for _, id := range students {
			school, err := h.Repo.StudentSchool(ctx, id)
			if err != nil {
				return err
			}
			if school == nil {
				return problem.New(problem.NotFound, "")
			}
			if schools != nil && !slices.Contains(schools, *school) {
				return problem.New(problem.PermissionDenied, "")
			}
			action := "student_cleanup.excluded"
			if remove {
				action = "student_cleanup.exclusion_removed"
				if _, err := h.Repo.RemoveExclusion(ctx, id, p.UserID); err != nil {
					return err
				}
			} else if _, err := h.Repo.AddExclusion(ctx, write.CleanupExclusion{ID: uuid.Must(uuid.NewV7()), TenantID: *p.TenantID, StudentID: id,
				SchoolID: *school, Until: until, Reason: reason, CreatedBy: p.UserID}); err != nil {
				return err
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "student", TargetID: id.String(), Result: audit.ResultSuccess,
				Reason: reason, SchoolID: school.String()}); err != nil {
				return err
			}
		}
		return nil
	})
}

// CleanupRunPage is one cursor page of listStudentCleanupRuns.
type CleanupRunPage struct {
	Runs []write.CleanupRun
	Page page.Info
}

// ListRuns implements listStudentCleanupRuns (System Admin: tenant_ids of the CrossTenantScope, or every tenant), keyset
// paginated (started_at DESC, run_id DESC; API_DESIGN §5).
func (h StudentCleanup) ListRuns(ctx context.Context, limit int, cursor *page.Cursor) (out CleanupRunPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ListStudentCleanupRuns")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID != nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformStudentCleanupApprove, authz.Resource{}); err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	out.Page.Limit = limit
	var after *write.CleanupRunKey
	if cursor != nil {
		at, err := time.Parse(time.RFC3339Nano, cursor.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		after = &write.CleanupRunKey{StartedAt: at, ID: cursor.ID}
	}
	var tenants []uuid.UUID
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if cs, ok := s.(tenancy.CrossTenantScope); ok {
			tenants = cs.TenantIDs()
		}
	}
	var rows []write.CleanupRun
	err = h.platform(ctx, func(ctx context.Context) error {
		rows, err = h.Repo.ListRuns(ctx, tenants, "", after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	if len(rows) > limit {
		rows = rows[:limit]
		out.Page.HasMore = true
		if h.Cursors != nil {
			last := rows[limit-1]
			next := h.Cursors.Encode(page.Cursor{Key: last.StartedAt.UTC().Format(time.RFC3339Nano), ID: last.ID})
			out.Page.NextCursor = &next
		}
	}
	out.Runs = rows
	return out, nil
}
