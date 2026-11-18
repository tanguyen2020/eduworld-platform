package write

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entenroll "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/enrollment"
	entuser "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/enrollment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// EnrollmentRepo implements write.EnrollmentRepository (org.enrollments).
type EnrollmentRepo struct{}

var _ write.EnrollmentRepository = EnrollmentRepo{}

// scopeTenants returns the tenants of the scope as text for explicit SQL (tenant_id = ANY($n::uuid[])).
func scopeTenants(ctx context.Context) ([]string, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return nil, entx.ErrMissingTenantScope
	}
	out := make([]string, 0, len(s.TenantIDs()))
	for _, t := range s.TenantIDs() {
		out = append(out, t.String())
	}
	return out, nil
}

// LockActive locks the active enrollment of the student (explicit SQL: ent has no FOR UPDATE here).
func (EnrollmentRepo) LockActive(ctx context.Context, studentID uuid.UUID) (*enrollment.Enrollment, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	tenants, err := scopeTenants(ctx)
	if err != nil {
		return nil, err
	}
	var e enrollment.Enrollment
	var end sql.NullTime
	var status string
	err = tx.QueryRowContext(ctx, `SELECT enrollment_id, tenant_id, school_id, student_id, class_id, status, start_date, end_date, source, version
		FROM org.enrollments
		WHERE student_id = $1 AND status = 'active' AND deleted_at IS NULL AND tenant_id = ANY($2::uuid[])
		FOR UPDATE`, studentID, tenants).
		Scan(&e.ID, &e.TenantID, &e.SchoolID, &e.StudentID, &e.ClassID, &status, &e.StartDate, &end, &e.Source, &e.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, problem.New(problem.NotFound, "no active enrollment")
	}
	if err != nil {
		return nil, err
	}
	e.Status = enrollment.Status(status)
	e.StartDate = e.StartDate.UTC()
	if end.Valid {
		t := end.Time.UTC()
		e.EndDate = &t
	}
	return &e, nil
}

// Close saves the closed enrollment.
func (EnrollmentRepo) Close(ctx context.Context, e *enrollment.Enrollment) error {
	u := client(ctx).Enrollment.Update().Where(entenroll.ID(e.ID), entenroll.Version(e.Version)).SetStatus(string(e.Status)).AddVersion(1)
	if e.EndDate != nil {
		u = u.SetEndDate(*e.EndDate)
	}
	n, err := u.Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.New(problem.Conflict, "enrollment was modified")
	}
	e.Version++
	return nil
}

// Open inserts the active enrollment (tenant explicit for cross-tenant scopes).
func (EnrollmentRepo) Open(ctx context.Context, e *enrollment.Enrollment) error {
	row, err := client(ctx).Enrollment.Create().SetID(e.ID).SetTenantID(e.TenantID).SetSchoolID(e.SchoolID).SetStudentID(e.StudentID).
		SetClassID(e.ClassID).SetStatus(string(e.Status)).SetStartDate(e.StartDate).SetSource(e.Source).Save(ctx)
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, "the student already has an active enrollment").Wrap(err)
	}
	if err != nil {
		return err
	}
	e.Version = row.Version
	return nil
}

// Placement returns the tenant and school of a live class of the scope.
func (EnrollmentRepo) Placement(ctx context.Context, classID uuid.UUID) (enrollment.Placement, error) {
	c, err := client(ctx).Class.Get(ctx, classID)
	if ent.IsNotFound(err) {
		return enrollment.Placement{}, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return enrollment.Placement{}, err
	}
	return enrollment.Placement{TenantID: c.TenantID, SchoolID: c.SchoolID, ClassID: c.ID}, nil
}

// StudentRepo implements write.StudentRepository.
type StudentRepo struct{}

var _ write.StudentRepository = StudentRepo{}

// Move rewrites the school and tenant of the student row (tenant_id is immutable in the ent mixin, so this
// is explicit SQL limited to the scope) and the user's primary tenant/school.
func (StudentRepo) Move(ctx context.Context, studentID uuid.UUID, to enrollment.Placement) error {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return entx.ErrNoTransaction
	}
	tenants, err := scopeTenants(ctx)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE org.students SET tenant_id = $1, school_id = $2, updated_at = $3, version = version + 1
		WHERE student_id = $4 AND deleted_at IS NULL AND tenant_id = ANY($5::uuid[])`,
		to.TenantID, to.SchoolID, time.Now().UTC(), studentID, tenants)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return problem.New(problem.NotFound, "student not found in scope")
	}
	// Bulk update: UpdateOne would open its own transaction instead of using the unit of work.
	n, err := client(ctx).User.Update().Where(entuser.ID(studentID)).SetPrimaryTenantID(to.TenantID).
		SetPrimarySchoolID(to.SchoolID).AddVersion(1).Save(ctx)
	if err != nil {
		return err
	}
	if n != 1 {
		return problem.New(problem.NotFound, "user not found")
	}
	return nil
}
