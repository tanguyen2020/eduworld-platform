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

// F103 — Boss battle with several phases and co-op quests: bosses are game content with versions on the F071 release
// core (G, ADR-0043), assigned to tenants (ADR-0029); a BossEncounter is the play of a student (solo) or of a party
// on a pinned version (T, DATABASE_DESIGN §2.8a, CHALLENGE_BOSS_SYSTEM §6). The encounter rows are the checkpoint of
// the room of the encounter (pkg/roomhost): Valkey only holds the registry and the lease (ADR-0004). A
// PartyQuestSession is the shared progress of a party on a quest version (research R4).

// BossDefinition — game.boss_definitions (G): identity of a platform boss and its release pointer.
type BossDefinition struct{ ent.Schema }

func (BossDefinition) Annotations() []schema.Annotation { return table("boss_definitions") }
func (BossDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "boss_id"}}}
}
func (BossDefinition) Fields() []ent.Field {
	return append([]ent.Field{
		field.String("code").MaxLen(64).Optional().Nillable(),
		field.String("owner_scope").MaxLen(64).Default("platform").Immutable(),
	}, pointerFields()...)
}
func (BossDefinition) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("boss_definitions_code_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// BossVersion — game.boss_versions (G): payload of specs/schemas/boss.schema.json plus the columns the player queries
// filter on (code, mode, grades, subject, area).
type BossVersion struct{ ent.Schema }

func (BossVersion) Annotations() []schema.Annotation { return table("boss_versions") }
func (BossVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "boss_version_id"}}}
}
func (BossVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("boss_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.JSON("grade_levels", []int{}).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.String("area_id").MaxLen(64).Immutable(),
		field.Int("phase_count").Immutable(),
	)
}
func (BossVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("boss_id", "version_no").Unique().StorageKey("boss_versions_no_uq")}
}

// BossEncounter — game.boss_encounters (T): one encounter on the pinned boss version, its phase, energy, retries and
// deadlines on the server clock, and its result. party_activity_id is the idempotency key of startPartyActivity.
type BossEncounter struct{ ent.Schema }

func (BossEncounter) Annotations() []schema.Annotation { return table("boss_encounters") }
func (BossEncounter) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "encounter_id"}}}
}
func (BossEncounter) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("boss_id", uuid.UUID{}).Immutable(),
		field.UUID("boss_version_id", uuid.UUID{}).Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.UUID("party_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("party_activity_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("leader_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("status").MaxLen(64),
		field.Int("phase_index").Default(0),
		field.String("current_phase").MaxLen(64).Optional().Nillable(),
		field.Int("phase_retries").Default(0),
		field.Int("plan_failures").Default(0),
		field.Int("phases_cleared").Default(0),
		field.Float("energy_remaining").Default(1).SchemaType(map[string]string{"postgres": "numeric(5,4)"}),
		field.Int64("state_version").Default(0),
		field.Time("ready_deadline_at").Immutable(),
		field.Time("deadline_at").Optional().Nillable(),
		field.Time("phase_deadline_at").Optional().Nillable(),
		field.String("outcome").MaxLen(64).Optional().Nillable(),
		field.String("end_reason").MaxLen(64).Optional().Nillable(),
		field.String("period_key").MaxLen(16).Immutable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("ended_at").Optional().Nillable(),
	}
}
func (BossEncounter) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("party_activity_id").Unique().StorageKey("boss_encounters_party_activity_uq").
			Annotations(entsql.IndexWhere("party_activity_id IS NOT NULL AND deleted_at IS NULL")),
		index.Fields("tenant_id", "party_id").StorageKey("boss_encounters_party_idx").
			Annotations(entsql.IndexWhere("party_id IS NOT NULL AND deleted_at IS NULL")),
		index.Fields("status").StorageKey("boss_encounters_live_idx").
			Annotations(entsql.IndexWhere("status IN ('lobby','phase_active') AND deleted_at IS NULL")),
	}
}

// BossEncounterMember — game.boss_encounter_members (T): a member of an encounter with the grade it joined with
// (per_grade), its ready flag, its join order, its disconnection and its leave. eligible is fixed when the encounter ends.
type BossEncounterMember struct{ ent.Schema }

func (BossEncounterMember) Annotations() []schema.Annotation { return table("boss_encounter_members") }
func (BossEncounterMember) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "boss_encounter_member_id"}}}
}
func (BossEncounterMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("encounter_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.Int("grade_level").Immutable(),
		field.Int("join_order").Immutable(),
		field.Bool("ready").Default(false),
		field.Time("joined_at").Immutable(),
		field.Time("disconnected_at").Optional().Nillable(),
		field.Time("left_at").Optional().Nillable(),
		field.String("left_reason").MaxLen(64).Optional().Nillable(),
		field.Bool("eligible").Optional().Nillable(),
		field.Bool("first_defeat").Default(false),
	}
}
func (BossEncounterMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("encounter_id", "student_id").Unique().StorageKey("boss_encounter_members_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "student_id", "joined_at").StorageKey("boss_encounter_members_student_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// BossEncounterContribution — game.boss_encounter_contributions (T, CHALLENGE_BOSS_SYSTEM §3.4): the contribution of a
// member to one phase (current round). Never shown to the other members beyond contribution_met (F103-AC-09).
type BossEncounterContribution struct{ ent.Schema }

func (BossEncounterContribution) Annotations() []schema.Annotation {
	return table("boss_encounter_contributions")
}
func (BossEncounterContribution) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "contribution_id"}}}
}
func (BossEncounterContribution) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("encounter_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("phase_id").MaxLen(64).Immutable(),
		field.Int("correct_count").Default(0),
		field.Int("activities_completed").Default(0),
		field.Float("contribution_pct").Default(0).SchemaType(map[string]string{"postgres": "numeric(5,2)"}),
		field.Bool("eligible").Default(false),
		// cleared: the phase was cleared while the member was present (it counts for the eligibility of the member).
		field.Bool("cleared").Default(false),
	}
}
func (BossEncounterContribution) Indexes() []ent.Index {
	return []ent.Index{index.Fields("encounter_id", "student_id", "phase_id").Unique().StorageKey("boss_encounter_contributions_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// BossPhasePlan — game.boss_phase_plans (T, research R1): the question plan module learning made for one member, one
// requirement, one round of a phase; items are the render payloads only (no answer data, ADR-0016).
type BossPhasePlan struct{ ent.Schema }

func (BossPhasePlan) Annotations() []schema.Annotation { return table("boss_phase_plans") }
func (BossPhasePlan) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "boss_phase_plan_id"}}}
}
func (BossPhasePlan) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("encounter_id", uuid.UUID{}).Immutable(),
		field.String("phase_id").MaxLen(64).Immutable(),
		field.Int("retry_no").Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("requirement_id").MaxLen(64).Immutable(),
		field.UUID("plan_id", uuid.UUID{}).Immutable(),
		field.Int("item_count").Immutable(),
		field.JSON("items", []map[string]any{}).Immutable(),
		field.Time("expires_at").Immutable(),
	}
}
func (BossPhasePlan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("encounter_id", "phase_id", "retry_no", "student_id", "requirement_id").Unique().StorageKey("boss_phase_plans_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("plan_id").StorageKey("boss_phase_plans_plan_idx").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// BossAnswerResult — game.boss_answer_results (TA): the graded result of one answer of a member (once per
// (plan_id, item_index, student_id)); counted = counted in the contribution (false: late, closed round, rapid guess). No
// raw response (R14).
type BossAnswerResult struct{ ent.Schema }

func (BossAnswerResult) Annotations() []schema.Annotation { return table("boss_answer_results") }
func (BossAnswerResult) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "answer_result_id"}}}
}
func (BossAnswerResult) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("encounter_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("plan_id", uuid.UUID{}).Immutable(),
		field.Int("item_index").Immutable(),
		field.String("phase_id").MaxLen(64).Immutable(),
		field.Int("retry_no").Immutable(),
		field.String("requirement_id").MaxLen(64).Immutable(),
		field.Bool("is_correct").Immutable(),
		field.Bool("counted").Immutable(),
		field.Bool("rapid_guess").Default(false).Immutable(),
		field.Time("received_at").Immutable(),
	}
}
func (BossAnswerResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plan_id", "item_index", "student_id").Unique().StorageKey("boss_answer_results_uq"),
		index.Fields("encounter_id", "student_id").StorageKey("boss_answer_results_member_idx"),
	}
}

// PartyQuestSession — game.party_quest_sessions (T, research R4): the shared progress of a party on a quest version
// (shared steps, roles of interdependent steps, shared clue cards — never their content) next to the personal
// QuestProgress of every member.
type PartyQuestSession struct{ ent.Schema }

func (PartyQuestSession) Annotations() []schema.Annotation { return table("party_quest_sessions") }
func (PartyQuestSession) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "party_quest_session_id"}}}
}
func (PartyQuestSession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("party_id", uuid.UUID{}).Immutable(),
		field.UUID("party_activity_id", uuid.UUID{}).Immutable(),
		field.UUID("leader_id", uuid.UUID{}),
		field.UUID("quest_id", uuid.UUID{}).Immutable(),
		field.UUID("quest_version_id", uuid.UUID{}).Immutable(),
		field.String("party_mode").MaxLen(64).Immutable(),
		field.String("shared_progress_policy").MaxLen(64).Immutable(),
		field.Int("party_size_min").Immutable(),
		field.String("status").MaxLen(64),
		field.JSON("member_ids", []string{}),
		field.JSON("member_states", []map[string]any{}),
		field.JSON("shared_step_states", map[string]any{}),
		field.JSON("role_assignments", map[string]any{}),
		field.JSON("shared_clues", []map[string]any{}),
		field.Int64("state_version").Default(0),
		field.Time("started_at").Immutable(),
		field.Time("ended_at").Optional().Nillable(),
		field.String("end_reason").MaxLen(64).Optional().Nillable(),
	}
}
func (PartyQuestSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("party_activity_id").Unique().StorageKey("party_quest_sessions_activity_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "party_id", "quest_id").StorageKey("party_quest_sessions_party_idx").
			Annotations(entsql.IndexWhere("status = 'active' AND deleted_at IS NULL")),
	}
}

// PartyQuestContribution — game.party_quest_contributions (T): the metric a member contributed to a collective step
// of a session (from its own graded attempts) and whether it reached min_contribution.
type PartyQuestContribution struct{ ent.Schema }

func (PartyQuestContribution) Annotations() []schema.Annotation {
	return table("party_quest_contributions")
}
func (PartyQuestContribution) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "party_quest_contribution_id"}}}
}
func (PartyQuestContribution) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("party_quest_session_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("step_id").MaxLen(64).Immutable(),
		field.Int("metric_value").Default(0),
		field.Bool("min_contribution_met").Default(false),
		field.JSON("attempt_ids", []string{}),
	}
}
func (PartyQuestContribution) Indexes() []ent.Index {
	return []ent.Index{index.Fields("party_quest_session_id", "student_id", "step_id").Unique().StorageKey("party_quest_contributions_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}
