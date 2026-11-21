package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entyear "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/academicyear"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
)

// maxAcademicYears bounds listSchoolAcademicYears (maxItems of the contract).
const maxAcademicYears = 100

// AcademicYears returns the live academic years of a school of the scope, newest first (ent global filters: tenant +
// deleted_at IS NULL). Terms are empty: no term is stored (F010 T047).
func (m OrgModel) AcademicYears(ctx context.Context, schoolID uuid.UUID) (out []dto.SchoolAcademicYear, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := cl(ctx).AcademicYear.Query().Where(entyear.SchoolID(schoolID)).
			Order(ent.Desc(entyear.FieldStartDate), ent.Asc(entyear.FieldID)).Limit(maxAcademicYears).All(ctx)
		for _, r := range rows {
			out = append(out, dto.SchoolAcademicYear{AcademicYearID: r.ID, Name: r.Name, StartDate: r.StartDate.Format(time.DateOnly),
				EndDate: r.EndDate.Format(time.DateOnly), IsCurrent: r.IsCurrent, Terms: []dto.AcademicTerm{}})
		}
		return err
	})
	return out, err
}
