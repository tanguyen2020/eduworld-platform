// Package worldsql reads the published world content (G tables) and the world progress of a student (T tables) on a
// transaction; shared by the write repositories and the read models of module game (F071). Explicit SQL: every tenant
// table carries its tenant_id and deleted_at conditions (AGENTS R27), platform content its deleted_at condition.
package worldsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Querier is the part of *sql.Tx the readers need.
type Querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// Row types are the port types (shared by the write repositories and the read models).
type (
	AreaRow        = port.AreaRow
	ChapterRow     = port.ChapterRow
	LevelRow       = port.LevelRow
	DialogueRow    = port.DialogueRow
	Student        = port.WorldStudent
	Progress       = port.WorldProgress
	ExplorationRow = port.ExplorationRow
)

func scanAreas(rows *sql.Rows) ([]AreaRow, error) {
	defer func() { _ = rows.Close() }()
	var out []AreaRow
	for rows.Next() {
		var r AreaRow
		var raw []byte
		if err := rows.Scan(&r.VersionID, &r.VersionNo, &r.SHA256, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &r.Area); err != nil {
			return nil, fmt.Errorf("world area payload: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CurrentAreas returns every published area (current version), ordered by area_id.
func CurrentAreas(ctx context.Context, q Querier) ([]AreaRow, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := q.QueryContext(ctx, `SELECT v.area_version_id, v.version_no, v.payload_sha256, v.payload FROM game.world_areas a
		JOIN game.world_area_versions v ON v.area_version_id = a.current_version_id
		WHERE a.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published' ORDER BY a.area_id`)
	if err != nil {
		return nil, fmt.Errorf("world areas: %w", err)
	}
	return scanAreas(rows)
}

// CurrentArea returns the published version of one area, nil when the area is unknown or unpublished.
func CurrentArea(ctx context.Context, q Querier, areaID string) (*AreaRow, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := q.QueryContext(ctx, `SELECT v.area_version_id, v.version_no, v.payload_sha256, v.payload FROM game.world_areas a
		JOIN game.world_area_versions v ON v.area_version_id = a.current_version_id
		WHERE a.area_id = $1 AND a.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, areaID)
	if err != nil {
		return nil, fmt.Errorf("world area: %w", err)
	}
	out, err := scanAreas(rows)
	if err != nil || len(out) == 0 {
		return nil, err
	}
	return &out[0], nil
}

// AreaVersion returns one version of an area whatever its status (the version pinned by a level session).
func AreaVersion(ctx context.Context, q Querier, versionID uuid.UUID) (*AreaRow, error) {
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	rows, err := q.QueryContext(ctx, `SELECT area_version_id, version_no, payload_sha256, payload FROM game.world_area_versions
		WHERE area_version_id = $1 AND deleted_at IS NULL`, versionID)
	if err != nil {
		return nil, fmt.Errorf("world area version: %w", err)
	}
	out, err := scanAreas(rows)
	if err != nil || len(out) == 0 {
		return nil, err
	}
	return &out[0], nil
}

// CurrentChapters returns every published chapter, ordered by order_no.
func CurrentChapters(ctx context.Context, q Querier) ([]ChapterRow, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := q.QueryContext(ctx, `SELECT v.chapter_version_id, v.version_no, v.payload_sha256, v.payload FROM game.story_chapters c
		JOIN game.story_chapter_versions v ON v.chapter_version_id = c.current_version_id
		WHERE c.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published' ORDER BY v.order_no, c.chapter_code`)
	if err != nil {
		return nil, fmt.Errorf("story chapters: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ChapterRow
	for rows.Next() {
		var r ChapterRow
		var raw []byte
		if err := rows.Scan(&r.VersionID, &r.VersionNo, &r.SHA256, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &r.Chapter); err != nil {
			return nil, fmt.Errorf("story chapter payload: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
const levelSelect = `SELECT d.level_id, v.level_version_id, v.version_no, v.payload_sha256, v.payload FROM game.level_definitions d
	JOIN game.level_versions v ON v.level_version_id = d.current_version_id
	WHERE d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`

func scanLevels(rows *sql.Rows) ([]LevelRow, error) {
	defer func() { _ = rows.Close() }()
	var out []LevelRow
	for rows.Next() {
		var r LevelRow
		var raw []byte
		if err := rows.Scan(&r.LevelID, &r.VersionID, &r.VersionNo, &r.SHA256, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &r.Level); err != nil {
			return nil, fmt.Errorf("level payload: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CurrentLevels returns every published level package.
func CurrentLevels(ctx context.Context, q Querier) ([]LevelRow, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := q.QueryContext(ctx, levelSelect+` ORDER BY d.area_id`)
	if err != nil {
		return nil, fmt.Errorf("levels: %w", err)
	}
	return scanLevels(rows)
}

// CurrentLevel returns the published version of a level, nil when unknown or unpublished.
func CurrentLevel(ctx context.Context, q Querier, levelID uuid.UUID) (*LevelRow, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := q.QueryContext(ctx, levelSelect+` AND d.level_id = $1`, levelID)
	if err != nil {
		return nil, fmt.Errorf("level: %w", err)
	}
	out, err := scanLevels(rows)
	if err != nil || len(out) == 0 {
		return nil, err
	}
	return &out[0], nil
}

// LevelOfArea returns the published level of an area, nil when none.
func LevelOfArea(ctx context.Context, q Querier, areaID string) (*LevelRow, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := q.QueryContext(ctx, levelSelect+` AND d.area_id = $1`, areaID)
	if err != nil {
		return nil, fmt.Errorf("level of area: %w", err)
	}
	out, err := scanLevels(rows)
	if err != nil || len(out) == 0 {
		return nil, err
	}
	return &out[0], nil
}

// CurrentDialogues returns the published dialogues of an NPC (every NPC when npcID is "").
func CurrentDialogues(ctx context.Context, q Querier, npcID string) ([]DialogueRow, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := q.QueryContext(ctx, `SELECT d.dialogue_id, v.dialogue_version_id, v.version_no, v.payload_sha256, v.npc_id,
			v.grade_levels, v.payload FROM game.npc_dialogues d
		JOIN game.npc_dialogue_versions v ON v.dialogue_version_id = d.current_version_id
		WHERE ($1 = '' OR d.npc_id = $1) AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'
		ORDER BY d.npc_id, d.dialogue_id`, npcID)
	if err != nil {
		return nil, fmt.Errorf("npc dialogues: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []DialogueRow
	for rows.Next() {
		var r DialogueRow
		var grades, raw []byte
		if err := rows.Scan(&r.DialogueID, &r.VersionID, &r.VersionNo, &r.SHA256, &r.NPCID, &grades, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(grades, &r.Grades); err != nil {
			return nil, fmt.Errorf("npc dialogue grades: %w", err)
		}
		r.Payload = raw
		out = append(out, r)
	}
	return out, rows.Err()
}

// StudentOf returns the projected student of tenant with the enabled subjects of its school, nil when not projected.
func StudentOf(ctx context.Context, q Querier, tenant, student uuid.UUID) (*Student, error) {
	var s Student
	var grade sql.NullInt64
	var deleted sql.NullTime
	var subjects []byte
	// tenancyguard:ok projections ref.students, ref.schools (RLS + explicit tenant_id); deleted_at read as the projected source state
	err := q.QueryRowContext(ctx, `SELECT st.school_id, st.grade_level, st.status, st.deleted_at, sc.settings_subset->'enabled_subject_codes'
		FROM ref.students st LEFT JOIN ref.schools sc ON sc.school_id = st.school_id AND sc.tenant_id = st.tenant_id
		WHERE st.student_id = $1 AND st.tenant_id = $2`, student, tenant).Scan(&s.SchoolID, &grade, &s.Status, &deleted, &subjects)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref students: %w", err)
	}
	s.Grade, s.Deleted = int(grade.Int64), deleted.Valid
	if len(subjects) > 0 && string(subjects) != "null" {
		if err := json.Unmarshal(subjects, &s.EnabledSubjects); err != nil {
			s.EnabledSubjects = nil // malformed setting: every subject stays on (never hide learning by accident)
		}
		if s.EnabledSubjects == nil {
			s.EnabledSubjects = []string{}
		}
	}
	return &s, nil
}

// ProgressOf reads every progress fact of a student in tenant (facts, story, unlocks, exploration, mastery, level)
// and computes the chapter counts with chapters (the published chapters).
func ProgressOf(ctx context.Context, q Querier, tenant, student uuid.UUID, chapters []ChapterRow) (*Progress, error) {
	p := &Progress{Facts: world.Facts{Quests: map[string]bool{}, Achievements: map[string]bool{}, Events: map[string]bool{},
		Mastery: map[string]string{}}, Story: map[string]*world.Progress{}, StoryDone: map[string]time.Time{},
		Unlocks: map[string]time.Time{}, Exploration: map[string]*ExplorationRow{}}
	// tenancyguard:ok tenant table game.world_unlock_facts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err := q.QueryContext(ctx, `SELECT fact_kind, fact_ref, fact_value FROM game.world_unlock_facts
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("world facts: %w", err)
	}
	for rows.Next() {
		var kind, ref string
		var value int
		if err := rows.Scan(&kind, &ref, &value); err != nil {
			_ = rows.Close()
			return nil, err
		}
		switch kind {
		case "quest_completed":
			p.Facts.Quests[ref] = true
		case "achievement_unlocked":
			p.Facts.Achievements[ref] = true
		case "event_participation":
			p.Facts.Events[ref] = true
		case "level":
			p.Facts.Level = max(p.Facts.Level, value)
		}
	}
	_ = rows.Close()
	var level sql.NullInt64
	// tenancyguard:ok tenant table game.game_profiles: explicit tenant_id + deleted_at IS NULL (RLS as well)
	if err := q.QueryRowContext(ctx, `SELECT max(level) FROM game.game_profiles WHERE tenant_id = $1 AND student_id = $2
		AND deleted_at IS NULL`, tenant, student).Scan(&level); err != nil {
		return nil, fmt.Errorf("game profile level: %w", err)
	}
	p.Facts.Level = max(p.Facts.Level, int(level.Int64), 1)
	// tenancyguard:ok projection ref.mastery_summary (RLS + explicit tenant_id; no soft-delete, mirrors the source)
	rows, err = q.QueryContext(ctx, `SELECT objective_id, level FROM ref.mastery_summary WHERE tenant_id = $1 AND student_id = $2`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("mastery summary: %w", err)
	}
	for rows.Next() {
		var id uuid.UUID
		var lvl string
		if err := rows.Scan(&id, &lvl); err != nil {
			_ = rows.Close()
			return nil, err
		}
		p.Facts.Mastery[id.String()] = lvl
	}
	_ = rows.Close()
	// tenancyguard:ok tenant table game.story_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err = q.QueryContext(ctx, `SELECT chapter_code, status, completed_quest_ids, completed_at FROM game.story_progress
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("story progress: %w", err)
	}
	for rows.Next() {
		var code string
		var sp world.Progress
		var quests []byte
		var done sql.NullTime
		if err := rows.Scan(&code, &sp.Status, &quests, &done); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = json.Unmarshal(quests, &sp.CompletedQuests)
		p.Story[code] = &sp
		if done.Valid {
			p.StoryDone[code] = done.Time.UTC()
		}
	}
	_ = rows.Close()
	for _, c := range chapters {
		if sp := p.Story[c.Chapter.Code]; sp != nil && sp.Status == world.ChapterCompleted {
			p.Facts.Chapters++
			if c.Chapter.IsSubjectChapter() {
				p.Facts.SubjectChapters++
			}
		}
	}
	// tenancyguard:ok tenant table game.world_area_unlocks: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err = q.QueryContext(ctx, `SELECT area_id, unlocked_at FROM game.world_area_unlocks WHERE tenant_id = $1 AND student_id = $2
		AND deleted_at IS NULL`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("area unlocks: %w", err)
	}
	for rows.Next() {
		var area string
		var at time.Time
		if err := rows.Scan(&area, &at); err != nil {
			_ = rows.Close()
			return nil, err
		}
		p.Unlocks[area] = at.UTC()
	}
	_ = rows.Close()
	// tenancyguard:ok tenant table game.exploration_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err = q.QueryContext(ctx, `SELECT area_id, discoveries, collectibles, completion_pct, completed_at, updated_at
		FROM game.exploration_progress WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("exploration progress: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var area string
		var d, c []byte
		var r ExplorationRow
		var done sql.NullTime
		if err := rows.Scan(&area, &d, &c, &r.Pct, &done, &r.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(d, &r.Exploration.Discoveries)
		_ = json.Unmarshal(c, &r.Exploration.Collectibles)
		if done.Valid {
			at := done.Time.UTC()
			r.CompletedAt, r.Exploration.Completed = &at, true
		}
		p.Exploration[area] = &r
	}
	return p, rows.Err()
}

// Content implements port.WorldContent on the transaction of ctx (read session or write transaction).
type Content struct{}

var _ port.WorldContent = Content{}

func txOf(ctx context.Context) (Querier, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// CurrentAreas implements port.WorldContent.
func (Content) CurrentAreas(ctx context.Context) ([]AreaRow, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return CurrentAreas(ctx, q)
}

// CurrentArea implements port.WorldContent.
func (Content) CurrentArea(ctx context.Context, areaID string) (*AreaRow, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return CurrentArea(ctx, q, areaID)
}

// AreaVersion implements port.WorldContent.
func (Content) AreaVersion(ctx context.Context, versionID uuid.UUID) (*AreaRow, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return AreaVersion(ctx, q, versionID)
}

// CurrentChapters implements port.WorldContent.
func (Content) CurrentChapters(ctx context.Context) ([]ChapterRow, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return CurrentChapters(ctx, q)
}

// CurrentLevels implements port.WorldContent.
func (Content) CurrentLevels(ctx context.Context) ([]LevelRow, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return CurrentLevels(ctx, q)
}

// CurrentLevel implements port.WorldContent.
func (Content) CurrentLevel(ctx context.Context, levelID uuid.UUID) (*LevelRow, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return CurrentLevel(ctx, q, levelID)
}

// LevelOfArea implements port.WorldContent.
func (Content) LevelOfArea(ctx context.Context, areaID string) (*LevelRow, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return LevelOfArea(ctx, q, areaID)
}

// CurrentDialogues implements port.WorldContent.
func (Content) CurrentDialogues(ctx context.Context, npcID string) ([]DialogueRow, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return CurrentDialogues(ctx, q, npcID)
}

// Student implements port.WorldContent.
func (Content) Student(ctx context.Context, tenant, student uuid.UUID) (*Student, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return StudentOf(ctx, q, tenant, student)
}

// Progress implements port.WorldContent.
func (Content) Progress(ctx context.Context, tenant, student uuid.UUID, chapters []ChapterRow) (*Progress, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return ProgressOf(ctx, q, tenant, student, chapters)
}
