package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// LessonModel implements read.LessonReadModel with explicit SQL in the read transaction of ctx.
type LessonModel struct{}

var _ read.LessonReadModel = LessonModel{}

// Version implements read.LessonReadModel.
func (LessonModel) Version(ctx context.Context, lessonID uuid.UUID, versionID *uuid.UUID) (*read.LessonVersionView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var v read.LessonVersionView
	var subject uuid.NullUUID
	var grades, objectives, title, summary, body []byte
	// H tables: RLS shows the tenant's own lessons and the platform lessons assigned to it.
	// tenancyguard:ok H tables content.lessons/lesson_versions: tenant + assignment visibility is their RLS policy; deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT l.lesson_id, v.lesson_version_id, v.version_no, v.status, v.title, v.summary, v.subject_id,
			v.grade_levels, v.objective_ids, v.body
		FROM content.lessons l JOIN content.lesson_versions v ON v.lesson_id = l.lesson_id
		WHERE l.lesson_id = $1 AND l.deleted_at IS NULL AND v.deleted_at IS NULL
			AND v.lesson_version_id = COALESCE($2::uuid, l.current_version_id)`, lessonID, versionID).
		Scan(&v.LessonID, &v.VersionID, &v.VersionNo, &v.Status, &title, &summary, &subject, &grades, &objectives, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lesson version: %w", err)
	}
	v.Title, v.Summary, v.Body = title, summary, body
	if subject.Valid {
		s := subject.UUID
		v.SubjectID = &s
	}
	_ = json.Unmarshal(grades, &v.GradeLevels)
	_ = json.Unmarshal(objectives, &v.ObjectiveIDs)
	return &v, nil
}

// Session implements read.LessonReadModel.
func (LessonModel) Session(ctx context.Context, studentID, lessonID uuid.UUID) (*read.LessonSessionView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var s read.LessonSessionView
	var acts string
	var ended sql.NullTime
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT lesson_version_id, status, array_to_string(completed_activity_ids, ','), ended_at
		FROM learning.lesson_sessions WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND lesson_id = $2 AND deleted_at IS NULL
		ORDER BY (status = 'in_progress') DESC, started_at DESC LIMIT 1`, studentID, lessonID).Scan(&s.LessonVersionID, &s.Status, &acts, &ended)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lesson session: %w", err)
	}
	for _, p := range strings.Split(acts, ",") {
		if id, err := uuid.Parse(p); err == nil {
			s.CompletedActivityIDs = append(s.CompletedActivityIDs, id)
		}
	}
	if ended.Valid {
		t := ended.Time.UTC()
		s.EndedAt = &t
	}
	return &s, nil
}

// Activities implements read.LessonReadModel.
func (LessonModel) Activities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]read.ActivityInfo, error) {
	out := map[uuid.UUID]read.ActivityInfo{}
	if len(ids) == 0 {
		return out, nil
	}
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	// tenancyguard:ok H tables content.activities/activity_versions: tenant + assignment visibility is their RLS policy; deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT a.activity_id, v.activity_type, v.title FROM content.activities a
		JOIN content.activity_versions v ON v.activity_version_id = a.current_version_id
		WHERE a.activity_id = ANY($1::uuid[]) AND a.deleted_at IS NULL AND v.deleted_at IS NULL`, "{"+strings.Join(strs, ",")+"}")
	if err != nil {
		return nil, fmt.Errorf("lesson activities: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var typ sql.NullString
		var title []byte
		if err := rows.Scan(&id, &typ, &title); err != nil {
			return nil, err
		}
		out[id] = read.ActivityInfo{ActivityType: typ.String, Title: title}
	}
	return out, rows.Err()
}
