package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// SchoolCurriculum reads the curriculum versions a school uses (F021 FR-CUR-008, moved from F040 by ADR-0095). No
// write, no event (R26).
type SchoolCurriculum struct {
	Session read.Session
	Model   read.CurriculumSettingsReadModel
}

// CurriculumSettings is getSchoolCurriculumSettings (F021-AC-30): lms.class.read in the school.
func (q SchoolCurriculum) CurriculumSettings(ctx context.Context, school uuid.UUID, yearID *uuid.UUID) (out dto.CurriculumSettingsView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetSchoolCurriculum")
	defer func() { done(err) }()
	p, err := staffPrincipal(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if err := authorizeSchool(ctx, p, authz.PermLmsClassRead, school, nil); err != nil {
			return err
		}
		out, err = q.Model.CurriculumSettings(ctx, school, yearID)
		return err
	})
	return out, err
}
