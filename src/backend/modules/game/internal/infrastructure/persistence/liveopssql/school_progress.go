package liveopssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
)

// SchoolEventProgress implements read.LiveOpsReadModel (F081-AC-10): the participants of the run (of School when set),
// the progress row of the goal scope (PostgreSQL — the Valkey counter is never read, R9) and the contributions by grade
// (grade at the time of the contribution, ADR-0095 §2). Aggregates only: no student id leaves the database (R16).
func (Store) SchoolEventProgress(ctx context.Context, f read.SchoolEventProgressFilter) (read.SchoolEventProgressRow, error) {
	out := read.SchoolEventProgressRow{Reached: []int{}}
	t, err := tx(ctx)
	if err != nil {
		return out, err
	}
	school := nullUUID(f.School)
	// tenancyguard:ok tenant table game.event_participation: explicit tenant_id + deleted_at IS NULL; a count only
	if err := t.QueryRowContext(ctx, `SELECT count(*) FROM game.event_participation WHERE tenant_id = $1 AND event_run_id = $2
		AND deleted_at IS NULL AND ($3::uuid IS NULL OR school_id = $3)`, f.Tenant, f.Run, school).Scan(&out.Participants); err != nil {
		return out, fmt.Errorf("school event participants: %w", err)
	}
	if f.Goal == nil {
		return out, nil
	}
	var reached []byte
	var at sql.NullTime
	// tenancyguard:ok tenant table game.community_goal_progress: explicit tenant_id + deleted_at IS NULL
	err = t.QueryRowContext(ctx, `SELECT total, milestones_reached, greatest(updated_at, recomputed_at) FROM game.community_goal_progress
		WHERE tenant_id = $1 AND goal_id = $2 AND scope_id = $3 AND deleted_at IS NULL`, f.Tenant, *f.Goal, f.ScopeID).Scan(&out.Total, &reached, &at)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return out, fmt.Errorf("school event goal progress: %w", err)
	default:
		_ = json.Unmarshal(reached, &out.Reached)
		if at.Valid {
			v := at.Time.UTC()
			out.UpdatedAt = &v
		}
	}
	// tenancyguard:ok append-only contributions (TA, no deleted_at): explicit tenant_id; aggregates only, no student id
	rows, err := t.QueryContext(ctx, `SELECT grade_level, count(DISTINCT student_id), sum(amount) FROM game.community_goal_contributions
		WHERE tenant_id = $1 AND goal_id = $2 AND ($3::uuid IS NULL OR school_id = $3) AND grade_level IS NOT NULL
		GROUP BY grade_level ORDER BY grade_level`, f.Tenant, *f.Goal, school)
	if err != nil {
		return out, fmt.Errorf("school event grades: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var g port.GradeContribution
		if err := rows.Scan(&g.Grade, &g.Contributors, &g.Progress); err != nil {
			return out, err
		}
		out.Grades = append(out.Grades, g)
	}
	return out, rows.Err()
}

// QuestOptionItems implements read.LiveOpsReadModel (F081-AC-02): name and grades of the current published version.
func (Store) QuestOptionItems(ctx context.Context, quests []uuid.UUID) (map[uuid.UUID]read.OptionItem, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
	return optionItems(ctx, quests, `SELECT d.quest_id, v.payload->'name', v.grade_levels FROM game.quest_definitions d
		JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		WHERE d.quest_id = ANY($1::uuid[]) AND d.deleted_at IS NULL`)
}

// ChallengeOptionItems implements read.LiveOpsReadModel (F081-AC-02): name and grades of the current published version.
func (Store) ChallengeOptionItems(ctx context.Context, challenges []uuid.UUID) (map[uuid.UUID]read.OptionItem, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
	return optionItems(ctx, challenges, `SELECT d.challenge_id, v.payload->'name', v.grade_levels FROM game.challenges d
		JOIN game.challenge_versions v ON v.challenge_version_id = d.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		WHERE d.challenge_id = ANY($1::uuid[]) AND d.deleted_at IS NULL`)
}

func optionItems(ctx context.Context, ids []uuid.UUID, q string) (map[uuid.UUID]read.OptionItem, error) {
	out := map[uuid.UUID]read.OptionItem{}
	if len(ids) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("template option items: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var it read.OptionItem
		var name, grades []byte
		if err := rows.Scan(&it.ID, &name, &grades); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(name, &it.Title)
		_ = json.Unmarshal(grades, &it.GradeLevels)
		out[it.ID] = it
	}
	return out, rows.Err()
}
