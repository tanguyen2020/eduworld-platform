package job

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
)

// ExamRunner runs the exam jobs (command.Exams, F042).
type ExamRunner interface {
	RunWindows(ctx context.Context, limit int) (command.ExamWindowReport, error)
	AutoSubmitExpired(ctx context.Context, limit int) (command.AutoSubmitReport, error)
}

// ExamWindow is the job exam-window of the worker profile jobs (F042-AC-16, EXAM_ENGINE §5): sessions open at window_start
// and close at window_end (auto-submit, absent, after_close release).
type ExamWindow struct {
	Runner ExamRunner
	Batch  int
}

// RunOnce opens and closes up to Batch sessions of each kind.
func (w ExamWindow) RunOnce(ctx context.Context) (int, error) {
	rep, err := w.Runner.RunWindows(ctx, w.Batch)
	return rep.Opened + rep.Closed, err
}

// ExamAutoSubmit is the job exam.autosubmit (F042-AC-16, EXAM_ENGINE §7.2): every exam_autosubmit_interval the attempts
// past deadline_at + exam_offline_sync_grace are submitted with auto_timeout.
type ExamAutoSubmit struct {
	Runner ExamRunner
	Batch  int
}

// RunOnce submits up to Batch expired attempts.
func (w ExamAutoSubmit) RunOnce(ctx context.Context) (int, error) {
	rep, err := w.Runner.AutoSubmitExpired(ctx, w.Batch)
	return rep.Submitted, err
}
