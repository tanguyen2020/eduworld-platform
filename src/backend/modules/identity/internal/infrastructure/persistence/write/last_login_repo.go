package write

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
)

// LastLoginRepo implements write.LastLoginWriter on identity.users (G table).
type LastLoginRepo struct{}

var _ write.LastLoginWriter = LastLoginRepo{}

// RecordGameLogin implements write.LastLoginWriter.
func (LastLoginRepo) RecordGameLogin(ctx context.Context, user uuid.UUID, at time.Time) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok identity.users is a global table (no tenant_id); one user by id, deleted_at filtered
	res, err := tx.ExecContext(ctx, `UPDATE identity.users SET last_login_at = $2
		WHERE user_id = $1 AND deleted_at IS NULL AND (last_login_at IS NULL OR last_login_at < $2)`, user, at)
	if err != nil {
		return false, fmt.Errorf("users: record game login: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
