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
)

// BattleRead implements read.BattleReader with explicit SQL under the read session: a match (G*) is reached only
// through a participant row of the session tenant; deleted rows are filtered (R27).
type BattleRead struct{}

var _ read.BattleReader = BattleRead{}

const matchColumns = `m.match_id, m.mode, m.matchmaking_type, m.subject_id, m.grade_level, m.status, m.ranked, m.is_bot, m.room_id,
	m.started_at, m.ended_at, m.end_reason, r.rated, m.question_plan_id, COALESCE(jsonb_array_length(m.question_plan->'items'), 0),
	r.summary, m.created_at`

func scanMatch(s scanner) (read.MatchRow, error) {
	var m read.MatchRow
	var room, plan uuid.NullUUID
	var started, ended sql.NullTime
	var reason sql.NullString
	var rated sql.NullBool
	var summary []byte
	err := s.Scan(&m.ID, &m.Mode, &m.Type, &m.SubjectID, &m.Grade, &m.Status, &m.Ranked, &m.IsBot, &room, &started, &ended, &reason,
		&rated, &plan, &m.QuestionCount, &summary, &m.CreatedAt)
	if err != nil {
		return m, err
	}
	m.RoomID, m.PlanID = nullID(room), nullID(plan)
	if started.Valid {
		t := started.Time.UTC()
		m.StartedAt = &t
	}
	if ended.Valid {
		t := ended.Time.UTC()
		m.EndedAt = &t
	}
	if reason.Valid {
		r := reason.String
		m.EndReason = &r
	}
	if rated.Valid {
		r := rated.Bool
		m.Rated = &r
	}
	m.Summary = summary
	m.CreatedAt = m.CreatedAt.UTC()
	return m, nil
}

func (BattleRead) players(ctx context.Context, t querier, rows []read.MatchRow) error {
	for i := range rows {
		ps, err := playersOf(ctx, t, rows[i].ID)
		if err != nil {
			return err
		}
		rows[i].Players = ps
	}
	return nil
}

func playersOf(ctx context.Context, t querier, match uuid.UUID) ([]read.MatchPlayer, error) {
	// tenancyguard:ok participants of the session tenant (RLS T + infra.current_tenant_id()), live rows
	rs, err := t.QueryContext(ctx, `SELECT student_id, tenant_id, school_id, team_no, outcome, score, correct_count
		FROM competition.match_participants WHERE match_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL
		ORDER BY team_no, created_at, student_id`, match)
	if err != nil {
		return nil, fmt.Errorf("match participants: %w", err)
	}
	defer func() { _ = rs.Close() }()
	var out []read.MatchPlayer
	for rs.Next() {
		var p read.MatchPlayer
		var outcome sql.NullString
		var score sql.NullFloat64
		var correct sql.NullInt32
		if err := rs.Scan(&p.StudentID, &p.TenantID, &p.SchoolID, &p.TeamNo, &outcome, &score, &correct); err != nil {
			return nil, err
		}
		if outcome.Valid {
			o := outcome.String
			p.Outcome = &o
		}
		if score.Valid {
			sc := score.Float64
			p.Score = &sc
		}
		if correct.Valid {
			c := int(correct.Int32)
			p.Correct = &c
		}
		out = append(out, p)
	}
	return out, rs.Err()
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Match implements read.BattleReader.
func (b BattleRead) Match(ctx context.Context, id uuid.UUID) (*read.MatchRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok competition.matches/match_results are G*: reached through a participant row of the session tenant, live rows
	m, err := scanMatch(t.QueryRowContext(ctx, `SELECT `+matchColumns+` FROM competition.matches m
		LEFT JOIN competition.match_results r ON r.match_id = m.match_id AND r.deleted_at IS NULL
		WHERE m.match_id = $1 AND m.deleted_at IS NULL AND EXISTS (SELECT 1 FROM competition.match_participants p
			WHERE p.match_id = m.match_id AND p.tenant_id = infra.current_tenant_id() AND p.deleted_at IS NULL)`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("match: %w", err)
	}
	rows := []read.MatchRow{m}
	if err := b.players(ctx, t, rows); err != nil {
		return nil, err
	}
	return &rows[0], nil
}

// MyMatches implements read.BattleReader.
func (b BattleRead) MyMatches(ctx context.Context, student uuid.UUID, mode string, before *time.Time, beforeID uuid.UUID, limit int) ([]read.MatchRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var cursor any
	if before != nil {
		cursor = *before
	}
	// tenancyguard:ok the participant rows of the student in the session tenant (RLS T + infra.current_tenant_id()); G* matches joined, live rows
	rs, err := t.QueryContext(ctx, `SELECT `+matchColumns+` FROM competition.match_participants p
		JOIN competition.matches m ON m.match_id = p.match_id AND m.deleted_at IS NULL
		LEFT JOIN competition.match_results r ON r.match_id = m.match_id AND r.deleted_at IS NULL
		WHERE p.tenant_id = infra.current_tenant_id() AND p.student_id = $1 AND p.deleted_at IS NULL
			AND ($2 = '' OR m.mode = $2) AND ($3::timestamptz IS NULL OR (m.created_at, m.match_id) < ($3::timestamptz, $4::uuid))
		ORDER BY m.created_at DESC, m.match_id DESC LIMIT $5`, student, mode, cursor, beforeID, limit)
	if err != nil {
		return nil, fmt.Errorf("my matches: %w", err)
	}
	out, err := scanMatches(rs)
	if err != nil {
		return nil, err
	}
	return out, b.players(ctx, t, out)
}

// Cards implements read.BattleReader (ref.player_profiles: nickname, level, avatar only).
func (BattleRead) Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error) {
	return PartyDirectory{}.Cards(ctx, tenant, students)
}

func scanMatches(rs *sql.Rows) ([]read.MatchRow, error) {
	defer func() { _ = rs.Close() }()
	var out []read.MatchRow
	for rs.Next() {
		m, err := scanMatch(rs)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rs.Err()
}
