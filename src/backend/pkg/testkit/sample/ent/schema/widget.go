// Package schema holds the ent schemas of the sample module used by the F003 tests (one entx mixin per
// entity, MaxLen on every string).
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Widget is a T table (Base): tenant-scoped, soft-delete, optimistic version.
type Widget struct{ ent.Schema }

func (Widget) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sample", Table: "widgets"}}
}
func (Widget) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{}} }
func (Widget) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(32).NotEmpty(),
		field.String("name").MaxLen(128).NotEmpty(),
	}
}
func (Widget) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "code").Unique().Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// WidgetLog is a TA table (TenantAppendOnly).
type WidgetLog struct{ ent.Schema }

func (WidgetLog) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sample", Table: "widget_logs"}}
}
func (WidgetLog) Mixin() []ent.Mixin { return []ent.Mixin{entx.TenantAppendOnly{}} }
func (WidgetLog) Fields() []ent.Field {
	return []ent.Field{field.String("message").MaxLen(255)}
}

// Template is a T* table (TenantOptional): tenant rows plus platform rows (tenant_id NULL).
type Template struct{ ent.Schema }

func (Template) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sample", Table: "templates"}}
}
func (Template) Mixin() []ent.Mixin { return []ent.Mixin{entx.TenantOptional{}} }
func (Template) Fields() []ent.Field {
	return []ent.Field{field.String("name").MaxLen(128)}
}

// Unit is a G table (GlobalBase): no tenant.
type Unit struct{ ent.Schema }

func (Unit) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sample", Table: "units"}}
}
func (Unit) Mixin() []ent.Mixin { return []ent.Mixin{entx.GlobalBase{}} }
func (Unit) Fields() []ent.Field {
	return []ent.Field{field.String("symbol").MaxLen(16)}
}
