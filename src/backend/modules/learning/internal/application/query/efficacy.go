package query

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Efficacy holds the reads of the efficacy study (F035 US2, US3, US6): platform permission first (R7), cross-tenant
// lists under the CrossTenantScope of tenant_ids (audited cross_tenant.accessed by the session), the classes and waves
// of a study under the audited bypass efficacy_study; never a score, a name of a student nor an arm to a student.
type Efficacy struct {
	Session read.Session
	Model   read.EfficacyReadModel
	Cursors *page.Codec
}

func efficacyReader(ctx context.Context, perm authz.Permission) error {
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		return problem.New(problem.PermissionDenied, "")
	}
	return nil
}

// crossTenants returns the tenants of the CrossTenantScope of tenant_ids (nil without one).
func crossTenants(ctx context.Context) []uuid.UUID {
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if cs, ok := s.(tenancy.CrossTenantScope); ok {
			return cs.TenantIDs()
		}
	}
	return nil
}

// AgreementView is the view of an agreement row.
func AgreementView(a read.AgreementRow) dto.ResearchAgreementView {
	return dto.ResearchAgreementView{AgreementID: a.ID, TenantID: a.TenantID, AgreementRef: a.Ref, Covers: a.Covers,
		AgreedOn: a.AgreedOn.Format("2006-01-02"), Status: a.Status, RecordedBy: a.RecordedBy, RevokedAt: a.RevokedAt, RevokedBy: a.RevokedBy,
		RevokeReason: a.RevokeReason, CreatedAt: a.CreatedAt}
}

// ListResearchAgreements is listResearchAgreements (F035-AC-11): tenant_ids required (CrossTenantScope, audited).
func (q Efficacy) ListResearchAgreements(ctx context.Context, status string, in PageInput) (out Page[dto.ResearchAgreementView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListResearchAgreements")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	tenants := crossTenants(ctx)
	if len(tenants) == 0 {
		return out, problem.Invalid("tenant_ids", problem.SubRequired, "lists the tenants of the agreements")
	}
	var rows []read.AgreementRow
	if err := q.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = q.Model.Agreements(ctx, tenants, status)
		return err
	}); err != nil {
		return out, err
	}
	views := make([]dto.ResearchAgreementView, len(rows))
	for i, a := range rows {
		views[i] = AgreementView(a)
	}
	return paginate(views, func(v dto.ResearchAgreementView) uuid.UUID { return v.AgreementID }, in, q.Cursors)
}

// StudyView is the view of a study with its classes.
func StudyView(s read.StudyRow, classes []read.StudyClassRead) dto.EfficacyStudyView {
	v := dto.EfficacyStudyView{StudyID: s.ID, Code: s.Code, Title: s.Title, SubjectID: s.SubjectID, GradeLevel: s.GradeLevel,
		ObjectiveIDs: s.ObjectiveIDs, FormA: s.FormA, FormB: s.FormB, MinWeeks: s.MinWeeks, Status: s.Status, CreatedBy: s.CreatedBy,
		CreatedAt: s.CreatedAt, ActivatedAt: s.ActivatedAt, CompletedAt: s.CompletedAt, Version: s.Version, TenantIDs: []uuid.UUID{}}
	if v.ObjectiveIDs == nil {
		v.ObjectiveIDs = []uuid.UUID{}
	}
	pairs := map[int]bool{}
	for _, c := range classes {
		v.ClassCount++
		if c.PairNo != nil {
			pairs[*c.PairNo] = true
		}
		if !slices.Contains(v.TenantIDs, c.TenantID) {
			v.TenantIDs = append(v.TenantIDs, c.TenantID)
		}
	}
	v.PairCount = len(pairs)
	return v
}

// bypass is the audited bypass efficacy_study of the reads of one study (every tenant of its classes).
func bypass(ctx context.Context) (context.Context, error) {
	return tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonEfficacyStudy)
}

// ListEfficacyStudies is listEfficacyStudies (F035-AC-11): every study, or with tenant_ids the studies holding a class of
// those tenants (CrossTenantScope, audited); class counts per study.
func (q Efficacy) ListEfficacyStudies(ctx context.Context, status string, in PageInput) (out Page[dto.EfficacyStudyView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListEfficacyStudies")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	tenants := crossTenants(ctx)
	rctx := ctx
	if tenants == nil {
		if rctx, err = bypass(ctx); err != nil {
			return out, err
		}
	}
	var views []dto.EfficacyStudyView
	err = q.Session.Read(rctx, func(ctx context.Context) error {
		rows, err := q.Model.Studies(ctx, status, tenants)
		if err != nil {
			return err
		}
		for _, s := range rows {
			classes, err := q.Model.StudyClasses(ctx, s.ID, tenants)
			if err != nil {
				return err
			}
			views = append(views, StudyView(s, classes))
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	return paginate(views, func(v dto.EfficacyStudyView) uuid.UUID { return v.StudyID }, in, q.Cursors)
}

// GetEfficacyStudy is getEfficacyStudy.
func (q Efficacy) GetEfficacyStudy(ctx context.Context, id uuid.UUID) (out dto.EfficacyStudyView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetEfficacyStudy")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	bctx, err := bypass(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(bctx, func(ctx context.Context) error {
		s, err := q.Model.Study(ctx, id)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		classes, err := q.Model.StudyClasses(ctx, id, nil)
		if err != nil {
			return err
		}
		out = StudyView(*s, classes)
		return nil
	})
	return out, err
}

// ListEfficacyStudyClasses is listEfficacyStudyClasses (F035-AC-11): the classes of the tenants of tenant_ids
// (CrossTenantScope) or of every tenant of the study (bypass efficacy_study); each row carries its tenant_id.
func (q Efficacy) ListEfficacyStudyClasses(ctx context.Context, studyID uuid.UUID, in PageInput) (out Page[dto.EfficacyStudyClassView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListEfficacyStudyClasses")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	tenants := crossTenants(ctx)
	rctx := ctx
	if tenants == nil {
		if rctx, err = bypass(ctx); err != nil {
			return out, err
		}
	}
	var views []dto.EfficacyStudyClassView
	err = q.Session.Read(rctx, func(ctx context.Context) error {
		s, err := q.Model.Study(ctx, studyID)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		rows, err := q.Model.StudyClasses(ctx, studyID, tenants)
		if err != nil {
			return err
		}
		for _, c := range rows {
			views = append(views, dto.EfficacyStudyClassView{StudyID: c.StudyID, ClassID: c.ClassID, TenantID: c.TenantID, SchoolID: c.SchoolID,
				ClassName: c.ClassName, GradeLevel: c.GradeLevel, PairNo: c.PairNo, Arm: c.Arm, AssignmentMethod: c.Method, FormOrder: c.FormOrder,
				Status: c.Status, ParticipantCount: c.Participants, WithdrawnAt: c.WithdrawnAt})
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	return paginate(views, func(v dto.EfficacyStudyClassView) uuid.UUID { return v.ClassID }, in, q.Cursors)
}

// ListEfficacyWaves is listEfficacyWaves: the waves of a study with the invited, completed and absent counts of each class
// (never a score).
func (q Efficacy) ListEfficacyWaves(ctx context.Context, studyID uuid.UUID) (out []dto.EfficacyWaveView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListEfficacyWaves")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	bctx, err := bypass(ctx)
	if err != nil {
		return out, err
	}
	out = []dto.EfficacyWaveView{}
	err = q.Session.Read(bctx, func(ctx context.Context) error {
		s, err := q.Model.Study(ctx, studyID)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		ws, err := q.Model.Waves(ctx, studyID)
		if err != nil {
			return err
		}
		for _, w := range ws {
			v := dto.EfficacyWaveView{WaveID: w.ID, StudyID: w.StudyID, Kind: w.Kind, Form: w.Form, WindowStart: w.WindowStart, WindowEnd: w.WindowEnd,
				DurationSeconds: w.DurationSec, Status: w.Status, OpenedAt: w.OpenedAt, ClosedAt: w.ClosedAt, Classes: []dto.EfficacyWaveClassView{}}
			sessions, err := q.Model.WaveSessions(ctx, w.ID)
			if err != nil {
				return err
			}
			for _, x := range sessions {
				v.Classes = append(v.Classes, dto.EfficacyWaveClassView{ClassID: x.ClassID, TenantID: x.TenantID, ExamSessionID: x.ExamSessionID,
					Form: x.Form, SessionStatus: x.Status, InvitedCount: x.Invited, CompletedCount: x.Completed, AbsentCount: x.Absent})
			}
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

// EfficacyReportView is the view of a report row.
func EfficacyReportView(r read.EfficacyReportRow) dto.EfficacyReportView {
	v := dto.EfficacyReportView{ReportID: r.ID, StudyID: r.StudyID, Status: r.Status, DescriptiveOnly: r.DescriptiveOnly, Excluded: r.Excluded,
		Results: r.Results, Error: r.Error, RequestedBy: r.RequestedBy, CreatedAt: r.CreatedAt, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt}
	if v.Excluded == nil {
		v.Excluded = []uuid.UUID{}
	}
	return v
}

// GetEfficacyStudyReport is getEfficacyStudyReport: the state and the aggregated results of a report (small cells hidden).
func (q Efficacy) GetEfficacyStudyReport(ctx context.Context, studyID, reportID uuid.UUID) (out dto.EfficacyReportView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetEfficacyStudyReport")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		r, err := q.Model.Report(ctx, studyID, reportID)
		if err != nil {
			return err
		}
		if r == nil {
			return problem.New(problem.NotFound, "")
		}
		out = EfficacyReportView(*r)
		return nil
	})
	return out, err
}
