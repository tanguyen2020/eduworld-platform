//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// F074-AC-08 (AC-GAM-015): learning_objective.mastered posts xp_objective_mastered once per (student, objective) — a
// redelivered event and the same objective mastered again after decay add nothing; another objective earns again.
func TestObjectiveMasteredOnce(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	lo := uuid.New()
	mastered := func(obj uuid.UUID, previous string) {
		e.mustDeliver(ctx, t, uc, e.envelope("learning_objective.mastered", "learning", s.tenant, map[string]any{"student_id": s.id,
			"objective_id": obj, "previous_level": previous, "value": 0.9, "evidence_count": 6, "distinct_days": 3, "model_version": "m1",
			"new_level": "mastered", "mastered_at": e.clock.Now()}))
	}
	mastered(lo, "proficient")
	mastered(lo, "proficient")        // redelivered under a new event id
	e.clock.Add(30 * 24 * 3600 * 1e9) // decay, then mastered again a month later
	mastered(lo, "proficient")
	rows := e.ledger(ctx, t, s)
	if len(rows) != 1 || rows[0].amount != 50 || rows[0].sourceCode != "mastery.objective_mastered" || rows[0].sourceID != lo {
		t.Fatalf("ledger %+v", rows)
	}
	mastered(uuid.New(), "developing")
	if rows = e.ledger(ctx, t, s); len(rows) != 2 {
		t.Fatalf("second objective: %+v", rows)
	}
}
