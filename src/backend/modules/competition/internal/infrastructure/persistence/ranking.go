package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/points"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Configuration types read by F106 (competition.game_configs.config_type, code default).
const (
	RankingRulesType           = "ranking-rules"
	LeaderboardDefinitionsType = "leaderboard-definitions"
	defaultCode                = "default"
)

// ErrNoRankingRules: no published Ranking Rules in effect (the migration seeds one).
var ErrNoRankingRules = errors.New("ranking: no published ranking rules in effect")

// RankingStore implements write.RankingRepository, write.LeaderboardRepository, write.SeasonRepository and
// read.RankingReader on eduworld_competition (F106). Every statement runs in the transaction of ctx (tenant RLS, or
// SystemPlatformScope for the jobs listing ids across tenants).
type RankingStore struct{}

var (
	_ write.RankingRepository     = RankingStore{}
	_ write.LeaderboardRepository = RankingStore{}
	_ write.SeasonRepository      = RankingStore{}
)

func dayArg(d time.Time) string { return d.UTC().Format("2006-01-02") }

// published returns the published configuration version of (type, default) in effect at at.
func published(ctx context.Context, typ string, at time.Time) (uuid.UUID, []byte, error) {
	t, err := tx(ctx)
	if err != nil {
		return uuid.Nil, nil, err
	}
	var id uuid.UUID
	var payload []byte
	// tenancyguard:ok platform configuration (G tables game_configs/game_config_versions, no tenant), live rows only
	err = t.QueryRowContext(ctx, `SELECT v.config_version_id, v.payload FROM competition.game_configs c
		JOIN competition.game_config_versions v ON v.config_id = c.config_id
		WHERE c.config_type = $1 AND c.code = $2 AND c.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'
			AND (v.effective_from IS NULL OR v.effective_from <= $3)
		ORDER BY v.effective_from DESC NULLS LAST, v.version_no DESC LIMIT 1`, typ, defaultCode, at).Scan(&id, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, nil, ErrNoRankingRules
	}
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("%s config: %w", typ, err)
	}
	return id, payload, nil
}

// Rules implements write.RankingRepository: the Ranking Rules in effect at at (effective_from: only matches after it,
// edge case "Ranking Rules mới giữa mùa").
func (RankingStore) Rules(ctx context.Context, at time.Time) (ranking.Rules, uuid.UUID, error) {
	id, payload, err := published(ctx, RankingRulesType, at)
	if err != nil {
		return ranking.Rules{}, uuid.Nil, err
	}
	r, err := ranking.ParseRules(payload)
	if err != nil {
		return ranking.Rules{}, uuid.Nil, err
	}
	return r, id, nil
}

const seasonCols = `season_id, code, name, theme_code, starts_at, ends_at, status, started_at, ended_at`

func scanSeason(s interface{ Scan(...any) error }) (write.SeasonRow, error) {
	var r write.SeasonRow
	var theme sql.NullString
	var started, ended sql.NullTime
	if err := s.Scan(&r.ID, &r.Code, &r.Name, &theme, &r.StartsAt, &r.EndsAt, &r.Status, &started, &ended); err != nil {
		return r, err
	}
	if theme.Valid {
		v := theme.String
		r.Theme = &v
	}
	if started.Valid {
		v := started.Time
		r.StartedAt = &v
	}
	if ended.Valid {
		v := ended.Time
		r.EndedAt = &v
	}
	return r, nil
}

func oneSeason(ctx context.Context, q string, args ...any) (*write.SeasonRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	r, err := scanSeason(t.QueryRowContext(ctx, q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("season: %w", err)
	}
	return &r, nil
}

// SeasonAt implements write.RankingRepository (edge case: a match is rated in the season containing its end).
func (RankingStore) SeasonAt(ctx context.Context, at time.Time) (*write.SeasonRow, error) {
	// tenancyguard:ok platform G table seasons, live rows only
	s, err := oneSeason(ctx, `SELECT `+seasonCols+` FROM competition.seasons WHERE deleted_at IS NULL AND status IN ('active','ended')
		AND starts_at <= $1 AND $1 < COALESCE(ended_at, ends_at) ORDER BY starts_at DESC LIMIT 1`, at)
	if err != nil || s != nil {
		return s, err
	}
	return RankingStore{}.ActiveSeason(ctx)
}

// ActiveSeason implements read.RankingReader.
func (RankingStore) ActiveSeason(ctx context.Context) (*write.SeasonRow, error) {
	// tenancyguard:ok platform G table seasons, live rows only
	return oneSeason(ctx, `SELECT `+seasonCols+` FROM competition.seasons WHERE deleted_at IS NULL AND status = 'active'
		ORDER BY starts_at DESC LIMIT 1`)
}

// Season implements read.RankingReader.
func (RankingStore) Season(ctx context.Context, id uuid.UUID) (*write.SeasonRow, error) {
	// tenancyguard:ok platform G table seasons, live rows only
	return oneSeason(ctx, `SELECT `+seasonCols+` FROM competition.seasons WHERE season_id = $1 AND deleted_at IS NULL`, id)
}

// Seasons implements write.SeasonRepository (oldest first).
func (RankingStore) Seasons(ctx context.Context) ([]write.SeasonRow, error) {
	// tenancyguard:ok platform G table seasons, live rows only
	return seasonList(ctx, `SELECT `+seasonCols+` FROM competition.seasons WHERE deleted_at IS NULL ORDER BY starts_at, season_id LIMIT 500`)
}

func seasonList(ctx context.Context, q string, args ...any) ([]write.SeasonRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("seasons: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.SeasonRow
	for rows.Next() {
		s, err := scanSeason(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

const ratingCols = `rating_id, tenant_id, school_id, student_id, mode, subject_id, season_id, grade_level, rating::float8, rd::float8,
	volatility::float8, games_played, tier_code, peak_tier_code, last_played_at, rule_version_id, version`

func scanRating(s interface{ Scan(...any) error }) (write.RatingRow, error) {
	var r write.RatingRow
	var tier, peak sql.NullString
	var last sql.NullTime
	if err := s.Scan(&r.ID, &r.Tenant, &r.School, &r.Student, &r.Mode, &r.Subject, &r.Season, &r.Grade, &r.Rating.R, &r.Rating.RD,
		&r.Rating.Sigma, &r.Games, &tier, &peak, &last, &r.RuleVersion, &r.Version); err != nil {
		return r, err
	}
	if tier.Valid {
		v := tier.String
		r.Tier = &v
	}
	if peak.Valid {
		v := peak.String
		r.Peak = &v
	}
	if last.Valid {
		v := last.Time
		r.LastPlayed = &v
	}
	return r, nil
}

func ratingList(ctx context.Context, q string, args ...any) ([]write.RatingRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ratings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.RatingRow
	for rows.Next() {
		r, err := scanRating(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LockRatings implements write.RankingRepository.
func (RankingStore) LockRatings(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, mode string, subject, season uuid.UUID) (map[uuid.UUID]write.RatingRow, error) {
	// tenancyguard:ok ratings of the event tenant (RLS + explicit tenant_id), live rows only, locked for the update
	rs, err := ratingList(ctx, `SELECT `+ratingCols+` FROM competition.ratings WHERE tenant_id = $1 AND student_id = ANY($2::uuid[])
		AND mode = $3 AND subject_id = $4 AND season_id = $5 AND deleted_at IS NULL ORDER BY student_id FOR UPDATE`,
		tenant, uuidArray(students), mode, subject, season)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]write.RatingRow, len(rs))
	for _, r := range rs {
		out[r.Student] = r
	}
	return out, nil
}

// PreviousRating implements write.RankingRepository.
func (RankingStore) PreviousRating(ctx context.Context, tenant, student uuid.UUID, mode string, subject uuid.UUID, before time.Time) (*write.RatingRow, error) {
	// tenancyguard:ok ratings of the event tenant, live rows only; seasons is a platform G table
	rs, err := ratingList(ctx, `SELECT r.rating_id, r.tenant_id, r.school_id, r.student_id, r.mode, r.subject_id, r.season_id, r.grade_level,
		r.rating::float8, r.rd::float8, r.volatility::float8, r.games_played, r.tier_code, r.peak_tier_code, r.last_played_at, r.rule_version_id,
		r.version FROM competition.ratings r JOIN competition.seasons s ON s.season_id = r.season_id
		WHERE r.tenant_id = $1 AND r.student_id = $2 AND r.mode = $3 AND r.subject_id = $4 AND r.deleted_at IS NULL AND s.starts_at < $5
		ORDER BY s.starts_at DESC LIMIT 1`, tenant, student, mode, subject, before)
	if err != nil || len(rs) == 0 {
		return nil, err
	}
	return &rs[0], nil
}

// SaveRating implements write.RankingRepository (optimistic version on update).
func (RankingStore) SaveRating(ctx context.Context, r write.RatingRow) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	if r.Version == 0 {
		// tenancyguard:ok insert of a rating of the event tenant (RLS WITH CHECK)
		_, err = t.ExecContext(ctx, `INSERT INTO competition.ratings (rating_id, tenant_id, school_id, student_id, mode, subject_id, season_id,
			grade_level, rating, rd, volatility, games_played, tier_code, peak_tier_code, last_played_at, rule_version_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`, ids.New(), r.Tenant, r.School, r.Student, r.Mode,
			r.Subject, r.Season, r.Grade, r.Rating.R, r.Rating.RD, r.Rating.Sigma, r.Games, r.Tier, r.Peak, r.LastPlayed, r.RuleVersion)
		if err != nil {
			return fmt.Errorf("insert rating: %w", err)
		}
		return nil
	}
	// tenancyguard:ok update of a locked rating of the event tenant, live row, optimistic version
	res, err := t.ExecContext(ctx, `UPDATE competition.ratings SET school_id = $3, grade_level = $4, rating = $5, rd = $6, volatility = $7,
		games_played = $8, tier_code = $9, peak_tier_code = $10, last_played_at = $11, rule_version_id = $12, updated_at = now(),
		version = version + 1 WHERE rating_id = $1 AND tenant_id = $2 AND version = $13 AND deleted_at IS NULL`, r.ID, r.Tenant, r.School,
		r.Grade, r.Rating.R, r.Rating.RD, r.Rating.Sigma, r.Games, r.Tier, r.Peak, r.LastPlayed, r.RuleVersion, r.Version)
	if err != nil {
		return fmt.Errorf("update rating: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("update rating %s: version conflict", r.ID)
	}
	return nil
}

func ratingJSON(r write.RatingRow) string {
	b, _ := json.Marshal(map[string]any{"rating": r.Rating.R, "rd": r.Rating.RD, "volatility": r.Rating.Sigma, "tier_code": r.Tier,
		"games_played": r.Games})
	return string(b)
}

// InsertRatingChange implements write.RankingRepository.
func (RankingStore) InsertRatingChange(ctx context.Context, match uuid.UUID, before, after write.RatingRow) (bool, error) {
	// tenancyguard:ok append-only rating_changes of the event tenant (RLS WITH CHECK); unique (match_id, student_id)
	return exec(ctx, `INSERT INTO competition.rating_changes (change_id, tenant_id, match_id, student_id, school_id, mode, subject_id,
		season_id, before, after, rule_version_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11)
		ON CONFLICT (match_id, student_id) DO NOTHING`, ids.New(), after.Tenant, match, after.Student, after.School, after.Mode, after.Subject,
		after.Season, ratingJSON(before), ratingJSON(after), after.RuleVersion)
}

// BeforeTiers implements write.RankingRepository.
func (RankingStore) BeforeTiers(ctx context.Context, match uuid.UUID) (map[uuid.UUID]*string, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only rating_changes under the event TenantScope (RLS), no deleted_at (TA)
	rows, err := t.QueryContext(ctx, `SELECT student_id, before->>'tier_code' FROM competition.rating_changes WHERE match_id = $1
		AND tenant_id = infra.current_tenant_id()`, match)
	if err != nil {
		return nil, fmt.Errorf("rating changes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]*string{}
	for rows.Next() {
		var id uuid.UUID
		var tier sql.NullString
		if err := rows.Scan(&id, &tier); err != nil {
			return nil, err
		}
		if tier.Valid {
			v := tier.String
			out[id] = &v
		} else {
			out[id] = nil
		}
	}
	return out, rows.Err()
}

// MatchTopic implements write.RankingRepository.
func (RankingStore) MatchTopic(ctx context.Context, match uuid.UUID) (*uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var topic uuid.NullUUID
	// tenancyguard:ok competition.matches is G* (reached by its id from the event of the match), live rows only
	err = t.QueryRowContext(ctx, `SELECT topic_id FROM competition.matches WHERE match_id = $1 AND deleted_at IS NULL`, match).Scan(&topic)
	if errors.Is(err, sql.ErrNoRows) || !topic.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("match topic: %w", err)
	}
	return &topic.UUID, nil
}

// PointDay implements write.RankingRepository.
func (RankingStore) PointDay(ctx context.Context, tenant, student uuid.UUID, day time.Time) (points.Day, error) {
	t, err := tx(ctx)
	if err != nil {
		return points.Day{}, err
	}
	var d points.Day
	// tenancyguard:ok append-only ledger of the event tenant (RLS + explicit tenant_id), no deleted_at (TA)
	err = t.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0), COUNT(*) FILTER (WHERE source_type = 'match')
		FROM competition.competition_point_ledger WHERE tenant_id = $1 AND student_id = $2 AND day_key = $3::date`, tenant, student,
		dayArg(day)).Scan(&d.Total, &d.Matches)
	if err != nil {
		return points.Day{}, fmt.Errorf("point day: %w", err)
	}
	return d, nil
}

// InsertPoints implements write.RankingRepository.
func (RankingStore) InsertPoints(ctx context.Context, e write.PointEntry) (bool, error) {
	b, _ := json.Marshal(e.Award.Breakdown)
	// tenancyguard:ok append-only ledger of the event tenant (RLS WITH CHECK); unique (source_type, source_id, student_id)
	return exec(ctx, `INSERT INTO competition.competition_point_ledger (entry_id, tenant_id, school_id, student_id, grade_level, season_id,
		subject_id, topic_id, mode, amount, source_type, source_id, reason, breakdown, rule_version_id, day_key, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15, $16::date, $17)
		ON CONFLICT (source_type, source_id, student_id) DO NOTHING`, ids.New(), e.Tenant, e.School, e.Student, e.Grade, e.Season,
		e.Subject, e.Topic, e.Mode, e.Award.Amount, e.SourceType, e.SourceID, e.Award.Reason, string(b), e.RuleVersion, dayArg(e.Day), e.At)
}

// SeasonPoints implements write.RankingRepository.
func (RankingStore) SeasonPoints(ctx context.Context, tenant, student, season uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok append-only ledger of the event tenant (RLS + explicit tenant_id), no deleted_at (TA)
	err = t.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM competition.competition_point_ledger WHERE tenant_id = $1
		AND student_id = $2 AND season_id = $3`, tenant, student, season).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("season points: %w", err)
	}
	return max(n, 0), nil
}

// Student implements write.RankingRepository and read.RankingReader.
func (RankingStore) Student(ctx context.Context, tenant, student uuid.UUID) (*write.StudentRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var s write.StudentRef
	var grade sql.NullInt16
	var status string
	var deleted sql.NullTime
	// tenancyguard:ok projection ref.students of the session tenant; deleted_at is the projected source state (checked below)
	err = t.QueryRowContext(ctx, `SELECT school_id, grade_level, status, deleted_at FROM ref.students WHERE student_id = $1 AND tenant_id = $2`,
		student, tenant).Scan(&s.School, &grade, &status, &deleted)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !grade.Valid) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref students: %w", err)
	}
	s.Grade = int(grade.Int16)
	s.Active = status == "active" && !deleted.Valid
	return &s, nil
}

// EffortDay implements write.RankingRepository.
func (RankingStore) EffortDay(ctx context.Context, tenant, student uuid.UUID, day time.Time) (ldb.EffortDay, error) {
	t, err := tx(ctx)
	if err != nil {
		return ldb.EffortDay{}, err
	}
	var d ldb.EffortDay
	// tenancyguard:ok append-only contributions of the event tenant (RLS + explicit tenant_id), no deleted_at (TA)
	err = t.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE source_type IN ('activity_completed','review_activity') AND amount > 0),
		COALESCE(SUM(amount), 0), bool_or(source_type = 'active_day') IS TRUE FROM competition.leaderboard_contributions
		WHERE tenant_id = $1 AND student_id = $2 AND metric = 'effort_points' AND day_key = $3::date`, tenant, student, dayArg(day)).
		Scan(&d.Activities, &d.Total, &d.Active)
	if err != nil {
		return ldb.EffortDay{}, fmt.Errorf("effort day: %w", err)
	}
	return d, nil
}

// InsertContribution implements write.RankingRepository.
func (RankingStore) InsertContribution(ctx context.Context, c write.Contribution) (bool, error) {
	// tenancyguard:ok append-only contributions of the event tenant (RLS WITH CHECK); unique (metric, source_type, source_id, student_id)
	return exec(ctx, `INSERT INTO competition.leaderboard_contributions (contribution_id, tenant_id, school_id, student_id, grade_level,
		metric, amount, source_type, source_id, day_key, rule_version_id, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::date, $11, $12) ON CONFLICT (metric, source_type, source_id, student_id) DO NOTHING`,
		ids.New(), c.Tenant, c.School, c.Student, c.Grade, c.Metric, c.Amount, c.SourceType, c.SourceID, dayArg(c.Day), c.RuleVersion, c.At)
}

// SchoolPolicy implements write.RankingRepository and read.RankingReader (ref.schools.settings_subset; a school not
// projected yet keeps the defaults: nothing disabled, no cross-school listing).
func (RankingStore) SchoolPolicy(ctx context.Context, tenant, school uuid.UUID) (write.SchoolPolicy, error) {
	t, err := tx(ctx)
	if err != nil {
		return write.SchoolPolicy{}, err
	}
	var raw []byte
	// tenancyguard:ok projection ref.schools of the session tenant (key = school_id, no deleted_at: rule 2)
	err = t.QueryRowContext(ctx, `SELECT settings_subset FROM ref.schools WHERE school_id = $1 AND tenant_id = $2`, school, tenant).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return write.SchoolPolicy{}, nil
	}
	if err != nil {
		return write.SchoolPolicy{}, fmt.Errorf("ref schools: %w", err)
	}
	var s struct {
		Disabled []string `json:"disabled_leaderboard_definitions"`
		Enabled  *bool    `json:"school_leaderboard_enabled"`
		Cross    *bool    `json:"allow_cross_school_listing"`
	}
	_ = json.Unmarshal(raw, &s)
	return write.SchoolPolicy{Disabled: s.Disabled, SchoolBoardsOff: s.Enabled != nil && !*s.Enabled, CrossSchoolAllow: s.Cross != nil && *s.Cross}, nil
}

// Restricted implements write.RankingRepository.
func (RankingStore) Restricted(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, at time.Time) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(students) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok restrictions of the session tenant (RLS + explicit tenant_id), live rows only
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT student_id FROM competition.competition_restrictions WHERE tenant_id = $1
		AND student_id = ANY($2::uuid[]) AND kind = 'restrict_competition' AND starts_at <= $3 AND ends_at > $3 AND deleted_at IS NULL`,
		tenant, uuidArray(students), at)
	if err != nil {
		return nil, fmt.Errorf("restrictions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
