package job

import (
	"context"
	"log/slog"
	"time"
)

// GameplayChatPurge is the job gameplay-chat-purge (ADR-0033, F117 US4): worker-chat only, database role chat_purger, one
// instance per run through ew:lock:job:gameplay-chat-purge, hourly. The ONLY physical delete of the platform (R21).
const GameplayChatPurge = "gameplay-chat-purge"

// ChatPartitionMaintainer keeps the daily partitions of the chat message tables ahead (worker-chat, chat_purger).
const ChatPartitionMaintainer = "chat-partition-maintainer"

// Runner is one run of a job.
type Runner interface {
	Run(ctx context.Context) (int, error)
}

// Every runs r at start and then every interval until ctx ends; a failed run is logged and retried at the next tick
// (worker-chat runs these jobs itself: module.Registry.Job only schedules in worker jobs).
func Every(name string, interval time.Duration, r Runner) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			n, err := r.Run(ctx)
			if err != nil {
				slog.WarnContext(ctx, "job failed", "module", "chat", "job", name, "error", err)
			} else if n > 0 {
				slog.InfoContext(ctx, "job done", "module", "chat", "job", name, "count", n)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-t.C:
			}
		}
	}
}

// PersistentChatRetention is the job persistent-chat-retention (F120 US6): worker-chat, role chat_app, anonymisation of the
// persistent channels (club team, direct) — never a DELETE (R21, ADR-0070).
const PersistentChatRetention = "persistent-chat-retention"
