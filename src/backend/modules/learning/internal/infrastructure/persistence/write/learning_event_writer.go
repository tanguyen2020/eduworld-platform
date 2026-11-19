package write

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// LearningEventWriter implements write.LearningEventWriter with explicit SQL on the partitioned append-only
// learning.learning_events (EVENT_DESIGN §5.4): the row is written in the transaction of ctx; an idempotency key
// already recorded for the tenant adds nothing. class_id is the active class of the student at that moment
// (ref.enrollments snapshot) when the caller does not give it.
type LearningEventWriter struct{}

var _ write.LearningEventWriter = LearningEventWriter{}

func uuidArray(ids []uuid.UUID) string {
	b := []byte{'{'}
	for i, id := range ids {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, id.String()...)
	}
	return string(append(b, '}'))
}

// Write implements write.LearningEventWriter.
func (LearningEventWriter) Write(ctx context.Context, e write.LearningEvent) error {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return entx.ErrNoTransaction
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("learning event payload: %w", err)
	}
	actor := e.ActorType
	if actor == "" {
		actor = "student"
	}
	// The NOT EXISTS and the enrollment snapshot (ref.enrollments has no deleted_at: status) read the tenant $6 only.
	// tenancyguard:ok append-only learning_events without deleted_at; insert of the session tenant (RLS insert policy)
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.learning_events (event_id, occurred_at, event_type, student_id, tenant_id, school_id,
		class_id, game_session_id, activity_id, content_id, content_version_id, objective_ids, context, actor_type, payload, idempotency_key)
		SELECT $1::uuid, $2::timestamptz, $3::varchar, $4::uuid, $6::uuid, $5::uuid,
			COALESCE($7::uuid, (SELECT en.class_id FROM ref.enrollments en WHERE en.student_id = $4 AND en.tenant_id = $6
				AND en.status = 'active' ORDER BY en.start_date DESC LIMIT 1)),
			$8::uuid, $9::uuid, $10::uuid, $11::uuid, $12::uuid[], $13::varchar, $14::varchar, $15::jsonb, $16::varchar
		WHERE NOT EXISTS (SELECT 1 FROM learning.learning_events le WHERE le.idempotency_key = $16::varchar AND le.tenant_id = $6::uuid)
		ON CONFLICT DO NOTHING`,
		e.ID, e.OccurredAt.UTC(), e.Type, e.StudentID, e.SchoolID, e.TenantID, e.ClassID, e.GameSessionID, e.ActivityID, e.ContentID,
		e.ContentVersionID, uuidArray(e.ObjectiveIDs), e.Context, actor, payload, e.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("learning event %s: %w", e.Type, err)
	}
	return nil
}
