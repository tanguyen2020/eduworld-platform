package job

import "context"

// Names of the jobs of the Game Operations Console (F078).
const (
	ScheduledPublish = "scheduled-publish"
	MediaPipeline    = "game-media-pipeline"
	RenderRetry      = "render-retry"
)

// ScheduledPublisher applies the due scheduled publications (command.GameAdmin).
type ScheduledPublisher interface {
	ApplyDueSchedules(ctx context.Context) (int, error)
}

// MediaRunner is the game media pipeline and the render retries (command.GameMedia).
type MediaRunner interface {
	ProcessUploads(ctx context.Context) (int, error)
	RetryRenders(ctx context.Context) (int, error)
}

// ScheduledPublishJob is one tick of scheduled-publish (F078-AC-12): each due publication is applied once (SKIP LOCKED),
// within one tick of its time.
func ScheduledPublishJob(p ScheduledPublisher) RunOnce {
	return func(ctx context.Context) (int, error) { return p.ApplyDueSchedules(ctx) }
}

// MediaPipelineJob is one tick of game-media-pipeline (F078-AC-18): uploaded files are checked and published.
func MediaPipelineJob(r MediaRunner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.ProcessUploads(ctx) }
}

// RenderRetryJob is one tick of render-retry (F078-AC-20): failed renders whose backoff elapsed are enqueued again.
func RenderRetryJob(r MediaRunner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.RetryRenders(ctx) }
}

// Names of the translation jobs (F082).
const (
	TranslationJobs     = "translation-jobs"
	TranslationBackfill = "game-translation-state-backfill"
)

// TranslationRunner runs the queued translation jobs (command.Translations).
type TranslationRunner interface {
	RunNext(ctx context.Context) (int, error)
}

// StateBackfiller computes the missing translation states (command.TranslationBackfill).
type StateBackfiller interface {
	Run(ctx context.Context) (int, error)
}

// TranslationJobsJob is one tick of translation-jobs (F082-AC-12, AC-13): one queued job per tick.
func TranslationJobsJob(r TranslationRunner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.RunNext(ctx) }
}

// TranslationBackfillJob is one tick of game-translation-state-backfill (Clarifications F082 2026-11-26).
func TranslationBackfillJob(b StateBackfiller) RunOnce {
	return func(ctx context.Context) (int, error) { return b.Run(ctx) }
}
