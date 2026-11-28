//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring/wiringtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// sourceStudent is one org.students row of the identity fixture.
type sourceStudent struct {
	id, tenant, school uuid.UUID
	grade              int
	active, deleted    bool
}

// seedIdentity writes a tenant with one school and its students straight into eduworld_identity (superuser): the
// rebuild reads them back through listProjectionSnapshot.
func seedIdentity(t *testing.T, db *sql.DB, code, status string, students []sourceStudent) (tenant, school uuid.UUID) {
	t.Helper()
	tenant, school = ids.New(), ids.New()
	var province string
	if err := db.QueryRow(`SELECT province_id FROM catalog.provinces WHERE code = 'vn.ha-noi'`).Scan(&province); err != nil {
		t.Fatal(err)
	}
	exec(t, db, `INSERT INTO platform.tenants (tenant_id, created_at, updated_at, code, name, tenant_type, keycloak_org_alias, settings, status, version)
		VALUES ($1, now(), now(), $2, $2, 'unit', $2, '{}', $3, 4)`, tenant, code, status)
	exec(t, db, `INSERT INTO org.schools (school_id, tenant_id, created_at, updated_at, code, name, country_code, province_id, school_level,
		grade_min, grade_max, settings, version) VALUES ($1, $2, now(), now(), $3, $3, 'VN', $4, 'primary', 1, 5, '{}', 2)`, school, tenant, code+"-s", province)
	return tenant, school
}

func seedStudent(t *testing.T, db *sql.DB, s sourceStudent) {
	t.Helper()
	exec(t, db, `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, primary_tenant_id, provisioned_at)
		VALUES ($1, now(), now(), 'student', $2, $2, $3, now())`, s.id, "hs."+s.id.String()[24:], s.tenant)
	status := "inactive"
	if s.active {
		status = "active"
	}
	exec(t, db, `INSERT INTO org.students (student_id, tenant_id, created_at, updated_at, school_id, grade_level, full_name, status, version, deleted_at)
		VALUES ($1, $2, now(), now(), $3, $4, 'Hoc Sinh', $5, 3, CASE WHEN $6::boolean THEN now() - interval '2 days' END)`,
		s.id, s.tenant, s.school, s.grade, status, s.deleted)
}

// refState is the projected state of the fixture in one module: tenants, schools and students as text rows.
func refState(t *testing.T, db *sql.DB, tenants []uuid.UUID) []string {
	t.Helper()
	var out []string
	query := func(q string, args ...any) {
		rows, err := db.Query(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
	for _, tn := range tenants {
		query(`SELECT concat_ws('|', 'tenant', tenant_id, tenant_type, status, source_version) FROM ref.tenants WHERE tenant_id = $1`, tn)
		query(`SELECT concat_ws('|', 'school', school_id, tenant_id, status, source_version) FROM ref.schools WHERE tenant_id = $1 ORDER BY school_id`, tn)
		query(`SELECT concat_ws('|', 'student', student_id, tenant_id, school_id, grade_level, status, source_version, coalesce(deleted_at::text, 'live'))
			FROM ref.students WHERE tenant_id = $1 ORDER BY student_id`, tn)
	}
	return out
}

// F003-AC-39 (TC-SVC-014), F003 Clarifications 2026-11-28: `worker projection rebuild --module=<m> --source=identity` rebuilds
// ref.tenants, ref.schools and ref.students of learning, game, competition and chat from listProjectionSnapshot through the
// projection consumer of the module, on the real module databases: a wiped table equals the identity state again, a rerun
// changes nothing (deleted_at included), a newer projected row is kept, and a student projected in another tenant is never
// moved there (exit ≠ 0, the row stays in its tenant).
func TestProjectionRebuildFromIdentity(t *testing.T) {
	ctx := context.Background()
	cluster := testkit.StartCluster(ctx, t, true)
	_, vkAddr := testkit.StartValkey(ctx, t)
	env := wiringtest.Env(wiringtest.Options{RepoRoot: testkit.RepoRoot(t), DSN: cluster.ModuleDSN, Issuer: "http://127.0.0.1:1/realms/eduworld",
		ValkeyAddr: vkAddr, NATSURL: "nats://127.0.0.1:1", SecretsDir: t.TempDir()})
	getenv := func(k string) string { return env[k] }
	super := func(m string) *sql.DB { return open(t, cluster.DSN("postgres", "eduworld_"+m)) }
	identityDB := super("identity")

	ta, sa := seedIdentity(t, identityDB, "pgd-rb-a", "active", nil)
	tb, sb := seedIdentity(t, identityDB, "pgd-rb-b", "disabled", nil)
	students := []sourceStudent{
		{id: ids.New(), tenant: ta, school: sa, grade: 3, active: true},
		{id: ids.New(), tenant: ta, school: sa, grade: 4, active: false},
		{id: ids.New(), tenant: ta, school: sa, grade: 5, deleted: true},
		{id: ids.New(), tenant: tb, school: sb, grade: 2, active: true},
	}
	for _, s := range students {
		seedStudent(t, identityDB, s)
	}
	var deletedAt time.Time
	if err := identityDB.QueryRow(`SELECT deleted_at FROM org.students WHERE student_id = $1`, students[2].id).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}

	reg, closeFn, err := composeProjection(getenv)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	rebuild := func(m string, extra ...string) (string, error) {
		var out bytes.Buffer
		err := runProjection(ctx, append([]string{"rebuild", "--module=" + m, "--source=identity"}, extra...), reg, &out)
		return out.String(), err
	}
	// expected is the identity state as each module projects it (status vocabulary of its consumer).
	expected := func(m string) []string {
		placement := m == "learning" || m == "competition"
		out := []string{
			fmt.Sprintf("tenant|%s|unit|active|4", ta), fmt.Sprintf("school|%s|%s|active|2", sa, ta),
		}
		var forA, forB []string
		for _, s := range students {
			status := "active"
			if !s.active {
				status = map[bool]string{true: "inactive", false: "deactivated"}[placement]
			}
			del := "live"
			if s.deleted {
				var at string
				db := super(m)
				if err := db.QueryRow(`SELECT $1::timestamptz::text`, deletedAt).Scan(&at); err != nil {
					t.Fatal(err)
				}
				del = at
			}
			row := fmt.Sprintf("student|%s|%s|%s|%d|%s|3|%s", s.id, s.tenant, s.school, s.grade, status, del)
			if s.tenant == ta {
				forA = append(forA, row)
			} else {
				forB = append(forB, row)
			}
		}
		sortStrings(forA)
		out = append(out, forA...)
		out = append(out, fmt.Sprintf("tenant|%s|unit|disabled|4", tb), fmt.Sprintf("school|%s|%s|active|2", sb, tb))
		return append(out, forB...)
	}
	tenants := []uuid.UUID{ta, tb}
	for _, m := range wiring.IdentityProjectionModules {
		db := super(m)
		out, err := rebuild(m)
		if err != nil {
			t.Fatalf("%s: rebuild: %v\n%s", m, err, out)
		}
		for _, table := range []string{"ref.schools", "ref.students", "ref.tenants"} {
			if !strings.Contains(out, table+":") {
				t.Fatalf("%s: %s not rebuilt:\n%s", m, table, out)
			}
		}
		if got, want := refState(t, db, tenants), expected(m); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%s: projection after rebuild\n got %v\nwant %v", m, got, want)
		}
		// The projection lost the students of tenant A (restore of the module database): the rebuild of ref.students alone
		// brings them back; a rerun changes nothing.
		exec(t, db, `DELETE FROM ref.students WHERE tenant_id = $1`, ta)
		if out, err := rebuild(m, "--table=ref.students"); err != nil || strings.Contains(out, "ref.tenants") {
			t.Fatalf("%s: rebuild ref.students: %v\n%s", m, err, out)
		}
		first := refState(t, db, tenants)
		if strings.Join(first, "\n") != strings.Join(expected(m), "\n") {
			t.Fatalf("%s: wiped ref.students not rebuilt: %v", m, first)
		}
		if _, err := rebuild(m); err != nil {
			t.Fatal(err)
		}
		if again := refState(t, db, tenants); strings.Join(again, "\n") != strings.Join(first, "\n") {
			t.Fatalf("%s: rerun changed the projection\n got %v\nwant %v", m, again, first)
		}
		// A newer row written by the event consumer is never overwritten by the snapshot (source_version rule).
		exec(t, db, `UPDATE ref.schools SET status = 'disabled', source_version = 99 WHERE school_id = $1`, sa)
		if _, err := rebuild(m, "--table=ref.schools"); err != nil {
			t.Fatal(err)
		}
		var st string
		var v int64
		if err := db.QueryRow(`SELECT status, source_version FROM ref.schools WHERE school_id = $1`, sa).Scan(&st, &v); err != nil || st != "disabled" || v != 99 {
			t.Fatalf("%s: newer school overwritten: %s %d %v", m, st, v, err)
		}
		exec(t, db, `UPDATE ref.schools SET status = 'active', source_version = 2 WHERE school_id = $1`, sa)
	}

	// Tenant isolation: learning projects a student of tenant A under tenant B (a row of another tenant): the rebuild of
	// tenant A refuses it (exit ≠ 0) and never moves the row; tenant B keeps its own rows.
	learningDB := super("learning")
	moved := students[0]
	exec(t, learningDB, `UPDATE ref.students SET tenant_id = $2, source_version = 1 WHERE student_id = $1`, moved.id, tb)
	if _, err := rebuild("learning", "--table=ref.students"); err == nil {
		t.Fatal("a student projected in another tenant must fail the rebuild")
	}
	var owner uuid.UUID
	if err := learningDB.QueryRow(`SELECT tenant_id FROM ref.students WHERE student_id = $1`, moved.id).Scan(&owner); err != nil || owner != tb {
		t.Fatalf("the rebuild moved the row to another tenant: %s %v", owner, err)
	}
	var bRows int
	if err := learningDB.QueryRow(`SELECT count(*) FROM ref.students WHERE tenant_id = $1 AND student_id = $2`, tb, students[3].id).Scan(&bRows); err != nil || bRows != 1 {
		t.Fatalf("tenant B rows: %d %v", bRows, err)
	}
	// An unknown module or table has nothing registered (exit ≠ 0).
	if _, err := rebuild("identity"); err == nil {
		t.Fatal("module identity keeps no identity projection")
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
