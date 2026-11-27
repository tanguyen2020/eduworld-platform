package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

func readTx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// Reader implements read.SocialReader inside entx.ReadSession: explicit SQL with tenant_id and deleted_at conditions
// (R27); only nickname, level and avatar of the other student (R16).
type Reader struct{}

var _ read.SocialReader = Reader{}

func scanRelations(rows *sql.Rows) ([]read.Relation, error) {
	defer func() { _ = rows.Close() }()
	var out []read.Relation
	for rows.Next() {
		var r read.Relation
		var nick sql.NullString
		var level sql.NullInt64
		var avatar, title []byte
		if err := rows.Scan(&r.RowID, &r.UserID, &r.CreatedAt, &nick, &level, &avatar, &title); err != nil {
			return nil, fmt.Errorf("social read: %w", err)
		}
		r.CreatedAt = r.CreatedAt.UTC()
		if nick.Valid {
			s := nick.String
			r.Nickname = &s
		}
		r.Level = 1
		if level.Valid {
			r.Level = int(level.Int64)
		}
		if len(avatar) > 0 && string(avatar) != "null" {
			r.Avatar = json.RawMessage(avatar)
		}
		r.Title = rawJSON(title)
		out = append(out, r)
	}
	return out, rows.Err()
}

func afterArgs(after *read.After) (any, any) {
	if after == nil {
		return nil, nil
	}
	return after.At, after.ID
}

// Friends implements read.SocialReader.
func (Reader) Friends(ctx context.Context, tenant, user uuid.UUID, after *read.After, limit int) ([]read.Relation, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := afterArgs(after)
	// tenancyguard:ok social.friendships with tenant_id + deleted_at; projections ref.students/ref.player_profiles of the same tenant (no deleted_at on profiles, cleaned-up students excluded)
	rows, err := t.QueryContext(ctx, `SELECT f.friendship_id, o.other, f.created_at, p.nickname, p.level, p.avatar_config, p.title
		FROM social.friendships f
		CROSS JOIN LATERAL (SELECT CASE WHEN f.user_low_id = $2 THEN f.user_high_id ELSE f.user_low_id END AS other) o
		JOIN ref.students s ON s.student_id = o.other AND s.tenant_id = f.tenant_id AND s.deleted_at IS NULL
		LEFT JOIN ref.player_profiles p ON p.student_id = o.other AND p.tenant_id = f.tenant_id
		WHERE f.tenant_id = $1 AND (f.user_low_id = $2 OR f.user_high_id = $2) AND f.deleted_at IS NULL
		AND ($3::timestamptz IS NULL OR (f.created_at, f.friendship_id) < ($3, $4::uuid))
		ORDER BY f.created_at DESC, f.friendship_id DESC LIMIT $5`, tenant, user, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("social read friends: %w", err)
	}
	return scanRelations(rows)
}

func (Reader) relations(ctx context.Context, table, key, from, to string, tenant, user uuid.UUID, after *read.After, limit int) ([]read.Relation, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := afterArgs(after)
	// tenancyguard:ok social blocks/mutes with tenant_id + deleted_at; projection ref.player_profiles of the same tenant (no deleted_at)
	rows, err := t.QueryContext(ctx, `SELECT r.`+key+`, r.`+to+`, r.created_at, p.nickname, p.level, p.avatar_config, p.title
		FROM social.`+table+` r LEFT JOIN ref.player_profiles p ON p.student_id = r.`+to+` AND p.tenant_id = r.tenant_id
		WHERE r.tenant_id = $1 AND r.`+from+` = $2 AND r.deleted_at IS NULL
		AND ($3::timestamptz IS NULL OR (r.created_at, r.`+key+`) < ($3, $4::uuid))
		ORDER BY r.created_at DESC, r.`+key+` DESC LIMIT $5`, tenant, user, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("social read %s: %w", table, err)
	}
	return scanRelations(rows)
}

// Blocks implements read.SocialReader (only the blocks the student created: never who blocked them, F095-AC-19).
func (r Reader) Blocks(ctx context.Context, tenant, user uuid.UUID, after *read.After, limit int) ([]read.Relation, error) {
	return r.relations(ctx, "blocks", "block_id", "blocker_id", "blocked_id", tenant, user, after, limit)
}

// Mutes implements read.SocialReader.
func (r Reader) Mutes(ctx context.Context, tenant, user uuid.UUID, after *read.After, limit int) ([]read.Relation, error) {
	return r.relations(ctx, "mutes", "mute_id", "muter_id", "muted_id", tenant, user, after, limit)
}

// Requests implements read.SocialReader.
func (Reader) Requests(ctx context.Context, tenant, user uuid.UUID, incoming bool, now time.Time, limit int) ([]read.Request, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	cond := `fr.requester_id = $2 AND fr.status IN ('pending','rejected')`
	if incoming {
		cond = `fr.addressee_id = $2 AND fr.status = 'pending'`
	}
	// tenancyguard:ok social.friend_requests with tenant_id + deleted_at; projection ref.player_profiles of the same tenant (no deleted_at)
	rows, err := t.QueryContext(ctx, `SELECT fr.request_id, fr.requester_id, fr.addressee_id, fr.status, fr.created_at, fr.expires_at,
			rp.nickname, rp.level, rp.avatar_config, rp.title, ap.nickname, ap.level, ap.avatar_config, ap.title
		FROM social.friend_requests fr
		LEFT JOIN ref.player_profiles rp ON rp.student_id = fr.requester_id AND rp.tenant_id = fr.tenant_id
		LEFT JOIN ref.player_profiles ap ON ap.student_id = fr.addressee_id AND ap.tenant_id = fr.tenant_id
		WHERE fr.tenant_id = $1 AND `+cond+` AND fr.expires_at > $3 AND fr.deleted_at IS NULL
		ORDER BY fr.created_at DESC, fr.request_id DESC LIMIT $4`, tenant, user, now, limit)
	if err != nil {
		return nil, fmt.Errorf("social read requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.Request
	for rows.Next() {
		var r read.Request
		var rn, an sql.NullString
		var rl, al sql.NullInt64
		var ra, rt, aa, at []byte
		if err := rows.Scan(&r.ID, &r.RequesterID, &r.AddresseeID, &r.Status, &r.CreatedAt, &r.ExpiresAt, &rn, &rl, &ra, &rt, &an, &al, &aa, &at); err != nil {
			return nil, fmt.Errorf("social read requests: %w", err)
		}
		r.CreatedAt, r.ExpiresAt = r.CreatedAt.UTC(), r.ExpiresAt.UTC()
		r.Requester, r.Addressee = relation(r.RequesterID, rn, rl, ra), relation(r.AddresseeID, an, al, aa)
		r.Requester.Title, r.Addressee.Title = rawJSON(rt), rawJSON(at)
		out = append(out, r)
	}
	return out, rows.Err()
}

func relation(id uuid.UUID, nick sql.NullString, level sql.NullInt64, avatar []byte) read.Relation {
	r := read.Relation{UserID: id, Level: 1}
	if nick.Valid {
		s := nick.String
		r.Nickname = &s
	}
	if level.Valid {
		r.Level = int(level.Int64)
	}
	if len(avatar) > 0 && string(avatar) != "null" {
		r.Avatar = json.RawMessage(avatar)
	}
	return r
}

// FriendIDs implements read.SocialReader.
func (Reader) FriendIDs(ctx context.Context, tenant, user uuid.UUID) ([]uuid.UUID, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT CASE WHEN f.user_low_id = $2 THEN f.user_high_id ELSE f.user_low_id END
		FROM social.friendships f
		WHERE f.tenant_id = $1 AND (f.user_low_id = $2 OR f.user_high_id = $2) AND f.deleted_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM social.blocks b WHERE b.tenant_id = $1 AND b.deleted_at IS NULL
			AND ((b.blocker_id = f.user_low_id AND b.blocked_id = f.user_high_id) OR (b.blocker_id = f.user_high_id AND b.blocked_id = f.user_low_id)))`,
		tenant, user)
	if err != nil {
		return nil, fmt.Errorf("social read friend ids: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("social read friend ids: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// rawJSON keeps a jsonb column (nil for SQL NULL or JSON null).
func rawJSON(b []byte) json.RawMessage {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	return json.RawMessage(b)
}

// Coplayers implements read.SocialReader ("Vừa chơi cùng", delta F095): the rows of social.recent_coplayers of the
// caller within the window, filtered at read time — never a friend, a pending request either way, a block either way,
// a student the caller muted, a student who is not active or cleaned up, nor (sameSchool) another school.
func (Reader) Coplayers(ctx context.Context, f read.CoplayerFilter) ([]read.Coplayer, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := afterArgs(f.After)
	var school any
	if f.SameSchool != nil {
		school = *f.SameSchool
	}
	// tenancyguard:ok social.recent_coplayers/friendships/friend_requests/blocks/mutes with tenant_id + deleted_at IS NULL; projections ref.students (deleted_at read as state) and ref.player_profiles (no deleted_at) of the same tenant
	rows, err := t.QueryContext(ctx, `SELECT c.other_id, c.context_kind, c.last_played_at, p.nickname, p.level, p.avatar_config, p.title
		FROM social.recent_coplayers c
		JOIN ref.students s ON s.student_id = c.other_id AND s.tenant_id = c.tenant_id AND s.status = 'active' AND s.deleted_at IS NULL
		LEFT JOIN ref.player_profiles p ON p.student_id = c.other_id AND p.tenant_id = c.tenant_id
		WHERE c.tenant_id = $1 AND c.user_id = $2 AND c.deleted_at IS NULL AND c.last_played_at >= $3
		AND ($4::uuid IS NULL OR s.school_id = $4)
		AND ($5::timestamptz IS NULL OR (c.last_played_at, c.other_id) < ($5, $6::uuid))
		AND NOT EXISTS (SELECT 1 FROM social.friendships fs WHERE fs.tenant_id = $1 AND fs.deleted_at IS NULL
			AND fs.user_low_id = LEAST(c.user_id, c.other_id) AND fs.user_high_id = GREATEST(c.user_id, c.other_id))
		AND NOT EXISTS (SELECT 1 FROM social.friend_requests fr WHERE fr.tenant_id = $1 AND fr.deleted_at IS NULL
			AND fr.status = 'pending' AND fr.expires_at > $7
			AND ((fr.requester_id = c.user_id AND fr.addressee_id = c.other_id) OR (fr.requester_id = c.other_id AND fr.addressee_id = c.user_id)))
		AND NOT EXISTS (SELECT 1 FROM social.blocks b WHERE b.tenant_id = $1 AND b.deleted_at IS NULL
			AND ((b.blocker_id = c.user_id AND b.blocked_id = c.other_id) OR (b.blocker_id = c.other_id AND b.blocked_id = c.user_id)))
		AND NOT EXISTS (SELECT 1 FROM social.mutes m WHERE m.tenant_id = $1 AND m.deleted_at IS NULL
			AND m.muter_id = c.user_id AND m.muted_id = c.other_id)
		ORDER BY c.last_played_at DESC, c.other_id DESC LIMIT $8`, f.Tenant, f.User, f.Since, school, at, id, f.Now, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("social read coplayers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.Coplayer
	for rows.Next() {
		var c read.Coplayer
		var nick sql.NullString
		var level sql.NullInt64
		var avatar, title []byte
		if err := rows.Scan(&c.UserID, &c.ContextKind, &c.LastPlayedAt, &nick, &level, &avatar, &title); err != nil {
			return nil, fmt.Errorf("social read coplayers: %w", err)
		}
		c.Relation = relation(c.UserID, nick, level, avatar)
		c.Title, c.LastPlayedAt = rawJSON(title), c.LastPlayedAt.UTC()
		c.RowID = c.UserID
		out = append(out, c)
	}
	return out, rows.Err()
}
