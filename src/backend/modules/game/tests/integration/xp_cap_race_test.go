//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
)

// T014 (edge case of the spec): 20 concurrent events of one student near the cap never exceed xp_daily_cap (the cap
// counters are locked FOR UPDATE in the award transaction).
func TestXPCapRace(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	p := defaultPolicy()
	p.DailyCap = 500
	uc := e.progressionWith(p)
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main")) // creates the profile first
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- e.deliver(ctx, t, uc, e.questEvent(s, "main"))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	rows := e.ledger(ctx, t, s)
	if total := sumAmounts(rows); total != 500 || len(rows) != 21 {
		t.Fatalf("total %d over %d rows (cap 500)", total, len(rows))
	}
	var granted int
	if err := e.platform.QueryRowContext(ctx, `SELECT granted FROM gamification.xp_daily_caps WHERE student_id = $1 AND bucket = 'xp:total'`,
		s.id).Scan(&granted); err != nil || granted != 500 {
		t.Fatalf("cap counter %d %v", granted, err)
	}
	if _, xp := e.xpOf(ctx, t, s); xp != 500 {
		t.Fatalf("xp_total %d", xp)
	}
}
