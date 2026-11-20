package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/evaluation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// RegradeRepo implements write.RegradeStore with explicit SQL in the transaction of ctx. assessment.regrade_requests has
// RLS T on tenant_id: platform requests (tenant_id NULL) are reachable through learning_platform only (PlatformScope or
// tenancy.Bypass of the job).
type RegradeRepo struct{}

var _ write.RegradeStore = RegradeRepo{}

const regradeColumns = `regrade_request_id, tenant_id, subject_id, scope, question_version_id, activity_version_id, corrected_version_id,
	action, reason, status, affected_attempts, re_evaluated_attempts, requested_by, requested_role, idempotency_key, created_at, started_at,
	finished_at`

func nullUUID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

func nullTime(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time.UTC()
	return &t
}

// ScanRegrade scans regradeColumns.
func scanRegrade(row interface{ Scan(...any) error }) (write.RegradeRequest, error) {
	var r write.RegradeRequest
	var tenant, subject, qv, av, cv uuid.NullUUID
	var started, finished sql.NullTime
	var action string
	err := row.Scan(&r.ID, &tenant, &subject, &r.Scope, &qv, &av, &cv, &action, &r.Reason, &r.Status, &r.Affected, &r.ReEvaluated,
		&r.RequestedBy, &r.RequestedRole, &r.IdempotencyKey, &r.CreatedAt, &started, &finished)
	r.TenantID, r.SubjectID, r.QuestionVersionID, r.ActivityVersionID, r.CorrectedVersionID =
		nullUUID(tenant), nullUUID(subject), nullUUID(qv), nullUUID(av), nullUUID(cv)
	r.Action, r.StartedAt, r.FinishedAt = evaluation.Action(action), nullTime(started), nullTime(finished)
	r.CreatedAt = r.CreatedAt.UTC()
	return r, err
}

// InsertRequest implements write.RegradeStore.
func (RegradeRepo) InsertRequest(ctx context.Context, r write.RegradeRequest) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok insert of the session tenant or a platform row under PlatformScope; conflict on the active-request unique
	res, err := tx.ExecContext(ctx, `INSERT INTO assessment.regrade_requests (regrade_request_id, tenant_id, subject_id, scope,
		question_version_id, activity_version_id, corrected_version_id, action, reason, status, requested_by, requested_role, idempotency_key,
		created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'queued', $10, $11, $12, $13, $13)
		ON CONFLICT ((coalesce(question_version_id, activity_version_id, exam_session_id)),
			(coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid)))
			WHERE status IN ('queued','running') AND deleted_at IS NULL DO NOTHING`,
		r.ID, r.TenantID, r.SubjectID, r.Scope, r.QuestionVersionID, r.ActivityVersionID, r.CorrectedVersionID, string(r.Action), r.Reason,
		r.RequestedBy, r.RequestedRole, r.IdempotencyKey, r.CreatedAt.UTC())
	if err != nil {
		return false, fmt.Errorf("insert regrade request: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// RequestByKey implements write.RegradeStore.
func (RegradeRepo) RequestByKey(ctx context.Context, requestedBy uuid.UUID, key string) (*write.RegradeRequest, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok rows of the requester under the scope of ctx (RLS T, platform rows only for learning_platform), deleted_at IS NULL
	r, err := scanRegrade(tx.QueryRowContext(ctx, `SELECT `+regradeColumns+` FROM assessment.regrade_requests
		WHERE requested_by = $1 AND idempotency_key = $2 AND deleted_at IS NULL`, requestedBy, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("regrade request by key: %w", err)
	}
	return &r, nil
}

// ClaimQueued implements write.RegradeStore (ctx: tenancy.Bypass(re_evaluation) of the job).
func (RegradeRepo) ClaimQueued(ctx context.Context, now time.Time) (*write.RegradeRequest, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok job regrade-runner under tenancy.Bypass(re_evaluation): queue of every tenant and the platform, deleted_at IS NULL
	r, err := scanRegrade(tx.QueryRowContext(ctx, `UPDATE assessment.regrade_requests SET status = 'running', started_at = $1, updated_at = $1
		WHERE regrade_request_id = (SELECT regrade_request_id FROM assessment.regrade_requests
			WHERE status = 'queued' AND deleted_at IS NULL ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING `+regradeColumns, now.UTC()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim regrade request: %w", err)
	}
	return &r, nil
}

// FinishRequest implements write.RegradeStore.
func (RegradeRepo) FinishRequest(ctx context.Context, id uuid.UUID, status string, affected, done int, lastError string, now time.Time) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var le any
	if lastError != "" {
		if len(lastError) > 500 {
			lastError = lastError[:500]
		}
		le = lastError
	}
	// tenancyguard:ok job regrade-runner under tenancy.Bypass(re_evaluation): one request by id, deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE assessment.regrade_requests SET status = $2, affected_attempts = $3, re_evaluated_attempts = $4,
		last_error = $5, finished_at = $6, updated_at = $6 WHERE regrade_request_id = $1 AND deleted_at IS NULL`,
		id, status, affected, done, le, now.UTC())
	if err != nil {
		return fmt.Errorf("finish regrade request: %w", err)
	}
	return nil
}

func (RegradeRepo) version(ctx context.Context, query string, id uuid.UUID) (*write.VersionInfo, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	v := write.VersionInfo{ID: id}
	var tenant, subject uuid.NullUUID
	err = tx.QueryRowContext(ctx, query, id).Scan(&v.ItemID, &tenant, &subject, &v.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("version %s: %w", id, err)
	}
	v.TenantID, v.SubjectID = nullUUID(tenant), nullUUID(subject)
	return &v, nil
}

// QuestionVersion implements write.RegradeStore (RLS H: own and assigned platform versions of the session tenant).
func (r RegradeRepo) QuestionVersion(ctx context.Context, id uuid.UUID) (*write.VersionInfo, error) {
	// tenancyguard:ok H table content.question_versions: tenant + assignment visibility is its RLS policy; deleted_at IS NULL
	return r.version(ctx, `SELECT question_id, tenant_id, subject_id, status FROM content.question_versions
		WHERE question_version_id = $1 AND deleted_at IS NULL`, id)
}

// ActivityVersion implements write.RegradeStore.
func (r RegradeRepo) ActivityVersion(ctx context.Context, id uuid.UUID) (*write.VersionInfo, error) {
	// tenancyguard:ok H table content.activity_versions: tenant + assignment visibility is its RLS policy; deleted_at IS NULL
	return r.version(ctx, `SELECT activity_id, tenant_id, subject_id, status FROM content.activity_versions
		WHERE activity_version_id = $1 AND deleted_at IS NULL`, id)
}

// AffectedAttempts implements write.RegradeStore: graded attempts (mode ≠ exam) whose items pin the question version, or
// pinned to the activity version; limited to the tenant of a tenant request.
func (RegradeRepo) AffectedAttempts(ctx context.Context, r write.RegradeRequest) ([]write.AffectedAttempt, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok job regrade-runner under tenancy.Bypass(re_evaluation): tenant of the request ($3) or every tenant, deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT a.attempt_id, a.tenant_id FROM assessment.attempts a
		WHERE a.status = 'graded' AND a.mode <> 'exam' AND a.deleted_at IS NULL AND ($3::uuid IS NULL OR a.tenant_id = $3)
			AND (($1::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM assessment.attempt_items i WHERE i.attempt_id = a.attempt_id
					AND i.question_version_id = $1 AND i.deleted_at IS NULL))
				OR ($2::uuid IS NOT NULL AND a.source_version_id = $2))
		ORDER BY a.tenant_id, a.attempt_id`, r.QuestionVersionID, r.ActivityVersionID, r.TenantID)
	if err != nil {
		return nil, fmt.Errorf("affected attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AffectedAttempt
	for rows.Next() {
		var a write.AffectedAttempt
		if err := rows.Scan(&a.ID, &a.TenantID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Effective implements write.RegradeStore.
func (RegradeRepo) Effective(ctx context.Context, attemptID uuid.UUID) (*write.EffectiveEvaluation, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var e write.EffectiveEvaluation
	var details []byte
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT evaluation_id, status, score::float8, max_score::float8, details FROM assessment.evaluations
		WHERE attempt_id = $1 AND effective AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, attemptID).
		Scan(&e.ID, &e.Status, &e.Score, &e.MaxScore, &details)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("effective evaluation: %w", err)
	}
	if e.Items, err = ItemsOf(details); err != nil {
		return nil, err
	}
	return &e, nil
}

// ItemsOf reads the items of evaluation details: {"items": [...]} with the keys of evaluation.Item, or the keys of the
// system evaluations of F030 (activity.ItemResult without JSON tags).
func ItemsOf(details []byte) ([]evaluation.Item, error) {
	if len(details) == 0 {
		return nil, nil
	}
	var d struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(details, &d); err != nil {
		return nil, fmt.Errorf("evaluation details: %w", err)
	}
	num := func(m map[string]any, keys ...string) float64 {
		for _, k := range keys {
			if f, ok := m[k].(float64); ok {
				return f
			}
		}
		return 0
	}
	flag := func(m map[string]any, keys ...string) bool {
		for _, k := range keys {
			if b, ok := m[k].(bool); ok {
				return b
			}
		}
		return false
	}
	out := make([]evaluation.Item, 0, len(d.Items))
	for _, m := range d.Items {
		out = append(out, evaluation.Item{Position: int(num(m, "position", "Position")), MaxScore: num(m, "max_score", "MaxScore"),
			Score: num(m, "score", "Score"), IsCorrect: flag(m, "is_correct", "IsCorrect"), Answered: flag(m, "answered", "Answered"),
			Dropped: flag(m, "dropped", "Dropped")})
	}
	return out, nil
}

// Supersede implements write.RegradeStore.
func (RegradeRepo) Supersede(ctx context.Context, id uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE assessment.evaluations SET status = 'superseded', effective = false, updated_at = now()
		WHERE evaluation_id = $1 AND status = 'final' AND effective AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("supersede evaluation: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return problem.New(problem.EvaluationNotEditable, "the evaluation is not the effective final evaluation")
	}
	return nil
}

// LastScoredResponse implements write.RegradeStore (partitioned append-only attempt_answers).
func (RegradeRepo) LastScoredResponse(ctx context.Context, attemptID uuid.UUID, position int) (json.RawMessage, bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, false, err
	}
	var resp []byte
	// tenancyguard:ok append-only attempt_answers without deleted_at; tenant of the session (RLS + infra.current_tenant_id())
	err = tx.QueryRowContext(ctx, `SELECT response FROM assessment.attempt_answers WHERE attempt_id = $1 AND position = $2 AND is_scored
		AND tenant_id = infra.current_tenant_id() ORDER BY answered_at DESC, attempt_no DESC LIMIT 1`, attemptID, position).Scan(&resp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("last scored response: %w", err)
	}
	return resp, true, nil
}
