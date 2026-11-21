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
	tenant, school, year, otherSchool string
}

// seedTenant creates a tenant of type unit with two schools and a year each (as platform role).
func seedTenant(ctx context.Context, t *testing.T, platform *sql.DB, code string) fixture {
	t.Helper()
	f := fixture{tenant: ids.New().String(), school: ids.New().String(), year: ids.New().String(), otherSchool: ids.New().String()}
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
	for _, s := range [][2]string{{f.school, f.year}, {f.otherSchool, ids.New().String()}} {
		exec(`INSERT INTO org.schools (school_id, tenant_id, created_at, updated_at, code, name, country_code, province_id,
			school_level, grade_min, grade_max, settings) VALUES ($1, $2, now(), now(), $3, $3, 'VN', $4, 'primary', 1, 5, '{}')`,
			s[0], f.tenant, code+"-"+s[0][len(s[0])-6:], province)
		exec(`INSERT INTO org.academic_years (academic_year_id, tenant_id, created_at, updated_at, school_id, name, start_date, end_date, is_current)
			VALUES ($1, $2, now(), now(), $3, '2026-2027', '2026-09-01', '2027-05-31', true)`, s[1], f.tenant, s[0])
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

func TestOrgSchema(t *testing.T) {
	ctx := context.Background()
	cl := testkit.Isolated(ctx, t)
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
		// ADR-0095 mục 8 (WP-R): org.classes (and the 9 virtual classes common-g1…g9 of the seed) and org.enrollments
		// are dropped; the CHECK values of the removed SIS kinds are gone on a database where no row carried them.
		if n := count(ctx, t, platform, `SELECT count(*) FROM (VALUES (to_regclass('org.classes')), (to_regclass('org.enrollments')),
			(to_regclass('org.classes_live')), (to_regclass('org.enrollments_live'))) v(r) WHERE r IS NOT NULL`); n != 0 {
			t.Fatalf("%d class/enrollment relations left (ADR-0095 mục 8)", n)
		}
		if n := count(ctx, t, platform, `SELECT count(*) FROM pg_constraint WHERE conname IN ('sis_links_type_chk', 'staging_records_entity_kind_check')
			AND conrelid IN ('sis.sis_links'::regclass, 'sis.staging_records'::regclass)
			AND pg_get_constraintdef(oid) NOT LIKE '%class%' AND pg_get_constraintdef(oid) NOT LIKE '%enrollment%'`); n != 2 {
			t.Fatalf("narrowed SIS CHECKs = %d, want 2", n)
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
		for _, table := range []string{"org.schools", "org.academic_years"} {
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
			_, err := tx.ExecContext(ctx, `INSERT INTO org.academic_years (academic_year_id, tenant_id, created_at, updated_at, school_id, name, start_date, end_date)
				VALUES ($1, $2, now(), now(), $3, '2027-2028', '2027-09-01', '2028-05-31')`, ids.New().String(), b.tenant, b.school)
			return err
		})
		if pgCode(err) != "42501" {
			t.Fatalf("insert into tenant B from A: %v (want RLS 42501)", err)
		}
		err = inTenant(ctx, t, app, a.tenant, false, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM org.academic_years WHERE academic_year_id = $1", a.year) // tenancyguard:ok asserts DELETE is refused
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
			student, err = insertStudent(ctx, tx, a, "hs.member")
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := count(ctx, t, platform, "SELECT count(*) FROM identity.tenant_memberships_v WHERE user_id = $1 AND tenant_id = $2", student, a.tenant); n != 1 {
			t.Fatalf("membership rows = %d, want 1", n)
		}
	})
}
