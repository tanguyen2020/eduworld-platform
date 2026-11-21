package command

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SchoolCurriculum holds the curriculum versions a school uses per academic year and grade (F021 FR-CUR-008; moved from
// F040 by ADR-0095). The use case authorizes explicitly with the school (authz.Require, R7).
type SchoolCurriculum struct {
	UoW   write.UnitOfWork
	Store write.CurriculumSettingsStore
	Read  read.Session // platform reference checks (RESOURCE_NOT_ASSIGNED)
	Audit write.Auditor
	Clock func() time.Time
}

// CurriculumSettingInput is one item of SchoolCurriculumSettingsInput.
type CurriculumSettingInput struct {
	AcademicYearID      uuid.UUID `json:"academic_year_id"`
	GradeLevel          *int      `json:"grade_level"`
	CurriculumVersionID uuid.UUID `json:"curriculum_version_id"`
}

// visibleVersion returns a version visible to the tenant: invisible but existing platform content is 403
// RESOURCE_NOT_ASSIGNED (ADR-0029), unknown is 422 at ptr, not published 409 CONTENT_NOT_PUBLISHED.
func (h SchoolCurriculum) visibleVersion(ctx context.Context, find func(ctx context.Context) (*write.VersionAccess, error), ptr string) (*write.VersionAccess, error) {
	v, err := find(ctx)
	if err != nil {
		return nil, err
	}
	if v == nil {
		if h.Read != nil {
			ps, err := authz.SystemPlatformScope("learning-reference-check")
			if err != nil {
				return nil, err
			}
			var platform *write.VersionAccess
			if err := h.Read.Read(tenancy.WithScope(ctx, ps), func(ctx context.Context) error {
				var err error
				platform, err = find(ctx)
				return err
			}); err != nil {
				return nil, err
			}
			if platform != nil && platform.Platform {
				return nil, problem.New(problem.ResourceNotAssigned, "the platform content is not assigned to the tenant")
			}
		}
		return nil, problem.Invalid(ptr, problem.SubInvalid, "is not a version visible to the tenant")
	}
	if v.Status != "published" {
		return nil, problem.New(problem.ContentNotPublished, "the version is not published")
	}
	return v, nil
}

// PutCurriculumSettings sets the curriculum versions of a school per academic year and grade (F021-AC-30, formerly
// F040-AC-03): published versions of the tenant or of assigned platform curricula only, If-Match on the settings of the
// school, audit.
func (h SchoolCurriculum) PutCurriculumSettings(ctx context.Context, school uuid.UUID, items []CurriculumSettingInput, ifMatch int64) (out dto.CurriculumSettingsView,
	err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SetSchoolCurriculum")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := AuthorizeLMS(ctx, p, authz.PermSchoolSettingsWrite, school, nil); err != nil {
			return err
		}
		_, version, err := h.Store.CurriculumSettings(ctx, school)
		if err != nil {
			return err
		}
		if version != ifMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		seen := map[string]bool{}
		var years []uuid.UUID
		rows := make([]write.CurriculumSettingRow, 0, len(items))
		for i, it := range items {
			key := it.AcademicYearID.String() + "|"
			if it.GradeLevel != nil {
				key += itoa(*it.GradeLevel)
			}
			if seen[key] {
				return problem.Invalid("/items/"+itoa(i), problem.SubInvalid, "one version per academic year and grade")
			}
			seen[key] = true
			vid := it.CurriculumVersionID
			if _, err := h.visibleVersion(ctx, func(ctx context.Context) (*write.VersionAccess, error) {
				return h.Store.CurriculumVersion(ctx, vid)
			}, "/items/"+itoa(i)+"/curriculum_version_id"); err != nil {
				return err
			}
			if !slices.Contains(years, it.AcademicYearID) {
				years = append(years, it.AcademicYearID)
			}
			rows = append(rows, write.CurriculumSettingRow{ID: ids.New(), TenantID: *p.TenantID, SchoolID: school, AcademicYearID: it.AcademicYearID,
				Grade: it.GradeLevel, CurriculumVersionID: vid, SetBy: p.UserID})
		}
		if err := h.Store.ReplaceCurriculumSettings(ctx, school, years, rows, p.UserID); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "school_curriculum.set", TargetType: "school", TargetID: school.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"items": len(rows)}, SchoolID: school.String()}); err != nil {
			return err
		}
		settings, version, err := h.Store.CurriculumSettings(ctx, school)
		if err != nil {
			return err
		}
		out = dto.CurriculumSettingsView{SchoolID: school, Items: []dto.CurriculumSettingView{}, Version: version}
		for _, s := range settings {
			out.Items = append(out.Items, dto.CurriculumSettingView{AcademicYearID: s.AcademicYearID, GradeLevel: s.Grade,
				CurriculumVersionID: s.CurriculumVersionID, SetBy: s.SetBy, UpdatedAt: s.UpdatedAt})
		}
		return nil
	})
	return out, err
}
