package write

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
)

// ErrDuplicate: a live row with the same unique key exists (category code, term, allowlist phrase) → 409 CONFLICT.
var ErrDuplicate = errors.New("chat: duplicate catalogue entry")

// ErrStale: the row changed since it was read (version) → 412 VERSION_MISMATCH.
var ErrStale = errors.New("chat: catalogue entry changed")

// CatalogRepository writes the sensitive-term catalogue in the transaction (F116 US2). tenant nil addresses the
// platform rows (tenant_id NULL, platform scope); a tenant addresses its extra terms (tenant scope).
type CatalogRepository interface {
	// Category returns the live platform category, nil when unknown.
	Category(ctx context.Context, id uuid.UUID) (*catalog.StoredCategory, error)
	// CategoryByCode returns the live platform category with code, nil when unknown.
	CategoryByCode(ctx context.Context, code string) (*catalog.StoredCategory, error)
	InsertCategory(ctx context.Context, c catalog.StoredCategory, actor uuid.UUID) error
	// UpdateCategory saves c when the stored version is c.Version - 1 (ErrStale otherwise).
	UpdateCategory(ctx context.Context, c catalog.StoredCategory, actor uuid.UUID) error
	SoftDeleteCategory(ctx context.Context, id, actor uuid.UUID) error
	// LiveTerms counts the live terms (every scope) of a category.
	LiveTerms(ctx context.Context, category uuid.UUID) (int, error)

	// Term returns the live term id visible to tenant (tenant nil: platform terms only; a tenant also sees the platform
	// terms, so a caller can tell them apart); nil when unknown.
	Term(ctx context.Context, tenant *uuid.UUID, id uuid.UUID) (*catalog.StoredTerm, error)
	InsertTerm(ctx context.Context, t catalog.StoredTerm, actor uuid.UUID) error
	// UpdateTerm saves t as a draft when the stored version is t.Version - 1 (ErrStale otherwise).
	UpdateTerm(ctx context.Context, t catalog.StoredTerm, actor uuid.UUID) error
	SoftDeleteTerm(ctx context.Context, tenant *uuid.UUID, id, actor uuid.UUID) error
	// Patterns counts the live pattern terms of the scope (sensitive_terms_engine_limits: ≤ 200), except one id.
	Patterns(ctx context.Context, tenant *uuid.UUID, except uuid.UUID) (int, error)

	// Allow returns the live platform allowlist entry, nil when unknown.
	Allow(ctx context.Context, id uuid.UUID) (*catalog.StoredAllow, error)
	InsertAllow(ctx context.Context, a catalog.StoredAllow, actor uuid.UUID) error
	SoftDeleteAllow(ctx context.Context, id, actor uuid.UUID) error

	// LiveSnapshot builds the catalogue of the scope from the live rows (drafts included): the platform part has the
	// live categories, the enabled live terms of live categories and the allowlist; a tenant part has the enabled live
	// extra terms of that tenant only. ids are the terms included.
	LiveSnapshot(ctx context.Context, tenant *uuid.UUID) (s catalog.Snapshot, ids []uuid.UUID, err error)
	// LockScope serialises the publishes of a scope until the end of the transaction.
	LockScope(ctx context.Context, tenant *uuid.UUID) error
	// NextVersion returns the next catalogue version of the scope.
	NextVersion(ctx context.Context, tenant *uuid.UUID) (int64, error)
	// PublishedSnapshot returns the platform snapshot in force (newest version).
	PublishedSnapshot(ctx context.Context) (catalog.Snapshot, error)
	InsertVersion(ctx context.Context, v catalog.StoredVersion) error
	// MarkPublished sets status published and published_in_version of the terms ids of the scope.
	MarkPublished(ctx context.Context, tenant *uuid.UUID, ids []uuid.UUID, version int64) error

	// TenantSettings returns the category settings of tenant (category id → enabled).
	TenantSettings(ctx context.Context, tenant uuid.UUID) (map[uuid.UUID]bool, error)
	// SetTenantCategory writes the setting of a category in tenant (update of the live row or insert).
	SetTenantCategory(ctx context.Context, tenant, category uuid.UUID, enabled bool, reason string, actor uuid.UUID) error
	// TenantTerms counts the live extra terms of tenant in a category.
	TenantTerms(ctx context.Context, tenant, category uuid.UUID) (int, error)
}
