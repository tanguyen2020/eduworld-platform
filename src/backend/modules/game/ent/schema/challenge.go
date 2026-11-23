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

// F076 — Challenge Engine solo: challenges are game content with versions on the F071 release core (G, ADR-0043),
// assigned to tenants (infra.tenant_resource_assignments, ADR-0029); a ChallengeAttempt is the play of a student on a
// pinned version (T, DATABASE_DESIGN §2.8a). The attempt row is the checkpoint of its room (pkg/roomhost).

// Challenge — game.challenges (G): identity of a platform challenge and its release pointer.
type Challenge struct{ ent.Schema }

func (Challenge) Annotations() []schema.Annotation { return table("challenges") }
func (Challenge) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "challenge_id"}}}
}
func (Challenge) Fields() []ent.Field {
	return append([]ent.Field{
		field.String("code").MaxLen(64).Optional().Nillable(),
		field.String("owner_scope").MaxLen(64).Default("platform").Immutable(),
	}, pointerFields()...)
}
func (Challenge) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("challenges_code_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// ChallengeVersion — game.challenge_versions (G): payload of specs/schemas/challenge.schema.json plus the columns the
// player queries filter on (type, mode, grades, subject, area, availability).
type ChallengeVersion struct{ ent.Schema }

func (ChallengeVersion) Annotations() []schema.Annotation { return table("challenge_versions") }
func (ChallengeVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "challenge_version_id"}}}
}
func (ChallengeVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("challenge_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.String("challenge_type").MaxLen(64).Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.JSON("grade_levels", []int{}).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.JSON("objective_ids", []string{}).Immutable(),
		field.String("area_id").MaxLen(64).Optional().Nillable().Immutable(),
		field.Time("available_from").Optional().Nillable().Immutable(),
		field.Time("available_to").Optional().Nillable().Immutable(),
	)
}
func (ChallengeVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("challenge_id", "version_no").Unique().StorageKey("challenge_versions_no_uq")}
}

// ChallengeAttempt — game.challenge_attempts (T, DATABASE_DESIGN §2.8a, CHALLENGE_BOSS_SYSTEM §2.3): one play of a
// student on the version pinned at start; score, lives and goal change only from graded results (ADR-0016). At most one
// attempt in progress per student.
type ChallengeAttempt struct{ ent.Schema }

func (ChallengeAttempt) Annotations() []schema.Annotation { return table("challenge_attempts") }
func (ChallengeAttempt) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "challenge_attempt_id"}}}
}
func (ChallengeAttempt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("challenge_id", uuid.UUID{}).Immutable(),
		field.UUID("challenge_version_id", uuid.UUID{}).Immutable(),
		field.String("challenge_type").MaxLen(64).Immutable(),
		field.String("difficulty").MaxLen(64).Immutable(),
		field.UUID("party_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("status").MaxLen(64),
		field.String("end_reason").MaxLen(64).Optional().Nillable(),
		field.Int("score").Default(0),
		field.Int("items_answered").Default(0),
		field.Int("correct_streak").Default(0),
		field.Int("lives_left").Optional().Nillable(),
		field.Float("goal_progress").Default(0).SchemaType(map[string]string{"postgres": "numeric(10,2)"}),
		field.Int("activities_completed").Default(0),
		field.String("period_key").MaxLen(16).Immutable(),
		field.Bool("first_clear").Default(false),
		field.Time("started_at").Default(nowUTC).Immutable(),
		field.Time("deadline_at").Optional().Nillable().Immutable(),
		field.Time("ended_at").Optional().Nillable(),
		field.JSON("learning_attempt_ids", []string{}),
		field.JSON("processed_attempt_ids", []string{}),
	}
}
func (ChallengeAttempt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("student_id").Unique().StorageKey("challenge_attempts_one_active_uq").
			Annotations(entsql.IndexWhere("status = 'in_progress' AND deleted_at IS NULL")),
		index.Fields("tenant_id", "student_id", "started_at").StorageKey("challenge_attempts_student_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "student_id", "challenge_id", "period_key").StorageKey("challenge_attempts_period_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
