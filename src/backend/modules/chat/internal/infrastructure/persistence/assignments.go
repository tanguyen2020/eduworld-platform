package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
)

// Assignments implements write.AssignmentStore on infra.tenant_resource_assignments of eduworld_chat (F082 T040, ADR-0029).
type Assignments struct{}

var _ write.AssignmentStore = Assignments{}

func uuidStrs(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func scanAssignments(rows *sql.Rows, err error) ([]write.Assignment, error) {
	if err != nil {
		return nil, fmt.Errorf("chat assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []write.Assignment{}
	for rows.Next() {
		var a write.Assignment
		if err := rows.Scan(&a.ID, &a.ResourceType, &a.ResourceID, &a.TenantID, &a.AssignedBy, &a.AssignedAt); err != nil {
			return nil, fmt.Errorf("chat assignments: %w", err)
		}
		a.AssignedAt = a.AssignedAt.UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}

// PackPublished implements write.AssignmentStore.
func (Assignments) PackPublished(ctx context.Context, id uuid.UUID) (bool, bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, false, err
	}
	var status string
	// tenancyguard:ok platform library (G table, no tenant_id); live pack only
	err = t.QueryRowContext(ctx, `SELECT status FROM chat.sticker_packs WHERE pack_id = $1 AND deleted_at IS NULL`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("chat assignments: pack: %w", err)
	}
	return true, status == "published", nil
}

// ActiveTenants implements write.AssignmentStore.
func (Assignments) ActiveTenants(ctx context.Context, tenants []uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.tenants: explicit tenant ids of the request; projections have no deleted_at
	rows, err := t.QueryContext(ctx, `SELECT tenant_id FROM ref.tenants WHERE tenant_id = ANY($1::uuid[]) AND status = 'active'`, uuidStrs(tenants))
	if err != nil {
		return nil, fmt.Errorf("chat assignments: tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Active implements write.AssignmentStore.
func (Assignments) Active(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID) ([]write.Assignment, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok explicit tenant condition (the requested tenants) on infra.tenant_resource_assignments; deleted_at filtered
	return scanAssignments(t.QueryContext(ctx, `SELECT assignment_id, resource_type, resource_id, tenant_id, assigned_by, assigned_at
		FROM infra.tenant_resource_assignments WHERE resource_type = $1 AND resource_id = $2 AND tenant_id = ANY($3::uuid[])
		AND deleted_at IS NULL ORDER BY tenant_id FOR UPDATE`, resourceType, id, uuidStrs(tenants)))
}

// Create implements write.AssignmentStore.
func (Assignments) Create(ctx context.Context, a write.Assignment) error {
	// tenancyguard:ok explicit tenant_id of the new assignment (CrossTenantScope of the requested tenants)
	_, err := exec(ctx, `INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at, created_by, updated_at, updated_by,
		resource_type, resource_id, assigned_by, assigned_at) VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $4, $3)`,
		a.ID, a.TenantID, a.AssignedAt, a.AssignedBy, a.ResourceType, a.ResourceID)
	if err != nil {
		return fmt.Errorf("chat assignments: create: %w", err)
	}
	return nil
}

// Revoke implements write.AssignmentStore (soft delete, R21).
func (Assignments) Revoke(ctx context.Context, a write.Assignment, by uuid.UUID, at time.Time) (write.Assignment, error) {
	// tenancyguard:ok explicit tenant_id of the assignment; soft delete (R21)
	_, err := exec(ctx, `UPDATE infra.tenant_resource_assignments SET deleted_at = $3, deleted_by = $4, delete_reason = 'revoked', updated_at = $3,
		updated_by = $4, version = version + 1 WHERE assignment_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, a.ID, a.TenantID, at, by)
	if err != nil {
		return a, fmt.Errorf("chat assignments: revoke: %w", err)
	}
	a.RevokedAt = &at
	return a, nil
}

// Page implements write.AssignmentStore.
func (Assignments) Page(ctx context.Context, resourceType string, id uuid.UUID, after *uuid.UUID, limit int) ([]write.Assignment, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var a any
	if after != nil {
		a = *after
	}
	// tenancyguard:ok every tenant of one platform resource (System Admin, platform.tenant_data.assign); deleted_at filtered; keyset
	return scanAssignments(t.QueryContext(ctx, `SELECT assignment_id, resource_type, resource_id, tenant_id, assigned_by, assigned_at
		FROM infra.tenant_resource_assignments WHERE resource_type = $1 AND resource_id = $2 AND deleted_at IS NULL
		AND ($3::uuid IS NULL OR tenant_id > $3) ORDER BY tenant_id LIMIT $4`, resourceType, id, a, limit))
}
