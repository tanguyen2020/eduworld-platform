package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// EmailDeliveryRepo implements write.EmailDeliveryRepository on identity.email_deliveries (T*): insert + status updates,
// never DELETE (R21); deleted_at in every statement. The job email-sender runs on identity_platform (platform scope).
type EmailDeliveryRepo struct{}

var _ write.EmailDeliveryRepository = EmailDeliveryRepo{}

// Enqueue implements write.EmailDeliveryRepository.
func (EmailDeliveryRepo) Enqueue(ctx context.Context, d write.EmailDelivery) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	if d.ID == uuid.Nil {
		d.ID = ids.New()
	}
	params, err := json.Marshal(d.Params)
	if err != nil {
		return false, err
	}
	if d.Locale == "" {
		d.Locale = "vi"
	}
	// tenancyguard:ok one delivery with its tenant (NULL for platform staff; RLS insert check)
	res, err := tx.ExecContext(ctx, `INSERT INTO identity.email_deliveries (delivery_id, tenant_id, created_at, updated_at, source_event_id,
			template_code, recipient_user_id, params, locale, status, attempts, next_attempt_at)
		VALUES ($1, $2, $3, $3, $4, $5, $6, $7::jsonb, $8, 'pending', 0, $3) ON CONFLICT DO NOTHING`,
		d.ID, d.TenantID, d.CreatedAt, d.SourceEventID, d.Template, d.RecipientID, string(params), d.Locale)
	if err != nil {
		return false, fmt.Errorf("enqueue email: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// Claim implements write.EmailDeliveryRepository.
func (EmailDeliveryRepo) Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]write.EmailDelivery, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok work list of the job email-sender (platform scope, every tenant); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `UPDATE identity.email_deliveries d SET next_attempt_at = $2, updated_at = $1
		WHERE d.delivery_id IN (SELECT delivery_id FROM identity.email_deliveries WHERE status = 'pending' AND deleted_at IS NULL
			AND (next_attempt_at IS NULL OR next_attempt_at <= $1) ORDER BY next_attempt_at NULLS FIRST, delivery_id LIMIT $3
			FOR UPDATE SKIP LOCKED)
		RETURNING d.delivery_id, d.tenant_id, d.source_event_id, d.template_code, d.recipient_user_id, d.params, d.locale, d.status,
			d.attempts, d.created_at`, now, now.Add(lease), limit)
	if err != nil {
		return nil, fmt.Errorf("claim emails: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.EmailDelivery
	for rows.Next() {
		var d write.EmailDelivery
		var tenant uuid.NullUUID
		var params []byte
		var attempts int16
		if err := rows.Scan(&d.ID, &tenant, &d.SourceEventID, &d.Template, &d.RecipientID, &params, &d.Locale, &d.Status, &attempts,
			&d.CreatedAt); err != nil {
			return nil, fmt.Errorf("claim emails: %w", err)
		}
		d.TenantID, d.Attempts = nullID(tenant), int(attempts)
		if err := json.Unmarshal(params, &d.Params); err != nil {
			return nil, fmt.Errorf("claim emails: params: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Finish implements write.EmailDeliveryRepository.
func (EmailDeliveryRepo) Finish(ctx context.Context, id uuid.UUID, status string, attempts int, code *string, at time.Time) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var sent any
	if status == "sent" {
		sent = at
	}
	// tenancyguard:ok one delivery by id (platform scope of the job); deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE identity.email_deliveries SET status = $2, attempts = $3, last_error_code = $4, sent_at = $5,
			next_attempt_at = NULL, updated_at = $6, version = version + 1
		WHERE delivery_id = $1 AND status = 'pending' AND deleted_at IS NULL`, id, status, attempts, code, sent, at)
	if err != nil {
		return fmt.Errorf("finish email: %w", err)
	}
	return nil
}

// Retry implements write.EmailDeliveryRepository.
func (EmailDeliveryRepo) Retry(ctx context.Context, id uuid.UUID, attempts int, next time.Time, code string) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok one delivery by id (platform scope of the job); deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE identity.email_deliveries SET attempts = $2, next_attempt_at = $3, last_error_code = $4,
			updated_at = now(), version = version + 1 WHERE delivery_id = $1 AND status = 'pending' AND deleted_at IS NULL`, id, attempts, next, code)
	if err != nil {
		return fmt.Errorf("retry email: %w", err)
	}
	return nil
}

// StaffDirectoryRepo implements write.StaffDirectory with explicit SQL: valid UserRoles (deleted_at IS NULL,
// valid_from ≤ now < valid_to) of live, active staff accounts (identity.users is global).
type StaffDirectoryRepo struct{}

var _ write.StaffDirectory = StaffDirectoryRepo{}

const validStaff = `u.deleted_at IS NULL AND u.status = 'active' AND u.user_type = 'staff'
	AND ur.deleted_at IS NULL AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now())`

func userIDs(ctx context.Context, q string, args ...any) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("staff directory: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("staff directory: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// TenantRole implements write.StaffDirectory.
func (StaffDirectoryRepo) TenantRole(ctx context.Context, tenant uuid.UUID, role string, school *uuid.UUID) ([]uuid.UUID, error) {
	cond, args := "", []any{tenant, role}
	if school != nil {
		cond, args = " AND ur.school_id = $3", append(args, *school)
	}
	// tenancyguard:ok explicit tenant condition on user_roles (RLS of the scope tenant); deleted_at and validity conditions
	return userIDs(ctx, `SELECT DISTINCT u.user_id FROM identity.user_roles ur
		JOIN identity.roles r ON r.role_id = ur.role_id AND r.deleted_at IS NULL AND r.code = $2
		JOIN identity.users u ON u.user_id = ur.user_id
		WHERE ur.tenant_id = $1`+cond+` AND `+validStaff+` ORDER BY u.user_id`, args...)
}

// PlatformPermission implements write.StaffDirectory.
func (StaffDirectoryRepo) PlatformPermission(ctx context.Context, perm, exceptRole string) ([]uuid.UUID, error) {
	// tenancyguard:ok platform roles only (tenant_id IS NULL, platform scope); deleted_at and validity conditions
	return userIDs(ctx, `SELECT DISTINCT u.user_id FROM identity.user_roles ur
		JOIN identity.roles r ON r.role_id = ur.role_id AND r.deleted_at IS NULL AND r.tenant_id IS NULL AND r.code <> $2
		JOIN identity.role_permissions rp ON rp.role_id = r.role_id AND rp.deleted_at IS NULL AND rp.permission_code = $1
		JOIN identity.users u ON u.user_id = ur.user_id
		WHERE ur.tenant_id IS NULL AND `+validStaff+` ORDER BY u.user_id`, perm, exceptRole)
}

// PlatformRole implements write.StaffDirectory.
func (StaffDirectoryRepo) PlatformRole(ctx context.Context, role string) ([]uuid.UUID, error) {
	// tenancyguard:ok platform roles only (tenant_id IS NULL, platform scope); deleted_at and validity conditions
	return userIDs(ctx, `SELECT DISTINCT u.user_id FROM identity.user_roles ur
		JOIN identity.roles r ON r.role_id = ur.role_id AND r.deleted_at IS NULL AND r.tenant_id IS NULL AND r.code = $1
		JOIN identity.users u ON u.user_id = ur.user_id
		WHERE ur.tenant_id IS NULL AND `+validStaff+` ORDER BY u.user_id`, role)
}

// Contact implements write.StaffDirectory.
func (StaffDirectoryRepo) Contact(ctx context.Context, user uuid.UUID) (*write.StaffContact, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var c write.StaffContact
	var email sql.NullString
	// tenancyguard:ok identity.users is global (G): one live, active staff account by id
	err = tx.QueryRowContext(ctx, `SELECT user_id, email, locale FROM identity.users WHERE user_id = $1 AND deleted_at IS NULL
		AND status = 'active' AND user_type = 'staff'`, user).Scan(&c.UserID, &email, &c.Locale)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (!email.Valid || strings.TrimSpace(email.String) == "")) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("staff contact: %w", err)
	}
	c.Email = email.String
	return &c, nil
}

// SchoolName implements write.StaffDirectory.
func (StaffDirectoryRepo) SchoolName(ctx context.Context, tenant, school uuid.UUID) (string, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return "", err
	}
	var name string
	// tenancyguard:ok explicit tenant condition; deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT name FROM org.schools WHERE tenant_id = $1 AND school_id = $2 AND deleted_at IS NULL`, tenant, school).
		Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("school name: %w", err)
	}
	return name, nil
}
