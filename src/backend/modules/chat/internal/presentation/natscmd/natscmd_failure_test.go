package natscmd

import (
	"context"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Delta F112-6: reopen_at of CURFEW_ACTIVE and of CHAT_NOT_ALLOWED outside the allowed hours reaches the client as
// details.reopen_at (RFC 3339, UTC); category_code stays the only other detail (never a matched term, F116-AC-05).
func TestFailureDetails(t *testing.T) {
	at := time.Date(2026, 11, 29, 7, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	r := failure(context.Background(), problem.New(problem.ChatNotAllowed, "").With("reopen_at", at))
	if r.Problem == nil || r.Problem.Code != "CHAT_NOT_ALLOWED" || r.Problem.Details["reopen_at"] != "2026-11-29T00:00:00Z" || len(r.Problem.Details) != 1 {
		t.Fatalf("chat not allowed: %+v", r.Problem)
	}
	r = failure(context.Background(), problem.New(problem.ChatNotAllowed, ""))
	if r.Problem.Details != nil {
		t.Fatalf("no reopen time: %+v", r.Problem.Details)
	}
	r = failure(context.Background(), problem.New(problem.CurfewActive, "").With("reopen_at", at.UTC()).With("category_code", "x"))
	if r.Problem.Details["reopen_at"] != "2026-11-29T00:00:00Z" || r.Problem.Details["category_code"] != "x" {
		t.Fatalf("curfew: %+v", r.Problem.Details)
	}
}
