package write

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// MergeRepo implements write.MergeLinkStore with explicit SQL in the transaction of ctx: learning.student_merge_links is
// a tenant table (RLS T, infra.current_tenant_id()), soft-delete only (AGENTS R21).
type MergeRepo struct{}

var _ write.MergeLinkStore = MergeRepo{}

const mergeColumns = `merge_id, tenant_id, source_student_id, target_student_id, reason, created_by, created_at`

func mergeOne(ctx context.Context, where string, arg uuid.UUID) (*write.MergeLink, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var l write.MergeLink
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL in the condition
	err = tx.QueryRowContext(ctx, `SELECT `+mergeColumns+` FROM learning.student_merge_links
		WHERE tenant_id = infra.current_tenant_id() AND deleted_at IS NULL AND `+where, arg).
		Scan(&l.MergeID, &l.TenantID, &l.SourceID, &l.TargetID, &l.Reason, &l.CreatedBy, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	l.CreatedAt = l.CreatedAt.UTC()
	return &l, nil
}

// MergeLinkByID implements write.MergeLinkStore.
func (MergeRepo) MergeLinkByID(ctx context.Context, mergeID uuid.UUID) (*write.MergeLink, error) {
	return mergeOne(ctx, `merge_id = $1`, mergeID)
}

// MergeLinkOfSource implements write.MergeLinkStore.
func (MergeRepo) MergeLinkOfSource(ctx context.Context, sourceID uuid.UUID) (*write.MergeLink, error) {
	return mergeOne(ctx, `source_student_id = $1`, sourceID)
}

// HasStudentData implements write.MergeLinkStore.
func (MergeRepo) HasStudentData(ctx context.Context, studentID uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var found bool
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on every table
	err = tx.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM assessment.attempts WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND deleted_at IS NULL)
		OR EXISTS (SELECT 1 FROM learning.objective_masteries WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND deleted_at IS NULL)
		OR EXISTS (SELECT 1 FROM learning.lesson_sessions WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND deleted_at IS NULL)`,
		studentID).Scan(&found)
	return found, err
}

// InsertMergeLink implements write.MergeLinkStore.
func (MergeRepo) InsertMergeLink(ctx context.Context, l write.MergeLink) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant_id of the row is the tenant of the session (RLS tenant_insert WITH CHECK)
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.student_merge_links (merge_id, tenant_id, source_student_id, target_student_id,
		reason, created_by, created_at, updated_at) VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6, $6)`,
		l.MergeID, l.SourceID, l.TargetID, l.Reason, l.CreatedBy, l.CreatedAt)
	return err
}
