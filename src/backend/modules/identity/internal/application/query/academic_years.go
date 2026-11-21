package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// ListSchoolAcademicYears is listSchoolAcademicYears (F010 T047, F056-2): the live academic years of a school, newest
// first, for the "Năm học này" / "Học kỳ này" presets of the School Learning Dashboard. analytics.school.read or
// analytics.class.read on the school of the path (the two permissions of action.school_lms.school_dashboard.view); no
// permission → 403, a school outside the tenant or the grant scope → 404. terms is always empty: module identity stores
// no school terms yet (Clarifications F010 2026-11-21, OQ F010-4).
func (q Org) ListSchoolAcademicYears(ctx context.Context, schoolID uuid.UUID) (out []dto.SchoolAcademicYear, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListSchoolAcademicYears")
	defer func() { done(err) }()
	if ctx, _, err = schoolScoped(ctx, schoolID, authz.PermAnalyticsSchoolRead, authz.PermAnalyticsClassRead); err != nil {
		return nil, err
	}
	if _, err = q.Model.School(ctx, schoolID); err != nil { // another tenant or a soft-deleted school → 404
		return nil, err
	}
	out, err = q.Model.AcademicYears(ctx, schoolID)
	if out == nil && err == nil {
		out = []dto.SchoolAcademicYear{}
	}
	return out, err
}
