// Package schema holds the ent schemas of module identity (F010: tenancy, catalog, organization,
// users). Columns, lengths and names follow DATABASE_DESIGN §2.0–§2.2a and DDL §3.0–§3.2; what ent
// cannot express (CHECK, composite FK, RLS, partial unique on expressions, views) lives in the
// hand-written migration of the same feature and is covered by the schema drift test.
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Tenant — platform.tenants (G): the unit of data isolation and login (ADR-0005, ADR-0026).
type Tenant struct{ ent.Schema }

func (Tenant) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "platform", Table: "tenants"}}
}
func (Tenant) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "tenant_id"}}}
}
func (Tenant) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty(),
		field.String("name").MaxLen(255).NotEmpty(),
		field.String("tenant_type").MaxLen(64).Immutable(),
		field.String("status").MaxLen(64).Default("active"),
		field.UUID("keycloak_org_id", uuid.UUID{}).Optional().Nillable(),
		field.String("keycloak_org_alias").MaxLen(64).Optional().Nillable(),
		field.String("login_code").MaxLen(16).Optional().Nillable(),
		field.JSON("settings", map[string]any{}).Default(map[string]any{}),
		field.Bool("is_qa").Default(false),
	}
}
func (Tenant) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("code").Unique().StorageKey("tenants_code_uq").Annotations(live),
		index.Fields("keycloak_org_alias").Unique().StorageKey("tenants_kc_alias_uq").Annotations(live),
		index.Fields("keycloak_org_id").Unique().StorageKey("tenants_kc_org_uq").Annotations(live),
		index.Fields("login_code").Unique().StorageKey("tenants_login_code_uq").Annotations(live),
		index.Fields("tenant_type").Unique().StorageKey("tenants_single_common_uq").
			Annotations(entsql.IndexWhere("tenant_type = 'common'")),
	}
}

// Country — catalog.countries (G, ADR-0060): ISO 3166-1 alpha-2 key.
type Country struct{ ent.Schema }

func (Country) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "catalog", Table: "countries"}}
}
func (Country) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "country_code", StringLen: 2}}}
}
func (Country) Fields() []ent.Field {
	return []ent.Field{
		field.JSON("name_i18n", map[string]string{}),
		field.String("default_locale").MaxLen(16),
		field.String("status").MaxLen(64).Default("active"),
	}
}

// Province — catalog.provinces (G): validity period and successions (ADR-0060).
type Province struct{ ent.Schema }

func (Province) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "catalog", Table: "provinces"}}
}
func (Province) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "province_id"}}}
}
func (Province) Fields() []ent.Field {
	return []ent.Field{
		field.String("country_code").MaxLen(2),
		field.String("code").MaxLen(64).NotEmpty(),
		field.String("official_code").MaxLen(16).Optional().Nillable(),
		field.JSON("name_i18n", map[string]string{}),
		field.JSON("short_name_i18n", map[string]string{}).Optional(),
		field.String("province_type").MaxLen(64),
		dateField("effective_from"),
		optionalDate("effective_to"),
		field.String("status").MaxLen(64).Default("active"),
	}
}
func (Province) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique().StorageKey("provinces_code_uq"),
		index.Fields("country_code", "status").StorageKey("provinces_country_status_idx"),
	}
}

// ProvinceSuccession — catalog.province_successions (G): one successor per former unit.
type ProvinceSuccession struct{ ent.Schema }

func (ProvinceSuccession) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "catalog", Table: "province_successions"}}
}
func (ProvinceSuccession) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "old_province_id"}}}
}
func (ProvinceSuccession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("new_province_id", uuid.UUID{}),
		dateField("effective_date"),
		field.String("kind").MaxLen(64),
	}
}

var dateType = map[string]string{"postgres": "date"}

// dateField is a required calendar date (PostgreSQL date), never a timestamp.
func dateField(name string) ent.Field { return field.Time(name).SchemaType(dateType) }

// optionalDate is a nullable calendar date.
func optionalDate(name string) ent.Field {
	return field.Time(name).SchemaType(dateType).Optional().Nillable()
}

func nowUTC() time.Time { return time.Now().UTC() }
