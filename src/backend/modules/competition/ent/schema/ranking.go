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

// F106 — ranking Glicko-2, tiers, Competition Points, leaderboards and seasons (specs/106-ranking-leaderboard/data-model.md,
// DATABASE_DESIGN §2.10, §2.16). PostgreSQL is the source of truth; Valkey only caches the open leaderboard periods
// (ADR-0004).

var numeric94 = map[string]string{"postgres": "numeric(9,4)"}

// Rating — competition.ratings (T): the Game Rating of a student per (mode, subject, season) (ADR-0018). rating, rd and
// volatility never leave module competition towards a student (F106-AC-07) nor module learning (F106-AC-09).
type Rating struct{ ent.Schema }

func (Rating) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "ratings"}}
}
func (Rating) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "rating_id"}}}
}
func (Rating) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.UUID("season_id", uuid.UUID{}).Immutable(),
		field.Int16("grade_level"),
		field.Float("rating").SchemaType(numeric94),
		field.Float("rd").SchemaType(numeric94),
		field.Float("volatility").SchemaType(map[string]string{"postgres": "numeric(9,6)"}),
		field.Int("games_played").Default(0),
		field.String("tier_code").MaxLen(64).Optional().Nillable(),
		field.String("peak_tier_code").MaxLen(64).Optional().Nillable(),
		field.Time("last_played_at").Optional().Nillable(),
		field.UUID("rule_version_id", uuid.UUID{}),
	}
}
func (Rating) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("student_id", "mode", "subject_id", "season_id").Unique().StorageKey("ratings_key_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "season_id", "last_played_at").StorageKey("ratings_season_played_idx"),
	}
}

// RatingChange — competition.rating_changes (TA): the change of one rating by one match; unique (match_id, student_id)
// so a replayed match.completed never changes a rating twice (F106-AC-02).
type RatingChange struct{ ent.Schema }

func (RatingChange) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "rating_changes"}}
}
func (RatingChange) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "change_id"}}}
}
func (RatingChange) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("match_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.UUID("season_id", uuid.UUID{}).Immutable(),
		field.JSON("before", map[string]any{}).Immutable(),
		field.JSON("after", map[string]any{}).Immutable(),
		field.UUID("rule_version_id", uuid.UUID{}).Immutable(),
	}
}
func (RatingChange) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("match_id", "student_id").Unique().StorageKey("rating_changes_match_student_uq"),
	}
}

// CompetitionPointEntry — competition.competition_point_ledger (TA): append-only Competition Points of a student for a
// source (match, tournament), unique (source_type, source_id, student_id) (F106-AC-10). grade_level and topic_id are the
// ones of the source at the time it was recorded (leaderboard scopes and filters, ADR-0095).
type CompetitionPointEntry struct{ ent.Schema }

func (CompetitionPointEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "competition_point_ledger"}}
}
func (CompetitionPointEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "entry_id"}}}
}
func (CompetitionPointEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.Int16("grade_level").Immutable(),
		field.UUID("season_id", uuid.UUID{}).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("topic_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("mode").MaxLen(64).Optional().Nillable().Immutable(),
		field.Int("amount").Immutable(),
		field.String("source_type").MaxLen(64).Immutable(),
		field.UUID("source_id", uuid.UUID{}).Immutable(),
		field.String("reason").MaxLen(64).Immutable(),
		field.JSON("breakdown", map[string]any{}).Immutable(),
		field.UUID("rule_version_id", uuid.UUID{}).Immutable(),
		field.Time("day_key").Immutable().SchemaType(map[string]string{"postgres": "date"}),
		field.Time("occurred_at").Immutable(),
	}
}
func (CompetitionPointEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_type", "source_id", "student_id").Unique().StorageKey("competition_point_ledger_source_uq"),
		index.Fields("tenant_id", "student_id", "day_key").StorageKey("competition_point_ledger_day_idx"),
		index.Fields("tenant_id", "season_id", "school_id", "grade_level").StorageKey("competition_point_ledger_scope_idx"),
	}
}

// LeaderboardContribution — competition.leaderboard_contributions (TA): the points of the other leaderboard metrics
// (effort_points, xp, quest_completed, team_points) from events of other modules, with the school and grade of the student
// at the time they were recorded (grade_goal, ADR-0095).
type LeaderboardContribution struct{ ent.Schema }

func (LeaderboardContribution) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "leaderboard_contributions"}}
}
func (LeaderboardContribution) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "contribution_id"}}}
}
func (LeaderboardContribution) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.Int16("grade_level").Immutable(),
		field.String("metric").MaxLen(64).Immutable(),
		field.Int("amount").Immutable(),
		field.String("source_type").MaxLen(64).Immutable(),
		field.String("source_id").MaxLen(64).Immutable(),
		field.Time("day_key").Immutable().SchemaType(map[string]string{"postgres": "date"}),
		field.UUID("rule_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("occurred_at").Immutable(),
	}
}
func (LeaderboardContribution) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("metric", "source_type", "source_id", "student_id").Unique().StorageKey("leaderboard_contributions_source_uq"),
		index.Fields("tenant_id", "metric", "day_key", "student_id").StorageKey("leaderboard_contributions_day_idx"),
		index.Fields("tenant_id", "school_id", "grade_level", "metric", "day_key").StorageKey("leaderboard_contributions_grade_idx"),
	}
}

// LeaderboardDefinition — competition.leaderboard_definitions (G): metric × scope × period × filter × grades, published
// by the platform (R20); never a class scope nor a class_goal metric (ADR-0095).
type LeaderboardDefinition struct{ ent.Schema }

func (LeaderboardDefinition) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "leaderboard_definitions"}}
}
func (LeaderboardDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "leaderboard_id"}}}
}
func (LeaderboardDefinition) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64),
		field.String("title").MaxLen(128),
		field.String("metric").MaxLen(64),
		field.String("scope_type").MaxLen(64),
		field.String("period").MaxLen(64),
		field.JSON("filters", map[string]any{}),
		field.JSON("grade_levels", []int{}),
		field.JSON("display", map[string]any{}),
		field.Int("sort_order").Default(0),
		field.String("status").MaxLen(64).Default("active"),
		field.UUID("config_version_id", uuid.UUID{}).Optional().Nillable(),
	}
}
func (LeaderboardDefinition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique().StorageKey("leaderboard_definitions_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// LeaderboardSnapshot — competition.leaderboard_snapshots (TA): the ranking of a closed period (top 100 + the rank of every
// member of a school/grade scope); a grade_goal board keeps one collective row (student_id and rank NULL).
type LeaderboardSnapshot struct{ ent.Schema }

func (LeaderboardSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "leaderboard_snapshots"}}
}
func (LeaderboardSnapshot) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "snapshot_id"}}}
}
func (LeaderboardSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("leaderboard_id", uuid.UUID{}).Immutable(),
		field.String("scope_id").MaxLen(80).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int16("grade_level").Optional().Nillable().Immutable(),
		field.String("period_key").MaxLen(64).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int("rank").Optional().Nillable().Immutable(),
		field.Float("value").Immutable().SchemaType(map[string]string{"postgres": "numeric(12,2)"}),
		field.Float("target").Optional().Nillable().Immutable().SchemaType(map[string]string{"postgres": "numeric(12,2)"}),
		field.Bool("reached").Optional().Nillable().Immutable(),
		field.Int("members").Optional().Nillable().Immutable(),
		field.Time("captured_at").Immutable(),
	}
}
func (LeaderboardSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("leaderboard_id", "scope_id", "period_key", "student_id").Unique().StorageKey("leaderboard_snapshots_member_uq").
			Annotations(entsql.IndexWhere("student_id IS NOT NULL")),
		index.Fields("leaderboard_id", "scope_id", "period_key").Unique().StorageKey("leaderboard_snapshots_collective_uq").
			Annotations(entsql.IndexWhere("student_id IS NULL")),
		index.Fields("tenant_id", "leaderboard_id", "scope_id", "captured_at").StorageKey("leaderboard_snapshots_history_idx"),
	}
}

// Season — competition.seasons (G): a platform season (8 weeks, soft reset); scheduled → active → ended.
type Season struct{ ent.Schema }

func (Season) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "seasons"}}
}
func (Season) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "season_id"}}}
}
func (Season) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64),
		field.String("name").MaxLen(128),
		field.String("theme_code").MaxLen(64).Optional().Nillable(),
		field.Time("starts_at"),
		field.Time("ends_at"),
		field.String("status").MaxLen(64).Default("scheduled"),
		field.JSON("rules", map[string]any{}),
		field.UUID("config_version_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("ended_at").Optional().Nillable(),
	}
}
func (Season) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique().StorageKey("seasons_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("status", "starts_at").StorageKey("seasons_status_idx"),
	}
}

// SeasonResult — competition.season_results (TA): the final rating, RD, peak tier, games and Competition Points of a
// student per (season, mode, subject), written once when the season ends (never deleted, F106-AC-23).
type SeasonResult struct{ ent.Schema }

func (SeasonResult) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "season_results"}}
}
func (SeasonResult) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "result_id"}}}
}
func (SeasonResult) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("season_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.Float("final_rating").Immutable().SchemaType(numeric94),
		field.Float("final_rd").Immutable().SchemaType(numeric94),
		field.String("peak_tier_code").MaxLen(64).Optional().Nillable().Immutable(),
		field.Int("games_played").Immutable(),
		field.Int("competition_points").Immutable(),
	}
}
func (SeasonResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("season_id", "student_id", "mode", "subject_id").Unique().StorageKey("season_results_key_uq"),
	}
}
