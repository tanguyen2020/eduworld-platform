package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/regulation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/reporting"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Reports are the reads of F044 US1, US3–US6 (US2 course results removed with the courses, ADR-0095): the export jobs
// (getLmsReport, getTenantReport, getPlatformReport), the reference report (getRegulationReferenceReport), the question statistics, the
// calibration queue and the exam session report.
type Reports struct {
	Session    read.Session
	Jobs       read.ReportJobReader
	Files      port.ReportFiles
	References read.ReferenceReader
	Quality    read.QualityReader
	Exams      read.ExamQualityReader
	Config     reporting.Config
	Cursors    *page.Codec
	Clock      func() time.Time
}

func (q Reports) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

func (q Reports) linkTTL() time.Duration {
	if q.Config.LinkTTL > 0 {
		return q.Config.LinkTTL
	}
	return reporting.DefaultConfig().LinkTTL
}

// GetReport returns a job of its creator (F044-AC-02, AC-05): report.export required (403); another user, another tenant,
// another route → 404; a succeeded job carries a link presigned for dashboard_export_link_ttl_minutes.
func (q Reports) GetReport(ctx context.Context, scope reporting.Scope, id uuid.UUID) (out dto.ReportJobView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetReport")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	// Any grant holding report.export (school-limited grants included): the job itself is the scope (creator only).
	if !holds(p, authz.PermReportExport) {
		return out, authz.Require(ctx, p, authz.PermReportExport, authz.Resource{TenantID: p.TenantID})
	}
	rctx := ctx
	if scope == reporting.ScopePlatform {
		if p.TenantID != nil || !p.IsSystemAdmin {
			return out, problem.New(problem.PermissionDenied, "")
		}
		if rctx, err = tenancy.Bypass(tenancy.WithModule(ctx, "learning"), tenancy.ReasonSystemAdminCrossTenant); err != nil {
			return out, err
		}
	} else if p.TenantID == nil || p.UserType == authz.UserTypeStudent {
		return out, problem.New(problem.PermissionDenied, "")
	}
	var key *string
	var expires *time.Time
	found := false
	err = q.Session.Read(rctx, func(ctx context.Context) error {
		j, ok, err := q.Jobs.ReportJob(ctx, id)
		if err != nil || !ok {
			return err
		}
		sameTenant := (j.TenantID == nil && p.TenantID == nil) || (j.TenantID != nil && p.TenantID != nil && *j.TenantID == *p.TenantID)
		found = j.RequestedBy == p.UserID && j.Scope == string(scope) && sameTenant
		out = dto.ReportJobView{ReportID: j.ID, Status: j.Status, ReportType: j.Type, Format: j.Format, RowCount: j.RowCount, Error: j.Error,
			CreatedAt: j.CreatedAt, FinishedAt: j.FinishedAt}
		key, expires = j.StorageKey, j.FileExpiresAt
		return nil
	})
	if err != nil {
		return out, err
	}
	if !found {
		return dto.ReportJobView{}, problem.New(problem.NotFound, "")
	}
	at := q.now()
	if reporting.Downloadable(out.Status, expires, at) && key != nil && q.Files != nil {
		url, exp, err := q.Files.ReportURL(ctx, *key, at, q.linkTTL())
		if err != nil {
			return out, err
		}
		out.DownloadURL, out.DownloadExpiresAt = &url, &exp
	}
	return out, nil
}

// lmsScope applies a school/subject permission of the LMS (F044-AC-10, AC-15, AC-25): no permission → 403; school outside
// every grant (or another tenant) → 404; subject outside a subject-limited grant → 403 PERMISSION_DENIED.
func lmsScope(ctx context.Context, p authz.Principal, perm authz.Permission, school, subject uuid.UUID) error {
	if p.TenantID == nil || p.UserType == authz.UserTypeStudent {
		return problem.New(problem.PermissionDenied, "")
	}
	if !holds(p, perm) {
		return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID})
	}
	if !coversSchool(p, perm, school) {
		_ = authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
		return problem.New(problem.NotFound, "")
	}
	return authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: &subject})
}

// ReferenceInput is the request of getRegulationReferenceReport.
type ReferenceInput struct {
	SchoolID, SubjectID uuid.UUID
	GradeLevel          *int
	Period              string
	AcademicYear        int
	Page                PageInput
}

// ReferencePage is the reference report and its page.
type ReferencePage struct {
	dto.RegulationReferenceReportView
	NextCursor *string
	HasMore    bool
	Limit      int
}

// GetReferenceReport is getRegulationReferenceReport (F044-AC-12, AC-15): analytics.school.read in the school and subject
// (subject outside the scope of an academic_manager → 403); the disclaimer on every response; rapid-guess ratio and the
// official exams with a low-reliability warning per student. Staff only: students never see a circular level (AC-13).
func (q Reports) GetReferenceReport(ctx context.Context, in ReferenceInput) (out ReferencePage, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetRegulationReferenceReport")
	defer func() { done(err) }()
	p, err := staff(ctx)
	if err != nil {
		return out, err
	}
	if !regulation.ValidPeriod(in.Period) {
		return out, problem.Invalid("period", problem.SubInvalid, "must be mid_term_1, end_term_1, mid_term_2 or end_term_2")
	}
	if _, err := schoolScope(ctx, p, in.SchoolID, &in.SubjectID, false); err != nil {
		return out, err
	}
	year := in.AcademicYear
	if year == 0 {
		_, year = regulation.CurrentPeriod(q.now())
	}
	limit := page.Limit(in.Page.Limit)
	after := uuid.Nil
	if cur, err := q.Cursors.Decode(in.Page.Cursor); err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	} else if cur != nil {
		after = cur.ID
	}
	out.Disclaimer = regulation.DefaultDisclaimer()["vi"]
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		ls, err := q.References.ReferenceLevels(ctx, in.SchoolID, in.SubjectID, year, in.Period, in.GradeLevel, after, limit)
		if err != nil {
			return err
		}
		if len(ls) > limit {
			ls, out.HasMore = ls[:limit], true
			s := q.Cursors.Encode(page.Cursor{ID: ls[limit-1].StudentID})
			out.NextCursor = &s
		}
		out.Items = make([]dto.ReferenceItem, 0, len(ls))
		for _, l := range ls {
			low := l.LowReliabilityExamIDs
			if low == nil {
				low = []uuid.UUID{}
			}
			out.Items = append(out.Items, dto.ReferenceItem{StudentID: l.StudentID, GradeLevel: l.GradeLevel, LevelCode: l.LevelCode,
				CoverageRatio: l.Coverage, ProficientRatio: l.Proficient, MasteredRatio: l.Mastered, OfficialScore: l.OfficialScore,
				RapidGuessRatio: l.RapidGuessRatio, LowReliabilityExamIDs: low, ProfileVersion: l.ProfileVersion})
		}
		if len(ls) > 0 {
			if d, err := q.References.ProfileDisclaimer(ctx, ls[0].ProfileCode, ls[0].ProfileVersion); err == nil && d["vi"] != "" {
				out.Disclaimer = d["vi"]
			}
		}
		return nil
	})
	out.SchoolID, out.SubjectID, out.Period, out.AcademicYear, out.Limit = in.SchoolID, in.SubjectID, in.Period, year, limit
	return out, err
}
