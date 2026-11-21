package write

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	entuser "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

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

// StudentRepo implements write.StudentRepository (org.students: the school and grade of a student, ADR-0095 mục 2).
type StudentRepo struct{}

var _ write.StudentRepository = StudentRepo{}

// Placement locks the student row FOR UPDATE and returns its placement, soft-deleted rows included (they are not
// effective); nil when the student has no row in the scope. Explicit SQL: ent has no FOR UPDATE and hides deleted rows.
func (StudentRepo) Placement(ctx context.Context, studentID uuid.UUID) (*placement.Record, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	tenants, err := scopeTenants(ctx)
	if err != nil {
		return nil, err
	}
	var r placement.Record
	var year uuid.NullUUID
	// tenancyguard:ok explicit tenant condition; soft-deleted rows are read on purpose (status inactive) and locked
	err = tx.QueryRowContext(ctx, `SELECT s.tenant_id, s.school_id, s.grade_level, s.status = 'active' AND s.deleted_at IS NULL, s.version,
			(SELECT y.academic_year_id FROM org.academic_years y WHERE y.school_id = s.school_id AND y.tenant_id = s.tenant_id
				AND y.is_current AND y.deleted_at IS NULL ORDER BY y.start_date DESC LIMIT 1)
		FROM org.students s WHERE s.student_id = $1 AND s.tenant_id = ANY($2::uuid[]) FOR UPDATE OF s`, studentID, tenants).
		Scan(&r.TenantID, &r.SchoolID, &r.Grade, &r.Active, &r.Version, &year)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if year.Valid {
		y := year.UUID
		r.AcademicYearID = &y
	}
	return &r, nil
}

// Move rewrites the tenant, school and grade of a live student row (tenant_id is immutable in the ent mixin, so this is
// explicit SQL limited to the scope) and the user's primary tenant/school.
func (StudentRepo) Move(ctx context.Context, studentID uuid.UUID, to placement.Placement) error {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return entx.ErrNoTransaction
	}
	tenants, err := scopeTenants(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok explicit tenant and deleted_at conditions
	res, err := tx.ExecContext(ctx, `UPDATE org.students SET tenant_id = $1, school_id = $2, grade_level = $3, updated_at = $4,
			version = version + 1
		WHERE student_id = $5 AND deleted_at IS NULL AND tenant_id = ANY($6::uuid[])`,
		to.TenantID, to.SchoolID, to.Grade, time.Now().UTC(), studentID, tenants)
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

// SetStatus sets the effective status of a live student row (active/inactive); unchanged rows are not written.
func (StudentRepo) SetStatus(ctx context.Context, studentID uuid.UUID, status string) error {
	if status != placement.StatusActive && status != placement.StatusInactive {
		return placement.ErrInvalid
	}
	tx, ok := entx.Tx(ctx)
	if !ok {
		return entx.ErrNoTransaction
	}
	tenants, err := scopeTenants(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok explicit tenant and deleted_at conditions
	_, err = tx.ExecContext(ctx, `UPDATE org.students SET status = $1, updated_at = $2, version = version + 1
		WHERE student_id = $3 AND deleted_at IS NULL AND status <> $1 AND tenant_id = ANY($4::uuid[])`,
		status, time.Now().UTC(), studentID, tenants)
	return err
}
