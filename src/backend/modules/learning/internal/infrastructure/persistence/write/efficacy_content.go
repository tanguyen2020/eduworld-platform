package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

var _ write.EfficacyContent = EfficacyRepo{}

// FormVersion implements write.EfficacyContent.
func (EfficacyRepo) FormVersion(ctx context.Context, versionID uuid.UUID) (*write.FormVersion, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	f := write.FormVersion{VersionID: versionID}
	var subject uuid.NullUUID
	var grades []byte
	err = tx.QueryRowContext(ctx, `SELECT v.assessment_blueprint_id, v.status, v.subject_id, v.grade_levels, v.body
		FROM content.assessment_blueprint_versions v WHERE v.assessment_blueprint_version_id = $1 AND v.tenant_id IS NULL
		AND v.deleted_at IS NULL`, versionID).Scan(&f.ItemID, &f.Status, &subject, &grades, &f.Body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("form version: %w", err)
	}
	f.SubjectID = nullUUID(subject)
	f.GradeLevels = jsonInts(grades)
	return &f, nil
}

// InstrumentQuestions implements write.EfficacyContent.
func (EfficacyRepo) InstrumentQuestions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]write.InstrumentQuestion, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT v.question_version_id, v.efficacy_instrument,
		COALESCE((SELECT jsonb_agg(o.objective_id ORDER BY o.objective_id) FROM content.question_version_objectives o
			WHERE o.question_version_id = v.question_version_id AND o.deleted_at IS NULL), '[]'::jsonb)
		FROM content.question_versions v WHERE v.question_version_id = ANY($1::uuid[]) AND v.tenant_id IS NULL AND v.status = 'published'
		AND v.deleted_at IS NULL`, uuidArray(ids))
	if err != nil {
		return nil, fmt.Errorf("instrument questions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]write.InstrumentQuestion{}
	for rows.Next() {
		var q write.InstrumentQuestion
		var los []byte
		if err := rows.Scan(&q.VersionID, &q.Efficacy, &los); err != nil {
			return nil, err
		}
		q.ObjectiveIDs = jsonUUIDs(los)
		out[q.VersionID] = q
	}
	return out, rows.Err()
}

func jsonInts(b []byte) []int {
	var out []int
	_ = json.Unmarshal(b, &out)
	return out
}

// InstrumentPool implements write.EfficacyContent.
func (EfficacyRepo) InstrumentPool(ctx context.Context, ids []uuid.UUID) ([]selection.Candidate, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// The view keeps published live versions; platform rows only (tenant_id IS NULL), efficacy instruments only.
	rows, err := tx.QueryContext(ctx, `SELECT v.question_version_id, v.question_id, v.type, v.difficulty, v.subject_id, v.topic_id,
		v.grade_levels, v.tags, v.exam_only,
		COALESCE((SELECT jsonb_agg(o.objective_id) FROM content.question_version_objectives o
			WHERE o.question_version_id = v.question_version_id AND o.deleted_at IS NULL), '[]'::jsonb)
		FROM content.question_delivery_v v WHERE v.question_version_id = ANY($1::uuid[]) AND v.tenant_id IS NULL AND v.efficacy_instrument
		ORDER BY v.question_version_id`, uuidArray(ids))
	if err != nil {
		return nil, fmt.Errorf("instrument pool: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []selection.Candidate
	for rows.Next() {
		var c selection.Candidate
		var grades, tags, los []byte
		if err := rows.Scan(&c.VersionID, &c.QuestionID, &c.Type, &c.Difficulty, &c.SubjectID, &c.TopicID, &grades, &tags, &c.ExamOnly, &los); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(grades, &c.GradeLevels)
		_ = json.Unmarshal(tags, &c.Tags)
		_ = json.Unmarshal(los, &c.ObjectiveIDs)
		out = append(out, c)
	}
	return out, rows.Err()
}
