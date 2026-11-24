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

// Challenge — competition.challenges (T, F107 data-model): a grade challenge or school tournament created by a school
// from a pinned version of a platform template assigned to the tenant (ADR-0027, ADR-0029). Its audience is in one school:
// a grade and/or a list of students of that school minus the excluded students (exclude wins, ADR-0095). A timed-quiz
// challenge holds the rooms of its quiz (match_ids, at most 40 students each); a tournament challenge holds its tournament.
type Challenge struct{ ent.Schema }

func (Challenge) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "challenges"}}
}
func (Challenge) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "challenge_id"}}}
}
func (Challenge) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("template_config_id", uuid.UUID{}).Immutable(),
		field.UUID("template_version_id", uuid.UUID{}).Immutable(),
		field.String("kind").MaxLen(64).Immutable(),
		field.String("format").MaxLen(64).Immutable(),
		field.String("name").MaxLen(128),
		field.Int16("grade_level").Optional().Nillable().Immutable(),
		field.JSON("targets", []map[string]any{}).Immutable(),
		field.JSON("exclude_student_ids", []uuid.UUID{}).Immutable(),
		field.String("participation").MaxLen(64).Default("individual").Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.UUID("question_set_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.JSON("params", map[string]any{}).Immutable(),
		field.Time("starts_at").Immutable(),
		field.Time("ends_at").Immutable(),
		field.Time("join_opens_at").Immutable(),
		field.String("status").MaxLen(64).Default("scheduled"),
		field.UUID("tournament_id", uuid.UUID{}).Optional().Nillable(),
		field.JSON("match_ids", []uuid.UUID{}).Optional(),
		field.String("cancel_reason").MaxLen(1000).Optional().Nillable(),
	}
}
func (Challenge) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("tenant_id", "school_id", "status", "starts_at").StorageKey("challenges_school_status_idx").Annotations(live),
		index.Fields("tenant_id", "starts_at", "id").StorageKey("challenges_keyset_idx").Annotations(live),
		index.Fields("status", "starts_at").StorageKey("challenges_due_idx").Annotations(live),
	}
}

// Tournament — competition.tournaments (T, COMPETITION_SYSTEM §10.2): a tournament of one school (grade tournament =
// owner_scope school + one grade) or of a tenant, always from a template version (rule_version_id). The lifecycle of
// §10.3 is driven by commands and the job tournament-scheduler; brackets and results live in PostgreSQL (ADR-0004).
// tenant_ids (hand-written uuid[] column) only prepares the cross-tenant platform tournaments (no API in F107).
type Tournament struct{ ent.Schema }

func (Tournament) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "tournaments"}}
}
func (Tournament) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "tournament_id"}}}
}
func (Tournament) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("owner_scope").MaxLen(64).Immutable(),
		field.UUID("owner_id", uuid.UUID{}).Immutable(),
		field.UUID("challenge_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("template_config_id", uuid.UUID{}).Immutable(),
		field.UUID("rule_version_id", uuid.UUID{}).Immutable(),
		field.String("name").MaxLen(128),
		field.String("format").MaxLen(64).Immutable(),
		field.String("participant_type").MaxLen(64).Default("individual").Immutable(),
		field.Int16("team_size").Optional().Nillable().Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.JSON("topic_ids", []uuid.UUID{}).Optional(),
		field.JSON("grade_levels", []int{}),
		field.JSON("question_policy", map[string]any{}).Optional(),
		field.Time("registration_open_at"),
		field.Time("registration_close_at"),
		field.Time("check_in_opens_at"),
		field.Time("starts_at"),
		field.Int16("max_entries"),
		field.Int16("min_entries"),
		field.String("seeding_policy").MaxLen(64),
		field.Int64("seed_value").Optional().Nillable(),
		field.Bool("auto_confirm").Default(false),
		field.JSON("reward_rule_refs", []map[string]any{}),
		field.String("reward_summary").MaxLen(1000).Optional().Nillable(),
		field.Int("round_gap_s").Default(300),
		field.String("status").MaxLen(64).Default("draft"),
		field.String("cancel_reason").MaxLen(1000).Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("rewarded_at").Optional().Nillable(),
	}
}
func (Tournament) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("tenant_id", "starts_at", "id").StorageKey("tournaments_keyset_idx").Annotations(live),
		index.Fields("tenant_id", "school_id", "status").StorageKey("tournaments_school_status_idx").Annotations(live),
		index.Fields("status", "starts_at").StorageKey("tournaments_due_idx").Annotations(live),
		index.Fields("challenge_id").Unique().StorageKey("tournaments_challenge_uq").
			Annotations(entsql.IndexWhere("challenge_id IS NOT NULL AND deleted_at IS NULL")),
	}
}

// TournamentEntry — competition.tournament_entries (T): one student, or one team (party_id, member_ids ≤ 8), of a
// tournament with its seed and group. A withdrawn entry keeps its row (status withdrawn, R21); registering again in
// the window revives it. member_ids always lists the students of the entry (one for an individual entry).
type TournamentEntry struct{ ent.Schema }

func (TournamentEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "tournament_entries"}}
}
func (TournamentEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "entry_id"}}}
}
func (TournamentEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tournament_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("party_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.JSON("member_ids", []uuid.UUID{}),
		field.Int16("seed").Optional().Nillable(),
		field.UUID("group_id", uuid.UUID{}).Optional().Nillable(),
		field.String("status").MaxLen(64).Default("registered"),
		field.UUID("registered_by", uuid.UUID{}),
		field.Time("checked_in_at").Optional().Nillable(),
	}
}
func (TournamentEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tournament_id", "student_id").Unique().StorageKey("tournament_entries_student_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL AND student_id IS NOT NULL")),
		index.Fields("tournament_id", "party_id").Unique().StorageKey("tournament_entries_party_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL AND party_id IS NOT NULL")),
		index.Fields("tenant_id", "tournament_id", "status").StorageKey("tournament_entries_status_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// TournamentGroup — competition.tournament_groups (T): a group of the group stage (groups_then_single_elim) or the one
// group of a round robin, with its last computed standings.
type TournamentGroup struct{ ent.Schema }

func (TournamentGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "tournament_groups"}}
}
func (TournamentGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "group_id"}}}
}
func (TournamentGroup) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tournament_id", uuid.UUID{}).Immutable(),
		field.String("code").MaxLen(16).Immutable(),
		field.JSON("standings", []map[string]any{}).Optional(),
	}
}
func (TournamentGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tournament_id", "code").Unique().StorageKey("tournament_groups_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// BracketNode — competition.bracket_nodes (T): one match slot of a bracket (winners, losers, grand_final, group) with its
// two entries, the winner, the nodes the winner/loser move to (slot 0 = entry_a, 1 = entry_b) and its state. attempts
// derives the proposal id of each ready check (F104 direct proposal), reschedule_count counts the server-failure
// reschedules (≤ 2, F107-AC-19), notices the sent tournament.match_scheduled (bit 1: 10 minutes, bit 2: 1 minute).
type BracketNode struct{ ent.Schema }

func (BracketNode) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "bracket_nodes"}}
}
func (BracketNode) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "node_id"}}}
}
func (BracketNode) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tournament_id", uuid.UUID{}).Immutable(),
		field.String("bracket").MaxLen(64).Immutable(),
		field.Int16("round").Immutable(),
		field.Int16("position").Immutable(),
		field.UUID("group_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("entry_a_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("entry_b_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("winner_entry_id", uuid.UUID{}).Optional().Nillable(),
		field.Bool("is_draw").Default(false),
		field.JSON("result", map[string]any{}).Optional(),
		field.UUID("match_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("scheduled_at").Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.UUID("next_win_node_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int16("next_win_slot").Optional().Nillable().Immutable(),
		field.UUID("next_lose_node_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int16("next_lose_slot").Optional().Nillable().Immutable(),
		field.Int16("wave").Default(0).Immutable(),
		field.Int16("attempts").Default(0),
		field.Int16("reschedule_count").Default(0),
		field.Int16("notices").Default(0),
		field.String("status").MaxLen(64).Default("pending"),
		field.Time("confirm_after").Optional().Nillable(),
		field.Bool("decision_required").Default(false),
		field.String("override_reason").MaxLen(1000).Optional().Nillable(),
		field.UUID("overridden_by", uuid.UUID{}).Optional().Nillable(),
	}
}
func (BracketNode) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("tournament_id", "bracket", "round", "position").Unique().StorageKey("bracket_nodes_slot_uq").Annotations(live),
		index.Fields("match_id").Unique().StorageKey("bracket_nodes_match_uq").Annotations(entsql.IndexWhere("match_id IS NOT NULL AND deleted_at IS NULL")),
		index.Fields("status", "scheduled_at").StorageKey("bracket_nodes_due_idx").Annotations(live),
	}
}

// TournamentResult — competition.tournament_results (TA): the final rank of an entry, written once when the tournament
// completes (unique tournament_id, entry_id — F107-AC-17, AC-21).
type TournamentResult struct{ ent.Schema }

func (TournamentResult) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "tournament_results"}}
}
func (TournamentResult) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "result_id"}}}
}
func (TournamentResult) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tournament_id", uuid.UUID{}).Immutable(),
		field.UUID("entry_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.Int16("final_rank").Immutable(),
		field.Int16("wins").Immutable(),
		field.Int16("losses").Immutable(),
		field.Int16("draws").Immutable(),
		field.Int("points").Immutable(),
		field.Int16("matches_played").Immutable(),
	}
}
func (TournamentResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tournament_id", "entry_id").Unique().StorageKey("tournament_results_entry_uq"),
	}
}

// TournamentReward — competition.tournament_rewards (TA): one reward granted to an entry by one rule of the template
// (unique tournament_id, entry_id, rule_id — F107-AC-20): granted at most once, then tournament.rewarded for module game.
type TournamentReward struct{ ent.Schema }

func (TournamentReward) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "tournament_rewards"}}
}
func (TournamentReward) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "reward_id"}}}
}
func (TournamentReward) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tournament_id", uuid.UUID{}).Immutable(),
		field.UUID("entry_id", uuid.UUID{}).Immutable(),
		field.String("rule_id").MaxLen(64).Immutable(),
		field.JSON("student_ids", []uuid.UUID{}).Immutable(),
		field.JSON("reward_ref", map[string]any{}).Immutable(),
		field.Int("points").Default(0).Immutable(),
		field.Time("granted_at").Default(nowUTC).Immutable(),
	}
}
func (TournamentReward) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tournament_id", "entry_id", "rule_id").Unique().StorageKey("tournament_rewards_rule_uq"),
	}
}

// TenantResourceAssignment — infra.tenant_resource_assignments (DATABASE_DESIGN §3.0, ADR-0029) of eduworld_competition:
// a tournament or challenge template (competition.game_configs) assigned to a tenant by the saga of module identity
// (F045 → facade applyTenantAssignments, F107-AC-29). Unassigning soft-deletes the row; tournaments already created keep
// their pinned template version.
type TenantResourceAssignment struct{ ent.Schema }

func (TenantResourceAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "infra", Table: "tenant_resource_assignments"}}
}
func (TenantResourceAssignment) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "assignment_id"}}}
}
func (TenantResourceAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.String("resource_type").MaxLen(64).Immutable(),
		field.UUID("resource_id", uuid.UUID{}).Immutable(),
		field.UUID("assigned_by", uuid.UUID{}).Immutable(),
		field.Time("assigned_at").Default(nowUTC).Immutable(),
	}
}
func (TenantResourceAssignment) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("resource_type", "resource_id", "tenant_id").Unique().StorageKey("tra_active_uq").Annotations(live),
		index.Fields("tenant_id", "resource_type", "resource_id").StorageKey("tra_tenant_idx").Annotations(live),
	}
}
