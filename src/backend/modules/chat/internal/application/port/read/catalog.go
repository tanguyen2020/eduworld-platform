package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
)

// CategoryCursor is the keyset position after the last category of a page.
type CategoryCursor struct {
	Code string
	ID   uuid.UUID
}

// CategoryRow is a live platform category with the number of its live platform terms.
type CategoryRow struct {
	catalog.StoredCategory
	TermCount int
}

// TenantCategoryRow is a platform category as a tenant sees it: its setting in the tenant (nil: not set) and the number
// of extra terms of the tenant. Never a term (F116-AC-18).
type TenantCategoryRow struct {
	catalog.StoredCategory
	InTenant        *bool
	TenantTermCount int
}

// TermQuery selects terms: Tenants nil = platform terms, otherwise the extra terms of those tenants.
type TermQuery struct {
	Tenants    []uuid.UUID
	CategoryID *uuid.UUID
	Text       string // case-insensitive substring of the term (staff with chat.sensitive_terms.read only)
	Locale     string // vi | en; empty = any (F122 T063, SQL)
	MatchType  string // word | phrase | contains | pattern; empty = any
	Status     string // draft | published; empty = any
	After      *Cursor
	Limit      int
}

// Cursor is the keyset position after the last row (created_at, id).
type Cursor struct {
	At time.Time
	ID uuid.UUID
}

// CatalogAdminReader is the read side of the catalogue administration (read session; explicit tenant and deleted_at
// conditions).
type CatalogAdminReader interface {
	// Categories returns one keyset page of the live platform categories after after, (code, category_id) order (owner rule
	// 2026-11-23: paged in SQL).
	Categories(ctx context.Context, after *CategoryCursor, limit int) ([]CategoryRow, error)
	Terms(ctx context.Context, q TermQuery) ([]catalog.StoredTerm, error)
	Allowlist(ctx context.Context, after *Cursor, limit int) ([]catalog.StoredAllow, error)
	// Versions lists the versions of the platform (tenants nil) or of tenants, newest first, before version (0: none).
	Versions(ctx context.Context, tenants []uuid.UUID, before int64, limit int) ([]catalog.StoredVersion, error)
	TenantCategories(ctx context.Context, tenant uuid.UUID, after *CategoryCursor, limit int) ([]TenantCategoryRow, error)
	// LiveSnapshot is CatalogRepository.LiveSnapshot in the read session (testSensitiveTerms with drafts).
	LiveSnapshot(ctx context.Context, tenant *uuid.UUID) (catalog.Snapshot, []uuid.UUID, error)
}
