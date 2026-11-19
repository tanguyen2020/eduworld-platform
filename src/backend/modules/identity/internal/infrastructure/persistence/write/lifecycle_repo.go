package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LifecycleRepo implements write.LifecycleRepository with explicit SQL: identity.users is global (G); the
// student row is read under the scope of the caller (RLS hides students of other tenants from identity_app);
// identity.user_status_history is TA* (RLS).
type LifecycleRepo struct{}

var _ write.LifecycleRepository = LifecycleRepo{}

// Target implements write.LifecycleRepository.
func (LifecycleRepo) Target(ctx context.Context, userID uuid.UUID) (*write.LifecycleTarget, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var (
		t             write.LifecycleTarget
		primary       uuid.NullUUID
		stTenant, sch uuid.NullUUID
		grade         sql.NullInt64
	)
	// tenancyguard:ok identity.users is global (G); org.students under the scope (RLS); deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT u.user_id, u.user_type, u.username, u.status, u.primary_tenant_id, u.version, s.tenant_id, s.school_id, s.grade_level
		FROM identity.users u
		LEFT JOIN org.students s ON s.student_id = u.user_id AND s.deleted_at IS NULL
		WHERE u.user_id = $1 AND u.deleted_at IS NULL`, userID).
		Scan(&t.UserID, &t.UserType, &t.Username, &t.Status, &primary, &t.Version, &stTenant, &sch, &grade)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lifecycle target: %w", err)
	}
	if primary.Valid {
		v := primary.UUID
		t.PrimaryTenantID = &v
	}
	if stTenant.Valid {
		v := stTenant.UUID
		t.StudentTenantID = &v
	}
	if sch.Valid {
		v := sch.UUID
		t.StudentSchoolID = &v
	}
	t.GradeLevel = int(grade.Int64)
	return &t, nil
}

// SetStatus implements write.LifecycleRepository.
func (LifecycleRepo) SetStatus(ctx context.Context, userID uuid.UUID, from, to string) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok identity.users is global (G); one live user by id and current status
	res, err := tx.ExecContext(ctx, `UPDATE identity.users SET status = $3::varchar, updated_at = now(), version = version + 1,
			deactivated_at = CASE WHEN $3::varchar = 'deactivated' THEN now() ELSE deactivated_at END
		WHERE user_id = $1 AND status = $2 AND deleted_at IS NULL`, userID, from, to)
	if err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return problem.New(problem.Conflict, "the status changed meanwhile")
	}
	return nil
}

// RecordStatusChange implements write.LifecycleRepository.
func (LifecycleRepo) RecordStatusChange(ctx context.Context, c write.StatusChange) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	var reason *string
	if c.Reason != "" {
		reason = &c.Reason
	}
	// tenancyguard:ok append-only history; RLS admits identity_app rows of its tenant, identity_platform any row
	_, err = tx.ExecContext(ctx, `INSERT INTO identity.user_status_history (history_id, tenant_id, user_id, from_status, to_status, reason,
			source, changed_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		uuid.Must(uuid.NewV7()), c.TenantID, c.UserID, c.From, c.To, reason, c.Source, c.By)
	if err != nil {
		return fmt.Errorf("status history: %w", err)
	}
	return nil
}

// KeycloakSubject implements write.LifecycleRepository.
func (LifecycleRepo) KeycloakSubject(ctx context.Context, userID uuid.UUID, issuers []string) (string, error) {
	return PortalAccountRepo{}.KeycloakSubject(ctx, userID, issuers)
}
