package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// AccountLockRepo implements write.AccountLockRepository with explicit SQL on identity.account_locks (T*: RLS admits the
// rows of the scope tenant to identity_app, every row to identity_platform; deleted_at in every statement, R27).
type AccountLockRepo struct{}

var _ write.AccountLockRepository = AccountLockRepo{}

// ActiveSources implements write.AccountLockRepository.
func (AccountLockRepo) ActiveSources(ctx context.Context, user uuid.UUID) ([]string, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok locks of one user under the RLS of the scope; released_at and deleted_at conditions
	rows, err := tx.QueryContext(ctx, `SELECT source FROM identity.account_locks WHERE user_id = $1 AND released_at IS NULL
		AND deleted_at IS NULL ORDER BY created_at`, user)
	if err != nil {
		return nil, fmt.Errorf("account locks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("account locks: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Add implements write.AccountLockRepository (the partial unique indexes keep one lock per sanction and one active lock
// per source without reference).
func (AccountLockRepo) Add(ctx context.Context, l write.AccountLock) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	if l.ID == uuid.Nil {
		l.ID = ids.New()
	}
	// tenancyguard:ok insert of one lock row with its tenant (RLS insert check)
	res, err := tx.ExecContext(ctx, `INSERT INTO identity.account_locks (lock_id, tenant_id, created_at, created_by, updated_at, updated_by,
			user_id, source, source_ref_id, case_id, reason, released_at, source_event_id)
		VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT DO NOTHING`,
		l.ID, l.TenantID, l.CreatedAt, l.CreatedBy, l.UserID, l.Source, l.SourceRefID, l.CaseID, l.Reason, l.ReleasedAt, l.SourceEventID)
	if err != nil {
		return false, fmt.Errorf("add account lock: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// ReleaseSource implements write.AccountLockRepository.
func (AccountLockRepo) ReleaseSource(ctx context.Context, user uuid.UUID, source string, by *uuid.UUID, reason string, event *uuid.UUID,
	at time.Time) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	var why *string
	if reason != "" {
		why = &reason
	}
	// tenancyguard:ok active locks of one user and one source under the RLS of the scope; deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE identity.account_locks SET released_at = $3, released_by = $4, release_reason = $5,
			source_event_id = coalesce($6, source_event_id), updated_at = $3, updated_by = $4, version = version + 1
		WHERE user_id = $1 AND source = $2 AND released_at IS NULL AND deleted_at IS NULL`, user, source, at, by, why, event)
	if err != nil {
		return 0, fmt.Errorf("release account lock: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// BySanction implements write.AccountLockRepository.
func (AccountLockRepo) BySanction(ctx context.Context, sanction uuid.UUID) (*write.AccountLock, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var l write.AccountLock
	var tenant, ref, caseID, by, event uuid.NullUUID
	var reason sql.NullString
	var released sql.NullTime
	// tenancyguard:ok the lock of one sanction under the RLS of the scope; deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT lock_id, tenant_id, user_id, source, source_ref_id, case_id, reason, created_by, created_at,
			released_at, source_event_id
		FROM identity.account_locks WHERE source = 'moderation_sanction' AND source_ref_id = $1 AND deleted_at IS NULL`, sanction).
		Scan(&l.ID, &tenant, &l.UserID, &l.Source, &ref, &caseID, &reason, &by, &l.CreatedAt, &released, &event)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("account lock of sanction: %w", err)
	}
	l.TenantID, l.SourceRefID, l.CaseID, l.CreatedBy, l.SourceEventID = nullID(tenant), nullID(ref), nullID(caseID), nullID(by), nullID(event)
	if reason.Valid {
		l.Reason = &reason.String
	}
	if released.Valid {
		t := released.Time
		l.ReleasedAt = &t
	}
	return &l, nil
}

// ReleaseSanction implements write.AccountLockRepository.
func (AccountLockRepo) ReleaseSanction(ctx context.Context, sanction uuid.UUID, event *uuid.UUID, at time.Time) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok the lock of one sanction under the RLS of the scope; deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE identity.account_locks SET released_at = $2, release_reason = 'sanction_lifted',
			source_event_id = coalesce($3, source_event_id), updated_at = $2, version = version + 1
		WHERE source = 'moderation_sanction' AND source_ref_id = $1 AND released_at IS NULL AND deleted_at IS NULL`, sanction, at, event)
	if err != nil {
		return false, fmt.Errorf("release sanction lock: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
