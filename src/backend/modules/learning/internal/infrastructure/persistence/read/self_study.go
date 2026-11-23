package read

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// SelfStudyModel implements read.SelfStudyReadModel with explicit SQL in the read transaction of ctx (TenantScope: RLS T).
type SelfStudyModel struct{}

var _ read.SelfStudyReadModel = SelfStudyModel{}

func idsOf(list string) []uuid.UUID {
	out := []uuid.UUID{}
	for _, s := range strings.Split(list, ",") {
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}

// WrongQuestions implements read.SelfStudyReadModel.
func (SelfStudyModel) WrongQuestions(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID, seek read.Seek) ([]read.WrongQuestionRecord, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT entry_id, question_version_id, subject_id, array_to_string(objective_ids, ','), wrong_count,
			last_wrong_at, next_due_at
		FROM learning.wrong_question_entries
		WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND status = 'active' AND deleted_at IS NULL
			AND ($2::uuid IS NULL OR subject_id = $2::uuid)
			AND ($3::timestamptz IS NULL OR (next_due_at, entry_id) > ($3::timestamptz, $4::uuid))
		ORDER BY next_due_at, entry_id LIMIT $5`, append([]any{studentID, subjectID}, seekArgs(seek, 1)...)...)
	if err != nil {
		return nil, fmt.Errorf("wrong questions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.WrongQuestionRecord
	for rows.Next() {
		var r read.WrongQuestionRecord
		var subject uuid.NullUUID
		var los string
		if err := rows.Scan(&r.EntryID, &r.QuestionVersionID, &subject, &los, &r.WrongCount, &r.LastWrongAt, &r.NextDueAt); err != nil {
			return nil, err
		}
		if subject.Valid {
			s := subject.UUID
			r.SubjectID = &s
		}
		r.ObjectiveIDs = idsOf(los)
		r.LastWrongAt, r.NextDueAt = r.LastWrongAt.UTC(), r.NextDueAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// Recommendations implements read.SelfStudyReadModel.
func (SelfStudyModel) Recommendations(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID, now time.Time) ([]read.RecommendationRecord, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT recommendation_id, rule_code, kind, target_type, target_id, subject_id, grade_level,
			array_to_string(objective_ids, ','), reason, reason_code, reason_params, priority, generated_at, expires_at
		FROM learning.rm_student_recommendations
		WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND status = 'active' AND deleted_at IS NULL AND expires_at > $3
			AND ($2::uuid IS NULL OR subject_id = $2::uuid)
		ORDER BY priority, generated_at, recommendation_id LIMIT 20`, studentID, subjectID, now)
	if err != nil {
		return nil, fmt.Errorf("recommendations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.RecommendationRecord
	for rows.Next() {
		var r read.RecommendationRecord
		var subject uuid.NullUUID
		var grade sql.NullInt32
		var los string
		if err := rows.Scan(&r.ID, &r.RuleCode, &r.Kind, &r.TargetType, &r.TargetID, &subject, &grade, &los, &r.Reason, &r.ReasonCode,
			&r.ReasonParams, &r.Priority, &r.GeneratedAt, &r.ExpiresAt); err != nil {
			return nil, err
		}
		if subject.Valid {
			s := subject.UUID
			r.SubjectID = &s
		}
		if grade.Valid {
			g := int(grade.Int32)
			r.Grade = &g
		}
		r.ObjectiveIDs = idsOf(los)
		r.GeneratedAt = r.GeneratedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// MasteryDetails implements read.SelfStudyReadModel.
func (SelfStudyModel) MasteryDetails(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID, grade *int) ([]read.MasteryDetail, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// The curriculum metadata of an objective (H tables visible to the tenant through RLS) may be missing: the row stays.
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on every table
	rows, err := q.QueryContext(ctx, `SELECT m.objective_id, COALESCE(lo.code, ''), COALESCE(lo.description, ''), lo.topic_id, t.subject_id,
			m.value::float8, m.evidence_count, m.distinct_days, m.last_evidence_at, COALESCE(to_char(m.last_evidence_date, 'YYYY-MM-DD'), ''),
			m.first_mastered_at, m.prior_source, m.prior_value::float8
		FROM learning.objective_masteries m
		LEFT JOIN curriculum.learning_objectives lo ON lo.objective_id = m.objective_id AND lo.deleted_at IS NULL
		LEFT JOIN curriculum.topics t ON t.topic_id = lo.topic_id AND t.deleted_at IS NULL
		WHERE m.tenant_id = infra.current_tenant_id() AND m.student_id = $1 AND m.deleted_at IS NULL
			AND ($2::uuid IS NULL OR t.subject_id = $2::uuid) AND ($3::int IS NULL OR t.grade_level = $3::int)
		ORDER BY t.grade_level, t.sort_order, lo.sort_order, m.objective_id LIMIT 500`, studentID, subjectID, grade)
	if err != nil {
		return nil, fmt.Errorf("mastery details: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.MasteryDetail
	for rows.Next() {
		var d read.MasteryDetail
		var topic, subject uuid.NullUUID
		s := &d.State
		if err := rows.Scan(&d.ObjectiveID, &d.Code, &d.Name, &topic, &subject, &s.Value, &s.EvidenceCount, &s.DistinctDays, &s.LastEvidenceAt,
			&s.LastEvidenceDate, &s.FirstMasteredAt, &s.PriorSource, &s.PriorValue); err != nil {
			return nil, err
		}
		if topic.Valid {
			x := topic.UUID
			d.TopicID = &x
		}
		if subject.Valid {
			x := subject.UUID
			d.SubjectID = &x
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
