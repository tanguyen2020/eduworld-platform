package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
)

// Quests implements read.QuestListReadModel: the quest list of a student paged in SQL (F072-AC-01, API_DESIGN §5).
type Quests struct{}

var _ read.QuestListReadModel = Quests{}

// questListSQL is listQuests in SQL. The candidate set is the quests whose latest progress (by started_at,
// quest_progress_id) is active on a readable pinned version, plus the published quests assigned to the tenant for the
// grade. Each assigned quest goes through the SQL form of quest.Decide (visibility: grade, subject unless
// prerequisites.required_subject_enabled = false, availability window, a discovery trigger found) and of quest.CanRepeat
// (repeatability.mode, else per_period when a recurrence applies, else once; per_period: a completion in the current
// daily/weekly period; repeatable: max_completions, then cooldown_hours after the last completion; once: any
// completion), then the status of F072: already completed → completed; a trigger the player cannot accept (auto_daily,
// auto_weekly, assignment, event) → the status of the latest progress, unlisted without one; cooldown → completed;
// else available. The page is ordered by (status rank, code byte-wise, quest_id) and cut by keyset + LIMIT. The
// equality with the domain rules is tested on a fixture matrix (tests/integration/quest_keyset_test.go).
//
// $1 tenant, $2 student, $3 grade, $4 enabled subjects (NULL = every subject), $5 discovered interactables, $6 now,
// $7 daily key, $8 weekly key, $9 quest id, $10 quest type, $11 status, $12 area, $13..$15 keyset, $16 limit.
const questListSQL = `WITH p AS (
		SELECT quest_progress_id, quest_id, quest_version_id, status, period_key, started_at, completed_at FROM game.quest_progress
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL),
	latest AS (SELECT DISTINCT ON (quest_id) quest_id, quest_version_id, status FROM p
		ORDER BY quest_id, started_at DESC, quest_progress_id DESC),
	hist AS (SELECT quest_id, count(*) FILTER (WHERE status = 'completed') AS completed,
			max(completed_at) FILTER (WHERE status = 'completed') AS last_completed,
			COALESCE(bool_or(status = 'completed' AND period_key = $7), false) AS done_daily,
			COALESCE(bool_or(status = 'completed' AND period_key = $8), false) AS done_weekly
		FROM p GROUP BY quest_id),
	active AS (SELECT l.quest_id, l.quest_version_id FROM latest l
		JOIN game.quest_versions v ON v.quest_version_id = l.quest_version_id AND v.deleted_at IS NULL AND v.status IN ('published', 'archived')
		WHERE l.status = 'active'),
	assigned AS (SELECT d.quest_id, v.quest_version_id, v.start_trigger,
			CASE WHEN COALESCE(v.payload->'availability'->>'recurrence', '') NOT IN ('', 'none') THEN v.payload->'availability'->>'recurrence'
				WHEN v.quest_type = 'daily' OR v.start_trigger = 'auto_daily' THEN 'daily'
				WHEN v.quest_type = 'weekly' OR v.start_trigger = 'auto_weekly' THEN 'weekly' ELSE 'none' END AS recurrence,
			NULLIF(v.payload->'repeatability'->>'mode', '') AS mode,
			COALESCE((v.payload->'repeatability'->>'max_completions')::int, 0) AS max_completions,
			COALESCE((v.payload->'repeatability'->>'cooldown_hours')::int, 0) AS cooldown_hours
		FROM game.quest_definitions d
		JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id
		JOIN infra.tenant_resource_assignments a ON a.resource_type = 'quest' AND a.resource_id = d.quest_id AND a.tenant_id = $1
			AND a.deleted_at IS NULL
		WHERE d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published' AND v.grade_levels @> jsonb_build_array($3::int)
			AND NOT EXISTS (SELECT 1 FROM active x WHERE x.quest_id = d.quest_id)
			AND (NOT COALESCE((v.payload->'prerequisites'->>'required_subject_enabled')::boolean, true)
				OR COALESCE(v.subject_code, '') = '' OR $4::text[] IS NULL OR v.subject_code = ANY($4::text[]))
			AND (v.available_from IS NULL OR v.available_from <= $6) AND (v.available_to IS NULL OR $6 < v.available_to)
			AND (v.start_trigger <> 'discovery' OR COALESCE(v.payload->'start_trigger'->'interactable_ids' ?| $5::text[], false))),
	repeat AS (SELECT a.quest_id, a.quest_version_id, a.start_trigger, l.status AS latest_status,
			CASE COALESCE(a.mode, CASE WHEN a.recurrence <> 'none' THEN 'per_period' ELSE 'once' END)
				WHEN 'per_period' THEN CASE WHEN (a.recurrence = 'daily' AND COALESCE(h.done_daily, false))
					OR (a.recurrence = 'weekly' AND COALESCE(h.done_weekly, false)) THEN 'already_completed' ELSE '' END
				WHEN 'repeatable' THEN CASE WHEN a.max_completions > 0 AND COALESCE(h.completed, 0) >= a.max_completions THEN 'already_completed'
					WHEN h.last_completed IS NOT NULL AND a.cooldown_hours > 0
						AND $6 < h.last_completed + make_interval(hours => a.cooldown_hours) THEN 'cooldown' ELSE '' END
				ELSE CASE WHEN COALESCE(h.completed, 0) > 0 THEN 'already_completed' ELSE '' END END AS rep
		FROM assigned a LEFT JOIN hist h ON h.quest_id = a.quest_id LEFT JOIN latest l ON l.quest_id = a.quest_id),
	cand AS (SELECT x.quest_id, x.quest_version_id, 'active'::text AS status FROM active x
		UNION ALL
		SELECT r.quest_id, r.quest_version_id, CASE WHEN r.rep = 'already_completed' THEN 'completed'
				WHEN r.start_trigger NOT IN ('npc', 'quest_board', 'chain', 'discovery', 'level_reached') THEN r.latest_status
				WHEN r.rep = 'cooldown' THEN 'completed' ELSE 'available' END
		FROM repeat r),
	ranked AS (SELECT c.quest_id, c.quest_version_id, c.status,
			CASE c.status WHEN 'active' THEN 0 WHEN 'available' THEN 1 WHEN 'completed' THEN 2 ELSE 3 END AS rank
		FROM cand c WHERE c.status IS NOT NULL)
	SELECT v.quest_id, v.quest_version_id, v.version_no, v.payload_sha256, v.payload, r.status, r.rank
	FROM ranked r JOIN game.quest_versions v ON v.quest_version_id = r.quest_version_id
	WHERE ($9::uuid IS NULL OR r.quest_id = $9) AND ($10::text IS NULL OR v.quest_type = $10) AND ($11::text IS NULL OR r.status = $11)
		AND ($12::text IS NULL OR v.payload->>'area_id' = $12 OR v.payload->'start_trigger'->>'area_id' = $12
			OR jsonb_path_exists(v.payload, '$.steps[*] ? (@.area_id == $a)', jsonb_build_object('a', $12::text)))
		AND ($13::int IS NULL OR (r.rank, v.code COLLATE "C", r.quest_id) > ($13::int, $14::text COLLATE "C", $15::uuid))
	ORDER BY r.rank, v.code COLLATE "C", r.quest_id
	LIMIT $16`

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Page implements read.QuestListReadModel.
func (Quests) Page(ctx context.Context, q read.QuestListQuery) ([]read.QuestListRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var subjects any
	if q.EnabledSubjects != nil {
		subjects = q.EnabledSubjects
	}
	discovered := q.Discovered
	if discovered == nil {
		discovered = []string{}
	}
	var questID, afterRank, afterCode, afterID any
	if q.QuestID != nil {
		questID = *q.QuestID
	}
	if q.After != nil {
		afterRank, afterCode, afterID = q.After.Rank, q.After.Code, q.After.QuestID
	}
	// tenancyguard:ok game.quest_progress of the session tenant (RLS + explicit tenant_id $1, deleted_at IS NULL); platform
	// quests (G tables, deleted_at filtered) joined with the live assignments of $1 (infra, explicit tenant_id + deleted_at)
	rows, err := t.QueryContext(ctx, questListSQL, q.Tenant, q.Student, q.Grade, subjects, discovered, q.Now, q.DailyKey, q.WeeklyKey, questID,
		nullable(q.QuestType), nullable(q.Status), nullable(q.AreaID), afterRank, afterCode, afterID, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("quest list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.QuestListRow
	for rows.Next() {
		var r read.QuestListRow
		var raw []byte
		if err := rows.Scan(&r.QuestID, &r.VersionID, &r.VersionNo, &r.SHA256, &raw, &r.Status, &r.Rank); err != nil {
			return nil, err
		}
		if r.Version, err = quest.Parse(raw); err != nil {
			return nil, fmt.Errorf("quest payload: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Histories implements read.QuestListReadModel.
func (Quests) Histories(ctx context.Context, tenant, student uuid.UUID, quests []uuid.UUID, periods []string) (map[uuid.UUID]*port.QuestHistory, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return questsql.HistoriesOf(ctx, t, tenant, student, quests, periods)
}

// CompletedQuests implements read.QuestListReadModel.
func (Quests) CompletedQuests(ctx context.Context, tenant, student uuid.UUID) (map[uuid.UUID]bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return questsql.CompletedQuests(ctx, t, tenant, student)
}
