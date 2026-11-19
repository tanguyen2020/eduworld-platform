package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// PortalAccountRepo implements write.PortalAccountRepository with explicit SQL in the transaction of the
// platform scope "portal-auth" (identity_platform): users and external identities are global (G); the role
// check reads user roles of every tenant and platform roles, so each statement states its own deleted_at,
// validity and tenant-status conditions (AGENTS R27).
type PortalAccountRepo struct{}

var _ write.PortalAccountRepository = PortalAccountRepo{}

// portalAccountSelect returns the account columns and whether the user holds a valid UserRole in an active
// tenant or a platform role (tenant_id NULL, scope platform).
const portalAccountSelect = `SELECT u.user_id, u.user_type, u.status, coalesce(u.email, ''), u.locale,
	EXISTS (SELECT 1 FROM identity.user_roles ur
		JOIN identity.roles r ON r.role_id = ur.role_id AND r.deleted_at IS NULL
		LEFT JOIN platform.tenants t ON t.tenant_id = ur.tenant_id AND t.deleted_at IS NULL
		WHERE ur.user_id = u.user_id AND ur.deleted_at IS NULL
			AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now())
			AND (ur.tenant_id IS NULL OR t.status = 'active'))
	FROM identity.users u`

func portalTx(ctx context.Context) (*sql.Tx, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return tx, nil
}

func scanPortalAccounts(rows *sql.Rows) ([]write.PortalAccount, error) {
	defer func() { _ = rows.Close() }()
	var out []write.PortalAccount
	for rows.Next() {
		var a write.PortalAccount
		if err := rows.Scan(&a.UserID, &a.UserType, &a.Status, &a.Email, &a.Locale, &a.HasPortalRole); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func onePortalAccount(ctx context.Context, q string, args ...any) (*write.PortalAccount, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("portal account: %w", err)
	}
	list, err := scanPortalAccounts(rows)
	if err != nil {
		return nil, fmt.Errorf("portal account: %w", err)
	}
	if len(list) != 1 {
		return nil, nil // none, or ambiguous
	}
	return &list[0], nil
}

// BySubject implements write.PortalAccountRepository.
func (PortalAccountRepo) BySubject(ctx context.Context, issuer, subject string) (*write.PortalAccount, error) {
	// tenancyguard:ok users/external_identities are global; user_roles is read across tenants with explicit conditions
	return onePortalAccount(ctx, portalAccountSelect+`
		JOIN identity.external_identities e ON e.user_id = u.user_id AND e.status = 'active' AND e.deleted_at IS NULL
		WHERE e.issuer = $1 AND e.subject = $2 AND u.deleted_at IS NULL`, issuer, subject)
}

// issuerIn returns "IN ($n, …)" for issuers starting at placeholder first.
func issuerIn(first int, issuers []string) (string, []any) {
	ph := make([]string, len(issuers))
	args := make([]any, len(issuers))
	for i, s := range issuers {
		ph[i] = fmt.Sprintf("$%d", first+i)
		args[i] = s
	}
	return "IN (" + strings.Join(ph, ", ") + ")", args
}

// ByKeycloakID implements write.PortalAccountRepository.
func (PortalAccountRepo) ByKeycloakID(ctx context.Context, issuers []string, subject string) (*write.PortalAccount, error) {
	if len(issuers) == 0 {
		return nil, errors.New("portal account: no trusted issuer configured")
	}
	in, args := issuerIn(2, issuers)
	// tenancyguard:ok users/external_identities are global; user_roles is read across tenants with explicit conditions
	return onePortalAccount(ctx, portalAccountSelect+`
		JOIN identity.external_identities e ON e.user_id = u.user_id AND e.status = 'active' AND e.deleted_at IS NULL
		WHERE e.subject = $1 AND e.issuer `+in+` AND u.deleted_at IS NULL`, append([]any{subject}, args...)...)
}

// ByID implements write.PortalAccountRepository.
func (PortalAccountRepo) ByID(ctx context.Context, userID uuid.UUID) (*write.PortalAccount, error) {
	// tenancyguard:ok users is global; user_roles is read across tenants with explicit conditions
	return onePortalAccount(ctx, portalAccountSelect+` WHERE u.user_id = $1 AND u.deleted_at IS NULL`, userID)
}

// ActiveStaffByLogin implements write.PortalAccountRepository (unique lower(username) and lower(email)).
func (PortalAccountRepo) ActiveStaffByLogin(ctx context.Context, login string) (*write.PortalAccount, error) {
	// tenancyguard:ok users is global; user_roles is read across tenants with explicit conditions
	return onePortalAccount(ctx, portalAccountSelect+`
		WHERE (lower(u.username) = $1 OR lower(u.email) = $1) AND u.user_type = 'staff' AND u.status = 'active'
			AND u.deleted_at IS NULL LIMIT 2`, strings.ToLower(login))
}

// KeycloakSubject implements write.PortalAccountRepository.
func (PortalAccountRepo) KeycloakSubject(ctx context.Context, userID uuid.UUID, issuers []string) (string, error) {
	if len(issuers) == 0 {
		return "", errors.New("portal account: no trusted issuer configured")
	}
	tx, err := portalTx(ctx)
	if err != nil {
		return "", err
	}
	in, args := issuerIn(2, issuers)
	var sub string
	// tenancyguard:ok external_identities is global (G, no tenant_id)
	err = tx.QueryRowContext(ctx, `SELECT subject FROM identity.external_identities
		WHERE user_id = $1 AND issuer `+in+` AND status = 'active' AND deleted_at IS NULL
		ORDER BY created_at LIMIT 1`, append([]any{userID}, args...)...).Scan(&sub)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return sub, err
}
