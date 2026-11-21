package job

import "context"

// ModerationSwitchExpirer is the job that switches the moderation steps back on at their expires_at and expires the
// disabling requests not approved within 24 hours (F116-AC-25, F116-AC-32; worker profile jobs, one replica per cycle
// through the lock ew:lock:job:moderation-switch-expirer).
const ModerationSwitchExpirer = "moderation-switch-expirer"

// ModerationSwitchExpirerJob returns the job function (command.SwitchExpiry).
func ModerationSwitchExpirerJob(h Expirer) func(ctx context.Context) (int, error) {
	return h.Run
}
