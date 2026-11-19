package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PlatformAdminRepo implements write.PlatformAdminRepository (F045) with explicit SQL in the platform scope
// (identity_platform): platform.tenant_clone_jobs, platform.tenant_default_assignments, identity.user_merges and
// the identity step of a merge. Tenant columns are always explicit conditions; no row is ever removed.
type PlatformAdminRepo struct{}

var _ write.PlatformAdminRepository = PlatformAdminRepo{}

func uuidArray(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func parseUUIDArray(s string) ([]uuid.UUID, error) {
	s = strings.Trim(s, "{}")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]uuid.UUID, 0, len(parts))
	for _, p := range parts {
		id, err := uuid.Parse(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// InactiveTenants implements write.PlatformAdminRepository.
func (PlatformAdminRepo) InactiveTenants(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform.tenants is global (G): which of the requested tenants are live and active
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id FROM platform.tenants
		WHERE tenant_id = ANY($1::uuid[]) AND status = 'active' AND deleted_at IS NULL`, uuidArray(ids))
	if err != nil {
		return nil, fmt.Errorf("active tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	active := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		active[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []uuid.UUID
	for _, id := range ids {
		if !active[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// ActiveTenantIDs implements write.PlatformAdminRepository.
func (PlatformAdminRepo) ActiveTenantIDs(ctx context.Context) ([]uuid.UUID, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform.tenants is global (G): every live, active tenant (apply_to_all_tenants)
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id FROM platform.tenants WHERE status = 'active' AND deleted_at IS NULL ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("active tenants: %w", err)
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

// CreateCloneJob implements write.PlatformAdminRepository.
func (PlatformAdminRepo) CreateCloneJob(ctx context.Context, j write.CloneJob) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform.tenant_clone_jobs is global (G); its tenants are explicit columns
	_, err = tx.ExecContext(ctx, `INSERT INTO platform.tenant_clone_jobs (job_id, source_tenant_id, target_tenant_id, resource_type,
			resource_ids, status, saga_id, requested_by, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::uuid[], 'queued', $6, $7, $8, $9, $9)`,
		j.ID, j.Source, j.Target, j.ResourceType, uuidArray(j.ResourceIDs), j.SagaID, j.RequestedBy, j.IdempotencyKey, j.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return problem.New(problem.Conflict, "a clone job with this Idempotency-Key exists")
	}
	if err != nil {
		return fmt.Errorf("create clone job: %w", err)
	}
	return nil
}

const cloneJobColumns = `job_id, source_tenant_id, target_tenant_id, resource_type, resource_ids::text, status, saga_id, requested_by,
	idempotency_key, created_at`

func scanCloneJob(sc interface{ Scan(...any) error }, extra ...any) (*write.CloneJob, error) {
	var j write.CloneJob
	var ids string
	dest := append([]any{&j.ID, &j.Source, &j.Target, &j.ResourceType, &ids, &j.Status, &j.SagaID, &j.RequestedBy, &j.IdempotencyKey,
		&j.CreatedAt}, extra...)
	if err := sc.Scan(dest...); err != nil {
		return nil, err
	}
	var err error
	if j.ResourceIDs, err = parseUUIDArray(ids); err != nil {
		return nil, err
	}
	return &j, nil
}

// CloneJobBySaga implements write.PlatformAdminRepository.
func (PlatformAdminRepo) CloneJobBySaga(ctx context.Context, sagaID uuid.UUID) (*write.CloneJob, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform.tenant_clone_jobs is global (G): one job by its saga
	j, err := scanCloneJob(tx.QueryRowContext(ctx, `SELECT `+cloneJobColumns+` FROM platform.tenant_clone_jobs WHERE saga_id = $1`, sagaID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

// OpenCloneJobs implements write.PlatformAdminRepository.
func (PlatformAdminRepo) OpenCloneJobs(ctx context.Context, limit int) ([]write.OpenCloneJob, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	out, err := openCloneJobs(ctx, tx, limit)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Steps, err = sagaStepsOf(ctx, tx, out[i].SagaID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func openCloneJobs(ctx context.Context, tx *sql.Tx, limit int) ([]write.OpenCloneJob, error) {
	// tenancyguard:ok platform.tenant_clone_jobs and platform.saga_runs are global (G): open jobs of every tenant
	rows, err := tx.QueryContext(ctx, `SELECT j.job_id, j.source_tenant_id, j.target_tenant_id, j.resource_type, j.resource_ids::text, j.status,
			j.saga_id, j.requested_by, j.idempotency_key, j.created_at, s.status, coalesce(s.last_error, '')
		FROM platform.tenant_clone_jobs j JOIN platform.saga_runs s ON s.saga_id = j.saga_id
		WHERE j.status IN ('queued', 'running') ORDER BY j.created_at LIMIT $1 FOR UPDATE OF j SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("open clone jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.OpenCloneJob
	for rows.Next() {
		var o write.OpenCloneJob
		j, err := scanCloneJob(rows, &o.SagaStatus, &o.SagaError)
		if err != nil {
			return nil, err
		}
		o.CloneJob = *j
		out = append(out, o)
	}
	return out, rows.Err()
}

func sagaStepsOf(ctx context.Context, tx *sql.Tx, sagaID uuid.UUID) ([]write.SagaStep, error) {
	// tenancyguard:ok platform.saga_steps is global (G), keyed by its saga
	rows, err := tx.QueryContext(ctx, `SELECT step_no, service, action, status, attempts, response, coalesce(error, '')
		FROM platform.saga_steps WHERE saga_id = $1 ORDER BY step_no`, sagaID)
	if err != nil {
		return nil, fmt.Errorf("clone job steps: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.SagaStep
	for rows.Next() {
		var s write.SagaStep
		var resp []byte
		if err := rows.Scan(&s.No, &s.Service, &s.Action, &s.Status, &s.Attempts, &resp, &s.Error); err != nil {
			return nil, err
		}
		s.Response = resp
		out = append(out, s)
	}
	return out, rows.Err()
}

// MarkCloneRunning implements write.PlatformAdminRepository.
func (PlatformAdminRepo) MarkCloneRunning(ctx context.Context, jobID uuid.UUID) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform.tenant_clone_jobs is global (G): one job by id
	_, err = tx.ExecContext(ctx, `UPDATE platform.tenant_clone_jobs SET status = 'running', updated_at = now(), version = version + 1
		WHERE job_id = $1 AND status = 'queued'`, jobID)
	if err != nil {
		return fmt.Errorf("clone job running: %w", err)
	}
	return nil
}

// CloneMap implements write.PlatformAdminRepository.
func (PlatformAdminRepo) CloneMap(ctx context.Context, jobID uuid.UUID) ([]platformadmin.ClonePair, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	// tenancyguard:ok platform.tenant_clone_jobs is global (G): one job by id
	if err := tx.QueryRowContext(ctx, `SELECT id_map FROM platform.tenant_clone_jobs WHERE job_id = $1`, jobID).Scan(&raw); err != nil {
		return nil, fmt.Errorf("clone map: %w", err)
	}
	var out []platformadmin.ClonePair
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AppendCloneMap implements write.PlatformAdminRepository.
func (PlatformAdminRepo) AppendCloneMap(ctx context.Context, jobID uuid.UUID, pairs []platformadmin.ClonePair) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	var raw []byte
	// tenancyguard:ok platform.tenant_clone_jobs is global (G): one job by id, locked for the merge of its map
	if err := tx.QueryRowContext(ctx, `SELECT id_map FROM platform.tenant_clone_jobs WHERE job_id = $1 FOR UPDATE`, jobID).Scan(&raw); err != nil {
		return fmt.Errorf("clone map: %w", err)
	}
	var cur []platformadmin.ClonePair
	if err := json.Unmarshal(raw, &cur); err != nil {
		return err
	}
	seen := make(map[string]bool, len(cur))
	for _, p := range cur {
		seen[p.ResourceType+":"+p.SourceID.String()] = true
	}
	for _, p := range pairs { // a replayed step brings the same pairs: added once (SC-004)
		if k := p.ResourceType + ":" + p.SourceID.String(); !seen[k] {
			seen[k] = true
			cur = append(cur, p)
		}
	}
	b, err := json.Marshal(cur)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform.tenant_clone_jobs is global (G): one job by id
	if _, err := tx.ExecContext(ctx, `UPDATE platform.tenant_clone_jobs SET id_map = $2::jsonb, status = CASE WHEN status = 'queued' THEN 'running' ELSE status END,
			updated_at = now(), version = version + 1 WHERE job_id = $1 AND status IN ('queued', 'running')`, jobID, string(b)); err != nil {
		return fmt.Errorf("append clone map: %w", err)
	}
	return nil
}

// FinishCloneJob implements write.PlatformAdminRepository.
func (PlatformAdminRepo) FinishCloneJob(ctx context.Context, jobID uuid.UUID, res platformadmin.CloneResult, at time.Time) (bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return false, err
	}
	stats, err := json.Marshal(res.Stats)
	if err != nil {
		return false, err
	}
	errs, err := json.Marshal(res.Errors)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform.tenant_clone_jobs is global (G): one job by id, final state written once
	r, err := tx.ExecContext(ctx, `UPDATE platform.tenant_clone_jobs SET status = $2, stats = $3::jsonb, errors = $4::jsonb,
			finished_at = $5, updated_at = $5, version = version + 1
		WHERE job_id = $1 AND status IN ('queued', 'running')`, jobID, res.Status, string(stats), string(errs), at)
	if err != nil {
		return false, fmt.Errorf("finish clone job: %w", err)
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// AddDefaultAssignment implements write.PlatformAdminRepository.
func (PlatformAdminRepo) AddDefaultAssignment(ctx context.Context, d write.DefaultAssignment) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform.tenant_default_assignments is global (G), soft-delete partial unique
	_, err = tx.ExecContext(ctx, `INSERT INTO platform.tenant_default_assignments (default_assignment_id, resource_type, resource_id,
			owner_module, created_by) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (resource_type, resource_id) WHERE deleted_at IS NULL DO NOTHING`,
		d.ID, d.ResourceType, d.ResourceID, d.OwnerModule, d.CreatedBy)
	if err != nil {
		return fmt.Errorf("default assignment: %w", err)
	}
	return nil
}

// DefaultAssignments implements write.PlatformAdminRepository.
func (PlatformAdminRepo) DefaultAssignments(ctx context.Context) ([]write.DefaultAssignment, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform.tenant_default_assignments is global (G); live rows only
	rows, err := tx.QueryContext(ctx, `SELECT default_assignment_id, resource_type, resource_id, owner_module, created_by
		FROM platform.tenant_default_assignments WHERE deleted_at IS NULL ORDER BY created_at, default_assignment_id`)
	if err != nil {
		return nil, fmt.Errorf("default assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.DefaultAssignment
	for rows.Next() {
		var d write.DefaultAssignment
		if err := rows.Scan(&d.ID, &d.ResourceType, &d.ResourceID, &d.OwnerModule, &d.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// MergeAccount implements write.PlatformAdminRepository.
func (PlatformAdminRepo) MergeAccount(ctx context.Context, userID, tenantID uuid.UUID) (*platformadmin.MergeAccount, bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, false, err
	}
	var acc platformadmin.MergeAccount
	var anonymized sql.NullTime
	// tenancyguard:ok identity.users and identity.user_merges are global/G: one live account by id
	err = tx.QueryRowContext(ctx, `SELECT u.user_id, u.user_type, u.status, u.anonymized_at,
			EXISTS (SELECT 1 FROM identity.user_merges m WHERE m.from_user_id = u.user_id)
		FROM identity.users u WHERE u.user_id = $1 AND u.deleted_at IS NULL`, userID).
		Scan(&acc.UserID, &acc.UserType, &acc.Status, &anonymized, &acc.MergedAway)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("merge account: %w", err)
	}
	acc.Anonymized = anonymized.Valid
	var member bool
	// tenancyguard:ok explicit tenant condition; the view filters deleted_at of its sources
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM identity.tenant_memberships_v WHERE user_id = $1 AND tenant_id = $2)`,
		userID, tenantID).Scan(&member); err != nil {
		return nil, false, fmt.Errorf("merge membership: %w", err)
	}
	return &acc, member, nil
}

// MergeIssue implements write.PlatformAdminRepository.
func (PlatformAdminRepo) MergeIssue(ctx context.Context, issueID, tenantID uuid.UUID) (*platformadmin.MergeIssue, *uuid.UUID, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, nil, err
	}
	var is platformadmin.MergeIssue
	var school uuid.NullUUID
	var resolution sql.NullString
	// tenancyguard:ok explicit tenant condition and deleted_at IS NULL (platform scope of the System Admin command)
	err = tx.QueryRowContext(ctx, `SELECT issue_type, status, resolution, school_id FROM sis.reconciliation_issues
		WHERE issue_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, issueID, tenantID).Scan(&is.Type, &is.Status, &resolution, &school)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("merge issue: %w", err)
	}
	is.Resolution = resolution.String
	if school.Valid {
		s := school.UUID
		return &is, &s, nil
	}
	return &is, nil, nil
}

const mergeColumns = `merge_id, tenant_id, from_user_id, into_user_id, school_id, reason, merged_by, merged_at, saga_id, reconciliation_issue_id`

func scanMerge(row *sql.Row) (*write.UserMerge, error) {
	var m write.UserMerge
	var school, issue uuid.NullUUID
	err := row.Scan(&m.ID, &m.TenantID, &m.From, &m.Into, &school, &m.Reason, &m.MergedBy, &m.MergedAt, &m.SagaID, &issue)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("user merge: %w", err)
	}
	if school.Valid {
		s := school.UUID
		m.SchoolID = &s
	}
	if issue.Valid {
		i := issue.UUID
		m.IssueID = &i
	}
	return &m, nil
}

// MergeByFrom implements write.PlatformAdminRepository.
func (PlatformAdminRepo) MergeByFrom(ctx context.Context, from uuid.UUID) (*write.UserMerge, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok identity.user_merges is read by its unique source account in the platform scope
	return scanMerge(tx.QueryRowContext(ctx, `SELECT `+mergeColumns+` FROM identity.user_merges WHERE from_user_id = $1`, from))
}

// MergeBySaga implements write.PlatformAdminRepository.
func (PlatformAdminRepo) MergeBySaga(ctx context.Context, sagaID uuid.UUID) (*write.UserMerge, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok identity.user_merges is read by its unique saga in the platform scope (saga job)
	return scanMerge(tx.QueryRowContext(ctx, `SELECT `+mergeColumns+` FROM identity.user_merges WHERE saga_id = $1`, sagaID))
}

// CreateMerge implements write.PlatformAdminRepository.
func (PlatformAdminRepo) CreateMerge(ctx context.Context, m write.UserMerge) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok identity.user_merges carries its tenant_id explicitly (System Admin, target tenant)
	_, err = tx.ExecContext(ctx, `INSERT INTO identity.user_merges (`+mergeColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		m.ID, m.TenantID, m.From, m.Into, m.SchoolID, m.Reason, m.MergedBy, m.MergedAt, m.SagaID, m.IssueID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return problem.New(problem.Conflict, "the source account was already merged")
	}
	if err != nil {
		return fmt.Errorf("create user merge: %w", err)
	}
	return nil
}

// TransferIdentities implements write.PlatformAdminRepository.
func (PlatformAdminRepo) TransferIdentities(ctx context.Context, m write.UserMerge, at time.Time) (out write.MergeTransfer, err error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return out, err
	}
	// External identities whose issuer the target does not hold actively move as they are: a login with them
	// enters the target (F045-AC-33). The others are superseded by the target's identity of that issuer.
	// tenancyguard:ok identity.external_identities is global (G): the identities of one account
	r, err := tx.ExecContext(ctx, `UPDATE identity.external_identities e SET user_id = $2, updated_at = $3, updated_by = $4, version = e.version + 1
		WHERE e.user_id = $1 AND e.status = 'active' AND e.deleted_at IS NULL AND NOT EXISTS (
			SELECT 1 FROM identity.external_identities t WHERE t.user_id = $2 AND t.issuer = e.issuer AND t.status = 'active' AND t.deleted_at IS NULL)`,
		m.From, m.Into, at, m.MergedBy)
	if err != nil {
		return out, fmt.Errorf("move identities: %w", err)
	}
	n, _ := r.RowsAffected()
	out.Identities = int(n)
	// tenancyguard:ok identity.external_identities is global (G): the remaining identities of one account
	r, err = tx.ExecContext(ctx, `UPDATE identity.external_identities e SET user_id = $2, status = 'superseded', superseded_at = $3,
			superseded_by = (SELECT t.external_identity_id FROM identity.external_identities t WHERE t.user_id = $2 AND t.issuer = e.issuer
				AND t.status = 'active' AND t.deleted_at IS NULL LIMIT 1),
			updated_at = $3, updated_by = $4, version = e.version + 1
		WHERE e.user_id = $1 AND e.deleted_at IS NULL`, m.From, m.Into, at, m.MergedBy)
	if err != nil {
		return out, fmt.Errorf("supersede identities: %w", err)
	}
	n, _ = r.RowsAffected()
	out.IdentitiesSuperseded = int(n)
	// SIS links: the target keeps its active link; the source's link moves, superseded when the target has one.
	// tenancyguard:ok explicit tenant condition (target tenant of the merge), deleted_at IS NULL
	r, err = tx.ExecContext(ctx, `UPDATE identity.sis_identity_links l SET user_id = $2, updated_at = $3, updated_by = $4, version = l.version + 1
		WHERE l.user_id = $1 AND l.tenant_id = $5 AND l.status = 'active' AND l.deleted_at IS NULL AND NOT EXISTS (
			SELECT 1 FROM identity.sis_identity_links t WHERE t.user_id = $2 AND t.status = 'active' AND t.deleted_at IS NULL)`,
		m.From, m.Into, at, m.MergedBy, m.TenantID)
	if err != nil {
		return out, fmt.Errorf("move sis links: %w", err)
	}
	n, _ = r.RowsAffected()
	out.SisLinks = int(n)
	// tenancyguard:ok explicit tenant condition (target tenant of the merge), deleted_at IS NULL
	r, err = tx.ExecContext(ctx, `UPDATE identity.sis_identity_links l SET user_id = $2, status = 'superseded', superseded_at = $3,
			superseded_by = (SELECT t.link_id FROM identity.sis_identity_links t WHERE t.user_id = $2 AND t.status = 'active' AND t.deleted_at IS NULL LIMIT 1),
			updated_at = $3, updated_by = $4, version = l.version + 1
		WHERE l.user_id = $1 AND l.tenant_id = $5 AND l.deleted_at IS NULL AND l.status = 'active'`, m.From, m.Into, at, m.MergedBy, m.TenantID)
	if err != nil {
		return out, fmt.Errorf("supersede sis links: %w", err)
	}
	n, _ = r.RowsAffected()
	out.SisLinksSuperseded = int(n)
	// The source account is soft-deleted (R21), never removed.
	// tenancyguard:ok identity.users is global (G): one account by id
	if _, err = tx.ExecContext(ctx, `UPDATE identity.users SET deleted_at = $2, deleted_by = $3, delete_reason = 'merged', updated_at = $2,
			updated_by = $3, version = version + 1 WHERE user_id = $1 AND deleted_at IS NULL`, m.From, at, m.MergedBy); err != nil {
		return out, fmt.Errorf("soft-delete merged account: %w", err)
	}
	if m.IssueID != nil {
		note := "merge " + m.ID.String()
		// tenancyguard:ok explicit tenant condition and deleted_at IS NULL: the issue that proposed the merge
		if _, err = tx.ExecContext(ctx, `UPDATE sis.reconciliation_issues SET status = 'resolved', resolution = 'merged', resolution_note = $3,
				resolved_by = $4, resolved_at = $5, updated_at = $5, updated_by = $4, version = version + 1
			WHERE issue_id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = 'open'`, *m.IssueID, m.TenantID, note, m.MergedBy, at); err != nil {
			return out, fmt.Errorf("close merge issue: %w", err)
		}
	}
	return out, nil
}
