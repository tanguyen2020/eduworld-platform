package read

import (
	"context"
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

// Columns common to every query: entity id, tenant id, version, deleted flag, then entity columns.
var snapshotSQL = map[string]string{
	"tenant": `SELECT t.tenant_id, t.tenant_id, t.version, t.deleted_at IS NOT NULL,
		json_build_object('tenant_id', t.tenant_id, 'code', t.code, 'name', t.name, 'tenant_type', t.tenant_type, 'status', t.status,
			'keycloak_org_alias', t.keycloak_org_alias, 'is_qa', t.is_qa,
			-- settings_subset as tenant.updated publishes it (settings.TenantSubset, F015 T044): both social keys, false unless true
			'settings_subset', json_build_object(
				'cross_school_social_enabled', coalesce(t.settings->'cross_school_social_enabled' = 'true'::jsonb, false),
				'common_social_enabled', coalesce(t.settings->'common_social_enabled' = 'true'::jsonb, false)))
		FROM platform.tenants t
		WHERE ($1::uuid IS NULL OR t.tenant_id = $1) AND ($2::uuid IS NULL OR t.tenant_id > $2)
		ORDER BY t.tenant_id LIMIT $3`,
	"school": `SELECT s.school_id, s.tenant_id, s.version, s.deleted_at IS NOT NULL,
		json_build_object('school_id', s.school_id, 'tenant_id', s.tenant_id, 'status', s.status, 'name', s.name,
			'name_i18n', coalesce(s.name_i18n, json_build_object('vi', s.name)::jsonb), 'province_code', p.code, 'school_level', s.school_level,
			'is_virtual', s.is_virtual, 'settings_subset', '{}'::json)
		FROM org.schools s LEFT JOIN catalog.provinces p ON p.province_id = s.province_id
		WHERE ($1::uuid IS NULL OR s.tenant_id = $1) AND ($2::uuid IS NULL OR s.school_id > $2)
		ORDER BY s.school_id LIMIT $3`,
	"class": `SELECT c.class_id, c.tenant_id, c.version, c.deleted_at IS NOT NULL,
		json_build_object('class_id', c.class_id, 'school_id', c.school_id, 'grade_level', c.grade_level, 'name', c.name,
			'academic_year', y.name, 'status', c.status)
		FROM org.classes c JOIN org.academic_years y ON y.academic_year_id = c.academic_year_id AND y.tenant_id = c.tenant_id
		WHERE ($1::uuid IS NULL OR c.tenant_id = $1) AND ($2::uuid IS NULL OR c.class_id > $2)
		ORDER BY c.class_id LIMIT $3`,
	"student": `SELECT st.student_id, st.tenant_id, st.version, st.deleted_at IS NOT NULL,
		json_build_object('student_id', st.student_id, 'school_id', st.school_id, 'grade_level', st.grade_level, 'status', st.status,
			'source_version', st.version)
		FROM org.students st
		WHERE ($1::uuid IS NULL OR st.tenant_id = $1) AND ($2::uuid IS NULL OR st.student_id > $2)
		ORDER BY st.student_id LIMIT $3`,
	"enrollment": `SELECT e.enrollment_id, e.tenant_id, e.version, e.deleted_at IS NOT NULL,
		json_build_object('enrollment_id', e.enrollment_id, 'student_id', e.student_id, 'class_id', e.class_id, 'school_id', e.school_id,
			'status', e.status, 'start_date', e.start_date, 'end_date', e.end_date, 'source', e.source, 'source_version', e.version)
		FROM org.enrollments e
		WHERE ($1::uuid IS NULL OR e.tenant_id = $1) AND ($2::uuid IS NULL OR e.enrollment_id > $2)
		ORDER BY e.enrollment_id LIMIT $3`,
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
			if err := rows.Scan(&it.EntityID, &it.TenantID, &it.SourceVersion, &it.Deleted, &raw); err != nil {
				return err
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
