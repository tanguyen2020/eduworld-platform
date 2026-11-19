package write

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// StaffRepo implements write.StaffRepository (identity.users and external_identities are global tables).
type StaffRepo struct{}

var _ write.StaffRepository = StaffRepo{}

// UsernameTaken checks every account, soft-deleted included (freed only by anonymization).
func (StaffRepo) UsernameTaken(ctx context.Context, username string) (bool, error) {
	return AccountRepo{}.UsernameTaken(ctx, username)
}

// EmailTaken checks the email of every account except one (soft-deleted accounts keep their email until
// anonymized, DATABASE_DESIGN §2.1 [Proposed]).
func (StaffRepo) EmailTaken(ctx context.Context, email string, except *uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var taken bool
	// tenancyguard:ok identity.users is global (no tenant_id); soft-deleted rows still hold their email
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM identity.users WHERE lower(email) = lower($1)
		AND ($2::uuid IS NULL OR user_id <> $2::uuid))`, email, except).Scan(&taken)
	return taken, err
}

func mapStaffErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		if pe.ConstraintName == "users_email_uq" {
			return problem.New(problem.EmailTaken, "").Wrap(err)
		}
		return problem.New(problem.UsernameTaken, "").Wrap(err)
	}
	return err
}

// CreateStaff inserts the staff user and its local identity.
func (StaffRepo) CreateStaff(ctx context.Context, s write.NewStaff) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var locale any
	if s.Locale != "" {
		locale = s.Locale
	}
	// tenancyguard:ok identity.users is global (no tenant_id)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, provisioned_at, status, user_type, username, email,
		display_name, locale, primary_tenant_id) VALUES ($1, now(), now(), now(), 'active', 'staff', $2, $3, $4, coalesce($5::varchar, 'vi-VN'), $6)`,
		s.UserID, s.Username, s.Email, s.DisplayName, locale, s.TenantID); err != nil {
		return mapStaffErr(err)
	}
	// tenancyguard:ok identity.external_identities is global (no tenant_id)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
		VALUES ($4, now(), now(), $1, $2, $3)`, s.Issuer, s.Subject, s.UserID, ids.New()); err != nil {
		return mapStaffErr(err)
	}
	return nil
}

// Staff returns a live staff account of the tenant.
func (StaffRepo) Staff(ctx context.Context, userID, tenantID uuid.UUID, issuer string) (*write.StaffRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var r write.StaffRow
	var email, locale, sub sql.NullString
	var home uuid.NullUUID
	// tenancyguard:ok identity.users is global; the explicit home-tenant condition scopes it to the session tenant
	err = tx.QueryRowContext(ctx, `SELECT u.user_id, u.username, u.email, u.display_name, u.locale, u.status, u.primary_tenant_id, u.version,
			(SELECT e.subject FROM identity.external_identities e WHERE e.user_id = u.user_id AND e.issuer = $3 AND e.status = 'active'
			   AND e.deleted_at IS NULL LIMIT 1)
		FROM identity.users u WHERE u.user_id = $1 AND u.user_type = 'staff' AND u.primary_tenant_id = $2 AND u.deleted_at IS NULL`,
		userID, tenantID, issuer).Scan(&r.UserID, &r.Username, &email, &r.DisplayName, &locale, &r.Status, &home, &r.Version, &sub)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	if email.Valid {
		r.Email = &email.String
	}
	if locale.Valid {
		r.Locale = &locale.String
	}
	if home.Valid {
		id := home.UUID
		r.PrimaryTenantID = &id
	}
	r.KeycloakID = sub.String
	return &r, nil
}

// UpdateStaff stores the editable fields with the optimistic version.
func (StaffRepo) UpdateStaff(ctx context.Context, s *write.StaffRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok identity.users is global (no tenant_id); optimistic version
	err = tx.QueryRowContext(ctx, `UPDATE identity.users SET display_name = $3, locale = $4::varchar, email = $5::varchar, status = $6::varchar, version = version + 1,
		updated_at = now(), deactivated_at = CASE WHEN $6::varchar = 'deactivated' THEN coalesce(deactivated_at, now()) ELSE NULL END
		WHERE user_id = $1 AND version = $2 AND deleted_at IS NULL RETURNING version`, s.UserID, s.Version, s.DisplayName, s.Locale, s.Email, s.Status).
		Scan(&s.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return problem.New(problem.VersionMismatch, "")
	}
	return mapStaffErr(err)
}
