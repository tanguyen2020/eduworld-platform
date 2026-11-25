package command

import (
	"context"
	"errors"
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
)

// Live-ops events (specs/events/payloads/game/*.v1, owner F077) and realtime messages (AsyncAPI).
const (
	EventEventScheduled           = "event.scheduled"
	EventParticipationStarted     = "event.participation_started"
	EventMilestoneClaimed         = "event.milestone_claimed"
	EventGoalMilestoneReached     = "community_goal.milestone_reached"
	EventGoalCompleted            = "community_goal.completed"
	EventSeasonTierReached        = "season_track.tier_reached"
	EventSeasonRewardClaimed      = "season_track.reward_claimed"
	RTEventStatusChanged          = "event.status.changed"
	RTEventProgressUpdated        = "event.progress.updated"
	RTEventCommunityGoalUpdated   = "event.community_goal.updated"
	RTSeasonTrackUpdated          = "season.track.updated"
	JobEventScheduler             = "event-scheduler"
	JobGoalRecompute              = "goal-recompute"
	auditEventRunOperated         = "event_run."
	auditMilestoneClaimed         = "event_milestone.claimed"
	auditSeasonTierClaimed        = "season_tier.claimed"
	auditSchoolEventCreated       = "school_event.created"
	auditSchoolEventUpdated       = "school_event.updated"
	auditSchoolEventCancelled     = "school_event.cancelled"
	auditMilestonesAutoGranted    = "event_milestone.auto_granted"
	auditSeasonTiersAutoGranted   = "season_tier.auto_granted"
	auditGoalRecomputed           = "community_goal.recomputed"
	liveOpsPage                   = 500
	maxSchedulerRounds            = 20
	defaultSchedulerLockTTL       = 12 * time.Second
	defaultFlushEvery             = 30 * time.Second
	defaultUpcomingVisible        = 7 * 24 * time.Hour
	defaultClaimGrace             = 72 * time.Hour
	defaultRuleChangeHour         = 4
	defaultSchoolEventMaxDays     = 28
	defaultSchoolEventsConcurrent = 3
	defaultMinCell                = 5
)

// liveOpsNS is the UUIDv5 namespace of the trigger_event_id of the F077 reward grants (participation, milestone, goal
// milestone, season tier): one grant per (student, source) whatever event triggers it.
var liveOpsNS = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-00000000f077")

// LiveOpsPolicy holds the platform parameters of live-ops (CONFIG_DEFAULTS, EVENT_SEASON_SYSTEM tham số).
type LiveOpsPolicy struct {
	Rules              liveops.Policy
	ClaimGrace         time.Duration // event_claim_grace_hours
	UpcomingVisible    time.Duration // event_upcoming_visible_days
	RuleChangeHour     int           // event_rule_change_effective (hour ICT of the next day)
	FlushEvery         time.Duration // community_goal_progress_flush_seconds
	SchoolEventMaxDays int           // school_event_max_duration_days
	SchoolEventsMax    int           // school_events_concurrent_per_tenant
	MinCell            int           // analytics_min_cell_size
	SchedulerLockTTL   time.Duration
}

// WithDefaults fills the zero parameters with the CONFIG_DEFAULTS values.
func (p LiveOpsPolicy) WithDefaults() LiveOpsPolicy {
	if p.ClaimGrace <= 0 {
		p.ClaimGrace = defaultClaimGrace
	}
	if p.UpcomingVisible <= 0 {
		p.UpcomingVisible = defaultUpcomingVisible
	}
	if p.RuleChangeHour <= 0 {
		p.RuleChangeHour = defaultRuleChangeHour
	}
	if p.FlushEvery <= 0 {
		p.FlushEvery = defaultFlushEvery
	}
	if p.SchoolEventMaxDays <= 0 {
		p.SchoolEventMaxDays = defaultSchoolEventMaxDays
	}
	if p.SchoolEventsMax <= 0 {
		p.SchoolEventsMax = defaultSchoolEventsConcurrent
	}
	if p.MinCell <= 0 {
		p.MinCell = defaultMinCell
	}
	if p.SchedulerLockTTL <= 0 {
		p.SchedulerLockTTL = defaultSchedulerLockTTL
	}
	return p
}

// LiveOpsMetrics observes live-ops; nil = not observed.
type LiveOpsMetrics interface {
	RunTransition(to string)
	SchedulerLag(seconds float64)
	Contribution(result string)
	CacheRebuild()
	SeasonPoints(capped bool)
	Claim(kind string)
}

// ManifestRebuilder rebuilds the content manifest of tenants after a run changes (command.GameManifest).
type ManifestRebuilder interface {
	RebuildTenants(ctx context.Context, tenants []uuid.UUID, cause uuid.UUID, itemType string) error
}

// LiveOps is the write side of live-ops (F077): the runs of the published events (scheduler, operator actions), the
// participation, event points and milestones of students, the community goals, the season track and the school events.
// Points, contributions and season points come only from verified server events (R5) into append-only ledgers
// idempotent per source event; every reward goes through the Reward Engine grant applier registry with its caps (Xu daily
// cap), once per key (ADR-0049).
type LiveOps struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Store       write.LiveOpsStore
	Students    port.StudentDirectory
	Tenants     write.TenantLister
	Manifests   ManifestRebuilder
	Grants      write.ProgressionStore
	Registry    write.GrantApplierRegistry
	EventSchema port.PayloadValidator // event.schema.json of a school event version
	Events      write.EventPublisher
	Audit       write.Auditor
	Screener    port.TextScreener
	Counter     port.GoalCounter
	Realtime    port.LiveOpsRealtime
	Lock        port.JobLock
	Metrics     LiveOpsMetrics
	Policy      LiveOpsPolicy
	Now         func() time.Time
}

func (h *LiveOps) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h *LiveOps) metric(fn func(m LiveOpsMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

func (h *LiveOps) policy() LiveOpsPolicy { return h.Policy.WithDefaults() }

// after runs fn after the commit of the transaction of ctx (pushes, cache, metrics).
func (h *LiveOps) after(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, fn)
	}
}

func liveopsProblem(err error) error {
	var fe *liveops.FieldError
	if errors.As(err, &fe) {
		return problem.Invalid(fe.Field, problem.SubInvalid, fe.Reason)
	}
	var te *liveops.TranslationError
	if errors.As(err, &te) {
		return problem.New(problem.TranslationIncomplete, te.Error())
	}
	var pe *liveops.ParamError
	if errors.As(err, &pe) {
		// errors[].field names the parameter so the portal shows it on its form field (F081-AC-04)
		return problem.New(problem.EventTemplateParamNotAllowed, pe.Error()).AddField(pe.Param, problem.SubInvalid, pe.Reason)
	}
	switch {
	case errors.Is(err, liveops.ErrInvalidTransition):
		return problem.New(problem.InvalidStateTransition, "")
	case errors.Is(err, liveops.ErrReason):
		return problem.Invalid("reason", problem.SubInvalid, "3 to 1000 characters")
	}
	return err
}

// runPayload is the payload of the run events (event.scheduled, started, suspended, resumed, ended, cancelled).
func runPayload(r port.EventRunRow, status string, reason *string, at time.Time) map[string]any {
	return map[string]any{"event_id": r.EventID, "event_run_id": r.RunID, "event_version_id": r.VersionID, "event_kind": r.Definition.Kind,
		"owner_scope": r.Definition.OwnerScope, "status": status, "starts_at": r.StartsAt, "ends_at": r.EndsAt, "claim_until": r.ClaimUntil,
		"audience": audienceJSON(r.Audience), "reason": reason, "changed_at": at}
}

func audienceJSON(a liveops.Audience) map[string]any {
	m := map[string]any{"scope": a.Scope, "grade_levels": nonNilInts(a.GradeLevels)}
	m["tenant_ids"] = nonNilIDs(a.TenantIDs)
	m["school_ids"] = nonNilIDs(a.SchoolIDs)
	return m
}

func nonNilIDs(s []uuid.UUID) []uuid.UUID {
	if s == nil {
		return []uuid.UUID{}
	}
	return s
}

func nonNilInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}

func (h *LiveOps) runEvent(ctx context.Context, r port.EventRunRow, typ, status string, reason *string, at time.Time) error {
	e := write.Event{Type: typ, SubjectType: "event_run", SubjectID: r.RunID.String(), PartitionKey: r.EventID.String(), OccurredAt: at,
		Payload: runPayload(r, status, reason, at)}
	if r.TenantID != nil {
		t := *r.TenantID
		e.TenantID = &t
	} else {
		e.Platform = true
	}
	return h.Events.Publish(ctx, e)
}

// EffectiveFrom is when a new version of event applies (F077-AC-11): 04:00 ICT the next day while its run is live or
// suspended, at once otherwise. A live run is never shortened by a version (only by suspend/cancel, EVENT_SEASON_SYSTEM
// §5.3): a version ending earlier is refused.
func (h *LiveOps) EffectiveFrom(ctx context.Context, event uuid.UUID, d liveops.Definition, at time.Time) (time.Time, error) {
	run, err := h.Store.ActiveRun(ctx, event)
	if err != nil {
		return at, err
	}
	if run == nil || run.Status == liveops.StatusScheduled {
		return at.UTC(), nil
	}
	if d.Schedule.EndAt.Before(run.EndsAt) {
		return at, problem.Invalid("schedule.end_at", problem.SubInvalid, "a running event is shortened only by suspend or cancel")
	}
	return liveops.RuleChangeEffective(at, true, h.policy().RuleChangeHour), nil
}

// EventPublished is the publish hook of an event version (F077-AC-01): the first version with a schedule creates a
// scheduled run (event.scheduled) and its community goal; a newer version re-pins a run that is not live at once — a live
// run waits for effective_from (the scheduler re-pins it then, never lowering what students reached).
func (h *LiveOps) EventPublished(ctx context.Context, d liveops.Definition, version uuid.UUID, versionNo int, at time.Time) error {
	run, err := h.Store.ActiveRun(ctx, d.EventID)
	if err != nil {
		return err
	}
	claim := d.ClaimUntil(h.policy().ClaimGrace)
	if run == nil {
		if !d.Schedule.EndAt.After(at) {
			return problem.Invalid("schedule.end_at", problem.SubInvalid, "the schedule has already ended")
		}
		r := port.EventRunRow{RunID: ids.New(), EventID: d.EventID, VersionID: version, VersionNo: versionNo, TenantID: d.TenantID,
			Status: liveops.StatusScheduled, StartsAt: d.Schedule.StartAt.UTC(), EndsAt: d.Schedule.EndAt.UTC(), ClaimUntil: claim.UTC(),
			StatusVersion: 1, Audience: d.Audience, Definition: d}
		if d.TenantID != nil && len(d.Audience.SchoolIDs) == 1 {
			s := d.Audience.SchoolIDs[0]
			r.SchoolID = &s
		}
		if err := h.Store.InsertRun(ctx, r, at); err != nil {
			return err
		}
		if err := h.upsertGoal(ctx, r.RunID, d, at); err != nil {
			return err
		}
		h.metric(func(m LiveOpsMetrics) { m.RunTransition(liveops.StatusScheduled) })
		return h.runEvent(ctx, r, EventEventScheduled, liveops.StatusScheduled, nil, at)
	}
	if run.Status != liveops.StatusScheduled {
		return nil // live or suspended: the scheduler re-pins at effective_from
	}
	return h.repin(ctx, *run, version, d, at)
}

func (h *LiveOps) repin(ctx context.Context, run port.EventRunRow, version uuid.UUID, d liveops.Definition, at time.Time) error {
	ends, claim := d.Schedule.EndAt.UTC(), d.ClaimUntil(h.policy().ClaimGrace).UTC()
	starts := d.Schedule.StartAt.UTC()
	if run.Status != liveops.StatusScheduled {
		starts = run.StartsAt
		if ends.Before(run.EndsAt) {
			ends = run.EndsAt
		}
		if claim.Before(run.ClaimUntil) {
			claim = run.ClaimUntil
		}
	}
	if err := h.Store.RepinRun(ctx, run.RunID, version, starts, ends, claim, d.Audience, at); err != nil {
		return err
	}
	return h.upsertGoal(ctx, run.RunID, d, at)
}

func (h *LiveOps) upsertGoal(ctx context.Context, run uuid.UUID, d liveops.Definition, at time.Time) error {
	g := d.Components.CommunityGoal
	if g == nil {
		return nil
	}
	existing, err := h.Store.RunGoal(ctx, run)
	if err != nil {
		return err
	}
	id := ids.New()
	if existing != nil {
		id = existing.GoalID
	}
	dailyCap := g.PerStudentDailyCap
	if dailyCap <= 0 {
		dailyCap = h.policy().Rules.GoalDailyCap
	}
	return h.Store.UpsertGoal(ctx, port.GoalRow{GoalID: id, RunID: run, TenantID: d.TenantID, Code: g.GoalCode, Metric: g.Metric, Scope: g.Scope,
		Target: g.Target, DailyCap: dailyCap, ShowGrades: g.ShowGrades(), Milestones: g.Milestones, Filter: g.MetricFilter}, at)
}

// RunScheduler is one tick of the job event-scheduler (F077-AC-01, AC-02, AC-05, AC-11): under the singleton lock
// ew:lock:job:event-scheduler, it re-pins the live runs whose newer version is in effect, then moves every due run
// (scheduled → live, live → ended, ended/cancelled → archived after granting the reached milestones), each in its own
// transaction with the run locked and its status version checked: repeated ticks or several instances never make a
// transition twice.
func (h *LiveOps) RunScheduler(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RunEventScheduler")
	defer func() { done(err) }()
	if h.Lock != nil {
		ok, err := h.Lock.Acquire(ctx, JobEventScheduler, h.policy().SchedulerLockTTL)
		if err != nil || !ok {
			return 0, err
		}
	}
	ps, err := authz.SystemPlatformScope(JobEventScheduler)
	if err != nil {
		return 0, err
	}
	now := h.now()
	var repins []uuid.UUID
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		repins, err = h.Store.PendingRepins(ctx, now, batch)
		return err
	}); err != nil {
		return 0, err
	}
	var errs []error
	for _, id := range repins {
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error { return h.applyRepin(ctx, id, now) }); err != nil {
			errs = append(errs, err)
		}
	}
	for round := 0; round < maxSchedulerRounds; round++ {
		var due []uuid.UUID
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
			var err error
			due, err = h.Store.DueRuns(ctx, now, batch)
			return err
		}); err != nil {
			return n, err
		}
		moved := 0
		for _, id := range due {
			ok, err := h.transition(ctx, id, now)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if ok {
				moved++
			}
		}
		n += moved
		if moved == 0 {
			break
		}
	}
	return n, errors.Join(errs...)
}

func (h *LiveOps) applyRepin(ctx context.Context, run uuid.UUID, now time.Time) error {
	r, err := h.Store.LockRun(ctx, run)
	if err != nil || r == nil {
		return err
	}
	cv, err := h.Store.CurrentEventVersion(ctx, r.EventID)
	if err != nil || cv == nil || cv.VersionID == r.VersionID {
		return err
	}
	if cv.EffectiveFrom != nil && cv.EffectiveFrom.After(now) {
		return nil
	}
	return h.repin(ctx, *r, cv.VersionID, cv.Definition, now)
}

// transition moves one due run (false when nothing was due or another instance moved it).
func (h *LiveOps) transition(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	var run *port.EventRunRow
	var to string
	sc, err := authz.SystemPlatformScope(JobEventScheduler)
	if err != nil {
		return false, err
	}
	// Archive: the reached milestones are granted first, tenant by tenant (idempotent keys: a retry grants nothing twice).
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		run, err = h.Store.Run(ctx, id)
		return err
	}); err != nil || run == nil {
		return false, err
	}
	if run.Run().Due(now) == liveops.StatusArchived {
		if err := h.grantReached(ctx, *run, now); err != nil {
			return false, err
		}
	}
	moved := false
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		r, err := h.Store.LockRun(ctx, id)
		if err != nil || r == nil {
			return err
		}
		to = r.Run().Due(now)
		if to == "" {
			return nil
		}
		ok, err := h.Store.SetRunStatus(ctx, id, r.StatusVersion, to, nil, nil, now)
		if err != nil || !ok {
			return err
		}
		moved, run = true, r
		lag := 0.0
		switch to {
		case liveops.StatusLive:
			lag = now.Sub(r.StartsAt).Seconds()
		case liveops.StatusEnded:
			lag = now.Sub(r.EndsAt).Seconds()
		}
		h.after(ctx, func(context.Context) {
			h.metric(func(m LiveOpsMetrics) { m.RunTransition(to); m.SchedulerLag(lag) })
		})
		if typ := liveops.EventFor(r.Status, to); typ != "" {
			return h.runEvent(ctx, *r, typ, to, nil, now)
		}
		return nil
	}); err != nil || !moved {
		return false, err
	}
	if to != liveops.StatusArchived {
		h.afterRunChange(ctx, *run, to, now)
	}
	return true, nil
}

// afterRunChange rebuilds the content manifest of the audience tenants (event entries added or removed, F077-AC-02) and
// pushes event.status.changed to the students of the audience with an active game session (best effort).
func (h *LiveOps) afterRunChange(ctx context.Context, r port.EventRunRow, status string, now time.Time) {
	ps, err := authz.SystemPlatformScope(JobEventScheduler)
	if err != nil {
		return
	}
	var tenants []uuid.UUID
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		active, err := h.Tenants.ActiveTenants(ctx)
		tenants = r.Audience.Tenants(r.TenantID, active)
		return err
	}); err != nil {
		return
	}
	if h.Manifests != nil {
		_ = h.Manifests.RebuildTenants(ctx, tenants, r.RunID, write.ItemEvent)
	}
	if h.Realtime == nil || status == liveops.StatusScheduled {
		return
	}
	payload := map[string]any{"event_id": r.EventID, "event_run_id": r.RunID, "status": status, "name": r.Definition.Name}
	for _, t := range tenants {
		h.pushOnline(ctx, t, r.Audience.SchoolIDs, r.Audience.GradeLevels, RTEventStatusChanged, payload, now)
	}
}

// pushOnline pushes a message to the students of tenant with an active game session in the audience (pages of 500).
func (h *LiveOps) pushOnline(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID, grades []int, typ string, payload map[string]any, now time.Time) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return
	}
	after := uuid.Nil
	for {
		var page []uuid.UUID
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			var err error
			page, err = h.Store.OnlineStudents(ctx, tenant, schools, grades, after, liveOpsPage)
			return err
		}); err != nil || len(page) == 0 {
			return
		}
		_ = h.Realtime.Push(ctx, typ, page, payload, now)
		if len(page) < liveOpsPage {
			return
		}
		after = page[len(page)-1]
	}
}

// grantReached grants every reached milestone neither claimed nor granted to the participants of a run, tenant by tenant
// (F077-AC-05): auto_granted_milestones keeps them, the grant keys make a retry harmless.
func (h *LiveOps) grantReached(ctx context.Context, r port.EventRunRow, now time.Time) error {
	ps, err := authz.SystemPlatformScope(JobEventScheduler)
	if err != nil {
		return err
	}
	var tenants []uuid.UUID
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		tenants, err = h.Store.ParticipantTenants(ctx, r.RunID)
		return err
	}); err != nil {
		return err
	}
	for _, t := range tenants {
		sc, err := authz.EventScope(t)
		if err != nil {
			return err
		}
		after := uuid.Nil
		for {
			var page []port.ParticipationRow
			granted := 0
			if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
				var err error
				if page, err = h.Store.Participants(ctx, t, r.RunID, after, liveOpsPage); err != nil {
					return err
				}
				for _, p := range page {
					n, err := h.autoGrant(ctx, r, p, now)
					if err != nil {
						return err
					}
					granted += n
				}
				if granted > 0 {
					return h.Audit.Record(ctx, audit.Event{Action: auditMilestonesAutoGranted, TargetType: "event_run", TargetID: r.RunID.String(),
						Result: audit.ResultSuccess, Metadata: map[string]any{"granted": granted}})
				}
				return nil
			}); err != nil {
				return err
			}
			if len(page) < liveOpsPage {
				break
			}
			after = page[len(page)-1].ID
		}
	}
	return nil
}

func (h *LiveOps) autoGrant(ctx context.Context, r port.EventRunRow, p port.ParticipationRow, now time.Time) (int, error) {
	if err := h.Store.LockStudent(ctx, p.StudentID); err != nil {
		return 0, err
	}
	locked, err := h.Store.LockParticipation(ctx, p.TenantID, r.RunID, p.StudentID)
	if err != nil || locked == nil {
		return 0, err
	}
	pending := liveops.Unclaimed(locked.Reached, locked.Claimed, locked.Granted)
	n := 0
	for _, id := range pending {
		m := r.Definition.Milestone(id)
		if m == nil {
			continue
		}
		if _, err := h.grant(ctx, locked.TenantID, locked.StudentID, &locked.SchoolID, write.GrantEventMilestone,
			"milestone:"+r.RunID.String()+":"+id+":"+locked.StudentID.String(), m.Rewards, now); err != nil {
			return n, err
		}
		locked.Granted = append(locked.Granted, id)
		n++
		if err := h.Events.Publish(ctx, write.Event{Type: EventMilestoneClaimed, SubjectType: "student", SubjectID: locked.StudentID.String(),
			PartitionKey: locked.StudentID.String(), TenantID: &locked.TenantID, OccurredAt: now, Payload: map[string]any{
				"event_id": r.EventID, "event_run_id": r.RunID, "milestone_id": id, "student_id": locked.StudentID, "auto_granted": true,
				"claimed_at": now}}); err != nil {
			return n, err
		}
	}
	if n == 0 {
		return 0, nil
	}
	h.metric(func(m LiveOpsMetrics) { m.Claim("event_milestone_auto") })
	return n, h.Store.SaveParticipation(ctx, *locked, now)
}

// grant applies rewards to a student through the Reward Engine registry once per key (reward_grants unique key; the
// appliers keep the Xu daily cap — never bypassed). Returns false when the key was granted already.
func (h *LiveOps) grant(ctx context.Context, tenant, student uuid.UUID, school *uuid.UUID, kind, key string, rewards []liveops.Reward,
	at time.Time) (bool, error) {
	if len(rewards) == 0 {
		return false, nil
	}
	g := write.RewardGrant{ID: ids.New(), TenantID: tenant, StudentID: student, SchoolID: school,
		TriggerEventID: uuid.NewSHA1(liveOpsNS, []byte(kind+":"+key)), TriggerKind: kind, Grants: copyGrants(rewards),
		IdempotencyKey: Key(kind, key), At: at}
	return applyGrant(ctx, h.Grants, h.Registry, h.Events, g, nil)
}

// RunOperation is suspendEventRun, resumeEventRun or cancelEventRun.
type RunOperation struct {
	EventID uuid.UUID
	RunID   uuid.UUID
	Action  string
	Reason  string
}

// OperateRun is the operator action on a platform run (F077-AC-03, AC-06): game.event.operate of a platform role
// (internal gateway, portal client — route guards), a reason of 3..1000 characters kept in the audit, the event of the
// transition; an invalid transition is 422 INVALID_STATE_TRANSITION. Suspended: the event components leave the manifest,
// started plays finish on their pinned versions, no new event reward (F077-AC-04).
func (h *LiveOps) OperateRun(ctx context.Context, op RunOperation) (view map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.OperateEventRun")
	defer func() { done(err) }()
	var run port.EventRunRow
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return view, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermGameEventOperate, authz.Resource{}); err != nil {
		return view, err
	}
	ps, err := authz.PlatformScope(p)
	if err != nil {
		return view, err
	}
	now := h.now()
	var to string
	by := p.UserID
	err = h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		r, err := h.Store.LockRun(ctx, op.RunID)
		if err != nil {
			return err
		}
		if r == nil || r.EventID != op.EventID {
			return problem.New(problem.NotFound, "")
		}
		if to, err = liveops.Operate(r.Status, op.Action, op.Reason); err != nil {
			return liveopsProblem(err)
		}
		reason := op.Reason
		ok, err := h.Store.SetRunStatus(ctx, r.RunID, r.StatusVersion, to, &reason, &by, now)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.Conflict, "the run changed meanwhile")
		}
		if err := h.runEvent(ctx, *r, liveops.EventFor(r.Status, to), to, &reason, now); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditEventRunOperated + op.Action, TargetType: "event_run", TargetID: r.RunID.String(),
			Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"event_id": r.EventID, "from": r.Status, "to": to}}); err != nil {
			return err
		}
		run = *r
		run.Status, run.StatusVersion = to, r.StatusVersion+1
		st, err := h.Store.RunStats(ctx, r.RunID)
		if err != nil {
			return err
		}
		view = port.RunStatsView(run, st, now)
		return idempotency.SetResult(ctx, view)
	})
	if err != nil {
		return view, err
	}
	h.metric(func(m LiveOpsMetrics) { m.RunTransition(to) })
	h.afterRunChange(ctx, run, to, now)
	return view, nil
}
