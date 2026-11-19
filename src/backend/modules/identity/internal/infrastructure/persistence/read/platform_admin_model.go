package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// PlatformAdminModel implements read.PlatformAdminReadModel (F045) with explicit SQL in a READ ONLY session.
// The caller sets the scope: a platform scope (System Admin) or tenancy.Bypass(trash) for the deleted accounts.
type PlatformAdminModel struct{ Session *entx.ReadSession }

var _ read.PlatformAdminReadModel = PlatformAdminModel{}

func (m PlatformAdminModel) tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return m.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		return fn(tx)
	})
}

// ActiveTenantIDs implements read.PlatformAdminReadModel.
func (m PlatformAdminModel) ActiveTenantIDs(ctx context.Context) (out []uuid.UUID, err error) {
	err = m.tx(ctx, func(tx *sql.Tx) error {
		// tenancyguard:ok platform.tenants is global (G): every live, active tenant
		rows, err := tx.QueryContext(ctx, `SELECT tenant_id FROM platform.tenants WHERE status = 'active' AND deleted_at IS NULL ORDER BY code`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, id)
		}
		return rows.Err()
	})
	return out, err
}

func sagaSteps(ctx context.Context, tx *sql.Tx, sagaID uuid.UUID) ([]read.SagaStepView, error) {
	// tenancyguard:ok platform.saga_steps is global (G), keyed by its saga
	rows, err := tx.QueryContext(ctx, `SELECT step_no, service, action, status, response, coalesce(error, '')
		FROM platform.saga_steps WHERE saga_id = $1 ORDER BY step_no`, sagaID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []read.SagaStepView
	for rows.Next() {
		var s read.SagaStepView
		var resp []byte
		if err := rows.Scan(&s.No, &s.Service, &s.Action, &s.Status, &resp, &s.Error); err != nil {
			return nil, err
		}
		s.Response = resp
		out = append(out, s)
	}
	return out, rows.Err()
}

// CloneJob implements read.PlatformAdminReadModel.
func (m PlatformAdminModel) CloneJob(ctx context.Context, id uuid.UUID) (out *read.CloneJobView, err error) {
	err = m.tx(ctx, func(tx *sql.Tx) error {
		var v read.CloneJobView
		var finished sql.NullTime
		var errs []byte
		var sagaID uuid.UUID
		// tenancyguard:ok platform.tenant_clone_jobs and platform.saga_runs are global (G): one job by id
		err := tx.QueryRowContext(ctx, `SELECT j.job_id, j.source_tenant_id, j.target_tenant_id, j.resource_type, j.status, j.id_map, j.errors,
				j.requested_by, j.created_at, j.finished_at, j.saga_id, s.status, coalesce(s.last_error, '')
			FROM platform.tenant_clone_jobs j JOIN platform.saga_runs s ON s.saga_id = j.saga_id WHERE j.job_id = $1`, id).
			Scan(&v.JobID, &v.Source, &v.Target, &v.ResourceType, &v.Status, &v.IDMap, &errs, &v.RequestedBy, &v.CreatedAt, &finished,
				&sagaID, &v.SagaStatus, &v.SagaError)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("clone job: %w", err)
		}
		if finished.Valid {
			v.FinishedAt = &finished.Time
		}
		if err := json.Unmarshal(errs, &v.Errors); err != nil {
			return err
		}
		if v.Steps, err = sagaSteps(ctx, tx, sagaID); err != nil {
			return err
		}
		out = &v
		return nil
	})
	return out, err
}

// UserMerge implements read.PlatformAdminReadModel.
func (m PlatformAdminModel) UserMerge(ctx context.Context, id uuid.UUID) (out *read.UserMergeView, err error) {
	err = m.tx(ctx, func(tx *sql.Tx) error {
		var v read.UserMergeView
		// tenancyguard:ok identity.user_merges is read by id in the platform scope of System Admin
		err := tx.QueryRowContext(ctx, `SELECT m.merge_id, m.saga_id, m.tenant_id, m.from_user_id, m.into_user_id, m.merged_at, s.status,
				coalesce(s.last_error, '')
			FROM identity.user_merges m JOIN platform.saga_runs s ON s.saga_id = m.saga_id WHERE m.merge_id = $1`, id).
			Scan(&v.MergeID, &v.SagaID, &v.TenantID, &v.From, &v.Into, &v.MergedAt, &v.SagaStatus, &v.SagaError)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("user merge: %w", err)
		}
		if v.Steps, err = sagaSteps(ctx, tx, v.SagaID); err != nil {
			return err
		}
		out = &v
		return nil
	})
	return out, err
}

func uuidList(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// DeletedAccounts implements read.PlatformAdminReadModel.
func (m PlatformAdminModel) DeletedAccounts(ctx context.Context, rt string, tenants []uuid.UUID, afterAt *time.Time, afterID uuid.UUID, limit int) (out []port.TrashEntry, err error) {
	err = m.tx(ctx, func(tx *sql.Tx) error {
		// The tenant of a deleted account: the tenant of its merge, else its primary tenant (students).
		// tenancyguard:ok explicit tenant condition (tenant list of the scope) under tenancy.Bypass(trash); deleted rows only
		rows, err := tx.QueryContext(ctx, `SELECT u.user_id, coalesce(m.tenant_id, u.primary_tenant_id), u.deleted_at,
				coalesce(u.deleted_by, m.merged_by, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(u.delete_reason, '')
			FROM identity.users u LEFT JOIN identity.user_merges m ON m.from_user_id = u.user_id
			WHERE u.deleted_at IS NOT NULL AND u.delete_reason = $5 AND ($5 <> 'inactive_cleanup' OR u.user_type = 'student')
				AND coalesce(m.tenant_id, u.primary_tenant_id) = ANY($1::uuid[])
				AND ($2::timestamptz IS NULL OR (u.deleted_at, u.user_id) < ($2, $3))
			ORDER BY u.deleted_at DESC, u.user_id DESC LIMIT $4`, uuidList(tenants), afterAt, afterID, limit+1, platformadmin.DeleteReasonOf(rt))
		if err != nil {
			return fmt.Errorf("deleted users: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var e port.TrashEntry
			var tenant uuid.UUID
			if err := rows.Scan(&e.ResourceID, &tenant, &e.DeletedAt, &e.DeletedBy, &e.DeleteReason); err != nil {
				return err
			}
			t := tenant
			e.TenantID, e.ResourceType, e.Title = &t, rt, rt // no PII in the trash
			e.Restorable = platformadmin.RestorableThroughTrash(rt)
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}
