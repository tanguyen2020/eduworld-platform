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

// School — org.schools (T): one tenant, one province; the common tenant has one virtual school.
type School struct{ ent.Schema }

func (School) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "org", Table: "schools"}}
}
func (School) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{PK: entx.PK{Column: "school_id"}}} }
func (School) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty(),
		field.String("name").MaxLen(255).NotEmpty(),
		field.JSON("name_i18n", map[string]string{}).Optional(),
		field.String("country_code").MaxLen(2),
		field.UUID("province_id", uuid.UUID{}).Optional().Nillable(),
		field.String("moet_code").MaxLen(32).Optional().Nillable(),
		field.String("school_level").MaxLen(64),
		field.Int16("grade_min"),
		field.Int16("grade_max"),
		field.String("address").MaxLen(500).Optional().Nillable(),
		field.String("status").MaxLen(64).Default("active"),
		field.Bool("is_virtual").Default(false),
		field.String("login_code").MaxLen(16).Optional().Nillable(),
		field.JSON("settings", map[string]any{}).Default(map[string]any{}),
	}
}
func (School) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("code").Unique().StorageKey("schools_code_uq").Annotations(live),
		index.Fields("login_code").Unique().StorageKey("schools_login_code_uq").Annotations(live),
		index.Fields("moet_code").Unique().StorageKey("schools_moet_code_uq").
			Annotations(entsql.IndexWhere("moet_code IS NOT NULL AND deleted_at IS NULL")),
		index.Fields("province_id", "school_level").StorageKey("schools_province_level_idx"),
		index.Fields("tenant_id", "status").StorageKey("schools_tenant_status_idx").Annotations(live),
	}
}

// AcademicYear — org.academic_years (T).
type AcademicYear struct{ ent.Schema }

func (AcademicYear) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "org", Table: "academic_years"}}
}
func (AcademicYear) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "academic_year_id"}}}
}
func (AcademicYear) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.String("name").MaxLen(32).NotEmpty(),
		dateField("start_date"),
		dateField("end_date"),
		field.Bool("is_current").Default(false),
	}
}
func (AcademicYear) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id").Unique().StorageKey("academic_years_current_uq").
			Annotations(entsql.IndexWhere("is_current AND deleted_at IS NULL")),
	}
}

// Student — org.students (T): student_id = identity.users.user_id.
type Student struct{ ent.Schema }

func (Student) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "org", Table: "students"}}
}
func (Student) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{PK: entx.PK{Column: "student_id"}}} }
func (Student) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}),
		field.Int16("grade_level"),
		field.String("student_code").MaxLen(32).Optional().Nillable(),
		field.String("full_name").MaxLen(128).Sensitive(),
		field.Bytes("date_of_birth_enc").Optional().Nillable().Sensitive(),
		field.String("national_id_hash").MaxLen(64).Optional().Nillable().Sensitive(),
		field.String("status").MaxLen(64).Default("active"),
	}
}
func (Student) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "student_code").Unique().StorageKey("students_code_uq").
			Annotations(entsql.IndexWhere("student_code IS NOT NULL AND deleted_at IS NULL")),
	}
}
