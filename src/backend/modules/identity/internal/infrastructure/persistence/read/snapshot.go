package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// SnapshotModel implements read.SnapshotReader with explicit SQL on the base tables: soft-deleted rows are
// part of a snapshot (deleted flag), so the ent soft-delete filter cannot be used. Every query is limited
// to the requested tenant; without tenant the caller runs under tenancy.Bypass(platform_job).
type SnapshotModel struct{ Session *entx.ReadSession }

var _ read.SnapshotReader = SnapshotModel{}

// Columns common to every query: entity id, tenant id, version, deleted flag, deleted_at, then entity columns.
var snapshotSQL = map[string]string{
	"tenant": `SELECT t.tenant_id, t.tenant_id, t.version, t.deleted_at IS NOT NULL, t.deleted_at,
		json_build_object('tenant_id', t.tenant_id, 'code', t.code, 'name', t.name, 'tenant_type', t.tenant_type, 'status', t.status,
			'keycloak_org_alias', t.keycloak_org_alias, 'is_qa', t.is_qa,
			-- settings_subset as tenant.updated publishes it (settings.TenantSubset, F015 T044): both social keys, false unless true
			'settings_subset', json_build_object(
				'cross_school_social_enabled', coalesce(t.settings->'cross_school_social_enabled' = 'true'::jsonb, false),
				'common_social_enabled', coalesce(t.settings->'common_social_enabled' = 'true'::jsonb, false)))
		FROM platform.tenants t
		WHERE ($1::uuid IS NULL OR t.tenant_id = $1) AND ($2::uuid IS NULL OR t.tenant_id > $2)
		ORDER BY t.tenant_id LIMIT $3`,
	"school": `SELECT s.school_id, s.tenant_id, s.version, s.deleted_at IS NOT NULL, s.deleted_at,
		json_build_object('school_id', s.school_id, 'tenant_id', s.tenant_id, 'status', s.status, 'name', s.name,
			'name_i18n', coalesce(s.name_i18n, json_build_object('vi', s.name)::jsonb), 'province_code', p.code, 'school_level', s.school_level,
			'is_virtual', s.is_virtual, 'settings_subset', '{}'::json)
		FROM org.schools s LEFT JOIN catalog.provinces p ON p.province_id = s.province_id
		WHERE ($1::uuid IS NULL OR s.tenant_id = $1) AND ($2::uuid IS NULL OR s.school_id > $2)
		ORDER BY s.school_id LIMIT $3`,
	"student": `SELECT st.student_id, st.tenant_id, st.version, st.deleted_at IS NOT NULL, st.deleted_at,
		-- the placement as student.placement_changed publishes it (ADR-0095): soft-deleted rows are inactive (deleted = true)
		json_build_object('student_id', st.student_id, 'tenant_id', st.tenant_id, 'school_id', st.school_id, 'grade_level', st.grade_level,
			'status', CASE WHEN st.status = 'active' AND st.deleted_at IS NULL THEN 'active' ELSE 'inactive' END,
			'academic_year_id', (SELECT y.academic_year_id FROM org.academic_years y WHERE y.school_id = st.school_id
				AND y.tenant_id = st.tenant_id AND y.is_current AND y.deleted_at IS NULL ORDER BY y.start_date DESC LIMIT 1),
			'source_version', st.version)
		FROM org.students st
		WHERE ($1::uuid IS NULL OR st.tenant_id = $1) AND ($2::uuid IS NULL OR st.student_id > $2)
		ORDER BY st.student_id LIMIT $3`,
}

// Snapshot returns up to limit items of entityType ordered by id after afterID.
func (m SnapshotModel) Snapshot(ctx context.Context, entityType string, tenantID, afterID *uuid.UUID, limit int) (out []read.SnapshotItem, err error) {
	q, ok := snapshotSQL[entityType]
	if !ok {
		return nil, fmt.Errorf("snapshot: unknown entity type %q", entityType)
	}
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		rows, err := tx.QueryContext(ctx, q, tenantID, afterID, limit)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			it := read.SnapshotItem{EntityType: entityType}
			var raw []byte
			var deletedAt sql.NullTime
			if err := rows.Scan(&it.EntityID, &it.TenantID, &it.SourceVersion, &it.Deleted, &deletedAt, &raw); err != nil {
				return err
			}
			if deletedAt.Valid {
				at := deletedAt.Time.UTC()
				it.DeletedAt = &at
			}
			if err := json.Unmarshal(raw, &it.Data); err != nil {
				return err
			}
			normalizeDates(it.Data)
			out = append(out, it)
		}
		return rows.Err()
	})
	return out, err
}

// normalizeDates turns timestamps of date columns into YYYY-MM-DD (json_build_object of a date is a
// string already; this keeps the contract format if the driver returns a timestamp).
func normalizeDates(d map[string]any) {
	for _, k := range []string{"start_date", "end_date"} {
		if s, ok := d[k].(string); ok && len(s) > 10 {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				d[k] = t.Format(time.DateOnly)
			}
		}
	}
}
