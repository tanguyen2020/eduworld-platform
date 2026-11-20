//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/reporting"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/export"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// f044Kit builds the use cases of F044 on the test environment: exports on the dashboard session (learning_dashboard and
// learning_platform), files on floci when the media kit is loaded, the quality use cases and the reads, all with the test
// clock. They are returned as pointers so a test can tune the configuration (limits).
func f044Kit(e *env, dash *entx.ReadSession, mk *mediaKit, qcmd command.Questions, codec *page.Codec, clk *testClock) (*command.Reports,
	*command.Quality, *query.Reports) {
	cfg := reporting.DefaultConfig()
	r := &command.Reports{UoW: e.uow, Jobs: write.ReportRepo{}, Queue: write.ReportRepo{}, Read: e.read, Scope: read.QualityModel{},
		DataSession: dash, Data: read.ReportModel{}, References: read.QualityModel{}, Writers: export.Writers{}, Audit: write.Auditor{R: e.rec},
		Events: write.Publisher{W: e.writer}, Config: cfg, Clock: clk.Now, Metrics: observability.ReportingCounter{}}
	q := &query.Reports{Session: e.read, Jobs: read.ReportModel{}, Courses: read.QualityModel{}, References: read.QualityModel{},
		Quality: read.QualityModel{}, Exams: read.QualityModel{}, Config: cfg, Cursors: codec, Clock: clk.Now}
	if mk != nil {
		files := storage.ExportBucket{S3: mk.s3}
		r.Files, q.Files = files, files
	}
	quality := &command.Quality{UoW: e.uow, Items: write.QualityRepo{}, Calibrations: write.QualityRepo{}, Exams: write.QualityRepo{},
		References: write.QualityRepo{}, Tenants: write.RollupRepo{}, Questions: qcmd, Audit: write.Auditor{R: e.rec},
		Config: command.DefaultQualityConfig(), Clock: clk.Now, Metrics: observability.ReportingCounter{}}
	return r, quality, q
}

// reportsProxy, qualityProxy and reportqProxy serve the routes with the current use cases of the test.
type reportsProxy struct{ r *command.Reports }

func (p reportsProxy) CreateReport(ctx context.Context, in command.ReportInput) (dto.ReportJobView, error) {
	return p.r.CreateReport(ctx, in)
}

type qualityProxy struct{ q *command.Quality }

func (p qualityProxy) AcceptCalibration(ctx context.Context, id uuid.UUID) (dto.CalibrationProposalView, error) {
	return p.q.AcceptCalibration(ctx, id)
}
func (p qualityProxy) DismissCalibration(ctx context.Context, id uuid.UUID) (dto.CalibrationProposalView, error) {
	return p.q.DismissCalibration(ctx, id)
}

type reportqProxy struct{ q *query.Reports }

func (p reportqProxy) GetReport(ctx context.Context, s reporting.Scope, id uuid.UUID) (dto.ReportJobView, error) {
	return p.q.GetReport(ctx, s, id)
}
func (p reportqProxy) GetCourseResults(ctx context.Context, in query.CourseResultsInput) (query.CourseResultsPage, error) {
	return p.q.GetCourseResults(ctx, in)
}
func (p reportqProxy) GetReferenceReport(ctx context.Context, in query.ReferenceInput) (query.ReferencePage, error) {
	return p.q.GetReferenceReport(ctx, in)
}
func (p reportqProxy) GetQuestionStats(ctx context.Context, id uuid.UUID) (dto.QuestionStatsView, error) {
	return p.q.GetQuestionStats(ctx, id)
}
func (p reportqProxy) ListCalibrationProposals(ctx context.Context, status string, in query.PageInput) (query.Page[dto.CalibrationProposalView], error) {
	return p.q.ListCalibrationProposals(ctx, status, in)
}
func (p reportqProxy) GetExamSessionReport(ctx context.Context, id uuid.UUID) (dto.ExamSessionReportView, error) {
	return p.q.GetExamSessionReport(ctx, id)
}

// exec runs a statement as learning_platform (fixtures).
func (c *curEnv) exec(ctx context.Context, t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := c.platform.ExecContext(ctx, q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}
