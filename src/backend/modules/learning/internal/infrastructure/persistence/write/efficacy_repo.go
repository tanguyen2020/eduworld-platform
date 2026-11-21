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
)

// EfficacyRepo implements write.EfficacyStore with explicit SQL (F035): the G tables (efficacy_studies, efficacy_waves,
// efficacy_reports) have no tenant; the T tables filter the tenant given as a parameter (System Admin commands and the
// jobs run on learning_platform under tenancy.Bypass(efficacy_study), the tenant commands under RLS) and deleted_at.
type EfficacyRepo struct{}

var _ write.EfficacyStore = EfficacyRepo{}

func jsonUUIDs(b []byte) []uuid.UUID {
	var out []uuid.UUID
	_ = json.Unmarshal(b, &out)
	return out
}

func jsonStrings(b []byte) []string {
	var out []string
	_ = json.Unmarshal(b, &out)
	return out
}

func nullString(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

func nullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// ---------- agreements (T)

const agreementCols = `agreement_id, tenant_id, agreement_ref, to_jsonb(covers), agreed_on, status, revoked_at, revoked_by, revoke_reason,
	recorded_by, version, created_at`

func scanAgreement(row interface{ Scan(...any) error }) (write.AgreementRow, error) {
	var a write.AgreementRow
	var covers []byte
	var revokedAt sql.NullTime
	var revokedBy uuid.NullUUID
	var reason sql.NullString
	err := row.Scan(&a.ID, &a.TenantID, &a.Ref, &covers, &a.AgreedOn, &a.Status, &revokedAt, &revokedBy, &reason, &a.RecordedBy, &a.Version,
		&a.CreatedAt)
	a.Covers, a.RevokedAt, a.RevokedBy, a.RevokeReason = jsonStrings(covers), nullTime(revokedAt), nullUUID(revokedBy), nullString(reason)
	return a, err
}

func oneAgreement(ctx context.Context, q string, args ...any) (*write.AgreementRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	a, err := scanAgreement(tx.QueryRowContext(ctx, q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("research agreement: %w", err)
	}
	return &a, nil
}

// InsertAgreement implements write.EfficacyStore.
func (EfficacyRepo) InsertAgreement(ctx context.Context, a write.AgreementRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.research_agreements (agreement_id, tenant_id, agreement_ref, covers, agreed_on, status,
		recorded_by, created_at, updated_at) VALUES ($1,$2,$3,$4::varchar(64)[],$5,$6,$7,$8,$8)`,
		a.ID, a.TenantID, a.Ref, textArray(a.Covers), a.AgreedOn, a.Status, a.RecordedBy, a.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert research agreement: %w", err)
	}
	return nil
}

// Agreement implements write.EfficacyStore.
func (EfficacyRepo) Agreement(ctx context.Context, tenant, id uuid.UUID, lock bool) (*write.AgreementRow, error) {
	q := `SELECT ` + agreementCols + ` FROM learning.research_agreements WHERE agreement_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	return oneAgreement(ctx, q, id, tenant)
}

// AgreementByRef implements write.EfficacyStore.
func (EfficacyRepo) AgreementByRef(ctx context.Context, tenant uuid.UUID, ref string) (*write.AgreementRow, error) {
	return oneAgreement(ctx, `SELECT `+agreementCols+` FROM learning.research_agreements WHERE tenant_id = $1 AND agreement_ref = $2
		AND deleted_at IS NULL`, tenant, ref)
}

// Agreements implements write.EfficacyStore.
func (EfficacyRepo) Agreements(ctx context.Context, tenant uuid.UUID) ([]write.AgreementRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+agreementCols+` FROM learning.research_agreements WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at, agreement_id`, tenant)
	if err != nil {
		return nil, fmt.Errorf("research agreements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AgreementRow
	for rows.Next() {
		a, err := scanAgreement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAgreement implements write.EfficacyStore.
func (EfficacyRepo) UpdateAgreement(ctx context.Context, a write.AgreementRow, prev int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	return affected(tx.ExecContext(ctx, `UPDATE learning.research_agreements SET status = $4, revoked_at = $5, revoked_by = $6, revoke_reason = $7,
		version = version + 1, updated_at = now() WHERE agreement_id = $1 AND tenant_id = $2 AND version = $3 AND deleted_at IS NULL`,
		a.ID, a.TenantID, prev, a.Status, a.RevokedAt, a.RevokedBy, a.RevokeReason))
}

// TenantIsQA implements write.EfficacyStore.
func (EfficacyRepo) TenantIsQA(ctx context.Context, tenant uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var qa bool
	// tenancyguard:ok projection ref.tenants (registry: tenant_id is its key, no deleted_at)
	err = tx.QueryRowContext(ctx, `SELECT is_qa FROM ref.tenants WHERE tenant_id = $1`, tenant).Scan(&qa)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return qa, err
}

// ---------- studies (G)

const studyCols = `study_id, code, title, subject_id, grade_level, to_jsonb(objective_ids), form_a_blueprint_version_id,
	form_b_blueprint_version_id, min_intervention_weeks, status, created_by, activated_at, completed_at, idempotency_key, version, created_at`

func scanStudy(row interface{ Scan(...any) error }) (write.StudyRow, error) {
	var s write.StudyRow
	var objectives []byte
	var activated, completed sql.NullTime
	err := row.Scan(&s.ID, &s.Code, &s.Title, &s.SubjectID, &s.GradeLevel, &objectives, &s.FormA, &s.FormB, &s.MinWeeks, &s.Status, &s.CreatedBy,
		&activated, &completed, &s.IdempotencyKey, &s.Version, &s.CreatedAt)
	s.ObjectiveIDs, s.ActivatedAt, s.CompletedAt = jsonUUIDs(objectives), nullTime(activated), nullTime(completed)
	return s, err
}

func oneStudy(ctx context.Context, q string, args ...any) (*write.StudyRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	s, err := scanStudy(tx.QueryRowContext(ctx, q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("efficacy study: %w", err)
	}
	return &s, nil
}

// InsertStudy implements write.EfficacyStore.
func (EfficacyRepo) InsertStudy(ctx context.Context, s write.StudyRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.efficacy_studies (study_id, code, title, subject_id, grade_level, objective_ids,
		form_a_blueprint_version_id, form_b_blueprint_version_id, min_intervention_weeks, status, created_by, idempotency_key, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6::uuid[],$7,$8,$9,$10,$11,$12,$13,$13)`,
		s.ID, s.Code, s.Title, s.SubjectID, s.GradeLevel, uuidArray(s.ObjectiveIDs), s.FormA, s.FormB, s.MinWeeks, s.Status, s.CreatedBy,
		s.IdempotencyKey, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert efficacy study: %w", err)
	}
	return nil
}

// Study implements write.EfficacyStore.
func (EfficacyRepo) Study(ctx context.Context, id uuid.UUID, lock bool) (*write.StudyRow, error) {
	q := `SELECT ` + studyCols + ` FROM learning.efficacy_studies WHERE study_id = $1 AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	// tenancyguard:ok platform table learning.efficacy_studies (G, no tenant_id), deleted_at IS NULL
	return oneStudy(ctx, q, id)
}

// StudyByKey implements write.EfficacyStore.
func (EfficacyRepo) StudyByKey(ctx context.Context, createdBy uuid.UUID, key string) (*write.StudyRow, error) {
	// tenancyguard:ok platform table learning.efficacy_studies (G, no tenant_id), deleted_at IS NULL
	return oneStudy(ctx, `SELECT `+studyCols+` FROM learning.efficacy_studies WHERE created_by = $1 AND idempotency_key = $2
		AND deleted_at IS NULL`, createdBy, key)
}

// StudyCodeTaken implements write.EfficacyStore.
func (EfficacyRepo) StudyCodeTaken(ctx context.Context, code string, except uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var n int
	// tenancyguard:ok platform table learning.efficacy_studies (G, no tenant_id), deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM learning.efficacy_studies WHERE code = $1 AND study_id <> $2 AND deleted_at IS NULL`,
		code, except).Scan(&n)
	return n > 0, err
}

// UpdateStudy implements write.EfficacyStore.
func (EfficacyRepo) UpdateStudy(ctx context.Context, s write.StudyRow, prev int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform table learning.efficacy_studies (G, no tenant_id), deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE learning.efficacy_studies SET title = $3, objective_ids = $4::uuid[],
		form_a_blueprint_version_id = $5, form_b_blueprint_version_id = $6, min_intervention_weeks = $7, status = $8, activated_at = $9,
		completed_at = $10, version = version + 1, updated_at = now() WHERE study_id = $1 AND version = $2 AND deleted_at IS NULL`,
		s.ID, prev, s.Title, uuidArray(s.ObjectiveIDs), s.FormA, s.FormB, s.MinWeeks, s.Status, s.ActivatedAt, s.CompletedAt))
}

// ---------- cohorts (cluster school × grade of the study, ADR-0095 D3)

// School implements write.EfficacyStore: a school of the tenant given (ref.schools) with its active, not deleted students
// of the grade (ref.students).
func (EfficacyRepo) School(ctx context.Context, tenant, schoolID uuid.UUID, grade int) (*write.EfficacySchoolRef, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	c := write.EfficacySchoolRef{SchoolID: schoolID}
	// tenancyguard:ok projections ref.schools/ref.students of the tenant given (students: deleted_at IS NULL; schools have none)
	err = tx.QueryRowContext(ctx, `SELECT sc.tenant_id, (SELECT count(*) FROM ref.students st WHERE st.tenant_id = sc.tenant_id
			AND st.school_id = sc.school_id AND st.grade_level = $3 AND st.status = 'active' AND st.deleted_at IS NULL)
		FROM ref.schools sc WHERE sc.school_id = $1 AND sc.tenant_id = $2`, schoolID, tenant, grade).Scan(&c.TenantID, &c.Students)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("school: %w", err)
	}
	return &c, nil
}

const studyCohortCols = `study_id, school_id, tenant_id, grade_level, pair_no, arm, assignment_method, form_order, status, conflict,
	withdrawn_at, withdrawal_id, added_by, deleted_at IS NOT NULL`

func scanStudyCohort(row interface{ Scan(...any) error }) (write.StudyCohortRow, error) {
	var c write.StudyCohortRow
	var pair sql.NullInt64
	var arm, order sql.NullString
	var withdrawn sql.NullTime
	var wid uuid.NullUUID
	err := row.Scan(&c.StudyID, &c.SchoolID, &c.TenantID, &c.GradeLevel, &pair, &arm, &c.Method, &order, &c.Status, &c.Conflict, &withdrawn,
		&wid, &c.AddedBy, &c.Deleted)
	c.PairNo, c.Arm, c.FormOrder, c.WithdrawnAt, c.WithdrawalID = nullInt(pair), nullString(arm), nullString(order), nullTime(withdrawn), nullUUID(wid)
	return c, err
}

// StudyCohort implements write.EfficacyStore.
func (EfficacyRepo) StudyCohort(ctx context.Context, studyID, schoolID uuid.UUID, withDeleted bool) (*write.StudyCohortRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + studyCohortCols + ` FROM learning.efficacy_study_cohorts WHERE study_id = $1 AND school_id = $2`
	if !withDeleted {
		q += ` AND deleted_at IS NULL`
	}
	// tenancyguard:ok the cohort key (study, school) names one tenant (RLS on learning_app; learning_platform under bypass); deleted_at in q
	c, err := scanStudyCohort(tx.QueryRowContext(ctx, q+` FOR UPDATE`, studyID, schoolID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("study cohort: %w", err)
	}
	return &c, nil
}

// StudyCohorts implements write.EfficacyStore.
func (EfficacyRepo) StudyCohorts(ctx context.Context, studyID uuid.UUID) ([]write.StudyCohortRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of the study: learning_platform under bypass efficacy_study (audited); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT `+studyCohortCols+` FROM learning.efficacy_study_cohorts WHERE study_id = $1 AND deleted_at IS NULL
		ORDER BY pair_no NULLS LAST, school_id`, studyID)
	if err != nil {
		return nil, fmt.Errorf("study cohorts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.StudyCohortRow
	for rows.Next() {
		c, err := scanStudyCohort(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CohortInOtherStudy implements write.EfficacyStore.
func (EfficacyRepo) CohortInOtherStudy(ctx context.Context, tenant, schoolID uuid.UUID, grade int, studyID uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var n int
	// tenancyguard:ok cohorts of the tenant given, deleted_at IS NULL on both tables
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM learning.efficacy_study_cohorts c JOIN learning.efficacy_studies s ON s.study_id = c.study_id
		AND s.deleted_at IS NULL AND s.status IN ('draft','active')
		WHERE c.tenant_id = $1 AND c.school_id = $2 AND c.grade_level = $3 AND c.study_id <> $4 AND c.status = 'enrolled'
			AND c.deleted_at IS NULL`, tenant, schoolID, grade, studyID).Scan(&n)
	return n > 0, err
}

// UpsertStudyCohort implements write.EfficacyStore: a new row, or the revival of a removed one (conflict cleared).
func (EfficacyRepo) UpsertStudyCohort(ctx context.Context, c write.StudyCohortRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy); the revival is limited to the same tenant and a deleted row
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.efficacy_study_cohorts (study_id, school_id, tenant_id, grade_level, pair_no,
		arm, assignment_method, form_order, status, added_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (study_id, school_id) DO UPDATE SET pair_no = EXCLUDED.pair_no, arm = EXCLUDED.arm,
			assignment_method = EXCLUDED.assignment_method, form_order = EXCLUDED.form_order, status = EXCLUDED.status, added_by = EXCLUDED.added_by,
			conflict = false, withdrawn_at = NULL, withdrawal_id = NULL, deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
			version = learning.efficacy_study_cohorts.version + 1, updated_at = now()
		WHERE learning.efficacy_study_cohorts.tenant_id = EXCLUDED.tenant_id AND learning.efficacy_study_cohorts.deleted_at IS NOT NULL`,
		c.StudyID, c.SchoolID, c.TenantID, c.GradeLevel, c.PairNo, c.Arm, c.Method, c.FormOrder, c.Status, c.AddedBy)
	if err != nil {
		return fmt.Errorf("upsert study cohort: %w", err)
	}
	return nil
}

// UpdateStudyCohort implements write.EfficacyStore.
func (EfficacyRepo) UpdateStudyCohort(ctx context.Context, c write.StudyCohortRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok one cohort of the tenant given, deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE learning.efficacy_study_cohorts SET pair_no = $4, arm = $5, assignment_method = $6, form_order = $7,
		status = $8, withdrawn_at = $9, withdrawal_id = $10, version = version + 1, updated_at = now()
		WHERE study_id = $1 AND school_id = $2 AND tenant_id = $3 AND deleted_at IS NULL`,
		c.StudyID, c.SchoolID, c.TenantID, c.PairNo, c.Arm, c.Method, c.FormOrder, c.Status, c.WithdrawnAt, c.WithdrawalID)
	if err != nil {
		return fmt.Errorf("update study cohort: %w", err)
	}
	return nil
}

// RemoveStudyCohort implements write.EfficacyStore (soft-delete, ADR-0028).
func (EfficacyRepo) RemoveStudyCohort(ctx context.Context, c write.StudyCohortRow, by uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok soft-delete of one cohort of the tenant given (deleted_at IS NULL)
	_, err = tx.ExecContext(ctx, `UPDATE learning.efficacy_study_cohorts SET deleted_at = now(), deleted_by = $4, delete_reason = 'removed',
		version = version + 1, updated_at = now() WHERE study_id = $1 AND school_id = $2 AND tenant_id = $3 AND deleted_at IS NULL`,
		c.StudyID, c.SchoolID, c.TenantID, by)
	if err != nil {
		return fmt.Errorf("remove study cohort: %w", err)
	}
	return nil
}

// CohortStudents implements write.EfficacyStore: the active, not deleted students of a grade of a school.
func (EfficacyRepo) CohortStudents(ctx context.Context, tenant, schoolID uuid.UUID, grade int) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.students of the tenant given, deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT student_id FROM ref.students WHERE tenant_id = $1 AND school_id = $2 AND grade_level = $3
		AND status = 'active' AND deleted_at IS NULL ORDER BY student_id`, tenant, schoolID, grade)
	if err != nil {
		return nil, fmt.Errorf("students of the cohort: %w", err)
	}
	return scanIDs(rows)
}

// ---------- waves (G) and wave sessions (T)

const waveCols = `wave_id, study_id, kind, form, window_start, window_end, duration_sec, status, opened_at, closed_at, created_by, idempotency_key,
	version, created_at`

func scanWave(row interface{ Scan(...any) error }) (write.WaveRow, error) {
	var w write.WaveRow
	var form sql.NullString
	var opened, closed sql.NullTime
	err := row.Scan(&w.ID, &w.StudyID, &w.Kind, &form, &w.WindowStart, &w.WindowEnd, &w.DurationSec, &w.Status, &opened, &closed, &w.CreatedBy,
		&w.IdempotencyKey, &w.Version, &w.CreatedAt)
	w.Form, w.OpenedAt, w.ClosedAt = nullString(form), nullTime(opened), nullTime(closed)
	w.WindowStart, w.WindowEnd = w.WindowStart.UTC(), w.WindowEnd.UTC()
	return w, err
}

func waves(ctx context.Context, q string, args ...any) ([]write.WaveRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("efficacy waves: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.WaveRow
	for rows.Next() {
		w, err := scanWave(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// InsertWave implements write.EfficacyStore.
func (EfficacyRepo) InsertWave(ctx context.Context, w write.WaveRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.efficacy_waves (wave_id, study_id, kind, form, window_start, window_end, duration_sec,
		status, created_by, idempotency_key, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)`,
		w.ID, w.StudyID, w.Kind, w.Form, w.WindowStart, w.WindowEnd, w.DurationSec, w.Status, w.CreatedBy, w.IdempotencyKey, w.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert efficacy wave: %w", err)
	}
	return nil
}

// Wave implements write.EfficacyStore.
func (EfficacyRepo) Wave(ctx context.Context, id uuid.UUID, lock bool) (*write.WaveRow, error) {
	q := `SELECT ` + waveCols + ` FROM learning.efficacy_waves WHERE wave_id = $1 AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	// tenancyguard:ok platform table learning.efficacy_waves (G, no tenant_id), deleted_at IS NULL
	ws, err := waves(ctx, q, id)
	if err != nil || len(ws) == 0 {
		return nil, err
	}
	return &ws[0], nil
}

// WaveByKey implements write.EfficacyStore.
func (EfficacyRepo) WaveByKey(ctx context.Context, createdBy uuid.UUID, key string) (*write.WaveRow, error) {
	// tenancyguard:ok platform table learning.efficacy_waves (G, no tenant_id), deleted_at IS NULL
	ws, err := waves(ctx, `SELECT `+waveCols+` FROM learning.efficacy_waves WHERE created_by = $1 AND idempotency_key = $2 AND deleted_at IS NULL`,
		createdBy, key)
	if err != nil || len(ws) == 0 {
		return nil, err
	}
	return &ws[0], nil
}

// Waves implements write.EfficacyStore.
func (EfficacyRepo) Waves(ctx context.Context, studyID uuid.UUID) ([]write.WaveRow, error) {
	// tenancyguard:ok platform table learning.efficacy_waves (G, no tenant_id), deleted_at IS NULL
	return waves(ctx, `SELECT `+waveCols+` FROM learning.efficacy_waves WHERE study_id = $1 AND deleted_at IS NULL ORDER BY window_start, wave_id`,
		studyID)
}

// UpdateWave implements write.EfficacyStore.
func (EfficacyRepo) UpdateWave(ctx context.Context, w write.WaveRow, prev int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform table learning.efficacy_waves (G, no tenant_id), deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE learning.efficacy_waves SET status = $3, opened_at = $4, closed_at = $5, version = version + 1,
		updated_at = now() WHERE wave_id = $1 AND version = $2 AND deleted_at IS NULL`, w.ID, prev, w.Status, w.OpenedAt, w.ClosedAt))
}

// DueWaves implements write.EfficacyStore.
func (EfficacyRepo) DueWaves(ctx context.Context, now time.Time, limit int) ([]write.WaveRow, error) {
	// tenancyguard:ok platform table learning.efficacy_waves (G, no tenant_id), deleted_at IS NULL
	return waves(ctx, `SELECT `+waveCols+` FROM learning.efficacy_waves WHERE deleted_at IS NULL
		AND ((status = 'scheduled' AND window_start <= $1) OR (status = 'open' AND window_end <= $1))
		ORDER BY window_start, wave_id LIMIT $2`, now, limit)
}

// InsertWaveSession implements write.EfficacyStore.
func (EfficacyRepo) InsertWaveSession(ctx context.Context, s write.WaveSessionRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.efficacy_wave_cohort_sessions (wave_id, school_id, tenant_id, grade_level, exam_session_id, form,
		status) VALUES ($1,$2,$3,$4,$5,$6,$7)`, s.WaveID, s.SchoolID, s.TenantID, s.GradeLevel, s.ExamSessionID, s.Form, s.Status)
	if err != nil {
		return fmt.Errorf("insert wave session: %w", err)
	}
	return nil
}

// WaveSessions implements write.EfficacyStore.
func (EfficacyRepo) WaveSessions(ctx context.Context, waveID uuid.UUID) ([]write.WaveSessionRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of the wave: learning_platform under bypass efficacy_study (audited), RLS otherwise; deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT wave_id, school_id, tenant_id, grade_level, exam_session_id, form, status
		FROM learning.efficacy_wave_cohort_sessions WHERE wave_id = $1 AND deleted_at IS NULL ORDER BY tenant_id, school_id`, waveID)
	if err != nil {
		return nil, fmt.Errorf("wave sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.WaveSessionRow
	for rows.Next() {
		var s write.WaveSessionRow
		if err := rows.Scan(&s.WaveID, &s.SchoolID, &s.TenantID, &s.GradeLevel, &s.ExamSessionID, &s.Form, &s.Status); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CancelWaveSession implements write.EfficacyStore.
func (EfficacyRepo) CancelWaveSession(ctx context.Context, s write.WaveSessionRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE learning.efficacy_wave_cohort_sessions SET status = 'cancelled', version = version + 1, updated_at = now()
		WHERE wave_id = $1 AND school_id = $2 AND tenant_id = $3 AND deleted_at IS NULL`, s.WaveID, s.SchoolID, s.TenantID)
	if err != nil {
		return fmt.Errorf("cancel wave session: %w", err)
	}
	return nil
}

// ---------- participants (T)

const participantCols = `study_id, student_id, tenant_id, school_id, arm, form_order, status, exclusion_reason, enrolled_at, withdrawn_at`

func participants(ctx context.Context, q string, args ...any) ([]write.ParticipantRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("efficacy participants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ParticipantRow
	for rows.Next() {
		var p write.ParticipantRow
		var reason sql.NullString
		var withdrawn sql.NullTime
		if err := rows.Scan(&p.StudyID, &p.StudentID, &p.TenantID, &p.SchoolID, &p.Arm, &p.FormOrder, &p.Status, &reason, &p.EnrolledAt,
			&withdrawn); err != nil {
			return nil, err
		}
		p.ExclusionReason, p.WithdrawnAt = nullString(reason), nullTime(withdrawn)
		out = append(out, p)
	}
	return out, rows.Err()
}

// InsertParticipants implements write.EfficacyStore: a student already a participant of the study is kept as he is.
func (EfficacyRepo) InsertParticipants(ctx context.Context, ps []write.ParticipantRow) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range ps {
		res, err := tx.ExecContext(ctx, `INSERT INTO learning.efficacy_participants (study_id, student_id, tenant_id, school_id, arm,
			form_order, status, enrolled_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (study_id, student_id) DO NOTHING`,
			p.StudyID, p.StudentID, p.TenantID, p.SchoolID, p.Arm, p.FormOrder, p.Status, p.EnrolledAt)
		if err != nil {
			return n, fmt.Errorf("insert participant: %w", err)
		}
		if k, _ := res.RowsAffected(); k > 0 {
			n++
		}
	}
	return n, nil
}

// Participants implements write.EfficacyStore.
func (EfficacyRepo) Participants(ctx context.Context, studyID uuid.UUID, schoolID *uuid.UUID, status string) ([]write.ParticipantRow, error) {
	// tenancyguard:ok every tenant of the study: learning_platform under bypass efficacy_study (audited), RLS otherwise; deleted_at IS NULL
	return participants(ctx, `SELECT `+participantCols+` FROM learning.efficacy_participants WHERE study_id = $1
		AND ($2::uuid IS NULL OR school_id = $2) AND ($3 = '' OR status = $3) AND deleted_at IS NULL ORDER BY tenant_id, student_id`,
		studyID, schoolID, status)
}

// Withdraw implements write.EfficacyStore.
func (EfficacyRepo) Withdraw(ctx context.Context, s write.WithdrawScope, at time.Time, withdrawalID uuid.UUID) ([]write.ParticipantRow, error) {
	students := ""
	if len(s.StudentIDs) > 0 {
		students = uuidArray(s.StudentIDs)
	}
	return participants(ctx, `UPDATE learning.efficacy_participants p SET status = 'withdrawn', exclusion_reason = 'withdrawn', withdrawn_at = $5,
		withdrawal_id = $6, version = p.version + 1, updated_at = now()
		WHERE p.tenant_id = $1 AND p.status = 'active' AND p.deleted_at IS NULL
			AND ($2::uuid IS NULL OR p.study_id = $2) AND ($3::uuid IS NULL OR p.school_id = $3)
			AND ($4 = '' OR p.student_id = ANY($4::uuid[]))
			AND EXISTS (SELECT 1 FROM learning.efficacy_studies st WHERE st.study_id = p.study_id AND st.status IN ('draft','active')
				AND st.deleted_at IS NULL)
		RETURNING `+participantCols, s.TenantID, s.StudyID, s.SchoolID, students, at, withdrawalID)
}

// MarkCohortsWithdrawn implements write.EfficacyStore.
func (EfficacyRepo) MarkCohortsWithdrawn(ctx context.Context, s write.WithdrawScope, at time.Time, withdrawalID uuid.UUID) ([]write.StudyCohortRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `UPDATE learning.efficacy_study_cohorts c SET status = 'withdrawn', withdrawn_at = $4, withdrawal_id = $5,
		version = c.version + 1, updated_at = now()
		WHERE c.tenant_id = $1 AND c.status = 'enrolled' AND c.deleted_at IS NULL AND ($2::uuid IS NULL OR c.study_id = $2)
			AND ($3::uuid IS NULL OR c.school_id = $3)
			AND EXISTS (SELECT 1 FROM learning.efficacy_studies st WHERE st.study_id = c.study_id AND st.status IN ('draft','active')
				AND st.deleted_at IS NULL)
		RETURNING `+studyCohortCols, s.TenantID, s.StudyID, s.SchoolID, at, withdrawalID)
	if err != nil {
		return nil, fmt.Errorf("withdraw cohorts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.StudyCohortRow
	for rows.Next() {
		c, err := scanStudyCohort(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ExcludeStudent implements write.EfficacyStore.
func (EfficacyRepo) ExcludeStudent(ctx context.Context, tenant, studentID uuid.UUID, reason string, at time.Time) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE learning.efficacy_participants SET status = 'excluded', exclusion_reason = $3, withdrawn_at = $4,
		version = version + 1, updated_at = now() WHERE tenant_id = $1 AND student_id = $2 AND status = 'active' AND deleted_at IS NULL`,
		tenant, studentID, reason, at)
	if err != nil {
		return 0, fmt.Errorf("exclude participant: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ---------- withdrawals (T, append-only)

// InsertWithdrawal implements write.EfficacyStore.
func (EfficacyRepo) InsertWithdrawal(ctx context.Context, w write.WithdrawalRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.efficacy_withdrawals (withdrawal_id, tenant_id, study_id, scope, school_id, grade_level,
		student_ids, reason, school_request_ref, affected_count, cancelled_session_count, requested_by, idempotency_key, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7::uuid[],$8,$9,$10,$11,$12,$13,$14)`,
		w.ID, w.TenantID, w.StudyID, w.Scope, w.SchoolID, w.GradeLevel, uuidArray(w.StudentIDs), w.Reason, w.SchoolRequestRef, w.Affected,
		w.CancelledSessions, w.RequestedBy, w.IdempotencyKey, w.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert withdrawal: %w", err)
	}
	return nil
}

// WithdrawalByKey implements write.EfficacyStore.
func (EfficacyRepo) WithdrawalByKey(ctx context.Context, requestedBy uuid.UUID, key string) (*write.WithdrawalRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var w write.WithdrawalRow
	var study, school uuid.NullUUID
	var grade sql.NullInt64
	var students []byte
	var ref sql.NullString
	// tenancyguard:ok append-only learning.efficacy_withdrawals (no deleted_at); the requester's key, tenant by RLS or bypass
	err = tx.QueryRowContext(ctx, `SELECT withdrawal_id, tenant_id, study_id, scope, school_id, grade_level, to_jsonb(student_ids), reason,
		school_request_ref, affected_count, cancelled_session_count, requested_by, idempotency_key, created_at
		FROM learning.efficacy_withdrawals WHERE requested_by = $1 AND idempotency_key = $2`, requestedBy, key).
		Scan(&w.ID, &w.TenantID, &study, &w.Scope, &school, &grade, &students, &w.Reason, &ref, &w.Affected, &w.CancelledSessions, &w.RequestedBy,
			&w.IdempotencyKey, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("withdrawal: %w", err)
	}
	w.StudyID, w.SchoolID, w.GradeLevel, w.StudentIDs, w.SchoolRequestRef = nullUUID(study), nullUUID(school), nullInt(grade), jsonUUIDs(students),
		nullString(ref)
	return &w, nil
}

// ---------- mastery snapshots (T)

// InsertSnapshots implements write.EfficacyStore: one snapshot per participant × objective (a second post wave keeps the
// first one).
func (EfficacyRepo) InsertSnapshots(ctx context.Context, rs []write.SnapshotRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	for _, r := range rs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO learning.efficacy_mastery_snapshots (study_id, student_id, objective_id, tenant_id, m_eff,
			level, model_version, evidence_count, taken_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`,
			r.StudyID, r.StudentID, r.ObjectiveID, r.TenantID, r.MEff, r.Level, r.ModelVersion, r.EvidenceCount, r.TakenAt); err != nil {
			return fmt.Errorf("insert mastery snapshot: %w", err)
		}
	}
	return nil
}

// SoftDeleteSnapshots implements write.EfficacyStore.
func (EfficacyRepo) SoftDeleteSnapshots(ctx context.Context, tenant uuid.UUID, studyID *uuid.UUID, students []uuid.UUID, reason string,
	by *uuid.UUID) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE learning.efficacy_mastery_snapshots SET deleted_at = now(), deleted_by = $4, delete_reason = $5,
		updated_at = now() WHERE tenant_id = $1 AND ($2::uuid IS NULL OR study_id = $2) AND student_id = ANY($3::uuid[]) AND deleted_at IS NULL`,
		tenant, studyID, uuidArray(students), by, reason)
	if err != nil {
		return 0, fmt.Errorf("soft-delete snapshots: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ---------- reports (G)

const efficacyReportCols = `report_id, study_id, status, COALESCE(params->'excluded_question_version_ids', '[]'::jsonb), results, inputs_hash,
	descriptive_only, error, requested_by, idempotency_key, started_at, finished_at, created_at`

func scanEfficacyReport(row interface{ Scan(...any) error }) (write.EfficacyReportRow, error) {
	var r write.EfficacyReportRow
	var excluded, results []byte
	var e sql.NullString
	var started, finished sql.NullTime
	err := row.Scan(&r.ID, &r.StudyID, &r.Status, &excluded, &results, &r.InputsHash, &r.DescriptiveOnly, &e, &r.RequestedBy, &r.IdempotencyKey,
		&started, &finished, &r.CreatedAt)
	r.Excluded, r.Error, r.StartedAt, r.FinishedAt = jsonUUIDs(excluded), nullString(e), nullTime(started), nullTime(finished)
	if len(results) > 0 {
		r.Results = results
	}
	return r, err
}

// InsertReport implements write.EfficacyStore.
func (EfficacyRepo) InsertReport(ctx context.Context, r write.EfficacyReportRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	excluded := r.Excluded
	if excluded == nil {
		excluded = []uuid.UUID{}
	}
	params, _ := json.Marshal(map[string]any{"excluded_question_version_ids": excluded})
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.efficacy_reports (report_id, study_id, status, params, requested_by, idempotency_key,
		created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$7)`, r.ID, r.StudyID, r.Status, params, r.RequestedBy, r.IdempotencyKey, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert efficacy report: %w", err)
	}
	return nil
}

// ReportByKey implements write.EfficacyStore.
func (EfficacyRepo) ReportByKey(ctx context.Context, requestedBy uuid.UUID, key string) (*write.EfficacyReportRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform table learning.efficacy_reports (G, no tenant_id), deleted_at IS NULL
	r, err := scanEfficacyReport(tx.QueryRowContext(ctx, `SELECT `+efficacyReportCols+` FROM learning.efficacy_reports WHERE requested_by = $1
		AND idempotency_key = $2 AND deleted_at IS NULL`, requestedBy, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("efficacy report: %w", err)
	}
	return &r, nil
}

// ClaimReports implements write.EfficacyStore: queued reports (and running ones without progress) locked with FOR UPDATE
// SKIP LOCKED, moved to running.
func (EfficacyRepo) ClaimReports(ctx context.Context, staleBefore time.Time, limit int) ([]write.EfficacyReportRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform table learning.efficacy_reports (G, no tenant_id), deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `UPDATE learning.efficacy_reports r SET status = 'running', started_at = now(), run_count = r.run_count + 1,
		version = r.version + 1, updated_at = now()
		WHERE r.report_id IN (SELECT report_id FROM learning.efficacy_reports WHERE deleted_at IS NULL
			AND (status = 'queued' OR (status = 'running' AND updated_at < $1)) ORDER BY created_at LIMIT $2 FOR UPDATE SKIP LOCKED)
		RETURNING `+efficacyReportCols, staleBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("claim efficacy reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.EfficacyReportRow
	for rows.Next() {
		r, err := scanEfficacyReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FinishReport implements write.EfficacyStore.
func (EfficacyRepo) FinishReport(ctx context.Context, r write.EfficacyReportRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var results any
	if len(r.Results) > 0 {
		results = []byte(r.Results)
	}
	// tenancyguard:ok platform table learning.efficacy_reports (G, no tenant_id), deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE learning.efficacy_reports SET status = $2, results = $3, inputs_hash = $4, descriptive_only = $5,
		error = $6, finished_at = $7, version = version + 1, updated_at = now() WHERE report_id = $1 AND deleted_at IS NULL`,
		r.ID, r.Status, results, r.InputsHash, r.DescriptiveOnly, r.Error, r.FinishedAt)
	if err != nil {
		return fmt.Errorf("finish efficacy report: %w", err)
	}
	return nil
}

// SessionTenant implements write.EfficacyStore.
func (EfficacyRepo) SessionTenant(ctx context.Context, tenant uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant.String()); err != nil {
		return fmt.Errorf("session tenant: %w", err)
	}
	return nil
}

// StudentMasteries implements write.EfficacyStore.
func (EfficacyRepo) StudentMasteries(ctx context.Context, tenant uuid.UUID, students, objectives []uuid.UUID) ([]write.MasteryRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+masteryColumns+` FROM learning.objective_masteries WHERE tenant_id = $1
		AND student_id = ANY($2::uuid[]) AND objective_id = ANY($3::uuid[]) AND deleted_at IS NULL ORDER BY student_id, objective_id`,
		tenant, uuidArray(students), uuidArray(objectives))
	if err != nil {
		return nil, fmt.Errorf("student masteries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.MasteryRow
	for rows.Next() {
		r, err := scanMastery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ActiveModel implements write.EfficacyStore.
func (EfficacyRepo) ActiveModel(ctx context.Context) (write.ModelConfig, error) {
	return MasteryRepo{}.ActiveModel(ctx)
}

// WithdrawnStudents implements write.EfficacyStore.
func (EfficacyRepo) WithdrawnStudents(ctx context.Context, tenant, studyID uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only learning.efficacy_withdrawals (no deleted_at) of the tenant given
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT s FROM learning.efficacy_withdrawals w, unnest(w.student_ids) s
		WHERE w.tenant_id = $1 AND (w.study_id = $2 OR w.study_id IS NULL) ORDER BY s`, tenant, studyID)
	if err != nil {
		return nil, fmt.Errorf("withdrawn students: %w", err)
	}
	return scanIDs(rows)
}
