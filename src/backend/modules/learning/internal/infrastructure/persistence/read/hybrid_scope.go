// Package read implements the read ports of module learning: read models with explicit SQL or ent, run inside
// entx.ReadSession (or the write transaction when a command needs to read); never the outbox.
package read

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

var identRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// HybridCondition returns the WHERE condition of an explicit read of hybrid content (MULTITENANCY §4.2) for
// the table aliased alias whose assignments match idColumn, with the scope of ctx, and its arguments numbered
// from $next:
//   - TenantScope / CrossTenantScope: rows of the scope's tenants, or platform rows (tenant_id IS NULL) with an
//     active assignment to one of them;
//   - PlatformScope: platform rows only;
//   - tenancy.Bypass(trash): no tenant condition and the deleted rows too (Thùng rác).
//
// Every other case also requires deleted_at IS NULL (AGENTS R21, R27). Without a scope it returns
// entx.ErrMissingTenantScope (F020-AC-10).
func HybridCondition(ctx context.Context, alias string, rt contentmodel.ResourceType, idColumn string, next int) (string, []any, error) {
	if !identRe.MatchString(alias) || !identRe.MatchString(idColumn) {
		return "", nil, fmt.Errorf("read: invalid identifier %q/%q", alias, idColumn)
	}
	if _, err := contentmodel.ParseResourceType(string(rt)); err != nil {
		return "", nil, err
	}
	if r, ok := tenancy.Bypassed(ctx); ok {
		if r == tenancy.ReasonTrash {
			return "TRUE", nil, nil
		}
		return alias + ".deleted_at IS NULL", nil, nil
	}
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return "", nil, entx.ErrMissingTenantScope
	}
	if s.PlatformOnly() {
		return alias + ".deleted_at IS NULL AND " + alias + ".tenant_id IS NULL", nil, nil
	}
	ids := s.TenantIDs()
	cond := fmt.Sprintf(`%[1]s.deleted_at IS NULL AND (%[1]s.tenant_id = ANY($%[2]d::uuid[]) OR (%[1]s.tenant_id IS NULL AND EXISTS (
	SELECT 1 FROM infra.tenant_resource_assignments a
	WHERE a.resource_type = $%[3]d AND a.resource_id = %[1]s.%[4]s AND a.tenant_id = ANY($%[2]d::uuid[]) AND a.deleted_at IS NULL)))`,
		alias, next, next+1, idColumn)
	return cond, []any{uuidArray(ids), string(rt)}, nil
}

// uuidArray formats ids as a PostgreSQL array literal ({…}) accepted by $n::uuid[].
func uuidArray(ids []uuid.UUID) string {
	b := []byte{'{'}
	for i, id := range ids {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, id.String()...)
	}
	return string(append(b, '}'))
}
