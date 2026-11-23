package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// F077 — event & season live-ops (EVENT_SEASON_SYSTEM §4–§13, DATABASE_DESIGN §2.8c). Platform events, their runs and
// community goals are platform rows (tenant_id NULL) of G/T tables whose tenant rows are the school events created from a
// template (T*: tenant_id nullable, RLS reads platform rows plus the rows of the session tenant); templates, season tracks
// are platform content (G); participation, goal progress and season track progress are tenant tables (T); the event point,
// contribution and season point ledgers are append-only tenant tables (TA, R21). JSON arrays replace the varchar[]/smallint[]
// columns of DATABASE_DESIGN (ent has no PostgreSQL arrays; CHECK bounds the length — spec Clarifications).

// Event — game.events (G/T): identity of a live-ops event and its release pointer. owner_scope platform ⇔ tenant_id NULL;
// a tenant (school) event keeps its template, the school it belongs to (NULL = the whole tenant) and the parameters the
// school filled (school_params, SchoolEventCreate).
type Event struct{ ent.Schema }

func (Event) Annotations() []schema.Annotation { return table("events") }
func (Event) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "event_id"}}}
}
func (Event) Fields() []ent.Field {
	return append([]ent.Field{
		field.String("owner_scope").MaxLen(16).Default("platform").Immutable(),
		field.String("code").MaxLen(64).Immutable(),
		field.String("event_kind").MaxLen(32),
		field.UUID("template_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.JSON("school_params", map[string]any{}).Optional(),
	}, pointerFields()...)
}

// EventVersion — game.event_versions (G/T): payload of specs/schemas/event.schema.json, immutable once published (R11),
// with the columns the scheduler and the checks read; effective_from is when its rules apply to a running run
// (event_rule_change_effective, F077-AC-11).
type EventVersion struct{ ent.Schema }

func (EventVersion) Annotations() []schema.Annotation { return table("event_versions") }
func (EventVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "event_version_id"}}}
}
func (EventVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("event_id", uuid.UUID{}).Immutable()),
		field.String("event_kind").MaxLen(32).Immutable(),
		field.Time("start_at").Immutable(),
		field.Time("end_at").Immutable(),
		field.Time("effective_from").Optional().Nillable().Immutable(),
		field.JSON("decoration_areas", []string{}).Immutable(),
	)
}
func (EventVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("event_id", "version_no").Unique().StorageKey("event_versions_event_no_uq")}
}

// EventTemplate — game.event_templates (G): identity of a platform template of school events (ADR-0029: assigned to
// tenants through infra.tenant_resource_assignments, resource_type event_template).
type EventTemplate struct{ ent.Schema }

func (EventTemplate) Annotations() []schema.Annotation { return table("event_templates") }
func (EventTemplate) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "template_id"}}}
}
func (EventTemplate) Fields() []ent.Field {
	return append([]ent.Field{field.String("code").MaxLen(64).Optional().Nillable()}, pointerFields()...)
}

// EventTemplateVersion — game.event_template_versions (G): payload of specs/schemas/event-template.schema.json.
type EventTemplateVersion struct{ ent.Schema }

func (EventTemplateVersion) Annotations() []schema.Annotation {
	return table("event_template_versions")
}
func (EventTemplateVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "template_version_id"}}}
}
func (EventTemplateVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("template_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.String("event_kind").MaxLen(32).Immutable(),
		field.JSON("allowed_params", []string{}).Immutable(),
		field.Int("max_duration_days").Immutable(),
	)
}

// EventRun — game.event_runs (G/T): one scheduled run of an event (scheduled → live → ended → archived; suspended,
// cancelled by an operator). status_version is the optimistic lock of the transitions (no duplicate transition across
// scheduler instances, F077-AC-01); audience is the audience of the pinned version (who sees the run).
type EventRun struct{ ent.Schema }

func (EventRun) Annotations() []schema.Annotation { return table("event_runs") }
func (EventRun) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "event_run_id"}}}
}
func (EventRun) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("event_id", uuid.UUID{}).Immutable(),
		field.UUID("event_version_id", uuid.UUID{}),
		field.String("status").MaxLen(16).Default("scheduled"),
		field.Time("starts_at"),
		field.Time("ends_at"),
		field.Time("claim_until"),
		field.String("suspended_reason").MaxLen(1000).Optional().Nillable(),
		field.UUID("status_changed_by", uuid.UUID{}).Optional().Nillable(),
		field.Int("status_version").Default(1),
		field.JSON("audience", map[string]any{}),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("cancelled_at").Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
	}
}
func (EventRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "starts_at").StorageKey("event_runs_status_starts_idx"),
		index.Fields("status", "ends_at").StorageKey("event_runs_status_ends_idx"),
		index.Fields("status", "claim_until").StorageKey("event_runs_status_claim_idx"),
		index.Fields("event_id").StorageKey("event_runs_event_idx"),
	}
}

// EventParticipation — game.event_participation (T): one student in one run (unique per run and student), the event
// points (Σ event_point_ledger) and the milestones reached, claimed and granted when the run is archived.
type EventParticipation struct{ ent.Schema }

func (EventParticipation) Annotations() []schema.Annotation { return table("event_participation") }
func (EventParticipation) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "participation_id"}}}
}
func (EventParticipation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("event_run_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.Time("joined_at").Immutable(),
		field.Int("event_points").Default(0),
		field.JSON("reached_milestones", []string{}),
		field.JSON("claimed_milestones", []string{}),
		field.JSON("auto_granted_milestones", []string{}),
	}
}
func (EventParticipation) Indexes() []ent.Index {
	return []ent.Index{index.Fields("event_run_id", "student_id").Unique().StorageKey("event_participation_run_student_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// EventPointEntry — game.event_point_ledger (TA): one credit of event points from a verified server event, idempotent per
// (run, source event, student); a capped credit says so. Never updated or deleted (no negative entry: missing a day never
// takes points, F077-AC-29).
type EventPointEntry struct{ ent.Schema }

func (EventPointEntry) Annotations() []schema.Annotation { return table("event_point_ledger") }
func (EventPointEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "entry_id"}}}
}
func (EventPointEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("event_run_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.Int("delta").Immutable(),
		field.String("source_code").MaxLen(64).Immutable(),
		field.UUID("source_event_id", uuid.UUID{}).Immutable(),
		field.Bool("capped").Default(false).Immutable(),
		field.Time("earned_on").SchemaType(map[string]string{"postgres": "date"}).Immutable(),
	}
}
func (EventPointEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_run_id", "source_event_id", "student_id").Unique().StorageKey("event_point_ledger_source_uq"),
		index.Fields("tenant_id", "event_run_id", "student_id", "earned_on").StorageKey("event_point_ledger_day_idx"),
	}
}

// CommunityGoal — game.community_goals (G/T): the goal of a run (metric, scope, target, milestones with their rewards).
type CommunityGoal struct{ ent.Schema }

func (CommunityGoal) Annotations() []schema.Annotation { return table("community_goals") }
func (CommunityGoal) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "goal_id"}}}
}
func (CommunityGoal) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("event_run_id", uuid.UUID{}).Immutable(),
		field.String("goal_code").MaxLen(64).Immutable(),
		field.String("metric").MaxLen(32),
		field.String("scope").MaxLen(16),
		field.Int64("target"),
		field.Int("per_student_daily_cap"),
		field.Bool("show_grade_contributions").Default(true),
		field.JSON("milestones", []map[string]any{}),
		field.JSON("metric_filter", map[string]any{}).Optional(),
	}
}
func (CommunityGoal) Indexes() []ent.Index {
	return []ent.Index{index.Fields("event_run_id", "goal_code").Unique().StorageKey("community_goals_run_code_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// CommunityGoalContribution — game.community_goal_contributions (TA): one contribution of a student to a goal from a
// verified server event (deduplicated per goal, source event and student; capped per student per day), with the school
// and grade of the student at that time (ADR-0095: no class).
type CommunityGoalContribution struct{ ent.Schema }

func (CommunityGoalContribution) Annotations() []schema.Annotation {
	return table("community_goal_contributions")
}
func (CommunityGoalContribution) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "contribution_id"}}}
}
func (CommunityGoalContribution) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("goal_id", uuid.UUID{}).Immutable(),
		field.UUID("scope_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.Int("grade_level").Optional().Nillable().Immutable(),
		field.Int("amount").Immutable(),
		field.UUID("source_event_id", uuid.UUID{}).Immutable(),
		field.Time("contributed_on").SchemaType(map[string]string{"postgres": "date"}).Immutable(),
	}
}
func (CommunityGoalContribution) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("goal_id", "source_event_id", "student_id").Unique().StorageKey("community_goal_contributions_source_uq"),
		index.Fields("goal_id", "student_id", "contributed_on").StorageKey("community_goal_contributions_day_idx"),
		index.Fields("goal_id", "scope_id").StorageKey("community_goal_contributions_scope_idx"),
	}
}

// CommunityGoalProgress — game.community_goal_progress (T): the total of a goal in one scope of the tenant (a school, the
// tenant; a platform goal keeps one row per tenant) and the milestones reached; the contributions are the truth.
type CommunityGoalProgress struct{ ent.Schema }

func (CommunityGoalProgress) Annotations() []schema.Annotation {
	return table("community_goal_progress")
}
func (CommunityGoalProgress) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "progress_id"}}}
}
func (CommunityGoalProgress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("goal_id", uuid.UUID{}).Immutable(),
		field.UUID("scope_id", uuid.UUID{}).Immutable(),
		field.Int64("total").Default(0),
		field.JSON("milestones_reached", []int{}),
		field.Time("recomputed_at").Optional().Nillable(),
	}
}
func (CommunityGoalProgress) Indexes() []ent.Index {
	return []ent.Index{index.Fields("goal_id", "scope_id").Unique().StorageKey("community_goal_progress_scope_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// SeasonTrack — game.season_tracks (G): the free reward track of one season (one per season).
type SeasonTrack struct{ ent.Schema }

func (SeasonTrack) Annotations() []schema.Annotation { return table("season_tracks") }
func (SeasonTrack) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "season_track_id"}}}
}
func (SeasonTrack) Fields() []ent.Field {
	return append([]ent.Field{
		field.UUID("season_id", uuid.UUID{}).Optional().Nillable(),
		field.String("code").MaxLen(64).Optional().Nillable(),
	}, pointerFields()...)
}

// SeasonTrackVersion — game.season_track_versions (G): payload of specs/schemas/season-track.schema.json.
type SeasonTrackVersion struct{ ent.Schema }

func (SeasonTrackVersion) Annotations() []schema.Annotation { return table("season_track_versions") }
func (SeasonTrackVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "season_track_version_id"}}}
}
func (SeasonTrackVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("season_track_id", uuid.UUID{}).Immutable()),
		field.UUID("season_id", uuid.UUID{}).Immutable(),
	)
}

// SeasonPointEntry — game.season_point_ledger (TA): one credit of season points, idempotent per (season, source event,
// student), capped per day (season_points_daily_cap).
type SeasonPointEntry struct{ ent.Schema }

func (SeasonPointEntry) Annotations() []schema.Annotation { return table("season_point_ledger") }
func (SeasonPointEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "entry_id"}}}
}
func (SeasonPointEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("season_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.Int("delta").Immutable(),
		field.String("source_code").MaxLen(64).Immutable(),
		field.UUID("source_event_id", uuid.UUID{}).Immutable(),
		field.Bool("capped").Default(false).Immutable(),
		field.Time("earned_on").SchemaType(map[string]string{"postgres": "date"}).Immutable(),
	}
}
func (SeasonPointEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("season_id", "source_event_id", "student_id").Unique().StorageKey("season_point_ledger_source_uq"),
		index.Fields("tenant_id", "season_id", "student_id", "earned_on").StorageKey("season_point_ledger_day_idx"),
	}
}

// SeasonTrackProgress — game.season_track_progress (T): the season points, tier and claimed tiers of a student; the tiers
// granted at the end of the season (auto_granted_tiers) and the history of the past seasons stay (never deleted).
type SeasonTrackProgress struct{ ent.Schema }

func (SeasonTrackProgress) Annotations() []schema.Annotation { return table("season_track_progress") }
func (SeasonTrackProgress) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "progress_id"}}}
}
func (SeasonTrackProgress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("season_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.Int("points").Default(0),
		field.Int("tier").Default(0),
		field.JSON("claimed_tiers", []int{}),
		field.JSON("auto_granted_tiers", []int{}),
		field.Time("auto_granted_at").Optional().Nillable(),
	}
}
func (SeasonTrackProgress) Indexes() []ent.Index {
	return []ent.Index{index.Fields("season_id", "student_id").Unique().StorageKey("season_track_progress_student_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}
