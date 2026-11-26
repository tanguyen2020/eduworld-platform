package read

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// AssignmentRecipientModel implements read.AssignmentRecipientReadModel with explicit SQL in the read transaction of ctx
// (TenantScope: RLS + infra.current_tenant_id(), deleted_at IS NULL).
type AssignmentRecipientModel struct{}

var _ read.AssignmentRecipientReadModel = AssignmentRecipientModel{}

// Recipients implements read.AssignmentRecipientReadModel: the snapshot of the targeted students
// (lms.assignment_target_students) minus a live excuse (lms.assignment_student_exceptions kind = excuse), keyset on
// student_id. Only ids leave the module (R16).
func (AssignmentRecipientModel) Recipients(ctx context.Context, id uuid.UUID, after *uuid.UUID, limit int) (string, bool, []uuid.UUID, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return "", false, nil, err
	}
	var title string
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT title FROM lms.assignments WHERE assignment_id = $1 AND tenant_id = infra.current_tenant_id()
		AND deleted_at IS NULL`, id).Scan(&title)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil, nil
	}
	if err != nil {
		return "", false, nil, err
	}
	var from any
	if after != nil {
		from = *after
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on both tables
	rows, err := q.QueryContext(ctx, `SELECT ts.student_id FROM lms.assignment_target_students ts
		WHERE ts.assignment_id = $1 AND ts.tenant_id = infra.current_tenant_id() AND ts.deleted_at IS NULL
			AND ($2::uuid IS NULL OR ts.student_id > $2::uuid)
			AND NOT EXISTS (SELECT 1 FROM lms.assignment_student_exceptions x WHERE x.assignment_id = ts.assignment_id
				AND x.student_id = ts.student_id AND x.kind = 'excuse' AND x.tenant_id = infra.current_tenant_id()
				AND x.deleted_at IS NULL)
		ORDER BY ts.student_id LIMIT $3`, id, from, limit+1)
	if err != nil {
		return title, true, nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []uuid.UUID{}
	for rows.Next() {
		var s uuid.UUID
		if err := rows.Scan(&s); err != nil {
			return title, true, nil, err
		}
		out = append(out, s)
	}
	return title, true, out, rows.Err()
}
