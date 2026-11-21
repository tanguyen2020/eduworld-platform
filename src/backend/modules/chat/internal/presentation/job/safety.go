package job

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// Jobs of the Platform Safety Team (F121, worker profile jobs; one replica per cycle through ew:lock:job:<name>).
const (
	// ModerationSLAEscalator escalates the cases past report_sla and refreshes the queue gauges (F121-AC-22, AC-23).
	ModerationSLAEscalator = "moderation-sla-escalator"
	// SchoolNotificationReminder reminds the unacknowledged school notifications past ack_due_at (F121-AC-20).
	SchoolNotificationReminder = "school-notification-reminder"
	// SanctionExpirer publishes sanction.lifted for the sanctions past ends_at and releases stale holds (F121-AC-15).
	SanctionExpirer = "sanction-expirer"
)

// SafetyJobs are the use cases run by the Safety Team jobs (command.Safety).
type SafetyJobs interface {
	EscalateOverdueCases(ctx context.Context, gauges command.SafetyGauges,
		counts func(ctx context.Context) ([]read.OpenCount, int, error)) (int, error)
	RemindOverdueNotifications(ctx context.Context) (int, error)
	ExpireSanctions(ctx context.Context) (int, error)
}

// SLAEscalatorJob returns the job function of moderation-sla-escalator.
func SLAEscalatorJob(h SafetyJobs, gauges command.SafetyGauges,
	counts func(ctx context.Context) ([]read.OpenCount, int, error)) func(ctx context.Context) (int, error) {
	return func(ctx context.Context) (int, error) { return h.EscalateOverdueCases(ctx, gauges, counts) }
}

// ReminderJob returns the job function of school-notification-reminder.
func ReminderJob(h SafetyJobs) func(ctx context.Context) (int, error) {
	return h.RemindOverdueNotifications
}

// SanctionExpirerJob returns the job function of sanction-expirer.
func SanctionExpirerJob(h SafetyJobs) func(ctx context.Context) (int, error) {
	return h.ExpireSanctions
}
