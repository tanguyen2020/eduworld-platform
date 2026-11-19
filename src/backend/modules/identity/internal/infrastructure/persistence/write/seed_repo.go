package write

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// PlatformStaffRepo implements write.PlatformStaffRepository (identity.users and external_identities are global
// tables; platform grants have tenant_id NULL and are visible to identity_platform only).
type PlatformStaffRepo struct{}

var _ write.PlatformStaffRepository = PlatformStaffRepo{}

// AccountByUsername implements write.PlatformStaffRepository.
func (PlatformStaffRepo) AccountByUsername(ctx context.Context, username, issuer string) (*write.SeedAccount, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var a write.SeedAccount
	var home uuid.NullUUID
	var sub sql.NullString
	// tenancyguard:ok identity.users and external_identities are global (no tenant_id); explicit deleted_at conditions
	err = tx.QueryRowContext(ctx, `SELECT u.user_id, u.user_type, u.status, u.primary_tenant_id,
			(SELECT e.subject FROM identity.external_identities e WHERE e.user_id = u.user_id AND e.issuer = $2 AND e.status = 'active'
			   AND e.deleted_at IS NULL LIMIT 1)
		FROM identity.users u WHERE lower(u.username) = lower($1) AND u.deleted_at IS NULL`, username, issuer).
		Scan(&a.UserID, &a.UserType, &a.Status, &home, &sub)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if home.Valid {
		h := home.UUID
		a.PrimaryTenantID = &h
	}
	if sub.Valid {
		s := sub.String
		a.Subject = &s
	}
	return &a, nil
}

// SystemAdminExists implements write.PlatformStaffRepository.
func (PlatformStaffRepo) SystemAdminExists(ctx context.Context) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok platform grants have tenant_id NULL (explicit); validity and deleted_at conditions
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM identity.user_roles ur
		JOIN identity.roles r ON r.role_id = ur.role_id AND r.code = 'system_admin' AND r.tenant_id IS NULL AND r.deleted_at IS NULL
		JOIN identity.users u ON u.user_id = ur.user_id AND u.deleted_at IS NULL
		WHERE ur.tenant_id IS NULL AND ur.deleted_at IS NULL AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now()))`).Scan(&ok)
	return ok, err
}

// CreatePlatformStaff implements write.PlatformStaffRepository.
func (PlatformStaffRepo) CreatePlatformStaff(ctx context.Context, s write.NewPlatformStaff) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok identity.users is global (no tenant_id); platform staff have no home tenant (ADR-0081)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, provisioned_at, status, user_type, username, email,
		display_name, locale, primary_tenant_id) VALUES ($1, now(), now(), now(), 'active', 'staff', $2, $3, $4, 'vi-VN', NULL)`,
		s.UserID, s.Username, s.Email, s.DisplayName); err != nil {
		return mapStaffErr(err)
	}
	// tenancyguard:ok identity.external_identities is global (no tenant_id)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
		VALUES ($4, now(), now(), $1, $2, $3)`, s.Issuer, s.Subject, s.UserID, ids.New()); err != nil {
		return mapStaffErr(err)
	}
	return nil
}

// HasGrant implements write.PlatformStaffRepository.
func (PlatformStaffRepo) HasGrant(ctx context.Context, k write.RoleGrantKey) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok explicit tenant condition (IS NOT DISTINCT FROM: NULL = platform grant), validity and deleted_at
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM identity.user_roles WHERE user_id = $1 AND role_id = $2
		AND tenant_id IS NOT DISTINCT FROM $3::uuid AND scope_type = $4 AND scope_id IS NOT DISTINCT FROM $5::uuid
		AND deleted_at IS NULL AND (valid_to IS NULL OR valid_to > now()))`, k.UserID, k.RoleID, k.TenantID, k.ScopeType, k.ScopeID).Scan(&ok)
	return ok, err
}

// SeedLookupRepo implements write.SeedLookupRepository.
type SeedLookupRepo struct{}

var _ write.SeedLookupRepository = SeedLookupRepo{}

func optionalID(row *sql.Row) (*uuid.UUID, error) {
	var id uuid.UUID
	err := row.Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// TenantByCode implements write.SeedLookupRepository.
func (SeedLookupRepo) TenantByCode(ctx context.Context, code string) (*uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform.tenants is the tenant registry (G, tenant_id is its key); explicit deleted_at
	return optionalID(tx.QueryRowContext(ctx, `SELECT tenant_id FROM platform.tenants WHERE code = $1 AND deleted_at IS NULL`, code))
}

// SchoolByCode implements write.SeedLookupRepository.
func (SeedLookupRepo) SchoolByCode(ctx context.Context, tenantID uuid.UUID, code string) (*uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok explicit tenant_id and deleted_at conditions (RLS applies as well)
	return optionalID(tx.QueryRowContext(ctx, `SELECT school_id FROM org.schools WHERE tenant_id = $1 AND code = $2 AND deleted_at IS NULL`,
		tenantID, code))
}

// CsvConnection implements write.SeedLookupRepository.
func (SeedLookupRepo) CsvConnection(ctx context.Context, tenantID uuid.UUID) (*uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok explicit tenant_id and deleted_at conditions (RLS applies as well)
	return optionalID(tx.QueryRowContext(ctx, `SELECT sis_connection_id FROM sis.sis_connections WHERE tenant_id = $1
		AND provider_kind = 'csv_import' AND deleted_at IS NULL ORDER BY created_at LIMIT 1`, tenantID))
}
