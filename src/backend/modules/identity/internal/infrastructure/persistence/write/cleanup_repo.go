package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CleanupRepo implements write.CleanupRepository with explicit SQL (AGENTS R27): students and
// exclusions under the TenantScope (RLS) with deleted_at conditions; users global by id; runs global under the
// platform scope of the job. Removing = setting deleted_at (R21), never a DELETE.
type CleanupRepo struct{}

var _ write.CleanupRepository = CleanupRepo{}

// Candidates implements write.CleanupRepository.
func (CleanupRepo) Candidates(ctx context.Context, dueBefore, now time.Time, school *uuid.UUID) ([]write.CleanupCandidate, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok org.students and platform.cleanup_exclusions under the TenantScope (RLS); identity.users global by id; deleted_at filtered
	rows, err := tx.QueryContext(ctx, `SELECT s.student_id, s.school_id, coalesce(u.display_name, s.full_name), u.last_login_at, u.provisioned_at
		FROM org.students s
		JOIN identity.users u ON u.user_id = s.student_id AND u.deleted_at IS NULL AND u.status IN ('active', 'pending')
		WHERE s.deleted_at IS NULL AND coalesce(u.last_login_at, u.provisioned_at) < $1
			AND ($3::uuid IS NULL OR s.school_id = $3)
			AND NOT EXISTS (SELECT 1 FROM platform.cleanup_exclusions x WHERE x.student_id = s.student_id AND x.deleted_at IS NULL
				AND (x.until IS NULL OR x.until > $2))
		ORDER BY coalesce(u.last_login_at, u.provisioned_at), s.student_id`, dueBefore, now, school)
	if err != nil {
		return nil, fmt.Errorf("cleanup candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.CleanupCandidate
	for rows.Next() {
		var c write.CleanupCandidate
		var last sql.NullTime
		if err := rows.Scan(&c.StudentID, &c.SchoolID, &c.DisplayName, &last, &c.ProvisionedAt); err != nil {
			return nil, err
		}
		if last.Valid {
			c.LastLoginAt = &last.Time
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CandidatePage implements write.CleanupRepository: keyset page of the candidates (never the whole tenant).
func (CleanupRepo) CandidatePage(ctx context.Context, q write.CandidatePageQuery) ([]write.CleanupCandidate, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var schools any
	if q.Schools != nil {
		schools = nonNilIDs(q.Schools)
	}
	// tenancyguard:ok org.students and platform.cleanup_exclusions under the TenantScope (RLS); identity.users global by id; deleted_at filtered
	rows, err := tx.QueryContext(ctx, `SELECT s.student_id, s.school_id, coalesce(u.display_name, s.full_name), u.last_login_at, u.provisioned_at
		FROM org.students s
		JOIN identity.users u ON u.user_id = s.student_id AND u.deleted_at IS NULL AND u.status IN ('active', 'pending')
		WHERE s.deleted_at IS NULL AND coalesce(u.last_login_at, u.provisioned_at) < $1
			AND ($3::uuid[] IS NULL OR s.school_id = ANY($3::uuid[]))
			AND ($4::timestamptz IS NULL OR (coalesce(u.last_login_at, u.provisioned_at), s.student_id) > ($4::timestamptz, $5::uuid))
			AND NOT EXISTS (SELECT 1 FROM platform.cleanup_exclusions x WHERE x.student_id = s.student_id AND x.deleted_at IS NULL
				AND (x.until IS NULL OR x.until > $2))
		ORDER BY coalesce(u.last_login_at, u.provisioned_at), s.student_id LIMIT $6`, q.DueBefore, q.Now, schools, q.AfterBase, q.AfterID, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("cleanup candidate page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []write.CleanupCandidate{}
	for rows.Next() {
		var c write.CleanupCandidate
		var last sql.NullTime
		if err := rows.Scan(&c.StudentID, &c.SchoolID, &c.DisplayName, &last, &c.ProvisionedAt); err != nil {
			return nil, err
		}
		if last.Valid {
			c.LastLoginAt = &last.Time
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// StudentCount implements write.CleanupRepository.
func (CleanupRepo) StudentCount(ctx context.Context) (int, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok org.students under the TenantScope (RLS); deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM org.students WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// SoftDeleteStudent implements write.CleanupRepository.
func (CleanupRepo) SoftDeleteStudent(ctx context.Context, studentID uuid.UUID) (bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok org.students under the TenantScope (RLS); soft-delete of one live row
	res, err := tx.ExecContext(ctx, `UPDATE org.students SET deleted_at = now(), delete_reason = 'inactive_cleanup', updated_at = now(),
		version = version + 1 WHERE student_id = $1 AND deleted_at IS NULL`, studentID)
	if err != nil {
		return false, fmt.Errorf("cleanup student: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	// tenancyguard:ok identity.users is global (G): the user of the student
	if _, err := tx.ExecContext(ctx, `UPDATE identity.users SET deleted_at = now(), delete_reason = 'inactive_cleanup', updated_at = now(),
		version = version + 1 WHERE user_id = $1 AND deleted_at IS NULL`, studentID); err != nil {
		return false, fmt.Errorf("cleanup user: %w", err)
	}
	return true, nil
}

// RestoreStudent implements write.CleanupRepository.
func (CleanupRepo) RestoreStudent(ctx context.Context, studentID uuid.UUID) (*time.Time, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var at time.Time
	// tenancyguard:ok identity.users is global (G): only a user removed by the inactive cleanup
	err = tx.QueryRowContext(ctx, `UPDATE identity.users u SET deleted_at = NULL, delete_reason = NULL, updated_at = now(), version = version + 1
		FROM (SELECT deleted_at FROM identity.users WHERE user_id = $1 AND delete_reason = 'inactive_cleanup' FOR UPDATE) old
		WHERE u.user_id = $1 AND u.delete_reason = 'inactive_cleanup' RETURNING old.deleted_at`, studentID).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("restore user: %w", err)
	}
	// tenancyguard:ok org.students under the TenantScope (RLS): the student removed by the cleanup
	if _, err := tx.ExecContext(ctx, `UPDATE org.students SET deleted_at = NULL, delete_reason = NULL, updated_at = now(), version = version + 1
		WHERE student_id = $1 AND delete_reason = 'inactive_cleanup'`, studentID); err != nil {
		return nil, fmt.Errorf("restore student: %w", err)
	}
	return &at, nil
}

// CleanedStudent implements write.CleanupRepository.
func (CleanupRepo) CleanedStudent(ctx context.Context, studentID uuid.UUID) (*uuid.UUID, *time.Time, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, nil, err
	}
	var tenant uuid.UUID
	var at time.Time
	// tenancyguard:ok platform scope: the tenant of one student removed by the inactive cleanup
	err = tx.QueryRowContext(ctx, `SELECT s.tenant_id, s.deleted_at FROM org.students s
		WHERE s.student_id = $1 AND s.delete_reason = 'inactive_cleanup' AND s.deleted_at IS NOT NULL`, studentID).Scan(&tenant, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("cleaned student: %w", err)
	}
	return &tenant, &at, nil
}

// SisStanding implements write.CleanupRepository.
func (CleanupRepo) SisStanding(ctx context.Context, studentID uuid.UUID) (bool, bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return false, false, err
	}
	var linked, valid bool
	// tenancyguard:ok platform scope: the active SIS link of one user and its live SIS record; deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT true, COALESCE(bool_or(s.record_status = 'active'), false)
		FROM identity.sis_identity_links l
		LEFT JOIN sis.synced_students s ON s.sis_connection_id = l.sis_connection_id AND s.student_external_id = l.student_external_id
			AND s.tenant_id = l.tenant_id AND s.deleted_at IS NULL
		WHERE l.user_id = $1 AND l.status = 'active' AND l.deleted_at IS NULL
		GROUP BY l.link_id`, studentID).Scan(&linked, &valid)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("sis standing: %w", err)
	}
	return linked, valid, nil
}

const runColumns = `run_id, tenant_id, mode, status, cutoff_days, candidates, cleaned, excluded, skipped_active, approved_by, started_at,
	finished_at, version`

func scanRun(row interface{ Scan(...any) error }) (write.CleanupRun, error) {
	var r write.CleanupRun
	var by uuid.NullUUID
	var fin sql.NullTime
	err := row.Scan(&r.ID, &r.TenantID, &r.Mode, &r.Status, &r.CutoffDays, &r.Candidates, &r.Cleaned, &r.Excluded, &r.SkippedActive,
		&by, &r.StartedAt, &fin, &r.Version)
	if by.Valid {
		v := by.UUID
		r.ApprovedBy = &v
	}
	if fin.Valid {
		r.FinishedAt = &fin.Time
	}
	return r, err
}

// CreateRun implements write.CleanupRepository.
func (CleanupRepo) CreateRun(ctx context.Context, r *write.CleanupRun) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform.student_cleanup_runs is written by the job in the platform scope; tenant_id explicit
	_, err = tx.ExecContext(ctx, `INSERT INTO platform.student_cleanup_runs (run_id, tenant_id, mode, status, cutoff_days, candidates, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, r.ID, r.TenantID, r.Mode, r.Status, r.CutoffDays, r.Candidates, r.StartedAt)
	if err != nil {
		return fmt.Errorf("create cleanup run: %w", err)
	}
	r.Version = 1
	return nil
}

// SaveRun implements write.CleanupRepository.
func (CleanupRepo) SaveRun(ctx context.Context, r *write.CleanupRun) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform scope: one run by id and version
	err = tx.QueryRowContext(ctx, `UPDATE platform.student_cleanup_runs SET status = $3, candidates = $4, cleaned = $5, excluded = $6,
			skipped_active = $7, approved_by = $8, approved_at = CASE WHEN $8::uuid IS NOT NULL AND approved_at IS NULL THEN now() ELSE approved_at END,
			finished_at = $9, version = version + 1
		WHERE run_id = $1 AND version = $2 RETURNING version`, r.ID, r.Version, r.Status, r.Candidates, r.Cleaned, r.Excluded, r.SkippedActive,
		r.ApprovedBy, r.FinishedAt).Scan(&r.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return problem.New(problem.VersionMismatch, "")
	}
	if err != nil {
		return fmt.Errorf("save cleanup run: %w", err)
	}
	return nil
}

// Run implements write.CleanupRepository.
func (CleanupRepo) Run(ctx context.Context, id uuid.UUID) (*write.CleanupRun, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform scope: one run by id
	r, err := scanRun(tx.QueryRowContext(ctx, `SELECT `+runColumns+` FROM platform.student_cleanup_runs WHERE run_id = $1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cleanup run: %w", err)
	}
	return &r, nil
}

// ApprovedRun implements write.CleanupRepository.
func (CleanupRepo) ApprovedRun(ctx context.Context, tenant uuid.UUID) (*write.CleanupRun, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform scope: the approved run of one tenant
	r, err := scanRun(tx.QueryRowContext(ctx, `SELECT `+runColumns+` FROM platform.student_cleanup_runs
		WHERE tenant_id = $1 AND status = 'approved' ORDER BY started_at LIMIT 1 FOR UPDATE`, tenant))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("approved run: %w", err)
	}
	return &r, nil
}

// ListRuns implements write.CleanupRepository.
func (CleanupRepo) ListRuns(ctx context.Context, tenants []uuid.UUID, status string, after *write.CleanupRunKey, limit int) ([]write.CleanupRun, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var st *string
	if status != "" {
		st = &status
	}
	var afterAt *time.Time
	afterID := uuid.Nil
	if after != nil {
		afterAt, afterID = &after.StartedAt, after.ID
	}
	// tenancyguard:ok platform scope (System Admin): runs of every tenant or of one, explicit tenant condition
	rows, err := tx.QueryContext(ctx, `SELECT `+runColumns+` FROM platform.student_cleanup_runs
		WHERE (cardinality($1::uuid[]) = 0 OR tenant_id = ANY($1::uuid[])) AND ($2::varchar IS NULL OR status = $2)
			AND ($4::timestamptz IS NULL OR (started_at, run_id) < ($4::timestamptz, $5::uuid))
		ORDER BY started_at DESC, run_id DESC LIMIT $3`, nonNilIDs(tenants), st, limit, afterAt, afterID)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.CleanupRun
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CleanupTenants implements write.CleanupRepository.
func (CleanupRepo) CleanupTenants(ctx context.Context) ([]uuid.UUID, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform.tenants is global (G): the active tenants for the cleanup job
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id FROM platform.tenants WHERE status = 'active' AND deleted_at IS NULL ORDER BY tenant_id`)
	if err != nil {
		return nil, err
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

// AddExclusion implements write.CleanupRepository (false when the student already has a live exclusion).
func (CleanupRepo) AddExclusion(ctx context.Context, e write.CleanupExclusion) (bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform.cleanup_exclusions under the TenantScope (RLS)
	res, err := tx.ExecContext(ctx, `INSERT INTO platform.cleanup_exclusions (exclusion_id, tenant_id, student_id, school_id, until, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (tenant_id, student_id) WHERE deleted_at IS NULL DO NOTHING`,
		e.ID, e.TenantID, e.StudentID, e.SchoolID, e.Until, e.Reason, e.CreatedBy)
	if err != nil {
		return false, fmt.Errorf("add exclusion: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// RemoveExclusion implements write.CleanupRepository (soft-delete of the live exclusion).
func (CleanupRepo) RemoveExclusion(ctx context.Context, studentID, by uuid.UUID) (bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform.cleanup_exclusions under the TenantScope (RLS); soft-delete of the live row
	res, err := tx.ExecContext(ctx, `UPDATE platform.cleanup_exclusions SET deleted_at = now(), deleted_by = $2, delete_reason = 'removed'
		WHERE student_id = $1 AND deleted_at IS NULL`, studentID, by)
	if err != nil {
		return false, fmt.Errorf("remove exclusion: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// StudentSchool implements write.CleanupRepository.
func (CleanupRepo) StudentSchool(ctx context.Context, studentID uuid.UUID) (*uuid.UUID, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var school uuid.UUID
	// tenancyguard:ok org.students under the TenantScope (RLS); deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT school_id FROM org.students WHERE student_id = $1 AND deleted_at IS NULL`, studentID).Scan(&school)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &school, nil
}

func nonNilIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}
