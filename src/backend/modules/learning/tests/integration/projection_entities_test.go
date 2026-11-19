//go:build integration

package integration

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// columnsOf lists the columns of a ref table.
func columnsOf(ctx context.Context, t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = 'ref'
		AND table_name = $1 ORDER BY column_name`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

// F020-AC-02 (TC-SVC-008, NFR-PRIV-002): each identity event updates the right ref.* table with exactly the
// minimal columns of DATABASE_DESIGN §2.17 (no extra column, no PII).
func TestProjectionEntities(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, false)

	want := map[string][]string{
		"tenants":     {"code", "is_qa", "settings_subset", "source_version", "status", "tenant_id", "tenant_type"},
		"provinces":   {"code", "country_code", "name_i18n", "source_version", "status"},
		"schools":     {"is_virtual", "name", "name_i18n", "province_code", "province_name_i18n", "school_id", "school_level", "settings_subset", "source_version", "status", "tenant_id"},
		"classes":     {"academic_year_id", "class_id", "grade_level", "is_virtual", "name", "school_id", "source_version", "tenant_id"},
		"students":    {"deleted_at", "grade_level", "school_id", "source_version", "status", "student_id", "tenant_id"},
		"enrollments": {"class_id", "end_date", "source_version", "start_date", "status", "student_id", "tenant_id"},
	}
	for table, cols := range want {
		if got := columnsOf(ctx, t, e.migrator, table); !slices.Equal(got, cols) {
			t.Errorf("ref.%s columns = %v, want %v", table, got, cols)
		}
	}

	p := startProjection(ctx, t, e)
	tenant, school, class, student := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	p.processed(ctx, t, p.publish(ctx, t, "tenant.created", &tenant, tenantPayload(tenant, "th-demo-a", "active", 1)))

	// province.updated (platform event) → ref.provinces.
	p.processed(ctx, t, p.publish(ctx, t, "province.updated", nil, map[string]any{"province_id": uuid.NewString(),
		"code": "vn.ha-noi", "country_code": "VN", "name": map[string]string{"vi": "Hà Nội", "en": "Ha Noi"}, "status": "active", "op": "created"}))
	var pname string
	if err := e.migrator.QueryRowContext(ctx, `SELECT name_i18n->>'vi' FROM ref.provinces WHERE code = 'vn.ha-noi'`).Scan(&pname); err != nil || pname != "Hà Nội" {
		t.Fatalf("ref.provinces: %q %v", pname, err)
	}

	// school.updated → ref.schools with the province name denormalized.
	p.processed(ctx, t, p.publish(ctx, t, "school.updated", &tenant, map[string]any{"school_id": school.String(), "tenant_id": tenant.String(),
		"status": "active", "name": "TH Demo A", "province_code": "vn.ha-noi", "school_level": "primary", "is_virtual": false,
		"settings_subset": map[string]any{"class_leaderboard_enabled": true}, "changed_fields": []string{}, "source_version": 1}))
	var sname, level, prov string
	if err := e.migrator.QueryRowContext(ctx, `SELECT name, school_level, province_name_i18n->>'vi' FROM ref.schools
		WHERE school_id = $1 AND tenant_id = $2`, school, tenant).Scan(&sname, &level, &prov); err != nil || sname != "TH Demo A" || level != "primary" || prov != "Hà Nội" {
		t.Fatalf("ref.schools: %q %q %q %v", sname, level, prov, err)
	}

	// class.synced → ref.classes (no source_version: the envelope time orders events).
	p.processed(ctx, t, p.publish(ctx, t, "class.synced", &tenant, map[string]any{"class_id": class.String(), "school_id": school.String(),
		"grade_level": 3, "name": "3A", "academic_year": "2026-2027", "status": "active", "op": "created", "trigger": "manual_import"}))
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.classes WHERE class_id = $1 AND tenant_id = $2 AND school_id = $3
		AND grade_level = 3 AND name = '3A'`, class, tenant, school); n != 1 {
		t.Fatalf("ref.classes rows = %d", n)
	}

	// student.synced → ref.students; enrollment.changed → ref.enrollments.
	p.processed(ctx, t, p.publish(ctx, t, "student.synced", &tenant, map[string]any{"student_id": student.String(), "school_id": school.String(),
		"action": "create", "sync_run_id": uuid.NewString(), "grade_level": 3, "source_version": 1}))
	p.processed(ctx, t, p.publish(ctx, t, "enrollment.changed", &tenant, map[string]any{"student_id": student.String(), "class_id": class.String(),
		"op": "added", "school_id": school.String(), "status": "active", "start_date": "2026-09-05", "source": "sis", "source_version": 1}))
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND tenant_id = $2 AND grade_level = 3
		AND status = 'active' AND deleted_at IS NULL`, student, tenant); n != 1 {
		t.Fatalf("ref.students rows = %d", n)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.enrollments WHERE student_id = $1 AND class_id = $2
		AND start_date = '2026-09-05' AND status = 'active' AND end_date IS NULL`, student, class); n != 1 {
		t.Fatalf("ref.enrollments rows = %d", n)
	}
	p.processed(ctx, t, p.publish(ctx, t, "enrollment.changed", &tenant, map[string]any{"student_id": student.String(), "class_id": class.String(),
		"op": "ended", "status": "ended", "start_date": "2026-09-05", "end_date": "2027-05-31", "source_version": 2}))
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.enrollments WHERE student_id = $1 AND status = 'ended'
		AND end_date = '2027-05-31'`, student); n != 1 {
		t.Fatalf("ended enrollment rows = %d", n)
	}

	// student.cleaned_up / student.restored → deleted_at of the projection.
	p.processed(ctx, t, p.publish(ctx, t, "student.cleaned_up", &tenant, map[string]any{"student_id": student.String(), "tenant_id": tenant.String(),
		"last_login_at": nil, "cutoff_days": 180, "run_id": uuid.NewString()}))
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND deleted_at IS NOT NULL`, student); n != 1 {
		t.Fatal("student.cleaned_up not projected")
	}
	p.processed(ctx, t, p.publish(ctx, t, "student.restored", &tenant, map[string]any{"student_id": student.String(), "tenant_id": tenant.String(),
		"cleaned_up_at": "2026-11-01T00:00:00Z", "restored_via": "login"}))
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND deleted_at IS NULL`, student); n != 1 {
		t.Fatal("student.restored not projected")
	}

	// Nothing personal was stored: no name column on students, no free text beyond school/class names.
	for _, c := range columnsOf(ctx, t, e.migrator, "students") {
		if strings.Contains(c, "name") || strings.Contains(c, "birth") || strings.Contains(c, "code") {
			t.Fatalf("ref.students has personal column %s", c)
		}
	}
}
