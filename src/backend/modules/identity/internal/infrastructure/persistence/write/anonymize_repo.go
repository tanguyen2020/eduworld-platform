package write

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
)

// AnonymizeRepo implements write.AnonymizeRepository with explicit SQL in the platform scope
// (identity_platform); platform.deletion_requests and platform.erasure_log are hand-written tables.
type AnonymizeRepo struct{}

var _ write.AnonymizeRepository = AnonymizeRepo{}

// FinishRequest implements write.AnonymizeRepository.
func (AnonymizeRepo) FinishRequest(ctx context.Context, id, erasureID uuid.UUID, at time.Time) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform scope: the request of a finished anonymize saga
	_, err = tx.ExecContext(ctx, `UPDATE platform.deletion_requests SET status = 'executed', erasure_id = $2, executed_at = $3, updated_at = $3,
		version = version + 1 WHERE request_id = $1 AND status = 'approved'`, id, erasureID, at)
	if err != nil {
		return fmt.Errorf("finish data request: %w", err)
	}
	return nil
}

// AnonymizeUser implements write.AnonymizeRepository.
func (AnonymizeRepo) AnonymizeUser(ctx context.Context, userID uuid.UUID, at time.Time) (bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok identity.users is global (G): overwrite the PII of one user, never delete the row
	res, err := tx.ExecContext(ctx, `UPDATE identity.users SET username = 'anon.' || user_id::text, email = NULL, display_name = NULL,
		status = 'anonymized', anonymized_at = $2, updated_at = $2, version = version + 1
		WHERE user_id = $1 AND status <> 'anonymized'`, userID, at)
	if err != nil {
		return false, fmt.Errorf("anonymize user: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	// tenancyguard:ok platform scope: the student row of the user (live or soft-deleted), PII overwritten
	if _, err := tx.ExecContext(ctx, `UPDATE org.students SET full_name = 'anon', student_code = NULL, date_of_birth_enc = NULL,
		national_id_hash = NULL, status = 'inactive', updated_at = $2, version = version + 1 WHERE student_id = $1`, userID, at); err != nil {
		return false, fmt.Errorf("anonymize student: %w", err)
	}
	return true, nil
}

// AppendErasure implements write.AnonymizeRepository.
func (AnonymizeRepo) AppendErasure(ctx context.Context, e write.ErasureEntry) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	scope, err := json.Marshal(e.Scope)
	if err != nil {
		return err
	}
	// tenancyguard:ok append-only erasure log (platform scope, identity_platform only)
	_, err = tx.ExecContext(ctx, `INSERT INTO platform.erasure_log (erasure_id, user_id, kind, requested_by, legal_basis, scope)
		VALUES ($1, $2, 'anonymize', $3, $4, $5)`, e.ID, e.UserID, e.RequestedBy, e.LegalBasis, scope)
	if err != nil {
		return fmt.Errorf("erasure log: %w", err)
	}
	return nil
}

// RetentionDue implements write.AnonymizeRepository.
func (AnonymizeRepo) RetentionDue(ctx context.Context, before time.Time, limit int) ([]write.AnonymizeSubject, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// Deactivated students whose last change to deactivated (status history, else updated_at) is before the limit.
	// tenancyguard:ok platform scope of the job retention-anonymizer (every tenant, deleted rows included on purpose)
	rows, err := tx.QueryContext(ctx, `SELECT u.user_id, s.tenant_id FROM identity.users u JOIN org.students s ON s.student_id = u.user_id
		WHERE u.user_type = 'student' AND u.status = 'deactivated'
		  AND COALESCE((SELECT max(h.changed_at) FROM identity.user_status_history h WHERE h.user_id = u.user_id AND h.to_status = 'deactivated'),
		               u.updated_at) < $1
		ORDER BY u.user_id LIMIT $2`, before, limit)
	if err != nil {
		return nil, fmt.Errorf("retention due: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AnonymizeSubject
	for rows.Next() {
		var s write.AnonymizeSubject
		var t uuid.UUID
		if err := rows.Scan(&s.UserID, &t); err != nil {
			return nil, err
		}
		s.TenantID, s.UserType, s.Status = &t, "student", "deactivated"
		out = append(out, s)
	}
	return out, rows.Err()
}
