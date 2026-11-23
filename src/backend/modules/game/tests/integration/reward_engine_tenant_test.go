//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

// F074-AC-11 (TC-TEN-005): an envelope whose tenant is not the tenant of the student in ref.students (or a student not
// projected) posts nothing; the durable consumer game.reward_engine retries, then dead-letters it, and counts it. A
// valid event of the same run is rewarded.
func TestRewardEngineTenantIsolation(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	b := newBus(ctx, t)
	if err := natsx.EnsureStreams(ctx, b.js, "learning", natsx.StreamOptions{}); err != nil {
		t.Fatal(err)
	}
	before := testutil.ToFloat64(observability.StudentUnknown().WithLabelValues(messaging.RewardEngineName))
	b.run(ctx, t, consumer.RewardEngineOptions(consumer.RewardEngine{Awards: e.progressionUC()}, e.uow, e.app, b.js))
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	wrong := e.lessonEvent(an, uuid.New(), []int{3}, "practice")
	tb := e.tenantB
	wrong.TenantID = &tb // student of tenant A announced in tenant B
	ghost := e.lessonEvent(student{id: uuid.New(), tenant: e.tenantA}, uuid.New(), []int{3}, "practice")
	good := e.lessonEvent(an, uuid.New(), []int{3}, "practice")
	for _, env := range []any{wrong, ghost, good} {
		raw, _ := json.Marshal(env)
		if _, err := b.js.Publish(ctx, natsx.EventSubject("learning", "lesson.completed"), raw); err != nil {
			t.Fatal(err)
		}
	}
	e.processed(ctx, t, messaging.RewardEngineName, good)
	e.deadLettered(ctx, t, messaging.RewardEngineName, wrong)
	e.deadLettered(ctx, t, messaging.RewardEngineName, ghost)
	rows := e.ledger(ctx, t, an)
	if len(rows) != 1 || rows[0].sourceID != uuid.MustParse(mustLessonVersion(t, good.Payload)) {
		t.Fatalf("ledger of A: %+v", rows)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.xp_ledger WHERE tenant_id = $1`, e.tenantB); n != 0 {
		t.Fatalf("rows in tenant B: %d", n)
	}
	if got := testutil.ToFloat64(observability.StudentUnknown().WithLabelValues(messaging.RewardEngineName)) - before; got < 2 {
		t.Fatalf("student_unknown metric +%v", got)
	}
}

func mustLessonVersion(t *testing.T, raw []byte) string {
	t.Helper()
	var p struct {
		LessonVersionID string `json:"lesson_version_id"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	return p.LessonVersionID
}
