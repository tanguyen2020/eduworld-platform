package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// TournamentStore implements write.TournamentRepository, write.AssignmentRepository, read.TournamentReader and
// read.AssignmentReader on eduworld_competition (F107). Every statement runs in the transaction of ctx: the tenant RLS
// plus an explicit tenant_id and deleted_at condition (R8, R27), the Due* listings and the assignments under
// SystemPlatformScope / CrossTenantScope (competition_platform).
type TournamentStore struct{}

var (
	_ write.TournamentRepository = TournamentStore{}
	_ write.AssignmentRepository = TournamentStore{}
	_ read.TournamentReader      = TournamentRead{}
	_ read.AssignmentReader      = TournamentStore{}
)

// TournamentRead implements read.TournamentReader (no lock, read session) on TournamentStore.
type TournamentRead struct{ TournamentStore }

// Challenge implements read.TournamentReader.
func (r TournamentRead) Challenge(ctx context.Context, tenant, id uuid.UUID) (*write.ChallengeRow, error) {
	return r.TournamentStore.Challenge(ctx, tenant, id, false)
}

// Tournament implements read.TournamentReader.
func (r TournamentRead) Tournament(ctx context.Context, tenant, id uuid.UUID) (*write.TournamentRow, error) {
	return r.TournamentStore.Tournament(ctx, tenant, id, false)
}

// Nodes implements read.TournamentReader.
func (r TournamentRead) Nodes(ctx context.Context, tenant, tournamentID uuid.UUID) ([]*trn.Node, error) {
	return r.TournamentStore.Nodes(ctx, tenant, tournamentID, false)
}

// ErrStaleVersion: an optimistic update lost (the row changed since it was read in this transaction).
var ErrStaleVersion = errors.New("tournament: stale version")

func jsonArg(v any) string {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return "[]"
	}
	return string(b)
}

func jsonObj(v any) string {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return "{}"
	}
	return string(b)
}

func intPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

func strPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

func lockClause(lock bool) string {
	if lock {
		return " FOR UPDATE"
	}
	return ""
}

// ---------- templates ----------

// Template implements write.TournamentRepository: the current published version of a template config and whether it is
// assigned to tenant (infra.tenant_resource_assignments, live rows only).
func (TournamentStore) Template(ctx context.Context, tenant, configID uuid.UUID) (*write.TemplateRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r write.TemplateRow
	var payload []byte
	// tenancyguard:ok platform templates (G tables game_configs/game_config_versions, live rows) + the assignment of the tenant (infra, explicit tenant_id)
	err = t.QueryRowContext(ctx, `SELECT c.config_id, v.config_version_id, v.version_no, c.config_type, c.code, v.payload,
		EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a WHERE a.resource_id = c.config_id AND a.tenant_id = $2
			AND a.resource_type = CASE c.config_type WHEN 'challenge-templates' THEN 'challenge_template' ELSE 'tournament_template' END
			AND a.deleted_at IS NULL)
		FROM competition.game_configs c JOIN competition.game_config_versions v ON v.config_version_id = c.current_version_id
		WHERE c.config_id = $1 AND c.config_type IN ('challenge-templates','tournament-templates') AND c.owner_scope = 'platform'
			AND c.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, configID, tenant).
		Scan(&r.ConfigID, &r.VersionID, &r.VersionNo, &r.ConfigType, &r.Code, &payload, &r.Assigned)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	if r.Template, err = challenge.ParseTemplate(payload); err != nil {
		return nil, err
	}
	return &r, nil
}

// TemplateVersion implements write.TournamentRepository (pinned version, whatever its status now).
func (TournamentStore) TemplateVersion(ctx context.Context, versionID uuid.UUID) (*challenge.Template, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var payload []byte
	// tenancyguard:ok platform template version (G table, pinned id, live row)
	err = t.QueryRowContext(ctx, `SELECT payload FROM competition.game_config_versions WHERE config_version_id = $1 AND deleted_at IS NULL`,
		versionID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("template version: %w", err)
	}
	tpl, err := challenge.ParseTemplate(payload)
	if err != nil {
		return nil, err
	}
	return &tpl, nil
}

// TemplatePage implements read.TournamentReader: published templates assigned to tenant, keyset (code, config_id).
func (TournamentStore) TemplatePage(ctx context.Context, tenant uuid.UUID, grade *int, subject *uuid.UUID, after *read.TemplateKeyset, limit int) ([]write.TemplateRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT c.config_id, v.config_version_id, v.version_no, c.config_type, c.code, v.payload
		FROM competition.game_configs c JOIN competition.game_config_versions v ON v.config_version_id = c.current_version_id
		JOIN infra.tenant_resource_assignments a ON a.resource_id = c.config_id AND a.tenant_id = $1 AND a.deleted_at IS NULL
			AND a.resource_type = CASE c.config_type WHEN 'challenge-templates' THEN 'challenge_template' ELSE 'tournament_template' END
		WHERE c.config_type IN ('challenge-templates','tournament-templates') AND c.deleted_at IS NULL AND v.deleted_at IS NULL
			AND v.status = 'published'`
	args := []any{tenant}
	if grade != nil {
		args = append(args, *grade)
		q += fmt.Sprintf(` AND v.payload->'grade_levels' @> to_jsonb($%d::int)`, len(args))
	}
	if subject != nil {
		args = append(args, subject.String())
		q += fmt.Sprintf(` AND (v.payload->>'subject_id' IS NULL OR v.payload->>'subject_id' = $%d)`, len(args))
	}
	if after != nil {
		args = append(args, after.Code, after.ID)
		q += fmt.Sprintf(` AND (c.code, c.config_id) > ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY c.code, c.config_id LIMIT $%d`, len(args))
	// tenancyguard:ok platform templates joined to the live assignments of the tenant (explicit tenant_id, deleted_at)
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.TemplateRow
	for rows.Next() {
		var r write.TemplateRow
		var payload []byte
		if err := rows.Scan(&r.ConfigID, &r.VersionID, &r.VersionNo, &r.ConfigType, &r.Code, &payload); err != nil {
			return nil, err
		}
		tpl, err := challenge.ParseTemplate(payload)
		if err != nil {
			continue // an invalid template is not offered
		}
		r.Template, r.Assigned = tpl, true
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------- projections ----------

// SchoolActive implements write.TournamentRepository.
func (TournamentStore) SchoolActive(ctx context.Context, tenant, school uuid.UUID) (bool, error) {
	// tenancyguard:ok projection ref.schools of the session tenant (no deleted_at, rule 2)
	return exists(ctx, `SELECT EXISTS (SELECT 1 FROM ref.schools WHERE school_id = $1 AND tenant_id = $2 AND status = 'active')`, school, tenant)
}

// SchoolOpen implements write.TournamentRepository: the allowed hours of the school (settings_subset.allowed_hours,
// OQ-17) contain every instant of at; no hours = always open.
func (TournamentStore) SchoolOpen(ctx context.Context, tenant, school uuid.UUID, at []time.Time) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var settings []byte
	// tenancyguard:ok projection ref.schools of the session tenant (no deleted_at, rule 2)
	err = t.QueryRowContext(ctx, `SELECT settings_subset FROM ref.schools WHERE school_id = $1 AND tenant_id = $2`, school, tenant).Scan(&settings)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("school hours: %w", err)
	}
	h := mm.ParseAllowedHours(settings)
	if h == nil {
		return true, nil
	}
	for _, a := range at {
		if !h.Contains(a) {
			return false, nil
		}
	}
	return true, nil
}

// GradeStudents implements write.TournamentRepository: the active students of grade in school (ref.students, ADR-0095).
func (TournamentStore) GradeStudents(ctx context.Context, tenant, school uuid.UUID, grade int) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.students of the session tenant; deleted_at mirrors the source (cleanup)
	rows, err := t.QueryContext(ctx, `SELECT student_id FROM ref.students WHERE tenant_id = $1 AND school_id = $2 AND grade_level = $3
		AND status = 'active' AND deleted_at IS NULL ORDER BY student_id`, tenant, school, grade)
	if err != nil {
		return nil, fmt.Errorf("grade students: %w", err)
	}
	return scanIDs(rows)
}

func scanIDs(rows *sql.Rows) ([]uuid.UUID, error) {
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

// Students implements write.TournamentRepository on the matchmaking view of the students (grade, school, active,
// restricted, school hours).
func (TournamentStore) Students(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, now time.Time) (map[uuid.UUID]write.StudentInfo, error) {
	ps, err := Matchmaking{}.Players(ctx, tenant, ids, now)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]write.StudentInfo, len(ps))
	for id, p := range ps {
		out[id] = write.StudentInfo{ID: id, School: p.SchoolID, Grade: p.Grade, Active: p.Active, Restricted: p.Restrict, OffHours: p.OffHours}
	}
	return out, nil
}

// Student implements read.TournamentReader.
func (s TournamentStore) Student(ctx context.Context, tenant, id uuid.UUID) (*write.StudentInfo, error) {
	m, err := s.Students(ctx, tenant, []uuid.UUID{id}, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if st, ok := m[id]; ok {
		return &st, nil
	}
	return nil, nil
}

// Cards implements read.TournamentReader (nickname and avatar only, R16).
func (TournamentStore) Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error) {
	return PartyDirectory{}.Cards(ctx, tenant, students)
}

// PartyMembers implements write.TournamentRepository: the leader and the current members of an active party.
func (TournamentStore) PartyMembers(ctx context.Context, tenant, party uuid.UUID) (*uuid.UUID, []uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, nil, err
	}
	// tenancyguard:ok parties and members of the session tenant (RLS + explicit tenant_id), live rows only
	rows, err := t.QueryContext(ctx, `SELECT m.student_id, m.role = 'leader' FROM multiplayer.parties p
		JOIN multiplayer.party_members m ON m.party_id = p.party_id AND m.tenant_id = p.tenant_id AND m.left_at IS NULL AND m.deleted_at IS NULL
		WHERE p.party_id = $1 AND p.tenant_id = $2 AND p.deleted_at IS NULL AND p.status <> 'disbanded' ORDER BY m.joined_at, m.student_id`,
		party, tenant)
	if err != nil {
		return nil, nil, fmt.Errorf("party members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var leader *uuid.UUID
	var members []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var isLeader bool
		if err := rows.Scan(&id, &isLeader); err != nil {
			return nil, nil, err
		}
		if isLeader {
			l := id
			leader = &l
		}
		members = append(members, id)
	}
	return leader, members, rows.Err()
}

// ---------- challenges ----------

const challengeCols = `challenge_id, tenant_id, school_id, template_config_id, template_version_id, kind, format, name, grade_level, targets,
	exclude_student_ids, subject_id, question_set_version_id, params, starts_at, ends_at, join_opens_at, status, tournament_id, match_ids,
	cancel_reason, COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid), created_at, version`

func scanChallenge(s scanner) (write.ChallengeRow, error) {
	var c write.ChallengeRow
	var grade sql.NullInt64
	var targets, exclude, params, matches []byte
	var qs, tour uuid.NullUUID
	var reason sql.NullString
	if err := s.Scan(&c.ID, &c.Tenant, &c.School, &c.TemplateConfigID, &c.TemplateVersionID, &c.Kind, &c.Format, &c.Name, &grade, &targets,
		&exclude, &c.SubjectID, &qs, &params, &c.StartsAt, &c.EndsAt, &c.JoinOpensAt, &c.Status, &tour, &matches, &reason, &c.CreatedBy,
		&c.CreatedAt, &c.Version); err != nil {
		return c, err
	}
	c.Grade, c.QuestionSetVersionID, c.TournamentID, c.CancelReason = intPtr(grade), nullID(qs), nullID(tour), strPtr(reason)
	var ts []struct {
		Type       string      `json:"target_type"`
		StudentIDs []uuid.UUID `json:"student_ids"`
	}
	_ = json.Unmarshal(targets, &ts)
	for _, t := range ts {
		if t.Type == challenge.TargetStudents {
			c.Listed = append(c.Listed, t.StudentIDs...)
		}
	}
	_ = json.Unmarshal(exclude, &c.Exclude)
	_ = json.Unmarshal(params, &c.Params)
	if len(matches) > 0 {
		_ = json.Unmarshal(matches, &c.MatchIDs)
	}
	c.StartsAt, c.EndsAt, c.JoinOpensAt, c.CreatedAt = c.StartsAt.UTC(), c.EndsAt.UTC(), c.JoinOpensAt.UTC(), c.CreatedAt.UTC()
	return c, nil
}

func targetsJSON(c write.ChallengeRow) string {
	var ts []map[string]any
	if c.Grade != nil {
		ts = append(ts, map[string]any{"target_type": challenge.TargetGrade, "grade_level": *c.Grade})
	}
	if len(c.Listed) > 0 {
		ts = append(ts, map[string]any{"target_type": challenge.TargetStudents, "student_ids": c.Listed})
	}
	return jsonArg(ts)
}

// InsertChallenge implements write.TournamentRepository.
func (TournamentStore) InsertChallenge(ctx context.Context, c write.ChallengeRow) error {
	excl := c.Exclude
	if excl == nil {
		excl = []uuid.UUID{}
	}
	// tenancyguard:ok insert of the session tenant (RLS WITH CHECK)
	_, err := exec(ctx, `INSERT INTO competition.challenges (challenge_id, tenant_id, created_at, created_by, updated_at, updated_by, school_id,
		template_config_id, template_version_id, kind, format, name, grade_level, targets, exclude_student_ids, participation, subject_id,
		question_set_version_id, params, starts_at, ends_at, join_opens_at, status)
		VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13::jsonb, 'individual', $14, $15, $16::jsonb, $17, $18, $19, $20)`,
		c.ID, c.Tenant, c.CreatedAt, c.CreatedBy, c.School, c.TemplateConfigID, c.TemplateVersionID, c.Kind, c.Format, c.Name, nullInt(c.Grade),
		targetsJSON(c), jsonArg(excl), c.SubjectID, c.QuestionSetVersionID, jsonObj(c.Params), c.StartsAt, c.EndsAt, c.JoinOpensAt, c.Status)
	if err != nil {
		return fmt.Errorf("insert challenge: %w", err)
	}
	return nil
}

// Challenge implements write.TournamentRepository and read.TournamentReader (live row of tenant).
func (TournamentStore) Challenge(ctx context.Context, tenant, id uuid.UUID, lock bool) (*write.ChallengeRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok challenge of the session tenant (RLS + explicit tenant_id), live row
	c, err := scanChallenge(t.QueryRowContext(ctx, `SELECT `+challengeCols+` FROM competition.challenges WHERE challenge_id = $1 AND tenant_id = $2
		AND deleted_at IS NULL`+lockClause(lock), id, tenant))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("challenge: %w", err)
	}
	return &c, nil
}

// UpdateChallenge implements write.TournamentRepository (optimistic version).
func (TournamentStore) UpdateChallenge(ctx context.Context, c write.ChallengeRow) error {
	var matches any
	if c.MatchIDs != nil {
		matches = jsonArg(c.MatchIDs)
	}
	// tenancyguard:ok update of a live challenge of the session tenant (RLS + explicit tenant_id)
	ok, err := exec(ctx, `UPDATE competition.challenges SET status = $3, tournament_id = $4, match_ids = $5::jsonb, cancel_reason = $6,
		version = version + 1, updated_at = now(), updated_by = $7 WHERE challenge_id = $1 AND tenant_id = $2 AND version = $8 AND deleted_at IS NULL`,
		c.ID, c.Tenant, c.Status, c.TournamentID, matches, c.CancelReason, actor(ctx), c.Version)
	if err != nil {
		return fmt.Errorf("update challenge: %w", err)
	}
	if !ok {
		return ErrStaleVersion
	}
	return nil
}

// ChallengePage implements read.TournamentReader: keyset (starts_at, challenge_id) descending, LIMIT in SQL.
func (TournamentStore) ChallengePage(ctx context.Context, tenant uuid.UUID, f read.ChallengeFilter, after *read.Keyset, limit int) ([]write.ChallengeRow, error) {
	q := `SELECT ` + challengeCols + ` FROM competition.challenges WHERE tenant_id = $1 AND deleted_at IS NULL`
	args := []any{tenant}
	if f.Schools != nil {
		args = append(args, uuidArray(f.Schools))
		q += fmt.Sprintf(` AND school_id = ANY($%d::uuid[])`, len(args))
	}
	if f.Grade != nil {
		args = append(args, *f.Grade)
		q += fmt.Sprintf(` AND grade_level = $%d`, len(args))
	}
	if f.Status != nil {
		args = append(args, *f.Status)
		q += fmt.Sprintf(` AND status = $%d`, len(args))
	}
	if after != nil {
		args = append(args, after.At, after.ID)
		q += fmt.Sprintf(` AND (starts_at, challenge_id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY starts_at DESC, challenge_id DESC LIMIT $%d`, len(args))
	return challengeList(ctx, q, args...)
}

func challengeList(ctx context.Context, q string, args ...any) ([]write.ChallengeRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok challenges of the session tenant (RLS + explicit tenant_id), live rows (built above)
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("challenges: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ChallengeRow
	for rows.Next() {
		c, err := scanChallenge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// audienceSQL is the audience condition of a student ($s student, $sc school, $g grade) on challenge alias c (ADR-0095).
func audienceSQL(s, sc, g int) string {
	return fmt.Sprintf(`c.school_id = $%d AND NOT (c.exclude_student_ids @> jsonb_build_array($%d::text))
		AND (c.grade_level = $%d OR c.targets @> jsonb_build_array(jsonb_build_object('student_ids', jsonb_build_array($%d::text))))`, sc, s, g, s)
}

// MyChallenges implements read.TournamentReader.
func (TournamentStore) MyChallenges(ctx context.Context, tenant uuid.UUID, st read.StudentScope, after *read.Keyset, limit int) ([]write.ChallengeRow, error) {
	args := []any{tenant, st.Student.String(), st.School, st.Grade}
	q := `SELECT ` + strings.ReplaceAll(challengeCols, "COALESCE(created_by", "COALESCE(c.created_by") + ` FROM competition.challenges c
		WHERE c.tenant_id = $1 AND c.deleted_at IS NULL AND c.status IN ('scheduled','running') AND ` + audienceSQL(2, 3, 4)
	if after != nil {
		args = append(args, after.At, after.ID)
		q += fmt.Sprintf(` AND (c.starts_at, c.challenge_id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY c.starts_at DESC, c.challenge_id DESC LIMIT $%d`, len(args))
	return challengeList(ctx, q, args...)
}

// ChallengeParticipants implements read.TournamentReader.
func (TournamentStore) ChallengeParticipants(ctx context.Context, tenant uuid.UUID, cs []write.ChallengeRow) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		var n int
		switch {
		case c.TournamentID != nil:
			// tenancyguard:ok entries of the session tenant (RLS + explicit tenant_id), live rows
			err = t.QueryRowContext(ctx, `SELECT count(*) FROM competition.tournament_entries WHERE tenant_id = $1 AND tournament_id = $2
				AND deleted_at IS NULL AND status NOT IN ('withdrawn','disqualified')`, tenant, *c.TournamentID).Scan(&n)
		case len(c.MatchIDs) > 0:
			// tenancyguard:ok participants of the rooms of the session tenant (RLS + explicit tenant_id), live rows
			err = t.QueryRowContext(ctx, `SELECT count(*) FROM competition.match_participants WHERE tenant_id = $1 AND match_id = ANY($2::uuid[])
				AND deleted_at IS NULL`, tenant, uuidArray(c.MatchIDs)).Scan(&n)
		}
		if err != nil {
			return nil, fmt.Errorf("challenge participants: %w", err)
		}
		out[c.ID] = n
	}
	return out, nil
}

// AudienceSize implements read.TournamentReader: |(grade students ∪ listed students of the school) − excluded|.
func (TournamentStore) AudienceSize(ctx context.Context, c write.ChallengeRow) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	grade := -1
	if c.Grade != nil {
		grade = *c.Grade
	}
	listed := c.Listed
	if listed == nil {
		listed = []uuid.UUID{}
	}
	excl := c.Exclude
	if excl == nil {
		excl = []uuid.UUID{}
	}
	// tenancyguard:ok projection ref.students of the session tenant; deleted_at mirrors the source (cleanup)
	err = t.QueryRowContext(ctx, `SELECT count(*) FROM ref.students WHERE tenant_id = $1 AND school_id = $2 AND status = 'active'
		AND deleted_at IS NULL AND (grade_level = $3 OR student_id = ANY($4::uuid[])) AND NOT (student_id = ANY($5::uuid[]))`,
		c.Tenant, c.School, grade, uuidArray(listed), uuidArray(excl)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("audience size: %w", err)
	}
	return n, nil
}

// ---------- tournaments ----------

const tournamentCols = `tournament_id, tenant_id, school_id, owner_scope, owner_id, challenge_id, template_config_id, rule_version_id, name, format,
	participant_type, team_size, mode, subject_id, topic_ids, grade_levels, registration_open_at, registration_close_at, check_in_opens_at,
	starts_at, max_entries, min_entries, seeding_policy, seed_value, auto_confirm, reward_rule_refs, reward_summary, round_gap_s, status,
	cancel_reason, completed_at, rewarded_at, created_at, version`

func scanTournament(s scanner) (write.TournamentRow, error) {
	var t write.TournamentRow
	var school, ch uuid.NullUUID
	var team, seed sql.NullInt64
	var topics, grades, rewards []byte
	var summary, reason sql.NullString
	var completed, rewarded sql.NullTime
	var gap int64
	if err := s.Scan(&t.ID, &t.Tenant, &school, &t.OwnerScope, &t.OwnerID, &ch, &t.TemplateConfigID, &t.RuleVersionID, &t.Name, &t.Format,
		&t.ParticipantType, &team, &t.Mode, &t.SubjectID, &topics, &grades, &t.RegOpenAt, &t.RegCloseAt, &t.CheckInOpensAt, &t.StartsAt,
		&t.MaxEntries, &t.MinEntries, &t.SeedingPolicy, &seed, &t.AutoConfirm, &rewards, &summary, &gap, &t.Status, &reason, &completed,
		&rewarded, &t.CreatedAt, &t.Version); err != nil {
		return t, err
	}
	t.School, t.ChallengeID, t.TeamSize, t.RewardSummary, t.CancelReason = nullID(school), nullID(ch), intPtr(team), strPtr(summary), strPtr(reason)
	t.CompletedAt, t.RewardedAt = timePtr(completed), timePtr(rewarded)
	if seed.Valid {
		v := seed.Int64
		t.SeedValue = &v
	}
	_ = json.Unmarshal(topics, &t.TopicIDs)
	_ = json.Unmarshal(grades, &t.Grades)
	_ = json.Unmarshal(rewards, &t.RewardRules)
	t.RoundGap = time.Duration(gap) * time.Second
	t.RegOpenAt, t.RegCloseAt, t.CheckInOpensAt, t.StartsAt, t.CreatedAt = t.RegOpenAt.UTC(), t.RegCloseAt.UTC(), t.CheckInOpensAt.UTC(),
		t.StartsAt.UTC(), t.CreatedAt.UTC()
	return t, nil
}

// InsertTournament implements write.TournamentRepository.
func (TournamentStore) InsertTournament(ctx context.Context, t write.TournamentRow) error {
	var topics any
	if t.TopicIDs != nil {
		topics = jsonArg(t.TopicIDs)
	}
	rewards := t.RewardRules
	if rewards == nil {
		rewards = []challenge.RewardRule{}
	}
	// tenancyguard:ok insert of the session tenant (RLS WITH CHECK)
	_, err := exec(ctx, `INSERT INTO competition.tournaments (tournament_id, tenant_id, created_at, created_by, updated_at, updated_by, school_id,
		owner_scope, owner_id, challenge_id, template_config_id, rule_version_id, name, format, participant_type, team_size, mode, subject_id,
		topic_ids, grade_levels, registration_open_at, registration_close_at, check_in_opens_at, starts_at, max_entries, min_entries,
		seeding_policy, auto_confirm, reward_rule_refs, reward_summary, round_gap_s, status)
		VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17::jsonb, $18::jsonb, $19, $20, $21, $22, $23,
			$24, $25, $26, $27::jsonb, $28, $29, $30)`,
		t.ID, t.Tenant, t.CreatedAt, actor(ctx), t.School, t.OwnerScope, t.OwnerID, t.ChallengeID, t.TemplateConfigID, t.RuleVersionID, t.Name,
		t.Format, t.ParticipantType, nullInt(t.TeamSize), t.Mode, t.SubjectID, topics, jsonArg(t.Grades), t.RegOpenAt, t.RegCloseAt,
		t.CheckInOpensAt, t.StartsAt, t.MaxEntries, t.MinEntries, t.SeedingPolicy, t.AutoConfirm, jsonArg(rewards), t.RewardSummary,
		int64(t.RoundGap/time.Second), t.Status)
	if err != nil {
		return fmt.Errorf("insert tournament: %w", err)
	}
	return nil
}

// Tournament implements write.TournamentRepository.
func (TournamentStore) Tournament(ctx context.Context, tenant, id uuid.UUID, lock bool) (*write.TournamentRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tournament of the session tenant (RLS + explicit tenant_id), live row
	r, err := scanTournament(t.QueryRowContext(ctx, `SELECT `+tournamentCols+` FROM competition.tournaments WHERE tournament_id = $1
		AND tenant_id = $2 AND deleted_at IS NULL`+lockClause(lock), id, tenant))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tournament: %w", err)
	}
	return &r, nil
}

// UpdateTournament implements write.TournamentRepository (optimistic version).
func (TournamentStore) UpdateTournament(ctx context.Context, t write.TournamentRow) error {
	// tenancyguard:ok update of a live tournament of the session tenant (RLS + explicit tenant_id)
	ok, err := exec(ctx, `UPDATE competition.tournaments SET status = $3, seed_value = $4, cancel_reason = $5, completed_at = $6, rewarded_at = $7,
		version = version + 1, updated_at = now(), updated_by = $8 WHERE tournament_id = $1 AND tenant_id = $2 AND version = $9 AND deleted_at IS NULL`,
		t.ID, t.Tenant, t.Status, t.SeedValue, t.CancelReason, t.CompletedAt, t.RewardedAt, actor(ctx), t.Version)
	if err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}
	if !ok {
		return ErrStaleVersion
	}
	return nil
}

// TournamentPage implements read.TournamentReader: keyset (starts_at, tournament_id) descending; a student sees the
// tournaments of its school and grade, not drafts, whose challenge audience holds it (or it has an entry); scope mine:
// only those with its entry.
func (TournamentStore) TournamentPage(ctx context.Context, tenant uuid.UUID, f read.TournamentFilter, after *read.Keyset, limit int) ([]write.TournamentRow, error) {
	cols := strings.ReplaceAll(tournamentCols, "\n", " ")
	var sel []string
	for _, c := range strings.Split(cols, ",") {
		sel = append(sel, "t."+strings.TrimSpace(c))
	}
	q := `SELECT ` + strings.Join(sel, ", ") + ` FROM competition.tournaments t WHERE t.tenant_id = $1 AND t.deleted_at IS NULL`
	args := []any{tenant}
	if f.Status != nil {
		args = append(args, *f.Status)
		q += fmt.Sprintf(` AND t.status = $%d`, len(args))
	}
	if f.Schools != nil {
		args = append(args, uuidArray(f.Schools))
		q += fmt.Sprintf(` AND t.school_id = ANY($%d::uuid[])`, len(args))
	}
	if f.School != nil {
		args = append(args, *f.School)
		q += fmt.Sprintf(` AND t.school_id = $%d`, len(args))
	}
	if s := f.Student; s != nil {
		args = append(args, s.Student.String())
		st := len(args)
		mine := fmt.Sprintf(`EXISTS (SELECT 1 FROM competition.tournament_entries e WHERE e.tenant_id = t.tenant_id AND e.tournament_id = t.tournament_id
			AND e.deleted_at IS NULL AND e.status <> 'withdrawn' AND e.member_ids @> jsonb_build_array($%d::text))`, st)
		if !s.Mine {
			args = append(args, s.School, s.Grade) // only bound when used (an unused parameter has no type)
		}
		sc, g := st+1, st+2
		scope := fmt.Sprintf(`(t.status <> 'draft' AND (t.school_id IS NULL OR t.school_id = $%d) AND t.grade_levels @> to_jsonb($%d::int)
			AND (t.challenge_id IS NULL OR EXISTS (SELECT 1 FROM competition.challenges c WHERE c.tenant_id = t.tenant_id
				AND c.challenge_id = t.challenge_id AND c.deleted_at IS NULL AND `+audienceSQL(st, sc, g)+`)))`, sc, g)
		if s.Mine {
			q += ` AND ` + mine
		} else {
			q += ` AND (` + scope + ` OR ` + mine + `)`
		}
	}
	if after != nil {
		args = append(args, after.At, after.ID)
		q += fmt.Sprintf(` AND (t.starts_at, t.tournament_id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY t.starts_at DESC, t.tournament_id DESC LIMIT $%d`, len(args))
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tournaments of the session tenant (RLS + explicit tenant_id), live rows (built above)
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("tournaments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.TournamentRow
	for rows.Next() {
		r, err := scanTournament(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EntryCounts implements read.TournamentReader (entries still in the tournament).
func (TournamentStore) EntryCounts(ctx context.Context, tenant uuid.UUID, tournaments []uuid.UUID) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	if len(tournaments) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok entries of the session tenant (RLS + explicit tenant_id), live rows
	rows, err := t.QueryContext(ctx, `SELECT tournament_id, count(*) FROM competition.tournament_entries WHERE tenant_id = $1
		AND tournament_id = ANY($2::uuid[]) AND deleted_at IS NULL AND status NOT IN ('withdrawn','disqualified') GROUP BY tournament_id`,
		tenant, uuidArray(tournaments))
	if err != nil {
		return nil, fmt.Errorf("entry counts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// ---------- entries ----------

const entryCols = `entry_id, tournament_id, tenant_id, school_id, student_id, party_id, member_ids, seed, group_id, status, registered_by,
	checked_in_at, version`

func scanEntry(s scanner) (write.EntryRow, error) {
	var e write.EntryRow
	var student, party, group uuid.NullUUID
	var members []byte
	var seed sql.NullInt64
	var checked sql.NullTime
	if err := s.Scan(&e.ID, &e.Tournament, &e.Tenant, &e.School, &student, &party, &members, &seed, &group, &e.Status, &e.RegisteredBy,
		&checked, &e.Version); err != nil {
		return e, err
	}
	e.Student, e.Party, e.Group, e.Seed, e.CheckedInAt = nullID(student), nullID(party), nullID(group), intPtr(seed), timePtr(checked)
	_ = json.Unmarshal(members, &e.Members)
	return e, nil
}

func entryList(ctx context.Context, q string, args ...any) ([]write.EntryRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok entries of the session tenant (RLS + explicit tenant_id), live rows (callers)
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("entries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.EntryRow
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Entries implements write.TournamentRepository and read.TournamentReader.
func (TournamentStore) Entries(ctx context.Context, tenant, tournamentID uuid.UUID) ([]write.EntryRow, error) {
	return entryList(ctx, `SELECT `+entryCols+` FROM competition.tournament_entries WHERE tenant_id = $1 AND tournament_id = $2
		AND deleted_at IS NULL ORDER BY created_at, entry_id`, tenant, tournamentID)
}

// Entry implements write.TournamentRepository.
func (TournamentStore) Entry(ctx context.Context, tenant, tournamentID, entryID uuid.UUID, lock bool) (*write.EntryRow, error) {
	es, err := entryList(ctx, `SELECT `+entryCols+` FROM competition.tournament_entries WHERE tenant_id = $1 AND tournament_id = $2
		AND entry_id = $3 AND deleted_at IS NULL`+lockClause(lock), tenant, tournamentID, entryID)
	if err != nil || len(es) == 0 {
		return nil, err
	}
	return &es[0], nil
}

// EntryOfMember implements write.TournamentRepository (an active entry first, else the latest withdrawn one).
func (TournamentStore) EntryOfMember(ctx context.Context, tenant, tournamentID, student uuid.UUID) (*write.EntryRow, error) {
	es, err := entryList(ctx, `SELECT `+entryCols+` FROM competition.tournament_entries WHERE tenant_id = $1 AND tournament_id = $2
		AND member_ids @> jsonb_build_array($3::text) AND deleted_at IS NULL ORDER BY (status = 'withdrawn'), updated_at DESC LIMIT 1 FOR UPDATE`,
		tenant, tournamentID, student.String())
	if err != nil || len(es) == 0 {
		return nil, err
	}
	return &es[0], nil
}

// InsertEntry implements write.TournamentRepository.
func (TournamentStore) InsertEntry(ctx context.Context, e write.EntryRow) error {
	// tenancyguard:ok insert of the session tenant (RLS WITH CHECK)
	_, err := exec(ctx, `INSERT INTO competition.tournament_entries (entry_id, tenant_id, created_at, created_by, updated_at, updated_by,
		tournament_id, school_id, student_id, party_id, member_ids, status, registered_by)
		VALUES ($1, $2, now(), $3, now(), $3, $4, $5, $6, $7, $8::jsonb, $9, $10)`, e.ID, e.Tenant, actor(ctx), e.Tournament, e.School, e.Student,
		e.Party, jsonArg(e.Members), e.Status, e.RegisteredBy)
	if unique(err, "tournament_entries_student_uq", "tournament_entries_party_uq") {
		return problem.New(problem.Conflict, "already registered")
	}
	if err != nil {
		return fmt.Errorf("insert entry: %w", err)
	}
	return nil
}

// UpdateEntry implements write.TournamentRepository (optimistic version).
func (TournamentStore) UpdateEntry(ctx context.Context, e write.EntryRow) error {
	// tenancyguard:ok update of a live entry of the session tenant (RLS + explicit tenant_id)
	ok, err := exec(ctx, `UPDATE competition.tournament_entries SET status = $3, seed = $4, group_id = $5, registered_by = $6, checked_in_at = $7,
		member_ids = $8::jsonb, version = version + 1, updated_at = now(), updated_by = $9
		WHERE entry_id = $1 AND tenant_id = $2 AND version = $10 AND deleted_at IS NULL`,
		e.ID, e.Tenant, e.Status, nullInt(e.Seed), e.Group, e.RegisteredBy, e.CheckedInAt, jsonArg(e.Members), actor(ctx), e.Version)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}
	if !ok {
		return ErrStaleVersion
	}
	return nil
}

// ---------- groups ----------

// Groups implements write.TournamentRepository and read.TournamentReader.
func (TournamentStore) Groups(ctx context.Context, tenant, tournamentID uuid.UUID) ([]write.GroupRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok groups of the session tenant (RLS + explicit tenant_id), live rows
	rows, err := t.QueryContext(ctx, `SELECT group_id, tournament_id, tenant_id, code, standings FROM competition.tournament_groups
		WHERE tenant_id = $1 AND tournament_id = $2 AND deleted_at IS NULL ORDER BY code`, tenant, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("groups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.GroupRow
	for rows.Next() {
		var g write.GroupRow
		var st []byte
		if err := rows.Scan(&g.ID, &g.Tournament, &g.Tenant, &g.Code, &st); err != nil {
			return nil, err
		}
		if len(st) > 0 {
			_ = json.Unmarshal(st, &g.Standings)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// InsertGroup implements write.TournamentRepository (idempotent on the group id).
func (TournamentStore) InsertGroup(ctx context.Context, g write.GroupRow) error {
	// tenancyguard:ok insert of the session tenant (RLS WITH CHECK)
	_, err := exec(ctx, `INSERT INTO competition.tournament_groups (group_id, tenant_id, created_at, created_by, updated_at, updated_by,
		tournament_id, code, standings) VALUES ($1, $2, now(), $3, now(), $3, $4, $5, '[]'::jsonb) ON CONFLICT (group_id) DO NOTHING`,
		g.ID, g.Tenant, actor(ctx), g.Tournament, g.Code)
	if err != nil {
		return fmt.Errorf("insert group: %w", err)
	}
	return nil
}

// UpdateGroupStandings implements write.TournamentRepository.
func (TournamentStore) UpdateGroupStandings(ctx context.Context, g write.GroupRow) error {
	st := g.Standings
	if st == nil {
		st = []trn.Row{}
	}
	// tenancyguard:ok update of a live group of the session tenant (RLS + explicit tenant_id)
	_, err := exec(ctx, `UPDATE competition.tournament_groups SET standings = $3::jsonb, version = version + 1, updated_at = now()
		WHERE group_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, g.ID, g.Tenant, jsonArg(st))
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	return nil
}

// ---------- nodes ----------

const nodeCols = `node_id, bracket, round, position, group_id, entry_a_id, entry_b_id, winner_entry_id, is_draw, result, match_id, scheduled_at,
	started_at, next_win_node_id, next_win_slot, next_lose_node_id, next_lose_slot, wave, attempts, reschedule_count, notices, status,
	confirm_after, decision_required, override_reason, overridden_by, version`

func scanNode(s scanner) (*trn.Node, error) {
	n := &trn.Node{}
	var group, a, b, w, match, nw, nl, by uuid.NullUUID
	var result []byte
	var sched, started, confirm sql.NullTime
	var nws, nls sql.NullInt64
	var reason sql.NullString
	if err := s.Scan(&n.ID, &n.Bracket, &n.Round, &n.Position, &group, &a, &b, &w, &n.Draw, &result, &match, &sched, &started, &nw, &nws, &nl,
		&nls, &n.Wave, &n.Attempts, &n.Reschedules, &n.Notices, &n.Status, &confirm, &n.DecisionRequired, &reason, &by, &n.Version); err != nil {
		return nil, err
	}
	n.GroupID, n.A, n.B, n.Winner, n.MatchID, n.NextWin, n.NextLose, n.OverriddenBy = nullID(group), nullID(a), nullID(b), nullID(w),
		nullID(match), nullID(nw), nullID(nl), nullID(by)
	n.ScheduledAt, n.StartedAt, n.ConfirmAfter, n.OverrideReason = timePtr(sched), timePtr(started), timePtr(confirm), strPtr(reason)
	if nws.Valid {
		n.NextWinSlot = int(nws.Int64)
	}
	if nls.Valid {
		n.NextLoseSlot = int(nls.Int64)
	}
	if len(result) > 0 {
		var r trn.Result
		if json.Unmarshal(result, &r) == nil {
			n.Result = &r
		}
	}
	return n, nil
}

func nodeList(ctx context.Context, q string, args ...any) ([]*trn.Node, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok bracket nodes of the session tenant (RLS + explicit tenant_id), live rows (callers)
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*trn.Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Nodes implements write.TournamentRepository (locked) and read.TournamentReader.
func (TournamentStore) Nodes(ctx context.Context, tenant, tournamentID uuid.UUID, lock bool) ([]*trn.Node, error) {
	return nodeList(ctx, `SELECT `+nodeCols+` FROM competition.bracket_nodes WHERE tenant_id = $1 AND tournament_id = $2 AND deleted_at IS NULL
		ORDER BY wave, bracket, round, position`+lockClause(lock), tenant, tournamentID)
}

func slotArg(id *uuid.UUID, slot int) any {
	if id == nil {
		return nil
	}
	return slot
}

func resultArg(r *trn.Result) any {
	if r == nil {
		return nil
	}
	b, _ := json.Marshal(r)
	return string(b)
}

// InsertNode implements write.TournamentRepository (idempotent on the derived node id).
func (TournamentStore) InsertNode(ctx context.Context, tenant, tournamentID uuid.UUID, n *trn.Node) error {
	// tenancyguard:ok insert of the session tenant (RLS WITH CHECK)
	_, err := exec(ctx, `INSERT INTO competition.bracket_nodes (node_id, tenant_id, created_at, created_by, updated_at, updated_by, tournament_id,
		bracket, round, position, group_id, entry_a_id, entry_b_id, winner_entry_id, is_draw, result, match_id, scheduled_at, started_at,
		next_win_node_id, next_win_slot, next_lose_node_id, next_lose_slot, wave, attempts, reschedule_count, notices, status, confirm_after,
		decision_required)
		VALUES ($1, $2, now(), $3, now(), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23,
			$24, $25, $26, $27) ON CONFLICT (node_id) DO NOTHING`,
		n.ID, tenant, actor(ctx), tournamentID, n.Bracket, n.Round, n.Position, n.GroupID, n.A, n.B, n.Winner, n.Draw, resultArg(n.Result),
		n.MatchID, n.ScheduledAt, n.StartedAt, n.NextWin, slotArg(n.NextWin, n.NextWinSlot), n.NextLose, slotArg(n.NextLose, n.NextLoseSlot),
		n.Wave, n.Attempts, n.Reschedules, n.Notices, n.Status, n.ConfirmAfter, n.DecisionRequired)
	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}
	n.Version = 1
	return nil
}

// UpdateNode implements write.TournamentRepository (optimistic version).
func (TournamentStore) UpdateNode(ctx context.Context, tenant uuid.UUID, n *trn.Node) error {
	// tenancyguard:ok update of a live node of the session tenant (RLS + explicit tenant_id)
	ok, err := exec(ctx, `UPDATE competition.bracket_nodes SET entry_a_id = $3, entry_b_id = $4, winner_entry_id = $5, is_draw = $6,
		result = $7::jsonb, match_id = $8, scheduled_at = $9, started_at = $10, attempts = $11, reschedule_count = $12, notices = $13, status = $14,
		confirm_after = $15, decision_required = $16, override_reason = $17, overridden_by = $18, version = version + 1, updated_at = now(),
		updated_by = $19 WHERE node_id = $1 AND tenant_id = $2 AND version = $20 AND deleted_at IS NULL`,
		n.ID, tenant, n.A, n.B, n.Winner, n.Draw, resultArg(n.Result), n.MatchID, n.ScheduledAt, n.StartedAt, n.Attempts, n.Reschedules, n.Notices,
		n.Status, n.ConfirmAfter, n.DecisionRequired, n.OverrideReason, n.OverriddenBy, actor(ctx), n.Version)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	if !ok {
		return ErrStaleVersion
	}
	return nil
}

// NodeOfMatch implements write.TournamentRepository.
func (TournamentStore) NodeOfMatch(ctx context.Context, tenant, matchID uuid.UUID) (*uuid.UUID, *uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, nil, err
	}
	var tid, nid uuid.UUID
	// tenancyguard:ok node of the session tenant (RLS + explicit tenant_id), live row
	err = t.QueryRowContext(ctx, `SELECT tournament_id, node_id FROM competition.bracket_nodes WHERE tenant_id = $1 AND match_id = $2
		AND deleted_at IS NULL`, tenant, matchID).Scan(&tid, &nid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("node of match: %w", err)
	}
	return &tid, &nid, nil
}

// MatchCreated implements write.TournamentRepository.
func (TournamentStore) MatchCreated(ctx context.Context, matchID uuid.UUID) (bool, error) {
	// tenancyguard:ok competition.matches is G* (no tenant_id): the id is the proposal of a node of the tenant, live row
	return exists(ctx, `SELECT EXISTS (SELECT 1 FROM competition.matches WHERE match_id = $1 AND deleted_at IS NULL)`, matchID)
}

// RoomOf implements write.TournamentRepository.
func (TournamentStore) RoomOf(ctx context.Context, tenant uuid.UUID, matchIDs []uuid.UUID, student uuid.UUID) (*uuid.UUID, error) {
	if len(matchIDs) == 0 {
		return nil, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	// tenancyguard:ok participants of the session tenant (RLS + explicit tenant_id), live rows
	err = t.QueryRowContext(ctx, `SELECT match_id FROM competition.match_participants WHERE tenant_id = $1 AND student_id = $2
		AND match_id = ANY($3::uuid[]) AND deleted_at IS NULL LIMIT 1`, tenant, student, uuidArray(matchIDs)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("room of: %w", err)
	}
	return &id, nil
}

// ---------- results, rewards ----------

// InsertResult implements write.TournamentRepository (once per entry).
func (TournamentStore) InsertResult(ctx context.Context, r write.ResultRow) (bool, error) {
	p := r.Placement
	// tenancyguard:ok append-only results of the session tenant (RLS WITH CHECK); unique (tournament_id, entry_id)
	return exec(ctx, `INSERT INTO competition.tournament_results (result_id, tenant_id, created_at, created_by, tournament_id, entry_id, school_id,
		final_rank, wins, losses, draws, points, matches_played) VALUES ($1, $2, now(), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (tournament_id, entry_id) DO NOTHING`, uuid.New(), r.Tenant, actor(ctx), r.Tournament, r.Entry, r.School, p.Rank, p.Wins,
		p.Losses, p.Draws, p.Points, p.Played)
}

// Results implements read.TournamentReader (final rank by entry).
func (TournamentStore) Results(ctx context.Context, tenant, tournamentID uuid.UUID) (map[uuid.UUID]int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only results of the session tenant (RLS + explicit tenant_id), no deleted_at (TA)
	rows, err := t.QueryContext(ctx, `SELECT entry_id, final_rank FROM competition.tournament_results WHERE tenant_id = $1 AND tournament_id = $2`,
		tenant, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("results: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var r int
		if err := rows.Scan(&id, &r); err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, rows.Err()
}

// InsertReward implements write.TournamentRepository (once per tournament, entry and rule — F107-AC-20).
func (TournamentStore) InsertReward(ctx context.Context, r write.RewardRow) (bool, error) {
	// tenancyguard:ok append-only rewards of the session tenant (RLS WITH CHECK); unique (tournament_id, entry_id, rule_id)
	return exec(ctx, `INSERT INTO competition.tournament_rewards (reward_id, tenant_id, created_at, created_by, tournament_id, entry_id, rule_id,
		student_ids, reward_ref, points, granted_at) VALUES ($1, $2, now(), $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10)
		ON CONFLICT (tournament_id, entry_id, rule_id) DO NOTHING`, uuid.New(), r.Tenant, actor(ctx), r.Tournament, r.Entry, r.RuleID,
		jsonArg(r.Students), jsonObj(r.RewardRef), r.Points, r.At)
}

// ---------- jobs ----------

// DueTournaments implements write.TournamentRepository (SystemPlatformScope: every tenant).
func (TournamentStore) DueTournaments(ctx context.Context, now time.Time, limit int) ([]write.DueRef, error) {
	// tenancyguard:ok job listing across tenants under SystemPlatformScope (competition_platform), live rows only
	return dueRefs(ctx, `SELECT tenant_id, tournament_id FROM competition.tournaments WHERE deleted_at IS NULL AND (
		(status = 'published' AND registration_open_at <= $1) OR (status = 'registration_open' AND registration_close_at <= $1)
		OR (status = 'scheduled' AND starts_at <= $1 + interval '10 minutes') OR status IN ('in_progress','completed'))
		ORDER BY starts_at, tournament_id LIMIT $2`, now, limit)
}

// DueChallenges implements write.TournamentRepository (timed quizzes to open or complete).
func (TournamentStore) DueChallenges(ctx context.Context, now time.Time, limit int) ([]write.DueRef, error) {
	// tenancyguard:ok job listing across tenants under SystemPlatformScope (competition_platform), live rows only
	return dueRefs(ctx, `SELECT tenant_id, challenge_id FROM competition.challenges WHERE deleted_at IS NULL AND format = 'timed_quiz' AND (
		(status = 'scheduled' AND starts_at <= $1) OR (status = 'running' AND ends_at <= $1)) ORDER BY starts_at, challenge_id LIMIT $2`, now, limit)
}

func dueRefs(ctx context.Context, q string, args ...any) ([]write.DueRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok job listing across tenants under SystemPlatformScope (callers above)
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("due: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.DueRef
	for rows.Next() {
		var r write.DueRef
		if err := rows.Scan(&r.Tenant, &r.ID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------- assignments ----------

// TemplateConfig implements write.AssignmentRepository: the config type of a live platform template with a published
// version ("" otherwise).
func (TournamentStore) TemplateConfig(ctx context.Context, configID uuid.UUID) (string, error) {
	t, err := tx(ctx)
	if err != nil {
		return "", err
	}
	var typ string
	// tenancyguard:ok platform template (G table, live row with a live published version)
	err = t.QueryRowContext(ctx, `SELECT c.config_type FROM competition.game_configs c JOIN competition.game_config_versions v
		ON v.config_version_id = c.current_version_id AND v.status = 'published' AND v.deleted_at IS NULL
		WHERE c.config_id = $1 AND c.owner_scope = 'platform' AND c.deleted_at IS NULL
			AND c.config_type IN ('challenge-templates','tournament-templates')`, configID).Scan(&typ)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("template config: %w", err)
	}
	return typ, nil
}

// ActiveTenants implements write.AssignmentRepository (ref.tenants, active).
func (TournamentStore) ActiveTenants(ctx context.Context, tenants []uuid.UUID) (map[uuid.UUID]bool, error) {
	// tenancyguard:ok projection ref.tenants of the requested tenants (CrossTenantScope of exactly them, no deleted_at — rule 2)
	return idSet(ctx, `SELECT tenant_id FROM ref.tenants WHERE tenant_id = ANY($1::uuid[]) AND status = 'active'`, uuidArray(tenants))
}

const assignmentCols = `assignment_id, resource_type, resource_id, tenant_id, assigned_by, assigned_at`

func assignmentList(ctx context.Context, q string, args ...any) ([]write.AssignmentRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok infra.tenant_resource_assignments (schema infra, no RLS), live rows of the requested tenants (callers)
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AssignmentRow
	for rows.Next() {
		var a write.AssignmentRow
		if err := rows.Scan(&a.ID, &a.ResourceType, &a.ResourceID, &a.Tenant, &a.AssignedBy, &a.AssignedAt); err != nil {
			return nil, err
		}
		a.AssignedAt = a.AssignedAt.UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}

// Active implements write.AssignmentRepository.
func (TournamentStore) Active(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID) ([]write.AssignmentRow, error) {
	return assignmentList(ctx, `SELECT `+assignmentCols+` FROM infra.tenant_resource_assignments WHERE resource_type = $1 AND resource_id = $2
		AND tenant_id = ANY($3::uuid[]) AND deleted_at IS NULL ORDER BY tenant_id FOR UPDATE`, resourceType, resourceID, uuidArray(tenants))
}

// Insert implements write.AssignmentRepository.
func (TournamentStore) Insert(ctx context.Context, a write.AssignmentRow) error {
	_, err := exec(ctx, `INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at, created_by, updated_at, updated_by,
		resource_type, resource_id, assigned_by, assigned_at) VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $4, $3)`,
		a.ID, a.Tenant, a.AssignedAt, a.AssignedBy, a.ResourceType, a.ResourceID)
	if unique(err, "tra_active_uq") {
		return problem.New(problem.Conflict, "already assigned")
	}
	if err != nil {
		return fmt.Errorf("insert assignment: %w", err)
	}
	return nil
}

// Revoke implements write.AssignmentRepository (soft-delete, R21).
func (TournamentStore) Revoke(ctx context.Context, id uuid.UUID, by uuid.UUID, at time.Time) error {
	_, err := exec(ctx, `UPDATE infra.tenant_resource_assignments SET deleted_at = $2, deleted_by = $3, delete_reason = 'unassigned',
		updated_at = $2, updated_by = $3, version = version + 1 WHERE assignment_id = $1 AND deleted_at IS NULL`, id, at, by)
	if err != nil {
		return fmt.Errorf("revoke assignment: %w", err)
	}
	return nil
}

// AssignmentPage implements read.AssignmentReader: keyset (assigned_at, assignment_id), LIMIT in SQL.
func (TournamentStore) AssignmentPage(ctx context.Context, resourceType string, resourceID uuid.UUID, after *write.AssignmentRow, limit int) ([]write.AssignmentRow, error) {
	if after == nil {
		return assignmentList(ctx, `SELECT `+assignmentCols+` FROM infra.tenant_resource_assignments WHERE resource_type = $1 AND resource_id = $2
			AND deleted_at IS NULL ORDER BY assigned_at, assignment_id LIMIT $3`, resourceType, resourceID, limit)
	}
	return assignmentList(ctx, `SELECT `+assignmentCols+` FROM infra.tenant_resource_assignments WHERE resource_type = $1 AND resource_id = $2
		AND deleted_at IS NULL AND (assigned_at, assignment_id) > ($3, $4) ORDER BY assigned_at, assignment_id LIMIT $5`, resourceType, resourceID,
		after.AssignedAt, after.ID, limit)
}
