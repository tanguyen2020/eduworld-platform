//go:build integration

package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-05 (TC-AUTH-016, SC-001): two devices starting at the same moment, 100 times: exactly one 201 and one 409
// ACTIVE_SESSION_ON_OTHER_DEVICE each time, never two active sessions (advisory lock + unique partial index).
func TestConcurrentStart(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	const rounds = 100
	for i := range rounds {
		s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
		devs := []device{e.device(s), e.device(s)}
		var wg sync.WaitGroup
		results := make([]resp, 2)
		start := make(chan struct{})
		for j := range devs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				results[j] = e.start(t, s, devs[j], newKey(), nil)
			}()
		}
		close(start)
		wg.Wait()
		created, conflicts := 0, 0
		for _, r := range results {
			switch {
			case r.status == http.StatusCreated:
				created++
			case r.status == http.StatusConflict && r.code() == string(problem.ActiveSessionOnOtherDevice):
				conflicts++
			default:
				t.Fatalf("round %d: unexpected %d %s", i, r.status, r.raw)
			}
		}
		if created != 1 || conflicts != 1 {
			t.Fatalf("round %d: %d created, %d conflicts", i, created, conflicts)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM (SELECT student_id FROM game.game_sessions WHERE ended_at IS NULL
		GROUP BY student_id HAVING count(*) > 1) d`); n != 0 {
		t.Fatalf("%d students with two active sessions", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE ended_at IS NULL`); n != rounds {
		t.Fatalf("active sessions = %d", n)
	}
}
