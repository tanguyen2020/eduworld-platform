// Package job holds the scheduled jobs of module chat (worker profile jobs).
package job

import (
	"context"
)

// ExpireFriendRequests is the job that expires pending friend requests past friend_request_ttl and soft-deletes
// expired friend codes (F095-AC-07).
const ExpireFriendRequests = "expire-friend-requests"

// Expirer is one run of the job (command.Expiry).
type Expirer interface {
	Run(ctx context.Context) (int, error)
}

// ExpireFriendRequestsJob returns the job function.
func ExpireFriendRequestsJob(h Expirer) func(ctx context.Context) (int, error) {
	return h.Run
}
