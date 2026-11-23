package port

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/season"
)

// EventRunRow is one run with the version it pins (game.event_runs + game.event_versions, F077).
type EventRunRow struct {
	RunID           uuid.UUID
	EventID         uuid.UUID
	VersionID       uuid.UUID
	VersionNo       int
	TenantID        *uuid.UUID // nil = platform event
	SchoolID        *uuid.UUID // owner school of a school event (nil = the whole tenant)
	Status          string
	StartsAt        time.Time
	EndsAt          time.Time
	ClaimUntil      time.Time
	StatusVersion   int
	Audience        liveops.Audience
	SuspendedReason *string
	CancelledAt     *time.Time
	Definition      liveops.Definition
}

// Run returns the domain view of the schedule and status.
func (r EventRunRow) Run() liveops.Run {
	return liveops.Run{Status: r.Status, StartsAt: r.StartsAt, EndsAt: r.EndsAt, ClaimUntil: r.ClaimUntil}
}

// GoalRow is the community goal of a run (game.community_goals).
type GoalRow struct {
	GoalID     uuid.UUID
	RunID      uuid.UUID
	TenantID   *uuid.UUID
	Code       string
	Metric     string
	Scope      string
	Target     int64
	DailyCap   int
	ShowGrades bool
	Milestones []liveops.GoalMilestone
	Filter     *liveops.MetricFilter
}

// Goal returns the domain view of the goal.
func (g GoalRow) Goal() liveops.Goal {
	show := g.ShowGrades
	return liveops.Goal{GoalCode: g.Code, Metric: g.Metric, Scope: g.Scope, Target: g.Target, PerStudentDailyCap: g.DailyCap,
		ShowGradeContributions: &show, Milestones: g.Milestones, MetricFilter: g.Filter}
}

// ParticipationRow is one student in one run (game.event_participation).
type ParticipationRow struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	RunID     uuid.UUID
	StudentID uuid.UUID
	SchoolID  uuid.UUID
	JoinedAt  time.Time
	Points    int
	Reached   []string
	Claimed   []string
	Granted   []string
}

// TemplateRow is the current published version of an event template.
type TemplateRow struct {
	TemplateID uuid.UUID
	VersionID  uuid.UUID
	VersionNo  int
	Template   liveops.Template
}

// SeasonRow is ref.seasons (projection of season.started / season.ended of module competition).
type SeasonRow struct {
	SeasonID      uuid.UUID
	Code          string
	Name          string
	ThemeCode     *string
	StartsAt      time.Time
	EndsAt        time.Time
	EndedAt       *time.Time
	Status        string // started, ended
	SourceVersion int64
}

// TrackRow is the current published track of a season.
type TrackRow struct {
	TrackID   uuid.UUID
	VersionID uuid.UUID
	SeasonID  uuid.UUID
	Track     season.Track
}

// SeasonProgressRow is the season track progress of a student (game.season_track_progress).
type SeasonProgressRow struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	SeasonID      uuid.UUID
	StudentID     uuid.UUID
	SchoolID      uuid.UUID
	Points        int
	Tier          int
	Claimed       []int
	Granted       []int
	AutoGrantedAt *time.Time
}

// SchoolEventRow is a tenant event with its current version, run and the parameters the school filled.
type SchoolEventRow struct {
	EventID    uuid.UUID
	TenantID   uuid.UUID
	SchoolID   *uuid.UUID
	TemplateID uuid.UUID
	Version    int64 // optimistic version of game.events (ETag / If-Match)
	Params     map[string]any
	CreatedBy  *uuid.UUID
	CreatedAt  time.Time
	Run        *EventRunRow
}

// GradeContribution is the contribution of one grade of a scope (count of contributing students, total).
type GradeContribution = liveops.GradeRow

// ErrScreeningUnavailable is the text screening of module chat failing or not wired (503 UNAVAILABLE: nothing is stored —
// never fail-open, F077-AC-23).
var ErrScreeningUnavailable = errors.New("text screening unavailable")

// TextScreener checks the texts a school types against the effective sensitive-word catalogue (facade chat screenText):
// the positions of the texts that are not clean, never the matched term (R15).
type TextScreener interface {
	Screen(ctx context.Context, tenant uuid.UUID, locale string, texts []string) ([]int, error)
}

// GoalCounter is the fast counter of a community goal in Valkey (ew:game:cgoal:<goal_id>:<scope_id>): a cache only —
// community_goal_progress and the contributions in PostgreSQL are the truth (R9, F077-AC-14).
type GoalCounter interface {
	// Get returns the cached total (false when missing).
	Get(ctx context.Context, goal, scope uuid.UUID) (int64, bool, error)
	// Raise sets the counter to total when it is missing or lower (monotonic under concurrent commits).
	Raise(ctx context.Context, goal, scope uuid.UUID, total int64) error
	// Set overwrites the counter (rebuild and the recompute job).
	Set(ctx context.Context, goal, scope uuid.UUID, total int64) error
	// Throttle reports whether a push of (goal, scope) may go now: at most one per window
	// (community_goal_progress_flush_seconds).
	Throttle(ctx context.Context, goal, scope uuid.UUID, window time.Duration) (bool, error)
}

// LiveOpsRealtime pushes the live-ops messages of the AsyncAPI to students (event.status.changed, event.progress.updated,
// event.community_goal.updated, season.track.updated) — best effort, after commit.
type LiveOpsRealtime interface {
	Push(ctx context.Context, typ string, students []uuid.UUID, payload map[string]any, at time.Time) error
}

// JobLock is the singleton lock of a job across worker instances (ew:lock:job:<name>).
type JobLock interface {
	Acquire(ctx context.Context, name string, ttl time.Duration) (bool, error)
}

// ManifestEvents are the event entries of the content manifest of a tenant (live runs whose audience holds the tenant).
type ManifestEvents interface {
	ManifestEvents(ctx context.Context, tenant uuid.UUID) ([]map[string]any, error)
}

// ProgressRow is game.community_goal_progress of one scope.
type ProgressRow struct {
	TenantID uuid.UUID
	GoalID   uuid.UUID
	ScopeID  uuid.UUID
	Total    int64
	Reached  []int
}

// LiveOpsReader reads the live-ops state in the transaction of ctx (use case or read session).
type LiveOpsReader interface {
	// Run returns a run with its pinned version, nil when unknown (platform rows or rows of the session tenant).
	Run(ctx context.Context, run uuid.UUID) (*EventRunRow, error)
	// EventRun returns the newest run of an event not archived and visible to tenant (platform or tenant event), nil when
	// none.
	EventRun(ctx context.Context, tenant, event uuid.UUID) (*EventRunRow, error)
	// EarningRuns lists the live runs visible to tenant (platform runs and the runs of tenant events).
	EarningRuns(ctx context.Context, tenant uuid.UUID) ([]EventRunRow, error)
	// RunGoal returns the goal of a run, nil when none.
	RunGoal(ctx context.Context, run uuid.UUID) (*GoalRow, error)
	// Goal returns a goal, nil when unknown.
	Goal(ctx context.Context, goal uuid.UUID) (*GoalRow, error)
	// Participation returns the participation of a student in a run, nil when none.
	Participation(ctx context.Context, tenant, run, student uuid.UUID) (*ParticipationRow, error)
	// EventPointsOn is Σ event points of a student in a run on the ICT day.
	EventPointsOn(ctx context.Context, tenant, run, student uuid.UUID, day time.Time) (int, error)
	// ContributedOn is Σ contributions of a student to a goal on the ICT day.
	ContributedOn(ctx context.Context, tenant, goal, student uuid.UUID, day time.Time) (int, error)
	// Contributed reports at least one contribution of a student to a goal.
	Contributed(ctx context.Context, tenant, goal, student uuid.UUID) (bool, error)
	// Progress returns the progress of a scope of tenant (zero row when none).
	Progress(ctx context.Context, tenant, goal, scope uuid.UUID) (ProgressRow, error)
	// PlatformTotal is the total of a platform goal over every tenant (a count only, never a student).
	PlatformTotal(ctx context.Context, goal uuid.UUID) (int64, error)
	// GradeContributions returns the contributions of goal in school by grade (students counted, total).
	GradeContributions(ctx context.Context, tenant, goal, school uuid.UUID) ([]GradeContribution, error)
	// KnownSchools counts the schools of tenant among schools (ref.schools).
	KnownSchools(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID) (int, error)
	// Template returns the current published version of a template assigned to tenant, nil when none.
	Template(ctx context.Context, tenant, template uuid.UUID) (*TemplateRow, error)
	// CurrentEventVersion returns the current published version of an event (id, number, payload, effective_from).
	CurrentEventVersion(ctx context.Context, event uuid.UUID) (*CurrentVersion, error)
	// Season returns the projection of a season, nil when unknown.
	Season(ctx context.Context, season uuid.UUID) (*SeasonRow, error)
	// CurrentSeason returns the started season holding at, nil when none.
	CurrentSeason(ctx context.Context, at time.Time) (*SeasonRow, error)
	// Track returns the current published track of a season, nil when none.
	Track(ctx context.Context, season uuid.UUID) (*TrackRow, error)
	// SeasonProgress returns the progress of a student in a season, nil when none.
	SeasonProgress(ctx context.Context, tenant, season, student uuid.UUID) (*SeasonProgressRow, error)
	// SeasonPointsOn is Σ season points of a student in a season on the ICT day.
	SeasonPointsOn(ctx context.Context, tenant, season, student uuid.UUID, day time.Time) (int, error)
	// RunStats counts the participants, claims and grants of a run over every tenant (platform scope).
	RunStats(ctx context.Context, run uuid.UUID) (RunStats, error)
	// QuestRecurrence returns availability.recurrence of the current version of quests (unknown quests are absent).
	QuestRecurrence(ctx context.Context, quests []uuid.UUID) (map[uuid.UUID]string, error)
	// OverlappingDecorations lists the events other than event whose current version decorates one of areas between from
	// and to (one decoration per area at a time, F077-AC-28).
	OverlappingDecorations(ctx context.Context, event uuid.UUID, areas []string, from, to time.Time) ([]uuid.UUID, error)
}

// CurrentVersion is the current published version of an event.
type CurrentVersion struct {
	VersionID     uuid.UUID
	VersionNo     int
	Definition    liveops.Definition
	EffectiveFrom *time.Time
	PublishedAt   *time.Time
}

// RunStats are the counters of a run over every tenant (counts only, never a student).
type RunStats struct {
	Participants    int
	MilestoneClaims map[string]int
	RewardsGranted  int
}

// RunStatsView is EventRunStats (getEventRunStats and the response of the operator actions).
func RunStatsView(r EventRunRow, st RunStats, at time.Time) map[string]any {
	claims := make([]map[string]any, 0, len(st.MilestoneClaims))
	ms := make([]string, 0, len(st.MilestoneClaims))
	for m := range st.MilestoneClaims {
		ms = append(ms, m)
	}
	slices.Sort(ms)
	for _, m := range ms {
		claims = append(claims, map[string]any{"milestone_id": m, "claims": st.MilestoneClaims[m]})
	}
	return map[string]any{"event_run_id": r.RunID, "status": r.Status, "participants": st.Participants, "participation_rate": nil,
		"milestone_claims": claims, "rewards_granted": st.RewardsGranted, "updated_at": at.UTC()}
}

// TenantName keeps vi and en of a text typed by a school (TenantLocalizedName: vi required, en optional).
func TenantName(l liveops.Localized) map[string]string {
	out := map[string]string{"vi": l["vi"]}
	if l["en"] != "" {
		out["en"] = l["en"]
	}
	return out
}

// SchoolStatus is SchoolEvent.status of a run (suspended shows live; archived shows ended or cancelled).
func SchoolStatus(r *EventRunRow) string {
	if r == nil {
		return "draft"
	}
	switch r.Status {
	case liveops.StatusLive, liveops.StatusSuspended:
		return "live"
	case liveops.StatusArchived:
		if r.CancelledAt != nil {
			return "cancelled"
		}
		return "ended"
	}
	return r.Status
}

// SchoolEventView is SchoolEvent: the parameters the school filled, the effective audience, schedule and goal target of
// the current version, status, version (ETag), creator.
func SchoolEventView(r SchoolEventRow) map[string]any {
	out := map[string]any{}
	for k, v := range r.Params {
		out[k] = v
	}
	out["event_id"], out["tenant_id"], out["template_id"], out["version"], out["created_at"] = r.EventID, r.TenantID, r.TemplateID, r.Version, r.CreatedAt
	if r.CreatedBy != nil {
		out["created_by"] = *r.CreatedBy
	}
	out["status"] = SchoolStatus(r.Run)
	if r.Run != nil {
		d := r.Run.Definition
		out["name"] = TenantName(d.Name)
		if d.Description != nil {
			out["description"] = TenantName(d.Description)
		}
		out["starts_at"], out["ends_at"] = r.Run.StartsAt, r.Run.EndsAt
		schools := r.Run.Audience.SchoolIDs
		if schools == nil {
			schools = []uuid.UUID{}
		}
		out["audience"] = map[string]any{"school_ids": schools, "grade_levels": r.Run.Audience.GradeLevels}
		if g := d.Components.CommunityGoal; g != nil {
			out["community_goal_target"] = g.Target
		}
	}
	return out
}

// ETag is the entity tag of a school event (its version).
func ETag(version int64) string { return strconv.FormatInt(version, 10) }
