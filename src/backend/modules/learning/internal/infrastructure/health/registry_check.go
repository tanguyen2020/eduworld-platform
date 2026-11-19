// Package health holds the readiness checks of module learning beyond the database and Atlas revision (F030).
package health

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
)

// Querier is the platform pool (learning_platform, BYPASSRLS) the probe reads published activity types with.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

// Rows is the subset of pgx.Rows the probe reads.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// PublishedTypes reads the activity types that have a published release in any tenant.
type PublishedTypes func(ctx context.Context) ([]string, error)

// SQLPublishedTypes reads the published activity types with q.
func SQLPublishedTypes(q Querier) PublishedTypes {
	return func(ctx context.Context) ([]string, error) {
		// tenancyguard:ok readiness probe of the platform role: distinct activity types of live released items of every tenant, no row data
		rows, err := q.Query(ctx, `SELECT DISTINCT v.activity_type FROM content.activities a
			JOIN content.activity_versions v ON v.activity_version_id = a.current_version_id
			WHERE a.deleted_at IS NULL AND v.deleted_at IS NULL AND v.activity_type IS NOT NULL`)
		if err != nil {
			return nil, fmt.Errorf("published activity types: %w", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				return nil, err
			}
			out = append(out, t)
		}
		return out, rows.Err()
	}
}

// ErrPluginMissing is the readiness failure of F030-AC-23: published content of a type without plugin.
var ErrPluginMissing = errors.New("activity registry: published content has no plugin")

// RegistryCheck fails while a published activity type has no plugin in reg (health.CheckFunc, kind Readiness).
func RegistryCheck(reg *activity.Registry, published PublishedTypes) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		types, err := published(ctx)
		if err != nil {
			return err
		}
		if missing := reg.Missing(types); len(missing) > 0 {
			return fmt.Errorf("%w: %s", ErrPluginMissing, strings.Join(missing, ","))
		}
		return nil
	}
}
