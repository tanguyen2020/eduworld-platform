// Package questsql reads the published quests (G tables), their tenant assignment (infra.tenant_resource_assignments)
// and the quest progress of a student (T tables) on a transaction; shared by the write repositories and the read
// models of module game (F072). Explicit SQL: every tenant table carries its tenant_id and deleted_at conditions
// (AGENTS R27), platform content its deleted_at condition.
package questsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Querier is the part of *sql.Tx the readers need.
type Querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// Content implements port.QuestContent on the transaction of ctx (read session or write transaction).
type Content struct{}

var _ port.QuestContent = Content{}

func q(ctx context.Context) (Querier, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

const versionCols = `v.quest_id, v.quest_version_id, v.version_no, v.payload_sha256, v.payload`

func scanQuests(rows *sql.Rows) ([]port.QuestRow, error) {
	defer func() { _ = rows.Close() }()
	var out []port.QuestRow
	for rows.Next() {
		var r port.QuestRow
		var raw []byte
		if err := rows.Scan(&r.QuestID, &r.VersionID, &r.VersionNo, &r.SHA256, &raw); err != nil {
			return nil, err
		}
		v, err := quest.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("quest payload: %w", err)
		}
		r.Version = v
		out = append(out, r)
	}
	return out, rows.Err()
}

func one(rows []port.QuestRow, err error) (*port.QuestRow, error) {
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// AssignedQuests implements port.QuestContent.
func (Content) AssignedQuests(ctx context.Context, tenant uuid.UUID, grade int) ([]port.QuestRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform quests (G tables) joined with the active assignments of the session tenant ($1, explicit; infra without RLS)
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.quest_definitions d
		JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id
		JOIN infra.tenant_resource_assignments a ON a.resource_type = 'quest' AND a.resource_id = d.quest_id AND a.tenant_id = $1
			AND a.deleted_at IS NULL
		WHERE d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published' AND ($2::int = 0 OR v.grade_levels @> jsonb_build_array($2::int))
		ORDER BY v.code`, tenant, grade)
	if err != nil {
		return nil, fmt.Errorf("assigned quests: %w", err)
	}
	return scanQuests(rows)
}

// Assigned implements port.QuestContent.
func (Content) Assigned(ctx context.Context, tenant, questID uuid.UUID) (bool, error) {
	t, err := q(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok infra.tenant_resource_assignments (no RLS, ADR-0089 §4): explicit tenant_id + deleted_at IS NULL
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM infra.tenant_resource_assignments
		WHERE resource_type = 'quest' AND resource_id = $2 AND tenant_id = $1 AND deleted_at IS NULL)`, tenant, questID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("quest assignment: %w", err)
	}
	return ok, nil
}

// CurrentQuest implements port.QuestContent.
func (Content) CurrentQuest(ctx context.Context, questID uuid.UUID) (*port.QuestRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.quest_definitions d
		JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id
		WHERE d.quest_id = $1 AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, questID)
	if err != nil {
		return nil, fmt.Errorf("quest: %w", err)
	}
	return one(scanQuests(rows))
}

// CurrentQuests implements port.QuestContent.
func (Content) CurrentQuests(ctx context.Context) ([]port.QuestRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.quest_definitions d
		JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id
		WHERE d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published' ORDER BY v.code`)
	if err != nil {
		return nil, fmt.Errorf("quests: %w", err)
	}
	return scanQuests(rows)
}

// QuestVersion implements port.QuestContent: a pinned version stays readable when archived by a newer publish.
func (Content) QuestVersion(ctx context.Context, versionID uuid.UUID) (*port.QuestRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.quest_versions v
		WHERE v.quest_version_id = $1 AND v.deleted_at IS NULL AND v.status IN ('published', 'archived')`, versionID)
	if err != nil {
		return nil, fmt.Errorf("quest version: %w", err)
	}
	return one(scanQuests(rows))
}

// ProgressColumns are the columns ScanProgress reads.
const ProgressColumns = `quest_progress_id, tenant_id, student_id, school_id, quest_id, quest_version_id, quest_type, status, step_states,
	start_trigger, period_key, completion_no, party_id, started_at, completed_at, expires_at, deadline_at, failed_at, ended_at, version`

// ScanProgress reads one quest_progress row of ProgressColumns.
func ScanProgress(row interface{ Scan(...any) error }) (*quest.Progress, error) {
	var p quest.Progress
	var steps []byte
	var period sql.NullString
	var party uuid.NullUUID
	var completed, expires, deadline, failed, ended sql.NullTime
	if err := row.Scan(&p.ID, &p.TenantID, &p.StudentID, &p.SchoolID, &p.QuestID, &p.VersionID, &p.QuestType, &p.Status, &steps,
		&p.StartTrigger, &period, &p.CompletionNo, &party, &p.StartedAt, &completed, &expires, &deadline, &failed, &ended, &p.Version); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(steps, &p.Steps); err != nil {
		return nil, fmt.Errorf("quest step states: %w", err)
	}
	if p.Steps == nil {
		p.Steps = map[string]quest.StepState{}
	}
	p.PeriodKey = period.String
	if party.Valid {
		p.PartyID = &party.UUID
	}
	p.StartedAt = p.StartedAt.UTC()
	p.CompletedAt, p.ExpiresAt, p.DeadlineAt, p.FailedAt, p.EndedAt = tp(completed), tp(expires), tp(deadline), tp(failed), tp(ended)
	return &p, nil
}

func tp(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// Histories implements port.QuestContent.
func (Content) Histories(ctx context.Context, tenant, student uuid.UUID) (map[uuid.UUID]*port.QuestHistory, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err := t.QueryContext(ctx, `SELECT `+ProgressColumns+` FROM game.quest_progress
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL ORDER BY started_at, quest_progress_id`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("quest progress: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]*port.QuestHistory{}
	for rows.Next() {
		p, err := ScanProgress(rows)
		if err != nil {
			return nil, err
		}
		h := out[p.QuestID]
		if h == nil {
			h = &port.QuestHistory{CompletedPeriods: map[string]bool{}}
			out[p.QuestID] = h
		}
		h.Latest = p
		if p.Status == quest.StatusCompleted {
			h.Completed++
			if h.LastCompletedAt == nil || (p.CompletedAt != nil && p.CompletedAt.After(*h.LastCompletedAt)) {
				h.LastCompletedAt = p.CompletedAt
			}
			if p.PeriodKey != "" {
				h.CompletedPeriods[p.PeriodKey] = true
			}
		}
	}
	return out, rows.Err()
}

// Discovered implements port.QuestContent.
func (Content) Discovered(ctx context.Context, tenant, student uuid.UUID) (map[string]bool, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.exploration_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err := t.QueryContext(ctx, `SELECT discoveries, collectibles FROM game.exploration_progress
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("exploration progress: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var d, c []byte
		if err := rows.Scan(&d, &c); err != nil {
			return nil, err
		}
		for _, raw := range [][]byte{d, c} {
			var ids []string
			if err := json.Unmarshal(raw, &ids); err != nil {
				return nil, fmt.Errorf("exploration ids: %w", err)
			}
			for _, id := range ids {
				out[id] = true
			}
		}
	}
	return out, rows.Err()
}
