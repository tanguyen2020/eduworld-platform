//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// F072-AC-21 (T033, TC-SVC-003): quest.completed of module game with objective_ids writes the learning event
// quest.completed once per quest_progress_id (mode quest, subject quest, valid against learning-event.schema.json), even
// when the event is delivered again with another event id; a quest without objectives writes nothing; a malformed event
// is permanent (DLQ); another tenant never sees the event.
func TestLearningEventOnQuestCompleted(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	student, school := uuid.New(), uuid.New()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO ref.schools (school_id, tenant_id, status, source_version) VALUES ($1, $2, 'active', 1)`,
		school, e.tenantA); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version)
		VALUES ($1, $2, $3, 3, 1)`, student, e.tenantA, school); err != nil {
		t.Fatal(err)
	}
	c := consumer.LearningEventOnQuestCompleted{Recorder: command.QuestLearningEvents{Students: read.SchoolSettings{}, Learning: write.LearningEventWriter{}}}
	progress, objective := uuid.New(), uuid.New()
	deliver := func(tenant uuid.UUID, payload map[string]any) error {
		b, _ := json.Marshal(payload)
		tt := tenant
		env := outbox.Envelope{EventID: uuid.New(), EventType: "quest.completed", SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "game",
			TenantID: &tt, Payload: b}
		return e.uow.InTenantTx(ctx, scopeOf(t, tenant), func(ctx context.Context) error { return c.Handle(ctx, env) })
	}
	started := time.Now().UTC().Add(-10 * time.Minute)
	payload := map[string]any{"quest_progress_id": progress, "quest_id": uuid.New(), "quest_version_id": uuid.New(), "quest_type": "learning",
		"student_id": student, "objective_ids": []uuid.UUID{objective}, "completion_no": 1, "rewards": []any{}, "started_at": started,
		"completed_at": time.Now().UTC()}
	for i := 0; i < 3; i++ {
		if err := deliver(e.tenantA, payload); err != nil {
			t.Fatalf("deliver %d: %v", i, err)
		}
	}
	var n int
	var progressRef, mode string
	err := e.uow.InTenantTx(ctx, scopeOf(t, e.tenantA), func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		return tx.QueryRowContext(ctx, `SELECT count(*), max(payload->>'quest_progress_id'), max(context) FROM learning.learning_events
			WHERE event_type = 'quest.completed' AND student_id = $1`, student).Scan(&n, &progressRef, &mode)
	})
	if err != nil || n != 1 || progressRef != progress.String() || mode != "quest" {
		t.Fatalf("learning event: %d %s %s %v", n, progressRef, mode, err)
	}
	other := payload
	other["quest_progress_id"], other["objective_ids"] = uuid.New(), []uuid.UUID{}
	if err := deliver(e.tenantA, other); err != nil {
		t.Fatal(err)
	}
	bad := map[string]any{"student_id": student}
	if err := deliver(e.tenantA, bad); err == nil || !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("malformed: %v", err)
	}
	var total, inB int
	_ = e.platform.QueryRowContext(ctx, `SELECT count(*) FROM learning.learning_events WHERE event_type = 'quest.completed'`).Scan(&total)
	_ = e.uow.InTenantTx(ctx, scopeOf(t, e.tenantB), func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		return tx.QueryRowContext(ctx, `SELECT count(*) FROM learning.learning_events WHERE event_type = 'quest.completed'`).Scan(&inB)
	})
	if total != 1 || inB != 0 {
		t.Fatalf("quest without objectives or tenant isolation: total %d, tenant B %d", total, inB)
	}
}
