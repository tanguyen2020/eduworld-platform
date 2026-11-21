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

// Match — competition.matches (G*, DATABASE_DESIGN §2.10, F104): a match accepted by every side of a matchmaking
// proposal (match_id = proposal_id, so creating it again is a no-op), of a friend duel or of a practice bot match. A
// match may span schools (and tenants for platform events only): it has no tenant_id but tenant_ids (hand-written
// uuid[] column) and is reached through competition.match_participants (RLS by tenant) in the use cases. F104 creates
// it in status lobby; the battle engine (F105) moves it on and writes question_plan.
type Match struct{ ent.Schema }

func (Match) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "matches"}}
}
func (Match) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "match_id"}}}
}
func (Match) Fields() []ent.Field {
	return []ent.Field{
		field.String("mode").MaxLen(64).Immutable(),
		field.String("matchmaking_type").MaxLen(64).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.UUID("topic_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int16("grade_level").Immutable(),
		field.String("status").MaxLen(64).Default("lobby"),
		field.Bool("ranked").Immutable(),
		field.Bool("is_bot").Default(false).Immutable(),
		field.String("scope_kind").MaxLen(64).Immutable(),
		field.UUID("tournament_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("tournament_node_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("duel_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("ruleset_version_id", uuid.UUID{}).Immutable(),
		field.JSON("question_plan", map[string]any{}).Optional(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("ended_at").Optional().Nillable(),
	}
}
func (Match) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tournament_id").StorageKey("matches_tournament_idx").Annotations(entsql.IndexWhere("tournament_id IS NOT NULL")),
		index.Fields("status", "created_at").StorageKey("matches_status_created_idx"),
	}
}

// MatchParticipant — competition.match_participants (T, DATABASE_DESIGN §2.10): one student of a match with its team.
// participant_id is the row key (ent needs one id column); the business key (match_id, student_id) is unique. The
// rating at match time is internal (never returned to a client); outcome, score and connected_ms are written by F105.
type MatchParticipant struct{ ent.Schema }

func (MatchParticipant) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "match_participants"}}
}
func (MatchParticipant) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "participant_id"}}}
}
func (MatchParticipant) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("match_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.Int16("team_no").Immutable(),
		field.UUID("party_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("outcome").MaxLen(64).Optional().Nillable(),
		field.Float("score").Optional().Nillable().SchemaType(map[string]string{"postgres": "numeric(9,3)"}),
		field.Int64("connected_ms").Optional().Nillable(),
		field.Float("rating_mu_at_match").Optional().Nillable().Immutable().
			SchemaType(map[string]string{"postgres": "numeric(8,3)"}),
		field.Float("rating_rd_at_match").Optional().Nillable().Immutable().
			SchemaType(map[string]string{"postgres": "numeric(8,3)"}),
	}
}
func (MatchParticipant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("match_id", "student_id").Unique().StorageKey("match_participants_match_student_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "student_id", "created_at").StorageKey("match_participants_student_idx"),
	}
}

// Duel — competition.duels (T, DATABASE_DESIGN §2.10, MATCHMAKING §8): a friend challenge, unranked, pending for 60 s.
// At most one pending duel per (challenger, opponent).
type Duel struct{ ent.Schema }

func (Duel) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "duels"}}
}
func (Duel) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "duel_id"}}}
}
func (Duel) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("challenger_id", uuid.UUID{}).Immutable(),
		field.UUID("opponent_id", uuid.UUID{}).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.UUID("topic_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("status").MaxLen(32).Default("pending"),
		field.UUID("match_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("expires_at").Immutable(),
		field.Time("responded_at").Optional().Nillable(),
	}
}
func (Duel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("challenger_id", "opponent_id").Unique().StorageKey("duels_pending_uq").
			Annotations(entsql.IndexWhere("status = 'pending' AND deleted_at IS NULL")),
		index.Fields("expires_at").StorageKey("duels_expiry_idx").Annotations(entsql.IndexWhere("status = 'pending' AND deleted_at IS NULL")),
		index.Fields("tenant_id", "opponent_id", "status").StorageKey("duels_opponent_idx"),
	}
}

// CompetitionRestriction — competition.competition_restrictions (T, F104 data-model): a time-boxed restriction of
// ranked play (abandon_penalty written by F105, restrict_competition written by the handling of violations). F104 only
// reads it when creating a ticket and a proposal (H5).
type CompetitionRestriction struct{ ent.Schema }

func (CompetitionRestriction) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "competition_restrictions"}}
}
func (CompetitionRestriction) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "restriction_id"}}}
}
func (CompetitionRestriction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("kind").MaxLen(64).Immutable(),
		field.String("reason").MaxLen(1000),
		field.Time("starts_at"),
		field.Time("ends_at"),
	}
}
func (CompetitionRestriction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "student_id", "ends_at").StorageKey("competition_restrictions_student_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// GameConfig — competition.game_configs (G, COMPETITION_SYSTEM §10, F104 data-model): a versioned platform
// configuration of competition (matchmaking-configs, ranking-rules…). Written by the Game Operations Console (F108);
// F104 reads the published matchmaking configuration only.
type GameConfig struct{ ent.Schema }

func (GameConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "game_configs"}}
}
func (GameConfig) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "config_id"}}}
}
func (GameConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("config_type").MaxLen(64).Immutable(),
		field.String("code").MaxLen(64).Immutable(),
		field.UUID("current_version_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("latest_version_no").Default(0),
		field.String("owner_scope").MaxLen(64).Default("platform"),
	}
}
func (GameConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("config_type", "code").Unique().StorageKey("game_configs_type_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// GameConfigVersion — competition.game_config_versions (G): one version of a GameConfig; a published version is
// immutable (trigger, R11/ADR-0015).
type GameConfigVersion struct{ ent.Schema }

func (GameConfigVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "game_config_versions"}}
}
func (GameConfigVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "config_version_id"}}}
}
func (GameConfigVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("config_id", uuid.UUID{}).Immutable(),
		field.Int("version_no").Immutable(),
		field.String("status").MaxLen(64).Default("draft"),
		field.String("title").MaxLen(255),
		field.JSON("payload", map[string]any{}),
		field.Time("effective_from").Optional().Nillable(),
		field.UUID("author_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("reviewer_id", uuid.UUID{}).Optional().Nillable(),
		field.String("change_note").MaxLen(500).Optional().Nillable(),
		field.UUID("based_on_version_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("published_at").Optional().Nillable(),
	}
}
func (GameConfigVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("config_id", "version_no").Unique().StorageKey("game_config_versions_config_version_uq"),
		index.Fields("config_id", "status", "effective_from").StorageKey("game_config_versions_active_idx"),
	}
}

// ContentRelease — competition.content_releases (GA, ADR-0043): append-only log of publish/rollback/unpublish of a
// GameConfig (written by F108).
type ContentRelease struct{ ent.Schema }

func (ContentRelease) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "competition", Table: "content_releases"}}
}
func (ContentRelease) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.AppendOnly{PK: entx.PK{Column: "release_id"}}}
}
func (ContentRelease) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("config_id", uuid.UUID{}).Immutable(),
		field.String("action").MaxLen(64).Immutable(),
		field.UUID("from_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("to_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(1000).Optional().Nillable().Immutable(),
		field.UUID("released_by", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("released_at").Default(nowUTC).Immutable(),
	}
}
func (ContentRelease) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("config_id", "released_at").StorageKey("content_releases_config_idx"),
	}
}
