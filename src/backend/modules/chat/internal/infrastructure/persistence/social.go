// Package persistence implements the write and read ports of module chat on eduworld_chat: explicit SQL in the
// transaction of entx.UnitOfWork / entx.ReadSession, with tenant_id and deleted_at conditions on every business table
// (AGENTS R27) on top of RLS. Rows are never physically deleted (ADR-0028): removal sets deleted_at.
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok || !entx.InWriteTx(ctx) {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

func exec(ctx context.Context, query string, args ...any) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	res, err := t.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func count(ctx context.Context, query string, args ...any) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	if err := t.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("social: count: %w", err)
	}
	return n, nil
}

func actor(ctx context.Context) any {
	if p, ok := authz.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return nil
}

// Social implements write.SocialRepository on social.*.
type Social struct{}

var _ write.SocialRepository = Social{}

// LockUsers implements write.SocialRepository: transaction advisory locks per student, in a fixed order.
func (Social) LockUsers(ctx context.Context, users ...uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(users))
	for _, u := range users {
		keys = append(keys, u.String())
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 && keys[i-1] == k {
			continue
		}
		if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('chat.social:' || $1, 0))`, k); err != nil {
			return fmt.Errorf("social: lock: %w", err)
		}
	}
	return nil
}

// Blocked implements write.SocialRepository.
func (Social) Blocked(ctx context.Context, tenant, a, b uuid.UUID) (bool, error) {
	n, err := count(ctx, `SELECT count(*) FROM social.blocks WHERE tenant_id = $1 AND deleted_at IS NULL
		AND ((blocker_id = $2 AND blocked_id = $3) OR (blocker_id = $3 AND blocked_id = $2))`, tenant, a, b)
	return n > 0, err
}

// Friends implements write.SocialRepository.
func (Social) Friends(ctx context.Context, tenant uuid.UUID, p social.Pair) (bool, error) {
	n, err := count(ctx, `SELECT count(*) FROM social.friendships WHERE tenant_id = $1 AND user_low_id = $2 AND user_high_id = $3
		AND deleted_at IS NULL`, tenant, p.Low, p.High)
	return n > 0, err
}

// CountFriends implements write.SocialRepository.
func (Social) CountFriends(ctx context.Context, tenant, user uuid.UUID) (int, error) {
	return count(ctx, `SELECT count(*) FROM social.friendships WHERE tenant_id = $1 AND (user_low_id = $2 OR user_high_id = $2)
		AND deleted_at IS NULL`, tenant, user)
}

// CountPending implements write.SocialRepository.
func (Social) CountPending(ctx context.Context, tenant, requester uuid.UUID, now time.Time) (int, error) {
	return count(ctx, `SELECT count(*) FROM social.friend_requests WHERE tenant_id = $1 AND requester_id = $2 AND status = 'pending'
		AND expires_at > $3 AND deleted_at IS NULL`, tenant, requester, now)
}

// CountSince implements write.SocialRepository (every request sent counts toward the daily quota, also soft-deleted).
func (Social) CountSince(ctx context.Context, tenant, requester uuid.UUID, since time.Time) (int, error) {
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; the daily quota counts every request ever sent, deleted_at included
	return count(ctx, `SELECT count(*) FROM social.friend_requests WHERE tenant_id = $1 AND requester_id = $2 AND created_at >= $3`,
		tenant, requester, since)
}

// PendingFrom implements write.SocialRepository.
func (Social) PendingFrom(ctx context.Context, tenant, requester, addressee uuid.UUID, now time.Time) (bool, error) {
	n, err := count(ctx, `SELECT count(*) FROM social.friend_requests WHERE tenant_id = $1 AND requester_id = $2 AND addressee_id = $3
		AND status = 'pending' AND expires_at > $4 AND deleted_at IS NULL`, tenant, requester, addressee, now)
	return n > 0, err
}

// ExpireStale implements write.SocialRepository.
func (Social) ExpireStale(ctx context.Context, tenant, a, b uuid.UUID, now time.Time) error {
	_, err := exec(ctx, `UPDATE social.friend_requests SET status = 'expired', responded_at = $4, updated_at = $4, version = version + 1
		WHERE tenant_id = $1 AND status = 'pending' AND expires_at <= $4 AND deleted_at IS NULL
		AND ((requester_id = $2 AND addressee_id = $3) OR (requester_id = $3 AND addressee_id = $2))`, tenant, a, b, now)
	if err != nil {
		return fmt.Errorf("social: expire stale: %w", err)
	}
	return nil
}

// CreateRequest implements write.SocialRepository.
func (Social) CreateRequest(ctx context.Context, r *social.FriendRequest) error {
	if r.ID == uuid.Nil {
		r.ID = ids.New()
	}
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id)
	_, err := exec(ctx, `INSERT INTO social.friend_requests (request_id, tenant_id, created_at, created_by, updated_at, requester_id,
		addressee_id, status, source, expires_at) VALUES ($1, $2, $3, $4, $3, $5, $6, 'pending', $7, $8)`,
		r.ID, r.TenantID, r.CreatedAt, actor(ctx), r.RequesterID, r.AddresseeID, r.Source, r.ExpiresAt)
	if err != nil {
		return fmt.Errorf("social: create request: %w", err)
	}
	return nil
}

// Request implements write.SocialRepository.
func (Social) Request(ctx context.Context, tenant, id uuid.UUID, lock bool) (*social.FriendRequest, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	r := social.FriendRequest{}
	var responded sql.NullTime
	q := `SELECT request_id, tenant_id, requester_id, addressee_id, status, source, created_at, expires_at,
		responded_at FROM social.friend_requests WHERE tenant_id = $1 AND request_id = $2 AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	err = t.QueryRowContext(ctx, q, tenant, id).
		Scan(&r.ID, &r.TenantID, &r.RequesterID, &r.AddresseeID, &r.Status, &r.Source, &r.CreatedAt, &r.ExpiresAt, &responded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("social: request: %w", err)
	}
	if responded.Valid {
		at := responded.Time.UTC()
		r.RespondedAt = &at
	}
	r.CreatedAt, r.ExpiresAt = r.CreatedAt.UTC(), r.ExpiresAt.UTC()
	return &r, nil
}

// SaveRequest implements write.SocialRepository.
func (Social) SaveRequest(ctx context.Context, r *social.FriendRequest) error {
	_, err := exec(ctx, `UPDATE social.friend_requests SET status = $3, responded_at = $4, updated_at = now(), updated_by = $5,
		version = version + 1 WHERE tenant_id = $1 AND request_id = $2 AND deleted_at IS NULL`,
		r.TenantID, r.ID, r.Status, r.RespondedAt, actor(ctx))
	if err != nil {
		return fmt.Errorf("social: save request: %w", err)
	}
	return nil
}

// CancelPending implements write.SocialRepository.
func (Social) CancelPending(ctx context.Context, tenant, a, b uuid.UUID, now time.Time) (int, error) {
	n, err := exec(ctx, `UPDATE social.friend_requests SET status = 'cancelled', responded_at = $4, updated_at = $4, updated_by = $5,
		version = version + 1 WHERE tenant_id = $1 AND status = 'pending' AND deleted_at IS NULL
		AND ((requester_id = $2 AND addressee_id = $3) OR (requester_id = $3 AND addressee_id = $2))`, tenant, a, b, now, actor(ctx))
	if err != nil {
		return 0, fmt.Errorf("social: cancel pending: %w", err)
	}
	return int(n), nil
}

// CancelOfStudent implements write.SocialRepository.
func (Social) CancelOfStudent(ctx context.Context, tenant, student uuid.UUID, now time.Time) (int, error) {
	n, err := exec(ctx, `UPDATE social.friend_requests SET status = 'cancelled', responded_at = $3, updated_at = $3, version = version + 1
		WHERE tenant_id = $1 AND status = 'pending' AND deleted_at IS NULL AND (requester_id = $2 OR addressee_id = $2)`, tenant, student, now)
	if err != nil {
		return 0, fmt.Errorf("social: cancel requests of student: %w", err)
	}
	return int(n), nil
}

// CreateFriendship implements write.SocialRepository.
func (Social) CreateFriendship(ctx context.Context, tenant uuid.UUID, p social.Pair) error {
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id); the unique live pair index refuses a duplicate
	_, err := exec(ctx, `INSERT INTO social.friendships (friendship_id, tenant_id, created_at, created_by, updated_at, user_low_id,
		user_high_id) VALUES ($1, $2, now(), $3, now(), $4, $5)`, ids.New(), tenant, actor(ctx), p.Low, p.High)
	if err != nil {
		return fmt.Errorf("social: create friendship: %w", err)
	}
	return nil
}

// RemoveFriendship implements write.SocialRepository (soft-delete, ADR-0028).
func (Social) RemoveFriendship(ctx context.Context, tenant uuid.UUID, p social.Pair, reason string) (bool, error) {
	n, err := exec(ctx, `UPDATE social.friendships SET deleted_at = now(), deleted_by = $4, delete_reason = $5, updated_at = now(),
		updated_by = $4, version = version + 1 WHERE tenant_id = $1 AND user_low_id = $2 AND user_high_id = $3 AND deleted_at IS NULL`,
		tenant, p.Low, p.High, actor(ctx), reason)
	if err != nil {
		return false, fmt.Errorf("social: remove friendship: %w", err)
	}
	return n > 0, nil
}

// ReplaceCode implements write.SocialRepository.
func (Social) ReplaceCode(ctx context.Context, tenant, owner uuid.UUID) error {
	_, err := exec(ctx, `UPDATE social.friend_codes SET deleted_at = now(), deleted_by = $3, delete_reason = 'replaced', updated_at = now(),
		version = version + 1 WHERE tenant_id = $1 AND owner_id = $2 AND deleted_at IS NULL`, tenant, owner, actor(ctx))
	if err != nil {
		return fmt.Errorf("social: replace friend code: %w", err)
	}
	return nil
}

// CreateCode implements write.SocialRepository.
func (Social) CreateCode(ctx context.Context, tenant, owner uuid.UUID, code string, expires time.Time) error {
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id); conflict on the live-code indexes
	n, err := exec(ctx, `INSERT INTO social.friend_codes (friend_code_id, tenant_id, created_at, created_by, updated_at, code, owner_id,
		expires_at) VALUES ($1, $2, now(), $3, now(), $4, $3, $5) ON CONFLICT DO NOTHING`, ids.New(), tenant, owner, code, expires)
	if err != nil {
		return fmt.Errorf("social: create friend code: %w", err)
	}
	if n == 0 {
		return write.ErrCodeTaken
	}
	return nil
}

// CodeOwner implements write.SocialRepository.
func (Social) CodeOwner(ctx context.Context, tenant uuid.UUID, code string, now time.Time) (*uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var owner uuid.UUID
	err = t.QueryRowContext(ctx, `SELECT owner_id FROM social.friend_codes WHERE tenant_id = $1 AND code = $2 AND expires_at > $3
		AND deleted_at IS NULL`, tenant, code, now).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("social: friend code: %w", err)
	}
	return &owner, nil
}

func setRelation(ctx context.Context, table, from, to string, tenant, a, b uuid.UUID, on bool) (bool, error) {
	var n int64
	var err error
	if on {
		// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id); the live pair index makes it idempotent
		n, err = exec(ctx, `INSERT INTO social.`+table+` (`+table[:len(table)-1]+`_id, tenant_id, created_at, created_by, updated_at, `+
			from+`, `+to+`) VALUES ($1, $2, now(), $3, now(), $3, $4) ON CONFLICT DO NOTHING`, ids.New(), tenant, a, b)
	} else {
		n, err = exec(ctx, `UPDATE social.`+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = 'removed', updated_at = now(),
			updated_by = $2, version = version + 1 WHERE tenant_id = $1 AND `+from+` = $2 AND `+to+` = $3 AND deleted_at IS NULL`,
			tenant, a, b)
	}
	if err != nil {
		return false, fmt.Errorf("social: %s: %w", table, err)
	}
	return n > 0, nil
}

// SetBlock implements write.SocialRepository.
func (Social) SetBlock(ctx context.Context, tenant, blocker, blocked uuid.UUID, on bool) (bool, error) {
	return setRelation(ctx, "blocks", "blocker_id", "blocked_id", tenant, blocker, blocked, on)
}

// SetMute implements write.SocialRepository.
func (Social) SetMute(ctx context.Context, tenant, muter, muted uuid.UUID, on bool) (bool, error) {
	return setRelation(ctx, "mutes", "muter_id", "muted_id", tenant, muter, muted, on)
}

// Expiry implements write.ExpiryRepository (PlatformScope: chat_platform across tenants, job expire-friend-requests).
type Expiry struct{}

var _ write.ExpiryRepository = Expiry{}

// ExpireRequests implements write.ExpiryRepository.
func (Expiry) ExpireRequests(ctx context.Context, now time.Time, limit int) (int, error) {
	// tenancyguard:ok platform job across tenants (PlatformScope, chat_platform); deleted_at filtered; rows keep their tenant_id
	n, err := exec(ctx, `UPDATE social.friend_requests SET status = 'expired', responded_at = $1, updated_at = $1, version = version + 1
		WHERE request_id IN (SELECT request_id FROM social.friend_requests WHERE status = 'pending' AND expires_at <= $1
			AND deleted_at IS NULL ORDER BY expires_at LIMIT $2 FOR UPDATE SKIP LOCKED)`, now, limit)
	if err != nil {
		return 0, fmt.Errorf("social: expire requests: %w", err)
	}
	return int(n), nil
}

// ExpireCodes implements write.ExpiryRepository (soft-delete only).
func (Expiry) ExpireCodes(ctx context.Context, now time.Time, limit int) (int, error) {
	// tenancyguard:ok platform job across tenants (PlatformScope, chat_platform); deleted_at filtered; rows keep their tenant_id
	n, err := exec(ctx, `UPDATE social.friend_codes SET deleted_at = $1, delete_reason = 'expired', updated_at = $1, version = version + 1
		WHERE friend_code_id IN (SELECT friend_code_id FROM social.friend_codes WHERE expires_at <= $1 AND deleted_at IS NULL
			ORDER BY expires_at LIMIT $2 FOR UPDATE SKIP LOCKED)`, now, limit)
	if err != nil {
		return 0, fmt.Errorf("social: expire codes: %w", err)
	}
	return int(n), nil
}

// Directory implements write.Directory on the projections ref.tenants, ref.students, ref.player_profiles.
type Directory struct{}

var _ write.Directory = Directory{}

// Tenant implements write.Directory.
func (Directory) Tenant(ctx context.Context, tenant uuid.UUID) (*social.Tenant, error) {
	t, err := anyTx(ctx) // SELECT only: a write transaction or a read session (F120 club queries)
	if err != nil {
		return nil, err
	}
	out := social.Tenant{ID: tenant}
	// tenancyguard:ok tenant registry ref.tenants (key = tenant_id, no deleted_at: projection rule 2)
	err = t.QueryRowContext(ctx, `SELECT tenant_type, coalesce((settings_subset->>'cross_school_social_enabled')::boolean, false),
		coalesce((SELECT o.friends_enabled FROM chat.common_tenant_social_overrides o WHERE o.tenant_id = ref.tenants.tenant_id
			AND o.deleted_at IS NULL), true) FROM ref.tenants WHERE tenant_id = $1`, tenant).
		Scan(&out.Type, &out.CrossSchool, &out.CommonFriendsEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref tenants: %w", err)
	}
	return &out, nil
}

// Student implements write.Directory.
func (Directory) Student(ctx context.Context, tenant, student uuid.UUID) (*social.Student, error) {
	t, err := anyTx(ctx) // SELECT only: a write transaction or a read session (F120 club queries)
	if err != nil {
		return nil, err
	}
	out := social.Student{ID: student, TenantID: tenant}
	// tenancyguard:ok projection ref.students under the TenantScope (RLS) plus the explicit tenant_id; deleted_at is read as state
	err = t.QueryRowContext(ctx, `SELECT school_id, status, deleted_at IS NOT NULL FROM ref.students WHERE student_id = $1 AND tenant_id = $2`,
		student, tenant).Scan(&out.SchoolID, &out.Status, &out.Deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref students: %w", err)
	}
	return &out, nil
}

// Cards implements write.Directory.
func (Directory) Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]write.Card, error) {
	t, err := anyTx(ctx) // SELECT only: a write transaction or a read session (F120 club queries)
	if err != nil {
		return nil, err
	}
	list := make([]string, len(students))
	for i, s := range students {
		list[i] = s.String()
	}
	// tenancyguard:ok projection ref.player_profiles under the TenantScope (RLS) plus the explicit tenant_id; ref.* has no deleted_at
	rows, err := t.QueryContext(ctx, `SELECT student_id, nickname, level, avatar_config FROM ref.player_profiles
		WHERE tenant_id = $1 AND student_id = ANY($2::uuid[])`, tenant, "{"+join(list)+"}")
	if err != nil {
		return nil, fmt.Errorf("ref player profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]write.Card{}
	for rows.Next() {
		var c write.Card
		var nick sql.NullString
		var avatar []byte
		if err := rows.Scan(&c.UserID, &nick, &c.Level, &avatar); err != nil {
			return nil, fmt.Errorf("ref player profiles: %w", err)
		}
		if nick.Valid {
			s := nick.String
			c.Nickname = &s
		}
		if len(avatar) > 0 && string(avatar) != "null" {
			c.Avatar = json.RawMessage(avatar)
		}
		out[c.UserID] = c
	}
	return out, rows.Err()
}

func join(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}

// Publisher implements write.EventPublisher with the outbox writer of module chat.
type Publisher struct{ W *outbox.Writer }

var _ write.EventPublisher = Publisher{}

// Publish writes e into the outbox of the transaction.
func (p Publisher) Publish(ctx context.Context, e write.Event) error {
	_, err := p.W.Publish(ctx, outbox.Message{Type: e.Type, Subject: outbox.Subject{Type: e.SubjectType, ID: e.SubjectID},
		PartitionKey: e.PartitionKey, TenantID: e.TenantID, Platform: e.Platform, OccurredAt: e.OccurredAt, Payload: e.Payload})
	return err
}

// Auditor implements write.Auditor with the outbox audit recorder.
type Auditor struct{ R audit.Recorder }

var _ write.Auditor = Auditor{}

// Record records e in the transaction of ctx.
func (a Auditor) Record(ctx context.Context, e audit.Event) error { return a.R.Record(ctx, e) }
