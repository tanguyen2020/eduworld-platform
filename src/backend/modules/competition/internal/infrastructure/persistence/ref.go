package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Refs writes the ref.* projections of module competition (DATABASE_DESIGN §2.17): only the consumers call it, inside
// the transaction opened with the event scope; every upsert applies a newer source_version only and never moves a row
// to another tenant.
type Refs struct{}

// OwnerTenant returns the tenant owning a projected row whatever the session tenant (nil when unknown).
func (Refs) OwnerTenant(ctx context.Context, table string, id uuid.UUID) (*uuid.UUID, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	var owner uuid.NullUUID
	if err := t.QueryRowContext(ctx, `SELECT ref.projection_tenant($1, $2)`, table, id).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("ref: owner of %s: %w", table, err)
	}
	if !owner.Valid {
		return nil, nil
	}
	return &owner.UUID, nil
}

// Tenant upserts ref.tenants.
func (Refs) Tenant(ctx context.Context, tenant uuid.UUID, code *string, typ, status string, isQA *bool, settings []byte, version int64) (bool, error) {
	var s any
	if len(settings) > 0 {
		s = string(settings)
	}
	// tenancyguard:ok projection upsert of the tenant registry ref.tenants (key = tenant_id, no deleted_at: rule 2)
	return exec(ctx, `INSERT INTO ref.tenants AS t (tenant_id, code, tenant_type, status, is_qa, settings_subset, source_version)
		VALUES ($1, $2::varchar, $3, $4, coalesce($5::boolean, false), coalesce($6::jsonb, '{}'::jsonb), $7)
		ON CONFLICT (tenant_id) DO UPDATE SET code = coalesce($2, t.code), tenant_type = $3, status = $4,
			is_qa = coalesce($5, t.is_qa), settings_subset = coalesce($6::jsonb, t.settings_subset), source_version = $7
		WHERE t.source_version < $7`, tenant, code, typ, status, isQA, s, version)
}

// Student upserts ref.students.
func (Refs) Student(ctx context.Context, student, tenant, school uuid.UUID, grade *int, status *string, version int64) (bool, error) {
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); deleted_at mirrors the source (cleanup)
	return exec(ctx, `INSERT INTO ref.students AS st (student_id, tenant_id, school_id, grade_level, status, source_version)
		VALUES ($1, $2, $3, $4::smallint, coalesce($5::varchar, 'active'), $6)
		ON CONFLICT (student_id) DO UPDATE SET school_id = $3, grade_level = coalesce($4, st.grade_level),
			status = coalesce($5, st.status), source_version = $6
		WHERE st.source_version < $6 AND st.tenant_id = $2`, student, tenant, school, grade, status, version)
}

// StudentStatus applies user.status_changed (tenant nil: platform event, every tenant of that student).
func (Refs) StudentStatus(ctx context.Context, student uuid.UUID, tenant *uuid.UUID, status string) (bool, error) {
	// tenancyguard:ok projection of user.status_changed; deleted_at is the projected source state
	return exec(ctx, `UPDATE ref.students SET status = $3 WHERE student_id = $1 AND ($2::uuid IS NULL OR tenant_id = $2)
		AND status <> $3`, student, tenant, status)
}

// StudentDeleted mirrors student.cleaned_up / student.restored (ADR-0032).
func (Refs) StudentDeleted(ctx context.Context, student, tenant uuid.UUID, deleted bool, at time.Time) (bool, error) {
	if deleted {
		// tenancyguard:ok projection of student.cleaned_up under the event TenantScope; deleted_at is the projected source state
		return exec(ctx, `UPDATE ref.students SET deleted_at = $3 WHERE student_id = $1 AND tenant_id = $2
			AND (deleted_at IS NULL OR deleted_at < $3)`, student, tenant, at.UTC())
	}
	// tenancyguard:ok projection of student.restored under the event TenantScope; deleted_at is the projected source state
	return exec(ctx, `UPDATE ref.students SET deleted_at = NULL WHERE student_id = $1 AND tenant_id = $2
		AND deleted_at IS NOT NULL AND deleted_at <= $3`, student, tenant, at.UTC())
}

// PlayerProfile upserts ref.player_profiles from game_profile.updated (public data only, R16).
func (Refs) PlayerProfile(ctx context.Context, student, tenant uuid.UUID, nickname *string, level int, avatar []byte, version int64) (bool, error) {
	var a any
	if len(avatar) > 0 {
		a = string(avatar)
	}
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); ref.* has no deleted_at (rule 2)
	return exec(ctx, `INSERT INTO ref.player_profiles AS p (student_id, tenant_id, nickname, level, avatar_config, source_version)
		VALUES ($1, $2, $3::varchar, $4, $5::jsonb, $6)
		ON CONFLICT (student_id) DO UPDATE SET nickname = $3, level = $4, avatar_config = $5::jsonb, source_version = $6
		WHERE p.source_version < $6 AND p.tenant_id = $2`, student, tenant, nickname, level, a, version)
}

// Friendship upserts ref.friendships from friend.added / friend.removed (newer source_version only; a pair never moves
// to another tenant).
func (Refs) Friendship(ctx context.Context, tenant, low, high uuid.UUID, active bool, version int64) (bool, error) {
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); ref.* has no deleted_at (rule 2)
	return exec(ctx, `INSERT INTO ref.friendships AS f (tenant_id, user_low_id, user_high_id, active, source_version)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_low_id, user_high_id) DO UPDATE SET active = $4, source_version = $5
		WHERE f.source_version < $5 AND f.tenant_id = $1`, tenant, low, high, active, version)
}

// Block upserts ref.blocks from user.blocked / user.unblocked (newer source_version only).
func (Refs) Block(ctx context.Context, tenant, blocker, blocked uuid.UUID, active bool, version int64) (bool, error) {
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); ref.* has no deleted_at (rule 2)
	return exec(ctx, `INSERT INTO ref.blocks AS b (tenant_id, blocker_id, blocked_id, active, source_version)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (blocker_id, blocked_id) DO UPDATE SET active = $4, source_version = $5
		WHERE b.source_version < $5 AND b.tenant_id = $1`, tenant, blocker, blocked, active, version)
}
