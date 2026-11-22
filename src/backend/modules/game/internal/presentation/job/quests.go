package job

import "context"

// Names of the F072 jobs.
const (
	QuestPeriodExpiry = "quest-period-expiry"
	QuestFailureTimer = "quest-failure-timer"
)

// QuestRunner is the use case behind the quest jobs (command.Quests).
type QuestRunner interface {
	ExpirePeriodQuests(ctx context.Context, batch int) (int, error)
	ApplyQuestFailures(ctx context.Context, batch int) (int, error)
}

// QuestPeriodExpiryJob is one run of quest-period-expiry (F072-AC-23).
func QuestPeriodExpiryJob(r QuestRunner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.ExpirePeriodQuests(ctx, Batch) }
}

// QuestFailureTimerJob is one run of quest-failure-timer (F072-AC-26).
func QuestFailureTimerJob(r QuestRunner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.ApplyQuestFailures(ctx, Batch) }
}
