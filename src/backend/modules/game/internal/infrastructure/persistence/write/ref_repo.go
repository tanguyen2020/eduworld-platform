package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// RefRepo implements write.ProjectionWriter with explicit SQL on the consumer's transaction (the projections have no
// ent schema: DATABASE_DESIGN §2.17 minimal columns). Every upsert applies only a newer source_version and never
// moves a row to another tenant (the WHERE of ON CONFLICT keeps the stored tenant).
type RefRepo struct{}

var _ write.ProjectionWriter = RefRepo{}

func exec(ctx context.Context, query string, args ...any) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	res, err := t.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func jsonOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// OwnerTenant implements write.ProjectionWriter through the owner-run function ref.projection_tenant, which sees
// every tenant (the consumer's session only sees its own).
func (RefRepo) OwnerTenant(ctx context.Context, table write.RefTable, id uuid.UUID) (*uuid.UUID, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	var owner uuid.NullUUID
	if err := t.QueryRowContext(ctx, `SELECT ref.projection_tenant($1, $2)`, string(table), id).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("ref: owner of %s %s: %w", table, id, err)
	}
	if !owner.Valid {
		return nil, nil
	}
	return &owner.UUID, nil
}

// UpsertTenant implements write.ProjectionWriter (ref.tenants is the tenant registry itself: no tenant filter).
func (RefRepo) UpsertTenant(ctx context.Context, r write.RefTenant) (bool, error) {
	// tenancyguard:ok projection upsert of the tenant registry ref.tenants (key = tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2)
	return exec(ctx, `INSERT INTO ref.tenants AS t (tenant_id, code, tenant_type, status, is_qa, settings_subset, source_version)
		VALUES ($1, $2::varchar, $3, $4, coalesce($5::boolean, false), coalesce($6::jsonb, '{}'::jsonb), $7)
		ON CONFLICT (tenant_id) DO UPDATE SET code = coalesce($2, t.code), tenant_type = $3, status = $4,
			is_qa = coalesce($5, t.is_qa), settings_subset = coalesce($6::jsonb, t.settings_subset), source_version = $7
		WHERE t.source_version < $7`,
		r.TenantID, r.Code, r.TenantType, r.Status, r.IsQA, jsonOrNil(r.SettingsSubset), r.SourceVersion)
}

// UpsertSchool implements write.ProjectionWriter.
func (RefRepo) UpsertSchool(ctx context.Context, r write.RefSchool) (bool, error) {
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); ref.* has no deleted_at
	return exec(ctx, `INSERT INTO ref.schools AS s (school_id, tenant_id, status, settings_subset, source_version)
		VALUES ($1, $2, coalesce($3::varchar, 'active'), coalesce($4::jsonb, '{}'::jsonb), $5)
		ON CONFLICT (school_id) DO UPDATE SET status = coalesce($3, s.status),
			settings_subset = coalesce($4::jsonb, s.settings_subset), source_version = $5
		WHERE s.source_version < $5 AND s.tenant_id = $2`,
		r.SchoolID, r.TenantID, r.Status, jsonOrNil(r.SettingsSubset), r.SourceVersion)
}

// UpsertStudent implements write.ProjectionWriter.
func (RefRepo) UpsertStudent(ctx context.Context, r write.RefStudent) (bool, error) {
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); deleted_at mirrors the source (cleanup)
	return exec(ctx, `INSERT INTO ref.students AS st (student_id, tenant_id, school_id, grade_level, status, source_version, sis_connection_id)
		VALUES ($1, $2, $3, $4::smallint, coalesce($5::varchar, 'active'), $6, $7::uuid)
		ON CONFLICT (student_id) DO UPDATE SET school_id = $3, grade_level = coalesce($4, st.grade_level),
			status = coalesce($5, st.status), source_version = $6, sis_connection_id = coalesce($7::uuid, st.sis_connection_id)
		WHERE st.source_version < $6 AND st.tenant_id = $2`,
		r.StudentID, r.TenantID, r.SchoolID, r.GradeLevel, r.Status, r.SourceVersion, r.SISConnectionID)
}

// SetStudentStatus implements write.ProjectionWriter.
func (RefRepo) SetStudentStatus(ctx context.Context, student uuid.UUID, tenant *uuid.UUID, status string) (bool, error) {
	// The event scope is one tenant (RLS) or the platform scope of a platform event (this one student, every tenant).
	// tenancyguard:ok projection of user.status_changed; deleted_at is the projected source state
	return exec(ctx, `UPDATE ref.students SET status = $3 WHERE student_id = $1 AND ($2::uuid IS NULL OR tenant_id = $2)
		AND status <> $3`, student, tenant, status)
}

// MarkStudentDeleted implements write.ProjectionWriter: cleanup sets deleted_at to the event time unless a later
// event already did; restore clears a deletion that is not newer than the restore (ADR-0032).
func (RefRepo) MarkStudentDeleted(ctx context.Context, student, tenant uuid.UUID, deleted bool, at time.Time) (bool, error) {
	if deleted {
		// tenancyguard:ok projection of student.cleaned_up under the event TenantScope; deleted_at is the projected source state
		return exec(ctx, `UPDATE ref.students SET deleted_at = $3
			WHERE student_id = $1 AND tenant_id = $2 AND (deleted_at IS NULL OR deleted_at < $3)`, student, tenant, at.UTC())
	}
	// tenancyguard:ok projection of student.restored under the event TenantScope; deleted_at is the projected source state
	return exec(ctx, `UPDATE ref.students SET deleted_at = NULL
		WHERE student_id = $1 AND tenant_id = $2 AND deleted_at IS NOT NULL AND deleted_at <= $3`, student, tenant, at.UTC())
}

// RefPlatformSettings implements write.PlatformSettingWriter on ref.platform_settings (platform projection written by
// the consumer game.platform_settings under SystemPlatformScope, role game_platform).
type RefPlatformSettings struct{}

var _ write.PlatformSettingWriter = RefPlatformSettings{}

// UpsertPlatformSetting implements write.PlatformSettingWriter: only a newer source_version replaces the value.
func (RefPlatformSettings) UpsertPlatformSetting(ctx context.Context, r write.RefPlatformSetting) (bool, error) {
	// tenancyguard:ok platform projection ref.platform_settings (platform data: no tenant_id, no deleted_at; rule 1)
	return exec(ctx, `INSERT INTO ref.platform_settings AS p (key, value, source_version, updated_at)
		VALUES ($1, $2::jsonb, $3, now())
		ON CONFLICT (key) DO UPDATE SET value = $2::jsonb, source_version = $3, updated_at = now()
		WHERE p.source_version < $3`,
		r.Key, string(r.Value), r.SourceVersion)
}

// SetStudentSIS implements write.ProjectionWriter.
func (RefRepo) SetStudentSIS(ctx context.Context, student, tenant, connection uuid.UUID) (bool, error) {
	// tenancyguard:ok projection of the SIS link events under the event TenantScope (RLS + explicit tenant_id); ref.* has no deleted_at filter (projected state)
	return exec(ctx, `UPDATE ref.students SET sis_connection_id = $3
		WHERE student_id = $1 AND tenant_id = $2 AND sis_connection_id IS DISTINCT FROM $3`, student, tenant, connection)
}
