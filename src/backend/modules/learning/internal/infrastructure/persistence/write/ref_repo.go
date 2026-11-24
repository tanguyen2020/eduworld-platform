// Package write implements the write ports of module learning inside entx.UnitOfWork: repositories (ent or
// explicit SQL on the transaction of ctx) and the outbox adapters (events, audit) of the same transaction.
package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// timeOrdered is the smallest stored version that is an envelope time (UnixMicro) rather than a source_version: real
// aggregate versions stay far below it, any event time after 1970-01-12 is above it.
const timeOrdered = 1_000_000_000_000

// newerVersion is the condition of the ref.schools upsert on the stored row alias and the argument
// $arg of the incoming version (F020 T044, OQ F020-1): within one kind (source_version or envelope time) only a newer
// value applies; a source_version replaces a time-ordered row, a time-ordered event never replaces a row ordered by
// source_version.
func newerVersion(alias string, arg int) string {
	return fmt.Sprintf("(((%[1]s.source_version < %[3]d) = ($%[2]d < %[3]d) AND %[1]s.source_version < $%[2]d) OR "+
		"(%[1]s.source_version >= %[3]d AND $%[2]d < %[3]d))", alias, arg, timeOrdered)
}

// RefRepo implements write.ProjectionWriter with explicit SQL on the consumer's transaction (the projections
// have no ent schema: DATABASE_DESIGN §2.17 minimal columns). Every upsert applies only a newer source_version
// and never moves a row to another tenant (the WHERE of ON CONFLICT keeps the stored tenant).
type RefRepo struct{}

var _ write.ProjectionWriter = RefRepo{}

func exec(ctx context.Context, query string, args ...any) (bool, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return false, entx.ErrNoTransaction
	}
	res, err := tx.ExecContext(ctx, query, args...)
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

// OwnerTenant implements write.ProjectionWriter through the owner-run function ref.projection_tenant, which
// sees every tenant (the consumer's session only sees its own).
func (RefRepo) OwnerTenant(ctx context.Context, table write.RefTable, id uuid.UUID) (*uuid.UUID, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	var t uuid.NullUUID
	if err := tx.QueryRowContext(ctx, `SELECT ref.projection_tenant($1, $2)`, string(table), id).Scan(&t); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("ref: owner of %s %s: %w", table, id, err)
	}
	if !t.Valid {
		return nil, nil
	}
	return &t.UUID, nil
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

// UpsertProvince implements write.ProjectionWriter and refreshes the province name denormalized in ref.schools.
func (RefRepo) UpsertProvince(ctx context.Context, r write.RefProvince) (bool, error) {
	// tenancyguard:ok projection upsert of the platform catalog ref.provinces (no tenant_id, no deleted_at)
	applied, err := exec(ctx, `INSERT INTO ref.provinces AS p (code, country_code, name_i18n, status, source_version)
		VALUES ($1, $2, $3::jsonb, $4, $5)
		ON CONFLICT (code) DO UPDATE SET country_code = $2, name_i18n = $3::jsonb, status = $4, source_version = $5
		WHERE p.source_version < $5`, r.Code, r.CountryCode, string(r.NameI18n), r.Status, r.SourceVersion)
	if err != nil || !applied {
		return applied, err
	}
	// tenancyguard:ok platform event (learning_platform): denormalized province name of every tenant's schools; ref.* has no deleted_at
	_, err = exec(ctx, `UPDATE ref.schools SET province_name_i18n = $2::jsonb WHERE province_code = $1`, r.Code, string(r.NameI18n))
	return true, err
}

// UpsertSchool implements write.ProjectionWriter.
func (RefRepo) UpsertSchool(ctx context.Context, r write.RefSchool) (bool, error) {
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); ref.* has no deleted_at
	return exec(ctx, `INSERT INTO ref.schools AS s (school_id, tenant_id, name, name_i18n, province_code, province_name_i18n,
			school_level, is_virtual, status, settings_subset, source_version)
		VALUES ($1, $2, $3::varchar, $4::jsonb, $5::varchar, (SELECT p.name_i18n FROM ref.provinces p WHERE p.code = $5::varchar), $6::varchar,
			coalesce($7::boolean, false), coalesce($8::varchar, 'active'), coalesce($9::jsonb, '{}'::jsonb), $10)
		ON CONFLICT (school_id) DO UPDATE SET name = coalesce($3, s.name), name_i18n = coalesce($4::jsonb, s.name_i18n),
			province_code = coalesce($5, s.province_code),
			province_name_i18n = (SELECT p.name_i18n FROM ref.provinces p WHERE p.code = coalesce($5, s.province_code)),
			school_level = coalesce($6, s.school_level), is_virtual = coalesce($7, s.is_virtual), status = coalesce($8, s.status),
			settings_subset = coalesce($9::jsonb, s.settings_subset), source_version = $10
		WHERE s.tenant_id = $2 AND `+newerVersion("s", 10),
		r.SchoolID, r.TenantID, r.Name, jsonOrNil(r.NameI18n), r.ProvinceCode, r.SchoolLevel, r.IsVirtual, r.Status,
		jsonOrNil(r.SettingsSubset), r.SourceVersion)
}

// UpsertStudent implements write.ProjectionWriter.
func (RefRepo) UpsertStudent(ctx context.Context, r write.RefStudent) (bool, error) {
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); deleted_at mirrors the source (cleanup)
	return exec(ctx, `INSERT INTO ref.students AS st (student_id, tenant_id, school_id, grade_level, status, source_version, academic_year_id)
		VALUES ($1, $2, $3, $4::smallint, coalesce($5::varchar, 'active'), $6, $7::uuid)
		ON CONFLICT (student_id) DO UPDATE SET school_id = $3, grade_level = coalesce($4, st.grade_level),
			status = coalesce($5, st.status), source_version = $6, academic_year_id = coalesce($7::uuid, st.academic_year_id)
		WHERE st.source_version < $6 AND st.tenant_id = $2`,
		r.StudentID, r.TenantID, r.SchoolID, r.GradeLevel, r.Status, r.SourceVersion, r.AcademicYearID)
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

var _ write.MinigameConfigProjector = RefRepo{}

// InsertMinigameConfigVersion implements write.MinigameConfigProjector: the projection ref.minigame_config_versions is
// insert-only (F073-AC-04); a replayed event or another event about a version already projected changes nothing.
func (RefRepo) InsertMinigameConfigVersion(ctx context.Context, v dto.MinigameConfigVersion) (bool, error) {
	grades := make([]string, len(v.GradeLevels))
	for i, g := range v.GradeLevels {
		grades[i] = strconv.Itoa(g)
	}
	// tenancyguard:ok projection of platform content (no tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2), learning_platform only
	return exec(ctx, `INSERT INTO ref.minigame_config_versions (minigame_config_version_id, minigame_config_id, version, minigame_type,
			grade_levels, snapshot, published_at, source_version)
		VALUES ($1, $2, $3, $4, $5::smallint[], $6::jsonb, $7, $8)
		ON CONFLICT DO NOTHING`, v.ID, v.ConfigID, v.Version, v.Type, "{"+strings.Join(grades, ",")+"}", string(v.Snapshot),
		v.PublishedAt.UTC(), v.SourceVersion)
}

// SetMinigamePointer implements write.MinigameConfigProjector (F078): the current version of a mini-game config follows
// the release pointer of module game (publish, rollback, unpublish); an older event (source_version) changes nothing.
func (RefRepo) SetMinigamePointer(ctx context.Context, configID uuid.UUID, versionID *uuid.UUID, sourceVersion int64, at time.Time) (bool, error) {
	// tenancyguard:ok projection of platform content (no tenant_id, no deleted_at: DATABASE_DESIGN §2.17 rule 2), learning_platform only
	return exec(ctx, `INSERT INTO ref.minigame_config_pointers (minigame_config_id, minigame_config_version_id, source_version, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (minigame_config_id) DO UPDATE SET minigame_config_version_id = EXCLUDED.minigame_config_version_id,
			source_version = EXCLUDED.source_version, updated_at = EXCLUDED.updated_at
		WHERE ref.minigame_config_pointers.source_version < EXCLUDED.source_version`, configID, versionID, sourceVersion, at.UTC())
}
