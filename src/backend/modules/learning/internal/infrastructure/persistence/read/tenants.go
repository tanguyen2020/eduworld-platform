package read

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// TenantDirectory implements port.TenantDirectory on ref.tenants (projection of module identity, no RLS: the
// tenant registry itself). It runs on the transaction of ctx (read session or write transaction).
type TenantDirectory struct{}

var _ port.TenantDirectory = TenantDirectory{}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func txOf(ctx context.Context) (querier, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return tx, nil
}

// ActiveTenants implements port.TenantDirectory.
func (TenantDirectory) ActiveTenants(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if len(ids) == 0 {
		// tenancyguard:ok ref.tenants is the tenant registry (key = tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2)
		rows, err = q.QueryContext(ctx, `SELECT tenant_id FROM ref.tenants WHERE status = 'active' ORDER BY tenant_id`)
	} else {
		// tenancyguard:ok ref.tenants is the tenant registry (key = tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2)
		rows, err = q.QueryContext(ctx, `SELECT tenant_id FROM ref.tenants WHERE status = 'active' AND tenant_id = ANY($1::uuid[])`, uuidArray(ids))
	}
	if err != nil {
		return nil, fmt.Errorf("ref.tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	active := map[uuid.UUID]bool{}
	var all []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		active[id] = true
		all = append(all, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return all, nil
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if active[id] {
			out = append(out, id)
		}
	}
	return out, nil
}
