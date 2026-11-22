package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// RankingRead implements read.RankingReader (READ ONLY transaction of ctx).
type RankingRead struct{ RankingStore }

var _ read.RankingReader = RankingRead{}

// Rules implements read.RankingReader.
func (RankingRead) Rules(ctx context.Context, at time.Time) (ranking.Rules, error) {
	r, _, err := RankingStore{}.Rules(ctx, at)
	return r, err
}

// Seasons implements read.RankingReader (newest first).
func (RankingRead) Seasons(ctx context.Context, limit, offset int) ([]write.SeasonRow, error) {
	// tenancyguard:ok platform G table seasons, live rows only
	return seasonList(ctx, `SELECT `+seasonCols+` FROM competition.seasons WHERE deleted_at IS NULL ORDER BY starts_at DESC, season_id
		LIMIT $1 OFFSET $2`, limit, offset)
}

// MyRatings implements read.RankingReader.
func (RankingRead) MyRatings(ctx context.Context, tenant, student, season uuid.UUID) ([]write.RatingRow, error) {
	// tenancyguard:ok ratings of the session tenant (RLS + explicit tenant_id) of the caller only, live rows
	return ratingList(ctx, `SELECT `+ratingCols+` FROM competition.ratings WHERE tenant_id = $1 AND student_id = $2 AND season_id = $3
		AND deleted_at IS NULL ORDER BY mode, subject_id`, tenant, student, season)
}

// PeakTiers implements read.RankingReader.
func (RankingRead) PeakTiers(ctx context.Context, tenant, student uuid.UUID, seasons []uuid.UUID) (map[uuid.UUID][]string, error) {
	out := map[uuid.UUID][]string{}
	if len(seasons) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok ratings of the session tenant (RLS + explicit tenant_id) of the caller only, live rows
	rows, err := t.QueryContext(ctx, `SELECT season_id, peak_tier_code FROM competition.ratings WHERE tenant_id = $1 AND student_id = $2
		AND season_id = ANY($3::uuid[]) AND peak_tier_code IS NOT NULL AND deleted_at IS NULL`, tenant, student, uuidArray(seasons))
	if err != nil {
		return nil, fmt.Errorf("peak tiers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var s uuid.UUID
		var code string
		if err := rows.Scan(&s, &code); err != nil {
			return nil, err
		}
		out[s] = append(out[s], code)
	}
	return out, rows.Err()
}

// SchoolKnown implements read.RankingReader.
func (RankingRead) SchoolKnown(ctx context.Context, tenant, school uuid.UUID) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok projections ref.schools/ref.students of the session tenant (RLS + explicit tenant_id; ref rule 2)
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM ref.schools WHERE school_id = $1 AND tenant_id = $2)
		OR EXISTS (SELECT 1 FROM ref.students WHERE school_id = $1 AND tenant_id = $2)`, school, tenant).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("school known: %w", err)
	}
	return ok, nil
}

// MyEffort implements read.RankingReader.
func (RankingRead) MyEffort(ctx context.Context, tenant, student, school uuid.UUID, grade int, start, end time.Time) (float64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var v float64
	// tenancyguard:ok append-only contributions of the session tenant (RLS + explicit tenant_id, TA) of the caller only
	err = t.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0)::float8 FROM competition.leaderboard_contributions WHERE tenant_id = $1
		AND student_id = $2 AND school_id = $3 AND grade_level = $4 AND metric = 'effort_points' AND occurred_at >= $5 AND occurred_at < $6`,
		tenant, student, school, grade, start, end).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("my effort: %w", err)
	}
	return v, nil
}

// Friends implements read.RankingReader (ref.friendships of the tenant, most recent first).
func (RankingRead) Friends(ctx context.Context, tenant, student uuid.UUID, limit int) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.friendships of the session tenant, active rows (no deleted_at: rule 2)
	rows, err := t.QueryContext(ctx, `SELECT CASE WHEN user_low_id = $2 THEN user_high_id ELSE user_low_id END FROM ref.friendships
		WHERE tenant_id = $1 AND active AND (user_low_id = $2 OR user_high_id = $2) ORDER BY source_version DESC LIMIT $3`, tenant, student, limit)
	if err != nil {
		return nil, fmt.Errorf("friends: %w", err)
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

// Cards implements read.RankingReader: nickname, level and avatar only (R16).
func (RankingRead) Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error) {
	return PartyDirectory{}.Cards(ctx, tenant, students)
}

// Excluded implements read.RankingReader.
func (RankingRead) Excluded(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, at time.Time) (map[uuid.UUID]bool, error) {
	out, err := RankingStore{}.Restricted(ctx, tenant, students, at)
	if err != nil || len(students) == 0 {
		return out, err
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.students of the session tenant; deleted_at/status are the projected source state
	rows, err := t.QueryContext(ctx, `SELECT u.id FROM unnest($2::uuid[]) AS u(id) WHERE NOT EXISTS (SELECT 1 FROM ref.students s
		WHERE s.student_id = u.id AND s.tenant_id = $1 AND s.status = 'active' AND s.deleted_at IS NULL)`, tenant, uuidArray(students))
	if err != nil {
		return nil, fmt.Errorf("inactive students: %w", err)
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

// Tiers implements read.RankingReader.
func (RankingRead) Tiers(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, mode string, subject, season uuid.UUID) (map[uuid.UUID]*string, error) {
	out := map[uuid.UUID]*string{}
	if len(students) == 0 {
		return out, nil
	}
	// tenancyguard:ok ratings of the session tenant (RLS + explicit tenant_id), live rows only
	rs, err := ratingList(ctx, `SELECT `+ratingCols+` FROM competition.ratings WHERE tenant_id = $1 AND student_id = ANY($2::uuid[])
		AND mode = $3 AND subject_id = $4 AND season_id = $5 AND deleted_at IS NULL`, tenant, uuidArray(students), mode, subject, season)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		out[r.Student] = r.Tier
	}
	return out, nil
}

const snapshotCols = `tenant_id, leaderboard_id, scope_id, school_id, grade_level, period_key, student_id, rank, value::float8, target::float8,
	reached, members, captured_at`

func scanSnapshot(s interface{ Scan(...any) error }) (write.SnapshotRow, error) {
	var r write.SnapshotRow
	var school, student uuid.NullUUID
	var grade, rank, members sql.NullInt64
	var target sql.NullFloat64
	var reached sql.NullBool
	if err := s.Scan(&r.Tenant, &r.Board, &r.Scope, &school, &grade, &r.Period, &student, &rank, &r.Value, &target, &reached, &members,
		&r.Captured); err != nil {
		return r, err
	}
	if school.Valid {
		r.School = &school.UUID
	}
	if student.Valid {
		r.Student = &student.UUID
	}
	for _, p := range []struct {
		n   sql.NullInt64
		dst **int
	}{{grade, &r.Grade}, {rank, &r.Rank}, {members, &r.Members}} {
		if p.n.Valid {
			v := int(p.n.Int64)
			*p.dst = &v
		}
	}
	if target.Valid {
		r.Target = &target.Float64
	}
	if reached.Valid {
		r.Reached = &reached.Bool
	}
	return r, nil
}

// Snapshots implements read.RankingReader: closed periods newest first, each with its top 100 and collective row.
func (q RankingRead) Snapshots(ctx context.Context, tenant, board uuid.UUID, scope string, limit, offset int) ([]read.SnapshotPeriod, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only snapshots of the session tenant (RLS + explicit tenant_id, TA)
	rows, err := t.QueryContext(ctx, `SELECT period_key, MAX(captured_at) FROM competition.leaderboard_snapshots WHERE tenant_id = $1
		AND leaderboard_id = $2 AND scope_id = $3 GROUP BY period_key ORDER BY 2 DESC, 1 DESC LIMIT $4 OFFSET $5`, tenant, board, scope, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("snapshot periods: %w", err)
	}
	periods, err := scanPeriods(rows)
	if err != nil {
		return nil, err
	}
	for i := range periods {
		p, err := q.Snapshot(ctx, tenant, board, scope, periods[i].Period)
		if err != nil {
			return nil, err
		}
		if p != nil {
			periods[i].Rows = p.Rows
		}
	}
	return periods, nil
}

func scanPeriods(rows *sql.Rows) ([]read.SnapshotPeriod, error) {
	defer func() { _ = rows.Close() }()
	var periods []read.SnapshotPeriod
	for rows.Next() {
		var p read.SnapshotPeriod
		if err := rows.Scan(&p.Period, &p.Captured); err != nil {
			return nil, err
		}
		periods = append(periods, p)
	}
	return periods, rows.Err()
}

// Snapshot implements read.RankingReader: the collective row and the top 100 of one closed period.
func (RankingRead) Snapshot(ctx context.Context, tenant, board uuid.UUID, scope, period string) (*read.SnapshotPeriod, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only snapshots of the session tenant (RLS + explicit tenant_id, TA)
	rows, err := t.QueryContext(ctx, `SELECT `+snapshotCols+` FROM competition.leaderboard_snapshots WHERE tenant_id = $1 AND leaderboard_id = $2
		AND scope_id = $3 AND period_key = $4 AND (rank IS NULL OR rank <= 100) ORDER BY rank NULLS FIRST LIMIT 101`, tenant, board, scope, period)
	if err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out *read.SnapshotPeriod
	for rows.Next() {
		r, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = &read.SnapshotPeriod{Period: period, Captured: r.Captured}
		}
		out.Rows = append(out.Rows, r)
	}
	return out, rows.Err()
}

// SnapshotRank returns the snapshot row of one member (its rank beyond the top 100).
func (RankingRead) SnapshotRank(ctx context.Context, tenant, board uuid.UUID, scope, period string, student uuid.UUID) (*write.SnapshotRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only snapshots of the session tenant (RLS + explicit tenant_id, TA) of the caller only
	r, err := scanSnapshot(t.QueryRowContext(ctx, `SELECT `+snapshotCols+` FROM competition.leaderboard_snapshots WHERE tenant_id = $1
		AND leaderboard_id = $2 AND scope_id = $3 AND period_key = $4 AND student_id = $5`, tenant, board, scope, period, student))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("snapshot rank: %w", err)
	}
	return &r, nil
}

// RatingReader implements port.RatingReader of matchmaking (F104) on the Game Ratings of the active season (F106 T014):
// a student without a rating there gets the default rating of the matchmaking configuration.
type RatingReader struct{}

var _ port.RatingReader = RatingReader{}

// Ratings implements port.RatingReader (inside the matchmaking transaction).
func (RatingReader) Ratings(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, mode string, subject uuid.UUID, def mm.Rating) (map[uuid.UUID]mm.Rating, error) {
	out := make(map[uuid.UUID]mm.Rating, len(students))
	for _, s := range students {
		out[s] = def
	}
	if len(students) == 0 {
		return out, nil
	}
	season, err := RankingStore{}.ActiveSeason(ctx)
	if err != nil || season == nil {
		return out, err
	}
	// tenancyguard:ok ratings of the session tenant (RLS + explicit tenant_id), live rows only
	rs, err := ratingList(ctx, `SELECT `+ratingCols+` FROM competition.ratings WHERE tenant_id = $1 AND student_id = ANY($2::uuid[])
		AND mode = $3 AND subject_id = $4 AND season_id = $5 AND deleted_at IS NULL`, tenant, uuidArray(students), mode, subject, season.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		out[r.Student] = mm.Rating{Mu: r.Rating.R, RD: r.Rating.RD}
	}
	return out, nil
}
