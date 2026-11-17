// Package entx is the data-access kit of every module (F003 US1/US2, ADR-0039, ADR-0089): one mixin per
// table kind with its global filter (interceptor) and mutation hook, soft-delete, and the write/read
// transaction scopes UnitOfWork / ReadSession that set app.tenant_id for RLS.
package entx

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Kind is the table kind of DATABASE_DESIGN §1 (ADR-0089 §2).
type Kind int

const (
	KindT      Kind = iota + 1 // Base: tenant-scoped, soft-delete
	KindTA                     // TenantAppendOnly
	KindTStar                  // TenantOptional: tenant nullable, soft-delete
	KindTAStar                 // TenantOptionalAppendOnly
	KindH                      // HybridContent: tenant rows + platform rows assigned to a tenant
	KindG                      // GlobalBase: no tenant, soft-delete
	KindGA                     // AppendOnly: no tenant, no soft-delete
)

func (k Kind) tenant() bool     { return k != KindG && k != KindGA }
func (k Kind) nullable() bool   { return k == KindTStar || k == KindTAStar || k == KindH }
func (k Kind) softDelete() bool { return k == KindT || k == KindTStar || k == KindH || k == KindG }

// Kinded is implemented by every entx mixin (mixin coverage test, ADR-0089 §2).
type Kinded interface{ TableKind() Kind }

func idField() ent.Field { return field.UUID("id", uuid.UUID{}).Default(ids.New).Immutable() }

func auditCreate() []ent.Field {
	return []ent.Field{
		field.Time("created_at").Default(nowUTC).Immutable(),
		field.UUID("created_by", uuid.UUID{}).Optional().Nillable().Immutable(),
	}
}

func auditUpdateDelete() []ent.Field {
	return []ent.Field{
		field.Time("updated_at").Default(nowUTC).UpdateDefault(nowUTC),
		field.UUID("updated_by", uuid.UUID{}).Optional().Nillable(),
		field.Time("deleted_at").Optional().Nillable(),
		field.UUID("deleted_by", uuid.UUID{}).Optional().Nillable(),
		field.String("delete_reason").MaxLen(64).Optional().Nillable(),
		field.Int64("version").Default(1),
	}
}

func nowUTC() time.Time { return time.Now().UTC() }

func tenantField(nullable bool) ent.Field {
	f := field.UUID("tenant_id", uuid.UUID{}).Immutable()
	if nullable {
		f = f.Optional().Nillable()
	}
	return f
}

func fields(k Kind) []ent.Field {
	out := []ent.Field{idField()}
	if k.tenant() {
		out = append(out, tenantField(k.nullable()))
	}
	out = append(out, auditCreate()...)
	if k.softDelete() {
		out = append(out, auditUpdateDelete()...)
	}
	return out
}

// Base is the mixin of T tables (tenant-scoped, soft-delete, optimistic version).
type Base struct{ mixin.Schema }

func (Base) TableKind() Kind                 { return KindT }
func (Base) Fields() []ent.Field             { return fields(KindT) }
func (Base) Interceptors() []ent.Interceptor { return []ent.Interceptor{filter(KindT, nil)} }
func (Base) Hooks() []ent.Hook               { return []ent.Hook{mutationHook(KindT, nil)} }

// TenantAppendOnly is the mixin of TA tables (tenant-scoped, append-only).
type TenantAppendOnly struct{ mixin.Schema }

func (TenantAppendOnly) TableKind() Kind     { return KindTA }
func (TenantAppendOnly) Fields() []ent.Field { return fields(KindTA) }
func (TenantAppendOnly) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{filter(KindTA, nil)}
}
func (TenantAppendOnly) Hooks() []ent.Hook { return []ent.Hook{mutationHook(KindTA, nil)} }

// TenantOptional is the mixin of T* tables: tenant_id nullable; NULL rows only under PlatformScope.
type TenantOptional struct{ mixin.Schema }

func (TenantOptional) TableKind() Kind     { return KindTStar }
func (TenantOptional) Fields() []ent.Field { return fields(KindTStar) }
func (TenantOptional) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{filter(KindTStar, nil)}
}
func (TenantOptional) Hooks() []ent.Hook { return []ent.Hook{mutationHook(KindTStar, nil)} }

// TenantOptionalAppendOnly is the mixin of TA* tables.
type TenantOptionalAppendOnly struct{ mixin.Schema }

func (TenantOptionalAppendOnly) TableKind() Kind     { return KindTAStar }
func (TenantOptionalAppendOnly) Fields() []ent.Field { return fields(KindTAStar) }
func (TenantOptionalAppendOnly) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{filter(KindTAStar, nil)}
}
func (TenantOptionalAppendOnly) Hooks() []ent.Hook { return []ent.Hook{mutationHook(KindTAStar, nil)} }

// HybridContent is the mixin of H tables (former PlatformBase): tenant rows, plus platform rows
// (tenant_id NULL) visible to a tenant only when assigned in infra.tenant_resource_assignments
// (ADR-0029). ResourceType is the resource_type of the assignment; the row id is matched.
type HybridContent struct {
	mixin.Schema
	ResourceType string
}

func (HybridContent) TableKind() Kind { return KindH }

// Fields adds owner_scope (platform/tenant) to the T* fields.
func (HybridContent) Fields() []ent.Field {
	return append(fields(KindH), field.String("owner_scope").MaxLen(64).Default("tenant"))
}
func (h HybridContent) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{filter(KindH, &hybrid{resourceType: h.ResourceType})}
}
func (h HybridContent) Hooks() []ent.Hook {
	return []ent.Hook{mutationHook(KindH, &hybrid{resourceType: h.ResourceType})}
}

// GlobalBase is the mixin of G tables (no tenant, soft-delete); no scope needed.
type GlobalBase struct{ mixin.Schema }

func (GlobalBase) TableKind() Kind                 { return KindG }
func (GlobalBase) Fields() []ent.Field             { return fields(KindG) }
func (GlobalBase) Interceptors() []ent.Interceptor { return []ent.Interceptor{filter(KindG, nil)} }
func (GlobalBase) Hooks() []ent.Hook               { return []ent.Hook{mutationHook(KindG, nil)} }

// AppendOnly is the mixin of GA tables (no tenant, no soft-delete): no filter; delete is forbidden.
type AppendOnly struct{ mixin.Schema }

func (AppendOnly) TableKind() Kind     { return KindGA }
func (AppendOnly) Fields() []ent.Field { return fields(KindGA) }
func (AppendOnly) Hooks() []ent.Hook   { return []ent.Hook{mutationHook(KindGA, nil)} }
