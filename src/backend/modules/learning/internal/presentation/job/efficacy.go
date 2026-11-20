package job

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
)

// EfficacyRunner runs the waves of the efficacy studies (command.Efficacy, F035 US3, US6).
type EfficacyRunner interface {
	RunWaveMonitor(ctx context.Context, limit int) (command.WaveMonitorReport, error)
}

// EfficacyWaveMonitor is the job efficacy-wave-monitor (every minute): opens the waves whose window started (participant
// and mastery snapshots) and closes the waves whose window ended.
type EfficacyWaveMonitor struct{ Runner EfficacyRunner }

// RunOnce opens and closes the due waves and returns how many changed.
func (j EfficacyWaveMonitor) RunOnce(ctx context.Context) (int, error) {
	r, err := j.Runner.RunWaveMonitor(ctx, 0)
	return r.Opened + r.Closed, err
}

// EfficacyReportRunner runs the queued efficacy reports (command.Efficacy, F035 US6).
type EfficacyReportRunner interface {
	RunEfficacyReports(ctx context.Context) (int, error)
}

// EfficacyReports is the job efficacy-reports: computes the queued efficacy reports.
type EfficacyReports struct{ Runner EfficacyReportRunner }

// RunOnce computes the queued reports and returns how many.
func (j EfficacyReports) RunOnce(ctx context.Context) (int, error) {
	return j.Runner.RunEfficacyReports(ctx)
}

// ExperimentRunner runs the experiment jobs (command.Experiments, F035 US5).
type ExperimentRunner interface {
	RunGuardrail(ctx context.Context) (int, error)
	RunLifecycle(ctx context.Context) (int, error)
}

// ExperimentGuardrail is the job experiment-guardrail (daily, F035-AC-26).
type ExperimentGuardrail struct{ Runner ExperimentRunner }

// RunOnce evaluates the running experiments.
func (j ExperimentGuardrail) RunOnce(ctx context.Context) (int, error) {
	return j.Runner.RunGuardrail(ctx)
}

// ExperimentLifecycle is the job experiment-lifecycle: running experiments past end_at complete.
type ExperimentLifecycle struct{ Runner ExperimentRunner }

// RunOnce completes the due experiments.
func (j ExperimentLifecycle) RunOnce(ctx context.Context) (int, error) {
	return j.Runner.RunLifecycle(ctx)
}
