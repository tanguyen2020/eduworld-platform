package read

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// MinigameSourceModel implements read.MinigameSourceReadModel with explicit SQL on content.question_versions (metadata
// columns only, published live versions) joined with content.questions under the HybridCondition of the scope of ctx: the
// delivery view content.question_delivery_v is filtered by the session tenant (RLS of its owner), which a PlatformScope
// does not have.
type MinigameSourceModel struct{}

var _ read.MinigameSourceReadModel = MinigameSourceModel{}

// Pool implements read.MinigameSourceReadModel: the current published release of every visible question.
func (MinigameSourceModel) Pool(ctx context.Context, f read.PoolFilter) ([]selection.Candidate, error) {
	cond, args, err := HybridCondition(ctx, "q", contentmodel.ResourceQuestion, "question_id", 1)
	if err != nil {
		return nil, err
	}
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	subject := uuid.NullUUID{}
	if f.SubjectID != nil {
		subject = uuid.NullUUID{UUID: *f.SubjectID, Valid: true}
	}
	args = append(args, subject)
	// tenancyguard:ok the condition of HybridCondition on q holds tenant_id and deleted_at IS NULL; v is a published live version
	rows, err := tx.QueryContext(ctx, `SELECT v.question_version_id, v.question_id, v.type, v.difficulty, v.subject_id, v.topic_id,
		v.grade_levels, v.tags, v.exam_only,
		COALESCE((SELECT jsonb_agg(o.objective_id) FROM content.question_version_objectives o
			WHERE o.question_version_id = v.question_version_id AND o.deleted_at IS NULL), '[]'::jsonb)
		FROM content.question_versions v
		JOIN content.questions q ON q.question_id = v.question_id AND q.current_version_id = v.question_version_id
		WHERE `+cond+fmt.Sprintf(` AND v.status = 'published' AND v.deleted_at IS NULL AND NOT v.efficacy_instrument
		AND ($%d::uuid IS NULL OR v.subject_id = $%d::uuid) ORDER BY v.question_version_id`, len(args), len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("minigame source pool: %w", err)
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

// QuestionTypes implements read.MinigameSourceReadModel: the type of each published version visible in the scope.
func (MinigameSourceModel) QuestionTypes(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(versionIDs) == 0 {
		return out, nil
	}
	cond, args, err := HybridCondition(ctx, "q", contentmodel.ResourceQuestion, "question_id", 1)
	if err != nil {
		return nil, err
	}
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	args = append(args, uuidArray(versionIDs))
	// tenancyguard:ok the condition of HybridCondition on q holds tenant_id and deleted_at IS NULL; v is a published live version
	rows, err := tx.QueryContext(ctx, `SELECT v.question_version_id, v.type FROM content.question_versions v
		JOIN content.questions q ON q.question_id = v.question_id
		WHERE `+cond+fmt.Sprintf(` AND v.status = 'published' AND v.deleted_at IS NULL AND v.question_version_id = ANY($%d::uuid[])`, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("minigame source types: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var t string
		if err := rows.Scan(&id, &t); err != nil {
			return nil, err
		}
		out[id] = t
	}
	return out, rows.Err()
}
