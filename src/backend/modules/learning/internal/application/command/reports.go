package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/regulation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/reporting"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions and events of the exports (F044 US1, US3).
const (
	ActionReportExported            = "report.exported"
	ActionReferenceReportExported   = "learning.reference_report_exported"
	EventReportCompleted            = "report.completed"
	exportStatementTimeout          = 30 * time.Second // per batch on the replica [Proposed]
	reportJobsPerRun                = 2
	reportErrorGeneric              = "export_failed"
	reportErrorTooManyRows          = "too_many_rows"
	reportErrorStorage              = "storage_unavailable"
	reportErrorReplica              = "replica_unavailable"
	reportErrorQueryTimeout         = "query_timeout"
	reportErrorReportTypeNotAllowed = "report_type_not_available"
	reportErrorPseudonymKey         = "pseudonym_key_missing"
)

// ReportMetrics observes the exports (eduworld_export_jobs_total{status}, eduworld_export_jobs_duration_seconds).
type ReportMetrics interface {
	Job(status string, seconds float64)
}

// Reports holds the asynchronous exports (F044 US1, DASHBOARD_DATA §10): createLmsReport, createTenantReport,
// createPlatformReport (job + Idempotency-Key + limits) and the worker (job report-jobs) that reads the replica in batches,
// writes the file with its watermark to ew-exports, audits report.exported and publishes report.completed.
type Reports struct {
	UoW         write.UnitOfWork
	Jobs        write.ReportJobStore
	Queue       write.ReportJobQueue
	Read        read.Session // primary read session: scope lookups of the requests
	Scope       read.ReportScopeReader
	DataSession read.Session // dashboard session (replica, learning_dashboard); nil = exports unavailable (503)
	Data        read.ReportReader
	References  read.ReferenceReader
	Files       port.ReportFiles
	Writers     port.ReportWriters
	Audit       write.Auditor
	Events      write.EventPublisher
	Config      reporting.Config
	Clock       func() time.Time
	Metrics     ReportMetrics
	// Efficacy produces the pseudonymous dataset of a study (report_type efficacy_dataset, F035 US7); nil = refused (503).
	Efficacy EfficacyDataset
}

// EfficacyDataset is the producer of the efficacy dataset (command.Efficacy).
type EfficacyDataset interface {
	Dataset(ctx context.Context, studyID uuid.UUID) ([]string, [][]string, error)
}

func (h Reports) now() time.Time { return now(h.Clock).UTC() }

func (h Reports) config() reporting.Config {
	if h.Config.MaxRows <= 0 {
		return reporting.DefaultConfig()
	}
	return h.Config
}

// ReportInput is the body of createLmsReport / createTenantReport / createPlatformReport.
type ReportInput struct {
	Scope          reporting.Scope
	Request        reporting.Request
	IdempotencyKey string
}

// ReportView is the ReportJob view of a row (download link added by the query).
func ReportView(j write.ReportJobRow) dto.ReportJobView {
	return dto.ReportJobView{ReportID: j.ID, Status: j.Status, ReportType: j.Type, Format: j.Format, RowCount: j.RowCount, Error: j.Error,
		CreatedAt: j.CreatedAt, FinishedAt: j.FinishedAt}
}

// CreateReport creates an export job (F044-AC-01, AC-04, AC-05, AC-06, AC-07): report.export in the scope of the route; the
// scope of the requester (schools, subjects) is stored in the job; at most dashboard_export_max_rows rows (422) and
// dashboard_export_max_concurrent_per_user running jobs (429 EXPORT_LIMIT_REACHED with Retry-After); 202 with the job.
func (h Reports) CreateReport(ctx context.Context, in ReportInput) (out dto.ReportJobView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateReport")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	cfg := h.config()
	in.Request.Scope = in.Scope
	params, err := h.authorizeReport(ctx, p, in.Request)
	if err != nil {
		return out, err
	}
	if err := reportViolation(in.Request.Validate(cfg)); err != nil {
		return out, err
	}
	var tenant *uuid.UUID
	if in.Scope != reporting.ScopePlatform {
		t := *p.TenantID
		tenant = &t
	}
	spec := read.ReportSpec{TenantID: tenant, Type: in.Request.Type, Params: params, MinCellSize: cfg.MinCellSize}
	wctx := ctx
	if in.Scope == reporting.ScopePlatform {
		// learning_platform for the job without tenant; audited cross_tenant.accessed with the requested tenants (F044-AC-06).
		if wctx, err = tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonSystemAdminCrossTenant,
			tenancy.WithTenantIDs(params.TenantIDs...)); err != nil {
			return out, err
		}
	}
	var n int64
	if in.Request.Type == reporting.TypeEfficacyDataset {
		// F035: one row per participant of a study (bounded by the study); produced by the efficacy study, not the replica rollups.
		if h.Efficacy == nil {
			return out, problem.New(problem.Unavailable, "the efficacy dataset is not configured").Retry()
		}
	} else if n, err = h.count(wctx, spec); err != nil {
		return out, err
	}
	if n > cfg.MaxRows {
		return out, problem.Invalid("/scope", problem.SubInvalid,
			fmt.Sprintf("the export has %d rows, more than %d: narrow the range or the scope", n, cfg.MaxRows))
	}
	err = h.UoW.Do(wctx, func(ctx context.Context) error {
		if j, ok, err := h.Jobs.ReportJobByKey(ctx, p.UserID, in.IdempotencyKey); err != nil || ok {
			out = ReportView(j)
			return err
		}
		active, err := h.Jobs.ActiveReportJobs(ctx, p.UserID)
		if err != nil {
			return err
		}
		if active >= cfg.MaxConcurrentPerUser {
			return &RetryAfter{After: 30 * time.Second, Err: problem.New(problem.ExportLimitReached,
				fmt.Sprintf("at most %d exports may run at the same time", cfg.MaxConcurrentPerUser))}
		}
		j := write.ReportJobRow{ID: ids.New(), TenantID: tenant, Scope: string(in.Scope), Type: in.Request.Type, Format: in.Request.Format,
			Params: params, RequestedBy: p.UserID, Status: reporting.StatusQueued, IdempotencyKey: in.IdempotencyKey, CreatedAt: h.now()}
		if err := h.Jobs.InsertReportJob(ctx, j); err != nil {
			return err
		}
		out = ReportView(j)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func (h Reports) count(ctx context.Context, spec read.ReportSpec) (int64, error) {
	if h.DataSession == nil || h.Data == nil {
		return 0, problem.New(problem.Unavailable, "exports are not configured (dashboard database)").Retry()
	}
	var n int64
	err := h.DataSession.Read(ctx, func(ctx context.Context) error {
		if err := h.Data.Prepare(ctx, exportStatementTimeout); err != nil {
			return err
		}
		var err error
		n, err = h.Data.CountReportRows(ctx, spec)
		return err
	})
	var pe *problem.Error
	if err != nil && !errors.As(err, &pe) {
		slog.ErrorContext(ctx, "export count unavailable", "error", err)
		return 0, problem.New(problem.Unavailable, "dashboard database unavailable").Retry().Wrap(err)
	}
	return n, err
}

func reportViolation(err error) error {
	var fe *reporting.FieldError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, reporting.ErrRangeTooLarge):
		return problem.New(problem.DashboardRangeTooLarge, "the range exceeds dashboard_max_range_days")
	case errors.As(err, &fe) && fe.Required:
		return problem.Invalid("/"+fe.Field, problem.SubRequired, fe.Reason)
	case errors.As(err, &fe):
		return problem.Invalid("/"+fe.Field, problem.SubInvalid, fe.Reason)
	}
	return err
}

// grantSchools returns the schools of the grants holding perm (nil when one grant is tenant-wide).
func grantSchools(p authz.Principal, perm authz.Permission) []uuid.UUID {
	out := []uuid.UUID{}
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, perm) {
			continue
		}
		if g.SchoolID == nil {
			return nil
		}
		out = append(out, *g.SchoolID)
	}
	return out
}

// grantSubjectsOf returns the subjects of the grants holding perm that cover school (nil when one is not subject-limited).
func grantSubjectsOf(p authz.Principal, perm authz.Permission, school uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, perm) || (g.SchoolID != nil && *g.SchoolID != school) {
			continue
		}
		if g.SubjectID == nil {
			return nil
		}
		out = append(out, *g.SubjectID)
	}
	return out
}

// authorizeReport applies report.export to the request and returns the parameters with the scope of the requester
// (F044-AC-03, AC-05, AC-06, AC-10): platform → System Admin without tenant only; tenant → report.export of the tenant
// (school-limited grants read their schools only); LMS → school and subject of the object (school outside the grants →
// 404, subject outside the grants of an academic_manager → 403).
func (h Reports) authorizeReport(ctx context.Context, p authz.Principal, r reporting.Request) (write.ReportParams, error) {
	perm := authz.PermReportExport
	params := write.ReportParams{SchoolID: r.SchoolID, ClassID: r.ClassID, CourseID: r.CourseID, AssignmentID: r.AssignmentID,
		SubjectID: r.SubjectID, Period: r.Period, AcademicYear: r.AcademicYear, SchoolIDs: r.SchoolIDs, TenantIDs: r.TenantIDs,
		GradeLevel: r.GradeLevel, StudyID: r.StudyID}
	if r.From != nil {
		params.From = r.From.Format(time.DateOnly)
	}
	if r.To != nil {
		params.To = r.To.Format(time.DateOnly)
	}
	switch r.Scope {
	case reporting.ScopePlatform:
		if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
			return params, err
		}
		if p.TenantID != nil || !p.IsSystemAdmin {
			return params, problem.New(problem.PermissionDenied, "")
		}
		if r.Type == reporting.TypeEfficacyDataset {
			// F035-AC-32, AC-34: report.export and platform.efficacy_study.export; the tenants are those of the study.
			return params, authz.Require(ctx, p, authz.PermPlatformEfficacyStudyExport, authz.Resource{})
		}
		if len(r.TenantIDs) == 0 {
			return params, problem.Invalid("/tenant_ids", problem.SubRequired, "is required for a cross-tenant export")
		}
		return params, nil
	case reporting.ScopeTenant:
		if p.TenantID == nil || p.UserType == authz.UserTypeStudent {
			return params, problem.New(problem.PermissionDenied, "")
		}
		if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID}); err != nil {
			return params, err
		}
		if len(r.TenantIDs) > 0 {
			return params, problem.Invalid("/tenant_ids", problem.SubInvalid, "is only accepted by /admin/reports")
		}
		params.ScopeSchoolIDs = grantSchools(p, perm)
		return params, nil
	}
	if p.TenantID == nil || p.UserType == authz.UserTypeStudent {
		return params, problem.New(problem.PermissionDenied, "")
	}
	if !holdsPerm(p, perm) {
		return params, authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID})
	}
	school, subject, err := h.reportObject(ctx, r)
	if err != nil {
		return params, err
	}
	if school == uuid.Nil {
		return params, nil // missing parameter: refused by Validate
	}
	if err := AuthorizeLMS(ctx, p, perm, school, subject); err != nil {
		return params, err
	}
	params.SchoolID = &school
	params.ScopeSchoolIDs = []uuid.UUID{school}
	if subject == nil {
		params.SubjectIDs = grantSubjectsOf(p, perm, school)
	}
	if r.Type == reporting.TypeRegulationReference && params.AcademicYear == 0 {
		_, params.AcademicYear = regulation.CurrentPeriod(h.now())
	}
	return params, nil
}

// reportObject resolves the school and subject of an LMS export (unknown object of the tenant → 404).
func (h Reports) reportObject(ctx context.Context, r reporting.Request) (uuid.UUID, *uuid.UUID, error) {
	var school uuid.UUID
	var subject *uuid.UUID
	err := h.Read.Read(ctx, func(ctx context.Context) error {
		switch {
		case r.Type == reporting.TypeAssignmentResults && r.AssignmentID != nil:
			sc, sub, ok, err := h.Scope.AssignmentScope(ctx, *r.AssignmentID)
			if err != nil || !ok {
				return notFoundOr(err)
			}
			school, subject = sc, &sub
		case r.Type == reporting.TypeCourseResults && r.CourseID != nil:
			c, err := h.Scope.Course(ctx, *r.CourseID)
			if err != nil || c == nil {
				return notFoundOr(err)
			}
			school, subject = c.SchoolID, &c.SubjectID
		case r.Type == reporting.TypeSkillMap && r.ClassID != nil:
			sc, ok, err := h.Scope.ClassSchool(ctx, *r.ClassID)
			if err != nil || !ok {
				return notFoundOr(err)
			}
			school, subject = sc, r.SubjectID
		case r.SchoolID != nil:
			school, subject = *r.SchoolID, r.SubjectID
		}
		return nil
	})
	return school, subject, err
}

func notFoundOr(err error) error {
	if err != nil {
		return err
	}
	return problem.New(problem.NotFound, "")
}

// ---------- worker (job report-jobs)

// RunReportJobs is the job report-jobs (F044-AC-01…AC-03): expires the files past their retention, claims queued jobs
// (and running jobs left without progress) and runs them. Returns the number of jobs run.
func (h Reports) RunReportJobs(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.ReportJobs")
	defer func() { done(err) }()
	cfg := h.config()
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return 0, err
	}
	var jobs []write.ReportJobRow
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		if _, err := h.Queue.ExpireReportJobs(ctx, h.now()); err != nil {
			return err
		}
		var err error
		jobs, err = h.Queue.ClaimReportJobs(ctx, h.now().Add(-cfg.StaleAfter), reportJobsPerRun)
		return err
	})
	if err != nil {
		return 0, err
	}
	for _, j := range jobs {
		h.runJob(ctx, j, cfg)
	}
	return len(jobs), nil
}

// scopeOf returns the context of the reads and of the closing write of a job: the TenantScope of its tenant, or for a
// cross-tenant job of System Admin a bypass audited with its tenants (reads: platform_analytics on rollups only).
func scopeOf(ctx context.Context, j write.ReportJobRow) (context.Context, context.Context, error) {
	ctx = tenancy.WithModule(ctx, module)
	if j.TenantID != nil {
		sc, err := authz.EventScope(*j.TenantID)
		if err != nil {
			return nil, nil, err
		}
		c := tenancy.WithScope(ctx, sc)
		return c, c, nil
	}
	rctx, err := tenancy.Bypass(ctx, tenancy.ReasonPlatformAnalytics, tenancy.WithTenantIDs(j.Params.TenantIDs...))
	if err != nil {
		return nil, nil, err
	}
	wctx, err := tenancy.Bypass(ctx, tenancy.ReasonPlatformJob)
	return rctx, wctx, err
}

func (h Reports) runJob(ctx context.Context, j write.ReportJobRow, cfg reporting.Config) {
	start := time.Now()
	rctx, wctx, err := scopeOf(ctx, j)
	if err != nil {
		slog.ErrorContext(ctx, "report job scope", "report_id", j.ID, "error", err)
		return
	}
	key, rows, meta, err := h.produce(rctx, j, cfg)
	at := h.now()
	j.FinishedAt = &at
	if err != nil {
		code := reportErrorCode(err)
		slog.WarnContext(ctx, "report job failed", "report_id", j.ID, "report_type", j.Type, "code", code, "error", err)
		j.Status, j.Error = reporting.StatusFailed, &code
	} else {
		exp := at.Add(cfg.FileRetention)
		j.Status, j.RowCount, j.StorageKey, j.FileExpiresAt = reporting.StatusSucceeded, &rows, &key, &exp
	}
	if err := h.UoW.Do(wctx, func(ctx context.Context) error { return h.finish(ctx, j, meta) }); err != nil {
		slog.ErrorContext(ctx, "report job finish", "report_id", j.ID, "error", err)
		return
	}
	if h.Metrics != nil {
		h.Metrics.Job(j.Status, time.Since(start).Seconds())
	}
}

func reportErrorCode(err error) string {
	var pe *problem.Error
	switch {
	case errors.Is(err, errTooManyRows):
		return reportErrorTooManyRows
	case errors.Is(err, errStorage):
		return reportErrorStorage
	case errors.As(err, &pe) && pe.Code == problem.DashboardQueryTimeout:
		return reportErrorQueryTimeout
	case errors.As(err, &pe) && pe.Code == problem.Unavailable:
		return reportErrorReplica
	case errors.Is(err, reporting.ErrNotAvailable):
		return reportErrorReportTypeNotAllowed
	case errors.Is(err, efficacy.ErrNoKey):
		return reportErrorPseudonymKey
	}
	return reportErrorGeneric
}

var (
	errTooManyRows = errors.New("report: too many rows")
	errStorage     = errors.New("report: object storage")
)

// produce reads the rows batch by batch (one replica transaction each) and writes the file; returns its key and row count.
func (h Reports) produce(ctx context.Context, j write.ReportJobRow, cfg reporting.Config) (string, int, port.ReportMeta, error) {
	meta := port.ReportMeta{Title: "EduWorld — " + j.Type, Watermark: reporting.Watermark(j.RequestedBy, h.now())}
	if j.Type == reporting.TypeEfficacyDataset {
		return h.produceEfficacy(ctx, j, meta)
	}
	if h.DataSession == nil || h.Data == nil || h.Files == nil || h.Writers == nil {
		return "", 0, meta, problem.New(problem.Unavailable, "exports are not configured")
	}
	spec := read.ReportSpec{TenantID: j.TenantID, Type: j.Type, Params: j.Params, MinCellSize: cfg.MinCellSize}
	if j.Type == reporting.TypeRegulationReference {
		meta.Disclaimer = regulation.DefaultDisclaimer()["vi"]
		if h.References != nil {
			_ = h.DataSession.Read(ctx, func(ctx context.Context) error {
				if d, err := h.References.ProfileDisclaimer(ctx, regulation.ProfileFor(1), 1); err == nil && d["vi"] != "" {
					meta.Disclaimer = d["vi"]
				}
				return nil
			})
		}
	}
	w, err := h.Writers.New(j.Format, meta)
	if err != nil {
		return "", 0, meta, err
	}
	after, rows, header := "", 0, false
	for {
		var b read.ReportBatch
		err := h.DataSession.Read(ctx, func(ctx context.Context) error {
			if err := h.Data.Prepare(ctx, exportStatementTimeout); err != nil {
				return err
			}
			var err error
			b, err = h.Data.ReportRows(ctx, spec, after, cfg.BatchRows)
			return err
		})
		if err != nil {
			return "", 0, meta, err
		}
		if !header {
			if err := w.Header(b.Columns); err != nil {
				return "", 0, meta, err
			}
			header = true
		}
		for _, r := range b.Rows {
			if err := w.Row(r); err != nil {
				return "", 0, meta, err
			}
		}
		rows += len(b.Rows)
		if int64(rows) > cfg.MaxRows {
			return "", 0, meta, errTooManyRows
		}
		if b.Next == "" {
			break
		}
		after = b.Next
	}
	data, err := w.Close()
	if err != nil {
		return "", 0, meta, err
	}
	key := reporting.StorageKey(j.TenantID, j.ID, j.Format)
	if err := h.Files.Put(ctx, key, data, reporting.ContentType(j.Format)); err != nil {
		return "", 0, meta, fmt.Errorf("%w: %w", errStorage, err)
	}
	return key, rows, meta, nil
}

// finish stores the outcome; a succeeded export is audited report.exported (and learning.reference_report_exported for
// the reference report); report.completed notifies the portal. No row content is logged nor audited (R14).
func (h Reports) finish(ctx context.Context, j write.ReportJobRow, _ port.ReportMeta) error {
	if err := h.Queue.FinishReportJob(ctx, j); err != nil {
		return err
	}
	school := ""
	if j.Params.SchoolID != nil {
		school = j.Params.SchoolID.String()
	}
	if j.Status == reporting.StatusSucceeded {
		meta := map[string]any{"report_id": j.ID.String(), "report_type": j.Type, "format": j.Format, "row_count": *j.RowCount,
			"scope_level": j.Scope, "requested_by": j.RequestedBy.String()}
		if len(j.Params.TenantIDs) > 0 {
			meta["tenant_ids"] = j.Params.TenantIDs
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: ActionReportExported, TargetType: "report", TargetID: j.ID.String(),
			Result: audit.ResultSuccess, Metadata: meta, SchoolID: school}); err != nil {
			return err
		}
		if j.Type == reporting.TypeEfficacyDataset && j.Params.StudyID != nil {
			// F035-AC-34: the study and the row count only (no PII).
			if err := h.Audit.Record(ctx, audit.Event{Action: ActionDatasetExported, TargetType: "efficacy_study", TargetID: j.Params.StudyID.String(),
				Result: audit.ResultSuccess, Metadata: map[string]any{"report_id": j.ID.String(), "row_count": *j.RowCount,
					"requested_by": j.RequestedBy.String()}}); err != nil {
				return err
			}
		}
		if j.Type == reporting.TypeRegulationReference {
			if err := h.Audit.Record(ctx, audit.Event{Action: ActionReferenceReportExported, TargetType: "report", TargetID: j.ID.String(),
				Result: audit.ResultSuccess, Metadata: map[string]any{"report_id": j.ID.String(), "period": j.Params.Period,
					"requested_by": j.RequestedBy.String()}, SchoolID: school}); err != nil {
				return err
			}
		}
	}
	if h.Events == nil {
		return nil
	}
	return h.Events.Publish(ctx, write.Event{Type: EventReportCompleted, SubjectType: "report", SubjectID: j.ID.String(), TenantID: j.TenantID, Platform: j.TenantID == nil,
		Payload: map[string]any{"report_id": j.ID, "scope_level": j.Scope, "report_type": j.Type, "format": j.Format, "status": j.Status,
			"row_count": j.RowCount, "requested_by": j.RequestedBy}})
}

// produceEfficacy writes the pseudonymous dataset of a study (F035-AC-32, AC-33): CSV, one row per active participant;
// a missing pseudonymization key fails the job before anything is written.
func (h Reports) produceEfficacy(ctx context.Context, j write.ReportJobRow, meta port.ReportMeta) (string, int, port.ReportMeta, error) {
	if h.Efficacy == nil || h.Files == nil || h.Writers == nil || j.Params.StudyID == nil {
		return "", 0, meta, problem.New(problem.Unavailable, "the efficacy dataset is not configured")
	}
	cols, rows, err := h.Efficacy.Dataset(ctx, *j.Params.StudyID)
	if err != nil {
		return "", 0, meta, err
	}
	meta.Title = "EduWorld — efficacy_dataset (định danh giả)"
	w, err := h.Writers.New(j.Format, meta)
	if err != nil {
		return "", 0, meta, err
	}
	if err := w.Header(cols); err != nil {
		return "", 0, meta, err
	}
	for _, r := range rows {
		if err := w.Row(r); err != nil {
			return "", 0, meta, err
		}
	}
	data, err := w.Close()
	if err != nil {
		return "", 0, meta, err
	}
	key := reporting.StorageKey(j.TenantID, j.ID, j.Format)
	if err := h.Files.Put(ctx, key, data, reporting.ContentType(j.Format)); err != nil {
		return "", 0, meta, fmt.Errorf("%w: %w", errStorage, err)
	}
	return key, len(rows), meta, nil
}
