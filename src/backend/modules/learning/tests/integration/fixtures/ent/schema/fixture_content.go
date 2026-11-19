// Package schema is the ent schema of the F020 test fixture.
//
// TEST FIXTURE — không dùng trong runtime (R17): F020 has no concrete content type yet (F021/F022/F024 add
// them), so its integration tests exercise the shared mixins, RLS, trash, assignment and clone on these
// tables, created by the tests only (never by a migration).
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	learningschema "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/schema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// FixtureResourceType is the resource type the fixture stands in for.
const FixtureResourceType = "question"

// FixtureContent — content.fixture_contents (H): a content item.
type FixtureContent struct{ ent.Schema }

func (FixtureContent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "fixture_contents"}}
}
func (FixtureContent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entx.HybridContent{ResourceType: FixtureResourceType, PK: entx.PK{Column: "fixture_content_id"}},
		learningschema.ContentItemMixin{},
	}
}
func (FixtureContent) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty(),
		field.String("title").MaxLen(255),
	}
}
func (FixtureContent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").Unique().StorageKey("fixture_contents_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// FixtureContentVersion — content.fixture_content_versions (H, visible through its item's assignment).
type FixtureContentVersion struct{ ent.Schema }

func (FixtureContentVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "fixture_content_versions"}}
}
func (FixtureContentVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entx.HybridContent{ResourceType: FixtureResourceType, PK: entx.PK{Column: "fixture_content_version_id"},
			ResourceIDColumn: "fixture_content_id"},
		learningschema.ContentVersionMixin{},
	}
}
func (FixtureContentVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("fixture_content_id", uuid.UUID{}).Immutable(),
		field.String("body").MaxLen(2000),
	}
}
func (FixtureContentVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("fixture_content_id", "version_no").Unique().StorageKey("fixture_content_versions_no_uq")}
}

// FixtureAttempt — content.fixture_attempts (T): student data that a clone must never copy (R22).
type FixtureAttempt struct{ ent.Schema }

func (FixtureAttempt) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "fixture_attempts"}}
}
func (FixtureAttempt) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "fixture_attempt_id"}}}
}
func (FixtureAttempt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("fixture_content_version_id", uuid.UUID{}).Immutable(),
		field.Int("score").Default(0),
	}
}
