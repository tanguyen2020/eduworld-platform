package read

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

// Request implements read.RegradeReadModel.
func (RegradeModel) Request(ctx context.Context, id uuid.UUID, platform bool) (*dto.RegradeRequestView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var v dto.RegradeRequestView
	var tenant, qv, av, cv uuid.NullUUID
	var started, finished sql.NullTime
	// tenancyguard:ok platform request (tenant_id IS NULL, PlatformScope) or a request of the session tenant (RLS + infra.current_tenant_id()); deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT regrade_request_id, scope, action, status, affected_attempts, re_evaluated_attempts, reason,
			requested_by, created_at, started_at, finished_at, question_version_id, activity_version_id, corrected_version_id, tenant_id
		FROM assessment.regrade_requests
		WHERE regrade_request_id = $1 AND deleted_at IS NULL
			AND (($2 AND tenant_id IS NULL) OR (NOT $2 AND tenant_id = infra.current_tenant_id()))`, id, platform).
		Scan(&v.RegradeRequestID, &v.Scope, &v.Action, &v.Status, &v.AffectedAttempts, &v.ReEvaluatedAttempts, &v.Reason, &v.RequestedBy,
			&v.CreatedAt, &started, &finished, &qv, &av, &cv, &tenant)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("regrade request: %w", err)
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
	return &v, nil
}
