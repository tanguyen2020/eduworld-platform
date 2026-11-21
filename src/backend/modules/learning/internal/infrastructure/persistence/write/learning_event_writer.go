package write

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/qbschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// LearningEventWriter implements write.LearningEventWriter with explicit SQL on the partitioned append-only
// learning.learning_events (EVENT_DESIGN §5.4): the row is written in the transaction of ctx; an idempotency key
// already recorded for the tenant adds nothing. grade_level is the grade of the student at that moment (ref.students,
// ADR-0095) when the caller does not give it. Every event is checked against
// learning-event.schema.json before it is written (F031-AC-01, F031-AC-05): an event outside the schema fails the
// transaction of the change it records.
type LearningEventWriter struct{}

var _ write.LearningEventWriter = LearningEventWriter{}

// Producer is the producer of the learning events of module learning (envelope "producer").
const Producer = "learning/learning_record"

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

// uniqueIDs keeps the first occurrence of each id (objective_ids are a set, uniqueItems in the schema).
func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
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
	e.ActorType = actor
	e.ObjectiveIDs = uniqueIDs(e.ObjectiveIDs)
	doc, err := json.Marshal(Envelope(e, payload, time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("learning event envelope: %w", err)
	}
	if err := qbschema.ValidateLearningEvent(doc); err != nil {
		return fmt.Errorf("learning event %s: %w", e.Type, err)
	}
	// The NOT EXISTS and the grade of ref.students (the grade at the time, even of a soft-deleted student) read the tenant $6
	// only.
	// tenancyguard:ok append-only learning_events without deleted_at; insert of the session tenant (RLS insert policy)
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.learning_events (event_id, occurred_at, event_type, student_id, tenant_id, school_id,
		game_session_id, activity_id, content_id, content_version_id, objective_ids, context, actor_type, payload, idempotency_key,
		efficacy_wave_id, experiment_arms, grade_level)
		SELECT $1::uuid, $2::timestamptz, $3::varchar, $4::uuid, $6::uuid, $5::uuid,
			$7::uuid, $8::uuid, $9::uuid, $10::uuid, $11::uuid[], $12::varchar, $13::varchar, $14::jsonb, $15::varchar,
			$16::uuid, $17::varchar(97)[],
			COALESCE($18::smallint, (SELECT st.grade_level FROM ref.students st WHERE st.student_id = $4 AND st.tenant_id = $6))
		WHERE NOT EXISTS (SELECT 1 FROM learning.learning_events le WHERE le.idempotency_key = $15::varchar AND le.tenant_id = $6::uuid)
		ON CONFLICT DO NOTHING`,
		e.ID, e.OccurredAt.UTC(), e.Type, e.StudentID, e.SchoolID, e.TenantID, e.GameSessionID, e.ActivityID, e.ContentID,
		e.ContentVersionID, uuidArray(e.ObjectiveIDs), e.Context, actor, payload, e.IdempotencyKey, e.EfficacyWaveID, armsArray(e.ExperimentArms),
		e.GradeLevel)
	if err != nil {
		return fmt.Errorf("learning event %s: %w", e.Type, err)
	}
	return nil
}

// Envelope is the learning-event.schema.json document of e (EVENT_DESIGN §5.1): the stored row plus the derived
// actor, subject and producer. actor.id is the student for student events, the staff user for staff events and the
// zero UUID for events the server generates on its own (system) [Proposed].
func Envelope(e write.LearningEvent, payload json.RawMessage, recordedAt time.Time) map[string]any {
	actorID := uuid.Nil
	switch {
	case e.ActorID != nil:
		actorID = *e.ActorID
	case e.ActorType == "student":
		actorID = e.StudentID
	}
	objectives := e.ObjectiveIDs
	if objectives == nil {
		objectives = []uuid.UUID{}
	}
	st, sid := subjectOf(e, payload)
	doc := map[string]any{"event_id": e.ID, "event_type": e.Type, "schema_version": 1,
		"occurred_at": e.OccurredAt.UTC().Format(time.RFC3339Nano), "recorded_at": recordedAt.UTC().Format(time.RFC3339Nano),
		"producer": Producer, "tenant_id": e.TenantID, "school_id": e.SchoolID, "student_id": e.StudentID,
		"actor": map[string]any{"type": e.ActorType, "id": actorID}, "subject": map[string]any{"type": st, "id": sid},
		"session_id": nil, "activity_id": nil, "content_id": nil, "content_version_id": nil,
		"context": eventContext(e), "objective_ids": objectives, "idempotency_key": e.IdempotencyKey,
		"payload": payload}
	if e.GameSessionID != nil {
		doc["session_id"] = *e.GameSessionID
	}
	if e.ActivityID != nil {
		doc["activity_id"] = e.ActivityID.String()
	}
	if e.ContentID != nil {
		doc["content_id"] = *e.ContentID
	}
	if e.ContentVersionID != nil {
		doc["content_version_id"] = *e.ContentVersionID
	}
	return doc
}

// subjectOf derives the envelope subject from the event type and its payload.
func subjectOf(e write.LearningEvent, payload json.RawMessage) (string, string) {
	var p struct {
		AttemptID   string `json:"attempt_id"`
		Position    int    `json:"position"`
		ObjectiveID string `json:"objective_id"`
		LessonID    string `json:"lesson_id"`
	}
	_ = json.Unmarshal(payload, &p)
	switch e.Type {
	case "question.answered", "question.viewed":
		if p.AttemptID != "" {
			return "attempt_item", p.AttemptID + ":" + strconv.Itoa(p.Position)
		}
	case "learning_objective.progressed", "learning_objective.mastered":
		if p.ObjectiveID != "" {
			return "learning_objective", p.ObjectiveID
		}
	case "lesson.completed":
		if e.ContentID != nil {
			return "lesson", e.ContentID.String()
		}
	}
	if p.AttemptID != "" {
		return "attempt", p.AttemptID
	}
	if e.ActivityID != nil {
		return "learning_activity", e.ActivityID.String()
	}
	return "learning_activity", e.ID.String()
}

// eventContext is the context of the envelope: the mode, the efficacy wave and the experiment arms (F035-AC-15, AC-25).
func eventContext(e write.LearningEvent) map[string]any {
	c := map[string]any{"mode": e.Context}
	if e.EfficacyWaveID != nil {
		c["efficacy_wave_id"] = *e.EfficacyWaveID
	}
	if len(e.ExperimentArms) > 0 {
		c["experiment_arms"] = e.ExperimentArms
	}
	return c
}

// armsArray is the varchar[] of the experiment arms (NULL when none).
func armsArray(arms []string) any {
	if len(arms) == 0 {
		return nil
	}
	return textArray(arms)
}
