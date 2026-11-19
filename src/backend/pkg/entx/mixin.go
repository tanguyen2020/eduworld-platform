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

// PK names the primary-key column of an entity when it is not "id" (DATABASE_DESIGN names keys per
// table: school_id, tenant_id, …). StringLen > 0 makes it a varchar(StringLen) key without default
// (catalog.countries.country_code); otherwise it is a UUIDv7 defaulting to ids.New.
type PK struct {
	Column    string
	StringLen int
}

// column returns the storage column of the key ("id" by default).
func (p PK) column() string {
	if p.Column == "" {
		return "id"
	}
	return p.Column
}

func idField(p PK) ent.Field {
	if p.StringLen > 0 {
		f := field.String("id").MaxLen(p.StringLen).NotEmpty().Immutable()
		if p.Column != "" {
			f = f.StorageKey(p.Column)
		}
		return f
	}
	f := field.UUID("id", uuid.UUID{}).Default(ids.New).Immutable()
	if p.Column != "" {
		f = f.StorageKey(p.Column)
	}
	return f
}

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

func fields(k Kind, p PK) []ent.Field {
	out := []ent.Field{idField(p)}
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
type Base struct {
	mixin.Schema
	PK PK
}

func (Base) TableKind() Kind                 { return KindT }
func (m Base) Fields() []ent.Field           { return fields(KindT, m.PK) }
func (Base) Interceptors() []ent.Interceptor { return []ent.Interceptor{filter(KindT, nil)} }
func (Base) Hooks() []ent.Hook               { return []ent.Hook{mutationHook(KindT, nil)} }

// TenantAppendOnly is the mixin of TA tables (tenant-scoped, append-only).
type TenantAppendOnly struct {
	mixin.Schema
	PK PK
}

func (TenantAppendOnly) TableKind() Kind       { return KindTA }
func (m TenantAppendOnly) Fields() []ent.Field { return fields(KindTA, m.PK) }
func (TenantAppendOnly) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{filter(KindTA, nil)}
}
func (TenantAppendOnly) Hooks() []ent.Hook { return []ent.Hook{mutationHook(KindTA, nil)} }

// TenantOptional is the mixin of T* tables: tenant_id nullable; NULL rows only under PlatformScope.
type TenantOptional struct {
	mixin.Schema
	PK PK
}

func (TenantOptional) TableKind() Kind       { return KindTStar }
func (m TenantOptional) Fields() []ent.Field { return fields(KindTStar, m.PK) }
func (TenantOptional) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{filter(KindTStar, nil)}
}
func (TenantOptional) Hooks() []ent.Hook { return []ent.Hook{mutationHook(KindTStar, nil)} }

// TenantOptionalAppendOnly is the mixin of TA* tables.
type TenantOptionalAppendOnly struct {
	mixin.Schema
	PK PK
}

func (TenantOptionalAppendOnly) TableKind() Kind       { return KindTAStar }
func (m TenantOptionalAppendOnly) Fields() []ent.Field { return fields(KindTAStar, m.PK) }
func (TenantOptionalAppendOnly) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{filter(KindTAStar, nil)}
}
func (TenantOptionalAppendOnly) Hooks() []ent.Hook { return []ent.Hook{mutationHook(KindTAStar, nil)} }

// HybridContent is the mixin of H tables (former PlatformBase): tenant rows, plus platform rows
// (tenant_id NULL) visible to a tenant only when assigned in infra.tenant_resource_assignments
// (ADR-0029). ResourceType is the resource_type of the assignment. The assignment's resource_id is
// matched against the row id, or against ResourceIDColumn when set: version tables of an item
// (question_versions) are assigned through their item (question_id), F020.
type HybridContent struct {
	mixin.Schema
	ResourceType     string
	PK               PK
	ResourceIDColumn string
}

func (HybridContent) TableKind() Kind { return KindH }

// Fields adds owner_scope (platform/tenant) to the T* fields.
func (h HybridContent) Fields() []ent.Field {
	return append(fields(KindH, h.PK), field.String("owner_scope").MaxLen(64).Default("tenant"))
}
func (h HybridContent) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{filter(KindH, h.hybrid())}
}
func (h HybridContent) Hooks() []ent.Hook {
	return []ent.Hook{mutationHook(KindH, h.hybrid())}
}

func (h HybridContent) hybrid() *hybrid {
	col := h.ResourceIDColumn
	if col == "" {
		col = h.PK.column()
	}
	return &hybrid{resourceType: h.ResourceType, idColumn: col}
}

// GlobalBase is the mixin of G tables (no tenant, soft-delete); no scope needed.
type GlobalBase struct {
	mixin.Schema
	PK PK
}

func (GlobalBase) TableKind() Kind                 { return KindG }
func (m GlobalBase) Fields() []ent.Field           { return fields(KindG, m.PK) }
func (GlobalBase) Interceptors() []ent.Interceptor { return []ent.Interceptor{filter(KindG, nil)} }
func (GlobalBase) Hooks() []ent.Hook               { return []ent.Hook{mutationHook(KindG, nil)} }

// AppendOnly is the mixin of GA tables (no tenant, no soft-delete): no filter; delete is forbidden.
type AppendOnly struct {
	mixin.Schema
	PK PK
}

func (AppendOnly) TableKind() Kind       { return KindGA }
func (m AppendOnly) Fields() []ent.Field { return fields(KindGA, m.PK) }
func (AppendOnly) Hooks() []ent.Hook     { return []ent.Hook{mutationHook(KindGA, nil)} }
