// Package job holds the periodic jobs of module identity run by the worker profile jobs (F011): the SIS
// import upload watcher and the staging anonymizer. Each job is one RunOnce use case; Every schedules it.
package job

import (
	"context"
	"log/slog"
	"time"
)

// RunOnce is one execution of a job.
type RunOnce func(ctx context.Context) (int, error)

// Every runs job at start and then every interval until ctx ends; a failed run is logged and retried at
// the next tick (never in a tight loop).
func Every(ctx context.Context, name string, interval time.Duration, job RunOnce) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if n, err := job(ctx); err != nil {
			slog.WarnContext(ctx, "job failed", "job", name, "error", err)
		} else if n > 0 {
			slog.InfoContext(ctx, "job done", "job", name, "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Intervals of the F011 jobs [Proposed].
const (
	SisImportWatchInterval  = 15 * time.Second // detect uploaded CSV files
	SisStagingAnonymizeTick = time.Hour        // sis-staging-anonymizer
)

// SisSchedulerInterval is the tick of the job sis-scheduler (F017): twice a minute so no scheduled minute is
// missed; jobs are deduplicated per connection, mode and minute [Proposed].
const SisSchedulerInterval = 30 * time.Second
