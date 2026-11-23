//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// F074-AC-27 (TC-GAM-004): lesson.completed posts the rule lesson.completed with source_type lesson and source_id =
// lesson_version_id, once per student × lesson version (also when redelivered); content ≥ 2 grades below the student ×
// xp_below_grade_multiplier, full XP in mode assignment or with a recommendation.
func TestLessonCompletedXP(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, s, 5)
	lv := uuid.New()
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, lv, []int{5}, "practice"))
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, lv, []int{5}, "practice")) // the same lesson version again
	rows := e.ledger(ctx, t, s)
	if len(rows) != 1 || rows[0].sourceType != "lesson" || rows[0].sourceCode != "lesson.completed" || rows[0].sourceID != lv ||
		rows[0].amount != 20 || rows[0].ruleID.UUID.String() != ruleLesson {
		t.Fatalf("ledger %+v", rows)
	}
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, uuid.New(), []int{3}, "practice"))   // 2 grades below: × 0.25
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, uuid.New(), []int{3}, "assignment")) // assigned: full
	rec := e.lessonEvent(s, uuid.New(), []int{2, 3}, "practice")
	rec.Payload = withKey(t, rec.Payload, "recommendation_id", uuid.NewString()) // recommended review: full
	e.mustDeliver(ctx, t, uc, rec)
	rows = e.ledger(ctx, t, s)
	if len(rows) != 4 || rows[1].amount != 5 || rows[2].amount != 20 || rows[3].amount != 20 {
		t.Fatalf("grade multiplier: %+v", rows)
	}
}
