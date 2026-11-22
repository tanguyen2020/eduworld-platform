package read

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// RegradeModel implements read.RegradeReadModel with explicit SQL in the read transaction of ctx.
type RegradeModel struct{}

var _ read.RegradeReadModel = RegradeModel{}

func nullID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

const regradeColumns = `regrade_request_id, scope, action, status, affected_attempts, re_evaluated_attempts, reason,
	requested_by, created_at, started_at, finished_at, question_version_id, activity_version_id, corrected_version_id, tenant_id`

func scanRegrade(row interface{ Scan(...any) error }) (dto.RegradeRequestView, error) {
	var v dto.RegradeRequestView
	var tenant, qv, av, cv uuid.NullUUID
	var started, finished sql.NullTime
	if err := row.Scan(&v.RegradeRequestID, &v.Scope, &v.Action, &v.Status, &v.AffectedAttempts, &v.ReEvaluatedAttempts, &v.Reason,
		&v.RequestedBy, &v.CreatedAt, &started, &finished, &qv, &av, &cv, &tenant); err != nil {
		return v, err
	}
	v.CreatedAt = v.CreatedAt.UTC()
	if started.Valid {
		t := started.Time.UTC()
		v.StartedAt = &t
	}
	if finished.Valid {
		t := finished.Time.UTC()
		v.FinishedAt = &t
	}
	v.QuestionVersionID, v.ActivityVersionID, v.CorrectedVersionID, v.TenantID = nullID(qv), nullID(av), nullID(cv), nullID(tenant)
	return v, nil
}

// Request implements read.RegradeReadModel.
func (RegradeModel) Request(ctx context.Context, id uuid.UUID, platform bool) (*dto.RegradeRequestView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform request (tenant_id IS NULL, PlatformScope) or a request of the session tenant (RLS + infra.current_tenant_id()); deleted_at IS NULL
	v, err := scanRegrade(q.QueryRowContext(ctx, `SELECT `+regradeColumns+` FROM assessment.regrade_requests
		WHERE regrade_request_id = $1 AND deleted_at IS NULL
			AND (($2 AND tenant_id IS NULL) OR (NOT $2 AND tenant_id = infra.current_tenant_id()))`, id, platform))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("regrade request: %w", err)
	}
	return &v, nil
}

// Requests implements read.RegradeReadModel.
func (RegradeModel) Requests(ctx context.Context, questionVersionID, activityVersionID *uuid.UUID, platform bool, after *read.RegradeKey, limit int) ([]dto.RegradeRequestView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var afterAt *time.Time
	afterID := uuid.Nil
	if after != nil {
		afterAt, afterID = &after.CreatedAt, after.ID
	}
	// tenancyguard:ok platform requests (tenant_id IS NULL, PlatformScope) or requests of the session tenant (RLS + infra.current_tenant_id()); deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT `+regradeColumns+` FROM assessment.regrade_requests
		WHERE deleted_at IS NULL AND (($1 AND tenant_id IS NULL) OR (NOT $1 AND tenant_id = infra.current_tenant_id()))
			AND ($2::uuid IS NULL OR question_version_id = $2) AND ($3::uuid IS NULL OR activity_version_id = $3)
			AND ($5::timestamptz IS NULL OR (created_at, regrade_request_id) < ($5::timestamptz, $6::uuid))
		ORDER BY created_at DESC, regrade_request_id DESC LIMIT $4`, platform, questionVersionID, activityVersionID, limit, afterAt, afterID)
	if err != nil {
		return nil, fmt.Errorf("regrade requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []dto.RegradeRequestView{}
	for rows.Next() {
		v, err := scanRegrade(rows)
		if err != nil {
			return nil, fmt.Errorf("regrade requests: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
