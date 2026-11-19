package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/audit"
)

// AuditRepo implements write.AuditLogRepository with explicit SQL in the platform scope "audit-sink"
// (identity_platform: INSERT and SELECT only on the append-only audit log).
type AuditRepo struct{}

var _ write.AuditLogRepository = AuditRepo{}

// Insert implements write.AuditLogRepository.
func (AuditRepo) Insert(ctx context.Context, e audit.Entry) (bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return false, err
	}
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return false, err
	}
	var reason, trace *string
	if e.Reason != "" {
		reason = &e.Reason
	}
	if e.TraceID != "" {
		trace = &e.TraceID
	}
	// tenancyguard:ok platform scope audit-sink: append-only journal of every tenant, tenant_id from the envelope
	res, err := tx.ExecContext(ctx, `INSERT INTO audit.audit_logs (audit_id, occurred_at, actor_id, actor_type, tenant_id, school_id,
			action, target_type, target_id, result, reason, metadata, trace_id, producer)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, $14)
		ON CONFLICT (audit_id, occurred_at) DO NOTHING`,
		e.ID, e.OccurredAt, e.ActorID, e.ActorType, e.TenantID, e.SchoolID, e.Action, e.TargetType, e.TargetID, e.Result, reason,
		string(meta), trace, e.Producer)
	if err != nil {
		return false, fmt.Errorf("audit insert: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// SchoolTenant implements write.AuditLogRepository.
func (AuditRepo) SchoolTenant(ctx context.Context, schoolID uuid.UUID) (*uuid.UUID, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var t uuid.UUID
	// tenancyguard:ok platform scope audit-sink: the owner tenant of one school, compared with the envelope; deleted schools included
	err = tx.QueryRowContext(ctx, `SELECT tenant_id FROM org.schools WHERE school_id = $1`, schoolID).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("school tenant: %w", err)
	}
	return &t, nil
}
