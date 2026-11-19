package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
)

// AnonymizeRepo implements write.AnonymizeRepository with explicit SQL in the platform scope
// (identity_platform); platform.deletion_requests and platform.erasure_log are hand-written tables.
type AnonymizeRepo struct{}

var _ write.AnonymizeRepository = AnonymizeRepo{}

// Subject implements write.AnonymizeRepository.
func (AnonymizeRepo) Subject(ctx context.Context, userID uuid.UUID) (*write.AnonymizeSubject, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	s := write.AnonymizeSubject{UserID: userID}
	var tenant uuid.NullUUID
	// tenancyguard:ok platform scope: one user (live or soft-deleted: a cleaned-up student can be anonymized)
	err = tx.QueryRowContext(ctx, `SELECT u.user_type, u.status, COALESCE(s.tenant_id, u.primary_tenant_id)
		FROM identity.users u LEFT JOIN org.students s ON s.student_id = u.user_id WHERE u.user_id = $1`, userID).Scan(&s.UserType, &s.Status, &tenant)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("anonymize subject: %w", err)
	}
	if tenant.Valid {
		t := tenant.UUID
		s.TenantID = &t
	}
	return &s, nil
}

// CreateRequest implements write.AnonymizeRepository.
func (AnonymizeRepo) CreateRequest(ctx context.Context, r write.DataRequest) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform scope: platform.deletion_requests (identity_platform only); tenant_id explicit
	_, err = tx.ExecContext(ctx, `INSERT INTO platform.deletion_requests (request_id, subject_type, subject_id, tenant_id, action, status,
		reason, requested_by, approved_by, requester_kind, reference, created_at, updated_at)
		VALUES ($1, 'user', $2, $3, 'anonymize', $4, $5, $6, $7, $8, $9, $10, $10)`,
		r.ID, r.SubjectID, r.TenantID, r.Status, r.Reason, r.RequestedBy, r.ApprovedBy, r.RequesterKind, r.Reference, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("create data request: %w", err)
	}
	return nil
}

const requestColumns = `request_id, subject_id, tenant_id, status, reason, requester_kind, reference, requested_by, approved_by,
	erasure_id, created_at, executed_at`

func scanRequest(row interface{ Scan(...any) error }) (write.DataRequest, error) {
	var r write.DataRequest
	var kind, ref sql.NullString
	var approved, erasure uuid.NullUUID
	var executed sql.NullTime
	err := row.Scan(&r.ID, &r.SubjectID, &r.TenantID, &r.Status, &r.Reason, &kind, &ref, &r.RequestedBy, &approved, &erasure, &r.CreatedAt, &executed)
	r.RequesterKind = kind.String
	if ref.Valid {
		v := ref.String
		r.Reference = &v
	}
	if approved.Valid {
		v := approved.UUID
		r.ApprovedBy = &v
	}
	if erasure.Valid {
		v := erasure.UUID
		r.ErasureID = &v
	}
	if executed.Valid {
		r.ExecutedAt = &executed.Time
	}
	return r, err
}

// Request implements write.AnonymizeRepository.
func (AnonymizeRepo) Request(ctx context.Context, id uuid.UUID) (*write.DataRequest, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform scope: one data request by id (the caller checks the tenant)
	r, err := scanRequest(tx.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM platform.deletion_requests WHERE request_id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("data request: %w", err)
	}
	return &r, nil
}

// ListRequests implements write.AnonymizeRepository.
func (AnonymizeRepo) ListRequests(ctx context.Context, tenants []uuid.UUID, status string, limit int) ([]write.DataRequest, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var ts any
	if tenants != nil {
		ts = nonNilIDs(tenants)
	}
	// tenancyguard:ok platform scope of a System Admin read; tenant filter explicit (nil = every tenant, audited by the caller)
	rows, err := tx.QueryContext(ctx, `SELECT `+requestColumns+` FROM platform.deletion_requests
		WHERE ($1::uuid[] IS NULL OR tenant_id = ANY($1)) AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC, request_id DESC LIMIT $3`, ts, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list data requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []write.DataRequest{}
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

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
