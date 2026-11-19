package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// LoginMembershipReader implements write.LoginMembershipReader (F013 T042, ADR-0089 §3): before any
// TenantScope exists, the login path calls identity.memberships_for_user (SECURITY DEFINER, owner
// identity_membership_reader) as identity_app inside tenancy.Bypass(ctx, auth_login), which records
// cross_tenant.accessed and eduworld_tenant_filter_bypass_total{reason="auth_login"}. platform.tenants is
// global. Only the login use cases (/auth/game/*, /auth/session, login-credentials:verify) use this type.
type LoginMembershipReader struct {
	Session *entx.ReadSession
}

var _ write.LoginMembershipReader = LoginMembershipReader{}

// LoginMemberships implements write.LoginMembershipReader.
func (r LoginMembershipReader) LoginMemberships(ctx context.Context, userID uuid.UUID) (out []gamelogin.Membership, err error) {
	ctx, err = tenancy.Bypass(ctx, tenancy.ReasonAuthLogin)
	if err != nil {
		return nil, err
	}
	err = r.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		// tenancyguard:ok the SECURITY DEFINER function returns only the rows of p_user_id; platform.tenants is global
		rows, err := tx.QueryContext(ctx, `SELECT m.tenant_id, m.member_kind, m.school_id, t.tenant_type, t.name, t.status
			FROM identity.memberships_for_user($1) m
			JOIN platform.tenants t ON t.tenant_id = m.tenant_id AND t.deleted_at IS NULL`, userID)
		if err != nil {
			return fmt.Errorf("login memberships: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var m gamelogin.Membership
			var school uuid.NullUUID
			if err := rows.Scan(&m.TenantID, &m.MemberKind, &school, &m.TenantType, &m.TenantName, &m.TenantStatus); err != nil {
				return fmt.Errorf("login memberships: %w", err)
			}
			if school.Valid {
				s := school.UUID
				m.SchoolID = &s
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	return out, err
}
