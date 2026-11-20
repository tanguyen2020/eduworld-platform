package job

import "context"

// ReportRunner runs the export queue (command.Reports, F044 US1).
type ReportRunner interface {
	RunReportJobs(ctx context.Context) (int, error)
}

// ReportJobs is the job report-jobs of the worker profile jobs (F044-AC-01…AC-03, DASHBOARD_DATA §10): each run expires the
// files past their retention and runs the queued exports.
type ReportJobs struct{ Runner ReportRunner }

// RunOnce runs the queued exports and returns how many.
func (j ReportJobs) RunOnce(ctx context.Context) (int, error) { return j.Runner.RunReportJobs(ctx) }

// QualityRunner runs the quality jobs (command.Quality, F044 US3–US5).
type QualityRunner interface {
	ComputeQuestionStats(ctx context.Context) (int, error)
	ProposeCalibrations(ctx context.Context) (int, error)
	ComputeReferenceLevels(ctx context.Context) (int, error)
}

// ItemAnalysisNightly is the job item-analysis-nightly (F044-AC-16, QUESTION_BANK §9).
type ItemAnalysisNightly struct{ Runner QualityRunner }

// RunOnce recomputes the statistics of the versions answered recently.
func (j ItemAnalysisNightly) RunOnce(ctx context.Context) (int, error) {
	return j.Runner.ComputeQuestionStats(ctx)
}

// CalibrationWeekly is the job calibration-weekly (F044-AC-19, QUESTION_BANK §9.1).
type CalibrationWeekly struct{ Runner QualityRunner }

// RunOnce creates the calibration proposals.
func (j CalibrationWeekly) RunOnce(ctx context.Context) (int, error) {
	return j.Runner.ProposeCalibrations(ctx)
}

// ReferenceLevelsNightly is the job reference-levels-nightly (F044-AC-11, ASSESSMENT_REGULATION_PROFILE §4).
type ReferenceLevelsNightly struct{ Runner QualityRunner }

// RunOnce recomputes the reference levels of the current period.
func (j ReferenceLevelsNightly) RunOnce(ctx context.Context) (int, error) {
	return j.Runner.ComputeReferenceLevels(ctx)
}
