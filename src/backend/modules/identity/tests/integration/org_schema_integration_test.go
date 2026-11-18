//go:build integration

// Package integration holds the integration tests of module identity (testcontainers PostgreSQL with
// every Atlas migration applied).
package integration

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

const commonTenant = "0193a000-0000-7000-8000-000000000001"

func open(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// inTenant runs f in a transaction of db with app.tenant_id = tenant ("" = unset), always rolled back
// unless commit is true.
func inTenant(ctx context.Context, t *testing.T, db *sql.DB, tenant string, commit bool, f func(tx *sql.Tx) error) error {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tenant != "" {
		if _, err := tx.ExecContext(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant); err != nil {
			t.Fatal(err)
		}
	}
	ferr := f(tx)
	if commit && ferr == nil {
		if err := tx.Commit(); err != nil {
			return err
		}
		return nil
	}
	_ = tx.Rollback()
	return ferr
}

func count(ctx context.Context, t *testing.T, q sqlQuerier, query string, args ...any) int {
	t.Helper()
	var n int
	if err := q.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

type sqlQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type fixture struct {
	tenant, school, year, class, otherSchool, otherClass string
}

// seedTenant creates a tenant of type unit with two schools, a year and a class each (as platform role).
func seedTenant(ctx context.Context, t *testing.T, platform *sql.DB, code string) fixture {
	t.Helper()
	f := fixture{tenant: ids.New().String(), school: ids.New().String(), year: ids.New().String(), class: ids.New().String(),
		otherSchool: ids.New().String(), otherClass: ids.New().String()}
	var province string
	if err := platform.QueryRowContext(ctx, "SELECT province_id FROM catalog.provinces WHERE code = 'vn.ha-noi'").Scan(&province); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := platform.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO platform.tenants (tenant_id, created_at, updated_at, code, name, tenant_type, keycloak_org_alias, settings)
		VALUES ($1, now(), now(), $2, $2, 'unit', $2, '{}')`, f.tenant, code)
	for _, s := range [][3]string{{f.school, f.year, f.class}, {f.otherSchool, ids.New().String(), f.otherClass}} {
		exec(`INSERT INTO org.schools (school_id, tenant_id, created_at, updated_at, code, name, country_code, province_id,
			school_level, grade_min, grade_max, settings) VALUES ($1, $2, now(), now(), $3, $3, 'VN', $4, 'primary', 1, 5, '{}')`,
			s[0], f.tenant, code+"-"+s[0][len(s[0])-6:], province)
		exec(`INSERT INTO org.academic_years (academic_year_id, tenant_id, created_at, updated_at, school_id, name, start_date, end_date, is_current)
			VALUES ($1, $2, now(), now(), $3, '2026-2027', '2026-09-01', '2027-05-31', true)`, s[1], f.tenant, s[0])
		exec(`INSERT INTO org.classes (class_id, tenant_id, created_at, updated_at, school_id, academic_year_id, grade_level, name)
			VALUES ($1, $2, now(), now(), $3, $4, 3, '3A')`, s[2], f.tenant, s[0], s[1])
	}
	return f
}

func insertStudent(ctx context.Context, tx *sql.Tx, f fixture, username string) (string, error) {
	id := ids.New().String()
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, primary_tenant_id, provisioned_at)
		VALUES ($1, now(), now(), 'student', $2, $2, $3, now())`, id, username, f.tenant); err != nil {
		return "", err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO org.students (student_id, tenant_id, created_at, updated_at, school_id, grade_level, full_name)
		VALUES ($1, $2, now(), now(), $3, 3, 'Hoc Sinh')`, id, f.tenant, f.school)
	return id, err
}

func enroll(ctx context.Context, tx *sql.Tx, f fixture, student, school, class string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO org.enrollments (enrollment_id, tenant_id, created_at, updated_at, school_id, student_id, class_id, status, start_date, source)
		VALUES ($1, $2, now(), now(), $3, $4, $5, 'active', current_date, 'manual')`, ids.New().String(), f.tenant, school, student, class)
	return err
}

func TestOrgSchema(t *testing.T) {
	ctx := context.Background()
	cl := testkit.StartCluster(ctx, t, true)
	app := open(t, cl.ModuleDSN("identity", "app"))
	platform := open(t, cl.ModuleDSN("identity", "platform"))
	a := seedTenant(ctx, t, platform, "pgd-a")
	b := seedTenant(ctx, t, platform, "pgd-b")

	// F010-AC-06 (TC-ORG-101): catalog seeded from specs/reference-data.
	t.Run("CatalogSeed", func(t *testing.T) {
		if n := count(ctx, t, app, "SELECT count(*) FROM catalog.provinces WHERE country_code = 'VN' AND status = 'active' AND deleted_at IS NULL"); n != 34 {
			t.Fatalf("active provinces = %d, want 34", n)
		}
		if n := count(ctx, t, app, "SELECT count(*) FROM catalog.provinces WHERE country_code = 'VN' AND status = 'inactive'"); n != 63 {
			t.Fatalf("former provinces = %d, want 63", n)
		}
		if n := count(ctx, t, app, `SELECT count(*) FROM catalog.provinces p WHERE p.status = 'inactive' AND NOT EXISTS (
			SELECT 1 FROM catalog.province_successions s JOIN catalog.provinces n ON n.province_id = s.new_province_id
			WHERE s.old_province_id = p.province_id AND n.status = 'active')`); n != 0 {
			t.Fatalf("%d former provinces without an active successor", n)
		}
		if n := count(ctx, t, app, "SELECT count(*) FROM catalog.countries WHERE status = 'active'"); n != 1 {
			t.Fatalf("active countries = %d, want 1 (VN)", n)
		}
	})

	t.Run("CommonTenantSeed", func(t *testing.T) {
		if n := count(ctx, t, platform, "SELECT count(*) FROM platform.tenants WHERE tenant_type = 'common' AND tenant_id = $1", commonTenant); n != 1 {
			t.Fatal("common tenant missing")
		}
		if n := count(ctx, t, platform, "SELECT count(*) FROM org.classes WHERE tenant_id = $1 AND is_virtual AND name LIKE 'common-g%'", commonTenant); n != 9 {
			t.Fatalf("virtual classes = %d, want 9", n)
		}
		// F010-AC-02 (DB layer): a second common tenant violates tenants_single_common_uq.
		_, err := platform.ExecContext(ctx, `INSERT INTO platform.tenants (tenant_id, created_at, updated_at, code, name, tenant_type, settings)
			VALUES ($1, now(), now(), 'common2', 'x', 'common', '{}')`, ids.New().String())
		if pgCode(err) != "23505" {
			t.Fatalf("second common tenant: %v", err)
		}
	})

	// T009: RLS on every new T table — tenant A never sees B, unset tenant sees nothing, no DELETE.
	t.Run("RLS", func(t *testing.T) {
		for _, table := range []string{"org.schools", "org.academic_years", "org.classes"} {
			if n := count(ctx, t, app, "SELECT count(*) FROM "+table); n != 0 {
				t.Fatalf("%s without app.tenant_id returned %d rows", table, n)
			}
			_ = inTenant(ctx, t, app, a.tenant, false, func(tx *sql.Tx) error {
				if n := count(ctx, t, tx, "SELECT count(*) FROM "+table+" WHERE tenant_id = $1", b.tenant); n != 0 {
					t.Fatalf("%s: tenant A read %d rows of B", table, n)
				}
				if n := count(ctx, t, tx, "SELECT count(*) FROM "+table); n != 2 {
					t.Fatalf("%s: tenant A sees %d rows, want 2", table, n)
				}
				return nil
			})
		}
		err := inTenant(ctx, t, app, a.tenant, false, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO org.classes (class_id, tenant_id, created_at, updated_at, school_id, academic_year_id, grade_level, name)
				VALUES ($1, $2, now(), now(), $3, $4, 3, '3Z')`, ids.New().String(), b.tenant, b.school, b.year)
			return err
		})
		if pgCode(err) != "42501" {
			t.Fatalf("insert into tenant B from A: %v (want RLS 42501)", err)
		}
		err = inTenant(ctx, t, app, a.tenant, false, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM org.classes WHERE class_id = $1", a.class) // tenancyguard:ok asserts DELETE is refused
			return err
		})
		if pgCode(err) != "42501" {
			t.Fatalf("DELETE by identity_app: %v (want 42501)", err)
		}
		err = inTenant(ctx, t, app, a.tenant, false, func(tx *sql.Tx) error {
			r, err := tx.ExecContext(ctx, "UPDATE org.schools SET name = 'x' WHERE school_id = $1", b.school)
			if err != nil {
				return err
			}
			if n, _ := r.RowsAffected(); n != 0 {
				t.Fatalf("tenant A updated %d rows of B", n)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	// F010-AC-11, F010-AC-12: one active enrollment; class of the enrollment's school (composite FK).
	t.Run("EnrollmentInvariants", func(t *testing.T) {
		err := inTenant(ctx, t, app, a.tenant, false, func(tx *sql.Tx) error {
			s, err := insertStudent(ctx, tx, a, "hs.inv.1")
			if err != nil {
				return err
			}
			if err := enroll(ctx, tx, a, s, a.school, a.class); err != nil {
				return err
			}
			if err := enroll(ctx, tx, a, s, a.otherSchool, a.otherClass); pgCode(err) != "23505" {
				t.Fatalf("second active enrollment: %v (want 23505)", err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		err = inTenant(ctx, t, app, a.tenant, false, func(tx *sql.Tx) error {
			s, err := insertStudent(ctx, tx, a, "hs.inv.2")
			if err != nil {
				return err
			}
			return enroll(ctx, tx, a, s, a.school, a.otherClass) // class of another school
		})
		if pgCode(err) != "23503" {
			t.Fatalf("class of another school: %v (want FK 23503)", err)
		}
	})

	// F010-AC-16: user_type ∈ {student, staff}; students have a primary tenant; username normalized.
	t.Run("UserChecks", func(t *testing.T) {
		for name, q := range map[string]string{
			"teacher type":       `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, email, provisioned_at) VALUES ($1, now(), now(), 'teacher', 'gv.x', 'x', 'x@x.vn', now())`,
			"student w/o tenant": `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, provisioned_at) VALUES ($1, now(), now(), 'student', 'hs.x', 'x', now())`,
			"username case":      `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, email, provisioned_at) VALUES ($1, now(), now(), 'staff', 'GV.Y', 'y', 'y@x.vn', now())`,
			"staff w/o email":    `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, provisioned_at) VALUES ($1, now(), now(), 'staff', 'gv.z', 'z', now())`,
		} {
			if _, err := platform.ExecContext(ctx, q, ids.New().String()); pgCode(err) != "23514" {
				t.Errorf("%s: %v (want CHECK 23514)", name, err)
			}
		}
		_, err := platform.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, email, provisioned_at)
			VALUES ($1, now(), now(), 'staff', 'gv.dup', 'd', 'dup@x.vn', now())`, ids.New().String())
		if err != nil {
			t.Fatal(err)
		}
		_, err = platform.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, email, provisioned_at)
			VALUES ($1, now(), now(), 'staff', 'gv.dup', 'd', 'dup2@x.vn', now())`, ids.New().String())
		if pgCode(err) != "23505" || !strings.Contains(err.Error(), "users_username_uq") {
			t.Fatalf("duplicate username: %v", err)
		}
	})

	t.Run("MembershipView", func(t *testing.T) {
		var student string
		err := inTenant(ctx, t, app, a.tenant, true, func(tx *sql.Tx) error {
			var err error
			if student, err = insertStudent(ctx, tx, a, "hs.member"); err != nil {
				return err
			}
			return enroll(ctx, tx, a, student, a.school, a.class)
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := count(ctx, t, platform, "SELECT count(*) FROM identity.tenant_memberships_v WHERE user_id = $1 AND tenant_id = $2", student, a.tenant); n != 1 {
			t.Fatalf("membership rows = %d, want 1", n)
		}
	})
}
