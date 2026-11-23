package command

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// playerOf returns the student principal, its tenant and its projection (school, grade).
func (h *LiveOps) playerOf(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return uuid.Nil, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return p.UserID, *p.TenantID, nil
}

func tenantScope(ctx context.Context, tenant uuid.UUID) (tenancy.Scope, error) {
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		return s, nil
	}
	return authz.EventScope(tenant)
}

// visibleRun returns the run of event a student sees (F077-AC-07): 404 EVENT_NOT_AVAILABLE outside its audience or
// status window.
func (h *LiveOps) visibleRun(ctx context.Context, tenant, student, event uuid.UUID, now time.Time) (*port.EventRunRow, *port.WorldStudent, error) {
	st, err := h.Students.Student(ctx, tenant, student)
	if err != nil {
		return nil, nil, err
	}
	if st == nil || st.Deleted || st.Status != "active" {
		return nil, nil, problem.New(problem.EventNotAvailable, "")
	}
	r, err := h.Store.EventRun(ctx, tenant, event)
	if err != nil {
		return nil, nil, err
	}
	if r == nil || !r.Audience.Includes(r.TenantID, tenant, st.SchoolID, st.Grade) || !liveops.Visible(r.Run(), now, h.policy().UpcomingVisible) {
		return nil, nil, problem.New(problem.EventNotAvailable, "")
	}
	return r, st, nil
}

// JoinResult is EventJoinResult.
type JoinResult struct {
	EventRunID  uuid.UUID `json:"event_run_id"`
	JoinedAt    time.Time `json:"joined_at"`
	EventPoints int       `json:"event_points"`
}

// JoinEvent is joinGameEvent (F077-AC-08): one participation per student and run (event.participation_started once);
// an event that ended takes no new participant (409 EVENT_ENDED); a scheduled or suspended event is not open (409).
func (h *LiveOps) JoinEvent(ctx context.Context, event uuid.UUID) (out JoinResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.JoinGameEvent")
	defer func() { done(err) }()
	student, tenant, err := h.playerOf(ctx)
	if err != nil {
		return out, err
	}
	sc, err := tenantScope(ctx, tenant)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		r, st, err := h.visibleRun(ctx, tenant, student, event, now)
		if err != nil {
			return err
		}
		switch r.Status {
		case liveops.StatusEnded:
			return problem.New(problem.EventEnded, "")
		case liveops.StatusLive:
		default:
			return problem.New(problem.Conflict, "the event is not open")
		}
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		p, err := h.participate(ctx, *r, tenant, student, st.SchoolID, now)
		if err != nil {
			return err
		}
		out = JoinResult{EventRunID: r.RunID, JoinedAt: p.JoinedAt, EventPoints: p.Points}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// participate returns the participation of a student, created once with event.participation_started and the
// participation rewards (granted once per run and student).
func (h *LiveOps) participate(ctx context.Context, r port.EventRunRow, tenant, student, school uuid.UUID, now time.Time) (*port.ParticipationRow, error) {
	p, err := h.Store.LockParticipation(ctx, tenant, r.RunID, student)
	if err != nil || p != nil {
		return p, err
	}
	row := port.ParticipationRow{ID: ids.New(), TenantID: tenant, RunID: r.RunID, StudentID: student, SchoolID: school, JoinedAt: now,
		Reached: []string{}, Claimed: []string{}, Granted: []string{}}
	ok, err := h.Store.InsertParticipation(ctx, row)
	if err != nil {
		return nil, err
	}
	if !ok {
		return h.Store.LockParticipation(ctx, tenant, r.RunID, student)
	}
	if err := h.Events.Publish(ctx, write.Event{Type: EventParticipationStarted, SubjectType: "student", SubjectID: student.String(),
		PartitionKey: student.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"event_id": r.EventID, "event_run_id": r.RunID,
			"event_kind": r.Definition.Kind, "student_id": student, "school_id": school, "joined_at": now}}); err != nil {
		return nil, err
	}
	sch := school
	if _, err := h.grant(ctx, tenant, student, &sch, write.GrantEventParticipation, "participation:"+r.RunID.String()+":"+student.String(),
		r.Definition.ParticipationRewards, now); err != nil {
		return nil, err
	}
	return &row, nil
}

// ClaimResult is the reward of a claim.
type ClaimResult struct {
	Claimed   bool
	Rewards   []map[string]any
	ClaimedAt time.Time
}

// View is RewardClaimResult (rewards as RewardGrantSummary).
func (c ClaimResult) View() map[string]any {
	rewards := make([]map[string]any, 0, len(c.Rewards))
	for _, g := range c.Rewards {
		kind, _ := g["type"].(string)
		m := map[string]any{"kind": kind}
		if n := liveops.Amount(g); n > 0 {
			m["amount"] = n
		}
		if v, ok := g["currency_code"].(string); ok {
			m["currency_code"] = v
		}
		for _, k := range []string{"item_id", "badge_id", "collection_item_id"} {
			if v, ok := g[k].(string); ok && v != "" {
				m["item_id"] = v
			}
		}
		rewards = append(rewards, m)
	}
	return map[string]any{"claimed": c.Claimed, "rewards": rewards, "claimed_at": c.ClaimedAt}
}

// ClaimMilestone is claimEventMilestone (F077-AC-10): a reached milestone (by the points under the pinned version, or
// reached earlier — never lowered) is granted once through the Reward Engine (event.milestone_claimed, audit); not
// reached → 409 EVENT_MILESTONE_NOT_REACHED; claimed or granted already → 409 CONFLICT; a suspended event gives no new
// reward (409); after claim_until the event is gone (404) and its milestones were granted at archive.
func (h *LiveOps) ClaimMilestone(ctx context.Context, event uuid.UUID, milestone string) (out ClaimResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ClaimEventMilestone")
	defer func() { done(err) }()
	student, tenant, err := h.playerOf(ctx)
	if err != nil {
		return out, err
	}
	sc, err := tenantScope(ctx, tenant)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		r, st, err := h.visibleRun(ctx, tenant, student, event, now)
		if err != nil {
			return err
		}
		if r.Status == liveops.StatusSuspended {
			return problem.New(problem.Conflict, "the event is suspended")
		}
		if !liveops.Claimable(r.Run(), now) {
			return problem.New(problem.EventEnded, "")
		}
		m := r.Definition.Milestone(milestone)
		if m == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		p, err := h.Store.LockParticipation(ctx, tenant, r.RunID, student)
		if err != nil {
			return err
		}
		if p == nil || (!slices.Contains(p.Reached, milestone) && p.Points < m.EventPoints) {
			return problem.New(problem.EventMilestoneNotReached, "")
		}
		if slices.Contains(p.Claimed, milestone) || slices.Contains(p.Granted, milestone) {
			return problem.New(problem.Conflict, "the milestone reward was given already")
		}
		school := st.SchoolID
		if _, err := h.grant(ctx, tenant, student, &school, write.GrantEventMilestone, "milestone:"+r.RunID.String()+":"+milestone+":"+
			student.String(), m.Rewards, now); err != nil {
			return err
		}
		if !slices.Contains(p.Reached, milestone) {
			p.Reached = append(p.Reached, milestone)
		}
		p.Claimed = append(p.Claimed, milestone)
		if err := h.Store.SaveParticipation(ctx, *p, now); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventMilestoneClaimed, SubjectType: "student", SubjectID: student.String(),
			PartitionKey: student.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"event_id": r.EventID, "event_run_id": r.RunID,
				"milestone_id": milestone, "student_id": student, "auto_granted": false, "claimed_at": now}}); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditMilestoneClaimed, TargetType: "event_run", TargetID: r.RunID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"milestone_id": milestone}}); err != nil {
			return err
		}
		out = ClaimResult{Claimed: true, Rewards: copyGrants(m.Rewards), ClaimedAt: now}
		h.after(ctx, func(context.Context) { h.metric(func(m LiveOpsMetrics) { m.Claim("event_milestone") }) })
		return idempotency.SetResult(ctx, out.View())
	})
	return out, err
}

// RecordProgress is the consumer step of game.liveops_progress* (F077-AC-08, AC-09, AC-13, AC-15, AC-18): a verified
// server event of a student earns the event points of every live run of its audience (one credit per run and source
// event, under the daily cap of the event; the first earning creates the participation), contributes to their community
// goals (deduplicated per source event, capped per student per day, never from a suspicious_speed attempt) and earns
// season points in the current season. Runs in the consumer transaction (TenantScope of the envelope).
func (h *LiveOps) RecordProgress(ctx context.Context, tenant uuid.UUID, f liveops.Fact) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RecordLiveopsProgress")
	defer func() { done(err) }()
	st, err := h.Students.Student(ctx, tenant, f.StudentID)
	if err != nil {
		return err
	}
	if st == nil {
		return ErrStudentNotProjected
	}
	if st.Deleted || st.Status != "active" {
		return nil
	}
	if err := h.Store.LockStudent(ctx, f.StudentID); err != nil {
		return err
	}
	runs, err := h.Store.EarningRuns(ctx, tenant)
	if err != nil {
		return err
	}
	now := h.now()
	day := liveops.Day(f.At)
	eventQuest := func(q uuid.UUID) bool {
		for _, r := range runs {
			if r.Audience.Includes(r.TenantID, tenant, st.SchoolID, st.Grade) && slices.Contains(r.Definition.Components.QuestIDs, q) {
				return true
			}
		}
		return false
	}
	for _, r := range runs {
		if !r.Audience.Includes(r.TenantID, tenant, st.SchoolID, st.Grade) {
			continue
		}
		if err := h.recordRun(ctx, r, tenant, *st, f, day, now); err != nil {
			return err
		}
	}
	return h.recordSeason(ctx, tenant, *st, f, day, eventQuest, now)
}

func (h *LiveOps) recordRun(ctx context.Context, r port.EventRunRow, tenant uuid.UUID, st port.WorldStudent, f liveops.Fact, day, now time.Time) error {
	d := r.Definition
	own := func(q uuid.UUID) bool { return slices.Contains(d.Components.QuestIDs, q) }
	code, pts := "", 0
	if d.PointsEnabled() {
		code, pts = liveops.PointsFor(d.EventPoints.Sources, f.SourceCodes(own))
	}
	g, err := h.Store.RunGoal(ctx, r.RunID)
	if err != nil {
		return err
	}
	amount := 0
	if g != nil {
		amount = f.MetricAmount(g.Metric, g.Filter)
	}
	if pts == 0 && amount == 0 {
		return nil
	}
	p, err := h.participate(ctx, r, tenant, f.StudentID, st.SchoolID, now)
	if err != nil {
		return err
	}
	if pts > 0 {
		if err := h.creditEventPoints(ctx, r, p, f, code, pts, day, now); err != nil {
			return err
		}
	}
	if amount > 0 {
		return h.contribute(ctx, r, *g, tenant, st, f, amount, day, now)
	}
	return nil
}

func (h *LiveOps) creditEventPoints(ctx context.Context, r port.EventRunRow, p *port.ParticipationRow, f liveops.Fact, code string, pts int,
	day, now time.Time) error {
	dailyCap := r.Definition.EventPoints.DailyCap
	if dailyCap <= 0 {
		dailyCap = h.policy().Rules.PointsDailyCap
	}
	today, err := h.Store.EventPointsOn(ctx, p.TenantID, r.RunID, p.StudentID, day)
	if err != nil {
		return err
	}
	credit, capped := liveops.Earn(pts, today, dailyCap)
	if credit == 0 {
		return nil // the cap took it all: no zero posting
	}
	ok, err := h.Store.InsertEventPoints(ctx, write.PointEntry{ID: ids.New(), TenantID: p.TenantID, Ref: r.RunID, StudentID: p.StudentID, Delta: credit,
		SourceCode: code, SourceEventID: f.EventID, Capped: capped, Day: day, At: now})
	if err != nil || !ok {
		return err
	}
	before := len(p.Reached)
	p.Points += credit
	p.Reached = liveops.Reached(r.Definition.MilestoneRewards, p.Points, p.Reached)
	if err := h.Store.SaveParticipation(ctx, *p, now); err != nil {
		return err
	}
	if h.Realtime != nil {
		newly := append([]string{}, p.Reached[before:]...)
		payload := map[string]any{"event_id": r.EventID, "event_run_id": r.RunID, "delta": credit, "event_points": p.Points,
			"daily_points_remaining": max(0, dailyCap-today-credit), "milestones_reached": newly}
		student, rt := p.StudentID, h.Realtime
		h.after(ctx, func(ctx context.Context) {
			_ = rt.Push(ctx, RTEventProgressUpdated, []uuid.UUID{student}, payload, now)
		})
	}
	return nil
}

// scopeOf is the scope id of a goal for a student: its school, its tenant, its tenant (platform goal: one row per tenant).
func scopeOf(scope string, tenant, school uuid.UUID) uuid.UUID {
	if scope == liveops.ScopeSchool {
		return school
	}
	return tenant
}

func (h *LiveOps) contribute(ctx context.Context, r port.EventRunRow, g port.GoalRow, tenant uuid.UUID, st port.WorldStudent, f liveops.Fact,
	amount int, day, now time.Time) error {
	today, err := h.Store.ContributedOn(ctx, tenant, g.GoalID, f.StudentID, day)
	if err != nil {
		return err
	}
	credit, capped := liveops.Earn(amount, today, g.DailyCap)
	if credit == 0 {
		h.metric(func(m LiveOpsMetrics) { m.Contribution("capped") })
		return nil
	}
	first, err := h.Store.Contributed(ctx, tenant, g.GoalID, f.StudentID)
	if err != nil {
		return err
	}
	first = !first
	scope := scopeOf(g.Scope, tenant, st.SchoolID)
	var grade *int
	if st.Grade > 0 {
		gr := st.Grade
		grade = &gr
	}
	ok, err := h.Store.InsertContribution(ctx, write.Contribution{ID: ids.New(), TenantID: tenant, GoalID: g.GoalID, ScopeID: scope,
		StudentID: f.StudentID, SchoolID: st.SchoolID, Grade: grade, Amount: credit, SourceEventID: f.EventID, Day: day, At: now})
	if err != nil {
		return err
	}
	if !ok {
		h.metric(func(m LiveOpsMetrics) { m.Contribution("duplicate") })
		return nil
	}
	result := "counted"
	if capped {
		result = "capped"
	}
	h.metric(func(m LiveOpsMetrics) { m.Contribution(result) })
	prog, err := h.Store.AddProgress(ctx, tenant, g.GoalID, scope, int64(credit), now)
	if err != nil {
		return err
	}
	total := prog.Total
	if g.Scope == liveops.ScopePlatform {
		if total, err = h.Store.PlatformTotal(ctx, g.GoalID); err != nil {
			return err
		}
	}
	if err := h.reachMilestones(ctx, r, g, tenant, scope, total, prog.Reached, now); err != nil {
		return err
	}
	if first {
		// A student contributing after a milestone was reached receives it too (every contributor of the scope, once).
		reached, err := h.Store.Progress(ctx, tenant, g.GoalID, scope)
		if err != nil {
			return err
		}
		school := st.SchoolID
		for _, pct := range reached.Reached {
			if err := h.grantGoal(ctx, g, pct, tenant, f.StudentID, &school, now); err != nil {
				return err
			}
		}
	}
	return h.afterContribution(ctx, r, g, tenant, scope, total, st, now)
}

// reachMilestones records the milestones the total crosses (community_goal.milestone_reached once per scope and
// milestone, community_goal.completed at 100 %) and grants each to every contributor of the scope in tenant (F077-AC-15).
func (h *LiveOps) reachMilestones(ctx context.Context, r port.EventRunRow, g port.GoalRow, tenant, scope uuid.UUID, total int64, reached []int,
	now time.Time) error {
	percents := make([]int, 0, len(g.Milestones))
	for _, m := range g.Milestones {
		percents = append(percents, m.Percent)
	}
	slices.Sort(percents)
	crossed := liveops.Crossed(g.Target, total, percents, reached)
	if len(crossed) == 0 {
		return nil
	}
	all := append(append([]int{}, reached...), crossed...)
	slices.Sort(all)
	if err := h.Store.SetReached(ctx, tenant, g.GoalID, scope, all, now); err != nil {
		return err
	}
	for _, pct := range crossed {
		ms := g.Goal().Milestone(pct)
		payload := map[string]any{"goal_id": g.GoalID, "event_run_id": r.RunID, "event_id": r.EventID, "goal_code": g.Code, "scope": g.Scope,
			"scope_id": scope, "milestone_pct": pct, "total": total, "target": g.Target, "reached_at": now}
		if ms != nil && ms.WorldChangeCode != "" {
			payload["world_change_code"] = ms.WorldChangeCode
		}
		t := tenant
		if err := h.Events.Publish(ctx, write.Event{Type: EventGoalMilestoneReached, SubjectType: "community_goal", SubjectID: g.GoalID.String(),
			PartitionKey: g.GoalID.String(), TenantID: &t, OccurredAt: now, Payload: payload}); err != nil {
			return err
		}
		if pct == 100 {
			if err := h.Events.Publish(ctx, write.Event{Type: EventGoalCompleted, SubjectType: "community_goal", SubjectID: g.GoalID.String(),
				PartitionKey: g.GoalID.String(), TenantID: &t, OccurredAt: now, Payload: map[string]any{"goal_id": g.GoalID, "event_run_id": r.RunID,
					"event_id": r.EventID, "goal_code": g.Code, "scope": g.Scope, "scope_id": scope, "total": total, "target": g.Target,
					"completed_at": now}}); err != nil {
				return err
			}
		}
		after := uuid.Nil
		for {
			page, err := h.Store.Contributors(ctx, tenant, g.GoalID, scope, after, liveOpsPage)
			if err != nil {
				return err
			}
			for _, c := range page {
				school := c.SchoolID
				if err := h.grantGoal(ctx, g, pct, tenant, c.StudentID, &school, now); err != nil {
					return err
				}
			}
			if len(page) < liveOpsPage {
				break
			}
			after = page[len(page)-1].StudentID
		}
	}
	return nil
}

func (h *LiveOps) grantGoal(ctx context.Context, g port.GoalRow, pct int, tenant, student uuid.UUID, school *uuid.UUID, now time.Time) error {
	ms := g.Goal().Milestone(pct)
	if ms == nil || len(ms.Rewards) == 0 {
		return nil
	}
	_, err := h.grant(ctx, tenant, student, school, write.GrantCommunityGoal, "goal:"+g.GoalID.String()+":"+itoa(pct)+":"+student.String(),
		ms.Rewards, now)
	return err
}

// afterContribution raises the Valkey counter after commit and pushes event.community_goal.updated to the online students
// of the scope, at most once per community_goal_progress_flush_seconds (never one push per contribution,
// EVENT_SEASON_SYSTEM §12): the window is taken and the students read in the transaction, the push goes after commit.
func (h *LiveOps) afterContribution(ctx context.Context, r port.EventRunRow, g port.GoalRow, tenant, scope uuid.UUID, total int64,
	st port.WorldStudent, now time.Time) error {
	if h.Counter == nil {
		return nil
	}
	counter := h.Counter
	h.after(ctx, func(ctx context.Context) { _ = counter.Raise(ctx, g.GoalID, scope, total) })
	if h.Realtime == nil {
		return nil
	}
	ok, err := counter.Throttle(ctx, g.GoalID, scope, h.policy().FlushEvery)
	if err != nil || !ok {
		return nil //nolint:nilerr // a throttle failure only skips a best-effort push
	}
	var schools []uuid.UUID
	if g.Scope == liveops.ScopeSchool {
		schools = []uuid.UUID{st.SchoolID}
	}
	var students []uuid.UUID
	after := uuid.Nil
	for {
		page, err := h.Store.OnlineStudents(ctx, tenant, schools, r.Audience.GradeLevels, after, liveOpsPage)
		if err != nil {
			return err
		}
		students = append(students, page...)
		if len(page) < liveOpsPage {
			break
		}
		after = page[len(page)-1]
	}
	if len(students) == 0 {
		return nil
	}
	payload := map[string]any{"goal_id": g.GoalID, "event_run_id": r.RunID, "scope": g.Scope, "target": g.Target, "progress": total}
	rt := h.Realtime
	h.after(ctx, func(ctx context.Context) { _ = rt.Push(ctx, RTEventCommunityGoalUpdated, students, payload, now) })
	return nil
}

// RecomputeGoals is the job goal-recompute (F077-AC-14): the total of every scope of the goals of runs not archived is
// recomputed from the contributions (the truth): a progress row that differs is corrected (audit), the Valkey counter is
// overwritten, and milestones the true total reaches are reached and granted. Returns the scopes corrected.
func (h *LiveOps) RecomputeGoals(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RecomputeGoals")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope(JobGoalRecompute)
	if err != nil {
		return 0, err
	}
	now := h.now()
	var after *write.GoalScope
	var errs []error
	for {
		var page []write.GoalScope
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
			var err error
			page, err = h.Store.GoalScopes(ctx, after, batch)
			return err
		}); err != nil {
			return n, err
		}
		for _, s := range page {
			sc, err := authz.EventScope(s.TenantID)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			gs := s
			fixed := false
			if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
				g, err := h.Store.Goal(ctx, gs.GoalID)
				if err != nil || g == nil {
					return err
				}
				r, err := h.Store.Run(ctx, g.RunID)
				if err != nil || r == nil {
					return err
				}
				if gs.Total != gs.Cached {
					if err := h.Store.SetTotal(ctx, gs.TenantID, gs.GoalID, gs.ScopeID, gs.Total, now); err != nil {
						return err
					}
					if err := h.Audit.Record(ctx, audit.Event{Action: auditGoalRecomputed, TargetType: "community_goal", TargetID: gs.GoalID.String(),
						Result: audit.ResultSuccess, Metadata: map[string]any{"scope_id": gs.ScopeID, "cached": gs.Cached, "total": gs.Total}}); err != nil {
						return err
					}
					fixed = true
				}
				total := gs.Total
				if g.Scope == liveops.ScopePlatform {
					if total, err = h.Store.PlatformTotal(ctx, g.GoalID); err != nil {
						return err
					}
				}
				if err := h.reachMilestones(ctx, *r, *g, gs.TenantID, gs.ScopeID, total, gs.Reached, now); err != nil {
					return err
				}
				if h.Counter != nil {
					t := total
					h.after(ctx, func(ctx context.Context) {
						_ = h.Counter.Set(ctx, gs.GoalID, gs.ScopeID, t)
						h.metric(func(m LiveOpsMetrics) { m.CacheRebuild() })
					})
				}
				return nil
			}); err != nil {
				errs = append(errs, err)
				continue
			}
			if fixed {
				n++
			}
		}
		if len(page) < batch || len(page) == 0 {
			break
		}
		last := page[len(page)-1]
		after = &last
	}
	return n, errors.Join(errs...)
}
