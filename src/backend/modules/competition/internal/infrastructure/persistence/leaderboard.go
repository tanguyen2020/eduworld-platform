package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

const definitionCols = `leaderboard_id, code, title, metric, scope_type, period, filters, grade_levels, display, sort_order, status`

func scanDefinition(s interface{ Scan(...any) error }) (ldb.Definition, error) {
	var d ldb.Definition
	var filters, grades, display []byte
	if err := s.Scan(&d.ID, &d.Code, &d.Title, &d.Metric, &d.Scope, &d.Period, &filters, &grades, &display, &d.SortOrder, &d.Status); err != nil {
		return d, err
	}
	_ = json.Unmarshal(filters, &d.Filters)
	_ = json.Unmarshal(grades, &d.Grades)
	var disp struct {
		Target float64 `json:"grade_goal_target"`
	}
	_ = json.Unmarshal(display, &disp)
	d.GradeGoalTarget = disp.Target
	return d, nil
}

// Definitions implements write.LeaderboardRepository, write.RankingRepository and read.RankingReader: every live
// definition (active or retired), in display order.
func (RankingStore) Definitions(ctx context.Context) ([]ldb.Definition, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform G table leaderboard_definitions, live rows only
	rows, err := t.QueryContext(ctx, `SELECT `+definitionCols+` FROM competition.leaderboard_definitions WHERE deleted_at IS NULL
		ORDER BY sort_order, code LIMIT 500`)
	if err != nil {
		return nil, fmt.Errorf("leaderboard definitions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ldb.Definition
	for rows.Next() {
		d, err := scanDefinition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullUUIDPtr(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// Ranking implements write.LeaderboardRepository and read.RankingReader: the board computed from the ledgers of the
// tenant of ctx (competition_point_ledger, leaderboard_contributions or ratings), best first, ties broken by who reached
// the value first. Sanctioned (restrict_competition), inactive or cleaned-up students are out (F106-AC-18); a board
// across schools keeps only the schools allowing allow_cross_school_listing.
func (RankingStore) Ranking(ctx context.Context, q write.BoardQuery) ([]port.BoardMember, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var students any
	if q.Students != nil {
		students = uuidArray(q.Students)
	}
	cross := q.Def.CrossSchool()
	common := `
		AND ($4::uuid IS NULL OR x.school_id = $4) AND ($5::int IS NULL OR x.grade_level = $5)
		AND ($6::uuid[] IS NULL OR x.student_id = ANY($6::uuid[]))
		AND NOT EXISTS (SELECT 1 FROM competition.competition_restrictions r WHERE r.tenant_id = x.tenant_id AND r.student_id = x.student_id
			AND r.kind = 'restrict_competition' AND r.starts_at <= $7 AND r.ends_at > $7 AND r.deleted_at IS NULL)
		AND EXISTS (SELECT 1 FROM ref.students s WHERE s.student_id = x.student_id AND s.tenant_id = x.tenant_id AND s.status = 'active'
			AND s.deleted_at IS NULL)
		AND (NOT $8::boolean OR EXISTS (SELECT 1 FROM ref.schools sc WHERE sc.school_id = x.school_id AND sc.tenant_id = x.tenant_id
			AND (sc.settings_subset->>'allow_cross_school_listing')::boolean IS TRUE))`
	var rows *sql.Rows
	f := q.Def.Filters
	switch q.Def.Metric {
	case ldb.MetricCompetitionPoints:
		// tenancyguard:ok append-only ledger of the session tenant (RLS + explicit tenant_id, TA: no deleted_at); restrictions live rows
		rows, err = t.QueryContext(ctx, `SELECT x.student_id, SUM(x.amount)::float8,
			COALESCE(MAX(x.occurred_at) FILTER (WHERE x.amount <> 0), MIN(x.occurred_at))
			FROM competition.competition_point_ledger x WHERE x.tenant_id = $1 AND x.occurred_at >= $2 AND x.occurred_at < $3`+common+`
			AND ($9::uuid IS NULL OR x.subject_id = $9) AND ($10::uuid IS NULL OR x.topic_id = $10) AND ($11::text IS NULL OR x.mode = $11)
			GROUP BY x.student_id HAVING SUM(x.amount) > 0 ORDER BY 2 DESC, 3, 1`, q.Tenant, q.Start, q.End, nullUUIDPtr(q.School),
			nullInt(q.Grade), students, q.At, cross, nullUUIDPtr(f.SubjectID), nullUUIDPtr(f.TopicID), nullStr(f.Mode))
	case ldb.MetricRating:
		if q.Season == nil || f.Mode == nil || f.SubjectID == nil {
			return nil, nil
		}
		// tenancyguard:ok ratings of the session tenant (RLS + explicit tenant_id), live rows only
		rows, err = t.QueryContext(ctx, `SELECT x.student_id, round(x.rating)::float8, x.updated_at FROM competition.ratings x
			WHERE x.tenant_id = $1 AND x.updated_at >= $2 AND x.updated_at < $3 AND x.deleted_at IS NULL`+common+`
			AND x.season_id = $9 AND x.mode = $10 AND x.subject_id = $11 AND x.games_played >= $12 AND x.rd <= $13
			ORDER BY x.rating DESC, x.updated_at, x.student_id`, q.Tenant, q.Start, q.End, nullUUIDPtr(q.School), nullInt(q.Grade), students,
			q.At, cross, *q.Season, *f.Mode, *f.SubjectID, q.Provisional.MinGames, q.Provisional.MaxRD)
	default:
		// tenancyguard:ok append-only contributions of the session tenant (RLS + explicit tenant_id, TA: no deleted_at)
		rows, err = t.QueryContext(ctx, `SELECT x.student_id, SUM(x.amount)::float8, MAX(x.occurred_at)
			FROM competition.leaderboard_contributions x WHERE x.tenant_id = $1 AND x.occurred_at >= $2 AND x.occurred_at < $3`+common+`
			AND x.metric = $9 GROUP BY x.student_id HAVING SUM(x.amount) > 0 ORDER BY 2 DESC, 3, 1`, q.Tenant, q.Start, q.End,
			nullUUIDPtr(q.School), nullInt(q.Grade), students, q.At, cross, q.Def.Metric)
	}
	if err != nil {
		return nil, fmt.Errorf("leaderboard ranking: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.BoardMember
	for rows.Next() {
		var m port.BoardMember
		var at time.Time
		if err := rows.Scan(&m.Student, &m.Value, &at); err != nil {
			return nil, err
		}
		m.Tick = ldb.Tick(q.Start, at)
		out = append(out, m)
	}
	return out, rows.Err()
}

// Scopes implements write.LeaderboardRepository (SystemPlatformScope: the ids of the scopes with data only).
func (RankingStore) Scopes(ctx context.Context, d ldb.Definition, start, end time.Time) ([]write.ScopeRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	switch d.Metric {
	case ldb.MetricCompetitionPoints:
		// tenancyguard:ok job leaderboard-period-close under SystemPlatformScope: distinct scope ids of every tenant (TA ledger)
		rows, err = t.QueryContext(ctx, `SELECT DISTINCT tenant_id, school_id, grade_level FROM competition.competition_point_ledger
			WHERE occurred_at >= $1 AND occurred_at < $2 LIMIT 100000`, start, end)
	case ldb.MetricRating:
		// tenancyguard:ok job leaderboard-period-close under SystemPlatformScope: distinct scope ids of every tenant, live rows
		rows, err = t.QueryContext(ctx, `SELECT DISTINCT tenant_id, school_id, grade_level FROM competition.ratings
			WHERE updated_at >= $1 AND updated_at < $2 AND deleted_at IS NULL LIMIT 100000`, start, end)
	default:
		metric := d.Metric
		if metric == ldb.MetricGradeGoal {
			metric = ldb.MetricEffortPoints
		}
		// tenancyguard:ok job leaderboard-period-close under SystemPlatformScope: distinct scope ids of every tenant (TA contributions)
		rows, err = t.QueryContext(ctx, `SELECT DISTINCT tenant_id, school_id, grade_level FROM competition.leaderboard_contributions
			WHERE metric = $3 AND occurred_at >= $1 AND occurred_at < $2 LIMIT 100000`, start, end, metric)
	}
	if err != nil {
		return nil, fmt.Errorf("leaderboard scopes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ScopeRef
	for rows.Next() {
		var s write.ScopeRef
		if err := rows.Scan(&s.Tenant, &s.School, &s.Grade); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GradeGoal implements write.LeaderboardRepository and read.RankingReader: the effort credited to one grade of one
// school (grade at the time it was recorded, ADR-0095) and the number of students who contributed.
func (RankingStore) GradeGoal(ctx context.Context, tenant, school uuid.UUID, grade int, start, end time.Time) (float64, int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, 0, err
	}
	var cur float64
	var n int
	// tenancyguard:ok append-only contributions of the session tenant (RLS + explicit tenant_id, TA: no deleted_at)
	err = t.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0)::float8, COUNT(DISTINCT student_id) FILTER (WHERE amount > 0)
		FROM competition.leaderboard_contributions WHERE tenant_id = $1 AND school_id = $2 AND grade_level = $3 AND metric = 'effort_points'
		AND occurred_at >= $4 AND occurred_at < $5`, tenant, school, grade, start, end).Scan(&cur, &n)
	if err != nil {
		return 0, 0, fmt.Errorf("grade goal: %w", err)
	}
	return cur, n, nil
}

// SnapshotTaken implements write.LeaderboardRepository.
func (RankingStore) SnapshotTaken(ctx context.Context, board uuid.UUID, scope, period string) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok append-only snapshots under the TenantScope of the job (RLS), no deleted_at (TA)
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM competition.leaderboard_snapshots WHERE leaderboard_id = $1 AND scope_id = $2
		AND period_key = $3 AND tenant_id = infra.current_tenant_id())`, board, scope, period).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("snapshot taken: %w", err)
	}
	return ok, nil
}

// InsertSnapshot implements write.LeaderboardRepository.
func (RankingStore) InsertSnapshot(ctx context.Context, rows []write.SnapshotRow) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		// tenancyguard:ok append-only snapshot of the job tenant (RLS WITH CHECK)
		if _, err := t.ExecContext(ctx, `INSERT INTO competition.leaderboard_snapshots (snapshot_id, tenant_id, leaderboard_id, scope_id,
			school_id, grade_level, period_key, student_id, rank, value, target, reached, members, captured_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`, ids.New(), r.Tenant, r.Board, r.Scope, nullUUIDPtr(r.School),
			nullInt(r.Grade), r.Period, nullUUIDPtr(r.Student), nullInt(r.Rank), r.Value, r.Target, r.Reached, nullInt(r.Members), r.Captured); err != nil {
			return fmt.Errorf("insert snapshot: %w", err)
		}
	}
	return nil
}

// PublishedDefinitions implements write.LeaderboardRepository.
func (RankingStore) PublishedDefinitions(ctx context.Context, at time.Time) ([]ldb.Definition, *uuid.UUID, error) {
	id, payload, err := published(ctx, LeaderboardDefinitionsType, at)
	if errors.Is(err, ErrNoRankingRules) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var doc struct {
		Definitions []struct {
			ID      uuid.UUID   `json:"leaderboard_id"`
			Code    string      `json:"code"`
			Title   string      `json:"title"`
			Metric  string      `json:"metric"`
			Scope   string      `json:"scope_type"`
			Period  string      `json:"period"`
			Filters ldb.Filters `json:"filters"`
			Grades  []int       `json:"grade_levels"`
			Display struct {
				Target float64 `json:"grade_goal_target"`
			} `json:"display"`
			SortOrder int    `json:"sort_order"`
			Status    string `json:"status"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return nil, nil, fmt.Errorf("leaderboard-definitions payload: %w", err)
	}
	out := make([]ldb.Definition, 0, len(doc.Definitions))
	for _, d := range doc.Definitions {
		out = append(out, ldb.Definition{ID: d.ID, Code: d.Code, Title: d.Title, Metric: d.Metric, Scope: d.Scope, Period: d.Period,
			Filters: d.Filters, Grades: d.Grades, GradeGoalTarget: d.Display.Target, SortOrder: d.SortOrder, Status: d.Status})
	}
	return out, &id, nil
}

// UpsertDefinition implements write.LeaderboardRepository (SystemPlatformScope, competition_platform).
func (RankingStore) UpsertDefinition(ctx context.Context, d ldb.Definition, version *uuid.UUID) (bool, error) {
	filters, _ := json.Marshal(d.Filters)
	grades, _ := json.Marshal(d.Grades)
	display := "{}"
	if d.GradeGoalTarget > 0 {
		b, _ := json.Marshal(map[string]any{"grade_goal_target": d.GradeGoalTarget})
		display = string(b)
	}
	// tenancyguard:ok platform G table leaderboard_definitions written by the platform job (no tenant); live rows only
	return exec(ctx, `INSERT INTO competition.leaderboard_definitions (leaderboard_id, code, title, metric, scope_type, period, filters,
		grade_levels, display, sort_order, status, config_version_id) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11, $12)
		ON CONFLICT (leaderboard_id) DO UPDATE SET code = EXCLUDED.code, title = EXCLUDED.title, metric = EXCLUDED.metric,
			scope_type = EXCLUDED.scope_type, period = EXCLUDED.period, filters = EXCLUDED.filters, grade_levels = EXCLUDED.grade_levels,
			display = EXCLUDED.display, sort_order = EXCLUDED.sort_order, status = EXCLUDED.status, config_version_id = EXCLUDED.config_version_id,
			updated_at = now(), version = competition.leaderboard_definitions.version + 1
		WHERE competition.leaderboard_definitions.deleted_at IS NULL
			AND competition.leaderboard_definitions.config_version_id IS DISTINCT FROM EXCLUDED.config_version_id`, d.ID, d.Code, d.Title,
		d.Metric, d.Scope, d.Period, string(filters), string(grades), display, d.SortOrder, d.Status, nullUUIDPtr(version))
}

// SetStatus implements write.SeasonRepository (SystemPlatformScope).
func (RankingStore) SetStatus(ctx context.Context, season uuid.UUID, from, to string, at time.Time) (bool, error) {
	// tenancyguard:ok platform G table seasons moved by the season scheduler (no tenant), live rows only
	return exec(ctx, `UPDATE competition.seasons SET status = $3::text, started_at = CASE WHEN $3::text = 'active' THEN $4::timestamptz ELSE started_at END,
		ended_at = CASE WHEN $3::text = 'ended' THEN $4::timestamptz ELSE ended_at END, updated_at = now(), version = version + 1
		WHERE season_id = $1 AND status = $2::text AND deleted_at IS NULL`, season, from, to, at)
}

// TenantsWithRatings implements write.SeasonRepository (SystemPlatformScope: tenant ids only).
func (RankingStore) TenantsWithRatings(ctx context.Context, season uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok season scheduler / rd-inflation under SystemPlatformScope: distinct tenant ids of the ratings, live rows
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT tenant_id FROM competition.ratings WHERE season_id = $1 AND deleted_at IS NULL`, season)
	if err != nil {
		return nil, fmt.Errorf("rating tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// WriteResults implements write.SeasonRepository: season_results from the ratings and the Competition Points of the
// season (tenant transaction; ON CONFLICT: a second run writes nothing).
func (RankingStore) WriteResults(ctx context.Context, tenant, season uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	// tenancyguard:ok season results of the job tenant (RLS + explicit tenant_id); ratings live rows, ledger TA
	res, err := t.ExecContext(ctx, `INSERT INTO competition.season_results (result_id, tenant_id, season_id, student_id, mode, subject_id,
		school_id, final_rating, final_rd, peak_tier_code, games_played, competition_points)
		SELECT gen_random_uuid(), r.tenant_id, r.season_id, r.student_id, r.mode, r.subject_id, r.school_id, r.rating, r.rd, r.peak_tier_code,
			r.games_played, COALESCE((SELECT SUM(l.amount) FROM competition.competition_point_ledger l WHERE l.tenant_id = r.tenant_id
				AND l.season_id = r.season_id AND l.student_id = r.student_id AND l.mode = r.mode AND l.subject_id = r.subject_id), 0)
		FROM competition.ratings r WHERE r.tenant_id = $1 AND r.season_id = $2 AND r.deleted_at IS NULL
		ON CONFLICT (season_id, student_id, mode, subject_id) DO NOTHING`, tenant, season)
	if err != nil {
		return 0, fmt.Errorf("season results: %w", err)
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// SoftReset implements write.SeasonRepository: the ratings of season to from those of season from (tenant transaction,
// idempotent), Competition Points start from 0 (a new season has no ledger).
func (RankingStore) SoftReset(ctx context.Context, tenant, from, to uuid.UUID, rule uuid.UUID, step func(ranking.Rating) ranking.Rating) (int, error) {
	// tenancyguard:ok ratings of the job tenant (RLS + explicit tenant_id), live rows only
	rs, err := ratingList(ctx, `SELECT `+ratingCols+` FROM competition.ratings WHERE tenant_id = $1 AND season_id = $2 AND deleted_at IS NULL
		ORDER BY student_id, mode, subject_id`, tenant, from)
	if err != nil {
		return 0, err
	}
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rs {
		nr := step(r.Rating)
		// tenancyguard:ok insert of the new season rating of the job tenant (RLS WITH CHECK); existing rows kept (idempotent)
		res, err := t.ExecContext(ctx, `INSERT INTO competition.ratings (rating_id, tenant_id, school_id, student_id, mode, subject_id, season_id,
			grade_level, rating, rd, volatility, games_played, rule_version_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 0, $12)
			ON CONFLICT (student_id, mode, subject_id, season_id) WHERE deleted_at IS NULL DO NOTHING`, ids.New(), r.Tenant, r.School, r.Student,
			r.Mode, r.Subject, to, r.Grade, nr.R, nr.RD, nr.Sigma, rule)
		if err != nil {
			return n, fmt.Errorf("soft reset: %w", err)
		}
		if k, _ := res.RowsAffected(); k > 0 {
			n++
		}
	}
	return n, nil
}

// InflateInactive implements write.SeasonRepository: the weekly no-games step for the ratings of season neither played
// nor changed since before (a second run in the same week changes nothing).
func (RankingStore) InflateInactive(ctx context.Context, tenant, season uuid.UUID, before time.Time, step func(ranking.Rating) ranking.Rating) (int, error) {
	// tenancyguard:ok ratings of the job tenant (RLS + explicit tenant_id), live rows only, locked for the update
	rs, err := ratingList(ctx, `SELECT `+ratingCols+` FROM competition.ratings WHERE tenant_id = $1 AND season_id = $2 AND deleted_at IS NULL
		AND COALESCE(last_played_at, created_at) <= $3 AND updated_at <= $3 ORDER BY rating_id LIMIT 5000 FOR UPDATE`, tenant, season, before)
	if err != nil {
		return 0, err
	}
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rs {
		nr := step(r.Rating)
		if nr.RD == r.Rating.RD {
			continue
		}
		// tenancyguard:ok update of a locked rating of the job tenant, live row: RD only, never the rating (no rank decay)
		if _, err := t.ExecContext(ctx, `UPDATE competition.ratings SET rd = $3, updated_at = now(), version = version + 1
			WHERE rating_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, r.ID, tenant, nr.RD); err != nil {
			return n, fmt.Errorf("inflate rd: %w", err)
		}
		n++
	}
	return n, nil
}
