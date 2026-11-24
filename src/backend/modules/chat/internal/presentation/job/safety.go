package job

import (
	"context"
	"time"

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

// Jobs of the Safety Team operations (F124, worker profile jobs; one replica per cycle through ew:lock:job:<name>).
const (
	// OncallEscalator pages the next on-call level of the open p1 cases and refreshes the coverage gauge (F124-AC-14, AC-15).
	OncallEscalator = "oncall-escalator"
	// BreakGlassExpirer marks the approved break-glass grants past expires_at expired (F124-AC-20).
	BreakGlassExpirer = "break-glass-expirer"
	// SafetyOpsTick is the period of both jobs (plan: every minute).
	SafetyOpsTick = time.Minute
)

// SafetyOpsJobs are the use cases run by the F124 jobs (command.Safety).
type SafetyOpsJobs interface {
	EscalateUnclaimedP1(ctx context.Context) (int, error)
	ExpireBreakGlass(ctx context.Context) (int, error)
}

// OncallEscalatorJob returns the job function of oncall-escalator.
func OncallEscalatorJob(h SafetyOpsJobs) func(ctx context.Context) (int, error) {
	return h.EscalateUnclaimedP1
}

// BreakGlassExpirerJob returns the job function of break-glass-expirer.
func BreakGlassExpirerJob(h SafetyOpsJobs) func(ctx context.Context) (int, error) {
	return h.ExpireBreakGlass
}
