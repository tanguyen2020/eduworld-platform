package job

import "context"

// ChallengeDeadline is the name of the F076 job: it ends the challenge attempts past their due time from PostgreSQL
// (fallback of the room timer, F076-AC-13, AC-19).
const ChallengeDeadline = "challenge-deadline"

// ChallengeRunner is the use case behind challenge-deadline (command.Challenges).
type ChallengeRunner interface {
	ExpireDue(ctx context.Context, batch int) (int, error)
}

// ChallengeDeadlineJob is one run of challenge-deadline.
func ChallengeDeadlineJob(r ChallengeRunner, batch int) RunOnce {
	return func(ctx context.Context) (int, error) { return r.ExpireDue(ctx, batch) }
}
