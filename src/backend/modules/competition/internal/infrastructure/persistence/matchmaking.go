package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Matchmaking implements write.MatchRepository, read.MatchmakingReader and write.DuelExpiryRepository on
// eduworld_competition (competition.*, ref.*): every statement runs in the transaction of ctx under its scope with
// explicit tenant_id and deleted_at conditions (R27); RLS is the second layer.
type Matchmaking struct{}

var (
	_ write.MatchRepository      = Matchmaking{}
	_ read.MatchmakingReader     = Matchmaking{}
	_ write.DuelExpiryRepository = Matchmaking{}
)

// MatchmakingConfigType and MatchmakingConfigCode identify the matchmaking configuration in competition.game_configs.
const (
	MatchmakingConfigType = "matchmaking-configs"
	MatchmakingConfigCode = "default"
)

// ErrNoConfig: no published matchmaking configuration is in effect (the migration seeds one).
var ErrNoConfig = errors.New("matchmaking: no published configuration in effect")

// ActiveConfig implements write.MatchRepository and read.MatchmakingReader: the published version with the latest
// effective_from ≤ at (MATCHMAKING §5.1); platform data, no tenant.
func (Matchmaking) ActiveConfig(ctx context.Context, at time.Time) (mm.Config, error) {
	t, err := tx(ctx)
	if err != nil {
		return mm.Config{}, err
	}
	var id uuid.UUID
	var payload []byte
	// tenancyguard:ok platform configuration (G tables game_configs/game_config_versions, no tenant), live rows only
	err = t.QueryRowContext(ctx, `SELECT v.config_version_id, v.payload FROM competition.game_configs c
		JOIN competition.game_config_versions v ON v.config_id = c.config_id
		WHERE c.config_type = $1 AND c.code = $2 AND c.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'
			AND (v.effective_from IS NULL OR v.effective_from <= $3)
		ORDER BY v.effective_from DESC NULLS LAST, v.version_no DESC LIMIT 1`, MatchmakingConfigType, MatchmakingConfigCode, at).
		Scan(&id, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return mm.Config{}, ErrNoConfig
	}
	if err != nil {
		return mm.Config{}, fmt.Errorf("matchmaking config: %w", err)
	}
	cfg, err := mm.Parse(payload)
	if err != nil {
		return mm.Config{}, err
	}
	cfg.VersionID = id
	return cfg, nil
}

// Players implements write.MatchRepository and read.MatchmakingReader: grade, school, status of the student and
// the restrictions in force at now — competition_restrictions (abandon_penalty, restrict_competition) and the
// multiplayer sanctions of ref.sanctions — plus the allowed hours of its school (H5).
func (Matchmaking) Players(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, now time.Time) (map[uuid.UUID]mm.Player, error) {
	out := map[uuid.UUID]mm.Player{}
	if len(ids) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection read under the TenantScope; ref.students.deleted_at mirrors the source (cleanup), ref.sanctions and ref.schools have none (rule 2); competition_restrictions filtered on deleted_at
	rows, err := t.QueryContext(ctx, `SELECT st.student_id, st.school_id, st.grade_level, st.status, st.deleted_at,
		EXISTS (SELECT 1 FROM ref.sanctions x WHERE x.tenant_id = st.tenant_id AND x.user_id = st.student_id AND x.active
			AND (x.until IS NULL OR x.until > $3))
		OR EXISTS (SELECT 1 FROM competition.competition_restrictions r WHERE r.tenant_id = st.tenant_id AND r.student_id = st.student_id
			AND r.deleted_at IS NULL AND r.starts_at <= $3 AND r.ends_at > $3),
		(SELECT sc.settings_subset FROM ref.schools sc WHERE sc.tenant_id = st.tenant_id AND sc.school_id = st.school_id)
		FROM ref.students st WHERE st.tenant_id = $1 AND st.student_id = ANY($2::uuid[])`, tenant, uuidArray(ids), now)
	if err != nil {
		return nil, fmt.Errorf("matchmaking players: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		p := mm.Player{TenantID: tenant}
		var grade sql.NullInt32
		var status string
		var deleted sql.NullTime
		var settings []byte
		if err := rows.Scan(&p.ID, &p.SchoolID, &grade, &status, &deleted, &p.Restrict, &settings); err != nil {
			return nil, err
		}
		if grade.Valid {
			p.Grade = int(grade.Int32)
		}
		p.Active = status == "active" && !deleted.Valid
		if h := mm.ParseAllowedHours(settings); h != nil {
			p.OffHours = !h.Contains(now)
		}
		out[p.ID] = p
	}
	return out, rows.Err()
}

func pairs(ctx context.Context, query string, args ...any) (map[write.Pair]bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("matchmaking pairs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[write.Pair]bool{}
	for rows.Next() {
		var a, b uuid.UUID
		if err := rows.Scan(&a, &b); err != nil {
			return nil, err
		}
		out[write.PairOf(a, b)] = true
	}
	return out, rows.Err()
}

// BlockedPairs implements write.MatchRepository (H6).
func (Matchmaking) BlockedPairs(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) (map[write.Pair]bool, error) {
	if len(ids) < 2 {
		return map[write.Pair]bool{}, nil
	}
	// tenancyguard:ok projection read under the TenantScope; ref.blocks has no deleted_at (rule 2)
	return pairs(ctx, `SELECT blocker_id, blocked_id FROM ref.blocks WHERE tenant_id = $1 AND active
		AND blocker_id = ANY($2::uuid[]) AND blocked_id = ANY($2::uuid[])`, tenant, uuidArray(ids))
}

// RecentPairs implements write.MatchRepository.
func (Matchmaking) RecentPairs(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, since time.Time) (map[write.Pair]bool, error) {
	if len(ids) < 2 {
		return map[write.Pair]bool{}, nil
	}
	return pairs(ctx, `SELECT a.student_id, b.student_id FROM competition.match_participants a
		JOIN competition.match_participants b ON b.match_id = a.match_id AND b.student_id <> a.student_id
			AND b.tenant_id = $1 AND b.deleted_at IS NULL
		WHERE a.tenant_id = $1 AND a.deleted_at IS NULL AND a.created_at >= $3 AND a.student_id = ANY($2::uuid[])
			AND b.student_id = ANY($2::uuid[])`, tenant, uuidArray(ids), since)
}

// CreateMatch implements write.MatchRepository: ON CONFLICT on the match id (= proposal id) makes a retry a no-op.
func (Matchmaking) CreateMatch(ctx context.Context, m write.NewMatch) (bool, error) {
	// tenancyguard:ok competition.matches is G* (tenant_ids uuid[], no tenant_id): reached through match_participants (DATABASE_DESIGN §4)
	created, err := exec(ctx, `INSERT INTO competition.matches (match_id, created_at, updated_at, created_by, mode, matchmaking_type, subject_id,
		topic_id, grade_level, status, ranked, is_bot, scope_kind, tournament_node_id, duel_id, ruleset_version_id, tenant_ids)
		VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8, 'lobby', $9, $10, $11, $12, $13, $14, $15::uuid[])
		ON CONFLICT (match_id) DO NOTHING`, m.ID, m.At, actor(ctx), m.Mode, m.Type, m.SubjectID, m.TopicID, m.Grade, m.Ranked, m.IsBot,
		m.ScopeKind, m.TournamentNodeID, m.DuelID, m.RulesetVersionID, uuidArray(m.TenantIDs))
	if err != nil || !created {
		return false, err
	}
	for _, p := range m.Participants {
		var mu, rd any
		if p.Rating != nil {
			mu, rd = p.Rating.Mu, p.Rating.RD
		}
		if _, err := exec(ctx, `INSERT INTO competition.match_participants (participant_id, tenant_id, created_at, updated_at, created_by,
			match_id, student_id, school_id, team_no, party_id, rating_mu_at_match, rating_rd_at_match)
			VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, ids.New(), p.TenantID, m.At, actor(ctx), m.ID, p.StudentID, p.SchoolID,
			p.TeamNo, p.PartyID, mu, rd); err != nil {
			return false, fmt.Errorf("match participant: %w", err)
		}
	}
	return true, nil
}

// MatchOf implements write.MatchRepository: the match row is read only through a participant row of tenant.
func (Matchmaking) MatchOf(ctx context.Context, tenant, id, student uuid.UUID) (*write.Match, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	m := write.Match{Teams: map[uuid.UUID]int{}}
	// tenancyguard:ok competition.matches is G*: joined through the participant row of the tenant (DATABASE_DESIGN §4)
	err = t.QueryRowContext(ctx, `SELECT m.match_id, m.mode, m.matchmaking_type, m.subject_id, m.grade_level, m.status, m.ranked, m.is_bot,
		m.created_at FROM competition.match_participants p JOIN competition.matches m ON m.match_id = p.match_id AND m.deleted_at IS NULL
		WHERE p.tenant_id = $1 AND p.match_id = $2 AND p.student_id = $3 AND p.deleted_at IS NULL`, tenant, id, student).
		Scan(&m.ID, &m.Mode, &m.Type, &m.SubjectID, &m.Grade, &m.Status, &m.Ranked, &m.IsBot, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("match: %w", err)
	}
	rows, err := t.QueryContext(ctx, `SELECT student_id, team_no FROM competition.match_participants WHERE tenant_id = $1 AND match_id = $2
		AND deleted_at IS NULL`, tenant, id)
	if err != nil {
		return nil, fmt.Errorf("match participants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var s uuid.UUID
		var team int
		if err := rows.Scan(&s, &team); err != nil {
			return nil, err
		}
		m.Teams[s] = team
	}
	return &m, rows.Err()
}

// CreateDuel implements write.MatchRepository.
func (Matchmaking) CreateDuel(ctx context.Context, d mm.Duel) error {
	_, err := exec(ctx, `INSERT INTO competition.duels (duel_id, tenant_id, created_at, updated_at, created_by, school_id, challenger_id,
		opponent_id, subject_id, topic_id, status, expires_at) VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, $9, 'pending', $10)`,
		d.ID, d.TenantID, d.CreatedAt, actor(ctx), d.SchoolID, d.ChallengerID, d.OpponentID, d.SubjectID, d.TopicID, d.ExpiresAt)
	if unique(err, "duels_pending_uq") {
		return write.ErrDuelPending
	}
	return err
}

// Duel implements write.MatchRepository.
func (Matchmaking) Duel(ctx context.Context, tenant, id uuid.UUID, lock bool) (*mm.Duel, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT duel_id, tenant_id, school_id, challenger_id, opponent_id, subject_id, topic_id, status, match_id, expires_at, responded_at,
		created_at FROM competition.duels WHERE tenant_id = $1 AND duel_id = $2 AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	var d mm.Duel
	var topic, match uuid.NullUUID
	var responded sql.NullTime
	err = t.QueryRowContext(ctx, q, tenant, id).Scan(&d.ID, &d.TenantID, &d.SchoolID, &d.ChallengerID, &d.OpponentID, &d.SubjectID, &topic,
		&d.Status, &match, &d.ExpiresAt, &responded, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("duel: %w", err)
	}
	if topic.Valid {
		d.TopicID = &topic.UUID
	}
	if match.Valid {
		d.MatchID = &match.UUID
	}
	if responded.Valid {
		at := responded.Time.UTC()
		d.RespondedAt = &at
	}
	d.ExpiresAt, d.CreatedAt = d.ExpiresAt.UTC(), d.CreatedAt.UTC()
	return &d, nil
}

// SaveDuel implements write.MatchRepository.
func (Matchmaking) SaveDuel(ctx context.Context, d mm.Duel) error {
	_, err := exec(ctx, `UPDATE competition.duels SET status = $3, match_id = $4, responded_at = $5, updated_at = now(), updated_by = $6,
		version = version + 1 WHERE tenant_id = $1 AND duel_id = $2 AND deleted_at IS NULL`, d.TenantID, d.ID, d.Status, d.MatchID,
		d.RespondedAt, actor(ctx))
	return err
}

// DuelsSince implements write.MatchRepository.
func (Matchmaking) DuelsSince(ctx context.Context, tenant, challenger uuid.UUID, since time.Time) ([]time.Time, error) {
	return times(ctx, `SELECT created_at FROM competition.duels WHERE tenant_id = $1 AND challenger_id = $2 AND created_at > $3
		AND deleted_at IS NULL ORDER BY created_at`, tenant, challenger, since)
}

// ExpiredDuels implements write.DuelExpiryRepository (SystemPlatformScope: every tenant, ids only).
func (Matchmaking) ExpiredDuels(ctx context.Context, now time.Time, limit int) ([]write.DuelRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok job duel-expiry under SystemPlatformScope: ids of pending duels of every tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT tenant_id, duel_id FROM competition.duels WHERE status = 'pending' AND expires_at <= $1
		AND deleted_at IS NULL ORDER BY expires_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("expired duels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.DuelRef
	for rows.Next() {
		var r write.DuelRef
		if err := rows.Scan(&r.TenantID, &r.DuelID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// School upserts ref.schools from school.updated (settings_subset merged: keys of the event replace the projected ones).
func (Refs) School(ctx context.Context, school, tenant uuid.UUID, status string, settings []byte, version int64) (bool, error) {
	var s any = "{}"
	if len(settings) > 0 {
		s = string(settings)
	}
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); ref.schools has no deleted_at (rule 2)
	return exec(ctx, `INSERT INTO ref.schools AS sc (school_id, tenant_id, status, settings_subset, source_version)
		VALUES ($1, $2, $3, $4::jsonb, $5)
		ON CONFLICT (school_id) DO UPDATE SET status = $3, settings_subset = sc.settings_subset || $4::jsonb, source_version = $5
		WHERE sc.source_version < $5 AND sc.tenant_id = $2`, school, tenant, status, s, version)
}
