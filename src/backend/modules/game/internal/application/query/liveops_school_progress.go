package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SchoolGoalMilestoneView is one milestone of the goal (percent of the target, reached in the scope).
type SchoolGoalMilestoneView struct {
	Pct     int  `json:"pct"`
	Reached bool `json:"reached"`
}

// SchoolGoalView is SchoolEventGoalProgress: the total and milestones of the goal in the scope viewed.
type SchoolGoalView struct {
	GoalID     uuid.UUID                 `json:"goal_id"`
	Metric     string                    `json:"metric"`
	Target     int64                     `json:"target"`
	Total      int64                     `json:"total"`
	Milestones []SchoolGoalMilestoneView `json:"milestones"`
}

// GradeContributionView is one grade of SchoolEventProgress.by_grade (a total and a count of students, never a student).
type GradeContributionView struct {
	GradeLevel        int   `json:"grade_level"`
	ContributionTotal int64 `json:"contribution_total"`
	ContributorsCount int   `json:"contributors_count"`
}

// SchoolEventProgressView is SchoolEventProgress (F081-AC-10): aggregates only — no student id, name, nickname or rank
// (R16).
type SchoolEventProgressView struct {
	EventID           uuid.UUID               `json:"event_id"`
	EventRunID        uuid.UUID               `json:"event_run_id"`
	Status            string                  `json:"status"`
	SchoolID          *uuid.UUID              `json:"school_id"`
	ParticipantsCount int                     `json:"participants_count"`
	CommunityGoal     *SchoolGoalView         `json:"community_goal"`
	ByGrade           []GradeContributionView `json:"by_grade"`
	DataAsOf          time.Time               `json:"data_as_of"`
}

// GetSchoolEventProgress is getSchoolEventProgress (F081-AC-10, AC-12): lms.school_event.write on the tenant or a school
// (403 + authz.denied without it); an event of another school or tenant, or a school outside the audience or the scope of
// the caller, is 404. The school viewed is `school` or the only school of the event; a school-scoped goal of an event of
// several schools needs one (422 on school_id). Totals and milestones come from community_goal_progress (PostgreSQL),
// the grades from the contributions through liveops.VisibleGrades (grades 4–9, ≥ analytics_min_cell_size students) when
// the event shows them (show_grade_contributions, EVENT_SEASON_SYSTEM §6).
func (h LiveOps) GetSchoolEventProgress(ctx context.Context, event uuid.UUID, school *uuid.UUID) (out SchoolEventProgressView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetSchoolEventProgress")
	defer func() { done(err) }()
	p, tenant, err := staffTenant(ctx)
	if err != nil {
		return out, err
	}
	schools, err := requireSchoolEvents(ctx, p, tenant)
	if err != nil {
		return out, err
	}
	minCell := h.Policy.MinCell
	if minCell <= 0 {
		minCell = 5
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		r, err := h.Data.SchoolEvent(ctx, tenant, event)
		if err != nil {
			return err
		}
		if r == nil || r.Run == nil || (schools != nil && (r.SchoolID == nil || !slices.Contains(schools, *r.SchoolID))) {
			return problem.New(problem.NotFound, "")
		}
		run := r.Run
		viewed, err := h.viewedSchool(ctx, tenant, schools, r.SchoolID, run.Audience.SchoolIDs, school)
		if err != nil {
			return err
		}
		g, err := h.Data.RunGoal(ctx, run.RunID)
		if err != nil {
			return err
		}
		f := read.SchoolEventProgressFilter{Tenant: tenant, Run: run.RunID, School: viewed}
		if g != nil {
			f.Goal, f.ScopeID = &g.GoalID, tenant
			if g.Scope == liveops.ScopeSchool {
				if viewed == nil {
					return problem.Invalid("school_id", problem.SubRequired, "the event counts per school: pick one of its schools")
				}
				f.ScopeID = *viewed
			}
		}
		row, err := h.Data.SchoolEventProgress(ctx, f)
		if err != nil {
			return err
		}
		out = SchoolEventProgressView{EventID: r.EventID, EventRunID: run.RunID, Status: run.Status, SchoolID: viewed, ParticipantsCount: row.Participants,
			ByGrade: []GradeContributionView{}, DataAsOf: h.now()}
		if row.UpdatedAt != nil {
			out.DataAsOf = *row.UpdatedAt
		}
		if g != nil {
			v := SchoolGoalView{GoalID: g.GoalID, Metric: g.Metric, Target: g.Target, Total: row.Total, Milestones: []SchoolGoalMilestoneView{}}
			for _, m := range g.Milestones {
				v.Milestones = append(v.Milestones, SchoolGoalMilestoneView{Pct: m.Percent, Reached: slices.Contains(row.Reached, m.Percent)})
			}
			slices.SortFunc(v.Milestones, func(a, b SchoolGoalMilestoneView) int { return a.Pct - b.Pct })
			out.CommunityGoal = &v
		}
		if g == nil || !g.ShowGrades {
			return nil // grades only when the event shows them (show_grade_contributions — EVENT_SEASON_SYSTEM §6)
		}
		for _, gr := range liveops.VisibleGrades(row.Grades, minCell) {
			out.ByGrade = append(out.ByGrade, GradeContributionView{GradeLevel: gr.Grade, ContributionTotal: gr.Progress, ContributorsCount: gr.Contributors})
		}
		return nil
	})
	return out, err
}

// viewedSchool resolves the school of the progress: the one asked (within the audience — or a school of the tenant for
// a tenant-wide event — and within the caller's schools, else 404), otherwise the school of the event or its only
// audience school (nil = none: the whole tenant).
func (h LiveOps) viewedSchool(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID, owner *uuid.UUID, audience []uuid.UUID,
	asked *uuid.UUID) (*uuid.UUID, error) {
	if asked == nil {
		if owner != nil {
			return owner, nil
		}
		if len(audience) == 1 {
			s := audience[0]
			return &s, nil
		}
		return nil, nil
	}
	if schools != nil && !slices.Contains(schools, *asked) {
		return nil, problem.New(problem.NotFound, "")
	}
	if len(audience) > 0 {
		if !slices.Contains(audience, *asked) {
			return nil, problem.New(problem.NotFound, "")
		}
		return asked, nil
	}
	n, err := h.Data.KnownSchools(ctx, tenant, []uuid.UUID{*asked})
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, problem.New(problem.NotFound, "")
	}
	return asked, nil
}

// crossTenantEvents resolves the tenants System Admin reads in listSchoolEvents without X-Tenant-Id (F081-AC-06): the
// tenants of the CrossTenantScope (tenant_ids), or every tenant under the audited bypass of the closed list (tenant_ids
// absent); other platform roles 403.
func crossTenantEvents(ctx context.Context, p authz.Principal) (context.Context, read.SchoolEventsFilter, error) {
	var f read.SchoolEventsFilter
	if err := authz.Require(ctx, p, authz.PermPlatformCrossTenantRead, authz.Resource{}); err != nil {
		return ctx, f, err
	}
	sc, ok := tenancy.ScopeFrom(ctx)
	if !ok || !p.IsSystemAdmin {
		return ctx, f, problem.New(problem.PermissionDenied, "")
	}
	switch s := sc.(type) {
	case tenancy.CrossTenantScope:
		f.Tenants = s.TenantIDs()
	case tenancy.PlatformScope:
		var err error
		if ctx, err = tenancy.Bypass(tenancy.WithModule(ctx, "game"), tenancy.ReasonSystemAdminCrossTenant, tenancy.WithMode(tenancy.ModeRead)); err != nil {
			return ctx, f, err
		}
		f.AllTenants = true
	default:
		return ctx, f, problem.New(problem.PermissionDenied, "")
	}
	return ctx, f, nil
}
