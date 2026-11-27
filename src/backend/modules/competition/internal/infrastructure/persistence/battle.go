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
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Battle implements write.BattleRepository with explicit SQL in the transaction of ctx (F105). competition.matches and
// competition.match_results are G* tables (tenant_ids uuid[]): they are always reached through a match_participants
// row of the tenant of the session (infra.current_tenant_id(), RLS T); every read filters deleted_at (R27).
type Battle struct{}

var _ write.BattleRepository = Battle{}

// BattleMatch implements write.BattleRepository.
func (Battle) BattleMatch(ctx context.Context, id uuid.UUID) (*write.BattleMatch, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	m := write.BattleMatch{}
	var tenants []byte
	var topic, tour, node, plan uuid.NullUUID
	// tenancyguard:ok competition.matches is G*: reached through a participant row of the session tenant (DATABASE_DESIGN §4)
	err = t.QueryRowContext(ctx, `SELECT m.match_id, m.mode, m.matchmaking_type, m.subject_id, m.topic_id, m.grade_level, m.status, m.ranked,
		m.is_bot, m.scope_kind, m.tournament_id, m.tournament_node_id, m.ruleset_version_id, array_to_json(m.tenant_ids), m.question_plan_id
		FROM competition.matches m WHERE m.match_id = $1 AND m.deleted_at IS NULL AND EXISTS (SELECT 1 FROM competition.match_participants p
			WHERE p.match_id = m.match_id AND p.tenant_id = infra.current_tenant_id() AND p.deleted_at IS NULL)`, id).
		Scan(&m.ID, &m.Mode, &m.Type, &m.SubjectID, &topic, &m.Grade, &m.Status, &m.Ranked, &m.IsBot, &m.ScopeKind, &tour, &node,
			&m.RulesetVersionID, &tenants, &plan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("battle match: %w", err)
	}
	_ = json.Unmarshal(tenants, &m.TenantIDs)
	m.TopicID, m.TournamentID, m.TournamentNodeID, m.PlanID = nullID(topic), nullID(tour), nullID(node), nullID(plan)
	// tenancyguard:ok participants of the session tenant (RLS T + infra.current_tenant_id()), live rows
	rows, err := t.QueryContext(ctx, `SELECT student_id, tenant_id, school_id, team_no FROM competition.match_participants
		WHERE match_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY team_no, created_at, student_id`, id)
	if err != nil {
		return nil, fmt.Errorf("battle participants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var p write.BattleParticipant
		if err := rows.Scan(&p.StudentID, &p.TenantID, &p.SchoolID, &p.TeamNo); err != nil {
			return nil, err
		}
		m.Participants = append(m.Participants, p)
	}
	return &m, rows.Err()
}

func nullID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

// ConfigPayload implements write.BattleRepository: a published version of a platform configuration (G, no tenant).
func (Battle) ConfigPayload(ctx context.Context, versionID uuid.UUID) (json.RawMessage, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var payload []byte
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant), live published rows only
	err = t.QueryRowContext(ctx, `SELECT payload FROM competition.game_config_versions WHERE config_version_id = $1 AND deleted_at IS NULL
		AND status IN ('published','unpublished','archived') AND published_at IS NOT NULL`, versionID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // the defaults of CONFIG_DEFAULTS apply
	}
	if err != nil {
		return nil, fmt.Errorf("battle config: %w", err)
	}
	return payload, nil
}

// PinPlan implements write.BattleRepository.
func (Battle) PinPlan(ctx context.Context, match, plan uuid.UUID, version string, items json.RawMessage) error {
	// tenancyguard:ok competition.matches is G*: the match of a participant row of the session tenant, live row
	_, err := exec(ctx, `UPDATE competition.matches m SET question_plan_id = $2, question_plan_version = $3, question_plan = $4::jsonb,
		room_id = $1, updated_at = now(), version = m.version + 1
		WHERE m.match_id = $1 AND m.deleted_at IS NULL AND m.status = 'lobby' AND EXISTS (SELECT 1 FROM competition.match_participants p
			WHERE p.match_id = m.match_id AND p.tenant_id = infra.current_tenant_id() AND p.deleted_at IS NULL)`, match, plan, version, string(items))
	return err
}

// OpenRoom implements write.BattleRepository.
func (Battle) OpenRoom(ctx context.Context, r write.BattleRoomRow) error {
	// tenancyguard:ok tenant_id of the row is the session tenant (RLS tenant_insert WITH CHECK)
	_, err := exec(ctx, `INSERT INTO multiplayer.rooms (room_id, tenant_id, created_at, updated_at, room_type, area_id, school_id, capacity,
		status, match_id) VALUES ($1, $2, $3, $3, $4, $5, $6, $7, 'open', $8) ON CONFLICT (room_id) DO NOTHING`, r.RoomID, r.TenantID, r.At,
		r.RoomType, r.Mode, r.SchoolID, min(max(r.Capacity, 1), 40), r.MatchID)
	return err
}

// MarkRunning implements write.BattleRepository.
func (Battle) MarkRunning(ctx context.Context, match uuid.UUID, at time.Time) error {
	// tenancyguard:ok competition.matches is G*: the match of a participant row of the session tenant, live row
	_, err := exec(ctx, `UPDATE competition.matches m SET status = 'running', started_at = $2, updated_at = now(), version = m.version + 1
		WHERE m.match_id = $1 AND m.deleted_at IS NULL AND m.status = 'lobby' AND EXISTS (SELECT 1 FROM competition.match_participants p
			WHERE p.match_id = m.match_id AND p.tenant_id = infra.current_tenant_id() AND p.deleted_at IS NULL)`, match, at)
	return err
}

// RecordResult implements write.BattleRepository: the result row is inserted once (PK match_id); the participants and
// the match follow in the same transaction.
func (Battle) RecordResult(ctx context.Context, o write.MatchOutcome) error {
	res := o.Result
	summary, err := json.Marshal(o.Summary)
	if err != nil {
		return err
	}
	var winner any
	if res.WinnerTeam != nil {
		winner = *res.WinnerTeam
	}
	// tenancyguard:ok competition.match_results is G* (tenant_ids): written for a match of a participant row of the session tenant
	created, err := exec(ctx, `INSERT INTO competition.match_results (match_id, created_at, updated_at, status, end_reason, winner_team, rated,
		rule_version_id, summary, verified_at, recorded_by, tenant_ids)
		SELECT m.match_id, $2, $2, $3, $4, $5, $6, $7, $8::jsonb, $2, $9, $10::uuid[] FROM competition.matches m
		WHERE m.match_id = $1 AND m.deleted_at IS NULL AND EXISTS (SELECT 1 FROM competition.match_participants p
			WHERE p.match_id = m.match_id AND p.tenant_id = infra.current_tenant_id() AND p.deleted_at IS NULL)
		ON CONFLICT (match_id) DO NOTHING`, res.MatchID, o.At, res.Status, res.EndReason, winner, res.Rated, o.RuleID, string(summary),
		o.RecordedBy, uuidArray(o.TenantIDs))
	if err != nil {
		return fmt.Errorf("match result: %w", err)
	}
	if !created {
		return write.ErrResultRecorded
	}
	for _, p := range res.Players {
		// tenancyguard:ok participant row of the session tenant (RLS T + infra.current_tenant_id()), live row
		if _, err := exec(ctx, `UPDATE competition.match_participants SET outcome = $3, score = $4, correct_count = $5, disconnects = $6,
			connected_ms = $7, updated_at = now(), version = version + 1
			WHERE match_id = $1 AND student_id = $2 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, res.MatchID, p.ID,
			p.Outcome, p.Score, min(p.Correct, 32767), min(p.Disconnects, 32767), p.ConnectedMS); err != nil {
			return fmt.Errorf("match participant result: %w", err)
		}
	}
	status := "completed"
	if res.Status == "cancelled" {
		status = "cancelled"
	}
	// tenancyguard:ok competition.matches is G*: the match of a participant row of the session tenant, live row
	_, err = exec(ctx, `UPDATE competition.matches m SET status = $2, end_reason = $3, ended_at = $4, updated_at = now(), version = m.version + 1
		WHERE m.match_id = $1 AND m.deleted_at IS NULL AND EXISTS (SELECT 1 FROM competition.match_participants p
			WHERE p.match_id = m.match_id AND p.tenant_id = infra.current_tenant_id() AND p.deleted_at IS NULL)`, res.MatchID, status,
		res.EndReason, o.At)
	return err
}

// AbortMatch implements write.BattleRepository.
func (Battle) AbortMatch(ctx context.Context, match uuid.UUID, reason string, at time.Time) (bool, error) {
	// tenancyguard:ok competition.matches is G*: the match of a participant row of the session tenant, live row
	return exec(ctx, `UPDATE competition.matches m SET status = 'aborted', end_reason = $2, ended_at = $3, updated_at = now(), version = m.version + 1
		WHERE m.match_id = $1 AND m.deleted_at IS NULL AND m.status IN ('lobby','running') AND EXISTS (SELECT 1 FROM competition.match_participants p
			WHERE p.match_id = m.match_id AND p.tenant_id = infra.current_tenant_id() AND p.deleted_at IS NULL)`, match, reason, at)
}

// CloseRoom implements write.BattleRepository.
func (Battle) CloseRoom(ctx context.Context, room uuid.UUID, at time.Time) error {
	// tenancyguard:ok room of the session tenant (RLS T + infra.current_tenant_id()), live row
	if _, err := exec(ctx, `UPDATE multiplayer.rooms SET status = 'closed', closed_at = $2, updated_at = now(), version = version + 1
		WHERE room_id = $1 AND tenant_id = infra.current_tenant_id() AND status = 'open' AND deleted_at IS NULL`, room, at); err != nil {
		return err
	}
	// tenancyguard:ok memberships of the session tenant (RLS T + infra.current_tenant_id()), live rows
	_, err := exec(ctx, `UPDATE multiplayer.session_memberships SET left_at = $2, leave_reason = 'room_closed', updated_at = now(),
		version = version + 1 WHERE room_id = $1 AND tenant_id = infra.current_tenant_id() AND left_at IS NULL AND deleted_at IS NULL`, room, at)
	return err
}

// JoinRoom implements write.BattleRepository (one open membership per room and student: a retry is a no-op).
func (Battle) JoinRoom(ctx context.Context, room uuid.UUID, roomType string, match, school, student, session uuid.UUID, at time.Time) error {
	var sess any
	if session != uuid.Nil {
		sess = session
	}
	// tenancyguard:ok tenant_id of the row is the session tenant (RLS tenant_insert WITH CHECK)
	_, err := exec(ctx, `INSERT INTO multiplayer.session_memberships (membership_id, tenant_id, created_at, updated_at, room_id, room_type,
		school_id, student_id, game_session_id, match_id, joined_at, disconnects)
		SELECT $1, infra.current_tenant_id(), $2, $2, $3, $4, $5, $6, $7, $8, $2, 0
		WHERE NOT EXISTS (SELECT 1 FROM multiplayer.session_memberships WHERE room_id = $3 AND student_id = $6 AND tenant_id = infra.current_tenant_id()
			AND left_at IS NULL AND deleted_at IS NULL)`, ids.New(), at, room, roomType, school, student, sess, match)
	return err
}

// LeaveRoom implements write.BattleRepository.
func (Battle) LeaveRoom(ctx context.Context, room, student, session uuid.UUID, reason string, at time.Time) error {
	var sess any
	if session != uuid.Nil {
		sess = session
	}
	// tenancyguard:ok membership of the session tenant (RLS T + infra.current_tenant_id()), live row
	_, err := exec(ctx, `UPDATE multiplayer.session_memberships SET left_at = $4, leave_reason = $5, updated_at = now(), version = version + 1
		WHERE room_id = $1 AND student_id = $2 AND tenant_id = infra.current_tenant_id() AND left_at IS NULL AND deleted_at IS NULL
			AND ($3::uuid IS NULL OR game_session_id = $3::uuid)`, room, student, sess, at, reason)
	return err
}

// Disconnected implements write.BattleRepository.
func (Battle) Disconnected(ctx context.Context, room, student uuid.UUID) error {
	// tenancyguard:ok membership of the session tenant (RLS T + infra.current_tenant_id()), live row
	_, err := exec(ctx, `UPDATE multiplayer.session_memberships SET disconnects = disconnects + 1, updated_at = now(), version = version + 1
		WHERE room_id = $1 AND student_id = $2 AND tenant_id = infra.current_tenant_id() AND left_at IS NULL AND deleted_at IS NULL`, room, student)
	return err
}

// InsertSignal implements write.BattleRepository (append-only).
func (Battle) InsertSignal(ctx context.Context, s write.IntegritySignalRow) error {
	ev, err := json.Marshal(s.Evidence)
	if err != nil {
		return err
	}
	var school any
	if s.SchoolID != uuid.Nil {
		school = s.SchoolID
	}
	// tenancyguard:ok append-only signal of the session tenant (RLS tenant_insert WITH CHECK)
	_, err = exec(ctx, `INSERT INTO competition.integrity_signals (signal_id, tenant_id, school_id, student_id, match_id, room_id, signal_type,
		score, evidence, created_at) VALUES ($1, $2, $3, $4, $5, $5, $6, $7, $8::jsonb, $9)`, ids.New(), s.TenantID, school, s.StudentID, s.MatchID,
		s.Type, s.Score, string(ev), s.At)
	return err
}

// Abandons implements write.BattleRepository: ranked matches the student abandoned since (cancelled/aborted never count).
func (Battle) Abandons(ctx context.Context, student uuid.UUID, since time.Time) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok participant rows of the session tenant (RLS T + infra.current_tenant_id()); matches joined by id, live rows
	err = t.QueryRowContext(ctx, `SELECT count(*) FROM competition.match_participants p JOIN competition.matches m ON m.match_id = p.match_id
		AND m.deleted_at IS NULL WHERE p.tenant_id = infra.current_tenant_id() AND p.student_id = $1 AND p.outcome = 'abandoned'
		AND p.deleted_at IS NULL AND m.ranked AND m.status = 'completed' AND m.ended_at >= $2`, student, since).Scan(&n)
	return n, err
}

// RecentOutcomes implements write.BattleRepository: outcomes of the latest completed matches of the student in the
// session tenant, newest first (no_contest left out; cancelled/aborted matches never count).
func (Battle) RecentOutcomes(ctx context.Context, student uuid.UUID, limit int) ([]string, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	// tenancyguard:ok participant rows of the session tenant (RLS T + infra.current_tenant_id()); matches joined by id, live rows
	rows, err := t.QueryContext(ctx, `SELECT p.outcome FROM competition.match_participants p JOIN competition.matches m ON m.match_id = p.match_id
		AND m.deleted_at IS NULL WHERE p.tenant_id = infra.current_tenant_id() AND p.student_id = $1 AND p.deleted_at IS NULL
		AND p.outcome IS NOT NULL AND p.outcome <> 'no_contest' AND m.status = 'completed'
		ORDER BY m.ended_at DESC NULLS LAST, m.match_id DESC LIMIT $2`, student, limit)
	if err != nil {
		return nil, fmt.Errorf("competition: recent outcomes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var o string
		if err := rows.Scan(&o); err != nil {
			return nil, fmt.Errorf("competition: recent outcomes: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// RestrictedUntil implements write.BattleRepository.
func (Battle) RestrictedUntil(ctx context.Context, student uuid.UUID, kind string, at time.Time) (*time.Time, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var until sql.NullTime
	// tenancyguard:ok restrictions of the session tenant (RLS T + infra.current_tenant_id()), live rows
	err = t.QueryRowContext(ctx, `SELECT max(ends_at) FROM competition.competition_restrictions WHERE tenant_id = infra.current_tenant_id()
		AND student_id = $1 AND kind = $2 AND starts_at <= $3 AND ends_at > $3 AND deleted_at IS NULL`, student, kind, at).Scan(&until)
	if err != nil || !until.Valid {
		return nil, err
	}
	u := until.Time.UTC()
	return &u, nil
}

// InsertRestriction implements write.BattleRepository.
func (Battle) InsertRestriction(ctx context.Context, r write.Restriction) (uuid.UUID, error) {
	id := ids.New()
	// tenancyguard:ok tenant_id of the row is the session tenant (RLS tenant_insert WITH CHECK)
	_, err := exec(ctx, `INSERT INTO competition.competition_restrictions (restriction_id, tenant_id, created_at, updated_at, school_id, student_id,
		kind, reason, starts_at, ends_at) VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $3, $8)`, id, r.TenantID, r.StartsAt, r.SchoolID, r.StudentID,
		r.Kind, r.Reason, r.EndsAt)
	return id, err
}
