package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
)

// Game content item types of the release core added by F077 (platform events, school event templates, season tracks).
const (
	ItemEvent         = "event"
	ItemEventTemplate = "event_template"
	ItemSeasonTrack   = "season_track"
)

// Reward grant trigger kinds of F077 (gamification.reward_grants.trigger_kind).
const (
	GrantEventParticipation = "event_participation"
	GrantEventMilestone     = "event_milestone"
	GrantCommunityGoal      = "community_goal"
	GrantSeasonTier         = "season_tier"
)

// PointEntry is one row of game.event_point_ledger or game.season_point_ledger (append-only; Ref is the run or season).
type PointEntry struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Ref           uuid.UUID
	StudentID     uuid.UUID
	Delta         int
	SourceCode    string
	SourceEventID uuid.UUID
	Capped        bool
	Day           time.Time
	At            time.Time
}

// Contribution is one row of game.community_goal_contributions (append-only).
type Contribution struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	GoalID        uuid.UUID
	ScopeID       uuid.UUID
	StudentID     uuid.UUID
	SchoolID      uuid.UUID
	Grade         *int
	Amount        int
	SourceEventID uuid.UUID
	Day           time.Time
	At            time.Time
}

// Contributor is a student with at least one contribution to a goal in a scope.
type Contributor struct {
	StudentID uuid.UUID
	SchoolID  uuid.UUID
}

// GoalScope is the total of a goal in one scope from the contributions (the truth) and as cached in the progress row.
type GoalScope struct {
	TenantID uuid.UUID
	GoalID   uuid.UUID
	ScopeID  uuid.UUID
	Total    int64 // Σ contributions
	Cached   int64 // community_goal_progress.total
	Reached  []int
}

// EventVersionRow is a new event version written by a school event (tenant events do not go through the release core).
type EventVersionRow struct {
	VersionID     uuid.UUID
	EventID       uuid.UUID
	TenantID      uuid.UUID
	VersionNo     int
	Payload       []byte
	SHA256        string
	Definition    liveops.Definition
	EffectiveFrom time.Time
	By            uuid.UUID
	At            time.Time
}

// LiveOpsStore writes the live-ops state of module game in the transaction of ctx (F077): runs and goals under the
// platform scope (platform events) or the tenant scope (school events); participation, ledgers and progress of a student
// in its tenant (RLS + explicit tenant_id). Reads that lock (FOR UPDATE) are here; plain reads are LiveOpsReader.
type LiveOpsStore interface {
	port.LiveOpsReader
	// ActiveRun returns the scheduled, live or suspended run of an event (locked), nil when none.
	ActiveRun(ctx context.Context, event uuid.UUID) (*port.EventRunRow, error)
	// InsertRun inserts a scheduled run.
	InsertRun(ctx context.Context, r port.EventRunRow, at time.Time) error
	// RepinRun pins a run to a newer version (schedule never shortened, audience of the version).
	RepinRun(ctx context.Context, run, version uuid.UUID, starts, ends, claim time.Time, a liveops.Audience, at time.Time) error
	// LockRun returns a run with its pinned version (locked), nil when unknown or locked by another transaction (skip).
	LockRun(ctx context.Context, run uuid.UUID) (*port.EventRunRow, error)
	// SetRunStatus moves a run from the status version read to status to (false: another transaction moved it first).
	SetRunStatus(ctx context.Context, run uuid.UUID, statusVersion int, to string, reason *string, by *uuid.UUID, at time.Time) (bool, error)
	// DueRuns lists the runs whose transition is due at now (platform scan).
	DueRuns(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
	// PendingRepins lists the runs whose event has a newer published version in effect at now (platform scan).
	PendingRepins(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
	// UpsertGoal writes the goal of a run (target and milestones of the pinned version).
	UpsertGoal(ctx context.Context, g port.GoalRow, at time.Time) error
	// ParticipantTenants lists the tenants with participants of a run (platform scan).
	ParticipantTenants(ctx context.Context, run uuid.UUID) ([]uuid.UUID, error)
	// Participants pages the participants of a run in tenant (keyset on participation_id).
	Participants(ctx context.Context, tenant, run, after uuid.UUID, limit int) ([]port.ParticipationRow, error)
	// LockStudent serializes the live-ops writes of one student (advisory lock of the transaction).
	LockStudent(ctx context.Context, student uuid.UUID) error
	// LockParticipation returns the participation of a student in a run (locked), nil when none.
	LockParticipation(ctx context.Context, tenant, run, student uuid.UUID) (*port.ParticipationRow, error)
	// InsertParticipation inserts a participation (false when it exists).
	InsertParticipation(ctx context.Context, p port.ParticipationRow) (bool, error)
	// SaveParticipation writes points and milestone lists.
	SaveParticipation(ctx context.Context, p port.ParticipationRow, at time.Time) error
	// InsertEventPoints appends event points (false on a replayed source event).
	InsertEventPoints(ctx context.Context, e PointEntry) (bool, error)
	// InsertContribution appends a contribution (false on a replayed source event).
	InsertContribution(ctx context.Context, c Contribution) (bool, error)
	// AddProgress adds amount to the total of a scope (row created when missing) and returns it.
	AddProgress(ctx context.Context, tenant, goal, scope uuid.UUID, amount int64, at time.Time) (port.ProgressRow, error)
	// SetReached writes the milestones reached of a scope.
	SetReached(ctx context.Context, tenant, goal, scope uuid.UUID, reached []int, at time.Time) error
	// Contributors pages the students with a contribution to goal in scope of tenant (keyset on student_id).
	Contributors(ctx context.Context, tenant, goal, scope, after uuid.UUID, limit int) ([]Contributor, error)
	// GoalScopes lists the scopes of the goals of runs not archived with their totals (platform scan for goal-recompute),
	// after the cursor (goal, scope, tenant).
	GoalScopes(ctx context.Context, after *GoalScope, limit int) ([]GoalScope, error)
	// SetTotal writes the recomputed total of a scope.
	SetTotal(ctx context.Context, tenant, goal, scope uuid.UUID, total int64, at time.Time) error
	// LockTenant serializes the school event writes of a tenant (count of active events).
	LockTenant(ctx context.Context, tenant uuid.UUID) error
	// ActiveTenantEvents counts the scheduled, live or suspended runs of tenant events of tenant.
	ActiveTenantEvents(ctx context.Context, tenant uuid.UUID) (int, error)
	// InsertTenantEvent inserts the identity of a school event.
	InsertTenantEvent(ctx context.Context, tenant, event uuid.UUID, code, kind string, template uuid.UUID, school *uuid.UUID,
		params map[string]any, by uuid.UUID, at time.Time) error
	// InsertTenantVersion inserts a published version of a school event and moves the pointer (the previous published
	// version is archived) and the parameters; the optimistic version of the event is incremented.
	InsertTenantVersion(ctx context.Context, v EventVersionRow, params map[string]any) error
	// LockSchoolEvent returns a school event of tenant with its current run (locked), nil when unknown.
	LockSchoolEvent(ctx context.Context, tenant, event uuid.UUID) (*port.SchoolEventRow, error)
	// UpsertSeason writes the projection of a season when sourceVersion is newer (false: an older or equal version).
	UpsertSeason(ctx context.Context, s port.SeasonRow) (bool, error)
	// SeasonTenants lists the tenants with season track progress of a season (platform scan).
	SeasonTenants(ctx context.Context, season uuid.UUID) ([]uuid.UUID, error)
	// SeasonProgressPage pages the season track progress of a season in tenant (keyset on progress_id).
	SeasonProgressPage(ctx context.Context, tenant, season, after uuid.UUID, limit int) ([]port.SeasonProgressRow, error)
	// LockSeasonProgress returns the progress of a student in a season (locked), nil when none.
	LockSeasonProgress(ctx context.Context, tenant, season, student uuid.UUID) (*port.SeasonProgressRow, error)
	// SaveSeasonProgress inserts or updates the progress of a student.
	SaveSeasonProgress(ctx context.Context, p port.SeasonProgressRow, at time.Time) error
	// InsertSeasonPoints appends season points (false on a replayed source event).
	InsertSeasonPoints(ctx context.Context, e PointEntry) (bool, error)
	// OnlineStudents pages the students of tenant with an active game session in the audience (schools, grades; empty =
	// every school, every grade), keyset on student_id — the targets of the realtime pushes.
	OnlineStudents(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID, grades []int, after uuid.UUID, limit int) ([]uuid.UUID, error)
}
