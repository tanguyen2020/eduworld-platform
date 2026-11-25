package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// QuestRepo implements write.QuestStore on game.quest_progress, game.quest_step_attempts and game.periodic_quest_grants
// (F072) and write.AssignmentStore on infra.tenant_resource_assignments.
type QuestRepo struct{}

var (
	_ write.QuestStore      = QuestRepo{}
	_ write.AssignmentStore = QuestRepo{}
)

// LockStudent implements write.QuestStore.
func (QuestRepo) LockStudent(ctx context.Context, student uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok advisory lock only (no table read)
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('game.quest:' || $1::text, 0))`, student); err != nil {
		return fmt.Errorf("quest: lock student: %w", err)
	}
	return nil
}

func queryProgress(ctx context.Context, query string, args ...any) ([]*quest.Progress, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("quest progress: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*quest.Progress
	for rows.Next() {
		p, err := questsql.ScanProgress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func firstProgress(ps []*quest.Progress, err error) (*quest.Progress, error) {
	if err != nil || len(ps) == 0 {
		return nil, err
	}
	return ps[0], nil
}

// Active implements write.QuestStore.
func (QuestRepo) Active(ctx context.Context, tenant, student, questID uuid.UUID) (*quest.Progress, error) {
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return firstProgress(queryProgress(ctx, `SELECT `+questsql.ProgressColumns+` FROM game.quest_progress
		WHERE tenant_id = $1 AND student_id = $2 AND quest_id = $3 AND status = 'active' AND deleted_at IS NULL FOR UPDATE`, tenant, student, questID))
}

// ByID implements write.QuestStore.
func (QuestRepo) ByID(ctx context.Context, tenant, id uuid.UUID) (*quest.Progress, error) {
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return firstProgress(queryProgress(ctx, `SELECT `+questsql.ProgressColumns+` FROM game.quest_progress
		WHERE tenant_id = $1 AND quest_progress_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, id))
}

// ActiveOfStudent implements write.QuestStore.
func (QuestRepo) ActiveOfStudent(ctx context.Context, tenant, student uuid.UUID) ([]*quest.Progress, error) {
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return queryProgress(ctx, `SELECT `+questsql.ProgressColumns+` FROM game.quest_progress
		WHERE tenant_id = $1 AND student_id = $2 AND status = 'active' AND deleted_at IS NULL ORDER BY started_at FOR UPDATE`, tenant, student)
}

// CountActive implements write.QuestStore.
func (QuestRepo) CountActive(ctx context.Context, tenant, student uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	if err := t.QueryRowContext(ctx, `SELECT count(*) FROM game.quest_progress
		WHERE tenant_id = $1 AND student_id = $2 AND status = 'active' AND deleted_at IS NULL`, tenant, student).Scan(&n); err != nil {
		return 0, fmt.Errorf("quest progress: count: %w", err)
	}
	return n, nil
}

func stepsJSON(p *quest.Progress) (string, error) {
	b, err := json.Marshal(p.Steps)
	if err != nil {
		return "", fmt.Errorf("quest step states: %w", err)
	}
	return string(b), nil
}

// Insert implements write.QuestStore: a unique index refusal (one active per quest, one per period) is false.
func (QuestRepo) Insert(ctx context.Context, p *quest.Progress) (bool, error) {
	steps, err := stepsJSON(p)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id (RLS insert policy); unique among live rows
	ok, err := exec(ctx, `INSERT INTO game.quest_progress (quest_progress_id, tenant_id, created_at, created_by, updated_at, student_id, school_id,
			quest_id, quest_version_id, quest_type, status, step_states, start_trigger, period_key, completion_no, party_id, started_at, expires_at,
			deadline_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, $13, $14, $15, $3, $16, $17)
		ON CONFLICT DO NOTHING`, p.ID, p.TenantID, p.StartedAt, actor(ctx), p.StudentID, p.SchoolID, p.QuestID, p.VersionID, p.QuestType, p.Status,
		steps, p.StartTrigger, nullString(p.PeriodKey), p.CompletionNo, p.PartyID, p.ExpiresAt, p.DeadlineAt)
	if err != nil {
		return false, fmt.Errorf("quest progress: insert: %w", err)
	}
	return ok, nil
}

// ErrProgressConflict is a concurrent change of a progress (optimistic version check).
var ErrProgressConflict = errors.New("quest progress: concurrent change")

// Save implements write.QuestStore.
func (QuestRepo) Save(ctx context.Context, p *quest.Progress) error {
	steps, err := stepsJSON(p)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id + deleted_at IS NULL (RLS update policy)
	// party_id: a progress joins the shared session of a party quest (F103 startPartyActivity); it is never cleared.
	ok, err := exec(ctx, `UPDATE game.quest_progress SET status = $3, step_states = $4::jsonb, completed_at = $5, expires_at = $6, deadline_at = $7,
			failed_at = $8, ended_at = $9, party_id = COALESCE($12, party_id), updated_at = now(), updated_by = $10, version = version + 1
		WHERE tenant_id = $1 AND quest_progress_id = $2 AND version = $11 AND deleted_at IS NULL`, p.TenantID, p.ID, p.Status, steps, p.CompletedAt,
		p.ExpiresAt, p.DeadlineAt, p.FailedAt, p.EndedAt, actor(ctx), p.Version, p.PartyID)
	if err != nil {
		return fmt.Errorf("quest progress: save: %w", err)
	}
	if !ok {
		return ErrProgressConflict
	}
	p.Version++
	return nil
}

// InsertAttempt implements write.QuestStore (a retried start with the same attempt adds nothing).
func (QuestRepo) InsertAttempt(ctx context.Context, a write.StepAttempt) error {
	// tenancyguard:ok tenant table game.quest_step_attempts: explicit tenant_id (RLS insert policy); unique among live rows
	_, err := exec(ctx, `INSERT INTO game.quest_step_attempts (quest_step_attempt_id, tenant_id, created_at, created_by, updated_at,
			quest_progress_id, step_id, student_id, attempt_id, content_version_id, started_on, status)
		VALUES ($1, $2, now(), $3, now(), $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id, attempt_id) WHERE deleted_at IS NULL DO NOTHING`, a.ID, a.TenantID, actor(ctx), a.ProgressID, a.StepID, a.StudentID,
		a.AttemptID, a.ContentVersionID, a.StartedOn, a.Status)
	if err != nil {
		return fmt.Errorf("quest step attempts: insert: %w", err)
	}
	return nil
}

// AttemptByAttemptID implements write.QuestStore.
func (QuestRepo) AttemptByAttemptID(ctx context.Context, tenant, attemptID uuid.UUID) (*write.StepAttempt, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var a write.StepAttempt
	var cv uuid.NullUUID
	var correct, items sql.NullInt64
	var graded sql.NullTime
	// tenancyguard:ok tenant table game.quest_step_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT quest_step_attempt_id, tenant_id, quest_progress_id, step_id, student_id, attempt_id, content_version_id,
			started_on, status, correct_count, item_count, graded_at
		FROM game.quest_step_attempts WHERE tenant_id = $1 AND attempt_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, attemptID).
		Scan(&a.ID, &a.TenantID, &a.ProgressID, &a.StepID, &a.StudentID, &a.AttemptID, &cv, &a.StartedOn, &a.Status, &correct, &items, &graded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("quest step attempts: %w", err)
	}
	if cv.Valid {
		a.ContentVersionID = &cv.UUID
	}
	if correct.Valid {
		v := int(correct.Int64)
		a.CorrectCount = &v
	}
	if items.Valid {
		v := int(items.Int64)
		a.ItemCount = &v
	}
	if graded.Valid {
		g := graded.Time.UTC()
		a.GradedAt = &g
	}
	return &a, nil
}

// SaveAttempt implements write.QuestStore.
func (QuestRepo) SaveAttempt(ctx context.Context, a write.StepAttempt) error {
	// tenancyguard:ok tenant table game.quest_step_attempts: explicit tenant_id + deleted_at IS NULL (RLS update policy)
	_, err := exec(ctx, `UPDATE game.quest_step_attempts SET status = $3, correct_count = $4, item_count = $5, graded_at = $6, updated_at = now(),
			updated_by = $7, version = version + 1
		WHERE tenant_id = $1 AND quest_step_attempt_id = $2 AND deleted_at IS NULL`, a.TenantID, a.ID, a.Status, a.CorrectCount, a.ItemCount,
		a.GradedAt, actor(ctx))
	if err != nil {
		return fmt.Errorf("quest step attempts: save: %w", err)
	}
	return nil
}

// CountAttemptsOn implements write.QuestStore.
func (QuestRepo) CountAttemptsOn(ctx context.Context, tenant, progressID uuid.UUID, stepID, day string) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok tenant table game.quest_step_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	if err := t.QueryRowContext(ctx, `SELECT count(*) FROM game.quest_step_attempts
		WHERE tenant_id = $1 AND quest_progress_id = $2 AND step_id = $3 AND started_on = $4 AND deleted_at IS NULL`,
		tenant, progressID, stepID, day).Scan(&n); err != nil {
		return 0, fmt.Errorf("quest step attempts: count: %w", err)
	}
	return n, nil
}

// InsertGrant implements write.QuestStore.
func (QuestRepo) InsertGrant(ctx context.Context, tenant, student uuid.UUID, kind, period string, questIDs []uuid.UUID, at time.Time) (bool, error) {
	b, err := json.Marshal(questIDs)
	if err != nil {
		return false, err
	}
	if questIDs == nil {
		b = []byte("[]")
	}
	// tenancyguard:ok tenant table game.periodic_quest_grants: explicit tenant_id (RLS insert policy); unique among live rows
	ok, err := exec(ctx, `INSERT INTO game.periodic_quest_grants (grant_id, tenant_id, created_at, created_by, updated_at, student_id, kind,
			period_key, quest_ids, granted_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8::jsonb, $3)
		ON CONFLICT (tenant_id, student_id, kind, period_key) WHERE deleted_at IS NULL DO NOTHING`, ids.New(), tenant, at, actor(ctx), student, kind,
		period, string(b))
	if err != nil {
		return false, fmt.Errorf("periodic quest grants: %w", err)
	}
	return ok, nil
}

func progressRefs(ctx context.Context, query string, args ...any) ([]write.ProgressRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("quest progress refs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ProgressRef
	for rows.Next() {
		var r write.ProgressRef
		if err := rows.Scan(&r.ID, &r.TenantID, &r.StudentID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Expiring implements write.QuestStore.
func (QuestRepo) Expiring(ctx context.Context, before time.Time, limit int) ([]write.ProgressRef, error) {
	// tenancyguard:ok platform job scope: lists active progress of every tenant (each is then handled in its tenant); live rows only
	return progressRefs(ctx, `SELECT quest_progress_id, tenant_id, student_id FROM game.quest_progress
		WHERE status = 'active' AND expires_at IS NOT NULL AND expires_at <= $1 AND deleted_at IS NULL ORDER BY expires_at LIMIT $2`, before, limit)
}

// Overdue implements write.QuestStore.
func (QuestRepo) Overdue(ctx context.Context, before time.Time, limit int) ([]write.ProgressRef, error) {
	// tenancyguard:ok platform job scope: lists active progress of every tenant (each is then handled in its tenant); live rows only
	return progressRefs(ctx, `SELECT quest_progress_id, tenant_id, student_id FROM game.quest_progress
		WHERE status = 'active' AND deadline_at IS NOT NULL AND deadline_at <= $1 AND deleted_at IS NULL ORDER BY deadline_at LIMIT $2`, before, limit)
}

// ActiveEndingBy implements write.QuestStore.
func (QuestRepo) ActiveEndingBy(ctx context.Context, before time.Time, limit int) ([]write.ProgressRef, error) {
	// tenancyguard:ok platform job scope: active progress of every tenant on a version whose availability ended; live rows only
	return progressRefs(ctx, `SELECT p.quest_progress_id, p.tenant_id, p.student_id FROM game.quest_progress p
		JOIN game.quest_versions v ON v.quest_version_id = p.quest_version_id AND v.deleted_at IS NULL
		WHERE p.status = 'active' AND p.deleted_at IS NULL AND v.available_to IS NOT NULL AND v.available_to <= $1
		ORDER BY v.available_to LIMIT $2`, before, limit)
}

// Assign implements write.AssignmentStore (platform scope).
func (QuestRepo) Assign(ctx context.Context, resourceType string, resourceID, tenant, by uuid.UUID, at time.Time) (bool, error) {
	// tenancyguard:ok infra.tenant_resource_assignments (no RLS, ADR-0089 §4): platform write of one tenant; unique among live rows
	ok, err := exec(ctx, `INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at, created_by, updated_at,
			resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $4, $3)
		ON CONFLICT (resource_type, resource_id, tenant_id) WHERE deleted_at IS NULL DO NOTHING`, ids.New(), tenant, at, by, resourceType, resourceID)
	if err != nil {
		return false, fmt.Errorf("tenant resource assignments: %w", err)
	}
	return ok, nil
}

// TenantByCode implements write.AssignmentStore.
func (QuestRepo) TenantByCode(ctx context.Context, code string) (*uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	// tenancyguard:ok projection ref.tenants read under the platform scope (seed): tenant lookup by code only
	err = t.QueryRowContext(ctx, `SELECT tenant_id FROM ref.tenants WHERE code = $1`, code).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref tenants: %w", err)
	}
	return &id, nil
}
