//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Staff of the LMS tests (quickstart F040 §1): School Admin of school A2 (another school of tenant A), Tenant Admin of
// tenant A, Academic Manager of another subject, System Admin.
const (
	adA2      = "ad.truong.a2"
	taA       = "ta.tenant.a"
	qlcmOther = "qlcm.mon.khac"
)

// lmsKit is the LMS of the tests: the school of adA with class 3A (hs.an.3a, hs.binh.3a), a published curriculum
// version with the subject of grade 3, the courses of the class.
type lmsKit struct {
	s       attSetup
	school  uuid.UUID
	class   uuid.UUID
	year    string
	courses map[string]string // subject id → course id
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
		VALUES ($1, $2, $3, $4, 1) ON CONFLICT (student_id) DO UPDATE SET school_id = EXCLUDED.school_id`, d.UserID, *d.TenantID, school, grade); err != nil {
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

// classSynced delivers class.synced of a class of tenant A to the consumer learning.lms_courses (in the transaction and
// event scope the outbox consumer opens).
func (c *curEnv) classSynced(ctx context.Context, t *testing.T, eventID, class, school uuid.UUID, grade int, year, status string) error {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"class_id": class, "school_id": school, "grade_level": grade, "name": "3A", "academic_year": year,
		"status": status, "op": "created", "trigger": "scheduled_sync"})
	tenant := c.tenantA
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return c.uow.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		return consumer.LMSCourses{Courses: c.lms}.Handle(ctx, outbox.Envelope{EventID: eventID, EventType: "class.synced", TenantID: &tenant,
			OccurredAt: time.Now(), Payload: payload})
	})
}

// lmsSetup builds the LMS kit.
func (c *curEnv) lmsSetup(ctx context.Context, t *testing.T) lmsKit {
	t.Helper()
	k := lmsKit{s: c.attemptSetup(ctx, t, nil), school: c.schoolOf(adA), class: uuid.New(), year: "2026-2027", courses: map[string]string{}}
	c.user(adA2, authztest.Staff(c.tenantA, "school_admin", authztest.AtSchool(uuid.New())).PrincipalData, true)
	c.user(taA, authztest.Staff(c.tenantA, "tenant_admin").PrincipalData, true)
	c.user(qlcmOther, authztest.Staff(c.tenantA, "academic_manager", authztest.ForSubject(uuid.New())).PrincipalData, true)
	c.subjectGrade(ctx, t, k.s.qc.cur, k.s.qc.version, k.s.qc.subject, 3)
	c.publishCurriculum(ctx, t, k.s.qc.version)
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.classes (class_id, tenant_id, school_id, grade_level, name, source_version)
		VALUES ($1, $2, $3, 3, '3A', 1)`, k.class, c.tenantA, k.school); err != nil {
		t.Fatal(err)
	}
	c.enrollIn(ctx, t, hsAn, k.school, k.class, 3)
	c.enrollIn(ctx, t, hsBinh, k.school, k.class, 3)
	if err := c.classSynced(ctx, t, ids.New(), k.class, k.school, 3, k.year, "active"); err != nil {
		t.Fatal(err)
	}
	rows, err := c.platform.QueryContext(ctx, `SELECT subject_id, course_id FROM lms.courses WHERE class_id = $1 AND deleted_at IS NULL`, k.class)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s, id string
		_ = rows.Scan(&s, &id)
		k.courses[s] = id
	}
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
