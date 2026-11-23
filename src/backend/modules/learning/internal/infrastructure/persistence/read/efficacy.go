package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// EfficacyModel implements read.EfficacyReadModel (F035) with explicit SQL.
type EfficacyModel struct{}

var _ read.EfficacyReadModel = EfficacyModel{}

func jsonIDs(b []byte) []uuid.UUID {
	var out []uuid.UUID
	_ = json.Unmarshal(b, &out)
	return out
}

func idsParam(ids []uuid.UUID) any {
	if ids == nil {
		return nil
	}
	return uuidArray(ids)
}

func optStr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

func optInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// Agreements implements read.EfficacyReadModel.
func (EfficacyModel) Agreements(ctx context.Context, tenants []uuid.UUID, status string, seek read.Seek) ([]write.AgreementRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// one keyset page (tenant_id, created_at, agreement_id) — API_DESIGN §5
	rows, err := tx.QueryContext(ctx, `SELECT agreement_id, tenant_id, agreement_ref, to_jsonb(covers), agreed_on, status, revoked_at, revoked_by,
		revoke_reason, recorded_by, version, created_at FROM learning.research_agreements
		WHERE tenant_id = ANY($1::uuid[]) AND ($2 = '' OR status = $2) AND deleted_at IS NULL
			AND ($3::uuid IS NULL OR (tenant_id, created_at, agreement_id) > ($3::uuid, $4::timestamptz, $5::uuid))
		ORDER BY tenant_id, created_at, agreement_id LIMIT $6`,
		append([]any{uuidArray(tenants), status}, seekArgs(seek, 2)...)...)
	if err != nil {
		return nil, fmt.Errorf("research agreements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AgreementRow
	for rows.Next() {
		var a write.AgreementRow
		var covers []byte
		var revokedAt sql.NullTime
		var revokedBy uuid.NullUUID
		var reason sql.NullString
		if err := rows.Scan(&a.ID, &a.TenantID, &a.Ref, &covers, &a.AgreedOn, &a.Status, &revokedAt, &revokedBy, &reason, &a.RecordedBy, &a.Version,
			&a.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(covers, &a.Covers)
		if revokedAt.Valid {
			t := revokedAt.Time.UTC()
			a.RevokedAt = &t
		}
		a.RevokedBy, a.RevokeReason = nullUUID(revokedBy), optStr(reason)
		out = append(out, a)
	}
	return out, rows.Err()
}

const studyReadCols = `s.study_id, s.code, s.title, s.subject_id, s.grade_level, to_jsonb(s.objective_ids), s.form_a_blueprint_version_id,
	s.form_b_blueprint_version_id, s.min_intervention_weeks, s.status, s.created_by, s.activated_at, s.completed_at, s.version, s.created_at`

func scanStudyRead(r rowScanner) (write.StudyRow, error) {
	var s write.StudyRow
	var objectives []byte
	var activated, completed sql.NullTime
	err := r.Scan(&s.ID, &s.Code, &s.Title, &s.SubjectID, &s.GradeLevel, &objectives, &s.FormA, &s.FormB, &s.MinWeeks, &s.Status, &s.CreatedBy,
		&activated, &completed, &s.Version, &s.CreatedAt)
	s.ObjectiveIDs = jsonIDs(objectives)
	if activated.Valid {
		t := activated.Time.UTC()
		s.ActivatedAt = &t
	}
	if completed.Valid {
		t := completed.Time.UTC()
		s.CompletedAt = &t
	}
	return s, err
}

// Studies implements read.EfficacyReadModel: tenants filters the studies with a class of one of them.
func (EfficacyModel) Studies(ctx context.Context, status string, tenants []uuid.UUID, seek read.Seek) ([]write.StudyRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform table learning.efficacy_studies (G); the cohort filter names tenant_id = ANY($2) and deleted_at
	rows, err := tx.QueryContext(ctx, `SELECT `+studyReadCols+` FROM learning.efficacy_studies s WHERE s.deleted_at IS NULL
		AND ($1 = '' OR s.status = $1) AND ($2::uuid[] IS NULL OR EXISTS (SELECT 1 FROM learning.efficacy_study_cohorts c
			WHERE c.study_id = s.study_id AND c.tenant_id = ANY($2::uuid[]) AND c.deleted_at IS NULL))
		AND ($3::timestamptz IS NULL OR (s.created_at, s.study_id) < ($3::timestamptz, $4::uuid))
		ORDER BY s.created_at DESC, s.study_id DESC LIMIT $5`, append([]any{status, idsParam(tenants)}, seekArgs(seek, 1)...)...)
	if err != nil {
		return nil, fmt.Errorf("efficacy studies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.StudyRow
	for rows.Next() {
		s, err := scanStudyRead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Study implements read.EfficacyReadModel.
func (EfficacyModel) Study(ctx context.Context, id uuid.UUID) (*write.StudyRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform table learning.efficacy_studies (G, no tenant_id), deleted_at IS NULL
	s, err := scanStudyRead(tx.QueryRowContext(ctx, `SELECT `+studyReadCols+` FROM learning.efficacy_studies s WHERE s.study_id = $1
		AND s.deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("efficacy study: %w", err)
	}
	return &s, nil
}

// StudyCohorts implements read.EfficacyReadModel.
func (EfficacyModel) StudyCohorts(ctx context.Context, studyID uuid.UUID, tenants []uuid.UUID, seek read.Seek) ([]read.StudyCohortRead, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant_id = ANY($2) when given, else every tenant of the study under the audited bypass efficacy_study; deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT c.study_id, c.school_id, c.tenant_id, c.grade_level, c.pair_no, c.arm,
		c.assignment_method, c.form_order, c.status, c.conflict, c.withdrawn_at, c.withdrawal_id, c.added_by,
		(SELECT count(*) FROM learning.efficacy_participants p WHERE p.study_id = c.study_id AND p.school_id = c.school_id
			AND p.tenant_id = c.tenant_id AND p.status = 'active' AND p.deleted_at IS NULL)
		FROM learning.efficacy_study_cohorts c
		WHERE c.study_id = $1 AND ($2::uuid[] IS NULL OR c.tenant_id = ANY($2::uuid[])) AND c.deleted_at IS NULL
			AND ($3::int IS NULL OR (COALESCE(c.pair_no, `+cohortNoPair+`), c.school_id) > ($3::int, $4::uuid))
		ORDER BY COALESCE(c.pair_no, `+cohortNoPair+`), c.school_id LIMIT $5`, append([]any{studyID, idsParam(tenants)}, seekArgs(seek, 1)...)...)
	if err != nil {
		return nil, fmt.Errorf("study cohorts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.StudyCohortRead
	for rows.Next() {
		var c read.StudyCohortRead
		var pair sql.NullInt64
		var arm, order sql.NullString
		var withdrawn sql.NullTime
		var wid uuid.NullUUID
		if err := rows.Scan(&c.StudyID, &c.SchoolID, &c.TenantID, &c.GradeLevel, &pair, &arm, &c.Method, &order, &c.Status, &c.Conflict,
			&withdrawn, &wid, &c.AddedBy, &c.Participants); err != nil {
			return nil, err
		}
		c.PairNo, c.Arm, c.FormOrder, c.WithdrawalID = optInt(pair), optStr(arm), optStr(order), nullUUID(wid)
		if withdrawn.Valid {
			t := withdrawn.Time.UTC()
			c.WithdrawnAt = &t
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// cohortNoPair sorts the cohorts without a pair (validation arm) after every pair (pair_no ≤ 200): the keyset of
// listEfficacyStudyCohorts is (COALESCE(pair_no, 32767), school_id).
const cohortNoPair = "32767" // = read.CohortNoPair

// Waves implements read.EfficacyReadModel (keyset (window_start, wave_id)).
func (EfficacyModel) Waves(ctx context.Context, studyID uuid.UUID, seek read.Seek) ([]write.WaveRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform table learning.efficacy_waves (G, no tenant_id), deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT wave_id, study_id, kind, form, window_start, window_end, duration_sec, status, opened_at, closed_at,
		created_by, version, created_at FROM learning.efficacy_waves WHERE study_id = $1 AND deleted_at IS NULL
			AND ($2::timestamptz IS NULL OR (window_start, wave_id) > ($2::timestamptz, $3::uuid))
		ORDER BY window_start, wave_id LIMIT $4`, append([]any{studyID}, seekArgs(seek, 1)...)...)
	if err != nil {
		return nil, fmt.Errorf("efficacy waves: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.WaveRow
	for rows.Next() {
		var w write.WaveRow
		var form sql.NullString
		var opened, closed sql.NullTime
		if err := rows.Scan(&w.ID, &w.StudyID, &w.Kind, &form, &w.WindowStart, &w.WindowEnd, &w.DurationSec, &w.Status, &opened, &closed,
			&w.CreatedBy, &w.Version, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Form, w.WindowStart, w.WindowEnd = optStr(form), w.WindowStart.UTC(), w.WindowEnd.UTC()
		if opened.Valid {
			t := opened.Time.UTC()
			w.OpenedAt = &t
		}
		if closed.Valid {
			t := closed.Time.UTC()
			w.ClosedAt = &t
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// WaveSessions implements read.EfficacyReadModel: the counts come from the participations of each session.
func (EfficacyModel) WaveSessions(ctx context.Context, waveID uuid.UUID) ([]read.WaveSessionRead, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of the wave under the audited bypass efficacy_study; tenant_id joins and deleted_at IS NULL on each table
	rows, err := tx.QueryContext(ctx, `SELECT s.wave_id, s.school_id, s.tenant_id, s.grade_level, s.exam_session_id, s.form, s.status,
		count(p.student_id), count(p.student_id) FILTER (WHERE p.status IN ('submitted','graded')),
		count(p.student_id) FILTER (WHERE p.status = 'absent')
		FROM learning.efficacy_wave_cohort_sessions s LEFT JOIN assessment.exam_participations p ON p.exam_session_id = s.exam_session_id
			AND p.tenant_id = s.tenant_id AND p.deleted_at IS NULL
		WHERE s.wave_id = $1 AND s.deleted_at IS NULL GROUP BY s.wave_id, s.school_id, s.tenant_id, s.grade_level, s.exam_session_id, s.form, s.status
		ORDER BY s.tenant_id, s.school_id`, waveID)
	if err != nil {
		return nil, fmt.Errorf("wave sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.WaveSessionRead
	for rows.Next() {
		var s read.WaveSessionRead
		if err := rows.Scan(&s.WaveID, &s.SchoolID, &s.TenantID, &s.GradeLevel, &s.ExamSessionID, &s.Form, &s.Status, &s.Invited, &s.Completed,
			&s.Absent); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Report implements read.EfficacyReadModel.
func (EfficacyModel) Report(ctx context.Context, studyID, reportID uuid.UUID) (*write.EfficacyReportRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var r write.EfficacyReportRow
	var excluded, results []byte
	var e sql.NullString
	var started, finished sql.NullTime
	// tenancyguard:ok platform table learning.efficacy_reports (G, no tenant_id), deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT report_id, study_id, status, COALESCE(params->'excluded_question_version_ids', '[]'::jsonb), results,
		descriptive_only, error, requested_by, started_at, finished_at, created_at FROM learning.efficacy_reports
		WHERE report_id = $1 AND study_id = $2 AND deleted_at IS NULL`, reportID, studyID).
		Scan(&r.ID, &r.StudyID, &r.Status, &excluded, &results, &r.DescriptiveOnly, &e, &r.RequestedBy, &started, &finished, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("efficacy report: %w", err)
	}
	r.Excluded, r.Error = jsonIDs(excluded), optStr(e)
	if len(results) > 0 {
		r.Results = results
	}
	if started.Valid {
		t := started.Time.UTC()
		r.StartedAt = &t
	}
	if finished.Valid {
		t := finished.Time.UTC()
		r.FinishedAt = &t
	}
	return &r, nil
}
