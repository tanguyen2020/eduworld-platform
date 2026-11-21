//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Staff of the LMS tests: School Admin of school A2 (another school of tenant A), Tenant Admin of
// tenant A, Academic Manager of another subject, System Admin.
const (
	adA2      = "ad.truong.a2"
	taA       = "ta.tenant.a"
	qlcmOther = "qlcm.mon.khac"
)

// lmsKit is the school of the LMS tests: the school of adA with grade 3 (hs.an.3a, hs.binh.3a in ref.students) and a
// published curriculum version with the subject of grade 3 (ADR-0095: no course). The class 3A of the projection
// ref.classes/ref.enrollments stays for the efficacy study (F035, cụm trường × khối in WP-L2).
type lmsKit struct {
	s      attSetup
	school uuid.UUID
	class  uuid.UUID
}

func (c *curEnv) schoolOf(sub string) uuid.UUID {
	for _, g := range c.users[sub].Grants {
		if g.SchoolID != nil {
			return *g.SchoolID
		}
	}
	return uuid.Nil
}

// enrollIn puts a student into ref.students of the school and ref.enrollments of the class (projection of F020).
func (c *curEnv) enrollIn(ctx context.Context, t *testing.T, sub string, school, class uuid.UUID, grade int) {
	t.Helper()
	d := c.users[sub]
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version)
		VALUES ($1, $2, $3, $4, 1) ON CONFLICT (student_id) DO UPDATE SET school_id = EXCLUDED.school_id,
		grade_level = EXCLUDED.grade_level`, d.UserID, *d.TenantID, school, grade); err != nil {
		t.Fatal(err)
	}
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.enrollments (student_id, class_id, start_date, tenant_id, status, source_version)
		VALUES ($1, $2, current_date, $3, 'active', 1)`, d.UserID, class, *d.TenantID); err != nil {
		t.Fatal(err)
	}
}

// subjectGrade adds a subject × grade to a curriculum version (TEST helper: authoring is F021).
func (c *curEnv) subjectGrade(ctx context.Context, t *testing.T, cur, version, subject string, grade int) {
	t.Helper()
	// tenancyguard:ok TEST helper under learning_platform
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO curriculum.subject_grades (subject_grade_id, tenant_id, created_at, updated_at,
		owner_scope, curriculum_id, curriculum_version_id, subject_id, grade_level) VALUES ($1, NULL, now(), now(), 'platform', $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`, ids.New(), cur, version, subject, grade); err != nil {
		t.Fatal(err)
	}
}

// lmsSetup builds the LMS kit.
func (c *curEnv) lmsSetup(ctx context.Context, t *testing.T) lmsKit {
	t.Helper()
	k := lmsKit{s: c.attemptSetup(ctx, t, nil), school: c.schoolOf(adA), class: uuid.New()}
	c.user(adA2, authztest.Staff(c.tenantA, "school_admin", authztest.AtSchool(uuid.New())).PrincipalData, true)
	c.user(taA, authztest.Staff(c.tenantA, "tenant_admin").PrincipalData, true)
	c.user(qlcmOther, authztest.Staff(c.tenantA, "academic_manager", authztest.ForSubject(uuid.New())).PrincipalData, true)
	c.subjectGrade(ctx, t, k.s.qc.cur, k.s.qc.version, k.s.qc.subject, 3)
	c.publishCurriculum(ctx, t, k.s.qc.version)
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.schools (school_id, tenant_id, status, source_version) VALUES ($1, $2, 'active', 1)
		ON CONFLICT (school_id) DO NOTHING`, k.school, c.tenantA); err != nil {
		t.Fatal(err)
	}
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.classes (class_id, tenant_id, school_id, grade_level, name, source_version)
		VALUES ($1, $2, $3, 3, '3A', 1)`, k.class, c.tenantA, k.school); err != nil {
		t.Fatal(err)
	}
	c.enrollIn(ctx, t, hsAn, k.school, k.class, 3)
	c.enrollIn(ctx, t, hsBinh, k.school, k.class, 3)
	return k
}

// activityVersion returns the current version of an activity.
func (c *curEnv) activityVersion(ctx context.Context, t *testing.T, id string) string {
	t.Helper()
	var v string
	if err := c.platform.QueryRowContext(ctx, `SELECT current_version_id FROM content.activities WHERE activity_id = $1`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// scopeOf is the TenantScope of a tenant (event scope of the outbox consumers).
func scopeOf(t *testing.T, tenant uuid.UUID) tenancy.Scope {
	t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}
