package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entreset "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/portalpasswordresettoken"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// PortalResetTokenRepo implements write.PortalResetTokenRepository on identity.portal_password_reset_tokens
// (G table, data-model F012): only the SHA-256 of the token is stored, superseding is a soft-delete through
// the ent hook (delete_reason superseded), use is one atomic UPDATE.
type PortalResetTokenRepo struct{}

var _ write.PortalResetTokenRepository = PortalResetTokenRepo{}

// ReasonSuperseded is the delete_reason of a token replaced by a newer request.
const ReasonSuperseded = "superseded"

// Issue implements write.PortalResetTokenRepository.
func (PortalResetTokenRepo) Issue(ctx context.Context, t write.PortalResetToken) (int, error) {
	c := client(ctx)
	n, err := c.PortalPasswordResetToken.Delete().
		Where(entreset.UserID(t.UserID), entreset.UsedAtIsNil(), entreset.DeletedAtIsNil()).
		Exec(entx.WithDeleteReason(ctx, ReasonSuperseded))
	if err != nil {
		return 0, fmt.Errorf("portal reset token: supersede: %w", err)
	}
	create := c.PortalPasswordResetToken.Create().SetID(t.ID).SetUserID(t.UserID).SetTokenHash(t.TokenHash).
		SetExpiresAt(t.ExpiresAt.UTC())
	if t.Locale != "" {
		create.SetLocale(t.Locale)
	}
	if t.DeviceHash != "" {
		create.SetRequestedDeviceHash(t.DeviceHash)
	}
	if _, err := create.Save(ctx); err != nil {
		if ent.IsConstraintError(err) {
			return 0, fmt.Errorf("portal reset token: concurrent request: %w", err)
		}
		return 0, fmt.Errorf("portal reset token: insert: %w", err)
	}
	return n, nil
}

// Use implements write.PortalResetTokenRepository: one UPDATE … RETURNING, so two concurrent uses of one
// token cannot both succeed (the second waits for the row lock, then sees used_at).
func (PortalResetTokenRepo) Use(ctx context.Context, tokenHash string, now time.Time) (uuid.UUID, bool, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return uuid.Nil, false, entx.ErrNoTransaction
	}
	var userID uuid.UUID
	// tenancyguard:ok identity.portal_password_reset_tokens is global (G, no tenant_id); deleted_at is filtered
	err := tx.QueryRowContext(ctx, `UPDATE identity.portal_password_reset_tokens
		SET used_at = $2, updated_at = $2, version = version + 1
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2 AND deleted_at IS NULL
		RETURNING user_id`, tokenHash, now.UTC()).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("portal reset token: use: %w", err)
	}
	return userID, true, nil
}
